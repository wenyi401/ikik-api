package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCarpoolRepositoryListPoolApplicantUsageStatsPassesPoolID(t *testing.T) {
	db, mock := newSQLMock(t)
	defer func() { _ = db.Close() }()

	rows := sqlmock.NewRows([]string{
		"user_id",
		"total_requests",
		"total_tokens",
		"last7d_requests",
		"last7d_tokens",
		"last30d_requests",
		"last30d_tokens",
	}).AddRow(int64(42), int64(9), int64(1000), int64(3), int64(400), int64(5), int64(700))

	mock.ExpectQuery(`FROM carpool_join_requests`).
		WithArgs(int64(99)).
		WillReturnRows(rows)

	repo := &carpoolRepository{db: db}
	got, err := repo.ListPoolApplicantUsageStats(context.Background(), 99)
	require.NoError(t, err)
	require.Equal(t, int64(9), got[42].TotalRequests)
	require.Equal(t, int64(1000), got[42].TotalTokens)
	require.NoError(t, mock.ExpectationsWereMet())
}
