package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	UserPrivateGroupValidityDays        = MaxValidityDays
	UserCarpoolGroupDefaultValidityDays = 365
)

var ErrUserPrivateGroupPlatformUnsupported = infraerrors.BadRequest(
	"USER_PRIVATE_GROUP_PLATFORM_UNSUPPORTED",
	"user private groups are not supported for this platform",
)

type UserPrivateGroupProvisioner interface {
	ProvisionUserPrivateGroups(ctx context.Context, userID int64) error
	GetActiveUserPrivateGroup(ctx context.Context, userID int64, platform string) (*Group, error)
}

type userPrivateGroupFinder interface {
	FindUserPrivateByOwnerAndPlatform(ctx context.Context, userID int64, platform string) (*Group, error)
}

type userPrivateGroupService struct {
	groupRepo          GroupRepository
	userRepo           UserRepository
	userSubRepo        UserSubscriptionRepository
	defaultSubAssigner DefaultSubscriptionAssigner
	settingService     *SettingService
}

func NewUserPrivateGroupService(
	groupRepo GroupRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	defaultSubAssigner DefaultSubscriptionAssigner,
	settingService *SettingService,
) UserPrivateGroupProvisioner {
	return &userPrivateGroupService{
		groupRepo:          groupRepo,
		userRepo:           userRepo,
		userSubRepo:        userSubRepo,
		defaultSubAssigner: defaultSubAssigner,
		settingService:     settingService,
	}
}

func (s *userPrivateGroupService) ProvisionUserPrivateGroups(ctx context.Context, userID int64) error {
	if err := s.validateUser(ctx, userID); err != nil {
		return err
	}
	for _, platform := range SupportedUserPrivateGroupPlatforms() {
		group, err := s.findOrCreateUserPrivateGroup(ctx, userID, platform)
		if err != nil {
			return err
		}
		if !group.IsActive() || !group.IsSubscriptionType() {
			return ErrGroupNotAllowed
		}
		if err := s.ensureInitialPrivateSubscription(ctx, userID, group.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *userPrivateGroupService) GetActiveUserPrivateGroup(ctx context.Context, userID int64, platform string) (*Group, error) {
	if err := s.validateUser(ctx, userID); err != nil {
		return nil, err
	}
	platform = normalizePrivateGroupPlatform(platform)
	if !IsSupportedUserPrivateGroupPlatform(platform) {
		return nil, ErrUserPrivateGroupPlatformUnsupported
	}

	group, err := s.findUserPrivateGroup(ctx, userID, platform)
	if err != nil {
		return nil, err
	}
	if !group.IsActive() || !group.IsSubscriptionType() || group.OwnerUserID == nil || *group.OwnerUserID != userID {
		return nil, ErrGroupNotAllowed
	}

	if _, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, userID, group.ID); err != nil {
		if errors.Is(err, ErrSubscriptionNotFound) {
			if existing, lookupErr := s.userSubRepo.GetByUserIDAndGroupID(ctx, userID, group.ID); lookupErr == nil && existing != nil && existing.IsExpired() {
				return nil, ErrSubscriptionExpired
			}
			return nil, ErrGroupNotAllowed
		}
		return nil, fmt.Errorf("get active private subscription: %w", err)
	}
	return group, nil
}

func (s *userPrivateGroupService) validateUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return ErrUserNotFound
	}
	if s == nil || s.groupRepo == nil || s.userRepo == nil || s.userSubRepo == nil {
		return ErrServiceUnavailable
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user == nil || user.ID <= 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *userPrivateGroupService) findOrCreateUserPrivateGroup(ctx context.Context, userID int64, platform string) (*Group, error) {
	platform = normalizePrivateGroupPlatform(platform)
	if !IsSupportedUserPrivateGroupPlatform(platform) {
		return nil, ErrUserPrivateGroupPlatformUnsupported
	}

	group, err := s.findUserPrivateGroup(ctx, userID, platform)
	if err == nil {
		return group, nil
	}
	if !errors.Is(err, ErrGroupNotFound) {
		return nil, err
	}

	template, err := s.loadTemplate(ctx)
	if err != nil {
		return nil, err
	}
	ownerID := userID
	group = &Group{
		Name:                        PrivateGroupName(userID, platform),
		Description:                 fmt.Sprintf("Private subscription group for user %d on %s.", userID, platform),
		Platform:                    platform,
		RateMultiplier:              template.RateMultiplier,
		IsExclusive:                 true,
		Status:                      StatusActive,
		OwnerUserID:                 &ownerID,
		Scope:                       GroupScopeUserPrivate,
		SubscriptionType:            SubscriptionTypeSubscription,
		AllowMessagesDispatch:       defaultPrivateGroupAllowMessagesDispatch(platform),
		DailyLimitUSD:               cloneFloat64Ptr(template.DailyLimitUSD),
		WeeklyLimitUSD:              cloneFloat64Ptr(template.WeeklyLimitUSD),
		MonthlyLimitUSD:             cloneFloat64Ptr(template.MonthlyLimitUSD),
		DefaultValidityDays:         UserPrivateGroupValidityDays,
		RPMLimit:                    template.RPMLimit,
		SupportedModelScopes:        []string{},
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{},
	}
	if err := s.groupRepo.Create(ctx, group); err != nil {
		if errors.Is(err, ErrGroupExists) {
			return s.findUserPrivateGroup(ctx, userID, platform)
		}
		return nil, fmt.Errorf("create private group: %w", err)
	}
	return group, nil
}

func (s *userPrivateGroupService) findUserPrivateGroup(ctx context.Context, userID int64, platform string) (*Group, error) {
	platform = normalizePrivateGroupPlatform(platform)
	if finder, ok := s.groupRepo.(userPrivateGroupFinder); ok {
		return finder.FindUserPrivateByOwnerAndPlatform(ctx, userID, platform)
	}

	groups, err := s.groupRepo.ListActiveByPlatform(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("list private groups: %w", err)
	}
	for i := range groups {
		group := &groups[i]
		if group.OwnerUserID != nil && *group.OwnerUserID == userID && group.IsUserPrivateScope() {
			return group, nil
		}
	}
	return nil, ErrGroupNotFound
}

func (s *userPrivateGroupService) ensureInitialPrivateSubscription(ctx context.Context, userID, groupID int64) error {
	if s.defaultSubAssigner == nil {
		return ErrServiceUnavailable
	}

	if _, err := s.userSubRepo.GetByUserIDAndGroupID(ctx, userID, groupID); err == nil {
	} else if !errors.Is(err, ErrSubscriptionNotFound) {
		return fmt.Errorf("get private subscription: %w", err)
	} else if _, _, err := s.defaultSubAssigner.AssignOrExtendSubscription(ctx, &AssignSubscriptionInput{
		UserID:       userID,
		GroupID:      groupID,
		ValidityDays: UserPrivateGroupValidityDays,
		Notes:        "auto assigned by user private group provisioning",
	}); err != nil && !errors.Is(err, ErrSubscriptionAlreadyExists) {
		return fmt.Errorf("assign private subscription: %w", err)
	}

	if err := s.userRepo.AddGroupToAllowedGroups(ctx, userID, groupID); err != nil {
		return fmt.Errorf("add private group to user: %w", err)
	}
	return nil
}

func (s *userPrivateGroupService) loadTemplate(ctx context.Context) (*UserPrivateGroupTemplate, error) {
	if s.settingService == nil {
		return nil, ErrServiceUnavailable
	}
	template, err := s.settingService.GetUserPrivateGroupTemplate(ctx)
	if err != nil {
		return nil, fmt.Errorf("get private group template: %w", err)
	}
	if template.RateMultiplier <= 0 {
		template.RateMultiplier = 1
	}
	if template.RPMLimit < 0 {
		template.RPMLimit = 0
	}
	return template, nil
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func normalizePrivateGroupPlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func defaultPrivateGroupAllowMessagesDispatch(platform string) bool {
	return normalizePrivateGroupPlatform(platform) == PlatformOpenAI
}
