package service

import (
	"context"

	"strings"

	"github.com/tidwall/gjson"
)

type apiKeyAuthCacheUserInvalidator interface {
	InvalidateAuthCacheByUserID(ctx context.Context, userID int64)
}

func calculatePrivateGroupCommissionCost(p *postUsageBillingParams) float64 {
	if p == nil || p.Cost == nil || !p.IsSubscriptionBill || p.APIKey == nil || p.APIKey.Group == nil {
		return 0
	}
	if !p.APIKey.Group.IsUserPrivateScope() || p.Cost.ActualCost <= 0 {
		return 0
	}
	rate := p.PrivateGroupCommissionRate
	if rate <= 0 {
		return 0
	}
	if rate > 1 {
		rate = 1
	}
	return p.Cost.ActualCost * rate
}
func extractFirstUserMessageTextFromRaw(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	messages := parseRawJSONView(raw)
	if !messages.IsArray() {
		return ""
	}
	var out string
	messages.ForEach(func(_, msg gjson.Result) bool {
		role := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
		if role != "" && role != "user" {
			return true
		}
		if content := msg.Get("content"); content.Exists() {
			out = extractTextFromContentRaw(content)
			return out == ""
		}
		if parts := msg.Get("parts"); parts.IsArray() {
			var builder strings.Builder
			parts.ForEach(func(_, part gjson.Result) bool {
				if text := part.Get("text").String(); text != "" {
					_, _ = builder.WriteString(text)
				}
				return true
			})
			out = builder.String()
			return out == ""
		}
		return true
	})
	return out
}

func extractReasoningTokensFromJSONBytes(body []byte) int {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return 0
	}
	paths := []string{
		"response.usage.output_tokens_details.reasoning_tokens",
		"usage.output_tokens_details.reasoning_tokens",
		"usage.completion_tokens_details.reasoning_tokens",
		"usage.reasoning_tokens",
	}
	for _, path := range paths {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Int() > 0 {
			return int(value.Int())
		}
	}
	return 0
}

func extractReasoningTokensFromUsageNode(usageNode gjson.Result) int {
	if !usageNode.Exists() {
		return 0
	}
	paths := []string{
		"output_tokens_details.reasoning_tokens",
		"completion_tokens_details.reasoning_tokens",
		"reasoning_tokens",
	}
	for _, path := range paths {
		if value := usageNode.Get(path); value.Exists() && value.Int() > 0 {
			return int(value.Int())
		}
	}
	return 0
}

func finalizeLegacyUsageBillingWallet(p *postUsageBillingParams, deps *billingDeps, result *UsageBillingApplyResult) {
	if p == nil || deps == nil || result == nil || p.User == nil {
		return
	}

	if result.PointsDeducted > 0 {
		if invalidator, ok := p.APIKeyService.(apiKeyAuthCacheUserInvalidator); ok {
			invalidator.InvalidateAuthCacheByUserID(context.Background(), p.User.ID)
		}
		if deps.billingCacheService != nil {
			_ = deps.billingCacheService.InvalidateUserBalance(context.Background(), p.User.ID)
		}
		return
	}

	if result.BalanceDeducted > 0 {
		if deps.billingCacheService != nil {
			_ = deps.billingCacheService.InvalidateUserBalance(context.Background(), p.User.ID)
		}
		return
	}

	if result.CommissionDeducted > 0 {
		if deps.billingCacheService != nil {
			_ = deps.billingCacheService.InvalidateUserBalance(context.Background(), p.User.ID)
		}
	}
}
func (s *GatewayService) isAccountAllowedForSchedulingRequest(ctx context.Context, account *Account) bool {
	if IsAccountVisibleToRequestUser(ctx, account) {
		return true
	}
	return isCarpoolSchedulingAccountAllowed(ctx, s.carpoolRepo, currentRequestGroupID(ctx), account)
}

type usageBillingWalletAdjuster interface {
	AdjustUsageBillingWallet(ctx context.Context, userID int64, amount float64, preferPoints bool, metadata map[string]any) (*UsageBillingApplyResult, error)
}
