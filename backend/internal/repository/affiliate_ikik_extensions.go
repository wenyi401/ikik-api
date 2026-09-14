package repository

import (
	"context"

	"database/sql"

	"fmt"

	"github.com/lib/pq"

	dbent "ikik-api/ent"
	"ikik-api/internal/service"
)

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
