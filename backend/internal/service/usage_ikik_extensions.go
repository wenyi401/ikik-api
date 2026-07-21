package service

import (
	"context"

	"fmt"
	"strconv"
	"strings"
	"time"

	"ikik-api/internal/pkg/usagestats"
)

// GetPublicTodayStats returns public homepage usage counters and health metrics.
func (s *UsageService) GetPublicTodayStats(ctx context.Context, startTime, endTime time.Time) (*usagestats.PublicTodayUsageStats, error) {
	globalStats, err := s.GetGlobalStats(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	healthGroupID := s.publicHomeStatsGroupID(ctx)
	healthStats := globalStats
	if healthGroupID > 0 {
		healthStats, err = s.publicHomeStats(ctx, startTime, endTime, healthGroupID)
		if err != nil {
			return nil, err
		}
	}

	errorCount, err := s.countPublicUsageErrors(ctx, startTime, endTime, healthGroupID)
	if err != nil {
		return nil, err
	}

	result := &usagestats.PublicTodayUsageStats{
		TodayRequests: globalStats.TotalRequests,
		TodayTokens:   globalStats.TotalTokens,
		SuccessCount:  healthStats.TotalRequests,
		ErrorCount:    errorCount,
	}

	totalObservedRequests := healthStats.TotalRequests + errorCount
	if totalObservedRequests > 0 {
		successRate := float64(healthStats.TotalRequests) / float64(totalObservedRequests) * 100
		result.SuccessRate = &successRate
	}
	if globalStats.TotalRequests > 0 {
		averageDurationMs := globalStats.AverageDurationMs
		result.AverageDurationMs = &averageDurationMs
	}
	if healthStats.RequestsWithFirstToken > 0 {
		averageFirstTokenMs := healthStats.AverageFirstTokenMs
		result.AverageFirstTokenMs = &averageFirstTokenMs
	}

	return result, nil
}

// GetUserAccountSharingDashboard returns owned-account consumption and public-sharing settlement stats.
func (s *UsageService) GetUserAccountSharingDashboard(ctx context.Context, userID int64, startTime, endTime time.Time, granularity string, accountPage, accountPageSize int) (*usagestats.AccountSharingDashboardStats, error) {
	stats, err := s.usageRepo.GetUserAccountSharingDashboard(ctx, userID, startTime, endTime, granularity, accountPage, accountPageSize)
	if err != nil {
		return nil, fmt.Errorf("get user account sharing dashboard: %w", err)
	}
	return stats, nil
}

// SetHomeStatsGroupReader injects group lookup for homepage stats group validation.
func (s *UsageService) SetHomeStatsGroupReader(reader DefaultSubscriptionGroupReader) {
	s.homeStatsGroupReader = reader
}

// SetSettingRepository injects settings for optional usage-stat presentation filters.
func (s *UsageService) SetSettingRepository(repo SettingRepository) {
	s.settingRepo = repo
}

func (s *UsageService) countPublicUsageErrors(ctx context.Context, startTime, endTime time.Time, groupID int64) (int64, error) {
	if s.entClient == nil {
		return 0, nil
	}
	hasStatusCode, err := s.tableColumnExists(ctx, "ops_error_logs", "status_code")
	if err != nil || !hasStatusCode {
		return 0, nil
	}

	args := []any{startTime, endTime}
	query := `
		SELECT COALESCE(COUNT(*), 0)
		FROM ops_error_logs
		WHERE created_at >= $1
		  AND created_at < $2
		  AND COALESCE(status_code, 0) >= 400
	`
	if groupID > 0 {
		hasGroupID, err := s.tableColumnExists(ctx, "ops_error_logs", "group_id")
		if err != nil || !hasGroupID {
			return 0, nil
		}
		args = append(args, groupID)
		query += fmt.Sprintf(" AND group_id = $%d", len(args))
	}

	hasBusinessLimited, err := s.tableColumnExists(ctx, "ops_error_logs", "is_business_limited")
	if err == nil && hasBusinessLimited {
		query += " AND NOT COALESCE(is_business_limited, false)"
	}

	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("count public usage errors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var count int64
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, fmt.Errorf("scan public usage errors: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate public usage errors: %w", err)
	}

	return count, nil
}
func (s *UsageService) publicHomeStats(ctx context.Context, startTime, endTime time.Time, groupID int64) (*usagestats.UsageStats, error) {
	if groupID <= 0 {
		return s.GetGlobalStats(ctx, startTime, endTime)
	}
	return s.GetStatsWithFilters(ctx, usagestats.UsageLogFilters{
		GroupID:   groupID,
		StartTime: &startTime,
		EndTime:   &endTime,
	})
}

func (s *UsageService) publicHomeStatsGroupID(ctx context.Context) int64 {
	if s == nil || s.settingRepo == nil {
		return 0
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyHomeStatsGroupID)
	if err != nil {
		return 0
	}
	groupID, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || groupID <= 0 {
		return 0
	}
	if s.homeStatsGroupReader != nil {
		group, err := s.homeStatsGroupReader.GetByID(ctx, groupID)
		if err != nil || !isAdministratorPublicGroup(group) {
			return 0
		}
	}
	return groupID
}

func (s *UsageService) tableColumnExists(ctx context.Context, tableName, columnName string) (bool, error) {
	rows, err := s.entClient.QueryContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = $1
			  AND column_name = $2
		)
	`, tableName, columnName)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	var exists bool
	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			return false, err
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	return exists, nil
}
