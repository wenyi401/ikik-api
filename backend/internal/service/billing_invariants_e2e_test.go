//go:build unit

// TASK-003 计费/配额端到端不变量测试（INVARIANTS.md ②③）。
//
// 本文件通过 GatewayService.RecordUsage 走完整后扣链路，锁定：
//   - I-2.6: 完整 usage 输入 → 最终扣费金额 + 余额扣减/订阅用量增量二选一分支
//     （含 image 计价路径与 ImageOutputTokens 独立计价，呼应 I-2.3 端到端联动）
//   - I-2.2: 5m/1h 两档缓存写入的端到端金额
//   - I-3.3: API Key quota_used 增量金额（统一命令路径 + legacy 直写路径）
//   - I-3.4: Account 级配额增量金额 = TotalCost × AccountRateMultiplier
//     （注意：用 TotalCost 而非 ActualCost，分组倍率不影响账号配额消耗）
//
// 断言对象为 UsageBillingRepository.Apply 收到的 UsageBillingCommand（生产原子
// 扣费路径的唯一输入）与 UsageLog 金额字段。所有期望值均人工核算（算式见注释）。
package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/config"
)

// billInvAccountRepoStub 捕获 legacy 路径下的账号配额增量。
type billInvAccountRepoStub struct {
	AccountRepository

	quotaIncrCalls  int
	lastQuotaAmount float64
}

func (s *billInvAccountRepoStub) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error {
	s.quotaIncrCalls++
	s.lastQuotaAmount = amount
	return nil
}

// billInvSubRepoStub 捕获 legacy 路径下的订阅用量增量金额。
type billInvSubRepoStub struct {
	UserSubscriptionRepository

	incrementCalls int
	lastCost       float64
}

func (s *billInvSubRepoStub) IncrementUsage(ctx context.Context, id int64, costUSD float64) error {
	s.incrementCalls++
	s.lastCost = costUSD
	return nil
}

func billInvF64Ptr(v float64) *float64 { return &v }

// billInvNewGatewayService 构造带可注入 BillingService / 计费仓储的网关服务。
func billInvNewGatewayService(
	billingSvc *BillingService,
	accountRepo AccountRepository,
	usageRepo UsageLogRepository,
	billingRepo UsageBillingRepository,
	userRepo UserRepository,
	subRepo UserSubscriptionRepository,
) *GatewayService {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.0
	if billingSvc == nil {
		billingSvc = NewBillingService(cfg, nil)
	}
	svc := NewGatewayService(
		accountRepo,
		nil, // accountSharePolicyRepo
		nil, // groupRepo
		usageRepo,
		billingRepo,
		userRepo,
		subRepo,
		nil, // userGroupRateRepo
		nil, // cache
		cfg,
		nil, // schedulerSnapshot
		nil, // concurrencyService
		billingSvc,
		nil, // rateLimitService
		&BillingCacheService{},
		nil, // identityService
		nil, // httpUpstream
		&DeferredService{},
		nil, // claudeTokenProvider
		nil, // sessionLimitCache
		nil, // rpmCache
		nil, // digestStore
		nil, // settingService
		nil, // tlsFPProfileService
		nil, // channelService
		nil, // resolver
		nil, // balanceNotifyService
	)
	return svc
}

// TestBillingInvariant_EndToEndUsageBillingCommand 表驱动锁定 I-2.6 / I-3.3 / I-3.4：
// 每行 = (定价/分组倍率/账号倍率/usage 输入) → (UsageBillingCommand 各项金额 + UsageLog 金额)。
func TestBillingInvariant_EndToEndUsageBillingCommand(t *testing.T) {
	groupID := int64(11)
	type expected struct {
		balanceCost         float64
		subscriptionCost    float64
		apiKeyQuotaCost     float64
		apiKeyRateLimitCost float64
		accountQuotaCost    float64
		subscriptionID      *int64
		logTotal            float64
		logActual           float64
		billingType         int8
	}

	// 公共 usage：claude-sonnet-4 fallback 价（in $3/MTok, out $15/MTok,
	// cache_write $3.75/MTok, cache_read $0.3/MTok）。
	// TotalCost = 1000×3e-6 + 500×15e-6 + 2000×3.75e-6 + 3000×0.3e-6
	//           = 0.003 + 0.0075 + 0.0075 + 0.0009 = 0.0189
	fourDimUsage := ClaudeUsage{
		InputTokens:              1000,
		OutputTokens:             500,
		CacheCreationInputTokens: 2000,
		CacheReadInputTokens:     3000,
	}

	tests := []struct {
		name        string
		pricingData map[string]*LiteLLMModelPricing // 可选：动态定价
		model       string
		usage       ClaudeUsage
		imageCount  int
		imageSize   string
		group       *Group
		account     *Account
		apiKeyQuota float64
		rateLimit5h float64
		sub         *UserSubscription
		want        expected
	}{
		{
			name:    "余额模式_四维token组合计价",
			model:   "claude-sonnet-4",
			usage:   fourDimUsage,
			group:   &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1.0},
			account: &Account{ID: 701, Type: AccountTypeOAuth},
			// API Key Quota=100 → quota 增量 = ActualCost
			apiKeyQuota: 100,
			want: expected{
				balanceCost:     0.0189, // ActualCost = 0.0189 × 1.0
				apiKeyQuotaCost: 0.0189, // I-3.3: = ActualCost
				logTotal:        0.0189,
				logActual:       0.0189,
				billingType:     BillingTypeBalance,
			},
		},
		{
			name:        "余额模式_分组倍率2x只影响Actual不影响Total",
			model:       "claude-sonnet-4",
			usage:       fourDimUsage,
			group:       &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 2.0},
			account:     &Account{ID: 701, Type: AccountTypeOAuth},
			apiKeyQuota: 100,
			want: expected{
				balanceCost:     0.0378, // 0.0189 × 2.0
				apiKeyQuotaCost: 0.0378, // I-3.3: 跟随 ActualCost
				logTotal:        0.0189, // TotalCost 不含分组倍率
				logActual:       0.0378,
				billingType:     BillingTypeBalance,
			},
		},
		{
			name:  "订阅模式_订阅用量增量替代余额扣减",
			model: "claude-sonnet-4",
			usage: fourDimUsage,
			group: &Group{
				ID: groupID, Platform: PlatformAnthropic,
				RateMultiplier: 1.5, SubscriptionType: SubscriptionTypeSubscription,
			},
			account: &Account{ID: 701, Type: AccountTypeOAuth},
			sub:     &UserSubscription{ID: 42},
			want: expected{
				balanceCost:      0,       // 二选一：订阅模式不扣余额
				subscriptionCost: 0.02835, // 0.0189 × 1.5
				subscriptionID:   i64p(42),
				logTotal:         0.0189,
				logActual:        0.02835,
				billingType:      BillingTypeSubscription,
			},
		},
		{
			name:  "订阅模式_免费分组倍率0时订阅增量为0但Total保留",
			model: "claude-sonnet-4",
			usage: fourDimUsage,
			group: &Group{
				ID: groupID, Platform: PlatformAnthropic,
				RateMultiplier: 0, SubscriptionType: SubscriptionTypeSubscription,
			},
			account: &Account{ID: 701, Type: AccountTypeOAuth},
			sub:     &UserSubscription{ID: 42},
			want: expected{
				balanceCost:      0,
				subscriptionCost: 0,        // ActualCost = 0
				subscriptionID:   i64p(42), // TotalCost > 0 仍记录订阅归属
				logTotal:         0.0189,
				logActual:        0,
				billingType:      BillingTypeSubscription,
			},
		},
		{
			name:  "账号配额增量用TotalCost乘账号倍率而非ActualCost",
			model: "claude-sonnet-4",
			usage: fourDimUsage,
			group: &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 2.0},
			account: &Account{
				ID: 702, Type: AccountTypeAPIKey,
				Extra:          map[string]any{"quota_limit": 100.0},
				RateMultiplier: billInvF64Ptr(0.5),
			},
			want: expected{
				balanceCost:      0.0378,  // 余额按分组倍率 2.0
				accountQuotaCost: 0.00945, // I-3.4: 0.0189(Total) × 0.5(账号倍率)
				logTotal:         0.0189,
				logActual:        0.0378,
				billingType:      BillingTypeBalance,
			},
		},
		{
			name:  "OAuth账号不计账号配额增量",
			model: "claude-sonnet-4",
			usage: fourDimUsage,
			group: &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1.0},
			account: &Account{
				ID: 703, Type: AccountTypeOAuth,
				Extra: map[string]any{"quota_limit": 100.0},
			},
			want: expected{
				balanceCost:      0.0189,
				accountQuotaCost: 0, // 仅 apikey/bedrock 账号计配额
				logTotal:         0.0189,
				logActual:        0.0189,
				billingType:      BillingTypeBalance,
			},
		},
		{
			name:        "APIKey无限额时quota增量为0",
			model:       "claude-sonnet-4",
			usage:       fourDimUsage,
			group:       &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1.0},
			account:     &Account{ID: 701, Type: AccountTypeOAuth},
			apiKeyQuota: 0, // Quota=0 表示不限额
			want: expected{
				balanceCost:     0.0189,
				apiKeyQuotaCost: 0,
				logTotal:        0.0189,
				logActual:       0.0189,
				billingType:     BillingTypeBalance,
			},
		},
		{
			name:        "APIKey限速窗口增量等于ActualCost",
			model:       "claude-sonnet-4",
			usage:       fourDimUsage,
			group:       &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1.0},
			account:     &Account{ID: 701, Type: AccountTypeOAuth},
			rateLimit5h: 10,
			want: expected{
				balanceCost:         0.0189,
				apiKeyRateLimitCost: 0.0189,
				logTotal:            0.0189,
				logActual:           0.0189,
				billingType:         BillingTypeBalance,
			},
		},
		{
			name:       "图片按张计价_分组2K价格",
			model:      "gemini-3-pro-image",
			imageCount: 2,
			imageSize:  "2K",
			group: &Group{
				ID: groupID, Platform: PlatformGemini, RateMultiplier: 1.0,
				ImagePrice2K: billInvF64Ptr(0.19),
			},
			account: &Account{ID: 701, Type: AccountTypeOAuth},
			want: expected{
				balanceCost: 0.38, // 2 × $0.19
				logTotal:    0.38,
				logActual:   0.38,
				billingType: BillingTypeBalance,
			},
		},
		{
			name:       "图片独立倍率只影响Actual",
			model:      "gemini-3-pro-image",
			imageCount: 2,
			imageSize:  "2K",
			group: &Group{
				ID: groupID, Platform: PlatformGemini, RateMultiplier: 1.0,
				ImagePrice2K:         billInvF64Ptr(0.19),
				ImageRateIndependent: true,
				ImageRateMultiplier:  2.0,
			},
			account: &Account{ID: 701, Type: AccountTypeOAuth},
			want: expected{
				balanceCost: 0.76, // 0.38 × 2.0（图片独立倍率）
				logTotal:    0.38,
				logActual:   0.76,
				billingType: BillingTypeBalance,
			},
		},
		{
			name: "ImageOutputTokens按独立单价从输出中拆分计价",
			pricingData: map[string]*LiteLLMModelPricing{
				"img-token-model": {
					InputCostPerToken:       3e-6,
					OutputCostPerToken:      15e-6,
					OutputCostPerImageToken: 30e-6,
				},
			},
			model: "img-token-model",
			usage: ClaudeUsage{
				InputTokens:       100,
				OutputTokens:      200, // 其中 50 为图片输出 token
				ImageOutputTokens: 50,
			},
			group:   &Group{ID: groupID, Platform: PlatformGemini, RateMultiplier: 1.0},
			account: &Account{ID: 701, Type: AccountTypeOAuth},
			want: expected{
				// input: 100×3e-6 = 0.0003
				// 文本输出: (200-50)×15e-6 = 0.00225
				// 图片输出: 50×30e-6 = 0.0015
				// Total = 0.00405
				balanceCost: 0.00405,
				logTotal:    0.00405,
				logActual:   0.00405,
				billingType: BillingTypeBalance,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			var billingSvc *BillingService
			if tt.pricingData != nil {
				cfg := &config.Config{}
				billingSvc = NewBillingService(cfg, &PricingService{pricingData: tt.pricingData})
			}
			svc := billInvNewGatewayService(billingSvc, nil, usageRepo, billingRepo,
				&openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
			quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}

			err := svc.RecordUsage(context.Background(), &RecordUsageInput{
				Result: &ForwardResult{
					RequestID:  "bill-inv-" + tt.name,
					Usage:      tt.usage,
					Model:      tt.model,
					ImageCount: tt.imageCount,
					ImageSize:  tt.imageSize,
					Duration:   time.Second,
				},
				APIKey: &APIKey{
					ID:          501,
					Quota:       tt.apiKeyQuota,
					RateLimit5h: tt.rateLimit5h,
					GroupID:     i64p(tt.group.ID),
					Group:       tt.group,
				},
				User:          &User{ID: 601},
				Account:       tt.account,
				Subscription:  tt.sub,
				APIKeyService: quotaSvc,
			})
			require.NoError(t, err)

			cmd := billingRepo.lastCmd
			require.NotNil(t, cmd, "应走统一计费命令路径")
			require.InDelta(t, tt.want.balanceCost, cmd.BalanceCost, 1e-10, "BalanceCost")
			require.InDelta(t, tt.want.subscriptionCost, cmd.SubscriptionCost, 1e-10, "SubscriptionCost")
			require.InDelta(t, tt.want.apiKeyQuotaCost, cmd.APIKeyQuotaCost, 1e-10, "APIKeyQuotaCost")
			require.InDelta(t, tt.want.apiKeyRateLimitCost, cmd.APIKeyRateLimitCost, 1e-10, "APIKeyRateLimitCost")
			require.InDelta(t, tt.want.accountQuotaCost, cmd.AccountQuotaCost, 1e-10, "AccountQuotaCost")
			if tt.want.subscriptionID != nil {
				require.NotNil(t, cmd.SubscriptionID)
				require.Equal(t, *tt.want.subscriptionID, *cmd.SubscriptionID)
			} else {
				require.Nil(t, cmd.SubscriptionID)
			}

			log := usageRepo.lastLog
			require.NotNil(t, log)
			require.InDelta(t, tt.want.logTotal, log.TotalCost, 1e-10, "UsageLog.TotalCost")
			require.InDelta(t, tt.want.logActual, log.ActualCost, 1e-10, "UsageLog.ActualCost")
			require.Equal(t, tt.want.billingType, log.BillingType)
		})
	}
}

// TestBillingInvariant_CacheTier5mVs1hEndToEnd 锁定 I-2.2 端到端：上游返回的
// 5m/1h ephemeral 明细流经 RecordUsage 后按两档单价分别计费（PR3061 漂移点②）。
func TestBillingInvariant_CacheTier5mVs1hEndToEnd(t *testing.T) {
	cfg := &config.Config{}
	billingSvc := NewBillingService(cfg, &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"claude-sonnet-4": {
				InputCostPerToken:                   3e-6,
				OutputCostPerToken:                  15e-6,
				CacheCreationInputTokenCost:         3.75e-6, // 5m 档
				CacheCreationInputTokenCostAbove1hr: 6e-6,    // 1h 档（> 5m → 启用两档）
				CacheReadInputTokenCost:             0.3e-6,
			},
		},
	})
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := billInvNewGatewayService(billingSvc, nil, usageRepo, billingRepo,
		&openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	groupID := int64(11)
	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "bill-inv-cache-tier-e2e",
			Usage: ClaudeUsage{
				InputTokens:              1000,
				OutputTokens:             500,
				CacheCreationInputTokens: 12000, // 总量 = 5m + 1h
				CacheCreation5mTokens:    8000,
				CacheCreation1hTokens:    4000,
				CacheReadInputTokens:     2000,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 501, GroupID: i64p(groupID), Group: &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1.0}},
		User:    &User{ID: 601},
		Account: &Account{ID: 701, Type: AccountTypeOAuth},
	})
	require.NoError(t, err)

	// cache_creation = 8000×3.75e-6 + 4000×6e-6 = 0.03 + 0.024 = 0.054
	// Total = 1000×3e-6 + 500×15e-6 + 0.054 + 2000×0.3e-6
	//       = 0.003 + 0.0075 + 0.054 + 0.0006 = 0.0651
	log := usageRepo.lastLog
	require.NotNil(t, log)
	require.InDelta(t, 0.054, log.CacheCreationCost, 1e-10)
	require.InDelta(t, 0.0651, log.TotalCost, 1e-10)
	require.InDelta(t, 0.0651, log.ActualCost, 1e-10)

	require.NotNil(t, billingRepo.lastCmd)
	require.InDelta(t, 0.0651, billingRepo.lastCmd.BalanceCost, 1e-10)
}

// TestBillingInvariant_LegacyPathIncrements 锁定 legacy 兜底路径（统一计费仓储
// 不可用时）的各级增量金额与生产命令路径一致：
//   - 余额扣减 = ActualCost（I-2.6）
//   - API Key quota_used 增量 = ActualCost（I-3.3）
//   - 账号配额增量 = TotalCost × AccountRateMultiplier（I-3.4）
//   - 订阅模式：IncrementUsage(ActualCost)，不扣余额（I-2.6 二选一）
func TestBillingInvariant_LegacyPathIncrements(t *testing.T) {
	groupID := int64(11)
	fourDimUsage := ClaudeUsage{
		InputTokens:              1000,
		OutputTokens:             500,
		CacheCreationInputTokens: 2000,
		CacheReadInputTokens:     3000,
	}

	t.Run("余额模式各级增量", func(t *testing.T) {
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		userRepo := &openAIRecordUsageUserRepoStub{}
		subRepo := &billInvSubRepoStub{}
		accountRepo := &billInvAccountRepoStub{}
		quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}
		// billingRepo == nil → applyUsageBilling 回退 postUsageBilling 直写
		svc := billInvNewGatewayService(nil, accountRepo, usageRepo, nil, userRepo, subRepo)

		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result: &ForwardResult{
				RequestID: "bill-inv-legacy-balance",
				Usage:     fourDimUsage,
				Model:     "claude-sonnet-4",
				Duration:  time.Second,
			},
			APIKey: &APIKey{
				ID: 501, Quota: 100,
				GroupID: i64p(groupID),
				Group:   &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 2.0},
			},
			User: &User{ID: 601},
			Account: &Account{
				ID: 702, Type: AccountTypeAPIKey,
				Extra:          map[string]any{"quota_limit": 100.0},
				RateMultiplier: billInvF64Ptr(0.5),
			},
			APIKeyService: quotaSvc,
		})
		require.NoError(t, err)

		// ActualCost = 0.0189 × 2.0 = 0.0378
		require.Equal(t, 1, userRepo.deductCalls)
		require.InDelta(t, 0.0378, userRepo.lastAmount, 1e-10, "余额扣减 = ActualCost")
		require.Equal(t, 1, quotaSvc.quotaCalls)
		require.InDelta(t, 0.0378, quotaSvc.lastAmount, 1e-10, "API Key quota 增量 = ActualCost")
		// 账号配额 = TotalCost(0.0189) × 账号倍率(0.5) = 0.00945
		require.Equal(t, 1, accountRepo.quotaIncrCalls)
		require.InDelta(t, 0.00945, accountRepo.lastQuotaAmount, 1e-10, "账号配额增量 = Total × 账号倍率")
		// 余额模式不应触发订阅增量
		require.Equal(t, 0, subRepo.incrementCalls)
	})

	t.Run("订阅模式增量替代余额扣减", func(t *testing.T) {
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		userRepo := &openAIRecordUsageUserRepoStub{}
		subRepo := &billInvSubRepoStub{}
		svc := billInvNewGatewayService(nil, &billInvAccountRepoStub{}, usageRepo, nil, userRepo, subRepo)

		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result: &ForwardResult{
				RequestID: "bill-inv-legacy-subscription",
				Usage:     fourDimUsage,
				Model:     "claude-sonnet-4",
				Duration:  time.Second,
			},
			APIKey: &APIKey{
				ID:      501,
				GroupID: i64p(groupID),
				Group: &Group{
					ID: groupID, Platform: PlatformAnthropic,
					RateMultiplier: 1.5, SubscriptionType: SubscriptionTypeSubscription,
				},
			},
			User:         &User{ID: 601},
			Account:      &Account{ID: 701, Type: AccountTypeOAuth},
			Subscription: &UserSubscription{ID: 42},
		})
		require.NoError(t, err)

		// 订阅增量 = ActualCost = 0.0189 × 1.5 = 0.02835；余额不扣
		require.Equal(t, 1, subRepo.incrementCalls)
		require.InDelta(t, 0.02835, subRepo.lastCost, 1e-10, "订阅用量增量 = ActualCost")
		require.Equal(t, 0, userRepo.deductCalls, "订阅模式不应扣余额（二选一）")
	})
}
