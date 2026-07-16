package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/pkg/pagination"
)

type apiKeyAvailableGroupsUserRepoStub struct {
	user *User
}

func (s *apiKeyAvailableGroupsUserRepoStub) Create(context.Context, *User) error {
	panic("unexpected Create call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	clone := *s.user
	return &clone, nil
}
func (s *apiKeyAvailableGroupsUserRepoStub) GetByEmail(context.Context, string) (*User, error) {
	panic("unexpected GetByEmail call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) GetFirstAdmin(context.Context) (*User, error) {
	panic("unexpected GetFirstAdmin call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) Update(context.Context, *User) error {
	panic("unexpected Update call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	panic("unexpected GetUserAvatar call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	panic("unexpected UpsertUserAvatar call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) DeleteUserAvatar(context.Context, int64) error {
	panic("unexpected DeleteUserAvatar call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserIDs call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserID call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	panic("unexpected UpdateUserLastActiveAt call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected UpdateBalance call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected DeductBalance call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	panic("unexpected ExistsByEmail call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	panic("unexpected ListUserAuthIdentities call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) UnbindUserAuthProvider(context.Context, int64, string) error {
	panic("unexpected UnbindUserAuthProvider call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected UpdateTotpSecret call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) EnableTotp(context.Context, int64) error {
	panic("unexpected EnableTotp call")
}
func (s *apiKeyAvailableGroupsUserRepoStub) DisableTotp(context.Context, int64) error {
	panic("unexpected DisableTotp call")
}

type apiKeyAvailableGroupsGroupRepoStub struct {
	groups []Group
}

func (s *apiKeyAvailableGroupsGroupRepoStub) Create(context.Context, *Group) error {
	panic("unexpected Create call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) GetByID(context.Context, int64) (*Group, error) {
	panic("unexpected GetByID call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) GetByIDLite(context.Context, int64) (*Group, error) {
	panic("unexpected GetByIDLite call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) Update(context.Context, *Group) error {
	panic("unexpected Update call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) DeleteCascade(context.Context, int64) ([]int64, error) {
	panic("unexpected DeleteCascade call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) List(context.Context, pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) ListActive(context.Context) ([]Group, error) {
	panic("unexpected ListActive call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) ListActiveByPlatform(context.Context, string) ([]Group, error) {
	panic("unexpected ListActiveByPlatform call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) ListActiveVisibleToUser(context.Context, int64, []int64) ([]Group, error) {
	groups := make([]Group, len(s.groups))
	copy(groups, s.groups)
	return groups, nil
}
func (s *apiKeyAvailableGroupsGroupRepoStub) ExistsByName(context.Context, string) (bool, error) {
	panic("unexpected ExistsByName call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) GetAccountCount(context.Context, int64) (int64, int64, error) {
	panic("unexpected GetAccountCount call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected DeleteAccountGroupsByGroupID call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	panic("unexpected GetAccountIDsByGroupIDs call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) BindAccountsToGroup(context.Context, int64, []int64) error {
	panic("unexpected BindAccountsToGroup call")
}
func (s *apiKeyAvailableGroupsGroupRepoStub) UpdateSortOrders(context.Context, []GroupSortOrderUpdate) error {
	panic("unexpected UpdateSortOrders call")
}

type apiKeyAvailableGroupsSubRepoStub struct {
	active []UserSubscription
}

func (s *apiKeyAvailableGroupsSubRepoStub) Create(context.Context, *UserSubscription) error {
	panic("unexpected Create call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) GetByID(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByID call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) GetByIDIncludeDeleted(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByIDIncludeDeleted call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	panic("unexpected GetByUserIDAndGroupID call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	panic("unexpected GetActiveByUserIDAndGroupID call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) Update(context.Context, *UserSubscription) error {
	panic("unexpected Update call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) Restore(context.Context, int64, string) (*UserSubscription, error) {
	panic("unexpected Restore call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) ListByUserID(context.Context, int64) ([]UserSubscription, error) {
	panic("unexpected ListByUserID call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	subs := make([]UserSubscription, len(s.active))
	copy(subs, s.active)
	return subs, nil
}
func (s *apiKeyAvailableGroupsSubRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) List(context.Context, pagination.PaginationParams, *int64, *int64, string, string, string, string) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) ExistsByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	panic("unexpected ExistsByUserIDAndGroupID call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) ExistsActiveByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	panic("unexpected ExistsActiveByUserIDAndGroupID call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) ExtendExpiry(context.Context, int64, time.Time) error {
	panic("unexpected ExtendExpiry call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) UpdateStatus(context.Context, int64, string) error {
	panic("unexpected UpdateStatus call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) UpdateNotes(context.Context, int64, string) error {
	panic("unexpected UpdateNotes call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) ActivateWindows(context.Context, int64, time.Time) error {
	panic("unexpected ActivateWindows call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) ResetDailyUsage(context.Context, int64, time.Time) error {
	panic("unexpected ResetDailyUsage call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) ResetWeeklyUsage(context.Context, int64, time.Time) error {
	panic("unexpected ResetWeeklyUsage call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) ResetMonthlyUsage(context.Context, int64, time.Time) error {
	panic("unexpected ResetMonthlyUsage call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) IncrementUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementUsage call")
}
func (s *apiKeyAvailableGroupsSubRepoStub) BatchUpdateExpiredStatus(context.Context) (int64, error) {
	panic("unexpected BatchUpdateExpiredStatus call")
}

func TestAPIKeyService_GetAvailableGroups_PublicBalanceGroupIsSelectable(t *testing.T) {
	userID := int64(7)
	otherUserID := int64(8)
	groups := []Group{
		{ID: 10, Name: "FREE shared pool", Status: StatusActive, Scope: GroupScopePublic, SubscriptionType: SubscriptionTypeStandard, IsExclusive: true},
		{ID: 11, Name: "legacy exclusive", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, IsExclusive: true},
		{ID: 12, Name: "other private subscription", Status: StatusActive, Scope: GroupScopeUserPrivate, SubscriptionType: SubscriptionTypeSubscription, IsExclusive: true, OwnerUserID: &otherUserID},
	}
	svc := NewAPIKeyService(
		nil,
		&apiKeyAvailableGroupsUserRepoStub{user: &User{ID: userID}},
		&apiKeyAvailableGroupsGroupRepoStub{groups: groups},
		&apiKeyAvailableGroupsSubRepoStub{},
		nil,
		nil,
		nil,
	)

	available, err := svc.GetAvailableGroups(context.Background(), userID)

	require.NoError(t, err)
	require.Len(t, available, 1)
	require.Equal(t, int64(10), available[0].ID)
}

func TestAPIKeyService_GetAvailableGroups_OwnPrivateSubscriptionRequiresActiveSubscription(t *testing.T) {
	userID := int64(7)
	groups := []Group{
		{ID: 20, Name: "own private subscription", Status: StatusActive, Scope: GroupScopeUserPrivate, SubscriptionType: SubscriptionTypeSubscription, IsExclusive: true, OwnerUserID: &userID},
	}
	svc := NewAPIKeyService(
		nil,
		&apiKeyAvailableGroupsUserRepoStub{user: &User{ID: userID}},
		&apiKeyAvailableGroupsGroupRepoStub{groups: groups},
		&apiKeyAvailableGroupsSubRepoStub{active: []UserSubscription{{UserID: userID, GroupID: 20, Status: SubscriptionStatusActive}}},
		nil,
		nil,
		nil,
	)

	available, err := svc.GetAvailableGroups(context.Background(), userID)

	require.NoError(t, err)
	require.Len(t, available, 1)
	require.Equal(t, int64(20), available[0].ID)
}
