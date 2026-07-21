package repository

import (
	"context"

	"fmt"

	"time"

	"ikik-api/internal/pkg/usagestats"
)

// GetUserAccountSharingDashboard returns owned-account self usage and external public-share settlement stats.
func (r *usageLogRepository) GetUserAccountSharingDashboard(ctx context.Context, userID int64, startTime, endTime time.Time, granularity string, accountPage, accountPageSize int) (*usagestats.AccountSharingDashboardStats, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user id must be positive")
	}
	if startTime.IsZero() {
		startTime = time.Now().AddDate(0, 0, -7)
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}
	accountPage, accountPageSize = normalizeAccountSharingPagination(accountPage, accountPageSize)

	accounts, summary, accountPageInfo, err := r.getUserAccountSharingAccountStats(ctx, userID, startTime, endTime, accountPage, accountPageSize)
	if err != nil {
		return nil, err
	}
	trend, err := r.getUserAccountSharingTrend(ctx, userID, startTime, endTime, granularity)
	if err != nil {
		return nil, err
	}

	endDisplay := endTime
	if endDisplay.After(startTime) {
		endDisplay = endDisplay.Add(-time.Nanosecond)
	}
	return &usagestats.AccountSharingDashboardStats{
		Summary:     summary,
		Accounts:    accounts,
		AccountPage: accountPageInfo,
		Trend:       trend,
		StartDate:   startTime.Format("2006-01-02"),
		EndDate:     endDisplay.Format("2006-01-02"),
		Granularity: granularity,
	}, nil
}
