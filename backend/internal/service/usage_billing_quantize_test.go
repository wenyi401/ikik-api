package service

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestUsageBillingCommandQuantizesMonetaryFields(t *testing.T) {
	const raw = 0.000078125
	cmd := &UsageBillingCommand{
		RequestID:                  "req-quantize",
		UserID:                     1,
		AccountID:                  2,
		APIKeyID:                   3,
		BalanceCost:                raw,
		SubscriptionCost:           raw,
		PrivateGroupCommissionCost: raw,
		APIKeyQuotaCost:            raw,
		APIKeyRateLimitCost:        raw,
		AccountQuotaCost:           raw,
	}
	expectedFingerprint := buildUsageBillingFingerprint(cmd)
	cmd.Normalize()

	require.Equal(t, expectedFingerprint, cmd.RequestFingerprint)
	for _, amount := range []float64{
		cmd.BalanceCost,
		cmd.SubscriptionCost,
		cmd.PrivateGroupCommissionCost,
		cmd.APIKeyQuotaCost,
		cmd.APIKeyRateLimitCost,
		cmd.AccountQuotaCost,
	} {
		require.LessOrEqual(t, -decimal.NewFromFloat(amount).Exponent(), int32(UsageBillingMonetaryScale))
	}
	require.Equal(t, cmd.BalanceCost, cmd.APIKeyQuotaCost)
}

func TestQuantizeUsageBillingAmountUsesNumericScale(t *testing.T) {
	for _, in := range []float64{0.000078124, 0.000078125, 0.000078126, -0.000078125} {
		want, _ := decimal.NewFromFloat(in).Round(UsageBillingMonetaryScale).Float64()
		require.Equal(t, want, QuantizeUsageBillingAmount(in))
	}
	require.True(t, math.IsNaN(QuantizeUsageBillingAmount(math.NaN())))
	require.True(t, math.IsInf(QuantizeUsageBillingAmount(math.Inf(1)), 1))
}
