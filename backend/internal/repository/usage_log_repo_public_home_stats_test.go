package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"ikik-api/internal/pkg/usagestats"
)

func TestGetGlobalStatsIncludesFirstTokenMetrics(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT[\s\S]*AVG\(first_token_ms\)[\s\S]*COUNT\(first_token_ms\)[\s\S]*FROM usage_logs`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "total_input_tokens", "total_output_tokens", "total_cache_tokens",
			"total_cost", "total_actual_cost", "avg_duration_ms", "avg_first_token_ms",
			"requests_with_first_token",
		}).AddRow(12, 100, 50, 20, 1.2, 0.6, 8200, 2450.5, 8))

	stats, err := repo.GetGlobalStats(context.Background(), start, end)
	require.NoError(t, err)
	require.Equal(t, 2450.5, stats.AverageFirstTokenMs)
	require.Equal(t, int64(8), stats.RequestsWithFirstToken)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetStatsWithFiltersIncludesFirstTokenMetrics(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT[\s\S]*AVG\(first_token_ms\)[\s\S]*COUNT\(first_token_ms\)[\s\S]*FROM usage_logs`).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "total_input_tokens", "total_output_tokens", "total_cache_tokens",
			"total_cache_creation_tokens", "total_cache_read_tokens", "total_cost", "total_actual_cost",
			"total_account_cost", "avg_duration_ms", "avg_first_token_ms", "requests_with_first_token",
		}).AddRow(6, 80, 40, 12, 4, 8, 0.8, 0.4, 0.5, 7000, 1875.25, 5))

	for range 3 {
		mock.ExpectQuery(`SELECT[\s\S]*GROUP BY endpoint ORDER BY requests DESC`).
			WillReturnRows(sqlmock.NewRows([]string{"endpoint", "requests", "total_tokens", "cost", "actual_cost"}))
	}

	stats, err := repo.GetStatsWithFilters(context.Background(), usagestats.UsageLogFilters{
		StartTime: &start,
		EndTime:   &end,
	})
	require.NoError(t, err)
	require.Equal(t, 1875.25, stats.AverageFirstTokenMs)
	require.Equal(t, int64(5), stats.RequestsWithFirstToken)
	require.NoError(t, mock.ExpectationsWereMet())
}
