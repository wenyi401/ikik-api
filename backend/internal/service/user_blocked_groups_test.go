//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserCanBindGroupBlockedTakesPrecedence(t *testing.T) {
	user := &User{
		AllowedGroups: []int64{10},
		BlockedGroups: []int64{10, 20},
	}

	require.False(t, user.CanBindGroup(10, true), "a block must override an exclusive-group grant")
	require.False(t, user.CanBindGroup(20, false), "a public group can be explicitly blocked")
	require.True(t, user.CanBindGroup(30, false))
	require.False(t, user.CanBindGroup(30, true))
}

func TestAPIKeyGroupChecksRejectBlockedSubscription(t *testing.T) {
	user := &User{ID: 7, BlockedGroups: []int64{42}}
	group := &Group{ID: 42, SubscriptionType: SubscriptionTypeSubscription}
	svc := &APIKeyService{}

	require.False(t, svc.canUserBindGroupInternal(user, group, map[int64]bool{42: true}))
	require.False(t, svc.canUserBindAPIKeyGroup(context.Background(), user, group))
}

func TestUserRiskGroupBlockExpiresWithoutRemovingManualBlocks(t *testing.T) {
	now := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	activeUntil := now.Add(time.Hour)
	expiredAt := now.Add(-time.Second)
	user := &User{
		BlockedGroups: []int64{10},
		RiskGroupBlocks: []UserRiskGroupBlock{
			{GroupID: 20, BlockedUntil: &activeUntil},
			{GroupID: 30, BlockedUntil: &expiredAt},
			{GroupID: 40, Permanent: true},
		},
	}

	require.True(t, user.isGroupBlockedAt(10, now), "manual blocks remain independent")
	require.True(t, user.isGroupBlockedAt(20, now))
	require.False(t, user.isGroupBlockedAt(30, now), "expired risk penalties automatically release the group")
	require.True(t, user.isGroupBlockedAt(40, now))
}

func TestAPIKeyAuthSnapshotPreservesBlockedGroups(t *testing.T) {
	blockedUntil := time.Now().Add(24 * time.Hour)
	svc := &APIKeyService{}
	apiKey := &APIKey{
		ID:     1,
		UserID: 7,
		Status: StatusActive,
		User: &User{
			ID:            7,
			Status:        StatusActive,
			BlockedGroups: []int64{12, 34},
			RiskGroupBlocks: []UserRiskGroupBlock{
				{GroupID: 56, BlockedUntil: &blockedUntil},
				{GroupID: 78, Permanent: true},
			},
		},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	require.Equal(t, []int64{12, 34}, snapshot.User.BlockedGroups)
	require.Equal(t, apiKey.User.RiskGroupBlocks, snapshot.User.RiskGroupBlocks)

	restored := svc.snapshotToAPIKey("test-key", snapshot)
	require.Equal(t, []int64{12, 34}, restored.User.BlockedGroups)
	require.Equal(t, apiKey.User.RiskGroupBlocks, restored.User.RiskGroupBlocks)
}

func TestAdminServiceUpdateUserInvalidatesAuthCacheOnBlockedGroupsChange(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", BlockedGroups: []int64{10}}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       &redeemRepoStub{},
		authCacheInvalidator: invalidator,
	}

	blockedGroups := []int64{20}
	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{BlockedGroups: &blockedGroups})
	require.NoError(t, err)
	require.Equal(t, []int64{20}, updated.BlockedGroups)
	require.Equal(t, []int64{42}, invalidator.userIDs)
}
