//go:build unit

package handler

import (
	"testing"
	"time"

	"ikik-api/internal/service"

	"github.com/stretchr/testify/require"
)

func TestBuildBotChannelSummaryMatchesMonitorNameIgnoringWhitespace(t *testing.T) {
	groupID := int64(16)
	latency := 1043
	ping := 175
	resetSeconds := 7200
	checkedAt := time.Date(2026, 8, 7, 1, 29, 0, 0, time.UTC)
	generatedAt := checkedAt.Add(time.Minute)
	dashboard := &service.UserAccountQuotaPoolDashboard{
		GeneratedAt: generatedAt,
		Platform: service.AccountQuotaDashboard{GroupSummaries: []service.AccountQuotaGroupSummary{
			{
				GroupID:                        &groupID,
				GroupName:                      "gpt pro 共享号池",
				Platform:                       service.PlatformOpenAI,
				AccountCount:                   8,
				ActiveAccountCount:             7,
				SchedulableAccountCount:        6,
				RateLimitedAccountCount:        1,
				ConcurrencyCapacity:            24,
				SchedulableConcurrencyCapacity: 18,
				UsageWindows: []service.AccountUsageWindowSummary{
					{Window: "5h", AccountCount: 6, KnownAccountCount: 5, AverageUtilization: 35, RemainingCapacityPercent: 325, MinRemainingSeconds: &resetSeconds},
					{Window: "7d", AccountCount: 6, KnownAccountCount: 5, AverageUtilization: 42, RemainingCapacityPercent: 290},
				},
			},
		}},
	}
	monitors := []*service.UserMonitorView{
		{
			ID: 1, Name: "gpt pro共享号池", Provider: service.PlatformOpenAI,
			PrimaryModel: "gpt-5.4-mini", PrimaryStatus: "operational",
			PrimaryLatencyMs: &latency, PrimaryPingLatencyMs: &ping, Availability7d: 97.69,
			Timeline: []service.UserMonitorTimelinePoint{{Status: "operational", CheckedAt: checkedAt}},
		},
	}

	got := buildBotChannelSummary(monitors, dashboard, []service.GroupCapacitySummary{
		{GroupID: groupID, ConcurrencyUsed: 4, ConcurrencyMax: 18},
	})

	require.Equal(t, generatedAt, got.GeneratedAt)
	require.Len(t, got.Items, 1)
	require.Equal(t, "gpt pro 共享号池", got.Items[0].GroupName)
	require.NotNil(t, got.Items[0].CheckedAt)
	require.NotNil(t, got.Items[0].Pool)
	require.Equal(t, 6, got.Items[0].Pool.SchedulableAccountCount)
	require.Equal(t, 4, got.Items[0].Pool.ConcurrencyUsed)
	require.Equal(t, 18, got.Items[0].Pool.SchedulableConcurrencyCapacity)
	require.Equal(t, 35.0, got.Items[0].Pool.FiveHour.AverageUtilization)
	require.Equal(t, 42.0, got.Items[0].Pool.Weekly.AverageUtilization)
}

func TestBuildBotChannelSummaryDoesNotAttachDifferentPlatformGroup(t *testing.T) {
	groupID := int64(1)
	monitor := &service.UserMonitorView{Name: "共享号池", Provider: service.PlatformOpenAI}
	dashboard := &service.UserAccountQuotaPoolDashboard{
		Platform: service.AccountQuotaDashboard{GroupSummaries: []service.AccountQuotaGroupSummary{
			{GroupID: &groupID, GroupName: "共享号池", Platform: service.PlatformGrok},
		}},
	}

	got := buildBotChannelSummary([]*service.UserMonitorView{monitor}, dashboard, nil)

	require.Len(t, got.Items, 1)
	require.Nil(t, got.Items[0].Pool)
}
