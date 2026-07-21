//go:build unit

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbent "ikik-api/ent"
)

func TestUserEntityToServiceMapsWalletBuckets(t *testing.T) {
	got := userEntityToService(&dbent.User{
		ID:                  42,
		Balance:             91.25,
		RechargeBalance:     40.5,
		InviteIncomeBalance: 12.25,
		ShareIncomeBalance:  38.5,
		PointsBalance:       7.75,
		PreferPointsBilling: true,
		TotalInviteIncome:   22.5,
		TotalShareIncome:    75.25,
	})

	require.NotNil(t, got)
	require.InDelta(t, 40.5, got.RechargeBalance, 0.000001)
	require.InDelta(t, 12.25, got.InviteIncomeBalance, 0.000001)
	require.InDelta(t, 38.5, got.ShareIncomeBalance, 0.000001)
	require.InDelta(t, 7.75, got.PointsBalance, 0.000001)
	require.True(t, got.PreferPointsBilling)
	require.InDelta(t, 22.5, got.TotalInviteIncome, 0.000001)
	require.InDelta(t, 75.25, got.TotalShareIncome, 0.000001)
}
