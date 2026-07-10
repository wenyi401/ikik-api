package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	dbent "ikik-api/ent"
	"ikik-api/internal/service"
)

const (
	affiliateCodeLength      = 12
	affiliateCodeMaxAttempts = 12
)

var affiliateCodeCharset = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

type affiliateQueryExecer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type affiliateRepository struct {
	client *dbent.Client
}

func NewAffiliateRepository(client *dbent.Client, _ *sql.DB) service.AffiliateRepository {
	return &affiliateRepository{client: client}
}

func (r *affiliateRepository) EnsureUserAffiliate(ctx context.Context, userID int64) (*service.AffiliateSummary, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	client := clientFromContext(ctx, r.client)
	return ensureUserAffiliateWithClient(ctx, client, userID)
}

func (r *affiliateRepository) GetAffiliateByCode(ctx context.Context, code string) (*service.AffiliateSummary, error) {
	client := clientFromContext(ctx, r.client)
	return queryAffiliateByCode(ctx, client, code)
}

func (r *affiliateRepository) GetCurrentInviteSharePercent(ctx context.Context) (float64, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
		SELECT invite_share_ratio::double precision * 100
		FROM account_share_policies
		WHERE deleted_at IS NULL
			AND enabled = TRUE
			AND effective_at <= NOW()
			AND scope_type = 'global'
		ORDER BY effective_at DESC, version DESC, id DESC
		LIMIT 1
	`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var percent float64
	if err := rows.Scan(&percent); err != nil {
		return 0, err
	}
	return percent, rows.Err()
}

func (r *affiliateRepository) BindInviter(ctx context.Context, userID, inviterID int64) (bool, error) {
	if userID <= 0 || inviterID <= 0 || userID == inviterID {
		return false, service.ErrAffiliateCodeInvalid
	}
	var bound bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, inviterID); err != nil {
			return err
		}

		res, err := txClient.ExecContext(txCtx,
			`UPDATE user_affiliates
			SET inviter_id = $1,
				inviter_bound_at = COALESCE(inviter_bound_at, NOW()),
				invite_bind_source = COALESCE(invite_bind_source, 'registration'),
				invite_reward_expires_at = CASE
					WHEN invite_reward_expires_at IS NOT NULL THEN invite_reward_expires_at
					WHEN duration.days > 0 THEN NOW() + make_interval(days => duration.days)
					ELSE NULL
				END,
				updated_at = NOW()
			FROM (
				SELECT COALESCE((
					SELECT CASE
						WHEN value ~ '^[0-9]+$' THEN LEAST(value::integer, $3)
						ELSE 0
					END
					FROM settings
					WHERE key = $4
					LIMIT 1
				), 0) AS days
			) duration
			WHERE user_affiliates.user_id = $2
				AND user_affiliates.inviter_id IS NULL`,
			inviterID, userID, service.AffiliateRebateDurationDaysMax, service.SettingKeyAffiliateRebateDurationDays,
		)
		if err != nil {
			return fmt.Errorf("bind inviter: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			bound = false
			return nil
		}

		if _, err = txClient.ExecContext(txCtx,
			"UPDATE user_affiliates SET aff_count = aff_count + 1, updated_at = NOW() WHERE user_id = $1",
			inviterID,
		); err != nil {
			return fmt.Errorf("increment inviter aff_count: %w", err)
		}
		bound = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return bound, nil
}

func (r *affiliateRepository) AdminBindInviter(ctx context.Context, userID, inviterID int64, resetValidity bool) (*service.AffiliateSummary, error) {
	if userID <= 0 || inviterID <= 0 || userID == inviterID {
		return nil, service.ErrAffiliateCodeInvalid
	}
	var out *service.AffiliateSummary
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		current, err := ensureUserAffiliateWithClient(txCtx, txClient, userID)
		if err != nil {
			return err
		}
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, inviterID); err != nil {
			return err
		}

		var oldInviterID int64
		if current.InviterID != nil {
			oldInviterID = *current.InviterID
		}

		resetArg := resetValidity
		_, err = txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET inviter_id = $1,
    inviter_bound_at = CASE
        WHEN $3::boolean OR inviter_bound_at IS NULL THEN NOW()
        ELSE inviter_bound_at
    END,
    invite_bind_source = 'admin',
    invite_reward_expires_at = CASE
        WHEN $3::boolean OR inviter_bound_at IS NULL THEN
            CASE
                WHEN duration.days > 0 THEN NOW() + make_interval(days => duration.days)
                ELSE NULL
            END
        ELSE invite_reward_expires_at
    END,
    updated_at = NOW()
FROM (
    SELECT COALESCE((
        SELECT CASE
            WHEN value ~ '^[0-9]+$' THEN LEAST(value::integer, $4)
            ELSE 0
        END
        FROM settings
        WHERE key = $5
        LIMIT 1
    ), 0) AS days
) duration
WHERE user_affiliates.user_id = $2`,
			inviterID, userID, resetArg, service.AffiliateRebateDurationDaysMax, service.SettingKeyAffiliateRebateDurationDays,
		)
		if err != nil {
			return fmt.Errorf("admin bind inviter: %w", err)
		}

		if oldInviterID > 0 && oldInviterID != inviterID {
			if _, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_count = GREATEST(aff_count - 1, 0),
    updated_at = NOW()
WHERE user_id = $1`, oldInviterID); err != nil {
				return fmt.Errorf("decrement old inviter aff_count: %w", err)
			}
		}
		if oldInviterID != inviterID {
			if _, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_count = aff_count + 1,
    updated_at = NOW()
WHERE user_id = $1`, inviterID); err != nil {
				return fmt.Errorf("increment new inviter aff_count: %w", err)
			}
		}

		out, err = queryAffiliateByUserID(txCtx, txClient, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *affiliateRepository) AdminExtendInviteRewards(ctx context.Context, req service.AffiliateInviteRewardExtensionRequest) (*service.AffiliateInviteRewardExtensionResult, error) {
	var (
		res sql.Result
		err error
	)

	client := clientFromContext(ctx, r.client)
	switch req.Scope {
	case service.AffiliateInviteRewardExtensionScopeSite:
		res, err = client.ExecContext(ctx, `
UPDATE user_affiliates
SET invite_reward_expires_at = invite_reward_expires_at + make_interval(days => $1),
    updated_at = NOW()
WHERE inviter_id IS NOT NULL
  AND invite_reward_expires_at IS NOT NULL
  AND invite_reward_expires_at > NOW()`, req.ExtendDays)
	case service.AffiliateInviteRewardExtensionScopeInviter:
		if req.AllInvitees {
			res, err = client.ExecContext(ctx, `
UPDATE user_affiliates
SET invite_reward_expires_at = invite_reward_expires_at + make_interval(days => $1),
    updated_at = NOW()
WHERE inviter_id = $2
  AND invite_reward_expires_at IS NOT NULL
  AND invite_reward_expires_at > NOW()`, req.ExtendDays, req.InviterUserID)
		} else {
			res, err = client.ExecContext(ctx, `
UPDATE user_affiliates
SET invite_reward_expires_at = invite_reward_expires_at + make_interval(days => $1),
    updated_at = NOW()
WHERE inviter_id = $2
  AND user_id = ANY($3)
  AND invite_reward_expires_at IS NOT NULL
  AND invite_reward_expires_at > NOW()`, req.ExtendDays, req.InviterUserID, pq.Array(req.InviteeUserIDs))
		}
	default:
		return nil, fmt.Errorf("unsupported affiliate extension scope: %s", req.Scope)
	}
	if err != nil {
		return nil, fmt.Errorf("extend affiliate invite rewards: %w", err)
	}
	affected, _ := res.RowsAffected()
	return &service.AffiliateInviteRewardExtensionResult{Affected: affected}, nil
}

func (r *affiliateRepository) AccrueQuota(ctx context.Context, inviterID, inviteeUserID int64, amount float64, freezeHours int, sourceOrderID *int64) (bool, error) {
	if amount <= 0 {
		return false, nil
	}

	var applied bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		// freezeHours > 0: add to frozen quota; == 0: add to available quota directly
		var updateSQL string
		if freezeHours > 0 {
			updateSQL = "UPDATE user_affiliates SET aff_frozen_quota = aff_frozen_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2"
		} else {
			updateSQL = "UPDATE user_affiliates SET aff_quota = aff_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2"
		}
		res, err := txClient.ExecContext(txCtx, updateSQL, amount, inviterID)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			applied = false
			return nil
		}

		if freezeHours > 0 {
			if _, err = txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, source_order_id, frozen_until, created_at, updated_at)
VALUES ($1, 'accrue', $2, $3, $4, NOW() + make_interval(hours => $5), NOW(), NOW())`,
				inviterID, amount, inviteeUserID, nullableInt64Arg(sourceOrderID), freezeHours); err != nil {
				return fmt.Errorf("insert affiliate accrue ledger: %w", err)
			}
		} else {
			if _, err = txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, source_order_id, created_at, updated_at)
VALUES ($1, 'accrue', $2, $3, $4, NOW(), NOW())`, inviterID, amount, inviteeUserID, nullableInt64Arg(sourceOrderID)); err != nil {
				return fmt.Errorf("insert affiliate accrue ledger: %w", err)
			}
		}

		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (r *affiliateRepository) GetAccruedRebateFromInvitee(ctx context.Context, inviterID, inviteeUserID int64) (float64, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx,
		`SELECT COALESCE(SUM(amount), 0)::double precision FROM user_affiliate_ledger WHERE user_id = $1 AND source_user_id = $2 AND action = 'accrue'`,
		inviterID, inviteeUserID)
	if err != nil {
		return 0, fmt.Errorf("query accrued rebate from invitee: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var total float64
	if rows.Next() {
		if err := rows.Scan(&total); err != nil {
			return 0, err
		}
	}
	return total, rows.Close()
}

func (r *affiliateRepository) ThawFrozenQuota(ctx context.Context, userID int64) (float64, error) {
	var thawed float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var err error
		thawed, err = thawFrozenQuotaTx(txCtx, txClient, userID)
		return err
	})
	return thawed, err
}

// thawFrozenQuotaTx moves matured frozen quota to available quota within an existing tx.
func thawFrozenQuotaTx(txCtx context.Context, txClient *dbent.Client, userID int64) (float64, error) {
	rows, err := txClient.QueryContext(txCtx, `
WITH matured AS (
    UPDATE user_affiliate_ledger
    SET frozen_until = NULL, updated_at = NOW()
    WHERE user_id = $1
      AND frozen_until IS NOT NULL
      AND frozen_until <= NOW()
    RETURNING amount
)
SELECT COALESCE(SUM(amount), 0) FROM matured`, userID)
	if err != nil {
		return 0, fmt.Errorf("thaw frozen quota: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var thawed float64
	if rows.Next() {
		if err := rows.Scan(&thawed); err != nil {
			return 0, err
		}
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if thawed <= 0 {
		return 0, nil
	}

	_, err = txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_quota = aff_quota + $1,
    aff_frozen_quota = GREATEST(aff_frozen_quota - $1, 0),
    updated_at = NOW()
WHERE user_id = $2`, thawed, userID)
	if err != nil {
		return 0, fmt.Errorf("move thawed quota: %w", err)
	}
	return thawed, nil
}

func (r *affiliateRepository) TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error) {
	var transferred float64
	var newBalance float64

	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}

		// Thaw any matured frozen quota before transfer.
		if _, err := thawFrozenQuotaTx(txCtx, txClient, userID); err != nil {
			return fmt.Errorf("thaw before transfer: %w", err)
		}

		rows, err := txClient.QueryContext(txCtx, `
WITH claimed AS (
	SELECT aff_quota::double precision AS amount
	FROM user_affiliates
	WHERE user_id = $1
	  AND aff_quota > 0
	FOR UPDATE
),
cleared AS (
	UPDATE user_affiliates ua
	SET aff_quota = 0,
	    updated_at = NOW()
	FROM claimed c
	WHERE ua.user_id = $1
	RETURNING c.amount
)
SELECT amount
FROM cleared`, userID)
		if err != nil {
			return fmt.Errorf("claim affiliate quota: %w", err)
		}

		if !rows.Next() {
			_ = rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			return service.ErrAffiliateQuotaEmpty
		}
		if err := rows.Scan(&transferred); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if transferred <= 0 {
			return service.ErrAffiliateQuotaEmpty
		}

		newBalance, err = creditWalletBucket(txCtx, txClient, userID, transferred, "invite")
		if err != nil {
			return fmt.Errorf("credit user balance by affiliate quota: %w", err)
		}

		if _, err = txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, created_at, updated_at)
VALUES ($1, 'transfer', $2, NULL, NOW(), NOW())`, userID, transferred); err != nil {
			return fmt.Errorf("insert affiliate transfer ledger: %w", err)
		}

		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return transferred, newBalance, nil
}

func (r *affiliateRepository) ListInvitees(ctx context.Context, inviterID int64, query service.AffiliateDetailQuery, limit int) ([]service.AffiliateInvitee, float64, error) {
	if limit <= 0 {
		limit = 100
	}
	client := clientFromContext(ctx, r.client)
	periodRebate, err := queryAffiliatePeriodRebate(ctx, client, inviterID, query)
	if err != nil {
		return nil, 0, err
	}
	rows, err := client.QueryContext(ctx, `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       COALESCE(ua.inviter_bound_at, ua.created_at),
       COALESCE(ua.invite_bind_source, ''),
       COALESCE(u.status, ''),
       COALESCE(SUM(ase.consumer_charge) FILTER (
           WHERE ase.status = 'applied'
             AND ase.inviter_user_id = $1
       ), 0)::double precision AS history_consumption,
       COALESCE(SUM(ase.invite_credit) FILTER (
           WHERE ase.status = 'applied'
             AND ase.inviter_user_id = $1
       ), 0)::double precision AS total_rebate,
       COALESCE(SUM(ase.consumer_charge) FILTER (
           WHERE ase.status = 'applied'
             AND ase.inviter_user_id = $1
             AND ($2::timestamptz IS NULL OR ase.created_at >= $2::timestamptz)
             AND ($3::timestamptz IS NULL OR ase.created_at < $3::timestamptz)
       ), 0)::double precision AS period_consumption,
       COALESCE(SUM(ase.invite_credit) FILTER (
           WHERE ase.status = 'applied'
             AND ase.inviter_user_id = $1
             AND ($2::timestamptz IS NULL OR ase.created_at >= $2::timestamptz)
             AND ($3::timestamptz IS NULL OR ase.created_at < $3::timestamptz)
       ), 0)::double precision AS period_rebate
FROM user_affiliates ua
LEFT JOIN users u ON u.id = ua.user_id
LEFT JOIN account_share_settlement_entries ase
       ON ase.consumer_user_id = ua.user_id
      AND ase.created_at >= COALESCE(ua.inviter_bound_at, ua.created_at)
WHERE ua.inviter_id = $1
GROUP BY ua.user_id, u.email, u.username, u.status, ua.inviter_bound_at, ua.created_at, ua.invite_bind_source
ORDER BY COALESCE(ua.inviter_bound_at, ua.created_at) DESC
LIMIT $4`, inviterID, nullableTimeArg(query.PeriodStart), nullableTimeArg(query.PeriodEnd), limit)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	invitees := make([]service.AffiliateInvitee, 0)
	for rows.Next() {
		var item service.AffiliateInvitee
		var createdAt time.Time
		if err := rows.Scan(
			&item.UserID,
			&item.Email,
			&item.Username,
			&createdAt,
			&item.InviteBindSource,
			&item.Status,
			&item.HistoryConsumption,
			&item.TotalRebate,
			&item.PeriodConsumption,
			&item.PeriodRebate,
		); err != nil {
			return nil, 0, err
		}
		item.CreatedAt = &createdAt
		invitees = append(invitees, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return invitees, periodRebate, nil
}

func queryAffiliatePeriodRebate(ctx context.Context, client affiliateQueryExecer, inviterID int64, query service.AffiliateDetailQuery) (float64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT COALESCE(SUM(invite_credit), 0)::double precision
FROM account_share_settlement_entries
WHERE status = 'applied'
  AND inviter_user_id = $1
  AND ($2::timestamptz IS NULL OR created_at >= $2::timestamptz)
  AND ($3::timestamptz IS NULL OR created_at < $3::timestamptz)`,
		inviterID, nullableTimeArg(query.PeriodStart), nullableTimeArg(query.PeriodEnd))
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var total float64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
}

func (r *affiliateRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin affiliate transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit affiliate transaction: %w", err)
	}
	return nil
}

func ensureUserAffiliateWithClient(ctx context.Context, client affiliateQueryExecer, userID int64) (*service.AffiliateSummary, error) {
	summary, err := queryAffiliateByUserID(ctx, client, userID)
	if err == nil {
		return summary, nil
	}
	if !errors.Is(err, service.ErrAffiliateProfileNotFound) {
		return nil, err
	}

	for i := 0; i < affiliateCodeMaxAttempts; i++ {
		code, codeErr := generateAffiliateCode()
		if codeErr != nil {
			return nil, codeErr
		}
		_, insertErr := client.ExecContext(ctx, `
INSERT INTO user_affiliates (user_id, aff_code, created_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (user_id) DO NOTHING`, userID, code)
		if insertErr == nil {
			break
		}
		if isAffiliateUniqueViolation(insertErr) {
			continue
		}
		return nil, insertErr
	}

	return queryAffiliateByUserID(ctx, client, userID)
}

func queryAffiliateByUserID(ctx context.Context, client affiliateQueryExecer, userID int64) (*service.AffiliateSummary, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       aff_code,
       aff_code_custom,
       aff_rebate_rate_percent,
       inviter_id,
       inviter_bound_at,
       invite_bind_source,
       invite_reward_expires_at,
       aff_count,
       aff_quota::double precision,
       aff_frozen_quota::double precision,
       aff_history_quota::double precision,
       created_at,
       updated_at
FROM user_affiliates
WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAffiliateProfileNotFound
	}

	var out service.AffiliateSummary
	var inviterID sql.NullInt64
	var inviterBoundAt sql.NullTime
	var inviteRewardExpiresAt sql.NullTime
	var rebateRate sql.NullFloat64
	var inviteBindSource sql.NullString
	if err := rows.Scan(
		&out.UserID,
		&out.AffCode,
		&out.AffCodeCustom,
		&rebateRate,
		&inviterID,
		&inviterBoundAt,
		&inviteBindSource,
		&inviteRewardExpiresAt,
		&out.AffCount,
		&out.AffQuota,
		&out.AffFrozenQuota,
		&out.AffHistoryQuota,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if inviterID.Valid {
		out.InviterID = &inviterID.Int64
	}
	if inviterBoundAt.Valid {
		t := inviterBoundAt.Time
		out.InviterBoundAt = &t
	}
	if inviteBindSource.Valid {
		out.InviteBindSource = inviteBindSource.String
	}
	if inviteRewardExpiresAt.Valid {
		t := inviteRewardExpiresAt.Time
		out.InviteRewardExpiresAt = &t
	}
	if rebateRate.Valid {
		v := rebateRate.Float64
		out.AffRebateRatePercent = &v
	}
	return &out, nil
}

func queryAffiliateByCode(ctx context.Context, client affiliateQueryExecer, code string) (*service.AffiliateSummary, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       aff_code,
       aff_code_custom,
       aff_rebate_rate_percent,
       inviter_id,
       inviter_bound_at,
       invite_bind_source,
       invite_reward_expires_at,
       aff_count,
       aff_quota::double precision,
       aff_frozen_quota::double precision,
       aff_history_quota::double precision,
       created_at,
       updated_at
FROM user_affiliates
WHERE aff_code = $1
  AND (aff_code_expires_at IS NULL OR aff_code_expires_at > NOW())
  AND (aff_code_usage_limit IS NULL OR aff_count < aff_code_usage_limit)
LIMIT 1`, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAffiliateProfileNotFound
	}

	var out service.AffiliateSummary
	var inviterID sql.NullInt64
	var inviterBoundAt sql.NullTime
	var inviteRewardExpiresAt sql.NullTime
	var rebateRate sql.NullFloat64
	var inviteBindSource sql.NullString
	if err := rows.Scan(
		&out.UserID,
		&out.AffCode,
		&out.AffCodeCustom,
		&rebateRate,
		&inviterID,
		&inviterBoundAt,
		&inviteBindSource,
		&inviteRewardExpiresAt,
		&out.AffCount,
		&out.AffQuota,
		&out.AffFrozenQuota,
		&out.AffHistoryQuota,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if inviterID.Valid {
		out.InviterID = &inviterID.Int64
	}
	if inviterBoundAt.Valid {
		t := inviterBoundAt.Time
		out.InviterBoundAt = &t
	}
	if inviteBindSource.Valid {
		out.InviteBindSource = inviteBindSource.String
	}
	if inviteRewardExpiresAt.Valid {
		t := inviteRewardExpiresAt.Time
		out.InviteRewardExpiresAt = &t
	}
	if rebateRate.Valid {
		v := rebateRate.Float64
		out.AffRebateRatePercent = &v
	}
	return &out, nil
}

func queryUserBalance(ctx context.Context, client affiliateQueryExecer, userID int64) (float64, error) {
	rows, err := client.QueryContext(ctx,
		"SELECT balance::double precision FROM users WHERE id = $1 LIMIT 1",
		userID,
	)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, service.ErrUserNotFound
	}
	var balance float64
	if err := rows.Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func generateAffiliateCode() (string, error) {
	buf := make([]byte, affiliateCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate affiliate code: %w", err)
	}
	for i := range buf {
		buf[i] = affiliateCodeCharset[int(buf[i])%len(affiliateCodeCharset)]
	}
	return string(buf), nil
}

func isAffiliateUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == "23505"
	}
	return false
}

// UpdateUserAffCode 改写用户的邀请码（自定义专属邀请码）。
// 唯一性冲突返回 ErrAffiliateCodeTaken。
func (r *affiliateRepository) UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	code := strings.ToUpper(strings.TrimSpace(newCode))
	if code == "" {
		return service.ErrAffiliateCodeInvalid
	}

	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_code = $1,
    aff_code_custom = true,
    updated_at = NOW()
WHERE user_id = $2`, code, userID)
		if err != nil {
			if isAffiliateUniqueViolation(err) {
				return service.ErrAffiliateCodeTaken
			}
			return fmt.Errorf("update aff_code: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

// UpdateUserAffiliateSettings persists all admin-managed exclusive-invite
// fields atomically. The frontend sends explicit clear flags for nullable
// fields so an empty form can intentionally reset a previous value.
func (r *affiliateRepository) UpdateUserAffiliateSettings(ctx context.Context, userID int64, update service.AffiliateUserSettingsUpdate) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}

	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}

		sets := make([]string, 0, 6)
		args := make([]any, 0, 6)
		add := func(expression string, value any) {
			args = append(args, value)
			sets = append(sets, fmt.Sprintf(expression, len(args)))
		}

		if update.AffCode != nil {
			add("aff_code = $%d", *update.AffCode)
			sets = append(sets, "aff_code_custom = true")
		}
		if update.ClearAffCodeUsageLimit {
			sets = append(sets, "aff_code_usage_limit = NULL")
		} else if update.AffCodeUsageLimit != nil {
			add("aff_code_usage_limit = $%d", *update.AffCodeUsageLimit)
		}
		if update.ClearAffCodeExpiresAt {
			sets = append(sets, "aff_code_expires_at = NULL")
		} else if update.AffCodeExpiresAt != nil {
			add("aff_code_expires_at = $%d", *update.AffCodeExpiresAt)
		}
		if update.AffSignupBonusBalance != nil {
			add("aff_signup_bonus_balance = $%d", *update.AffSignupBonusBalance)
		}
		if update.ClearAffAutoGroupID {
			sets = append(sets, "aff_auto_group_id = NULL")
		} else if update.AffAutoGroupID != nil {
			add("aff_auto_group_id = $%d", *update.AffAutoGroupID)
		}
		if len(sets) == 0 {
			return nil
		}
		sets = append(sets, "updated_at = NOW()")

		query := "UPDATE user_affiliates SET " + strings.Join(sets, ", ") + fmt.Sprintf(" WHERE user_id = $%d", len(args)+1)
		args = append(args, userID)
		res, err := txClient.ExecContext(txCtx, query, args...)
		if err != nil {
			if isAffiliateUniqueViolation(err) {
				return service.ErrAffiliateCodeTaken
			}
			return fmt.Errorf("update affiliate settings: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

// ResetUserAffCode 把 aff_code 还原为系统随机码，并清除 aff_code_custom 标记。
func (r *affiliateRepository) ResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", service.ErrUserNotFound
	}
	var newCode string
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		for i := 0; i < affiliateCodeMaxAttempts; i++ {
			candidate, codeErr := generateAffiliateCode()
			if codeErr != nil {
				return codeErr
			}
			res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_code = $1,
    aff_code_custom = false,
    aff_code_usage_limit = NULL,
    aff_code_expires_at = NULL,
    aff_signup_bonus_balance = 0,
    aff_auto_group_id = NULL,
    updated_at = NOW()
WHERE user_id = $2`, candidate, userID)
			if err != nil {
				if isAffiliateUniqueViolation(err) {
					continue
				}
				return fmt.Errorf("reset aff_code: %w", err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return service.ErrUserNotFound
			}
			newCode = candidate
			return nil
		}
		return fmt.Errorf("reset aff_code: exhausted attempts")
	})
	if err != nil {
		return "", err
	}
	return newCode, nil
}

// SetUserRebateRate 设置或清除用户专属返利比例。ratePercent==nil 表示清除（沿用全局）。
func (r *affiliateRepository) SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		// nullableArg lets us use a single UPDATE for both "set value" and
		// "clear" cases — database/sql converts nil interface{} to SQL NULL.
		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_rebate_rate_percent = $1,
    updated_at = NOW()
WHERE user_id = $2`, nullableArg(ratePercent), userID)
		if err != nil {
			return fmt.Errorf("set aff_rebate_rate_percent: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

// BatchSetUserRebateRate 批量为多个用户设置专属比例（nil 清除）。
func (r *affiliateRepository) BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		for _, uid := range userIDs {
			if uid <= 0 {
				continue
			}
			if _, err := ensureUserAffiliateWithClient(txCtx, txClient, uid); err != nil {
				return err
			}
		}
		_, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_rebate_rate_percent = $1,
    updated_at = NOW()
WHERE user_id = ANY($2)`, nullableArg(ratePercent), pq.Array(userIDs))
		if err != nil {
			return fmt.Errorf("batch set aff_rebate_rate_percent: %w", err)
		}
		return nil
	})
}

// nullableArg unwraps a *float64 into an interface{} suitable for SQL parameter
// binding: nil pointer → SQL NULL, non-nil → the float value.
func nullableArg(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableTimeArg(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt64Arg(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

// ListUsersWithCustomSettings 列出有专属邀请码配置的用户。
//
// 单一查询同时处理"无搜索"与"按邮箱/用户名模糊搜索"：
// 空 search 时拼接出的 LIKE 模式为 "%%"，匹配所有行；非空时按 ILIKE 子串匹配。
// 这避免了为两种情况维护两份 SQL 模板。
func (r *affiliateRepository) ListUsersWithCustomSettings(ctx context.Context, filter service.AffiliateAdminFilter) ([]service.AffiliateAdminEntry, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	likePattern := "%" + strings.TrimSpace(filter.Search) + "%"

	const baseFrom = `
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
LEFT JOIN groups g ON g.id = ua.aff_auto_group_id AND g.deleted_at IS NULL
WHERE (ua.aff_code_custom = true OR ua.aff_rebate_rate_percent IS NOT NULL
       OR ua.aff_code_usage_limit IS NOT NULL OR ua.aff_code_expires_at IS NOT NULL
       OR ua.aff_signup_bonus_balance <> 0 OR ua.aff_auto_group_id IS NOT NULL)
  AND (u.email ILIKE $1 OR u.username ILIKE $1)`

	client := clientFromContext(ctx, r.client)

	total, err := scanInt64(ctx, client, "SELECT COUNT(*)"+baseFrom, likePattern)
	if err != nil {
		return nil, 0, fmt.Errorf("count affiliate admin entries: %w", err)
	}

	listQuery := `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ua.aff_code,
       ua.aff_code_custom,
       ua.aff_rebate_rate_percent,
       ua.aff_code_usage_limit,
       ua.aff_code_expires_at,
       ua.aff_signup_bonus_balance::double precision,
       ua.aff_auto_group_id,
       COALESCE(g.name, ''),
       ua.aff_count` + baseFrom + `
ORDER BY ua.updated_at DESC
LIMIT $2 OFFSET $3`

	rows, err := client.QueryContext(ctx, listQuery, likePattern, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate admin entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]service.AffiliateAdminEntry, 0)
	for rows.Next() {
		var e service.AffiliateAdminEntry
		var rebate sql.NullFloat64
		var usageLimit sql.NullInt64
		var expiresAt sql.NullTime
		var signupBonus sql.NullFloat64
		var autoGroupID sql.NullInt64
		if err := rows.Scan(&e.UserID, &e.Email, &e.Username, &e.AffCode,
			&e.AffCodeCustom, &rebate, &usageLimit, &expiresAt, &signupBonus,
			&autoGroupID, &e.AffAutoGroupName, &e.AffCount); err != nil {
			return nil, 0, err
		}
		if rebate.Valid {
			v := rebate.Float64
			e.AffRebateRatePercent = &v
		}
		if usageLimit.Valid {
			v := int(usageLimit.Int64)
			e.AffCodeUsageLimit = &v
		}
		if expiresAt.Valid {
			v := expiresAt.Time
			e.AffCodeExpiresAt = &v
		}
		if signupBonus.Valid {
			e.AffSignupBonusBalance = signupBonus.Float64
		}
		if autoGroupID.Valid {
			e.AffAutoGroupID = &autoGroupID.Int64
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// scanInt64 runs a query expected to return a single int64 column (e.g. COUNT).
func scanInt64(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (int64, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var v int64
	if err := rows.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}
