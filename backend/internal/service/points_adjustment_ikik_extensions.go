package service

import (
	"context"

	"database/sql"

	"encoding/json"
	"errors"

	"strings"

	"entgo.io/ent/dialect"
	"github.com/shopspring/decimal"
	dbent "ikik-api/ent"
	infraerrors "ikik-api/internal/pkg/errors"
)

func applyPointsAdjustmentInTx(ctx context.Context, tx *dbent.Tx, in pointsAdjustmentInput) error {
	if tx == nil {
		return errors.New("points adjustment requires transaction")
	}
	if in.UserID <= 0 {
		return ErrUserNotFound
	}
	if in.Delta == 0 {
		return nil
	}
	queryer, ok := tx.Driver().(serviceSQLQueryer)
	if !ok {
		return errors.New("points adjustment requires QueryContext support")
	}
	execer, ok := tx.Driver().(serviceSQLExecer)
	if !ok {
		return errors.New("points adjustment requires ExecContext support")
	}

	balanceBefore, err := currentPointsBalanceWithQueryer(ctx, queryer, in.UserID, tx.Driver().Dialect() == dialect.Postgres)
	if err != nil {
		return err
	}

	delta := in.Delta
	if in.ClampZero && delta < 0 && balanceBefore+delta < 0 {
		delta = -balanceBefore
	}
	balanceAfter := balanceBefore + delta
	if balanceAfter < -1e-9 {
		return infraerrors.BadRequest("POINTS_BALANCE_NEGATIVE", "points balance cannot be negative")
	}
	if balanceAfter < 0 {
		balanceAfter = 0
	}

	amount := delta
	direction := "credit"
	if amount < 0 {
		direction = "debit"
		amount = -amount
	}
	dialectName := tx.Driver().Dialect()
	amountValue := decimal.NewFromFloat(amount).Round(10).StringFixed(10)
	balanceBeforeValue := decimal.NewFromFloat(balanceBefore).Round(10).StringFixed(10)
	balanceAfterValue := decimal.NewFromFloat(balanceAfter).Round(10).StringFixed(10)
	updateQuery := `
		UPDATE users
		SET points_balance = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND deleted_at IS NULL
	`
	if dialectName == dialect.Postgres {
		updateQuery = `
			UPDATE users
			SET points_balance = $1::numeric,
				updated_at = NOW()
			WHERE id = $2 AND deleted_at IS NULL
		`
	}
	if _, err := execer.ExecContext(ctx, updateQuery, balanceAfterValue, in.UserID); err != nil {
		return err
	}

	if amount == 0 {
		return nil
	}
	metadata := in.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	rawMetadata, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	var refID any
	if in.RefID > 0 {
		refID = in.RefID
	}
	var operatorUserID any
	if in.OperatorUserID > 0 {
		operatorUserID = in.OperatorUserID
	}
	insertQuery := `
		INSERT INTO points_ledger (
			user_id, direction, amount, reason, ref_type, ref_id,
			balance_before, balance_after, operator_user_id, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10
		)
		ON CONFLICT DO NOTHING
	`
	if dialectName == dialect.Postgres {
		insertQuery = `
			INSERT INTO points_ledger (
				user_id, direction, amount, reason, ref_type, ref_id,
				balance_before, balance_after, operator_user_id, metadata
			) VALUES (
				$1, $2, $3::numeric, $4, $5, $6,
				$7::numeric, $8::numeric, $9, $10::jsonb
			)
			ON CONFLICT DO NOTHING
		`
	}
	_, err = execer.ExecContext(ctx, insertQuery,
		in.UserID,
		direction,
		amountValue,
		strings.TrimSpace(in.Reason),
		strings.TrimSpace(in.RefType),
		refID,
		balanceBeforeValue,
		balanceAfterValue,
		operatorUserID,
		string(rawMetadata),
	)
	return err
}
func currentPointsBalanceInTx(ctx context.Context, tx *dbent.Tx, userID int64) (float64, error) {
	if tx == nil {
		return 0, errors.New("points balance lookup requires transaction")
	}
	if userID <= 0 {
		return 0, ErrUserNotFound
	}
	queryer, ok := tx.Driver().(serviceSQLQueryer)
	if !ok {
		return 0, errors.New("points balance lookup requires QueryContext support")
	}
	return currentPointsBalanceWithQueryer(ctx, queryer, userID, tx.Driver().Dialect() == dialect.Postgres)
}

func currentPointsBalanceWithQueryer(ctx context.Context, queryer serviceSQLQueryer, userID int64, forUpdate bool) (float64, error) {
	var balanceBefore float64
	query := `
		SELECT points_balance
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := queryer.QueryContext(ctx, query, userID)
	if err != nil {
		return 0, err
	}
	if rows.Next() {
		if err := rows.Scan(&balanceBefore); err != nil {
			_ = rows.Close()
			return 0, err
		}
	} else {
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return 0, err
		}
		_ = rows.Close()
		return 0, ErrUserNotFound
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	return balanceBefore, nil
}

type pointsAdjustmentInput struct {
	UserID         int64
	Delta          float64
	Reason         string
	RefType        string
	RefID          int64
	OperatorUserID int64
	Metadata       map[string]any
	ClampZero      bool
}
type serviceSQLExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
type serviceSQLQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}
