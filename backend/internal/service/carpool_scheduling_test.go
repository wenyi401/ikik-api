package service

import (
	"context"
	"testing"
	"time"

	"ikik-api/internal/pkg/ctxkey"

	"github.com/stretchr/testify/require"
)

func TestIsUserCarpoolRequestGroup(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{
		ID:    6,
		Scope: GroupScopePublic,
	})
	require.False(t, isUserCarpoolRequestGroup(ctx, 6))

	ownerID := int64(42)
	ctx = context.WithValue(context.Background(), ctxkey.Group, &Group{
		ID:          12,
		Scope:       GroupScopeUserCarpool,
		OwnerUserID: &ownerID,
	})
	require.True(t, isUserCarpoolRequestGroup(ctx, 12))
	require.False(t, isUserCarpoolRequestGroup(ctx, 13))
}

func TestIsCarpoolAccountSchedulable_IgnoresLocalRateLimitReset(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	resetAt := now.Add(72 * time.Hour)

	account := &Account{
		ID:               1,
		Status:           StatusActive,
		Schedulable:      true,
		RateLimitResetAt: &resetAt,
	}

	require.True(t, isCarpoolAccountSchedulableAt(account, now))
}

func TestIsCarpoolAccountSchedulable_RespectsHardUnschedulableState(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	until := now.Add(10 * time.Minute)

	account := &Account{
		ID:                      1,
		Status:                  StatusActive,
		Schedulable:             true,
		TempUnschedulableUntil:  &until,
		TempUnschedulableReason: "token refresh cooldown",
	}

	require.False(t, isCarpoolAccountSchedulableAt(account, now))
}
