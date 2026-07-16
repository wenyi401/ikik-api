//go:build unit

// TASK-003 计费精度不变量测试（INVARIANTS.md ② 计费精度）。
//
// 本文件覆盖纯计算层（BillingService）的不变量：
//   - I-2.2: 5m 与 1h 缓存写入差异化计价（两档金额断言 + 无明细回退 + breakdown 开关门控）
//   - I-2.4: 倍率叠加顺序 serviceTier → rateMultiplier；rateMultiplier=0 时
//     ActualCost=0 但 TotalCost 保留
//
// 所有期望值均为人工核算的固定值（算式在行内注释），容差 1e-10。
// 这些是 characterization 测试：固化当前 main 上的实际行为，插件化改造期间
// 任何金额漂移都应使其失败。
package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/config"
)

// billInvManualPricingService 构造带手工定价表的 BillingService（无动态价格源）。
func billInvManualPricingService(pricing *ModelPricing) *BillingService {
	return &BillingService{
		cfg: &config.Config{},
		fallbackPrices: map[string]*ModelPricing{
			"claude-sonnet-4": pricing,
		},
	}
}

// TestBillingInvariant_CacheTier5mVs1h 锁定 I-2.2：cache_creation 分
// CacheCreation5mPrice / CacheCreation1hPrice 两档分别计价。
// 定价：input $3/MTok, output $15/MTok, cache_read $0.3/MTok,
// 5m 写入 $4/MTok, 1h 写入 $5/MTok（SupportsCacheBreakdown=true）。
func TestBillingInvariant_CacheTier5mVs1h(t *testing.T) {
	breakdownPricing := &ModelPricing{
		InputPricePerToken:         3e-6,
		OutputPricePerToken:        15e-6,
		CacheReadPricePerToken:     0.3e-6,
		CacheCreationPricePerToken: 3.75e-6,
		SupportsCacheBreakdown:     true,
		CacheCreation5mPrice:       4e-6,
		CacheCreation1hPrice:       5e-6,
	}
	noBreakdownPricing := &ModelPricing{
		InputPricePerToken:         3e-6,
		OutputPricePerToken:        15e-6,
		CacheReadPricePerToken:     0.3e-6,
		CacheCreationPricePerToken: 3.75e-6,
		SupportsCacheBreakdown:     false,
	}

	tests := []struct {
		name    string
		pricing *ModelPricing
		tokens  UsageTokens
		// 期望值（固定值 + 1e-10 容差）
		wantInput         float64
		wantOutput        float64
		wantCacheCreation float64
		wantCacheRead     float64
		wantTotal         float64
	}{
		{
			name:    "5m与1h两档分别计价",
			pricing: breakdownPricing,
			tokens: UsageTokens{
				InputTokens:  1000,
				OutputTokens: 500,
				// 上游通常同时给出总量与 ephemeral 明细；有明细时按明细计价
				CacheCreationTokens:   12000,
				CacheCreation5mTokens: 8000,
				CacheCreation1hTokens: 4000,
				CacheReadTokens:       2000,
			},
			wantInput:  0.003,  // 1000 × $3/MTok = 0.003
			wantOutput: 0.0075, // 500 × $15/MTok = 0.0075
			// 8000 × 4e-6 = 0.032; 4000 × 5e-6 = 0.020; 合计 0.052
			wantCacheCreation: 0.052,
			wantCacheRead:     0.0006, // 2000 × $0.3/MTok = 0.0006
			wantTotal:         0.0631, // 0.003+0.0075+0.052+0.0006
		},
		{
			name:    "仅1h写入按1h单价",
			pricing: breakdownPricing,
			tokens: UsageTokens{
				CacheCreationTokens:   10000,
				CacheCreation1hTokens: 10000,
			},
			wantCacheCreation: 0.05, // 10000 × 5e-6 = 0.05
			wantTotal:         0.05,
		},
		{
			name:    "无ephemeral明细时全部回退5m单价",
			pricing: breakdownPricing,
			tokens: UsageTokens{
				CacheCreationTokens: 6000, // 5m/1h 明细均为 0
			},
			wantCacheCreation: 0.024, // 6000 × 4e-6 = 0.024（回退 5m 档）
			wantTotal:         0.024,
		},
		{
			name:    "breakdown关闭时按标准单价计CacheCreationTokens并忽略5m1h明细",
			pricing: noBreakdownPricing,
			tokens: UsageTokens{
				CacheCreationTokens:   6000,
				CacheCreation5mTokens: 4000,
				CacheCreation1hTokens: 2000,
			},
			wantCacheCreation: 0.0225, // 6000 × 3.75e-6 = 0.0225（明细被忽略）
			wantTotal:         0.0225,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := billInvManualPricingService(tt.pricing)
			cost, err := svc.CalculateCost("claude-sonnet-4", tt.tokens, 1.0)
			require.NoError(t, err)
			require.InDelta(t, tt.wantInput, cost.InputCost, 1e-10)
			require.InDelta(t, tt.wantOutput, cost.OutputCost, 1e-10)
			require.InDelta(t, tt.wantCacheCreation, cost.CacheCreationCost, 1e-10)
			require.InDelta(t, tt.wantCacheRead, cost.CacheReadCost, 1e-10)
			require.InDelta(t, tt.wantTotal, cost.TotalCost, 1e-10)
			require.InDelta(t, tt.wantTotal, cost.ActualCost, 1e-10) // 倍率 1.0
		})
	}
}

// TestBillingInvariant_CacheTierBreakdownGating 锁定 I-2.2 的动态定价门控：
// 仅当 LiteLLM 的 1h 单价存在且严格大于 5m 单价时启用两档计费
// （防止上游数据错误导致少收费，见 billing_service.go GetModelPricing）。
func TestBillingInvariant_CacheTierBreakdownGating(t *testing.T) {
	tokens := UsageTokens{
		CacheCreationTokens:   1000,
		CacheCreation5mTokens: 600,
		CacheCreation1hTokens: 400,
	}

	tests := []struct {
		name              string
		price5m, price1h  float64
		wantBreakdown     bool
		wantCacheCreation float64
	}{
		{
			name:    "1h大于5m启用两档",
			price5m: 4e-6, price1h: 5e-6,
			wantBreakdown: true,
			// 600 × 4e-6 = 0.0024; 400 × 5e-6 = 0.002; 合计 0.0044
			wantCacheCreation: 0.0044,
		},
		{
			name:    "1h等于5m禁用两档防少收费",
			price5m: 4e-6, price1h: 4e-6,
			wantBreakdown: false,
			// 回退标准单价：1000 × 4e-6 = 0.004
			wantCacheCreation: 0.004,
		},
		{
			name:    "1h缺失禁用两档",
			price5m: 4e-6, price1h: 0,
			wantBreakdown: false,
			// 1000 × 4e-6 = 0.004
			wantCacheCreation: 0.004,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewBillingService(&config.Config{}, &PricingService{
				pricingData: map[string]*LiteLLMModelPricing{
					"tier-gate-model": {
						InputCostPerToken:                   3e-6,
						OutputCostPerToken:                  15e-6,
						CacheCreationInputTokenCost:         tt.price5m,
						CacheCreationInputTokenCostAbove1hr: tt.price1h,
						CacheReadInputTokenCost:             0.3e-6,
					},
				},
			})

			pricing, err := svc.GetModelPricing("tier-gate-model")
			require.NoError(t, err)
			require.Equal(t, tt.wantBreakdown, pricing.SupportsCacheBreakdown)

			cost, err := svc.CalculateCost("tier-gate-model", tokens, 1.0)
			require.NoError(t, err)
			require.InDelta(t, tt.wantCacheCreation, cost.CacheCreationCost, 1e-10)
		})
	}
}

// TestBillingInvariant_ServiceTierThenRateMultiplier 锁定 I-2.4 的应用顺序：
// 各分项成本先应用 serviceTier 倍率（或 priority 显式价），TotalCost 为分项之和
// （含 tier、不含 rateMultiplier），最后 ActualCost = TotalCost × rateMultiplier。
func TestBillingInvariant_ServiceTierThenRateMultiplier(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		serviceTier    string
		rateMultiplier float64
		tokens         UsageTokens
		wantInput      float64
		wantOutput     float64
		wantCacheWrite float64
		wantCacheRead  float64
		wantTotal      float64
		wantActual     float64
	}{
		{
			// gpt-5.4 fallback 价：in $2.5/MTok, out $15/MTok, cache_write $2.5/MTok,
			// cache_read $0.25/MTok；flex 无显式价 → 0.5 倍 tier multiplier。
			name:           "flex_tier后再乘rateMultiplier",
			model:          "gpt-5.4",
			serviceTier:    "flex",
			rateMultiplier: 1.5,
			tokens:         UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheCreationTokens: 400, CacheReadTokens: 2000},
			wantInput:      0.00125,  // 1000×2.5e-6=0.0025 ×0.5
			wantOutput:     0.00375,  // 500×15e-6=0.0075 ×0.5
			wantCacheWrite: 0.0005,   // 400×2.5e-6=0.001 ×0.5
			wantCacheRead:  0.00025,  // 2000×0.25e-6=0.0005 ×0.5
			wantTotal:      0.00575,  // 分项之和（含 tier、不含 rate）
			wantActual:     0.008625, // 0.00575 × 1.5
		},
		{
			// gpt-5.4 有显式 priority 价：in $5/MTok, out $30/MTok, cache_read $0.5/MTok。
			// 注意（characterization）：cache_write 无 priority 显式价，因此该分项
			// 回退到 priority tier multiplier 2 倍。
			name:           "priority显式价后再乘rateMultiplier",
			model:          "gpt-5.4",
			serviceTier:    "priority",
			rateMultiplier: 2.0,
			tokens:         UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheCreationTokens: 1000, CacheReadTokens: 2000},
			wantInput:      0.005, // 1000 × 5e-6（priority 价）
			wantOutput:     0.015, // 500 × 30e-6（priority 价）
			wantCacheWrite: 0.005, // 1000 × 2.5e-6 × 2（无 priority 显式价，回退 2 倍）
			wantCacheRead:  0.001, // 2000 × 0.5e-6（priority 价）
			wantTotal:      0.026,
			wantActual:     0.052, // 0.026 × 2.0
		},
		{
			// claude-sonnet-4 fallback 无 priority 显式价 → 回退 2.0 倍 tier multiplier，
			// 此路径下 cache_write 也参与 2 倍上浮。
			name:           "priority无显式价回退2倍tier后再乘rateMultiplier",
			model:          "claude-sonnet-4",
			serviceTier:    "priority",
			rateMultiplier: 1.5,
			tokens:         UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheCreationTokens: 2000, CacheReadTokens: 3000},
			wantInput:      0.006,  // 1000×3e-6=0.003 ×2
			wantOutput:     0.015,  // 500×15e-6=0.0075 ×2
			wantCacheWrite: 0.015,  // 2000×3.75e-6=0.0075 ×2
			wantCacheRead:  0.0018, // 3000×0.3e-6=0.0009 ×2
			wantTotal:      0.0378,
			wantActual:     0.0567, // 0.0378 × 1.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestBillingService()
			cost, err := svc.CalculateCostWithServiceTier(tt.model, tt.tokens, tt.rateMultiplier, tt.serviceTier)
			require.NoError(t, err)
			require.InDelta(t, tt.wantInput, cost.InputCost, 1e-10)
			require.InDelta(t, tt.wantOutput, cost.OutputCost, 1e-10)
			require.InDelta(t, tt.wantCacheWrite, cost.CacheCreationCost, 1e-10)
			require.InDelta(t, tt.wantCacheRead, cost.CacheReadCost, 1e-10)
			require.InDelta(t, tt.wantTotal, cost.TotalCost, 1e-10)
			require.InDelta(t, tt.wantActual, cost.ActualCost, 1e-10)
		})
	}
}

// TestBillingInvariant_ZeroAndNegativeRateMultiplier 锁定 I-2.4：
// rateMultiplier=0（免费账号）时 ActualCost=0 但 TotalCost 保留；
// 负数倍率按 0 处理（防缓存/迁移残留导致按 1x 误扣）。
func TestBillingInvariant_ZeroAndNegativeRateMultiplier(t *testing.T) {
	svc := newTestBillingService()
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheCreationTokens: 2000, CacheReadTokens: 3000}
	// claude-sonnet-4: 0.003 + 0.0075 + 2000×3.75e-6(=0.0075) + 3000×0.3e-6(=0.0009) = 0.0189
	const wantTotal = 0.0189

	for _, multiplier := range []float64{0, -2} {
		cost, err := svc.CalculateCost("claude-sonnet-4", tokens, multiplier)
		require.NoError(t, err)
		require.InDelta(t, wantTotal, cost.TotalCost, 1e-10, "multiplier=%v 时 TotalCost 应保留原始费用", multiplier)
		require.InDelta(t, 0.0, cost.ActualCost, 1e-10, "multiplier=%v 时 ActualCost 应为 0", multiplier)
	}
}
