package handler

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"ikik-api/internal/pkg/apicompat"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/pkg/usagestats"
	"ikik-api/internal/service"
)

const (
	petToolAccountOverview = "get_my_account_overview"
	petToolAvailableGroups = "list_my_available_groups"
	petToolAPIKeys         = "list_my_api_keys"
	petToolUsageSummary    = "get_my_usage_summary"
	petToolMaxCalls        = 4
)

type petSupportUserReader interface {
	GetProfile(ctx context.Context, userID int64) (*service.User, error)
}

type petSupportAPIReader interface {
	GetAvailableGroups(ctx context.Context, userID int64) ([]service.Group, error)
	GetUserGroupRates(ctx context.Context, userID int64) (map[int64]float64, error)
	List(ctx context.Context, userID int64, params pagination.PaginationParams, filters service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error)
}

type petSupportUsageReader interface {
	GetBatchUserUsageStats(ctx context.Context, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.BatchUserUsageStats, error)
}

// petSupportToolRegistry is deliberately read-only. The authenticated user ID
// is supplied by the server and is never accepted from model-generated input.
type petSupportToolRegistry struct {
	users petSupportUserReader
	api   petSupportAPIReader
	usage petSupportUsageReader
}

func newPetSupportToolRegistry(users petSupportUserReader, api petSupportAPIReader, usage petSupportUsageReader) *petSupportToolRegistry {
	return &petSupportToolRegistry{users: users, api: api, usage: usage}
}

func (r *petSupportToolRegistry) Definitions() []apicompat.ChatTool {
	if r == nil {
		return nil
	}
	strict := false
	emptyObject := json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	return []apicompat.ChatTool{
		{
			Type: "function",
			Function: &apicompat.ChatFunction{
				Name:        petToolAccountOverview,
				Description: "读取当前登录用户自己的账户状态、可用余额、积分、并发和 RPM 限制。不得用于查询其他用户。",
				Parameters:  emptyObject,
				Strict:      &strict,
			},
		},
		{
			Type: "function",
			Function: &apicompat.ChatFunction{
				Name:        petToolAvailableGroups,
				Description: "列出当前登录用户可使用的分组、平台、实际倍率、订阅限制以及生图能力。",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":50,"description":"最多返回多少个分组，默认 20"}},"additionalProperties":false}`),
				Strict:      &strict,
			},
		},
		{
			Type: "function",
			Function: &apicompat.ChatFunction{
				Name:        petToolAPIKeys,
				Description: "列出当前登录用户自己的 API 密钥状态、分组、额度、限流窗口和有效期。永远不返回密钥明文。",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":20,"description":"最多返回多少个密钥，默认 10"}},"additionalProperties":false}`),
				Strict:      &strict,
			},
		},
		{
			Type: "function",
			Function: &apicompat.ChatFunction{
				Name:        petToolUsageSummary,
				Description: "查询当前登录用户今日以及最近若干天的实际扣费，并按平台汇总。",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"days":{"type":"integer","minimum":1,"maximum":30,"description":"统计最近多少天，默认 7"}},"additionalProperties":false}`),
				Strict:      &strict,
			},
		},
	}
}

func (r *petSupportToolRegistry) Execute(ctx context.Context, userID int64, call apicompat.ChatToolCall) string {
	if r == nil || userID <= 0 {
		return petToolError("tool_unavailable")
	}
	switch call.Function.Name {
	case petToolAccountOverview:
		if !petToolArgumentsAllowed(call.Function.Arguments) {
			return petToolError("invalid_arguments")
		}
		return r.accountOverview(ctx, userID)
	case petToolAvailableGroups:
		args, ok := petToolIntegerArguments(call.Function.Arguments, "limit", 20, 1, 50)
		if !ok {
			return petToolError("invalid_arguments")
		}
		return r.availableGroups(ctx, userID, args)
	case petToolAPIKeys:
		args, ok := petToolIntegerArguments(call.Function.Arguments, "limit", 10, 1, 20)
		if !ok {
			return petToolError("invalid_arguments")
		}
		return r.apiKeys(ctx, userID, args)
	case petToolUsageSummary:
		args, ok := petToolIntegerArguments(call.Function.Arguments, "days", 7, 1, 30)
		if !ok {
			return petToolError("invalid_arguments")
		}
		return r.usageSummary(ctx, userID, args)
	default:
		return petToolError("tool_not_allowed")
	}
}

func (r *petSupportToolRegistry) accountOverview(ctx context.Context, userID int64) string {
	if r.users == nil {
		return petToolError("tool_unavailable")
	}
	user, err := r.users.GetProfile(ctx, userID)
	if err != nil || user == nil {
		return petToolError("query_failed")
	}
	return petToolSuccess(map[string]any{
		"status":                user.Status,
		"balance_usd":           user.Balance,
		"points_balance":        user.PointsBalance,
		"prefer_points_billing": user.PreferPointsBilling,
		"concurrency_limit":     user.Concurrency,
		"rpm_limit":             user.RPMLimit,
		"last_active_at":        petToolTime(user.LastActiveAt),
	})
}

func (r *petSupportToolRegistry) availableGroups(ctx context.Context, userID int64, limit int) string {
	if r.api == nil {
		return petToolError("tool_unavailable")
	}
	groups, err := r.api.GetAvailableGroups(ctx, userID)
	if err != nil {
		return petToolError("query_failed")
	}
	rates, err := r.api.GetUserGroupRates(ctx, userID)
	if err != nil {
		return petToolError("query_failed")
	}
	items := make([]map[string]any, 0, min(limit, len(groups)))
	for i := range groups {
		if len(items) >= limit {
			break
		}
		group := &groups[i]
		rate := group.RateMultiplier
		if custom, ok := rates[group.ID]; ok {
			rate = custom
		}
		items = append(items, map[string]any{
			"id":                        group.ID,
			"name":                      group.Name,
			"platform":                  group.Platform,
			"billing_type":              group.SubscriptionType,
			"effective_rate_multiplier": rate,
			"daily_limit_usd":           group.DailyLimitUSD,
			"weekly_limit_usd":          group.WeeklyLimitUSD,
			"monthly_limit_usd":         group.MonthlyLimitUSD,
			"image_generation_enabled":  group.AllowImageGeneration,
			"claude_code_only":          group.ClaudeCodeOnly,
		})
	}
	return petToolSuccess(map[string]any{
		"groups":    items,
		"returned":  len(items),
		"total":     len(groups),
		"truncated": len(items) < len(groups),
	})
}

func (r *petSupportToolRegistry) apiKeys(ctx context.Context, userID int64, limit int) string {
	if r.api == nil {
		return petToolError("tool_unavailable")
	}
	keys, _, err := r.api.List(ctx, userID, pagination.PaginationParams{
		Page: 1, PageSize: 50, SortBy: "id", SortOrder: pagination.SortOrderDesc,
	}, service.APIKeyListFilters{})
	if err != nil {
		return petToolError("query_failed")
	}
	items := make([]map[string]any, 0, min(limit, len(keys)))
	for i := range keys {
		if len(items) >= limit {
			break
		}
		key := &keys[i]
		if strings.HasPrefix(key.Name, playgroundAPIKeyPrefix) {
			continue
		}
		status := key.Status
		if key.IsExpired() {
			status = service.StatusAPIKeyExpired
		} else if key.IsQuotaExhausted() {
			status = service.StatusAPIKeyQuotaExhausted
		}
		groupName := ""
		if key.Group != nil {
			groupName = key.Group.Name
		}
		items = append(items, map[string]any{
			"id":                  key.ID,
			"name":                key.Name,
			"group_id":            key.GroupID,
			"group_name":          groupName,
			"status":              status,
			"quota_unlimited":     key.Quota <= 0,
			"quota_usd":           key.Quota,
			"quota_used_usd":      key.QuotaUsed,
			"quota_remaining_usd": key.GetQuotaRemaining(),
			"usage_5h_usd":        key.EffectiveUsage5h(),
			"limit_5h_usd":        key.RateLimit5h,
			"usage_1d_usd":        key.EffectiveUsage1d(),
			"limit_1d_usd":        key.RateLimit1d,
			"usage_7d_usd":        key.EffectiveUsage7d(),
			"limit_7d_usd":        key.RateLimit7d,
			"expires_at":          petToolTime(key.ExpiresAt),
			"last_used_at":        petToolTime(key.LastUsedAt),
		})
	}
	return petToolSuccess(map[string]any{
		"api_keys": items,
		"returned": len(items),
	})
}

func (r *petSupportToolRegistry) usageSummary(ctx context.Context, userID int64, days int) string {
	if r.usage == nil {
		return petToolError("tool_unavailable")
	}
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	result, err := r.usage.GetBatchUserUsageStats(ctx, []int64{userID}, start, end)
	if err != nil {
		return petToolError("query_failed")
	}
	stats := result[userID]
	if stats == nil {
		stats = &usagestats.BatchUserUsageStats{UserID: userID}
	}
	return petToolSuccess(map[string]any{
		"period_days":             days,
		"period_start":            start.Format(time.RFC3339),
		"period_end":              end.Format(time.RFC3339),
		"today_charged_cost_usd":  stats.TodayActualCost,
		"period_charged_cost_usd": stats.TotalActualCost,
		"by_platform":             stats.ByPlatform,
	})
}

func petToolArgumentsAllowed(arguments string) bool {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" || arguments == "null" {
		return true
	}
	var raw map[string]json.RawMessage
	return json.Unmarshal([]byte(arguments), &raw) == nil && raw != nil && len(raw) == 0
}

func petToolIntegerArguments(arguments, key string, fallback, minimum, maximum int) (int, bool) {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" || arguments == "null" {
		return fallback, true
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(arguments), &raw); err != nil || len(raw) > 1 {
		return 0, false
	}
	for name := range raw {
		if name != key {
			return 0, false
		}
	}
	value := fallback
	if encoded, ok := raw[key]; ok {
		if err := json.Unmarshal(encoded, &value); err != nil {
			return 0, false
		}
	}
	if value < minimum || value > maximum {
		return 0, false
	}
	return value, true
}

func petToolSuccess(data any) string {
	return petToolJSON(map[string]any{"ok": true, "data": data})
}

func petToolError(code string) string {
	return petToolJSON(map[string]any{"ok": false, "error": code})
}

func petToolJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `{"ok":false,"error":"encode_failed"}`
	}
	return string(encoded)
}

func petToolTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339)
}
