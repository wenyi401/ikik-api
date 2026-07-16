package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ikik-api/internal/service"
)

type walletBucketDebitBreakdown struct {
	Recharge float64
	Invite   float64
	Share    float64
}

type walletBucketUpdateResult struct {
	NewBalance          float64
	NewRechargeBalance  float64
	NewInviteBalance    float64
	NewShareBalance     float64
	Debit               walletBucketDebitBreakdown
}

func creditWalletBucket(ctx context.Context, exec sqlQueryExecutor, userID int64, amount float64, bucket string) (float64, error) {
	if exec == nil {
		return 0, fmt.Errorf("sql executor is not configured")
	}
	if userID <= 0 {
		return 0, service.ErrUserNotFound
	}
	if amount <= 0 {
		current, err := getUserBalanceForWallet(ctx, exec, userID)
		return current, err
	}

	var query string
	switch bucket {
	case "share":
		query = `
UPDATE users
SET balance = balance + $1::numeric,
	share_income_balance = share_income_balance + $1::numeric,
	total_share_income = total_share_income + $1::numeric,
	updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL
RETURNING balance::double precision`
	case "invite":
		query = `
UPDATE users
SET balance = balance + $1::numeric,
	invite_income_balance = invite_income_balance + $1::numeric,
	total_invite_income = total_invite_income + $1::numeric,
	updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL
RETURNING balance::double precision`
	case "recharge":
		query = `
UPDATE users
SET balance = balance + $1::numeric,
	recharge_balance = recharge_balance + $1::numeric,
	total_recharged = total_recharged + $1::numeric,
	updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL
RETURNING balance::double precision`
	default:
		return 0, fmt.Errorf("unknown wallet bucket %q", bucket)
	}

	var newBalance float64
	if err := scanSingleRow(ctx, exec, query, []any{amount, userID}, &newBalance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, service.ErrUserNotFound
		}
		return 0, err
	}
	return newBalance, nil
}

func adjustRechargeWalletBalance(ctx context.Context, exec sqlQueryExecutor, userID int64, amount float64) (walletBucketUpdateResult, error) {
	if amount >= 0 {
		newBalance, err := creditWalletBucket(ctx, exec, userID, amount, "recharge")
		if err != nil {
			return walletBucketUpdateResult{}, err
		}
		return loadWalletBucketUpdateResult(ctx, exec, userID, newBalance)
	}
	return debitWalletBuckets(ctx, exec, userID, -amount)
}

func debitWalletBuckets(ctx context.Context, exec sqlQueryExecutor, userID int64, amount float64) (walletBucketUpdateResult, error) {
	if exec == nil {
		return walletBucketUpdateResult{}, fmt.Errorf("sql executor is not configured")
	}
	if userID <= 0 {
		return walletBucketUpdateResult{}, service.ErrUserNotFound
	}
	if amount <= 0 {
		current, err := getUserBalanceForWallet(ctx, exec, userID)
		if err != nil {
			return walletBucketUpdateResult{}, err
		}
		return loadWalletBucketUpdateResult(ctx, exec, userID, current)
	}

	const query = `
WITH locked AS (
	SELECT id, recharge_balance, invite_income_balance, share_income_balance
	FROM users
	WHERE id = $1 AND deleted_at IS NULL
	FOR UPDATE
), first_pass AS (
	SELECT
		id,
		LEAST(recharge_balance, $2::numeric) AS recharge_debit,
		GREATEST($2::numeric - LEAST(recharge_balance, $2::numeric), 0) AS after_recharge,
		invite_income_balance,
		share_income_balance
	FROM locked
), second_pass AS (
	SELECT
		id,
		recharge_debit,
		LEAST(invite_income_balance, after_recharge) AS invite_debit,
		GREATEST(after_recharge - LEAST(invite_income_balance, after_recharge), 0) AS after_invite,
		share_income_balance
	FROM first_pass
), calc AS (
	SELECT
		id,
		recharge_debit,
		invite_debit,
		LEAST(share_income_balance, after_invite) AS share_debit
	FROM second_pass
)
UPDATE users u
SET balance = u.balance - $2::numeric,
	recharge_balance = u.recharge_balance - calc.recharge_debit,
	invite_income_balance = u.invite_income_balance - calc.invite_debit,
	share_income_balance = u.share_income_balance - calc.share_debit,
	updated_at = NOW()
FROM calc
WHERE u.id = calc.id
RETURNING
	u.balance::double precision,
	u.recharge_balance::double precision,
	u.invite_income_balance::double precision,
	u.share_income_balance::double precision,
	calc.recharge_debit::double precision,
	calc.invite_debit::double precision,
	calc.share_debit::double precision`

	var result walletBucketUpdateResult
	if err := scanSingleRow(ctx, exec, query, []any{userID, amount},
		&result.NewBalance,
		&result.NewRechargeBalance,
		&result.NewInviteBalance,
		&result.NewShareBalance,
		&result.Debit.Recharge,
		&result.Debit.Invite,
		&result.Debit.Share,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return walletBucketUpdateResult{}, service.ErrUserNotFound
		}
		return walletBucketUpdateResult{}, err
	}
	return result, nil
}

func getUserBalanceForWallet(ctx context.Context, exec sqlQueryExecutor, userID int64) (float64, error) {
	var balance float64
	if err := scanSingleRow(ctx, exec, `
SELECT balance::double precision
FROM users
WHERE id = $1 AND deleted_at IS NULL`, []any{userID}, &balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, service.ErrUserNotFound
		}
		return 0, err
	}
	return balance, nil
}

func loadWalletBucketUpdateResult(ctx context.Context, exec sqlQueryExecutor, userID int64, fallbackBalance float64) (walletBucketUpdateResult, error) {
	result := walletBucketUpdateResult{NewBalance: fallbackBalance}
	if err := scanSingleRow(ctx, exec, `
SELECT balance::double precision,
	recharge_balance::double precision,
	invite_income_balance::double precision,
	share_income_balance::double precision
FROM users
WHERE id = $1 AND deleted_at IS NULL`, []any{userID},
		&result.NewBalance,
		&result.NewRechargeBalance,
		&result.NewInviteBalance,
		&result.NewShareBalance,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return walletBucketUpdateResult{}, service.ErrUserNotFound
		}
		return walletBucketUpdateResult{}, err
	}
	return result, nil
}
