package handler

import (
	"strings"
	"time"
	"unicode"

	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type botChannelSummaryResponse struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Items       []botChannelSummaryItem `json:"items"`
}

type botChannelSummaryItem struct {
	ID                   int64                        `json:"id"`
	Name                 string                       `json:"name"`
	Provider             string                       `json:"provider"`
	GroupName            string                       `json:"group_name"`
	PrimaryModel         string                       `json:"primary_model"`
	PrimaryStatus        string                       `json:"primary_status"`
	PrimaryLatencyMs     *int                         `json:"primary_latency_ms"`
	PrimaryPingLatencyMs *int                         `json:"primary_ping_latency_ms"`
	Availability7d       float64                      `json:"availability_7d"`
	CheckedAt            *time.Time                   `json:"checked_at,omitempty"`
	ExtraModels          []botChannelExtraModelStatus `json:"extra_models"`
	Pool                 *botChannelSharedPoolSummary `json:"pool,omitempty"`
}

type botChannelExtraModelStatus struct {
	Model     string `json:"model"`
	Status    string `json:"status"`
	LatencyMs *int   `json:"latency_ms"`
}

type botChannelSharedPoolSummary struct {
	GroupID                        int64                         `json:"group_id"`
	GroupName                      string                        `json:"group_name"`
	Platform                       string                        `json:"platform"`
	AccountCount                   int                           `json:"account_count"`
	ActiveAccountCount             int                           `json:"active_account_count"`
	SchedulableAccountCount        int                           `json:"schedulable_account_count"`
	RateLimitedAccountCount        int                           `json:"rate_limited_account_count"`
	ErrorAccountCount              int                           `json:"error_account_count"`
	DisabledAccountCount           int                           `json:"disabled_account_count"`
	ConcurrencyUsed                int                           `json:"concurrency_used"`
	ConcurrencyCapacity            int                           `json:"concurrency_capacity"`
	SchedulableConcurrencyCapacity int                           `json:"schedulable_concurrency_capacity"`
	FiveHour                       *botChannelUsageWindowSummary `json:"five_hour,omitempty"`
	Weekly                         *botChannelUsageWindowSummary `json:"weekly,omitempty"`
}

type botChannelUsageWindowSummary struct {
	AccountCount             int        `json:"account_count"`
	KnownAccountCount        int        `json:"known_account_count"`
	AverageUtilization       float64    `json:"average_utilization"`
	RemainingCapacityPercent float64    `json:"remaining_capacity_percent"`
	EstimatedSupportHours    *float64   `json:"estimated_support_hours,omitempty"`
	MinRemainingSeconds      *int       `json:"min_remaining_seconds,omitempty"`
	NextResetAt              *time.Time `json:"next_reset_at,omitempty"`
}

// BotSummary returns the enabled monitor status together with the matching
// public shared-pool capacity visible to the developer-token owner.
func (h *ChannelMonitorUserHandler) BotSummary(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if !h.featureEnabled(c) {
		response.Success(c, botChannelSummaryResponse{GeneratedAt: time.Now().UTC(), Items: []botChannelSummaryItem{}})
		return
	}
	if h.monitorService == nil || h.groupCapacityService == nil || h.accountService == nil {
		response.Error(c, 500, "Channel pool summary service is unavailable")
		return
	}

	monitors, err := h.monitorService.ListUserView(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	dashboard, err := h.accountService.GetQuotaPoolDashboard(c.Request.Context(), subject.UserID)
	if err != nil {
		response.Error(c, 500, "Failed to get shared pool quota summary")
		return
	}
	capacities, err := h.groupCapacityService.GetUserVisibleGroupCapacity(c.Request.Context(), subject.UserID)
	if err != nil {
		response.Error(c, 500, "Failed to get shared pool capacity summary")
		return
	}

	response.Success(c, buildBotChannelSummary(monitors, dashboard, capacities))
}

func buildBotChannelSummary(
	monitors []*service.UserMonitorView,
	dashboard *service.UserAccountQuotaPoolDashboard,
	capacities []service.GroupCapacitySummary,
) botChannelSummaryResponse {
	generatedAt := time.Now().UTC()
	quotaGroups := []service.AccountQuotaGroupSummary{}
	if dashboard != nil {
		generatedAt = dashboard.GeneratedAt
		quotaGroups = dashboard.Platform.GroupSummaries
	}
	capacityByGroupID := make(map[int64]service.GroupCapacitySummary, len(capacities))
	for _, capacity := range capacities {
		capacityByGroupID[capacity.GroupID] = capacity
	}

	items := make([]botChannelSummaryItem, 0, len(monitors))
	for _, monitor := range monitors {
		if monitor == nil {
			continue
		}
		item := botChannelSummaryItem{
			ID:                   monitor.ID,
			Name:                 monitor.Name,
			Provider:             monitor.Provider,
			GroupName:            monitor.GroupName,
			PrimaryModel:         monitor.PrimaryModel,
			PrimaryStatus:        monitor.PrimaryStatus,
			PrimaryLatencyMs:     monitor.PrimaryLatencyMs,
			PrimaryPingLatencyMs: monitor.PrimaryPingLatencyMs,
			Availability7d:       monitor.Availability7d,
			ExtraModels:          make([]botChannelExtraModelStatus, 0, len(monitor.ExtraModels)),
		}
		if len(monitor.Timeline) > 0 {
			checkedAt := monitor.Timeline[0].CheckedAt
			item.CheckedAt = &checkedAt
		}
		for _, extra := range monitor.ExtraModels {
			item.ExtraModels = append(item.ExtraModels, botChannelExtraModelStatus{
				Model: extra.Model, Status: extra.Status, LatencyMs: extra.LatencyMs,
			})
		}
		if quotaGroup := matchMonitorQuotaGroup(monitor, quotaGroups); quotaGroup != nil && quotaGroup.GroupID != nil {
			capacity := capacityByGroupID[*quotaGroup.GroupID]
			item.GroupName = quotaGroup.GroupName
			item.Pool = botSharedPoolSummary(*quotaGroup, capacity)
		}
		items = append(items, item)
	}

	return botChannelSummaryResponse{GeneratedAt: generatedAt, Items: items}
}

func matchMonitorQuotaGroup(monitor *service.UserMonitorView, groups []service.AccountQuotaGroupSummary) *service.AccountQuotaGroupSummary {
	if monitor == nil {
		return nil
	}
	candidates := []string{monitor.GroupName, monitor.Name}
	for _, candidate := range candidates {
		normalized := normalizeMonitorGroupName(candidate)
		if normalized == "" {
			continue
		}
		for index := range groups {
			group := &groups[index]
			if monitor.Provider != "" && group.Platform != "" && !strings.EqualFold(monitor.Provider, group.Platform) {
				continue
			}
			if normalizeMonitorGroupName(group.GroupName) == normalized {
				return group
			}
		}
	}
	return nil
}

func normalizeMonitorGroupName(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(value))
}

func botSharedPoolSummary(
	quota service.AccountQuotaGroupSummary,
	capacity service.GroupCapacitySummary,
) *botChannelSharedPoolSummary {
	groupID := int64(0)
	if quota.GroupID != nil {
		groupID = *quota.GroupID
	}
	return &botChannelSharedPoolSummary{
		GroupID:                        groupID,
		GroupName:                      quota.GroupName,
		Platform:                       quota.Platform,
		AccountCount:                   quota.AccountCount,
		ActiveAccountCount:             quota.ActiveAccountCount,
		SchedulableAccountCount:        quota.SchedulableAccountCount,
		RateLimitedAccountCount:        quota.RateLimitedAccountCount,
		ErrorAccountCount:              quota.ErrorAccountCount,
		DisabledAccountCount:           quota.DisabledAccountCount,
		ConcurrencyUsed:                capacity.ConcurrencyUsed,
		ConcurrencyCapacity:            quota.ConcurrencyCapacity,
		SchedulableConcurrencyCapacity: quota.SchedulableConcurrencyCapacity,
		FiveHour:                       botUsageWindow(quota.UsageWindows, "5h"),
		Weekly:                         botUsageWindow(quota.UsageWindows, "7d"),
	}
}

func botUsageWindow(windows []service.AccountUsageWindowSummary, wanted string) *botChannelUsageWindowSummary {
	for _, window := range windows {
		if window.Window != wanted {
			continue
		}
		return &botChannelUsageWindowSummary{
			AccountCount:             window.AccountCount,
			KnownAccountCount:        window.KnownAccountCount,
			AverageUtilization:       window.AverageUtilization,
			RemainingCapacityPercent: window.RemainingCapacityPercent,
			EstimatedSupportHours:    window.EstimatedSupportHours,
			MinRemainingSeconds:      window.MinRemainingSeconds,
			NextResetAt:              window.NextResetAt,
		}
	}
	return nil
}
