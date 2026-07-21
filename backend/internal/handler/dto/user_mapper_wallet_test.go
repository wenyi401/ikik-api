//go:build unit

package dto

import (
	"testing"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/service"
)

func TestUserFromServiceShallow_MapsWalletBuckets(t *testing.T) {
	out := UserFromServiceShallow(&service.User{
		ID:                  7,
		Balance:             61,
		RechargeBalance:     11,
		InviteIncomeBalance: 13,
		ShareIncomeBalance:  17,
		PointsBalance:       19,
		PreferPointsBilling: true,
		TotalRecharged:      23,
		TotalInviteIncome:   29,
		TotalShareIncome:    31,
	})

	require.NotNil(t, out)
	require.Equal(t, 11.0, out.RechargeBalance)
	require.Equal(t, 13.0, out.InviteIncomeBalance)
	require.Equal(t, 17.0, out.ShareIncomeBalance)
	require.Equal(t, 19.0, out.PointsBalance)
	require.True(t, out.PreferPointsBilling)
	require.Equal(t, 23.0, out.TotalRecharged)
	require.Equal(t, 29.0, out.TotalInviteIncome)
	require.Equal(t, 31.0, out.TotalShareIncome)
}
