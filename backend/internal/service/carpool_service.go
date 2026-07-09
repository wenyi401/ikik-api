package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"math"
	"strings"
	"time"
)

const (
	carpoolExternalUsageEpsilon        = 0.000001
	carpoolExternalOverageNotifyWindow = 24 * time.Hour
	carpoolUsagePointsPerAccount       = 100.0
)

type subscriptionCacheInvalidator interface {
	InvalidateSubscription(ctx context.Context, userID, groupID int64) error
}

type userCarpoolGroupFinder interface {
	FindUserCarpoolByOwnerAndPlatform(ctx context.Context, userID int64, platform string) (*Group, error)
}

type carpoolActivePoolByUserFinder interface {
	FindActivePoolByUserID(ctx context.Context, userID, excludePoolID int64) (*CarpoolPool, error)
}

type carpoolMemberUsageStatsByPoolLister interface {
	ListPoolMemberUsageStatsByPoolID(ctx context.Context, poolID int64, userIDs []int64) (map[int64]CarpoolMemberUsageStats, error)
}

type CarpoolService struct {
	repo                    CarpoolRepository
	groupRepo               GroupRepository
	accountRepo             AccountRepository
	proxyRepo               ProxyRepository
	userRepo                UserRepository
	userSubRepo             UserSubscriptionRepository
	subscriptionService     *SubscriptionService
	settingService          *SettingService
	subscriptionInvalidator subscriptionCacheInvalidator
	authCacheInvalidator    APIKeyAuthCacheInvalidator
	accountUsageService     *AccountUsageService
	emailService            *EmailService
	rateLimitService        *RateLimitService
}

func NewCarpoolService(
	repo CarpoolRepository,
	groupRepo GroupRepository,
	accountRepo AccountRepository,
	proxyRepo ProxyRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	subscriptionService *SubscriptionService,
	settingService *SettingService,
	subscriptionInvalidator subscriptionCacheInvalidator,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	accountUsageService *AccountUsageService,
	emailService *EmailService,
	rateLimitService *RateLimitService,
) *CarpoolService {
	return &CarpoolService{
		repo:                    repo,
		groupRepo:               groupRepo,
		accountRepo:             accountRepo,
		proxyRepo:               proxyRepo,
		userRepo:                userRepo,
		userSubRepo:             userSubRepo,
		subscriptionService:     subscriptionService,
		settingService:          settingService,
		subscriptionInvalidator: subscriptionInvalidator,
		authCacheInvalidator:    authCacheInvalidator,
		accountUsageService:     accountUsageService,
		emailService:            emailService,
		rateLimitService:        rateLimitService,
	}
}

func (s *CarpoolService) SetExternalUsageServices(accountUsageService *AccountUsageService, emailService *EmailService) {
	s.accountUsageService = accountUsageService
	s.emailService = emailService
}

type CarpoolMineOverview struct {
	Owned  []CarpoolPoolSummary `json:"owned"`
	Joined []CarpoolPoolSummary `json:"joined"`
}

func (s *CarpoolService) AdminListPools(ctx context.Context, filters AdminCarpoolPoolFilters) (*AdminCarpoolPoolListResult, error) {
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.PageSize <= 0 {
		filters.PageSize = 20
	}
	if filters.PageSize > 1000 {
		filters.PageSize = 1000
	}
	filters.Search = strings.TrimSpace(filters.Search)
	filters.Platform = NormalizeCarpoolPlatform(filters.Platform)
	filters.Status = strings.TrimSpace(filters.Status)

	items, total, err := s.repo.ListAdminPools(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("list admin carpool pools: %w", err)
	}
	return &AdminCarpoolPoolListResult{Items: items, Total: total}, nil
}

func (s *CarpoolService) AdminGetDetail(ctx context.Context, poolID int64) (*CarpoolPoolDetail, error) {
	pool, err := s.repo.GetPoolByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	return s.getDetail(ctx, pool.OwnerUserID, poolID, true, true)
}

func (s *CarpoolService) AdminClosePool(ctx context.Context, poolID int64) (*CarpoolPoolDetail, error) {
	pool, err := s.repo.GetPoolByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	if pool.Status != CarpoolPoolStatusClosed {
		if err := s.repo.UpdatePoolStatus(ctx, poolID, CarpoolPoolStatusClosed); err != nil {
			return nil, fmt.Errorf("close carpool pool: %w", err)
		}
		if pool.GroupID != nil && *pool.GroupID > 0 && s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, *pool.GroupID)
		}
	}
	return s.AdminGetDetail(ctx, poolID)
}

func (s *CarpoolService) AdminRepairPool(ctx context.Context, poolID int64) (*CarpoolPoolDetail, error) {
	pool, err := s.repo.GetPoolByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repairPoolIntegrity(ctx, pool); err != nil {
		return nil, fmt.Errorf("repair carpool pool integrity: %w", err)
	}
	return s.AdminGetDetail(ctx, poolID)
}

func (s *CarpoolService) AdminDeletePool(ctx context.Context, poolID int64) error {
	pool, err := s.repo.GetPoolByID(ctx, poolID)
	if err != nil {
		return err
	}
	return s.deletePool(ctx, pool)
}

func (s *CarpoolService) ListMine(ctx context.Context, userID int64) (*CarpoolMineOverview, error) {
	owned, err := s.repo.ListOwnedPools(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list owned carpool pools: %w", err)
	}
	joined, err := s.repo.ListJoinedPools(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list joined carpool pools: %w", err)
	}
	return &CarpoolMineOverview{Owned: owned, Joined: joined}, nil
}

func (s *CarpoolService) ListHall(ctx context.Context, userID int64) ([]CarpoolPoolSummary, error) {
	return []CarpoolPoolSummary{}, nil
}

func (s *CarpoolService) GetDetail(ctx context.Context, userID, poolID int64) (*CarpoolPoolDetail, error) {
	return s.getDetail(ctx, userID, poolID, false, true)
}

func (s *CarpoolService) GetDetailByInviteCode(ctx context.Context, userID int64, inviteCode string) (*CarpoolPoolDetail, error) {
	pool, err := s.repo.GetPoolByInviteCode(ctx, inviteCode)
	if err != nil {
		return nil, err
	}
	return s.getDetail(ctx, userID, pool.ID, true, true)
}

func (s *CarpoolService) GetUsageOverviewByGroupAndUser(ctx context.Context, groupID, userID int64) (*CarpoolUsageOverview, error) {
	if s == nil || s.repo == nil || groupID <= 0 || userID <= 0 {
		return nil, nil
	}
	pool, err := s.repo.GetPoolByGroupID(ctx, groupID)
	if errors.Is(err, ErrCarpoolPoolNotFound) {
		if s.groupRepo == nil {
			return nil, nil
		}
		group, groupErr := s.groupRepo.GetByID(ctx, groupID)
		if groupErr != nil || group == nil || !group.IsUserCarpoolScope() || group.OwnerUserID == nil || *group.OwnerUserID != userID {
			return nil, nil
		}
		finder, ok := s.repo.(carpoolActivePoolByUserPlatformFinder)
		if !ok {
			return nil, ErrServiceUnavailable
		}
		pool, err = finder.FindActivePoolByUserAndPlatform(ctx, userID, group.Platform)
		if err != nil {
			return nil, err
		}
		if pool == nil {
			return nil, nil
		}
	}
	if err != nil {
		return nil, err
	}
	detail, err := s.getDetail(ctx, userID, pool.ID, true, false)
	if err != nil {
		return nil, err
	}
	member := carpoolMemberProfileByUserID(detail.Members, userID)
	if member == nil {
		return nil, nil
	}
	if carpoolWeeklyUsageLimitExceeded(member.UsageWindows) && s.forceRefreshCarpoolOpenAIUsageSnapshots(ctx, pool.ID) {
		detail, err = s.getDetail(ctx, userID, pool.ID, true, false)
		if err != nil {
			return nil, err
		}
		member = carpoolMemberProfileByUserID(detail.Members, userID)
		if member == nil {
			return nil, nil
		}
	}
	return &CarpoolUsageOverview{
		Pool:    detail.Pool,
		Member:  *member,
		Windows: member.UsageWindows,
	}, nil
}

func carpoolMemberProfileByUserID(profiles []CarpoolMemberProfile, userID int64) *CarpoolMemberProfile {
	for i := range profiles {
		if profiles[i].Member.UserID == userID {
			return &profiles[i]
		}
	}
	return nil
}

func carpoolWeeklyUsageLimitExceeded(windows []CarpoolUsageWindow) bool {
	for i := range windows {
		window := windows[i]
		if window.Window != "7d" || window.LimitPoints <= 0 {
			continue
		}
		return window.UsedPoints >= window.LimitPoints || window.Utilization >= 100
	}
	return false
}

func (s *CarpoolService) getDetail(ctx context.Context, userID, poolID int64, allowInviteOnly bool, allowRepair bool) (*CarpoolPoolDetail, error) {
	pool, err := s.repo.GetPoolByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	isOwner := pool.OwnerUserID == userID
	member, _ := s.repo.GetMemberByPoolAndUser(ctx, poolID, userID)
	if !allowInviteOnly && !isOwner && member == nil && pool.Visibility == CarpoolPoolVisibilityInviteOnly {
		return nil, ErrCarpoolPoolNotFound
	}
	if allowRepair && (isOwner || member != nil) {
		repaired, repairErr := s.repairPoolIntegrity(ctx, pool)
		if repairErr != nil {
			return nil, fmt.Errorf("repair carpool pool integrity: %w", repairErr)
		}
		if repaired {
			pool, err = s.repo.GetPoolByID(ctx, poolID)
			if err != nil {
				return nil, err
			}
			member, _ = s.repo.GetMemberByPoolAndUser(ctx, poolID, userID)
		}
	}

	detail := &CarpoolPoolDetail{Pool: *pool}
	if pool.GroupID != nil && *pool.GroupID > 0 {
		group, groupErr := s.groupRepo.GetByID(ctx, *pool.GroupID)
		if groupErr == nil {
			detail.Group = group
		}
	}

	summaryList, err := s.repo.ListOwnedPools(ctx, pool.OwnerUserID)
	if err == nil {
		for i := range summaryList {
			if summaryList[i].Pool.ID == poolID {
				detail.Summaries = summaryList[i]
				break
			}
		}
	}
	if detail.Summaries.Pool.ID == 0 {
		detail.Summaries.Pool = *pool
	}
	detail.Summaries.IsOwner = isOwner
	if isOwner {
		detail.Summaries.CurrentUserStatus = CarpoolMemberRoleOwner
	} else if member != nil {
		detail.Summaries.CurrentUserStatus = member.Status
	} else if openReq, openErr := s.repo.GetOpenJoinRequestByPoolAndUser(ctx, poolID, userID); openErr == nil && openReq != nil {
		detail.Summaries.CurrentUserStatus = openReq.Status
		detail.Summaries.CurrentUserRequestID = &openReq.ID
	} else {
		detail.Summaries.CurrentUserStatus = ""
		detail.Summaries.CurrentUserRequestID = nil
	}

	var accounts []CarpoolPoolAccount
	shouldSyncPoolUsage := s.accountUsageService != nil && (isOwner || member != nil)
	if isOwner || shouldSyncPoolUsage {
		accounts, err = s.repo.ListPoolAccounts(ctx, poolID)
		if err != nil {
			return nil, fmt.Errorf("list carpool accounts: %w", err)
		}
		accounts = s.enrichCarpoolPoolAccountLimits(ctx, accounts)
		if shouldSyncPoolUsage {
			if err := s.reconcilePoolExternalUsage(ctx, pool, detail.Group, accounts); err != nil {
				slog.Warn("carpool_external_usage_reconcile_failed", "pool_id", poolID, "error", err)
			}
		}
		if isOwner {
			detail.Accounts = accounts
		}
	}
	detail.PoolUsageWindows = s.carpoolPoolUsageWindows(ctx, accounts)

	members, err := s.repo.ListPoolMembers(ctx, poolID)
	if err != nil {
		return nil, fmt.Errorf("list carpool members: %w", err)
	}
	canViewMemberRoster := isOwner || (member != nil && member.Status == CarpoolMemberStatusActive)
	memberProfiles := make([]CarpoolMemberProfile, 0, len(members))
	for i := range members {
		if !isOwner {
			if canViewMemberRoster {
				if members[i].Status != CarpoolMemberStatusActive && members[i].UserID != userID {
					continue
				}
			} else if members[i].UserID != userID {
				continue
			}
		}
		user, userErr := s.userRepo.GetByID(ctx, members[i].UserID)
		if userErr != nil {
			return nil, fmt.Errorf("load carpool member user %d: %w", members[i].UserID, userErr)
		}
		memberItem := normalizeCarpoolMemberUsageWindow(members[i], time.Now().UTC())
		profile := CarpoolMemberProfile{
			Member:      memberItem,
			MaskedEmail: MaskCarpoolEmail(user.Email),
			Username:    user.Username,
		}
		if memberItem.SubscriptionID != nil && *memberItem.SubscriptionID > 0 {
			sub, subErr := s.userSubRepo.GetByID(ctx, *memberItem.SubscriptionID)
			if subErr == nil && sub != nil {
				if memberItem.WeeklyLimitUSD > 0 {
					profile.WeeklyLimitUSD = memberItem.WeeklyLimitUSD
				} else if detail.Group != nil && detail.Group.WeeklyLimitUSD != nil {
					profile.WeeklyLimitUSD = *detail.Group.WeeklyLimitUSD
				}
				profile.WeeklyUsageUSD = sub.WeeklyUsageUSD
				profile.WeeklyResetAt = sub.WeeklyResetTime()
			}
		}
		memberProfiles = append(memberProfiles, profile)
	}
	s.attachCarpoolMemberUsageWindows(ctx, pool, accounts, memberProfiles)
	s.attachCarpoolMemberUsageStats(ctx, pool, memberProfiles)
	detail.Members = memberProfiles

	if isOwner {
		requests, err := s.repo.ListPoolJoinRequests(ctx, poolID)
		if err != nil {
			return nil, fmt.Errorf("list carpool join requests: %w", err)
		}
		usageMap, err := s.repo.ListPoolApplicantUsageStats(ctx, poolID)
		if err != nil {
			return nil, fmt.Errorf("list carpool applicant usage stats: %w", err)
		}
		requestProfiles := make([]CarpoolJoinRequestProfile, 0, len(requests))
		for i := range requests {
			user, userErr := s.userRepo.GetByID(ctx, requests[i].UserID)
			if userErr != nil {
				return nil, fmt.Errorf("load applicant user %d: %w", requests[i].UserID, userErr)
			}
			requestProfiles = append(requestProfiles, CarpoolJoinRequestProfile{
				Request:     requests[i],
				MaskedEmail: MaskCarpoolEmail(user.Email),
				Username:    user.Username,
				Usage:       usageMap[requests[i].UserID],
			})
		}
		detail.JoinRequests = requestProfiles
	}

	return detail, nil
}

func (s *CarpoolService) enrichCarpoolPoolAccountLimits(ctx context.Context, accounts []CarpoolPoolAccount) []CarpoolPoolAccount {
	if s.accountRepo == nil || len(accounts) == 0 {
		return accounts
	}
	for i := range accounts {
		account, err := s.accountRepo.GetByID(ctx, accounts[i].AccountID)
		if err != nil {
			slog.Warn("carpool_account_quota_load_failed", "account_id", accounts[i].AccountID, "error", err)
			continue
		}
		accounts[i].FiveHourLimitUSD = account.GetWindowCostLimit()
		accounts[i].WeeklyLimitUSD = account.GetQuotaWeeklyLimit()
	}
	return accounts
}

func (s *CarpoolService) allocateSystemProxyIDs(ctx context.Context, accountCount int) ([]int64, error) {
	if accountCount <= 0 {
		return nil, nil
	}
	if s.proxyRepo == nil {
		return nil, ErrCarpoolSystemProxyUnavailable
	}
	proxies, err := s.proxyRepo.ListActiveWithAccountCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active system proxies: %w", err)
	}
	now := time.Now().UTC()
	candidates := make([]ProxyWithAccountCount, 0, len(proxies))
	for i := range proxies {
		if proxies[i].IsExpired(now) {
			continue
		}
		candidates = append(candidates, proxies[i])
	}
	if len(candidates) == 0 {
		return nil, ErrCarpoolSystemProxyUnavailable
	}

	ids := make([]int64, 0, accountCount)
	for len(ids) < accountCount {
		best := 0
		for i := 1; i < len(candidates); i++ {
			if candidates[i].AccountCount < candidates[best].AccountCount ||
				(candidates[i].AccountCount == candidates[best].AccountCount && candidates[i].ID < candidates[best].ID) {
				best = i
			}
		}
		ids = append(ids, candidates[best].ID)
		candidates[best].AccountCount++
	}
	return ids, nil
}

func (s *CarpoolService) ensureNoActiveCarpoolForUser(ctx context.Context, userID, excludePoolID int64) error {
	if userID <= 0 {
		return ErrUserNotFound
	}
	finder, ok := s.repo.(carpoolActivePoolByUserFinder)
	if !ok {
		return ErrServiceUnavailable
	}
	pool, err := finder.FindActivePoolByUserID(ctx, userID, excludePoolID)
	if err != nil {
		return fmt.Errorf("check active carpool pool: %w", err)
	}
	if pool != nil {
		return ErrCarpoolUserAlreadyInPool
	}
	return nil
}

func (s *CarpoolService) ensureUserCarpoolGroupSubscription(ctx context.Context, userID int64, platform string, validityDays int, notes string, riskControl bool) (*Group, *UserSubscription, error) {
	if s == nil || s.groupRepo == nil || s.userRepo == nil || s.subscriptionService == nil {
		return nil, nil, ErrServiceUnavailable
	}
	group, err := s.findOrCreateUserCarpoolGroup(ctx, userID, platform)
	if err != nil {
		return nil, nil, err
	}
	if !group.IsActive() || !group.IsSubscriptionType() || group.OwnerUserID == nil || *group.OwnerUserID != userID {
		return nil, nil, ErrGroupNotAllowed
	}
	if validityDays <= 0 {
		validityDays = group.DefaultValidityDays
	}
	if validityDays <= 0 {
		validityDays = UserCarpoolGroupDefaultValidityDays
	}
	sub, _, err := s.subscriptionService.AssignOrExtendSubscription(ctx, &AssignSubscriptionInput{
		UserID:       userID,
		GroupID:      group.ID,
		ValidityDays: validityDays,
		Notes:        strings.TrimSpace(notes),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("assign user carpool subscription: %w", err)
	}
	if err := s.userRepo.AddGroupToAllowedGroups(ctx, userID, group.ID); err != nil {
		return nil, nil, fmt.Errorf("add user carpool group to user: %w", err)
	}
	if riskControl && s.settingService != nil {
		if err := s.settingService.AddContentModerationGroup(ctx, group.ID); err != nil {
			return nil, nil, fmt.Errorf("assign user carpool risk control group: %w", err)
		}
	}
	s.invalidateUserCarpoolGroupCaches(ctx, userID, group.ID)
	return group, sub, nil
}

func (s *CarpoolService) findOrCreateUserCarpoolGroup(ctx context.Context, userID int64, platform string) (*Group, error) {
	if s == nil || s.groupRepo == nil {
		return nil, ErrServiceUnavailable
	}
	platform = NormalizeCarpoolPlatform(platform)
	if !IsSupportedUserCarpoolGroupPlatform(platform) {
		return nil, ErrCarpoolInvalidPlatform
	}
	group, err := s.findUserCarpoolGroup(ctx, userID, platform)
	if err == nil {
		return group, nil
	}
	if !errors.Is(err, ErrGroupNotFound) {
		return nil, err
	}

	template := &UserPrivateGroupTemplate{RateMultiplier: 1}
	if s.settingService != nil {
		if loaded, loadErr := s.settingService.GetUserPrivateGroupTemplate(ctx); loadErr == nil && loaded != nil {
			template = loaded
		}
	}
	if template.RateMultiplier <= 0 {
		template.RateMultiplier = 1
	}
	if template.RPMLimit < 0 {
		template.RPMLimit = 0
	}

	ownerID := userID
	group = &Group{
		Name:                        CarpoolUserGroupName(userID, platform),
		Description:                 fmt.Sprintf("Carpool subscription group for user %d on %s.", userID, platform),
		Platform:                    platform,
		RateMultiplier:              template.RateMultiplier,
		IsExclusive:                 true,
		Status:                      StatusActive,
		OwnerUserID:                 &ownerID,
		Scope:                       GroupScopeUserCarpool,
		SubscriptionType:            SubscriptionTypeSubscription,
		DefaultValidityDays:         UserCarpoolGroupDefaultValidityDays,
		RPMLimit:                    template.RPMLimit,
		AllowMessagesDispatch:       defaultPrivateGroupAllowMessagesDispatch(platform),
		SupportedModelScopes:        []string{},
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{},
	}
	if err := s.groupRepo.Create(ctx, group); err != nil {
		if errors.Is(err, ErrGroupExists) {
			return s.findUserCarpoolGroup(ctx, userID, platform)
		}
		return nil, fmt.Errorf("create user carpool group: %w", err)
	}
	return group, nil
}

func (s *CarpoolService) findUserCarpoolGroup(ctx context.Context, userID int64, platform string) (*Group, error) {
	if s == nil || s.groupRepo == nil {
		return nil, ErrServiceUnavailable
	}
	platform = NormalizeCarpoolPlatform(platform)
	if finder, ok := s.groupRepo.(userCarpoolGroupFinder); ok {
		return finder.FindUserCarpoolByOwnerAndPlatform(ctx, userID, platform)
	}
	groups, err := s.groupRepo.ListActiveByPlatform(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("list user carpool groups: %w", err)
	}
	for i := range groups {
		group := &groups[i]
		if group.OwnerUserID != nil && *group.OwnerUserID == userID && group.IsUserCarpoolScope() {
			return group, nil
		}
	}
	return nil, ErrGroupNotFound
}

func (s *CarpoolService) invalidateUserCarpoolGroupCaches(ctx context.Context, userID, groupID int64) {
	if groupID <= 0 {
		return
	}
	if s.subscriptionInvalidator != nil && userID > 0 {
		_ = s.subscriptionInvalidator.InvalidateSubscription(ctx, userID, groupID)
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, groupID)
	}
}

func (s *CarpoolService) CreatePool(ctx context.Context, ownerUserID int64, req CreateCarpoolPoolRequest) (*CarpoolPoolDetail, error) {
	if req.TargetSeats < 2 || req.TargetSeats > 6 {
		return nil, ErrCarpoolInvalidSeats
	}
	if req.DurationDays <= 0 || req.DurationDays > 365 {
		return nil, ErrCarpoolInvalidDuration
	}
	platform := NormalizeCarpoolPlatform(req.Platform)
	if !IsSupportedCarpoolPlatform(platform) {
		return nil, ErrCarpoolInvalidPlatform
	}
	if err := s.ensureNoActiveCarpoolForUser(ctx, ownerUserID, 0); err != nil {
		return nil, err
	}

	pool, err := s.repo.CreatePool(ctx, CreateCarpoolPoolInput{
		OwnerUserID:          ownerUserID,
		InviteCode:           generateCarpoolInviteCode(),
		Name:                 strings.TrimSpace(req.Name),
		Platform:             platform,
		Visibility:           CarpoolPoolVisibilityInviteOnly,
		TargetSeats:          req.TargetSeats,
		DurationDays:         req.DurationDays,
		SeatPrice:            req.SeatPrice,
		ExtraFee:             req.ExtraFee,
		ExtraFeeDescription:  strings.TrimSpace(req.ExtraFeeDescription),
		SystemProxyEnabled:   req.SystemProxyEnabled,
		RiskControlEnabled:   req.RiskControlEnabled,
		Notes:                strings.TrimSpace(req.Notes),
		InitialQuotaSnapshot: CarpoolQuotaSnapshot{SnapshotAt: nil},
	})
	if err != nil {
		return nil, fmt.Errorf("create carpool pool: %w", err)
	}

	now := time.Now().UTC()
	_, sub, err := s.ensureUserCarpoolGroupSubscription(ctx, ownerUserID, platform, req.DurationDays, "carpool owner auto-assignment", pool.RiskControlEnabled)
	if err != nil {
		return nil, fmt.Errorf("assign owner carpool subscription: %w", err)
	}

	if _, err := s.repo.UpsertMember(ctx, UpsertCarpoolMemberInput{
		PoolID:           pool.ID,
		UserID:           ownerUserID,
		SubscriptionID:   &sub.ID,
		Role:             CarpoolMemberRoleOwner,
		Status:           CarpoolMemberStatusActive,
		PaidConfirmedAt:  &now,
		QuotaShareRatio:  defaultCarpoolQuotaShare(pool.TargetSeats),
		FiveHourLimitUSD: 0,
		WeeklyLimitUSD:   0,
	}); err != nil {
		return nil, fmt.Errorf("create owner carpool member: %w", err)
	}

	return s.GetDetail(ctx, ownerUserID, pool.ID)
}

func (s *CarpoolService) BindAccounts(ctx context.Context, ownerUserID, poolID int64, req BindCarpoolAccountsRequest) (*CarpoolPoolDetail, error) {
	accountIDs := uniquePositiveCarpoolIDs(req.AccountIDs)
	if len(accountIDs) == 0 {
		return nil, ErrCarpoolAccountsRequired
	}
	pool, err := s.requireOwnerPool(ctx, ownerUserID, poolID)
	if err != nil {
		return nil, err
	}

	var group *Group
	if pool.GroupID != nil && *pool.GroupID > 0 {
		group, err = s.groupRepo.GetByID(ctx, *pool.GroupID)
		if err != nil {
			return nil, fmt.Errorf("get carpool group: %w", err)
		}
	}

	accounts := make([]*Account, 0, len(accountIDs))
	var systemProxyIDs []int64
	if pool.SystemProxyEnabled {
		systemProxyIDs, err = s.allocateSystemProxyIDs(ctx, len(accountIDs))
		if err != nil {
			return nil, err
		}
	}
	if pool.RiskControlEnabled && s.settingService != nil {
		if group != nil && group.ID > 0 {
			if err := s.settingService.AddContentModerationGroup(ctx, group.ID); err != nil {
				return nil, fmt.Errorf("assign carpool risk control group: %w", err)
			}
		} else if members, memberErr := s.repo.ListPoolMembers(ctx, pool.ID); memberErr != nil {
			return nil, fmt.Errorf("list carpool members for risk control: %w", memberErr)
		} else {
			for i := range members {
				if members[i].Status != CarpoolMemberStatusActive {
					continue
				}
				userGroup, groupErr := s.findOrCreateUserCarpoolGroup(ctx, members[i].UserID, pool.Platform)
				if groupErr != nil {
					return nil, groupErr
				}
				if err := s.settingService.AddContentModerationGroup(ctx, userGroup.ID); err != nil {
					return nil, fmt.Errorf("assign user carpool risk control group: %w", err)
				}
			}
		}
	}
	for _, accountID := range accountIDs {
		account, getErr := s.accountRepo.GetByID(ctx, accountID)
		if getErr != nil {
			return nil, fmt.Errorf("get carpool account %d: %w", accountID, getErr)
		}
		if account.OwnerUserID == nil || *account.OwnerUserID != ownerUserID {
			return nil, ErrCarpoolAccountOwnership
		}
		if NormalizeCarpoolPlatform(account.Platform) != pool.Platform {
			return nil, ErrCarpoolAccountPlatform
		}
		existingPool, bindErr := s.repo.FindActivePoolByAccountID(ctx, accountID, poolID)
		if bindErr != nil {
			return nil, fmt.Errorf("check carpool account binding %d: %w", accountID, bindErr)
		}
		if existingPool != nil {
			return nil, ErrCarpoolAccountAlreadyBound
		}
		account.ShareMode = AccountShareModePrivate
		account.ShareStatus = AccountShareStatusApproved
		account.ErrorMessage = ""
		if pool.SystemProxyEnabled {
			proxyID := systemProxyIDs[len(accounts)%len(systemProxyIDs)]
			account.ProxyID = &proxyID
		}
		if err := s.accountRepo.Update(ctx, account); err != nil {
			return nil, fmt.Errorf("update carpool account %d: %w", accountID, err)
		}
		if group != nil && group.ID > 0 {
			groupIDs := append([]int64(nil), account.GroupIDs...)
			groupIDs = append(groupIDs, group.ID)
			if err := s.accountRepo.BindGroups(ctx, account.ID, uniquePositiveCarpoolIDs(groupIDs)); err != nil {
				return nil, fmt.Errorf("bind carpool account %d to group: %w", accountID, err)
			}
		}
		accounts = append(accounts, account)
	}

	if err := s.repo.ReplacePoolAccounts(ctx, poolID, accountIDs); err != nil {
		return nil, fmt.Errorf("replace carpool pool accounts: %w", err)
	}

	snapshot := buildCarpoolQuotaSnapshot(accounts, pool.TargetSeats)
	if _, err := s.repo.UpdatePoolGroupAndQuota(ctx, pool.ID, pool.GroupID, snapshot); err != nil {
		return nil, fmt.Errorf("update carpool pool quota snapshot: %w", err)
	}
	if err := s.repo.UpdateMembersQuotaFromSnapshot(ctx, pool.ID, snapshot, defaultCarpoolQuotaShare(pool.TargetSeats)); err != nil {
		return nil, fmt.Errorf("update carpool member quota limits: %w", err)
	}
	if group != nil && group.ID > 0 {
		group.WeeklyLimitUSD = positiveFloat64Ptr(s.carpoolGroupWeeklyLimit(ctx, pool.ID, snapshot.PerMemberWeeklyLimitUSD))
		if err := s.groupRepo.Update(ctx, group); err != nil {
			return nil, fmt.Errorf("update carpool group weekly limit: %w", err)
		}
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, group.ID)
		}
	}

	return s.GetDetail(ctx, ownerUserID, poolID)
}

func (s *CarpoolService) ResetPoolAccountLocalLimit(ctx context.Context, ownerUserID, poolID, accountID int64) (*CarpoolPoolDetail, error) {
	pool, err := s.requireOwnerPool(ctx, ownerUserID, poolID)
	if err != nil {
		return nil, err
	}
	if accountID <= 0 {
		return nil, ErrAccountNotFound
	}

	accounts, err := s.repo.ListPoolAccounts(ctx, poolID)
	if err != nil {
		return nil, fmt.Errorf("list carpool accounts: %w", err)
	}
	var tracked *CarpoolPoolAccount
	for i := range accounts {
		if accounts[i].AccountID == accountID {
			tracked = &accounts[i]
			break
		}
	}
	if tracked == nil {
		return nil, ErrCarpoolAccountOwnership
	}

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get carpool account %d: %w", accountID, err)
	}
	if account.OwnerUserID == nil || *account.OwnerUserID != ownerUserID || NormalizeCarpoolPlatform(account.Platform) != pool.Platform {
		return nil, ErrCarpoolAccountOwnership
	}

	oneAccount := s.enrichCarpoolPoolAccountLimits(ctx, []CarpoolPoolAccount{*tracked})
	var group *Group
	if pool.GroupID != nil && *pool.GroupID > 0 && s.groupRepo != nil {
		if loaded, groupErr := s.groupRepo.GetByID(ctx, *pool.GroupID); groupErr == nil {
			group = loaded
		}
	}
	if err := s.reconcilePoolExternalUsageWithOptions(ctx, pool, group, oneAccount, carpoolExternalUsageReconcileOptions{
		chargeCurrentOnFirstSample: true,
		chargeFullWeeklyUsage:      true,
	}); err != nil {
		return nil, fmt.Errorf("reconcile carpool account usage before reset: %w", err)
	}

	if err := s.accountRepo.ClearError(ctx, accountID); err != nil {
		return nil, fmt.Errorf("clear carpool account error: %w", err)
	}
	if s.rateLimitService != nil {
		if err := s.rateLimitService.ClearRateLimit(ctx, accountID); err != nil {
			return nil, fmt.Errorf("clear carpool account local limit: %w", err)
		}
	} else {
		if err := s.accountRepo.ClearRateLimit(ctx, accountID); err != nil {
			return nil, fmt.Errorf("clear carpool account rate limit: %w", err)
		}
		if err := s.accountRepo.ClearAntigravityQuotaScopes(ctx, accountID); err != nil {
			return nil, fmt.Errorf("clear carpool account quota scopes: %w", err)
		}
		if err := s.accountRepo.ClearModelRateLimits(ctx, accountID); err != nil {
			return nil, fmt.Errorf("clear carpool account model limits: %w", err)
		}
		if err := s.accountRepo.ClearTempUnschedulable(ctx, accountID); err != nil {
			return nil, fmt.Errorf("clear carpool account temp unschedulable: %w", err)
		}
	}

	if pool.GroupID != nil && *pool.GroupID > 0 && s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, *pool.GroupID)
	} else if userGroup, groupErr := s.findUserCarpoolGroup(ctx, ownerUserID, pool.Platform); groupErr == nil && userGroup != nil {
		s.invalidateUserCarpoolGroupCaches(ctx, ownerUserID, userGroup.ID)
	}
	return s.GetDetail(ctx, ownerUserID, poolID)
}

func (s *CarpoolService) DeletePool(ctx context.Context, ownerUserID, poolID int64) error {
	pool, err := s.requireOwnerPool(ctx, ownerUserID, poolID)
	if err != nil {
		return err
	}
	return s.deletePool(ctx, pool)
}

func (s *CarpoolService) deletePool(ctx context.Context, pool *CarpoolPool) error {
	if pool == nil {
		return ErrCarpoolPoolNotFound
	}
	members, err := s.repo.ListPoolMembers(ctx, pool.ID)
	if err != nil {
		return fmt.Errorf("list carpool members before delete: %w", err)
	}

	var affectedUserIDs []int64
	var legacyGroupID int64
	if pool.GroupID != nil && *pool.GroupID > 0 {
		group, groupErr := s.groupRepo.GetByID(ctx, *pool.GroupID)
		if groupErr != nil && !errors.Is(groupErr, ErrGroupNotFound) {
			return fmt.Errorf("load carpool group before delete: %w", groupErr)
		}
		if group == nil || !group.IsUserCarpoolScope() {
			legacyGroupID = *pool.GroupID
			if pool.RiskControlEnabled && s.settingService != nil {
				if err := s.settingService.RemoveContentModerationGroup(ctx, legacyGroupID); err != nil {
					return fmt.Errorf("remove carpool risk control group: %w", err)
				}
			}
			var err error
			affectedUserIDs, err = s.groupRepo.DeleteCascade(ctx, legacyGroupID)
			if err != nil && !errors.Is(err, ErrGroupNotFound) {
				return fmt.Errorf("delete carpool group: %w", err)
			}
			if s.authCacheInvalidator != nil {
				s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, legacyGroupID)
			}
		}
	}
	if legacyGroupID == 0 {
		for i := range members {
			member := members[i]
			if member.Status != CarpoolMemberStatusActive {
				continue
			}
			if member.SubscriptionID != nil && *member.SubscriptionID > 0 && s.subscriptionService != nil {
				if err := s.subscriptionService.RevokeSubscription(ctx, *member.SubscriptionID); err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
					return fmt.Errorf("revoke carpool member subscription: %w", err)
				}
			}
			if pool.RiskControlEnabled && s.settingService != nil {
				if userGroup, groupErr := s.findUserCarpoolGroup(ctx, member.UserID, pool.Platform); groupErr == nil && userGroup != nil {
					if err := s.settingService.RemoveContentModerationGroup(ctx, userGroup.ID); err != nil {
						return fmt.Errorf("remove user carpool risk control group: %w", err)
					}
					s.invalidateUserCarpoolGroupCaches(ctx, member.UserID, userGroup.ID)
				}
			}
		}
	}
	if err := s.repo.DeletePool(ctx, pool.ID); err != nil {
		return fmt.Errorf("delete carpool pool: %w", err)
	}
	if err := s.repo.ReplacePoolAccounts(ctx, pool.ID, nil); err != nil {
		return fmt.Errorf("clear carpool pool accounts: %w", err)
	}
	if s.subscriptionInvalidator != nil {
		seen := map[int64]struct{}{pool.OwnerUserID: struct{}{}}
		if legacyGroupID > 0 {
			_ = s.subscriptionInvalidator.InvalidateSubscription(ctx, pool.OwnerUserID, legacyGroupID)
		}
		for _, userID := range affectedUserIDs {
			if _, ok := seen[userID]; ok {
				continue
			}
			seen[userID] = struct{}{}
			if legacyGroupID > 0 {
				_ = s.subscriptionInvalidator.InvalidateSubscription(ctx, userID, legacyGroupID)
			}
		}
	}
	return nil
}

func (s *CarpoolService) Apply(ctx context.Context, userID, poolID int64, req ApplyCarpoolPoolRequest) (*CarpoolJoinRequest, error) {
	return nil, ErrCarpoolPoolNotFound
}

func (s *CarpoolService) apply(ctx context.Context, userID, poolID int64, req ApplyCarpoolPoolRequest, allowInviteOnly bool) (*CarpoolJoinRequest, error) {
	pool, err := s.repo.GetPoolByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	if pool.Visibility == CarpoolPoolVisibilityInviteOnly && !allowInviteOnly {
		return nil, ErrCarpoolPoolNotFound
	}
	if pool.OwnerUserID == userID {
		return nil, ErrCarpoolSelfJoinNotAllowed
	}
	if pool.Status == CarpoolPoolStatusClosed {
		return nil, ErrCarpoolPoolClosed
	}
	if err := s.ensureNoActiveCarpoolForUser(ctx, userID, poolID); err != nil {
		return nil, err
	}
	if existingMember, _ := s.repo.GetMemberByPoolAndUser(ctx, poolID, userID); existingMember != nil && existingMember.Status == CarpoolMemberStatusActive {
		return nil, ErrCarpoolAlreadyMember
	}
	if openReq, err := s.repo.GetOpenJoinRequestByPoolAndUser(ctx, poolID, userID); err != nil {
		return nil, fmt.Errorf("get open carpool join request: %w", err)
	} else if openReq != nil {
		return nil, ErrCarpoolAlreadyApplied
	}

	members, err := s.repo.ListPoolMembers(ctx, poolID)
	if err != nil {
		return nil, fmt.Errorf("list carpool pool members: %w", err)
	}
	activeCount := 0
	for i := range members {
		if members[i].Status == CarpoolMemberStatusActive {
			activeCount++
		}
	}
	if activeCount >= pool.TargetSeats {
		return nil, ErrCarpoolPoolFull
	}

	item, err := s.repo.CreateJoinRequest(ctx, poolID, userID, strings.TrimSpace(req.Note))
	if err != nil {
		return nil, fmt.Errorf("create carpool join request: %w", err)
	}
	return item, nil
}

func (s *CarpoolService) ApplyByInviteCode(ctx context.Context, userID int64, inviteCode string, req ApplyCarpoolPoolRequest) (*CarpoolJoinRequest, error) {
	pool, err := s.repo.GetPoolByInviteCode(ctx, inviteCode)
	if err != nil {
		return nil, err
	}
	return s.apply(ctx, userID, pool.ID, req, true)
}

func (s *CarpoolService) ApproveJoinRequest(ctx context.Context, ownerUserID, poolID, requestID int64, req ReviewCarpoolJoinRequest) (*CarpoolJoinRequest, error) {
	pool, err := s.requireOwnerPool(ctx, ownerUserID, poolID)
	if err != nil {
		return nil, err
	}
	if pool.Status == CarpoolPoolStatusClosed {
		return nil, ErrCarpoolPoolClosed
	}
	request, err := s.repo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if request.PoolID != poolID {
		return nil, ErrCarpoolJoinRequestNotFound
	}
	if request.Status != CarpoolJoinRequestStatusPending {
		return nil, ErrCarpoolJoinRequestReviewed
	}
	item, err := s.repo.UpdateJoinRequestStatus(ctx, requestID, CarpoolJoinRequestStatusApproved, strings.TrimSpace(req.ReviewNote), time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("approve carpool join request: %w", err)
	}
	return item, nil
}

func (s *CarpoolService) RejectJoinRequest(ctx context.Context, ownerUserID, poolID, requestID int64, req ReviewCarpoolJoinRequest) (*CarpoolJoinRequest, error) {
	if _, err := s.requireOwnerPool(ctx, ownerUserID, poolID); err != nil {
		return nil, err
	}
	request, err := s.repo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if request.PoolID != poolID {
		return nil, ErrCarpoolJoinRequestNotFound
	}
	if request.Status != CarpoolJoinRequestStatusPending && request.Status != CarpoolJoinRequestStatusApproved {
		return nil, ErrCarpoolJoinRequestReviewed
	}
	item, err := s.repo.UpdateJoinRequestStatus(ctx, requestID, CarpoolJoinRequestStatusRejected, strings.TrimSpace(req.ReviewNote), time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("reject carpool join request: %w", err)
	}
	return item, nil
}

func (s *CarpoolService) ConfirmJoinPaid(ctx context.Context, ownerUserID, poolID, requestID int64) (*CarpoolPoolDetail, error) {
	pool, err := s.requireOwnerPool(ctx, ownerUserID, poolID)
	if err != nil {
		return nil, err
	}
	request, err := s.repo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if request.PoolID != poolID {
		return nil, ErrCarpoolJoinRequestNotFound
	}
	if request.Status != CarpoolJoinRequestStatusApproved {
		return nil, ErrCarpoolJoinRequestNotApproved
	}
	if err := s.ensureNoActiveCarpoolForUser(ctx, request.UserID, poolID); err != nil {
		return nil, err
	}
	if activeMember, _ := s.repo.GetMemberByPoolAndUser(ctx, poolID, request.UserID); activeMember != nil && activeMember.Status == CarpoolMemberStatusActive {
		return nil, ErrCarpoolAlreadyMember
	}

	now := time.Now().UTC()
	userGroup, sub, err := s.ensureUserCarpoolGroupSubscription(ctx, request.UserID, pool.Platform, pool.DurationDays, fmt.Sprintf("carpool pool %d member activation", poolID), pool.RiskControlEnabled)
	if err != nil {
		return nil, fmt.Errorf("assign carpool member subscription: %w", err)
	}
	if _, err := s.repo.UpsertMember(ctx, UpsertCarpoolMemberInput{
		PoolID:             poolID,
		UserID:             request.UserID,
		SubscriptionID:     &sub.ID,
		Role:               CarpoolMemberRoleMember,
		Status:             CarpoolMemberStatusActive,
		PaidConfirmedAt:    &now,
		QuotaShareRatio:    defaultCarpoolQuotaShare(pool.TargetSeats),
		FiveHourLimitUSD:   carpoolLimitFromShare(pool.TotalFiveHourLimitUSD, pool.PerMemberFiveHourLimitUSD, defaultCarpoolQuotaShare(pool.TargetSeats)),
		WeeklyLimitUSD:     carpoolLimitFromShare(pool.TotalWeeklyLimitUSD, pool.PerMemberWeeklyLimitUSD, defaultCarpoolQuotaShare(pool.TargetSeats)),
		ResetFiveHourUsage: true,
	}); err != nil {
		return nil, fmt.Errorf("upsert carpool member: %w", err)
	}
	if pool.GroupID != nil && *pool.GroupID > 0 {
		if group, groupErr := s.groupRepo.GetByID(ctx, *pool.GroupID); groupErr == nil && group != nil {
			group.WeeklyLimitUSD = positiveFloat64Ptr(s.carpoolGroupWeeklyLimit(ctx, pool.ID, pool.PerMemberWeeklyLimitUSD))
			if err := s.groupRepo.Update(ctx, group); err != nil {
				return nil, fmt.Errorf("update carpool group weekly limit: %w", err)
			}
		}
	}
	if err := s.repo.ActivateJoinRequest(ctx, requestID, now); err != nil {
		return nil, fmt.Errorf("activate carpool join request: %w", err)
	}
	if err := s.refreshPoolStatus(ctx, pool); err != nil {
		return nil, err
	}
	s.invalidateUserCarpoolGroupCaches(ctx, request.UserID, userGroup.ID)

	return s.GetDetail(ctx, ownerUserID, poolID)
}

func (s *CarpoolService) RemoveMember(ctx context.Context, ownerUserID, poolID, memberID int64) (*CarpoolPoolDetail, error) {
	pool, err := s.requireOwnerPool(ctx, ownerUserID, poolID)
	if err != nil {
		return nil, err
	}
	member, err := s.repo.GetMemberByID(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if member.PoolID != poolID {
		return nil, ErrCarpoolMemberNotFound
	}
	if member.Role == CarpoolMemberRoleOwner {
		return nil, ErrCarpoolOwnerOnly
	}
	if err := s.repo.UpdateMemberStatus(ctx, memberID, CarpoolMemberStatusRemoved, time.Now().UTC()); err != nil {
		return nil, fmt.Errorf("remove carpool member: %w", err)
	}
	if member.SubscriptionID != nil && *member.SubscriptionID > 0 {
		if err := s.subscriptionService.RevokeSubscription(ctx, *member.SubscriptionID); err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
			return nil, fmt.Errorf("revoke carpool member subscription: %w", err)
		}
	}
	if pool.GroupID != nil && *pool.GroupID > 0 {
		if group, groupErr := s.groupRepo.GetByID(ctx, *pool.GroupID); groupErr == nil && group != nil {
			group.WeeklyLimitUSD = positiveFloat64Ptr(s.carpoolGroupWeeklyLimit(ctx, pool.ID, pool.PerMemberWeeklyLimitUSD))
			if err := s.groupRepo.Update(ctx, group); err != nil {
				return nil, fmt.Errorf("update carpool group weekly limit: %w", err)
			}
		}
	}
	if pool.GroupID != nil && *pool.GroupID > 0 && s.subscriptionInvalidator != nil {
		_ = s.subscriptionInvalidator.InvalidateSubscription(ctx, member.UserID, *pool.GroupID)
	} else if userGroup, groupErr := s.findUserCarpoolGroup(ctx, member.UserID, pool.Platform); groupErr == nil && userGroup != nil {
		s.invalidateUserCarpoolGroupCaches(ctx, member.UserID, userGroup.ID)
		if pool.RiskControlEnabled && s.settingService != nil {
			if err := s.settingService.RemoveContentModerationGroup(ctx, userGroup.ID); err != nil {
				return nil, fmt.Errorf("remove user carpool risk control group: %w", err)
			}
		}
	}
	if err := s.refreshPoolStatus(ctx, pool); err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, ownerUserID, poolID)
}

func (s *CarpoolService) UpdateMemberAllocations(ctx context.Context, ownerUserID, poolID int64, req UpdateCarpoolMemberAllocationsRequest) (*CarpoolPoolDetail, error) {
	pool, err := s.requireOwnerPool(ctx, ownerUserID, poolID)
	if err != nil {
		return nil, err
	}
	if pool.Status == CarpoolPoolStatusClosed {
		return nil, ErrCarpoolPoolClosed
	}
	members, err := s.repo.ListPoolMembers(ctx, poolID)
	if err != nil {
		return nil, fmt.Errorf("list carpool members: %w", err)
	}

	activeMembers := make(map[int64]CarpoolMember, len(members))
	userIDs := make([]int64, 0, len(members))
	for i := range members {
		if members[i].Status != CarpoolMemberStatusActive {
			continue
		}
		activeMembers[members[i].ID] = members[i]
		userIDs = append(userIDs, members[i].UserID)
	}
	if len(activeMembers) == 0 || len(req.Allocations) != len(activeMembers) {
		return nil, ErrCarpoolInvalidAllocation
	}

	seen := make(map[int64]struct{}, len(req.Allocations))
	updates := make([]CarpoolMemberAllocationUpdate, 0, len(req.Allocations))
	totalShare := 0.0
	totalFiveHour := carpoolEffectiveTotalLimit(pool.TotalFiveHourLimitUSD, pool.PerMemberFiveHourLimitUSD, pool.TargetSeats)
	totalWeekly := carpoolEffectiveTotalLimit(pool.TotalWeeklyLimitUSD, pool.PerMemberWeeklyLimitUSD, pool.TargetSeats)
	maxWeekly := 0.0
	for _, allocation := range req.Allocations {
		member, ok := activeMembers[allocation.MemberID]
		if !ok {
			return nil, ErrCarpoolInvalidAllocation
		}
		if _, exists := seen[allocation.MemberID]; exists {
			return nil, ErrCarpoolInvalidAllocation
		}
		seen[allocation.MemberID] = struct{}{}
		share := allocation.QuotaShareRatio
		if math.IsNaN(share) || math.IsInf(share, 0) || share < 0 || share > 1 {
			return nil, ErrCarpoolInvalidAllocation
		}
		totalShare += share
		fiveHourLimit := totalFiveHour * share
		weeklyLimit := totalWeekly * share
		if weeklyLimit > maxWeekly {
			maxWeekly = weeklyLimit
		}
		updates = append(updates, CarpoolMemberAllocationUpdate{
			MemberID:         member.ID,
			QuotaShareRatio:  share,
			FiveHourLimitUSD: fiveHourLimit,
			WeeklyLimitUSD:   weeklyLimit,
		})
	}
	if math.Abs(totalShare-1) > 0.0001 {
		return nil, ErrCarpoolInvalidAllocation
	}

	if err := s.repo.UpdateMemberAllocations(ctx, poolID, updates); err != nil {
		return nil, fmt.Errorf("update carpool member allocations: %w", err)
	}
	if pool.GroupID != nil && *pool.GroupID > 0 {
		group, groupErr := s.groupRepo.GetByID(ctx, *pool.GroupID)
		if groupErr != nil {
			return nil, fmt.Errorf("get carpool group: %w", groupErr)
		}
		group.WeeklyLimitUSD = positiveFloat64Ptr(maxWeekly)
		if err := s.groupRepo.Update(ctx, group); err != nil {
			return nil, fmt.Errorf("update carpool group weekly limit: %w", err)
		}
	}
	if s.subscriptionInvalidator != nil {
		s.invalidateCarpoolMemberSubscriptionCaches(ctx, pool, userIDs)
	}
	if s.authCacheInvalidator != nil && pool.GroupID != nil && *pool.GroupID > 0 {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, *pool.GroupID)
	}
	return s.GetDetail(ctx, ownerUserID, poolID)
}

func (s *CarpoolService) CheckGroupFiveHourEligibility(ctx context.Context, userID, groupID int64, now time.Time) error {
	if s.repo == nil || groupID <= 0 || userID <= 0 {
		return nil
	}
	limit, err := s.repo.GetRuntimeMemberLimitByGroupAndUser(ctx, groupID, userID, now)
	if err != nil {
		return fmt.Errorf("get runtime carpool member limit: %w", err)
	}
	if limit == nil || limit.FiveHourLimitUSD <= 0 {
		return nil
	}
	used := limit.FiveHourUsedUSD
	if limit.FiveHourWindowStart != nil && limit.FiveHourWindowStart.Add(5*time.Hour).Before(now) {
		used = 0
	}
	if used >= limit.FiveHourLimitUSD {
		return ErrCarpoolFiveHourLimitExceeded
	}
	return nil
}

func (s *CarpoolService) repairPoolIntegrity(ctx context.Context, pool *CarpoolPool) (bool, error) {
	if s == nil || pool == nil {
		return false, nil
	}
	if pool.Status == CarpoolPoolStatusClosed {
		return false, nil
	}
	var repaired bool
	var group *Group
	var legacyGroupID int64
	if pool.GroupID != nil && *pool.GroupID > 0 {
		var err error
		pool, group, repaired, err = s.ensureCarpoolPoolGroup(ctx, pool)
		if err != nil {
			return repaired, err
		}
		if group != nil && group.ID > 0 && !group.IsUserCarpoolScope() {
			legacyGroupID = group.ID
		}
	}

	accountRepaired, err := s.repairPoolAccountBindings(ctx, pool, legacyGroupID)
	if err != nil {
		return repaired || accountRepaired, err
	}
	repaired = repaired || accountRepaired

	var memberRepaired bool
	var affectedUserIDs []int64
	if legacyGroupID > 0 {
		memberRepaired, affectedUserIDs, err = s.repairPoolMemberSubscriptions(ctx, pool, legacyGroupID)
	} else {
		memberRepaired, affectedUserIDs, err = s.repairUserCarpoolMemberSubscriptions(ctx, pool)
	}
	if err != nil {
		return repaired || memberRepaired, err
	}
	repaired = repaired || memberRepaired
	if repaired {
		if err := s.refreshPoolStatus(ctx, pool); err != nil {
			return true, err
		}
		if s.authCacheInvalidator != nil && legacyGroupID > 0 {
			s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, legacyGroupID)
		}
		s.invalidateCarpoolMemberSubscriptionCaches(ctx, pool, affectedUserIDs)
	}
	return repaired, nil
}

func (s *CarpoolService) ensureCarpoolPoolGroup(ctx context.Context, pool *CarpoolPool) (*CarpoolPool, *Group, bool, error) {
	if s.groupRepo == nil || s.repo == nil || pool == nil {
		return pool, nil, false, nil
	}
	var repaired bool
	var group *Group
	if pool.GroupID != nil && *pool.GroupID > 0 {
		loaded, err := s.groupRepo.GetByID(ctx, *pool.GroupID)
		if err != nil && !errors.Is(err, ErrGroupNotFound) {
			return pool, nil, repaired, fmt.Errorf("load carpool group: %w", err)
		}
		group = loaded
		if group != nil && group.IsUserCarpoolScope() {
			return pool, group, repaired, nil
		}
	}
	if group == nil {
		group = &Group{
			Name:                CarpoolGroupName(pool.ID, pool.Name),
			Description:         strings.TrimSpace(pool.Notes),
			Platform:            pool.Platform,
			RateMultiplier:      1,
			IsExclusive:         true,
			Status:              StatusActive,
			Scope:               GroupScopePublic,
			SubscriptionType:    SubscriptionTypeSubscription,
			WeeklyLimitUSD:      positiveFloat64Ptr(s.carpoolGroupWeeklyLimit(ctx, pool.ID, pool.PerMemberWeeklyLimitUSD)),
			DefaultValidityDays: pool.DurationDays,
		}
		if err := s.groupRepo.Create(ctx, group); err != nil {
			return pool, nil, repaired, fmt.Errorf("recreate carpool group: %w", err)
		}
		updated, err := s.repo.UpdatePoolGroupAndQuota(ctx, pool.ID, &group.ID, CarpoolQuotaSnapshot{
			TotalFiveHourLimitUSD:     pool.TotalFiveHourLimitUSD,
			TotalWeeklyLimitUSD:       pool.TotalWeeklyLimitUSD,
			PerMemberFiveHourLimitUSD: pool.PerMemberFiveHourLimitUSD,
			PerMemberWeeklyLimitUSD:   pool.PerMemberWeeklyLimitUSD,
			SnapshotAt:                pool.QuotaSnapshotAt,
		})
		if err != nil {
			return pool, nil, repaired, fmt.Errorf("rebind recreated carpool group: %w", err)
		}
		pool = updated
		repaired = true
	} else {
		groupChanged := false
		expectedName := CarpoolGroupName(pool.ID, pool.Name)
		if strings.TrimSpace(group.Name) == "" {
			group.Name = expectedName
			groupChanged = true
		}
		if group.Platform != pool.Platform {
			group.Platform = pool.Platform
			groupChanged = true
		}
		if group.RateMultiplier <= 0 {
			group.RateMultiplier = 1
			groupChanged = true
		}
		if !group.IsExclusive {
			group.IsExclusive = true
			groupChanged = true
		}
		if group.Status != StatusActive {
			group.Status = StatusActive
			groupChanged = true
		}
		if NormalizeGroupScope(group.Scope) != GroupScopePublic {
			group.Scope = GroupScopePublic
			groupChanged = true
		}
		if group.SubscriptionType != SubscriptionTypeSubscription {
			group.SubscriptionType = SubscriptionTypeSubscription
			groupChanged = true
		}
		if group.DefaultValidityDays <= 0 && pool.DurationDays > 0 {
			group.DefaultValidityDays = pool.DurationDays
			groupChanged = true
		}
		expectedWeekly := positiveFloat64Ptr(s.carpoolGroupWeeklyLimit(ctx, pool.ID, pool.PerMemberWeeklyLimitUSD))
		if !sameOptionalCarpoolFloat(group.WeeklyLimitUSD, expectedWeekly) {
			group.WeeklyLimitUSD = expectedWeekly
			groupChanged = true
		}
		if groupChanged {
			if err := s.groupRepo.Update(ctx, group); err != nil {
				return pool, nil, repaired, fmt.Errorf("repair carpool group: %w", err)
			}
			repaired = true
		}
	}
	if pool.RiskControlEnabled && s.settingService != nil {
		if err := s.settingService.AddContentModerationGroup(ctx, group.ID); err != nil {
			return pool, group, repaired, fmt.Errorf("repair carpool risk control group: %w", err)
		}
	}
	return pool, group, repaired, nil
}

func (s *CarpoolService) repairPoolAccountBindings(ctx context.Context, pool *CarpoolPool, groupID int64) (bool, error) {
	if s.repo == nil || s.accountRepo == nil || pool == nil {
		return false, nil
	}
	trackedAccounts, err := s.repo.ListPoolAccounts(ctx, pool.ID)
	if err != nil {
		return false, fmt.Errorf("list carpool accounts for repair: %w", err)
	}
	var repaired bool
	var systemProxyIDs []int64
	for i := range trackedAccounts {
		tracked := trackedAccounts[i]
		account, err := s.accountRepo.GetByID(ctx, tracked.AccountID)
		if err != nil {
			slog.Warn("carpool_repair_account_load_failed", "pool_id", pool.ID, "account_id", tracked.AccountID, "error", err)
			continue
		}
		if account.OwnerUserID == nil || *account.OwnerUserID != pool.OwnerUserID || NormalizeCarpoolPlatform(account.Platform) != pool.Platform {
			slog.Warn("carpool_repair_account_skipped", "pool_id", pool.ID, "account_id", account.ID, "reason", "owner_or_platform_mismatch")
			continue
		}

		accountChanged := false
		if account.ShareMode != AccountShareModePrivate {
			account.ShareMode = AccountShareModePrivate
			accountChanged = true
		}
		if account.ShareStatus != AccountShareStatusApproved {
			account.ShareStatus = AccountShareStatusApproved
			accountChanged = true
		}
		if account.ErrorMessage != "" {
			account.ErrorMessage = ""
			accountChanged = true
		}
		if pool.SystemProxyEnabled && account.ProxyID == nil {
			if len(systemProxyIDs) == 0 {
				systemProxyIDs, err = s.allocateSystemProxyIDs(ctx, 1)
				if err != nil {
					return repaired, err
				}
			}
			if len(systemProxyIDs) > 0 {
				proxyID := systemProxyIDs[0]
				account.ProxyID = &proxyID
				accountChanged = true
			}
		}
		if accountChanged {
			if err := s.accountRepo.Update(ctx, account); err != nil {
				return repaired, fmt.Errorf("repair carpool account %d: %w", account.ID, err)
			}
			repaired = true
		}
		if groupID > 0 && !containsCarpoolInt64(account.GroupIDs, groupID) {
			nextGroupIDs := append([]int64(nil), account.GroupIDs...)
			nextGroupIDs = append(nextGroupIDs, groupID)
			if err := s.accountRepo.BindGroups(ctx, account.ID, uniquePositiveCarpoolIDs(nextGroupIDs)); err != nil {
				return repaired, fmt.Errorf("repair carpool account %d group binding: %w", account.ID, err)
			}
			repaired = true
		}
	}
	return repaired, nil
}

func (s *CarpoolService) repairPoolMemberSubscriptions(ctx context.Context, pool *CarpoolPool, groupID int64) (bool, []int64, error) {
	if s.repo == nil || s.userSubRepo == nil || pool == nil || groupID <= 0 {
		return false, nil, nil
	}
	members, err := s.repo.ListPoolMembers(ctx, pool.ID)
	if err != nil {
		return false, nil, fmt.Errorf("list carpool members for repair: %w", err)
	}
	var repaired bool
	affectedUserIDs := make([]int64, 0, len(members)+1)
	ownerActive := false
	for i := range members {
		member := members[i]
		if member.Status != CarpoolMemberStatusActive {
			continue
		}
		if member.UserID == pool.OwnerUserID && member.Role == CarpoolMemberRoleOwner {
			ownerActive = true
		}
		itemRepaired, err := s.repairActiveCarpoolMemberSubscription(ctx, pool, groupID, member)
		if err != nil {
			return repaired, affectedUserIDs, err
		}
		if itemRepaired {
			repaired = true
			affectedUserIDs = append(affectedUserIDs, member.UserID)
		}
	}
	if !ownerActive && pool.OwnerUserID > 0 {
		paidAt := pool.CreatedAt.UTC()
		member := CarpoolMember{
			PoolID:           pool.ID,
			UserID:           pool.OwnerUserID,
			Role:             CarpoolMemberRoleOwner,
			Status:           CarpoolMemberStatusActive,
			PaidConfirmedAt:  &paidAt,
			QuotaShareRatio:  defaultCarpoolQuotaShare(pool.TargetSeats),
			FiveHourLimitUSD: pool.PerMemberFiveHourLimitUSD,
			FiveHourUsedUSD:  0,
			WeeklyLimitUSD:   pool.PerMemberWeeklyLimitUSD,
			CreatedAt:        pool.CreatedAt,
			UpdatedAt:        pool.UpdatedAt,
		}
		itemRepaired, err := s.repairActiveCarpoolMemberSubscription(ctx, pool, groupID, member)
		if err != nil {
			return repaired, affectedUserIDs, err
		}
		if itemRepaired {
			repaired = true
			affectedUserIDs = append(affectedUserIDs, pool.OwnerUserID)
		}
	}
	return repaired, affectedUserIDs, nil
}

func (s *CarpoolService) repairUserCarpoolMemberSubscriptions(ctx context.Context, pool *CarpoolPool) (bool, []int64, error) {
	if s.repo == nil || s.userSubRepo == nil || pool == nil {
		return false, nil, nil
	}
	members, err := s.repo.ListPoolMembers(ctx, pool.ID)
	if err != nil {
		return false, nil, fmt.Errorf("list carpool members for repair: %w", err)
	}
	var repaired bool
	affectedUserIDs := make([]int64, 0, len(members)+1)
	ownerActive := false
	for i := range members {
		member := members[i]
		if member.Status != CarpoolMemberStatusActive {
			continue
		}
		if member.UserID == pool.OwnerUserID && member.Role == CarpoolMemberRoleOwner {
			ownerActive = true
		}
		itemRepaired, err := s.repairActiveUserCarpoolMemberSubscription(ctx, pool, member)
		if err != nil {
			return repaired, affectedUserIDs, err
		}
		if itemRepaired {
			repaired = true
			affectedUserIDs = append(affectedUserIDs, member.UserID)
		}
	}
	if !ownerActive && pool.OwnerUserID > 0 {
		paidAt := pool.CreatedAt.UTC()
		member := CarpoolMember{
			PoolID:           pool.ID,
			UserID:           pool.OwnerUserID,
			Role:             CarpoolMemberRoleOwner,
			Status:           CarpoolMemberStatusActive,
			PaidConfirmedAt:  &paidAt,
			QuotaShareRatio:  defaultCarpoolQuotaShare(pool.TargetSeats),
			FiveHourLimitUSD: pool.PerMemberFiveHourLimitUSD,
			FiveHourUsedUSD:  0,
			WeeklyLimitUSD:   pool.PerMemberWeeklyLimitUSD,
			CreatedAt:        pool.CreatedAt,
			UpdatedAt:        pool.UpdatedAt,
		}
		itemRepaired, err := s.repairActiveUserCarpoolMemberSubscription(ctx, pool, member)
		if err != nil {
			return repaired, affectedUserIDs, err
		}
		if itemRepaired {
			repaired = true
			affectedUserIDs = append(affectedUserIDs, pool.OwnerUserID)
		}
	}
	return repaired, affectedUserIDs, nil
}

func (s *CarpoolService) repairActiveUserCarpoolMemberSubscription(ctx context.Context, pool *CarpoolPool, member CarpoolMember) (bool, error) {
	if s.userSubRepo == nil || s.repo == nil || s.userRepo == nil || pool == nil || member.UserID <= 0 {
		return false, nil
	}
	group, err := s.findOrCreateUserCarpoolGroup(ctx, member.UserID, pool.Platform)
	if err != nil {
		return false, err
	}
	if err := s.userRepo.AddGroupToAllowedGroups(ctx, member.UserID, group.ID); err != nil {
		return false, fmt.Errorf("add user carpool group to user: %w", err)
	}
	if pool.RiskControlEnabled && s.settingService != nil {
		if err := s.settingService.AddContentModerationGroup(ctx, group.ID); err != nil {
			return false, fmt.Errorf("assign user carpool risk control group: %w", err)
		}
	}
	return s.repairActiveCarpoolMemberSubscription(ctx, pool, group.ID, member)
}

func (s *CarpoolService) repairActiveCarpoolMemberSubscription(ctx context.Context, pool *CarpoolPool, groupID int64, member CarpoolMember) (bool, error) {
	if s.userSubRepo == nil || s.repo == nil || pool == nil || member.UserID <= 0 || groupID <= 0 {
		return false, nil
	}
	startAt, expiresAt := carpoolMemberSubscriptionWindow(pool, member)
	now := time.Now()
	if !expiresAt.After(now) {
		return false, nil
	}

	sub, err := s.loadReusableCarpoolMemberSubscription(ctx, member, groupID)
	if err != nil {
		return false, err
	}
	var repaired bool
	if sub == nil {
		sub = &UserSubscription{
			UserID:     member.UserID,
			GroupID:    groupID,
			StartsAt:   startAt,
			ExpiresAt:  expiresAt,
			Status:     SubscriptionStatusActive,
			AssignedAt: now,
			Notes:      fmt.Sprintf("carpool pool %d integrity repair", pool.ID),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := s.userSubRepo.Create(ctx, sub); err != nil {
			existing, getErr := s.userSubRepo.GetByUserIDAndGroupID(ctx, member.UserID, groupID)
			if getErr != nil {
				return repaired, fmt.Errorf("create carpool member subscription: %w", err)
			}
			sub = existing
		}
		repaired = true
	}
	changedSub, err := s.repairCarpoolSubscriptionWindow(ctx, sub, expiresAt)
	if err != nil {
		return repaired, err
	}
	repaired = repaired || changedSub

	if member.SubscriptionID == nil || *member.SubscriptionID != sub.ID {
		if _, err := s.repo.UpsertMember(ctx, UpsertCarpoolMemberInput{
			PoolID:             pool.ID,
			UserID:             member.UserID,
			SubscriptionID:     &sub.ID,
			Role:               member.Role,
			Status:             CarpoolMemberStatusActive,
			PaidConfirmedAt:    member.PaidConfirmedAt,
			QuotaShareRatio:    normalizeCarpoolQuotaShare(member.QuotaShareRatio, pool.TargetSeats),
			FiveHourLimitUSD:   member.FiveHourLimitUSD,
			WeeklyLimitUSD:     member.WeeklyLimitUSD,
			ResetFiveHourUsage: false,
		}); err != nil {
			return repaired, fmt.Errorf("repair carpool member subscription binding: %w", err)
		}
		repaired = true
	}
	return repaired, nil
}

func (s *CarpoolService) loadReusableCarpoolMemberSubscription(ctx context.Context, member CarpoolMember, groupID int64) (*UserSubscription, error) {
	if member.SubscriptionID != nil && *member.SubscriptionID > 0 {
		sub, err := s.userSubRepo.GetByID(ctx, *member.SubscriptionID)
		if err == nil && sub != nil && sub.UserID == member.UserID && sub.GroupID == groupID {
			return sub, nil
		}
		if err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
			return nil, fmt.Errorf("load carpool member subscription: %w", err)
		}
	}
	sub, err := s.userSubRepo.GetByUserIDAndGroupID(ctx, member.UserID, groupID)
	if errors.Is(err, ErrSubscriptionNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load carpool member subscription by user/group: %w", err)
	}
	return sub, nil
}

func (s *CarpoolService) repairCarpoolSubscriptionWindow(ctx context.Context, sub *UserSubscription, expectedExpiresAt time.Time) (bool, error) {
	if sub == nil {
		return false, nil
	}
	var repaired bool
	if sub.ExpiresAt.Before(expectedExpiresAt) {
		if err := s.userSubRepo.ExtendExpiry(ctx, sub.ID, expectedExpiresAt); err != nil {
			return repaired, fmt.Errorf("repair carpool subscription expiry: %w", err)
		}
		repaired = true
	}
	if sub.Status != SubscriptionStatusActive {
		if err := s.userSubRepo.UpdateStatus(ctx, sub.ID, SubscriptionStatusActive); err != nil {
			return repaired, fmt.Errorf("repair carpool subscription status: %w", err)
		}
		repaired = true
	}
	return repaired, nil
}

func carpoolMemberSubscriptionWindow(pool *CarpoolPool, member CarpoolMember) (time.Time, time.Time) {
	startAt := time.Now().UTC()
	if member.PaidConfirmedAt != nil && !member.PaidConfirmedAt.IsZero() {
		startAt = member.PaidConfirmedAt.UTC()
	} else if !member.CreatedAt.IsZero() {
		startAt = member.CreatedAt.UTC()
	} else if pool != nil && !pool.CreatedAt.IsZero() {
		startAt = pool.CreatedAt.UTC()
	}
	durationDays := 30
	if pool != nil && pool.DurationDays > 0 {
		durationDays = pool.DurationDays
	}
	expiresAt := startAt.AddDate(0, 0, durationDays)
	if expiresAt.After(MaxExpiresAt) {
		expiresAt = MaxExpiresAt
	}
	return startAt, expiresAt
}

func (s *CarpoolService) requireOwnerPool(ctx context.Context, ownerUserID, poolID int64) (*CarpoolPool, error) {
	pool, err := s.repo.GetPoolByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	if pool.OwnerUserID != ownerUserID {
		return nil, ErrCarpoolOwnerOnly
	}
	return pool, nil
}

func (s *CarpoolService) refreshPoolStatus(ctx context.Context, pool *CarpoolPool) error {
	if pool == nil {
		return nil
	}
	members, err := s.repo.ListPoolMembers(ctx, pool.ID)
	if err != nil {
		return fmt.Errorf("list members for pool status refresh: %w", err)
	}
	active := 0
	for i := range members {
		if members[i].Status == CarpoolMemberStatusActive {
			active++
		}
	}
	nextStatus := NormalizeCarpoolPoolStatus(pool.Status, active, pool.TargetSeats)
	if nextStatus == pool.Status {
		return nil
	}
	if err := s.repo.UpdatePoolStatus(ctx, pool.ID, nextStatus); err != nil {
		return fmt.Errorf("update carpool pool status: %w", err)
	}
	return nil
}

func (s *CarpoolService) SyncExternalUsageForAccount(ctx context.Context, accountID int64) error {
	if s == nil || s.repo == nil || accountID <= 0 {
		return nil
	}
	pool, err := s.repo.FindActivePoolByAccountID(ctx, accountID, 0)
	if errors.Is(err, ErrCarpoolPoolNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if pool == nil {
		return nil
	}
	accounts, err := s.repo.ListPoolAccounts(ctx, pool.ID)
	if err != nil {
		return fmt.Errorf("list carpool accounts: %w", err)
	}
	accounts = s.enrichCarpoolPoolAccountLimits(ctx, accounts)
	var group *Group
	if pool.GroupID != nil && *pool.GroupID > 0 && s.groupRepo != nil {
		if loaded, groupErr := s.groupRepo.GetByID(ctx, *pool.GroupID); groupErr == nil {
			group = loaded
		}
	}
	return s.reconcilePoolExternalUsage(ctx, pool, group, accounts)
}

func (s *CarpoolService) forceRefreshCarpoolOpenAIUsageSnapshots(ctx context.Context, poolID int64) bool {
	if s == nil || s.repo == nil || s.accountRepo == nil || s.accountUsageService == nil || poolID <= 0 {
		return false
	}
	accounts, err := s.repo.ListPoolAccounts(ctx, poolID)
	if err != nil {
		slog.Warn("carpool_usage_force_refresh_list_accounts_failed", "pool_id", poolID, "error", err)
		return false
	}
	refreshed := false
	for i := range accounts {
		account, err := s.accountRepo.GetByID(ctx, accounts[i].AccountID)
		if err != nil {
			slog.Warn("carpool_usage_force_refresh_load_account_failed", "pool_id", poolID, "account_id", accounts[i].AccountID, "error", err)
			continue
		}
		if account == nil || account.Platform != PlatformOpenAI || !account.IsOAuth() {
			continue
		}
		updates, err := s.accountUsageService.probeOpenAICodexSnapshot(ctx, account)
		if err != nil {
			slog.Warn("carpool_usage_force_refresh_probe_failed", "pool_id", poolID, "account_id", account.ID, "error", err)
			continue
		}
		if len(updates) == 0 {
			continue
		}
		if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
			slog.Warn("carpool_usage_force_refresh_persist_failed", "pool_id", poolID, "account_id", account.ID, "error", err)
			continue
		}
		refreshed = true
	}
	return refreshed
}

type carpoolExternalUsageReconcileOptions struct {
	chargeCurrentOnFirstSample bool
	chargeFullWeeklyUsage      bool
}

func (s *CarpoolService) reconcilePoolExternalUsage(ctx context.Context, pool *CarpoolPool, group *Group, accounts []CarpoolPoolAccount) error {
	return s.reconcilePoolExternalUsageWithOptions(ctx, pool, group, accounts, carpoolExternalUsageReconcileOptions{})
}

func (s *CarpoolService) reconcilePoolExternalUsageWithOptions(ctx context.Context, pool *CarpoolPool, group *Group, accounts []CarpoolPoolAccount, options carpoolExternalUsageReconcileOptions) error {
	if pool == nil || len(accounts) == 0 || s.accountUsageService == nil || s.repo == nil {
		return nil
	}
	now := time.Now().UTC()
	var totalFiveHourDelta float64
	var totalWeeklyDelta float64
	var fiveHourResetChanged bool
	var weeklyResetChanged bool
	var fiveHourWindowStart *time.Time
	var weeklyWindowStart time.Time
	var notifyAccount *CarpoolPoolAccount
	accountLabels := make([]string, 0, len(accounts))

	for i := range accounts {
		tracked := accounts[i]
		account, err := s.accountRepo.GetByID(ctx, tracked.AccountID)
		if err != nil {
			slog.Warn("carpool_external_usage_load_account_failed", "pool_id", pool.ID, "account_id", tracked.AccountID, "error", err)
			continue
		}
		usage, err := s.accountUsageService.GetUsage(ctx, account.ID)
		if err != nil {
			slog.Warn("carpool_external_usage_get_usage_failed", "pool_id", pool.ID, "account_id", account.ID, "error", err)
			continue
		}

		fiveHourInternal := 0.0
		if usage != nil && usage.FiveHour != nil && usage.FiveHour.WindowStats != nil {
			fiveHourInternal = usage.FiveHour.WindowStats.Cost
		}
		fiveHour := computeCarpoolExternalUsageDelta(
			tracked.ExternalFiveHourUsedUSD,
			tracked.ExternalFiveHourResetAt,
			tracked.ExternalCheckedAt,
			carpoolExternalUsageLimit(account.GetWindowCostLimit()),
			progressResetAt(usageProgress(usage, "5h")),
			progressUtilization(usageProgress(usage, "5h")),
			carpoolExternalUsageInternal(account.GetWindowCostLimit(), fiveHourInternal),
			options.chargeCurrentOnFirstSample,
		)

		weeklyInternal := 0.0
		if usage != nil && usage.SevenDay != nil && !options.chargeFullWeeklyUsage {
			if stats, statErr := s.accountUsageService.GetAccountWindowStats(ctx, account.ID, now.Add(-7*24*time.Hour)); statErr == nil && stats != nil {
				weeklyInternal = stats.Cost
			} else if statErr != nil {
				slog.Warn("carpool_external_usage_weekly_stats_failed", "pool_id", pool.ID, "account_id", account.ID, "error", statErr)
			}
		}
		weekly := computeCarpoolExternalUsageDelta(
			tracked.ExternalWeeklyUsedUSD,
			tracked.ExternalWeeklyResetAt,
			tracked.ExternalCheckedAt,
			carpoolExternalUsageLimit(account.GetQuotaWeeklyLimit()),
			progressResetAt(usageProgress(usage, "7d")),
			progressUtilization(usageProgress(usage, "7d")),
			carpoolExternalUsageInternal(account.GetQuotaWeeklyLimit(), weeklyInternal),
			options.chargeCurrentOnFirstSample,
		)

		if !fiveHour.Valid && !weekly.Valid {
			continue
		}
		update := CarpoolPoolAccountExternalUsageUpdate{
			ExternalFiveHourUsedUSD: tracked.ExternalFiveHourUsedUSD,
			ExternalWeeklyUsedUSD:   tracked.ExternalWeeklyUsedUSD,
			ExternalFiveHourResetAt: tracked.ExternalFiveHourResetAt,
			ExternalWeeklyResetAt:   tracked.ExternalWeeklyResetAt,
			CheckedAt:               now,
		}
		if fiveHour.Valid {
			update.ExternalFiveHourUsedUSD = fiveHour.CurrentExternalUSD
			update.ExternalFiveHourResetAt = fiveHour.ResetAt
			totalFiveHourDelta += fiveHour.DeltaUSD
			accounts[i].ExternalFiveHourUsedUSD = update.ExternalFiveHourUsedUSD
			accounts[i].ExternalFiveHourResetAt = update.ExternalFiveHourResetAt
			if fiveHour.ResetChanged {
				fiveHourResetChanged = true
				fiveHourWindowStart = carpoolWindowStartFromResetAt(fiveHour.ResetAt, 5*time.Hour)
			}
		}
		if weekly.Valid {
			update.ExternalWeeklyUsedUSD = weekly.CurrentExternalUSD
			update.ExternalWeeklyResetAt = weekly.ResetAt
			totalWeeklyDelta += weekly.DeltaUSD
			accounts[i].ExternalWeeklyUsedUSD = update.ExternalWeeklyUsedUSD
			accounts[i].ExternalWeeklyResetAt = update.ExternalWeeklyResetAt
			if weekly.ResetChanged {
				weeklyResetChanged = true
				weeklyWindowStart = carpoolWindowStartFromResetAtOrNow(weekly.ResetAt, 7*24*time.Hour, now)
			}
		}
		if err := s.repo.UpdatePoolAccountExternalUsage(ctx, pool.ID, account.ID, update); err != nil {
			return fmt.Errorf("update carpool account external usage: %w", err)
		}
		checkedAt := update.CheckedAt
		accounts[i].ExternalCheckedAt = &checkedAt
		if (fiveHour.DeltaUSD > carpoolExternalUsageEpsilon || weekly.DeltaUSD > carpoolExternalUsageEpsilon) && notifyAccount == nil {
			trackedCopy := tracked
			notifyAccount = &trackedCopy
		}
		if fiveHour.DeltaUSD > carpoolExternalUsageEpsilon || weekly.DeltaUSD > carpoolExternalUsageEpsilon {
			accountLabels = append(accountLabels, account.Name)
		}
	}

	if fiveHourResetChanged {
		resetUserIDs, err := s.repo.ResetPoolMembersFiveHourUsage(ctx, pool.ID, fiveHourWindowStart)
		if err != nil {
			return fmt.Errorf("reset carpool five-hour usage: %w", err)
		}
		s.invalidateCarpoolMemberSubscriptionCaches(ctx, pool, resetUserIDs)
	}
	if weeklyResetChanged {
		resetUserIDs, err := s.repo.ResetPoolMemberWeeklyUsage(ctx, pool.ID, weeklyWindowStart)
		if err != nil {
			return fmt.Errorf("reset carpool weekly usage: %w", err)
		}
		s.invalidateCarpoolMemberSubscriptionCaches(ctx, pool, resetUserIDs)
	}

	if totalFiveHourDelta <= carpoolExternalUsageEpsilon && totalWeeklyDelta <= carpoolExternalUsageEpsilon {
		return nil
	}

	ownerMember, err := s.repo.GetMemberByPoolAndUser(ctx, pool.ID, pool.OwnerUserID)
	if err != nil {
		return fmt.Errorf("get carpool owner member: %w", err)
	}
	if totalFiveHourDelta > carpoolExternalUsageEpsilon {
		ownerMember, err = s.repo.IncrementOwnerMemberFiveHourUsage(ctx, pool.ID, pool.OwnerUserID, totalFiveHourDelta, now)
		if err != nil {
			return fmt.Errorf("increment owner carpool five-hour usage: %w", err)
		}
	}

	var ownerSub *UserSubscription
	if totalWeeklyDelta > carpoolExternalUsageEpsilon && ownerMember.SubscriptionID != nil {
		if err := s.userSubRepo.IncrementUsage(ctx, *ownerMember.SubscriptionID, totalWeeklyDelta); err != nil {
			return fmt.Errorf("increment owner carpool subscription usage: %w", err)
		}
		ownerSub, _ = s.userSubRepo.GetByID(ctx, *ownerMember.SubscriptionID)
	}
	if s.subscriptionInvalidator != nil && pool.GroupID != nil && *pool.GroupID > 0 {
		_ = s.subscriptionInvalidator.InvalidateSubscription(ctx, pool.OwnerUserID, *pool.GroupID)
	} else if pool.GroupID == nil {
		if userGroup, groupErr := s.findUserCarpoolGroup(ctx, pool.OwnerUserID, pool.Platform); groupErr == nil && userGroup != nil {
			s.invalidateUserCarpoolGroupCaches(ctx, pool.OwnerUserID, userGroup.ID)
		}
	}

	if notifyAccount != nil && shouldNotifyCarpoolExternalOverage(*notifyAccount, now) && s.isOwnerOverCarpoolLimit(ownerMember, ownerSub, group) {
		if err := s.sendCarpoolExternalOverageEmail(ctx, pool, group, ownerMember, ownerSub, totalFiveHourDelta, totalWeeklyDelta, accountLabels); err != nil {
			slog.Warn("carpool_external_overage_email_failed", "pool_id", pool.ID, "owner_user_id", pool.OwnerUserID, "error", err)
		} else if err := s.repo.MarkPoolAccountExternalOverageNotified(ctx, pool.ID, notifyAccount.AccountID, now); err != nil {
			slog.Warn("carpool_external_overage_mark_notified_failed", "pool_id", pool.ID, "account_id", notifyAccount.AccountID, "error", err)
		}
	}
	return nil
}

func carpoolExternalUsageLimit(configuredLimit float64) float64 {
	if configuredLimit > 0 {
		return configuredLimit
	}
	return carpoolUsagePointsPerAccount
}

func carpoolExternalUsageInternal(configuredLimit, internalCost float64) float64 {
	if configuredLimit > 0 {
		return internalCost
	}
	return 0
}

type carpoolExternalUsageDelta struct {
	Valid              bool
	CurrentExternalUSD float64
	DeltaUSD           float64
	ResetAt            *time.Time
	ResetChanged       bool
}

func computeCarpoolExternalUsageDelta(previousExternalUSD float64, previousResetAt, previousCheckedAt *time.Time, limitUSD float64, resetAt *time.Time, utilizationPercent, internalCostUSD float64, chargeCurrentOnFirstSample ...bool) carpoolExternalUsageDelta {
	if limitUSD <= 0 || utilizationPercent < 0 {
		return carpoolExternalUsageDelta{}
	}
	if internalCostUSD < 0 {
		internalCostUSD = 0
	}
	upstreamUsedUSD := limitUSD * utilizationPercent / 100
	if upstreamUsedUSD < 0 {
		upstreamUsedUSD = 0
	}
	currentExternalUSD := upstreamUsedUSD - internalCostUSD
	if currentExternalUSD < carpoolExternalUsageEpsilon {
		currentExternalUSD = 0
	}

	delta := 0.0
	resetChanged := false
	if previousCheckedAt != nil {
		base := previousExternalUSD
		resetChanged = !sameCarpoolResetAt(previousResetAt, resetAt)
		if resetChanged {
			base = 0
		}
		delta = currentExternalUSD - base
		if delta < carpoolExternalUsageEpsilon {
			delta = 0
		}
	} else if len(chargeCurrentOnFirstSample) > 0 && chargeCurrentOnFirstSample[0] {
		delta = currentExternalUSD
	}
	return carpoolExternalUsageDelta{
		Valid:              true,
		CurrentExternalUSD: currentExternalUSD,
		DeltaUSD:           delta,
		ResetAt:            resetAt,
		ResetChanged:       resetChanged,
	}
}

func carpoolWindowStartFromResetAt(resetAt *time.Time, window time.Duration) *time.Time {
	if resetAt == nil {
		return nil
	}
	windowStart := resetAt.UTC().Add(-window)
	return &windowStart
}

func carpoolWindowStartFromResetAtOrNow(resetAt *time.Time, window time.Duration, now time.Time) time.Time {
	if start := carpoolWindowStartFromResetAt(resetAt, window); start != nil {
		return *start
	}
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func normalizeCarpoolMemberUsageWindow(member CarpoolMember, now time.Time) CarpoolMember {
	if member.FiveHourWindowStart == nil {
		return member
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if member.FiveHourWindowStart.Add(5 * time.Hour).After(now.UTC()) {
		return member
	}
	member.FiveHourUsedUSD = 0
	member.FiveHourWindowStart = nil
	return member
}

func (s *CarpoolService) attachCarpoolMemberUsageWindows(ctx context.Context, pool *CarpoolPool, accounts []CarpoolPoolAccount, profiles []CarpoolMemberProfile) {
	if pool == nil || len(profiles) == 0 {
		return
	}
	if len(accounts) == 0 && s.repo != nil {
		loaded, err := s.repo.ListPoolAccounts(ctx, pool.ID)
		if err == nil {
			accounts = loaded
		}
	}
	displayLimitPoints := carpoolUsagePointsPerAccount
	fiveHourPoolUsage := s.carpoolPoolWindowUsagePoints(ctx, accounts, "5h")
	weeklyPoolUsage := s.carpoolPoolWindowUsagePoints(ctx, accounts, "7d")

	totalFiveHourUsed := 0.0
	totalWeeklyUsed := 0.0
	for i := range profiles {
		totalFiveHourUsed += positiveCarpoolFloat(profiles[i].Member.FiveHourUsedUSD)
		totalWeeklyUsed += positiveCarpoolFloat(profiles[i].WeeklyUsageUSD)
	}

	for i := range profiles {
		actualMemberLimitPoints := carpoolMemberLimitPoints(pool.TargetSeats, len(accounts), profiles[i].Member.QuotaShareRatio)
		fiveHourResetAt := carpoolMemberFiveHourResetAt(profiles[i].Member)
		if fiveHourResetAt == nil {
			fiveHourResetAt = fiveHourPoolUsage.ResetAt
		}
		weeklyResetAt := profiles[i].WeeklyResetAt
		if weeklyResetAt == nil {
			weeklyResetAt = weeklyPoolUsage.ResetAt
		}

		profiles[i].UsageWindows = []CarpoolUsageWindow{
			newCarpoolUsageWindow("5h", carpoolNormalizedMemberUsedPoints(
				profiles[i].Member.FiveHourUsedUSD,
				displayLimitPoints,
				actualMemberLimitPoints,
				totalFiveHourUsed,
				fiveHourPoolUsage.UsedPoints,
				fiveHourPoolUsage.Valid,
			), displayLimitPoints, fiveHourResetAt),
			newCarpoolUsageWindow("7d", carpoolNormalizedMemberUsedPoints(
				profiles[i].WeeklyUsageUSD,
				displayLimitPoints,
				actualMemberLimitPoints,
				totalWeeklyUsed,
				weeklyPoolUsage.UsedPoints,
				weeklyPoolUsage.Valid,
			), displayLimitPoints, weeklyResetAt),
		}
	}
}

func (s *CarpoolService) carpoolPoolUsageWindows(ctx context.Context, accounts []CarpoolPoolAccount) []CarpoolUsageWindow {
	limitPoints := float64(len(accounts)) * carpoolUsagePointsPerAccount
	fiveHourUsage := s.carpoolPoolWindowUsagePoints(ctx, accounts, "5h")
	weeklyUsage := s.carpoolPoolWindowUsagePoints(ctx, accounts, "7d")
	return []CarpoolUsageWindow{
		newCarpoolUsageWindow("5h", fiveHourUsage.UsedPoints, limitPoints, fiveHourUsage.ResetAt),
		newCarpoolUsageWindow("7d", weeklyUsage.UsedPoints, limitPoints, weeklyUsage.ResetAt),
	}
}

func (s *CarpoolService) attachCarpoolMemberUsageStats(ctx context.Context, pool *CarpoolPool, profiles []CarpoolMemberProfile) {
	if s == nil || s.repo == nil || pool == nil || len(profiles) == 0 {
		return
	}
	userIDs := make([]int64, 0, len(profiles))
	seen := make(map[int64]struct{}, len(profiles))
	for i := range profiles {
		userID := profiles[i].Member.UserID
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	if len(userIDs) == 0 {
		return
	}
	var (
		statsByUser map[int64]CarpoolMemberUsageStats
		err         error
	)
	if lister, ok := s.repo.(carpoolMemberUsageStatsByPoolLister); ok {
		statsByUser, err = lister.ListPoolMemberUsageStatsByPoolID(ctx, pool.ID, userIDs)
	} else if pool.GroupID != nil && *pool.GroupID > 0 {
		statsByUser, err = s.repo.ListPoolMemberUsageStats(ctx, *pool.GroupID, userIDs)
	}
	if err != nil {
		var groupID any
		if pool.GroupID != nil {
			groupID = *pool.GroupID
		}
		slog.Warn("carpool_member_usage_stats_failed", "pool_id", pool.ID, "group_id", groupID, "error", err)
		return
	}
	if statsByUser == nil {
		return
	}
	for i := range profiles {
		stats := statsByUser[profiles[i].Member.UserID]
		profiles[i].TotalTokens = stats.TotalTokens
		profiles[i].TotalCostUSD = positiveCarpoolFloat(stats.TotalCostUSD)
	}
}

type carpoolPoolUsagePoints struct {
	UsedPoints float64
	ResetAt    *time.Time
	Valid      bool
}

func (s *CarpoolService) carpoolPoolWindowUsagePoints(ctx context.Context, accounts []CarpoolPoolAccount, window string) carpoolPoolUsagePoints {
	if s == nil || s.accountUsageService == nil || len(accounts) == 0 {
		return carpoolPoolUsagePoints{}
	}

	out := carpoolPoolUsagePoints{}
	for i := range accounts {
		usage, err := s.accountUsageService.GetUsage(ctx, accounts[i].AccountID)
		if err != nil {
			slog.Warn("carpool_usage_get_usage_failed", "pool_id", accounts[i].PoolID, "account_id", accounts[i].AccountID, "window", window, "error", err)
			continue
		}
		progress := usageProgress(usage, window)
		if progress == nil || progress.Utilization < 0 {
			continue
		}
		out.Valid = true
		out.UsedPoints += carpoolUsagePointsPerAccount * progress.Utilization / 100
		if resetAt := progressResetAt(progress); resetAt != nil {
			out.ResetAt = earliestCarpoolTime(out.ResetAt, resetAt)
		}
	}
	if out.UsedPoints < 0 {
		out.UsedPoints = 0
	}
	return out
}

func carpoolPerMemberLimitPoints(targetSeats, accountCount int) float64 {
	if targetSeats <= 0 || accountCount <= 0 {
		return 0
	}
	return float64(accountCount) * carpoolUsagePointsPerAccount / float64(targetSeats)
}

func carpoolMemberLimitPoints(targetSeats, accountCount int, shareRatio float64) float64 {
	if accountCount <= 0 {
		return 0
	}
	share := normalizeCarpoolQuotaShare(shareRatio, targetSeats)
	if share <= 0 {
		return 0
	}
	return float64(accountCount) * carpoolUsagePointsPerAccount * share
}

func carpoolMemberUsedPoints(rawUsed, rawLimit, pointLimit, totalRawUsed, poolUsedPoints float64, hasPoolUsage bool) float64 {
	rawUsed = positiveCarpoolFloat(rawUsed)
	if rawUsed <= 0 {
		return 0
	}
	if rawLimit > 0 && pointLimit > 0 {
		return rawUsed / rawLimit * pointLimit
	}
	if hasPoolUsage && totalRawUsed > 0 {
		return rawUsed / totalRawUsed * positiveCarpoolFloat(poolUsedPoints)
	}
	return rawUsed
}

func carpoolNormalizedMemberUsedPoints(rawUsed, displayLimit, actualPointLimit, totalRawUsed, poolUsedPoints float64, hasPoolUsage bool) float64 {
	rawUsed = positiveCarpoolFloat(rawUsed)
	displayLimit = positiveCarpoolFloat(displayLimit)
	if displayLimit <= 0 {
		displayLimit = carpoolUsagePointsPerAccount
	}
	if rawUsed <= 0 {
		return 0
	}
	if hasPoolUsage && totalRawUsed > 0 && actualPointLimit > 0 {
		actualUsedPoints := rawUsed / totalRawUsed * positiveCarpoolFloat(poolUsedPoints)
		return actualUsedPoints / actualPointLimit * displayLimit
	}
	return rawUsed
}

func newCarpoolUsageWindow(window string, used, limit float64, resetAt *time.Time) CarpoolUsageWindow {
	used = positiveCarpoolFloat(used)
	limit = positiveCarpoolFloat(limit)
	remaining := 0.0
	utilization := 0.0
	if limit > 0 {
		remaining = limit - used
		if remaining < 0 {
			remaining = 0
		}
		utilization = used / limit * 100
	}
	return CarpoolUsageWindow{
		Window:          window,
		UsedPoints:      used,
		LimitPoints:     limit,
		RemainingPoints: remaining,
		Utilization:     utilization,
		ResetAt:         resetAt,
	}
}

func carpoolMemberFiveHourResetAt(member CarpoolMember) *time.Time {
	if member.FiveHourWindowStart == nil || member.FiveHourUsedUSD <= 0 {
		return nil
	}
	resetAt := member.FiveHourWindowStart.UTC().Add(5 * time.Hour)
	return &resetAt
}

func positiveCarpoolFloat(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func containsCarpoolInt64(values []int64, needle int64) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func sameOptionalCarpoolFloat(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Abs(*a-*b) < 0.000001
}

func earliestCarpoolTime(current, candidate *time.Time) *time.Time {
	if candidate == nil {
		return current
	}
	c := candidate.UTC()
	if current == nil || c.Before(current.UTC()) {
		return &c
	}
	return current
}

func (s *CarpoolService) invalidateCarpoolMemberSubscriptionCaches(ctx context.Context, pool *CarpoolPool, userIDs []int64) {
	if pool == nil {
		return
	}
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		if pool.GroupID != nil && *pool.GroupID > 0 {
			if s.subscriptionInvalidator != nil {
				_ = s.subscriptionInvalidator.InvalidateSubscription(ctx, userID, *pool.GroupID)
			}
			continue
		}
		if group, err := s.findUserCarpoolGroup(ctx, userID, pool.Platform); err == nil && group != nil {
			s.invalidateUserCarpoolGroupCaches(ctx, userID, group.ID)
		}
	}
}

func usageProgress(usage *UsageInfo, window string) *UsageProgress {
	if usage == nil {
		return nil
	}
	switch window {
	case "5h":
		return usage.FiveHour
	case "7d":
		return usage.SevenDay
	default:
		return nil
	}
}

func progressUtilization(progress *UsageProgress) float64 {
	if progress == nil {
		return -1
	}
	return progress.Utilization
}

func progressResetAt(progress *UsageProgress) *time.Time {
	if progress == nil || progress.ResetsAt == nil {
		return nil
	}
	t := progress.ResetsAt.UTC()
	return &t
}

func sameCarpoolResetAt(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Abs(a.UTC().Sub(b.UTC()).Seconds()) <= 5*60
}

func shouldNotifyCarpoolExternalOverage(account CarpoolPoolAccount, now time.Time) bool {
	if account.ExternalOverageNotifiedAt == nil {
		return true
	}
	return now.Sub(*account.ExternalOverageNotifiedAt) >= carpoolExternalOverageNotifyWindow
}

func (s *CarpoolService) isOwnerOverCarpoolLimit(member *CarpoolMember, sub *UserSubscription, group *Group) bool {
	if member != nil && member.FiveHourLimitUSD > 0 && member.FiveHourUsedUSD >= member.FiveHourLimitUSD {
		return true
	}
	if member != nil && sub != nil && member.WeeklyLimitUSD > 0 && sub.WeeklyUsageUSD >= member.WeeklyLimitUSD {
		return true
	}
	if sub != nil && member == nil && group != nil && group.WeeklyLimitUSD != nil && *group.WeeklyLimitUSD > 0 && sub.WeeklyUsageUSD >= *group.WeeklyLimitUSD {
		return true
	}
	return false
}

func (s *CarpoolService) sendCarpoolExternalOverageEmail(ctx context.Context, pool *CarpoolPool, group *Group, member *CarpoolMember, sub *UserSubscription, fiveHourDelta, weeklyDelta float64, accountLabels []string) error {
	if s.emailService == nil || pool == nil {
		return nil
	}
	owner, err := s.userRepo.GetByID(ctx, pool.OwnerUserID)
	if err != nil {
		return fmt.Errorf("load pool owner: %w", err)
	}
	to := strings.TrimSpace(owner.Email)
	if to == "" {
		return nil
	}

	accountsText := "绑定账号"
	if len(accountLabels) > 0 {
		accountsText = strings.Join(accountLabels, "、")
	}
	fiveHourLine := ""
	if member != nil && member.FiveHourLimitUSD > 0 {
		fiveHourLine = fmt.Sprintf("<li>5小时额度：已用 %.4f / %.4f USD，本次站外新增 %.4f USD</li>", member.FiveHourUsedUSD, member.FiveHourLimitUSD, fiveHourDelta)
	}
	weeklyLine := ""
	if sub != nil && member != nil && member.WeeklyLimitUSD > 0 {
		weeklyLine = fmt.Sprintf("<li>周额度：已用 %.4f / %.4f USD，本次站外新增 %.4f USD</li>", sub.WeeklyUsageUSD, member.WeeklyLimitUSD, weeklyDelta)
	} else if sub != nil && group != nil && group.WeeklyLimitUSD != nil && *group.WeeklyLimitUSD > 0 {
		weeklyLine = fmt.Sprintf("<li>周额度：已用 %.4f / %.4f USD，本次站外新增 %.4f USD</li>", sub.WeeklyUsageUSD, *group.WeeklyLimitUSD, weeklyDelta)
	}
	subject := "拼车池站外使用额度提醒"
	body := fmt.Sprintf(`
<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#f7f7f8;padding:24px;color:#202123;">
  <div style="max-width:560px;margin:0 auto;background:#ffffff;border:1px solid #d9d9e3;border-radius:14px;padding:24px;">
    <h2 style="margin:0 0 12px;font-size:20px;">拼车池额度提醒</h2>
    <p style="line-height:1.7;margin:0 0 12px;">你的拼车池 <strong>%s</strong> 检测到站外使用：%s。</p>
    <p style="line-height:1.7;margin:0 0 12px;">系统已按规则把站外消耗计入池主额度，号池不会因此自动暂停。</p>
    <ul style="line-height:1.8;margin:0 0 12px;padding-left:20px;">%s%s</ul>
    <p style="line-height:1.7;margin:0;color:#6e6e80;">建议检查池主是否直接登录官方客户端或网页使用了该账号。</p>
  </div>
</div>`, html.EscapeString(pool.Name), html.EscapeString(accountsText), fiveHourLine, weeklyLine)
	return s.emailService.SendEmail(ctx, to, subject, body)
}

func buildCarpoolQuotaSnapshot(accounts []*Account, seats int) CarpoolQuotaSnapshot {
	var total5h float64
	var totalWeekly float64
	for _, account := range accounts {
		if account == nil {
			continue
		}
		total5h += account.GetWindowCostLimit()
		totalWeekly += account.GetQuotaWeeklyLimit()
	}
	perMember5h := 0.0
	perMemberWeekly := 0.0
	if seats > 0 {
		perMember5h = total5h / float64(seats)
		perMemberWeekly = totalWeekly / float64(seats)
	}
	now := time.Now().UTC()
	return CarpoolQuotaSnapshot{
		TotalFiveHourLimitUSD:     total5h,
		TotalWeeklyLimitUSD:       totalWeekly,
		PerMemberFiveHourLimitUSD: perMember5h,
		PerMemberWeeklyLimitUSD:   perMemberWeekly,
		SnapshotAt:                &now,
	}
}

func defaultCarpoolQuotaShare(targetSeats int) float64 {
	if targetSeats <= 0 {
		return 0
	}
	return 1 / float64(targetSeats)
}

func normalizeCarpoolQuotaShare(shareRatio float64, targetSeats int) float64 {
	if shareRatio > 0 && !math.IsNaN(shareRatio) && !math.IsInf(shareRatio, 0) {
		return shareRatio
	}
	return defaultCarpoolQuotaShare(targetSeats)
}

func carpoolLimitFromShare(totalLimit, fallbackPerMemberLimit, shareRatio float64) float64 {
	shareRatio = positiveCarpoolFloat(shareRatio)
	if shareRatio <= 0 {
		return 0
	}
	if totalLimit > 0 {
		return totalLimit * shareRatio
	}
	return positiveCarpoolFloat(fallbackPerMemberLimit)
}

func carpoolEffectiveTotalLimit(totalLimit, fallbackPerMemberLimit float64, targetSeats int) float64 {
	if totalLimit > 0 {
		return totalLimit
	}
	if fallbackPerMemberLimit > 0 && targetSeats > 0 {
		return fallbackPerMemberLimit * float64(targetSeats)
	}
	return 0
}

func (s *CarpoolService) carpoolGroupWeeklyLimit(ctx context.Context, poolID int64, fallback float64) float64 {
	out := positiveCarpoolFloat(fallback)
	if s == nil || s.repo == nil || poolID <= 0 {
		return out
	}
	members, err := s.repo.ListPoolMembers(ctx, poolID)
	if err != nil {
		slog.Warn("carpool_group_weekly_limit_members_failed", "pool_id", poolID, "error", err)
		return out
	}
	for i := range members {
		if members[i].Status != CarpoolMemberStatusActive {
			continue
		}
		if members[i].WeeklyLimitUSD > out {
			out = members[i].WeeklyLimitUSD
		}
	}
	return out
}

func positiveFloat64Ptr(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	v := value
	return &v
}

func generateCarpoolInviteCode() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("cp-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

func uniquePositiveCarpoolIDs(values []int64) []int64 {
	out := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
