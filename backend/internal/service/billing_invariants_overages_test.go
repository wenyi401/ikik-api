//go:build unit

// TASK-003 overages 不变量测试（INVARIANTS.md I-2.5，PR3061 漂移点①）。
//
// overages（AI Credits 超量请求）由 accounts.extra["allow_overages"] 控制，仅
// antigravity 平台生效。本文件锁定：
//   - 开关解析的允许/拒绝语义（平台门控 + 字段缺失/类型错误按拒绝处理）
//   - 拒绝路径：上游 429 quota_exhausted 时不注入 enabledCreditTypes、不发起
//     credits 重试（开关关闭 / 积分已耗尽两种拒绝原因）
//
// 允许路径（注入 credits 并继续请求）已由 antigravity_credits_overages_test.go
// 覆盖。超额部分的"计价"没有独立分支：credits 重试成功后的响应走与普通请求
// 完全相同的 RecordUsage 计费链路（由本任务 I-2.6 端到端测试锁定金额），因此
// 这里只需锁定允许/拒绝语义。
package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBillingInvariant_OveragesFlagSemantics 锁定 allow_overages 开关解析语义。
func TestBillingInvariant_OveragesFlagSemantics(t *testing.T) {
	tests := []struct {
		name    string
		account Account
		want    bool
	}{
		{
			name:    "antigravity平台显式true允许",
			account: Account{Platform: PlatformAntigravity, Extra: map[string]any{"allow_overages": true}},
			want:    true,
		},
		{
			name:    "antigravity平台显式false拒绝",
			account: Account{Platform: PlatformAntigravity, Extra: map[string]any{"allow_overages": false}},
			want:    false,
		},
		{
			name:    "字段缺失默认拒绝",
			account: Account{Platform: PlatformAntigravity, Extra: map[string]any{}},
			want:    false,
		},
		{
			name:    "Extra为nil默认拒绝",
			account: Account{Platform: PlatformAntigravity},
			want:    false,
		},
		{
			name:    "非bool类型按拒绝处理",
			account: Account{Platform: PlatformAntigravity, Extra: map[string]any{"allow_overages": "true"}},
			want:    false,
		},
		{
			name:    "非antigravity平台即使为true也拒绝",
			account: Account{Platform: PlatformAnthropic, Extra: map[string]any{"allow_overages": true}},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.IsOveragesEnabled())
		})
	}
}

// TestBillingInvariant_OveragesDeniedNoCreditsInjection 锁定拒绝路径：
// 上游返回 429 quota_exhausted 时，若 overages 关闭或积分已耗尽，
// handleSmartRetry 不得注入 enabledCreditTypes 发起 credits 重试，
// 而是落入默认重试逻辑（smartRetryActionContinue）。
func TestBillingInvariant_OveragesDeniedNoCreditsInjection(t *testing.T) {
	quotaExhaustedBody := []byte(`{"error":{"status":"RESOURCE_EXHAUSTED","message":"QUOTA_EXHAUSTED"}}`)

	tests := []struct {
		name  string
		extra map[string]any
	}{
		{
			name:  "开关关闭时不注入credits",
			extra: map[string]any{}, // allow_overages 缺失 → 拒绝
		},
		{
			name: "积分已耗尽时不注入credits",
			extra: map[string]any{
				"allow_overages": true,
				modelRateLimitsKey: map[string]any{
					creditsExhaustedKey: map[string]any{
						"rate_limited_at":     time.Now().UTC().Format(time.RFC3339),
						"rate_limit_reset_at": time.Now().Add(5 * time.Hour).UTC().Format(time.RFC3339),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &mockSmartRetryUpstream{}
			account := &Account{
				ID:       201,
				Name:     "acc-201",
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Extra:    tt.extra,
			}
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewReader(quotaExhaustedBody)),
			}
			params := antigravityRetryLoopParams{
				ctx:            context.Background(),
				prefix:         "[bill-inv]",
				account:        account,
				accessToken:    "token",
				action:         "generateContent",
				body:           []byte(`{"model":"claude-sonnet-4-5","request":{}}`),
				httpUpstream:   upstream,
				accountRepo:    &stubAntigravityAccountRepo{},
				requestedModel: "claude-sonnet-4-5",
				handleError: func(ctx context.Context, prefix string, account *Account, statusCode int, headers http.Header, body []byte, requestedModel string, groupID int64, sessionHash string, isStickySession bool) *handleModelRateLimitResult {
					return nil
				},
			}

			svc := &AntigravityGatewayService{}
			result := svc.handleSmartRetry(params, resp, quotaExhaustedBody, "https://ag-1.test", 0, []string{"https://ag-1.test"})

			require.NotNil(t, result)
			require.Equal(t, smartRetryActionContinue, result.action, "拒绝 overages 后应落入默认重试逻辑")
			require.Empty(t, upstream.requestBodies, "不得发起 credits 注入重试请求")
		})
	}
}
