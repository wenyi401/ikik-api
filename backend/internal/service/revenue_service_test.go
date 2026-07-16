package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRevenueSnapshotDateRangeUsesShanghaiDay(t *testing.T) {
	params := RevenueQueryParams{
		StartTime: time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2024, 1, 2, 16, 0, 0, 0, time.UTC),
	}

	startDate, endDate := revenueSnapshotDateRange(params)

	require.Equal(t, "2024-01-02", startDate)
	require.Equal(t, "2024-01-03", endDate)
}

func TestShouldUseRevenueDailySnapshotsRequiresShanghaiFullDay(t *testing.T) {
	loc := revenueSnapshotBusinessLocation()
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)

	require.True(t, shouldUseRevenueDailySnapshots(RevenueQueryParams{
		StartTime:   start.UTC(),
		EndTime:     end.UTC(),
		Granularity: RevenueGranularityDay,
		Timezone:    revenueSnapshotBusinessTimezone,
	}))
	require.False(t, shouldUseRevenueDailySnapshots(RevenueQueryParams{
		StartTime:   start.UTC(),
		EndTime:     end.UTC(),
		Granularity: RevenueGranularityDay,
		Timezone:    "UTC",
	}))
	require.False(t, shouldUseRevenueDailySnapshots(RevenueQueryParams{
		StartTime:   start.Add(time.Hour),
		EndTime:     end,
		Granularity: RevenueGranularityDay,
		Timezone:    revenueSnapshotBusinessTimezone,
	}))
}
