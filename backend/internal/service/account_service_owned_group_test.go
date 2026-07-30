package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/pkg/pagination"
)

type ownedAccountGroupRepoStub struct {
	groupRepoNoop
	groups map[int64]*Group
}

func (s *ownedAccountGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	group := s.groups[id]
	if group == nil {
		return nil, ErrGroupNotFound
	}
	cp := *group
	return &cp, nil
}

type ownedAccountUserSubRepoStub struct {
	active map[int64]*UserSubscription
}

func (s *ownedAccountUserSubRepoStub) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	sub := s.active[groupID]
	if sub == nil || sub.UserID != userID {
		return nil, ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

type ownedAccountUserRepoStub struct {
	user *User
	err  error
}

func (s *ownedAccountUserRepoStub) GetByID(_ context.Context, _ int64) (*User, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.user == nil {
		return nil, ErrUserNotFound
	}
	cp := *s.user
	return &cp, nil
}

type ownedPublicShareGroupRepoStub struct {
	groupRepoNoop
	groups []Group
}

func (s *ownedPublicShareGroupRepoStub) ListActiveByPlatform(_ context.Context, platform string) ([]Group, error) {
	out := make([]Group, 0, len(s.groups))
	for _, group := range s.groups {
		if group.Platform == platform && group.IsActive() {
			out = append(out, group)
		}
	}
	return out, nil
}

type ownedPublicSharePolicyRepoStub struct {
	policy *AccountSharePolicy
	err    error
}

func (s *ownedPublicSharePolicyRepoStub) ListAccountSharePolicies(context.Context, pagination.PaginationParams, AccountSharePolicyFilters) ([]AccountSharePolicy, *pagination.PaginationResult, error) {
	panic("unexpected ListAccountSharePolicies call")
}

func (s *ownedPublicSharePolicyRepoStub) GetAccountSharePolicyByID(context.Context, int64) (*AccountSharePolicy, error) {
	panic("unexpected GetAccountSharePolicyByID call")
}

func (s *ownedPublicSharePolicyRepoStub) ResolveEnabledAccountSharePolicy(context.Context, int64, *int64, string, *int64) (*AccountSharePolicy, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.policy == nil {
		return nil, nil
	}
	cp := *s.policy
	return &cp, nil
}

func (s *ownedPublicSharePolicyRepoStub) CreateAccountSharePolicy(context.Context, CreateAccountSharePolicyInput) (*AccountSharePolicy, error) {
	panic("unexpected CreateAccountSharePolicy call")
}

func (s *ownedPublicSharePolicyRepoStub) UpdateAccountSharePolicy(context.Context, int64, UpdateAccountSharePolicyInput) (*AccountSharePolicy, error) {
	panic("unexpected UpdateAccountSharePolicy call")
}

func (s *ownedPublicSharePolicyRepoStub) DeleteAccountSharePolicy(context.Context, int64) error {
	panic("unexpected DeleteAccountSharePolicy call")
}

type ownedPrivateGroupProvisionerStub struct {
	group          *Group
	err            error
	provisionErr   error
	provisionCalls int
}

func (s *ownedPrivateGroupProvisionerStub) ProvisionUserPrivateGroups(context.Context, int64) error {
	s.provisionCalls++
	return s.provisionErr
}

func (s *ownedPrivateGroupProvisionerStub) GetActiveUserPrivateGroup(context.Context, int64, string) (*Group, error) {
	if s.err != nil && s.provisionCalls == 0 {
		return nil, s.err
	}
	if s.group == nil {
		return nil, ErrGroupNotFound
	}
	cp := *s.group
	return &cp, nil
}

type ownedAccountDuplicateRepoStub struct {
	createdAccounts     []*Account
	updatedAccounts     []*Account
	bulkUpdateCalls     int
	bulkUpdateIDs       []int64
	bulkUpdatePayload   AccountBulkUpdate
	boundAccountIDs     []int64
	boundGroupIDs       map[int64][]int64
	getByIDAccounts     map[int64]*Account
	getByIDsAccounts    map[int64]*Account
	listOwnedByPlatform map[string][]Account
}

func (s *ownedAccountDuplicateRepoStub) Create(_ context.Context, account *Account) error {
	cp := *account
	s.createdAccounts = append(s.createdAccounts, &cp)
	return nil
}

func (s *ownedAccountDuplicateRepoStub) Update(_ context.Context, account *Account) error {
	cp := *account
	s.updatedAccounts = append(s.updatedAccounts, &cp)
	if s.getByIDAccounts != nil {
		stored := cp
		s.getByIDAccounts[account.ID] = &stored
	}
	if s.getByIDsAccounts != nil {
		stored := cp
		s.getByIDsAccounts[account.ID] = &stored
	}
	return nil
}

func (s *ownedAccountDuplicateRepoStub) BulkUpdate(_ context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	s.bulkUpdateCalls++
	s.bulkUpdateIDs = append([]int64(nil), ids...)
	s.bulkUpdatePayload = updates
	return int64(len(ids)), nil
}

func (s *ownedAccountDuplicateRepoStub) BindGroups(_ context.Context, accountID int64, groupIDs []int64) error {
	s.boundAccountIDs = append(s.boundAccountIDs, accountID)
	if s.boundGroupIDs == nil {
		s.boundGroupIDs = map[int64][]int64{}
	}
	s.boundGroupIDs[accountID] = append([]int64(nil), groupIDs...)
	return nil
}

func (s *ownedAccountDuplicateRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	account := s.getByIDAccounts[id]
	if account == nil {
		return nil, ErrAccountNotFound
	}
	cp := *account
	return &cp, nil
}

func (s *ownedAccountDuplicateRepoStub) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		account := s.getByIDsAccounts[id]
		if account == nil {
			continue
		}
		cp := *account
		out = append(out, &cp)
	}
	return out, nil
}

func (s *ownedAccountDuplicateRepoStub) ListOwnedWithFilters(_ context.Context, ownerUserID int64, params pagination.PaginationParams, platform, accountType, status, search string, groupID, proxyID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	rows := s.listOwnedByPlatform[platform]
	filtered := make([]Account, 0, len(rows))
	for _, row := range rows {
		if row.OwnerUserID == nil || *row.OwnerUserID != ownerUserID {
			continue
		}
		if accountType != "" && row.Type != accountType {
			continue
		}
		filtered = append(filtered, row)
	}
	offset := params.Offset()
	limit := params.Limit()
	if offset >= len(filtered) {
		return nil, &pagination.PaginationResult{Total: int64(len(filtered))}, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], &pagination.PaginationResult{Total: int64(len(filtered))}, nil
}

func (s *ownedAccountDuplicateRepoStub) ListAllWithFilters(_ context.Context, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, error) {
	var out []Account
	for _, accounts := range s.listOwnedByPlatform {
		for _, account := range accounts {
			if platform != "" && account.Platform != platform {
				continue
			}
			if accountType != "" && account.Type != accountType {
				continue
			}
			if status != "" && account.Status != status {
				continue
			}
			out = append(out, account)
		}
	}
	return out, nil
}

func (s *ownedAccountDuplicateRepoStub) ListShadowsByParent(context.Context, int64) ([]*Account, error) {
	return nil, nil
}

func (s *ownedAccountDuplicateRepoStub) RevertProxyFallback(context.Context, int64) error {
	return nil
}

func (s *ownedAccountDuplicateRepoStub) ExistsByID(context.Context, int64) (bool, error) {
	panic("unexpected ExistsByID call")
}

func (s *ownedAccountDuplicateRepoStub) GetByCRSAccountID(context.Context, string) (*Account, error) {
	panic("unexpected GetByCRSAccountID call")
}

func (s *ownedAccountDuplicateRepoStub) FindByExtraField(context.Context, string, any) ([]Account, error) {
	panic("unexpected FindByExtraField call")
}

func (s *ownedAccountDuplicateRepoStub) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	panic("unexpected ListCRSAccountIDs call")
}

func (s *ownedAccountDuplicateRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *ownedAccountDuplicateRepoStub) List(context.Context, pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *ownedAccountDuplicateRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *ownedAccountDuplicateRepoStub) ListByGroup(context.Context, int64) ([]Account, error) {
	panic("unexpected ListByGroup call")
}

func (s *ownedAccountDuplicateRepoStub) ListActive(context.Context) ([]Account, error) {
	panic("unexpected ListActive call")
}

func (s *ownedAccountDuplicateRepoStub) ListByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected ListByPlatform call")
}

func (s *ownedAccountDuplicateRepoStub) UpdateLastUsed(context.Context, int64) error {
	panic("unexpected UpdateLastUsed call")
}

func (s *ownedAccountDuplicateRepoStub) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	panic("unexpected BatchUpdateLastUsed call")
}

func (s *ownedAccountDuplicateRepoStub) SetError(context.Context, int64, string) error {
	panic("unexpected SetError call")
}

func (s *ownedAccountDuplicateRepoStub) ClearError(context.Context, int64) error {
	panic("unexpected ClearError call")
}

func (s *ownedAccountDuplicateRepoStub) SetSchedulable(context.Context, int64, bool) error {
	panic("unexpected SetSchedulable call")
}

func (s *ownedAccountDuplicateRepoStub) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	panic("unexpected AutoPauseExpiredAccounts call")
}

func (s *ownedAccountDuplicateRepoStub) ListSchedulable(context.Context) ([]Account, error) {
	panic("unexpected ListSchedulable call")
}

func (s *ownedAccountDuplicateRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupID call")
}

func (s *ownedAccountDuplicateRepoStub) ListSchedulableByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatform call")
}

func (s *ownedAccountDuplicateRepoStub) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatform call")
}

func (s *ownedAccountDuplicateRepoStub) ListSchedulableByPlatforms(context.Context, []string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatforms call")
}

func (s *ownedAccountDuplicateRepoStub) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatforms call")
}

func (s *ownedAccountDuplicateRepoStub) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatform call")
}

func (s *ownedAccountDuplicateRepoStub) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatforms call")
}

func (s *ownedAccountDuplicateRepoStub) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]Account, error) {
	panic("unexpected ListModelAvailabilityCandidates call")
}

func (s *ownedAccountDuplicateRepoStub) SetRateLimited(context.Context, int64, time.Time) error {
	panic("unexpected SetRateLimited call")
}

func (s *ownedAccountDuplicateRepoStub) SetModelRateLimit(context.Context, int64, string, time.Time, ...string) error {
	panic("unexpected SetModelRateLimit call")
}

func (s *ownedAccountDuplicateRepoStub) SetOverloaded(context.Context, int64, time.Time) error {
	panic("unexpected SetOverloaded call")
}

func (s *ownedAccountDuplicateRepoStub) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	panic("unexpected SetTempUnschedulable call")
}

func (s *ownedAccountDuplicateRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	panic("unexpected ClearTempUnschedulable call")
}

func (s *ownedAccountDuplicateRepoStub) ClearRateLimit(context.Context, int64) error {
	panic("unexpected ClearRateLimit call")
}

func (s *ownedAccountDuplicateRepoStub) ClearAntigravityQuotaScopes(context.Context, int64) error {
	panic("unexpected ClearAntigravityQuotaScopes call")
}

func (s *ownedAccountDuplicateRepoStub) ClearModelRateLimits(context.Context, int64) error {
	panic("unexpected ClearModelRateLimits call")
}

func (s *ownedAccountDuplicateRepoStub) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	panic("unexpected UpdateSessionWindow call")
}

func (s *ownedAccountDuplicateRepoStub) UpdateSessionWindowEnd(context.Context, int64, time.Time) error {
	return nil
}

func (s *ownedAccountDuplicateRepoStub) UpdateExtra(context.Context, int64, map[string]any) error {
	panic("unexpected UpdateExtra call")
}

func (s *ownedAccountDuplicateRepoStub) IncrementQuotaUsed(context.Context, int64, float64) error {
	panic("unexpected IncrementQuotaUsed call")
}

func (s *ownedAccountDuplicateRepoStub) ResetQuotaUsed(context.Context, int64) error {
	panic("unexpected ResetQuotaUsed call")
}

func TestAccountServiceValidateOwnedAccountGroupBinding(t *testing.T) {
	t.Run("allows active standard group and deduplicates ids", func(t *testing.T) {
		svc := newOwnedAccountGroupValidationService(
			&User{ID: 101},
			map[int64]*Group{
				10: {ID: 10, Platform: PlatformOpenAI, Status: StatusActive},
			},
			nil,
		)

		groupIDs, err := svc.validateOwnedAccountGroupBinding(context.Background(), 101, PlatformOpenAI, AccountTypeOAuth, []int64{10, 10})

		require.NoError(t, err)
		require.Equal(t, []int64{10}, groupIDs)
	})

	t.Run("rejects platform mismatch", func(t *testing.T) {
		svc := newOwnedAccountGroupValidationService(
			&User{ID: 101},
			map[int64]*Group{
				10: {ID: 10, Platform: PlatformAnthropic, Status: StatusActive},
			},
			nil,
		)

		_, err := svc.validateOwnedAccountGroupBinding(context.Background(), 101, PlatformOpenAI, AccountTypeOAuth, []int64{10})

		require.ErrorIs(t, err, ErrOwnedAccountGroupPlatformMismatch)
	})

	t.Run("rejects exclusive group without user permission", func(t *testing.T) {
		svc := newOwnedAccountGroupValidationService(
			&User{ID: 101, AllowedGroups: []int64{20}},
			map[int64]*Group{
				10: {ID: 10, Platform: PlatformOpenAI, Status: StatusActive, IsExclusive: true},
			},
			nil,
		)

		_, err := svc.validateOwnedAccountGroupBinding(context.Background(), 101, PlatformOpenAI, AccountTypeOAuth, []int64{10})

		require.ErrorIs(t, err, ErrGroupNotAllowed)
	})

	t.Run("requires active subscription for subscription group", func(t *testing.T) {
		svc := newOwnedAccountGroupValidationService(
			&User{ID: 101},
			map[int64]*Group{
				10: {ID: 10, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
			},
			nil,
		)

		_, err := svc.validateOwnedAccountGroupBinding(context.Background(), 101, PlatformOpenAI, AccountTypeOAuth, []int64{10})

		require.ErrorIs(t, err, ErrGroupNotAllowed)
	})

	t.Run("allows subscription group with active subscription", func(t *testing.T) {
		svc := newOwnedAccountGroupValidationService(
			&User{ID: 101},
			map[int64]*Group{
				10: {ID: 10, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
			},
			map[int64]*UserSubscription{
				10: {UserID: 101, GroupID: 10},
			},
		)

		groupIDs, err := svc.validateOwnedAccountGroupBinding(context.Background(), 101, PlatformOpenAI, AccountTypeOAuth, []int64{10})

		require.NoError(t, err)
		require.Equal(t, []int64{10}, groupIDs)
	})
}

func newOwnedAccountGroupValidationService(user *User, groups map[int64]*Group, activeSubs map[int64]*UserSubscription) *AccountService {
	return &AccountService{
		groupRepo: &ownedAccountGroupRepoStub{
			groups: groups,
		},
		userRepo: &ownedAccountUserRepoStub{
			user: user,
		},
		userSubRepo: &ownedAccountUserSubRepoStub{
			active: activeSubs,
		},
	}
}

func TestAccountServiceResolveOwnedPublicShareGroup(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 10, Name: "FREE共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelFree},
				{ID: 11, Name: "PLUS共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelPlus},
				{ID: 12, Name: "TEAM共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelTeam},
			},
		},
	}

	group, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformOpenAI, AccountLevel: AccountLevelPlus})

	require.NoError(t, err)
	require.Equal(t, int64(11), group.ID)
}

func TestAccountServiceResolveOwnedPublicShareGroupRequiresExplicitMarkerForNonOpenAI(t *testing.T) {
	platforms := []string{PlatformAnthropic, PlatformGemini, PlatformAntigravity, PlatformGrok, PlatformKiro}
	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			svc := &AccountService{
				groupRepo: &ownedPublicShareGroupRepoStub{
					groups: []Group{
						{ID: 20, Name: "regular public group", Platform: platform, Status: StatusActive, Scope: GroupScopePublic},
						{ID: 21, Name: "shared pool", Platform: platform, Status: StatusActive, Scope: GroupScopePublic, IsSharedPool: true},
					},
				},
			}

			group, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: platform})

			require.NoError(t, err)
			require.Equal(t, int64(21), group.ID)
		})
	}
}

func TestAccountServiceResolveOwnedPublicShareGroupRejectsUnsupportedCustomPool(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 30, Name: "custom public group", Platform: PlatformCustom, Status: StatusActive, Scope: GroupScopePublic, IsSharedPool: true},
			},
		},
	}

	group, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformCustom})

	require.ErrorIs(t, err, ErrOwnedAccountPublicPoolUnavailable)
	require.Nil(t, group)
}

func TestAccountServiceResolveOwnedPublicShareGroupAllowsHigherLevelFallbackToLowerPool(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 10, Name: "PLUS共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelPlus},
			},
		},
	}

	group, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformOpenAI, AccountLevel: AccountLevelPro})

	require.NoError(t, err)
	require.Equal(t, int64(10), group.ID)
}

func TestAccountServiceResolveOwnedPublicShareGroupDoesNotTreatTeamPoolAsPlus(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 12, Name: "TEAM共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelTeam},
			},
		},
	}

	group, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformOpenAI, AccountLevel: AccountLevelPlus})

	require.Error(t, err)
	require.Nil(t, group)
}

func TestAccountServiceResolveOwnedPublicShareGroupMatchesTeamPool(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 12, Name: "TEAM共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelTeam},
			},
		},
	}

	group, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformOpenAI, AccountLevel: AccountLevelTeam})

	require.NoError(t, err)
	require.Equal(t, int64(12), group.ID)
}

func TestAccountServiceResolveOwnedPublicShareGroupMatchesK12Pool(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 14, Name: "K12共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelK12},
			},
		},
	}

	group, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformOpenAI, AccountLevel: AccountLevelK12})

	require.NoError(t, err)
	require.Equal(t, int64(14), group.ID)
}

func TestAccountServiceResolveOwnedPublicShareGroupRejectsHigherPoolForLowerLevel(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 13, Name: "PRO共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelPro},
			},
		},
	}

	_, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformOpenAI, AccountLevel: AccountLevelPlus})

	require.ErrorIs(t, err, ErrOwnedAccountPublicPoolUnavailable)
}

func TestAccountServiceResolveOwnedPublicShareGroupTreatsUnknownOpenAILevelAsFree(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 10, Name: "FREE共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelFree},
			},
		},
	}

	group, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformOpenAI, AccountLevel: AccountLevelUnknown})

	require.NoError(t, err)
	require.Equal(t, int64(10), group.ID)
}

func TestAccountServiceResolveOwnedPublicShareGroupIgnoresOpenAIUnclassifiedSystemPool(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 59, Name: "Codex【兜底】", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic},
				{ID: 1197, Name: "FREE共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelFree},
			},
		},
	}

	group, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformOpenAI, AccountLevel: AccountLevelUnknown})

	require.NoError(t, err)
	require.Equal(t, int64(1197), group.ID)
}

func TestAccountServiceResolveOwnedPublicShareGroupRejectsOpenAIWhenOnlyUnclassifiedSystemPoolExists(t *testing.T) {
	svc := &AccountService{
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 59, Name: "Codex【兜底】", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic},
			},
		},
	}

	_, err := svc.resolveOwnedPublicShareGroup(context.Background(), &Account{Platform: PlatformOpenAI, AccountLevel: AccountLevelFree})

	require.ErrorIs(t, err, ErrOwnedAccountPublicPoolUnavailable)
}

func TestShouldRepairSuspectedOpenAIFreeAccount(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	account := &Account{
		Platform:     PlatformOpenAI,
		Type:         AccountTypeOAuth,
		AccountLevel: AccountLevelPlus,
		Extra: map[string]any{
			"quota_weekly_limit":    50.0,
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
		},
	}

	require.True(t, ShouldRepairSuspectedOpenAIFreeAccount(account, 60, now))
	require.False(t, ShouldRepairSuspectedOpenAIFreeAccount(account, 40, now))

	account.Extra["codex_7d_used_percent"] = 99.9
	require.False(t, ShouldRepairSuspectedOpenAIFreeAccount(account, 60, now))
}

func TestAccountServiceAutoRepairSuspectedOpenAIFreeAccountSuspendsPublicShareAndRebindsFreePool(t *testing.T) {
	ownerID := int64(101)
	accountID := int64(202)
	now := time.Now().UTC()
	repo := &ownedAccountDuplicateRepoStub{
		getByIDAccounts: map[int64]*Account{
			accountID: {
				ID:           accountID,
				Platform:     PlatformOpenAI,
				Type:         AccountTypeOAuth,
				AccountLevel: AccountLevelPlus,
				OwnerUserID:  &ownerID,
				ShareMode:    AccountShareModePublic,
				ShareStatus:  AccountShareStatusApproved,
				Status:       StatusActive,
				Schedulable:  true,
				Concurrency:  OpenAIPlusDefaultConcurrency,
				GroupIDs:     []int64{99, 11},
				Extra: map[string]any{
					"quota_weekly_limit":    50.0,
					"codex_7d_used_percent": 100.0,
					"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
				},
			},
		},
	}
	svc := &AccountService{
		accountRepo: repo,
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 10, Name: "FREE共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelFree},
				{ID: 11, Name: "PLUS共享号池", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelPlus},
			},
		},
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
	}

	account, repaired, err := svc.AutoRepairSuspectedOpenAIFreeAccount(context.Background(), accountID, 60, "quota proof")

	require.NoError(t, err)
	require.True(t, repaired)
	require.Equal(t, AccountLevelFree, account.AccountLevel)
	require.Equal(t, AccountShareStatusSuspended, account.ShareStatus)
	require.Equal(t, "quota proof", account.ErrorMessage)
	require.Equal(t, []int64{99, 10}, repo.boundGroupIDs[accountID])
	require.Len(t, repo.updatedAccounts, 1)
	require.Equal(t, AccountLevelFree, repo.updatedAccounts[0].AccountLevel)
}

func TestAccountServiceValidateOwnedPublicSharePolicyRequiresEnabledPositivePolicy(t *testing.T) {
	account := &Account{ID: 20, Platform: PlatformOpenAI}
	group := &Group{ID: 11, Platform: PlatformOpenAI}

	t.Run("allows positive policy", func(t *testing.T) {
		svc := &AccountService{
			accountSharePolicyRepo: &ownedPublicSharePolicyRepoStub{
				policy: &AccountSharePolicy{ID: 1, OwnerShareRatio: 0.7, Enabled: true},
			},
		}

		err := svc.validateOwnedPublicSharePolicy(context.Background(), account, group)

		require.NoError(t, err)
	})

	t.Run("rejects missing policy", func(t *testing.T) {
		svc := &AccountService{accountSharePolicyRepo: &ownedPublicSharePolicyRepoStub{}}

		err := svc.validateOwnedPublicSharePolicy(context.Background(), account, group)

		require.ErrorIs(t, err, ErrOwnedAccountPublicPolicyUnavailable)
	})

	t.Run("rejects zero owner share ratio", func(t *testing.T) {
		svc := &AccountService{
			accountSharePolicyRepo: &ownedPublicSharePolicyRepoStub{
				policy: &AccountSharePolicy{ID: 1, OwnerShareRatio: 0, Enabled: true},
			},
		}

		err := svc.validateOwnedPublicSharePolicy(context.Background(), account, group)

		require.ErrorIs(t, err, ErrOwnedAccountPublicPolicyUnavailable)
	})
}

func TestAccountServiceInitialOwnedAccountGroupIDsKeepsPendingPublicAccountPrivate(t *testing.T) {
	svc := &AccountService{
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
	}

	groupIDs, err := svc.initialOwnedAccountGroupIDs(context.Background(), 101, &Account{
		Platform:     PlatformOpenAI,
		Type:         AccountTypeOAuth,
		ShareMode:    AccountShareModePublic,
		AccountLevel: AccountLevelPlus,
	}, []int64{99})

	require.NoError(t, err)
	require.Equal(t, []int64{99}, groupIDs)
}

func TestAccountServiceCreateOwnedAllowsPublicShareWithOwnedProxy(t *testing.T) {
	ownerID := int64(101)
	proxyID := int64(7)
	repo := &ownedAccountDuplicateRepoStub{}
	svc := &AccountService{
		accountRepo: repo,
		proxyRepo: &ownedOAuthProxyRepoStub{
			proxy: &Proxy{ID: proxyID, Status: StatusActive},
		},
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
	}

	account, err := svc.CreateOwned(context.Background(), ownerID, CreateAccountRequest{
		Name:        "proxied-public-account",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token"},
		ProxyID:     &proxyID,
		ShareMode:   AccountShareModePublic,
		Concurrency: 99,
		LoadFactor:  intPtr(8),
		Priority:    9,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, AccountShareModePublic, account.ShareMode)
	require.Equal(t, AccountShareStatusPending, account.ShareStatus)
	require.Equal(t, &proxyID, account.ProxyID)
	require.Equal(t, ownedPersonalDefaultConcurrency, account.Concurrency)
	require.Nil(t, account.LoadFactor)
	require.Equal(t, ownedPersonalDefaultPriority, account.Priority)
	require.Equal(t, []int64{99}, account.GroupIDs)
	require.Len(t, repo.createdAccounts, 1)
}

func TestAccountServiceInitialOwnedAccountGroupIDsIgnoresRequestedGroupsForPrivateMode(t *testing.T) {
	svc := &AccountService{
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
	}

	groupIDs, err := svc.initialOwnedAccountGroupIDs(context.Background(), 101, &Account{
		Platform:  PlatformOpenAI,
		Type:      AccountTypeOAuth,
		ShareMode: AccountShareModePrivate,
	}, []int64{11})

	require.NoError(t, err)
	require.Equal(t, []int64{99}, groupIDs)
}

func TestAccountServiceManagedGroupIDsSwitchesApprovedPublicAccountBackToPrivateGroup(t *testing.T) {
	ownerID := int64(101)
	svc := &AccountService{
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 18, Name: "Plus Shared Pool", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelPlus},
			},
		},
	}
	account := &Account{
		ID:           20,
		Platform:     PlatformOpenAI,
		Type:         AccountTypeOAuth,
		OwnerUserID:  &ownerID,
		ShareMode:    AccountShareModePublic,
		ShareStatus:  AccountShareStatusApproved,
		AccountLevel: AccountLevelPlus,
	}

	groupIDs, err := svc.managedOwnedAccountGroupIDsForShareMode(context.Background(), ownerID, account, AccountShareModePrivate)

	require.NoError(t, err)
	require.Equal(t, []int64{99}, groupIDs)
}

func TestAccountServiceGetPrivateGroupForOwnedAccountProvisionsMissingPrivateGroup(t *testing.T) {
	provisioner := &ownedPrivateGroupProvisionerStub{
		group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		err:   ErrGroupNotFound,
	}
	svc := &AccountService{privateGroupProvisioner: provisioner}

	group, err := svc.getPrivateGroupForOwnedAccount(context.Background(), 101, PlatformOpenAI)

	require.NoError(t, err)
	require.Equal(t, int64(99), group.ID)
	require.Equal(t, 1, provisioner.provisionCalls)
}

func TestAccountServiceCreateOwnedRejectsDuplicateOpenAIIdentity(t *testing.T) {
	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{
		listOwnedByPlatform: map[string][]Account{
			PlatformOpenAI: {
				{
					ID:          1,
					Platform:    PlatformOpenAI,
					Type:        AccountTypeOAuth,
					OwnerUserID: &ownerID,
					Credentials: map[string]any{"chatgpt_account_id": "acct-1"},
				},
			},
		},
	}
	svc := &AccountService{
		accountRepo: repo,
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
	}

	account, err := svc.CreateOwned(context.Background(), ownerID, CreateAccountRequest{
		Name:        "duplicate",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token", "chatgpt_account_id": "acct-1"},
		Concurrency: 1,
		Priority:    1,
	})

	require.Nil(t, account)
	require.ErrorIs(t, err, ErrOwnedAccountAlreadyExists)
	require.Empty(t, repo.createdAccounts)
}

func TestAccountServiceCreateOwnedIgnoresManualAccountLevelAndDetectsCredentials(t *testing.T) {
	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{}
	svc := &AccountService{
		accountRepo: repo,
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
	}

	account, err := svc.CreateOwned(context.Background(), ownerID, CreateAccountRequest{
		Name:         "manual-level",
		Platform:     PlatformOpenAI,
		Type:         AccountTypeOAuth,
		AccountLevel: AccountLevelTeam,
		Credentials: map[string]any{
			"access_token": "token",
			"plan_type":    "pro",
		},
		Concurrency: 1,
		Priority:    1,
	})

	require.NoError(t, err)
	require.Equal(t, AccountLevelPro, account.AccountLevel)
	require.Len(t, repo.createdAccounts, 1)
	require.Equal(t, AccountLevelPro, repo.createdAccounts[0].AccountLevel)
}

func TestValidateOwnedAccountSourceAllowsOAuthMetadataURLs(t *testing.T) {
	err := validateOwnedAccountSource(AccountTypeOAuth, map[string]any{
		"access_token": "oauth-access-token",
		"scope":        "openid https://www.googleapis.com/auth/cloud-platform",
	}, map[string]any{
		"issuer":     "https://auth.openai.com",
		"avatar_url": "https://cdn.example.com/avatar.png",
	})

	require.NoError(t, err)
}

func TestValidateOwnedAccountSourceRejectsCustomEndpointURL(t *testing.T) {
	err := validateOwnedAccountSource(AccountTypeOAuth, map[string]any{
		"access_token": "oauth-access-token",
	}, map[string]any{
		"custom_base_url": "https://evil.example.com",
	})

	require.ErrorIs(t, err, ErrOwnedAccountCredentialsNotAllowed)
}

func TestValidateOwnedAccountSourceAllowsAPIKeyAccountAndRejectsOAuthBaseURL(t *testing.T) {
	err := validateOwnedAccountSource(AccountTypeAPIKey, map[string]any{
		"api_key":  "sk-test",
		"base_url": "https://third-party.example.com",
	}, nil)

	require.NoError(t, err)

	err = validateOwnedAccountSource(AccountTypeOAuth, map[string]any{
		"access_token": "oauth-access-token",
		"base_url":     "https://third-party.example.com",
	}, nil)

	require.ErrorIs(t, err, ErrOwnedAccountCredentialsNotAllowed)
}

func TestValidateOwnedAccountSourceAllowsUserCredentialAccountTypes(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		credentials map[string]any
	}{
		{
			name:        "setup token",
			accountType: AccountTypeSetupToken,
			credentials: map[string]any{"access_token": "setup-token"},
		},
		{
			name:        "bedrock sigv4",
			accountType: AccountTypeBedrock,
			credentials: map[string]any{
				"auth_mode":             "sigv4",
				"aws_access_key_id":     "AKIAEXAMPLE",
				"aws_secret_access_key": "secret",
			},
		},
		{
			name:        "bedrock api key",
			accountType: AccountTypeBedrock,
			credentials: map[string]any{"auth_mode": "api_key", "api_key": "bedrock-key"},
		},
		{
			name:        "vertex service account",
			accountType: AccountTypeServiceAccount,
			credentials: map[string]any{"service_account_json": `{"type":"service_account"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, validateOwnedAccountSource(tt.accountType, tt.credentials, nil))
		})
	}
}

func TestAccountServiceCreateOwnedCredentialAccountIsPrivateAndKeepsConcurrency(t *testing.T) {
	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{}
	svc := &AccountService{
		accountRepo: repo,
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
	}

	account, err := svc.CreateOwned(context.Background(), ownerID, CreateAccountRequest{
		Name:        "private-api-key",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
		ShareMode:   AccountShareModePublic,
		Concurrency: 4,
		LoadFactor:  intPtr(3),
		Priority:    2,
	})

	require.NoError(t, err)
	require.Equal(t, AccountShareModePrivate, account.ShareMode)
	require.Equal(t, AccountShareStatusApproved, account.ShareStatus)
	require.Equal(t, 4, account.Concurrency)
	require.NotNil(t, account.LoadFactor)
	require.Equal(t, 3, *account.LoadFactor)
	require.Equal(t, []int64{99}, account.GroupIDs)
}

func TestAccountServiceCreateAllowsAdminOpenAIAPIKeyBaseURL(t *testing.T) {
	repo := &ownedAccountDuplicateRepoStub{}
	svc := &AccountService{accountRepo: repo}

	account, err := svc.Create(context.Background(), CreateAccountRequest{
		Name:         "admin-openai-proxy",
		Platform:     PlatformOpenAI,
		Type:         AccountTypeAPIKey,
		Credentials:  map[string]any{"api_key": "sk-test", "base_url": "https://third-party.example.com"},
		ShareMode:    AccountShareModePrivate,
		Concurrency:  1,
		LoadFactor:   intPtr(1),
		Priority:     1,
		AccountLevel: AccountLevelFree,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Len(t, repo.createdAccounts, 1)
	require.Equal(t, "https://third-party.example.com", repo.createdAccounts[0].Credentials["base_url"])
	require.Equal(t, "sk-test", repo.createdAccounts[0].Credentials["api_key"])
}

func TestAccountServiceUpdateOwnedRejectsDuplicateAnthropicIdentity(t *testing.T) {
	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{
		getByIDAccounts: map[int64]*Account{
			2: {
				ID:          2,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				OwnerUserID: &ownerID,
				Credentials: map[string]any{"access_token": "token"},
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
			},
		},
		listOwnedByPlatform: map[string][]Account{
			PlatformAnthropic: {
				{
					ID:          1,
					Platform:    PlatformAnthropic,
					Type:        AccountTypeOAuth,
					OwnerUserID: &ownerID,
					Credentials: map[string]any{"access_token": "token", "org_uuid": "org-a", "account_uuid": "acc-a"},
				},
				{
					ID:          2,
					Platform:    PlatformAnthropic,
					Type:        AccountTypeOAuth,
					OwnerUserID: &ownerID,
				},
			},
		},
	}
	svc := &AccountService{accountRepo: repo}
	credentials := map[string]any{"access_token": "token", "org_uuid": "org-a", "account_uuid": "acc-a"}

	account, err := svc.UpdateOwned(context.Background(), ownerID, 2, UpdateAccountRequest{Credentials: &credentials})

	require.Nil(t, account)
	require.ErrorIs(t, err, ErrOwnedAccountAlreadyExists)
	require.Empty(t, repo.updatedAccounts)
}

func TestAccountServiceUpdateOwnedRejectsManualAccountLevel(t *testing.T) {
	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{}
	svc := &AccountService{accountRepo: repo}
	level := AccountLevelTeam

	account, err := svc.UpdateOwned(context.Background(), ownerID, 2, UpdateAccountRequest{AccountLevel: &level})

	require.Nil(t, account)
	require.ErrorIs(t, err, ErrOwnedAccountLevelNotAllowed)
	require.Empty(t, repo.updatedAccounts)
}

func TestAccountServiceBulkUpdateOwnedRejectsBatchDuplicateIdentityBeforeWrite(t *testing.T) {
	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{
		getByIDsAccounts: map[int64]*Account{
			1: {
				ID:          1,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				OwnerUserID: &ownerID,
				Credentials: map[string]any{"access_token": "token-1"},
				Concurrency: 1,
				Priority:    1,
			},
			2: {
				ID:          2,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				OwnerUserID: &ownerID,
				Credentials: map[string]any{"access_token": "token-2"},
				Concurrency: 1,
				Priority:    1,
			},
		},
		listOwnedByPlatform: map[string][]Account{
			PlatformOpenAI: {
				{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, OwnerUserID: &ownerID},
				{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, OwnerUserID: &ownerID},
			},
		},
	}
	svc := &AccountService{accountRepo: repo}

	result, err := svc.BulkUpdateOwned(context.Background(), ownerID, &BulkUpdateOwnedAccountsInput{
		AccountIDs:  []int64{1, 2},
		Credentials: map[string]any{"chatgpt_user_id": "user-same"},
	})

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrOwnedAccountAlreadyExists)
	require.Equal(t, 0, repo.bulkUpdateCalls)
	require.Empty(t, repo.updatedAccounts)
}

func TestAccountServiceBulkUpdateOwnedRejectsManualAccountLevel(t *testing.T) {
	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{}
	svc := &AccountService{accountRepo: repo}
	level := AccountLevelTeam

	result, err := svc.BulkUpdateOwned(context.Background(), ownerID, &BulkUpdateOwnedAccountsInput{
		AccountIDs:   []int64{1},
		AccountLevel: &level,
	})

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrOwnedAccountLevelNotAllowed)
	require.Equal(t, 0, repo.bulkUpdateCalls)
	require.Empty(t, repo.updatedAccounts)
}

func TestAccountServiceBulkUpdateOwnedRejectsDuplicateExistingOutsideBatch(t *testing.T) {
	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{
		getByIDsAccounts: map[int64]*Account{
			2: {
				ID:          2,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				OwnerUserID: &ownerID,
				Credentials: map[string]any{"access_token": "token"},
				Concurrency: 1,
				Priority:    1,
			},
		},
		listOwnedByPlatform: map[string][]Account{
			PlatformOpenAI: {
				{
					ID:          1,
					Platform:    PlatformOpenAI,
					Type:        AccountTypeOAuth,
					OwnerUserID: &ownerID,
					Credentials: map[string]any{"access_token": "token", "email": "same@example.com"},
				},
				{
					ID:          2,
					Platform:    PlatformOpenAI,
					Type:        AccountTypeOAuth,
					OwnerUserID: &ownerID,
				},
			},
		},
	}
	svc := &AccountService{accountRepo: repo}

	result, err := svc.BulkUpdateOwned(context.Background(), ownerID, &BulkUpdateOwnedAccountsInput{
		AccountIDs:  []int64{2},
		Credentials: map[string]any{"email": "SAME@example.com"},
	})

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrOwnedAccountAlreadyExists)
	require.Equal(t, 0, repo.bulkUpdateCalls)
}

func TestAccountServiceBulkUpdateOwnedSchedulableUsesRepositoryBulkUpdate(t *testing.T) {
	ownerID := int64(101)
	enabled := false
	repo := &ownedAccountDuplicateRepoStub{
		getByIDsAccounts: map[int64]*Account{
			1: {
				ID:          1,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				OwnerUserID: &ownerID,
				Credentials: map[string]any{"access_token": "token-1"},
				Concurrency: 1,
				Priority:    7,
			},
			2: {
				ID:          2,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				OwnerUserID: &ownerID,
				Credentials: map[string]any{"access_token": "token-2"},
				Concurrency: 1,
				Priority:    9,
			},
		},
	}
	svc := &AccountService{accountRepo: repo}

	result, err := svc.BulkUpdateOwned(context.Background(), ownerID, &BulkUpdateOwnedAccountsInput{
		AccountIDs:  []int64{1, 2},
		Schedulable: &enabled,
	})

	require.NoError(t, err)
	require.Equal(t, 2, result.Success)
	require.Equal(t, 0, result.Failed)
	require.Equal(t, 1, repo.bulkUpdateCalls)
	require.Equal(t, []int64{1, 2}, repo.bulkUpdateIDs)
	require.NotNil(t, repo.bulkUpdatePayload.Schedulable)
	require.False(t, *repo.bulkUpdatePayload.Schedulable)
	require.Nil(t, repo.bulkUpdatePayload.Priority)
	require.Empty(t, repo.updatedAccounts)
}

func TestAccountServiceBulkUpdateOwnedPreservesExplicitModelMapping(t *testing.T) {
	ownerID := int64(101)
	base := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		OwnerUserID: &ownerID,
		Credentials: map[string]any{
			"access_token": "token",
			"model_mapping": map[string]any{
				"gpt-5.4": "gpt-5.4",
			},
		},
		Concurrency: 1,
		Priority:    7,
		Status:      StatusActive,
		Schedulable: true,
	}
	byID := *base
	byIDs := *base
	repo := &ownedAccountDuplicateRepoStub{
		getByIDAccounts: map[int64]*Account{
			1: &byID,
		},
		getByIDsAccounts: map[int64]*Account{
			1: &byIDs,
		},
	}
	svc := &AccountService{accountRepo: repo}
	requestedMapping := map[string]any{
		"gpt-5.3-codex-spark": "gpt-5.3-codex-spark",
	}

	result, err := svc.BulkUpdateOwned(context.Background(), ownerID, &BulkUpdateOwnedAccountsInput{
		AccountIDs: []int64{1},
		Credentials: map[string]any{
			"model_mapping": requestedMapping,
		},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Success)
	require.Equal(t, 0, repo.bulkUpdateCalls)
	require.Len(t, repo.updatedAccounts, 1)
	require.Equal(t, "token", repo.updatedAccounts[0].Credentials["access_token"])
	require.Equal(t, requestedMapping, repo.updatedAccounts[0].Credentials["model_mapping"])
	require.NotContains(t, repo.updatedAccounts[0].Credentials, "compact_model_mapping")
}

func TestAccountServiceBulkUpdateOwnedShareModeUsesPerAccountUpdateOnly(t *testing.T) {
	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{
		getByIDAccounts: map[int64]*Account{
			1: {
				ID:           1,
				Platform:     PlatformOpenAI,
				AccountLevel: AccountLevelPlus,
				Type:         AccountTypeOAuth,
				OwnerUserID:  &ownerID,
				Credentials:  map[string]any{"access_token": "token", "chatgpt_account_id": "acct-1"},
				ShareMode:    AccountShareModePrivate,
				ShareStatus:  AccountShareStatusApproved,
				Status:       StatusActive,
				Schedulable:  true,
				Concurrency:  OpenAIPlusDefaultConcurrency,
				Priority:     1,
			},
		},
		getByIDsAccounts: map[int64]*Account{
			1: {
				ID:           1,
				Platform:     PlatformOpenAI,
				AccountLevel: AccountLevelPlus,
				Type:         AccountTypeOAuth,
				OwnerUserID:  &ownerID,
				Credentials:  map[string]any{"access_token": "token", "chatgpt_account_id": "acct-1"},
				ShareMode:    AccountShareModePrivate,
				ShareStatus:  AccountShareStatusApproved,
				Status:       StatusActive,
				Schedulable:  true,
				Concurrency:  OpenAIPlusDefaultConcurrency,
				Priority:     1,
			},
		},
		listOwnedByPlatform: map[string][]Account{
			PlatformOpenAI: {
				{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, OwnerUserID: &ownerID, Credentials: map[string]any{"chatgpt_account_id": "acct-1"}},
			},
		},
	}
	svc := &AccountService{
		accountRepo: repo,
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
	}
	shareMode := AccountShareModePrivate
	status := StatusDisabled

	result, err := svc.BulkUpdateOwned(context.Background(), ownerID, &BulkUpdateOwnedAccountsInput{
		AccountIDs: []int64{1},
		Status:     status,
		ShareMode:  &shareMode,
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Success)
	require.Equal(t, 0, repo.bulkUpdateCalls)
	require.Len(t, repo.updatedAccounts, 1)
	require.Equal(t, StatusDisabled, repo.updatedAccounts[0].Status)
	require.Equal(t, AccountShareModePrivate, repo.updatedAccounts[0].ShareMode)
}

func TestAccountServiceUpdateOwnedSwitchesApprovedPublicAccountBackToPrivateGroup(t *testing.T) {
	ownerID := int64(101)
	accountID := int64(20)
	repo := &ownedAccountDuplicateRepoStub{
		getByIDAccounts: map[int64]*Account{
			accountID: {
				ID:           accountID,
				Platform:     PlatformOpenAI,
				AccountLevel: AccountLevelPlus,
				Type:         AccountTypeOAuth,
				OwnerUserID:  &ownerID,
				Credentials:  map[string]any{"access_token": "token", "chatgpt_account_id": "acct-1"},
				ShareMode:    AccountShareModePublic,
				ShareStatus:  AccountShareStatusApproved,
				Status:       StatusActive,
				Schedulable:  true,
				Concurrency:  OpenAIPlusDefaultConcurrency,
				Priority:     1,
				GroupIDs:     []int64{18},
			},
		},
	}
	svc := &AccountService{
		accountRepo: repo,
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 18, Name: "Plus Shared Pool", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelPlus},
			},
		},
	}
	shareMode := AccountShareModePrivate
	redactedCredentials := map[string]any{
		"chatgpt_account_id": "acct-1",
		"model_mapping":      map[string]any{"gpt-5.5": "gpt-5.5"},
	}

	account, err := svc.UpdateOwned(context.Background(), ownerID, accountID, UpdateAccountRequest{
		ShareMode:   &shareMode,
		Credentials: &redactedCredentials,
	})

	require.NoError(t, err)
	require.Equal(t, AccountShareModePrivate, account.ShareMode)
	require.Equal(t, AccountShareStatusApproved, account.ShareStatus)
	require.Equal(t, []int64{99}, account.GroupIDs)
	require.Len(t, repo.updatedAccounts, 1)
	require.Equal(t, AccountShareModePrivate, repo.updatedAccounts[0].ShareMode)
	require.Equal(t, "token", repo.updatedAccounts[0].Credentials["access_token"])
	require.Equal(t, redactedCredentials["model_mapping"], repo.updatedAccounts[0].Credentials["model_mapping"])
	require.Equal(t, []int64{99}, repo.boundGroupIDs[accountID])
}

func TestAccountServiceUpdateOwnedAllowsProxiedAccountToEnterPublicPool(t *testing.T) {
	ownerID := int64(101)
	accountID := int64(20)
	proxyID := int64(7)
	repo := &ownedAccountDuplicateRepoStub{
		getByIDAccounts: map[int64]*Account{
			accountID: {
				ID:           accountID,
				Platform:     PlatformOpenAI,
				AccountLevel: AccountLevelPlus,
				Type:         AccountTypeOAuth,
				OwnerUserID:  &ownerID,
				Credentials:  map[string]any{"access_token": "token", "chatgpt_account_id": "acct-1"},
				ProxyID:      &proxyID,
				ShareMode:    AccountShareModePrivate,
				ShareStatus:  AccountShareStatusApproved,
				Status:       StatusActive,
				Schedulable:  true,
				Concurrency:  3,
				Priority:     1,
			},
		},
		listOwnedByPlatform: map[string][]Account{
			PlatformOpenAI: {
				{ID: accountID, Platform: PlatformOpenAI, Type: AccountTypeOAuth, OwnerUserID: &ownerID, Credentials: map[string]any{"chatgpt_account_id": "acct-1"}},
			},
		},
	}
	svc := &AccountService{
		accountRepo: repo,
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
	}
	shareMode := AccountShareModePublic
	redactedCredentials := map[string]any{
		"chatgpt_account_id": "acct-1",
		"model_mapping":      map[string]any{"gpt-5.5": "gpt-5.5"},
	}

	account, err := svc.UpdateOwned(context.Background(), ownerID, accountID, UpdateAccountRequest{
		ShareMode:   &shareMode,
		Credentials: &redactedCredentials,
	})

	require.NoError(t, err)
	require.Equal(t, AccountShareModePublic, account.ShareMode)
	require.Equal(t, AccountShareStatusPending, account.ShareStatus)
	require.Equal(t, &proxyID, account.ProxyID)
	require.Equal(t, ownedPersonalDefaultConcurrency, account.Concurrency)
	require.Equal(t, []int64{99}, account.GroupIDs)
	require.Equal(t, "token", account.Credentials["access_token"])
	require.Equal(t, redactedCredentials["model_mapping"], account.Credentials["model_mapping"])
	require.Equal(t, []int64{99}, repo.boundGroupIDs[accountID])
}

func TestAccountServiceApproveOwnedPublicShareAllowsProxiedAccount(t *testing.T) {
	ownerID := int64(101)
	accountID := int64(20)
	proxyID := int64(7)
	repo := &ownedAccountDuplicateRepoStub{
		getByIDAccounts: map[int64]*Account{
			accountID: {
				ID:           accountID,
				Platform:     PlatformOpenAI,
				AccountLevel: AccountLevelPlus,
				Type:         AccountTypeOAuth,
				OwnerUserID:  &ownerID,
				Credentials:  map[string]any{"access_token": "token"},
				ProxyID:      &proxyID,
				ShareMode:    AccountShareModePublic,
				ShareStatus:  AccountShareStatusPending,
				Status:       StatusActive,
				Schedulable:  true,
				Concurrency:  ownedPersonalDefaultConcurrency,
				Priority:     1,
			},
		},
	}
	svc := &AccountService{
		accountRepo: repo,
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 18, Name: "Plus Shared Pool", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelPlus},
			},
		},
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
		accountSharePolicyRepo: &ownedPublicSharePolicyRepoStub{
			policy: &AccountSharePolicy{ID: 1, OwnerShareRatio: 0.7, Enabled: true},
		},
	}

	account, err := svc.ApproveOwnedPublicShare(context.Background(), ownerID, accountID)

	require.NoError(t, err)
	require.Equal(t, AccountShareModePublic, account.ShareMode)
	require.Equal(t, AccountShareStatusApproved, account.ShareStatus)
	require.Equal(t, &proxyID, account.ProxyID)
	require.Equal(t, []int64{99, 18}, account.GroupIDs)
	require.Equal(t, []int64{99, 18}, repo.boundGroupIDs[accountID])
}

func TestAccountServiceManagedGroupIDsKeepsApprovedPublicAccountInPublicPool(t *testing.T) {
	ownerID := int64(101)
	svc := &AccountService{
		privateGroupProvisioner: &ownedPrivateGroupProvisionerStub{
			group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
		},
		groupRepo: &ownedPublicShareGroupRepoStub{
			groups: []Group{
				{ID: 18, Name: "Plus Shared Pool", Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopePublic, RequiredAccountLevel: AccountLevelPlus},
			},
		},
	}
	account := &Account{
		ID:           20,
		Platform:     PlatformOpenAI,
		Type:         AccountTypeOAuth,
		OwnerUserID:  &ownerID,
		ShareMode:    AccountShareModePublic,
		ShareStatus:  AccountShareStatusApproved,
		AccountLevel: AccountLevelPlus,
	}

	groupIDs, err := svc.managedOwnedAccountGroupIDsForShareMode(context.Background(), ownerID, account, AccountShareModePublic)

	require.NoError(t, err)
	require.Equal(t, []int64{99, 18}, groupIDs)
}

func TestAccountServiceDuplicateIdentityKeys(t *testing.T) {
	t.Run("openai prefers stable user identity over account identity", func(t *testing.T) {
		keys := accountDuplicateIdentityKeys(&Account{
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Credentials: map[string]any{"chatgpt_account_id": "acct", "chatgpt_user_id": "user", "organization_id": "org"},
		})

		require.Contains(t, keys, ownedAccountDuplicateKey{Name: "openai.chatgpt_user_id", Value: "user"})
		require.NotContains(t, keys, ownedAccountDuplicateKey{Name: "openai.chatgpt_account_id", Value: "acct"})
		require.NotContains(t, keys, ownedAccountDuplicateKey{Name: "openai.organization_id", Value: "org"})
	})

	t.Run("openai falls back to account identity when user identity is unavailable", func(t *testing.T) {
		keys := accountDuplicateIdentityKeys(&Account{
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Credentials: map[string]any{"chatgpt_account_id": "acct"},
		})

		require.Contains(t, keys, ownedAccountDuplicateKey{Name: "openai.chatgpt_account_id", Value: "acct"})
	})

	t.Run("openai uses email before shared account identity when user identity is unavailable", func(t *testing.T) {
		keys := accountDuplicateIdentityKeys(&Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"chatgpt_account_id": "team-account",
				"email":              "Seat@Example.COM",
			},
		})

		require.Contains(t, keys, ownedAccountDuplicateKey{Name: "openai.email", Value: "seat@example.com"})
		require.NotContains(t, keys, ownedAccountDuplicateKey{Name: "openai.chatgpt_account_id", Value: "team-account"})
	})

	t.Run("anthropic combines org and account uuid", func(t *testing.T) {
		keys := accountDuplicateIdentityKeys(&Account{
			Platform:    PlatformAnthropic,
			Type:        AccountTypeOAuth,
			Credentials: map[string]any{"org_uuid": "ORG", "account_uuid": "ACC"},
		})

		require.Contains(t, keys, ownedAccountDuplicateKey{Name: "anthropic.org_account", Value: "org|acc"})
	})

	t.Run("antigravity email is case insensitive", func(t *testing.T) {
		keys := accountDuplicateIdentityKeys(&Account{
			Platform:    PlatformAntigravity,
			Type:        AccountTypeOAuth,
			Credentials: map[string]any{"email": "USER@Example.COM"},
		})

		require.Contains(t, keys, ownedAccountDuplicateKey{Name: "antigravity.email", Value: "user@example.com"})
	})
}

func TestIsOwnedAccountPublicShareApprovableAllowsRateLimitedAccountWithOption(t *testing.T) {
	resetAt := time.Now().Add(time.Hour)
	account := &Account{
		Platform:         PlatformOpenAI,
		Type:             AccountTypeOAuth,
		Status:           StatusActive,
		Schedulable:      true,
		RateLimitResetAt: &resetAt,
	}

	require.False(t, isOwnedAccountPublicShareApprovable(account, false))
	require.True(t, isOwnedAccountPublicShareApprovable(account, true))
}

func TestIsOwnedAccountPublicShareApprovableStillRejectsDisabledAccount(t *testing.T) {
	resetAt := time.Now().Add(time.Hour)
	account := &Account{
		Platform:         PlatformOpenAI,
		Type:             AccountTypeOAuth,
		Status:           StatusDisabled,
		Schedulable:      true,
		RateLimitResetAt: &resetAt,
	}

	require.False(t, isOwnedAccountPublicShareApprovable(account, true))
}

func TestAccountQuotaDashboardWindowCapacityUsesOnlySchedulableAccounts(t *testing.T) {
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	limitedUntil := now.Add(time.Hour)
	usageReset := now.Add(2 * time.Hour).Format(time.RFC3339)

	builder := newAccountQuotaDashboardBuilder(now)
	for _, account := range []Account{
		{
			ID:          1,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 4,
			Extra: map[string]any{
				"codex_5h_used_percent": 40.0,
				"codex_5h_reset_at":     usageReset,
				"codex_7d_used_percent": 60.0,
				"codex_7d_reset_at":     usageReset,
			},
		},
		{
			ID:               2,
			Platform:         PlatformOpenAI,
			Type:             AccountTypeOAuth,
			Status:           StatusActive,
			Schedulable:      true,
			Concurrency:      8,
			RateLimitResetAt: &limitedUntil,
			Extra: map[string]any{
				"codex_5h_used_percent": 10.0,
				"codex_5h_reset_at":     usageReset,
				"codex_7d_used_percent": 10.0,
				"codex_7d_reset_at":     usageReset,
			},
		},
		{
			ID:          3,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusError,
			Schedulable: true,
			Concurrency: 2,
		},
		{
			ID:          4,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusDisabled,
			Schedulable: true,
			Concurrency: 3,
		},
		{
			ID:               5,
			Platform:         PlatformOpenAI,
			Type:             AccountTypeOAuth,
			Status:           StatusError,
			Schedulable:      true,
			Concurrency:      5,
			RateLimitResetAt: &limitedUntil,
		},
	} {
		builder.addAccount(account)
	}

	dashboard := builder.finalize()

	require.Equal(t, 5, dashboard.Totals.AccountCount)
	require.Equal(t, 1, dashboard.Totals.SchedulableAccountCount)
	require.Equal(t, 1, dashboard.Totals.RateLimitedAccountCount)
	require.Equal(t, 2, dashboard.Totals.ErrorAccountCount)
	require.Equal(t, 1, dashboard.Totals.DisabledAccountCount)
	require.Equal(t, 22, dashboard.Totals.ConcurrencyCapacity)
	require.Equal(t, 4, dashboard.Totals.SchedulableConcurrencyCapacity)
	require.Len(t, dashboard.Totals.UsageWindows, 2)
	for _, window := range dashboard.Totals.UsageWindows {
		require.Equal(t, 1, window.AccountCount)
		require.Equal(t, 1, window.KnownAccountCount)
	}
}

func TestAccountQuotaDashboardCountsQuotaProtectedAccounts(t *testing.T) {
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	builder := newAccountQuotaDashboardBuilder(now)
	builder.addAccount(Account{
		ID:          1,
		Platform:    PlatformCustom,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 6,
		Extra: map[string]any{
			"quota_limit": 2.0,
			"quota_used":  2.0,
		},
	})
	builder.addAccount(Account{
		ID:          2,
		Platform:    PlatformCustom,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 4,
		Extra: map[string]any{
			"quota_limit": 2.0,
			"quota_used":  1.0,
		},
	})

	dashboard := builder.finalize()
	require.Equal(t, 2, dashboard.Totals.AccountCount)
	require.Equal(t, 1, dashboard.Totals.QuotaProtectedAccountCount)
	require.Equal(t, 1, dashboard.Totals.SchedulableAccountCount)
	require.Equal(t, 10, dashboard.Totals.ConcurrencyCapacity)
	require.Equal(t, 4, dashboard.Totals.SchedulableConcurrencyCapacity)
}

func TestAccountQuotaGroupDashboardUsesGeneratedAtForSchedulability(t *testing.T) {
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	limitedUntil := now.Add(time.Hour)
	group := &Group{
		ID:                   101,
		Name:                 "OpenAI shared pool",
		Platform:             PlatformOpenAI,
		Status:               StatusActive,
		Scope:                GroupScopePublic,
		RequiredAccountLevel: AccountLevelPlus,
		RateMultiplier:       0.08,
	}

	builder := newAccountQuotaGroupDashboardBuilder(now)
	builder.addAccount(Account{
		ID:               1,
		Platform:         PlatformOpenAI,
		Type:             AccountTypeOAuth,
		Status:           StatusActive,
		Schedulable:      true,
		RateLimitResetAt: &limitedUntil,
		Groups:           []*Group{group},
	})

	summaries := builder.finalize()
	require.Len(t, summaries, 1)
	require.Equal(t, 1, summaries[0].AccountCount)
	require.Equal(t, 0, summaries[0].SchedulableAccountCount)
	require.Equal(t, 1, summaries[0].RateLimitedAccountCount)
	require.Equal(t, AccountLevelPlus, summaries[0].AccountLevel)
	require.InDelta(t, 0.08, summaries[0].RateMultiplier, 1e-12)
}
