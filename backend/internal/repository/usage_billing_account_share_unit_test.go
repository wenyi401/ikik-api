//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"ikik-api/internal/service"
)

func TestApplyAccountShareSettlement_CreditsOwnerOnce(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`INSERT INTO account_share_settlement_entries`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery(`UPDATE users[\s\S]*share_income_balance`).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.8))
	mock.ExpectExec(`INSERT INTO user_balance_ledger`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	ownerID := int64(9)
	result := &service.UsageBillingApplyResult{Applied: true}
	err = applyAccountShareSettlement(ctx, tx, &service.UsageBillingCommand{
		RequestID:             "req-share-once",
		APIKeyID:              2,
		UserID:                3,
		AccountID:             4,
		BalanceCost:           1,
		UsageLog:              &service.UsageLog{ID: 6, ActualCost: 1},
		ShareSnapshotCaptured: true,
		ShareOwnerUserID:      &ownerID,
		ShareModeSnapshot:     service.AccountShareModePublic,
		ShareStatusSnapshot:   service.AccountShareStatusApproved,
		OwnerShareRatio:       0.8,
	}, result)
	require.NoError(t, err)
	require.Equal(t, []int64{ownerID}, result.BalanceCreditUserIDs)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyAccountShareSettlement_DuplicateDoesNotCreditAgain(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`INSERT INTO account_share_settlement_entries`).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	ownerID := int64(9)
	result := &service.UsageBillingApplyResult{Applied: true}
	err = applyAccountShareSettlement(ctx, tx, &service.UsageBillingCommand{
		RequestID:             "req-share-duplicate",
		APIKeyID:              2,
		UserID:                3,
		AccountID:             4,
		BalanceCost:           1,
		UsageLog:              &service.UsageLog{ID: 6, ActualCost: 1},
		ShareSnapshotCaptured: true,
		ShareOwnerUserID:      &ownerID,
		ShareModeSnapshot:     service.AccountShareModePublic,
		ShareStatusSnapshot:   service.AccountShareStatusApproved,
		OwnerShareRatio:       0.8,
	}, result)
	require.NoError(t, err)
	require.Empty(t, result.BalanceCreditUserIDs)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
