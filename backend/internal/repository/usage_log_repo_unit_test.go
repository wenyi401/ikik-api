//go:build unit

package repository

import (
	"strings"
	"testing"
	"time"

	"ikik-api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSafeDateFormat(t *testing.T) {
	tests := []struct {
		name        string
		granularity string
		expected    string
	}{
		// 合法值
		{"hour", "hour", "YYYY-MM-DD HH24:00"},
		{"day", "day", "YYYY-MM-DD"},
		{"week", "week", "IYYY-IW"},
		{"month", "month", "YYYY-MM"},

		// 非法值回退到默认
		{"空字符串", "", "YYYY-MM-DD"},
		{"未知粒度 year", "year", "YYYY-MM-DD"},
		{"未知粒度 minute", "minute", "YYYY-MM-DD"},

		// 恶意字符串
		{"SQL 注入尝试", "'; DROP TABLE users; --", "YYYY-MM-DD"},
		{"带引号", "day'", "YYYY-MM-DD"},
		{"带括号", "day)", "YYYY-MM-DD"},
		{"Unicode", "日", "YYYY-MM-DD"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := safeDateFormat(tc.granularity)
			require.Equal(t, tc.expected, got, "safeDateFormat(%q)", tc.granularity)
		})
	}
}

func TestBuildUsageLogBatchInsertQuery_UsesConflictDoNothing(t *testing.T) {
	log := &service.UsageLog{
		UserID:       1,
		APIKeyID:     2,
		AccountID:    3,
		RequestID:    "req-batch-no-update",
		Model:        "gpt-5",
		InputTokens:  10,
		OutputTokens: 5,
		TotalCost:    1.2,
		ActualCost:   1.2,
		CreatedAt:    time.Now().UTC(),
	}
	prepared := prepareUsageLogInsert(log)

	query, _ := buildUsageLogBatchInsertQuery([]string{usageLogBatchKey(log.RequestID, log.APIKeyID)}, map[string]usageLogInsertPrepared{
		usageLogBatchKey(log.RequestID, log.APIKeyID): prepared,
	})

	require.Contains(t, query, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
	require.NotContains(t, strings.ToUpper(query), "DO UPDATE")
}

func TestUsageSnapshotBusinessDateUsesShanghaiDay(t *testing.T) {
	got := usageSnapshotBusinessDate(time.Date(2024, 1, 1, 16, 30, 0, 0, time.UTC))
	require.Equal(t, "2024-01-02", got)
}

func TestIsUsageSnapshotBusinessFullDayRange(t *testing.T) {
	loc := usageSnapshotBusinessLocation()
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)

	require.True(t, isUsageSnapshotBusinessFullDayRange(start.UTC(), end.UTC()))
	require.False(t, isUsageSnapshotBusinessFullDayRange(start.Add(time.Hour), end))
}

func TestBuildSnapshotUsageStatsConditionsUsesShanghaiDateStrings(t *testing.T) {
	start := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 2, 16, 0, 0, 0, time.UTC)
	_, args := buildSnapshotUsageStatsConditions(UsageLogFilters{StartTime: &start, EndTime: &end})

	require.Equal(t, []any{"2024-01-02", "2024-01-03"}, args)
}
