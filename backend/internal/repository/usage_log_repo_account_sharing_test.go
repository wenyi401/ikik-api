package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetUserAccountSharingAccountStatsExcludesPrivateAccountDetails(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)

	rows := sqlmock.NewRows([]string{
		"owned_accounts", "private_accounts", "public_pending_accounts", "public_approved_accounts", "public_suspended_accounts",
		"self_requests", "self_tokens", "self_actual_cost", "self_account_cost",
		"external_requests", "external_consumer_charge", "external_account_cost", "external_owner_credit", "external_platform_fee",
		"account_id", "name", "platform", "share_mode", "share_status",
		"account_self_requests", "account_self_tokens", "account_self_actual_cost", "account_self_cost",
		"account_external_requests", "account_external_charge", "account_external_cost", "account_owner_credit", "account_platform_fee",
	}).AddRow(
		3, 2, 0, 1, 0,
		10, 1000, 1.2, 0.8,
		5, 2.0, 1.0, 0.7, 0.3,
		99, "shared-account", "openai", "public", "approved",
		2, 200, 0.2, 0.1,
		5, 2.0, 1.0, 0.7, 0.3,
	)

	mock.ExpectQuery(`(?s)paged_accounts AS \(\s*SELECT \*\s*FROM account_stats\s*WHERE share_mode = 'public'`).
		WithArgs(int64(7), start, end, 20, 0).
		WillReturnRows(rows)

	accounts, summary, page, err := repo.getUserAccountSharingAccountStats(context.Background(), 7, start, end, 1, 20)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, "public", accounts[0].ShareMode)
	require.Equal(t, int64(2), summary.PrivateAccounts)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, 1, page.Pages)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNormalizeAccountSharingPaginationAllowsOneThousandRows(t *testing.T) {
	page, pageSize := normalizeAccountSharingPagination(1, 1000)
	require.Equal(t, 1, page)
	require.Equal(t, 1000, pageSize)

	_, cappedPageSize := normalizeAccountSharingPagination(1, 1001)
	require.Equal(t, 1000, cappedPageSize)
}
