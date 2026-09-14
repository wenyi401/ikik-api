package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"

	"ikik-api/internal/service"
)

func (r *accountRepository) ListQuotaPoolAccounts(ctx context.Context, ownerUserID int64) ([]service.Account, error) {
	if ownerUserID <= 0 {
		return nil, service.ErrUserNotFound
	}
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("account repository sql executor is unavailable")
	}

	accounts, err := r.listQuotaPoolAccountRows(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return []service.Account{}, nil
	}
	if err := r.loadQuotaPoolAccountGroupRows(ctx, accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *accountRepository) listQuotaPoolAccountRows(ctx context.Context, ownerUserID int64) ([]service.Account, error) {
	rows, err := r.sql.QueryContext(ctx, `
		WITH quota_pool_account_ids AS (
			SELECT id
			FROM accounts
			WHERE deleted_at IS NULL
				AND owner_user_id = $1
			UNION
			SELECT a.id
			FROM account_groups ag
			JOIN groups g ON g.id = ag.group_id
			JOIN accounts a ON a.id = ag.account_id
			WHERE a.deleted_at IS NULL
				AND g.deleted_at IS NULL
				AND g.is_exclusive = false
				AND g.owner_user_id IS NULL
				AND g.scope = 'public'
				AND COALESCE(g.subscription_type, '') IN ('', 'standard')
				AND (
					a.owner_user_id IS NULL
					OR (
						a.owner_user_id IS NOT NULL
						AND a.share_mode = 'public'
						AND a.share_status = 'approved'
					)
				)
		)
		SELECT
			a.id,
			a.name,
			a.platform,
			a.account_level,
			a.type,
			a.extra->>'quota_limit',
			a.extra->>'quota_used',
			a.extra->>'quota_daily_limit',
			a.extra->>'quota_daily_used',
			a.extra->>'quota_daily_start',
			a.extra->>'quota_daily_reset_mode',
			a.extra->>'quota_daily_reset_at',
			a.extra->>'quota_weekly_limit',
			a.extra->>'quota_weekly_used',
			a.extra->>'quota_weekly_start',
			a.extra->>'quota_weekly_reset_mode',
			a.extra->>'quota_weekly_reset_at',
			a.extra->>'codex_5h_used_percent',
			a.extra->>'codex_5h_reset_after_seconds',
			a.extra->>'codex_5h_reset_at',
			a.extra->>'codex_7d_used_percent',
			a.extra->>'codex_7d_reset_after_seconds',
			a.extra->>'codex_7d_reset_at',
			a.extra->>'codex_usage_updated_at',
			a.extra->>'privacy_mode',
			a.owner_user_id,
			a.share_mode,
			a.share_status,
			a.concurrency,
			a.status,
			a.expires_at,
			a.auto_pause_on_expired,
			a.schedulable,
			a.rate_limit_reset_at,
			a.overload_until,
			a.temp_unschedulable_until
		FROM accounts a
		JOIN quota_pool_account_ids q ON q.id = a.id
		ORDER BY a.id
	`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	accounts := make([]service.Account, 0)
	for rows.Next() {
		var account service.Account
		var accountOwnerUserID sql.NullInt64
		var quotaLimit, quotaUsed, quotaDailyLimit, quotaDailyUsed sql.NullString
		var quotaDailyStart, quotaDailyResetMode, quotaDailyResetAt sql.NullString
		var quotaWeeklyLimit, quotaWeeklyUsed, quotaWeeklyStart sql.NullString
		var quotaWeeklyResetMode, quotaWeeklyResetAt sql.NullString
		var codex5hUsedPercent, codex5hResetAfterSeconds, codex5hResetAt sql.NullString
		var codex7dUsedPercent, codex7dResetAfterSeconds, codex7dResetAt sql.NullString
		var codexUsageUpdatedAt, privacyMode sql.NullString
		if err := rows.Scan(
			&account.ID,
			&account.Name,
			&account.Platform,
			&account.AccountLevel,
			&account.Type,
			&quotaLimit,
			&quotaUsed,
			&quotaDailyLimit,
			&quotaDailyUsed,
			&quotaDailyStart,
			&quotaDailyResetMode,
			&quotaDailyResetAt,
			&quotaWeeklyLimit,
			&quotaWeeklyUsed,
			&quotaWeeklyStart,
			&quotaWeeklyResetMode,
			&quotaWeeklyResetAt,
			&codex5hUsedPercent,
			&codex5hResetAfterSeconds,
			&codex5hResetAt,
			&codex7dUsedPercent,
			&codex7dResetAfterSeconds,
			&codex7dResetAt,
			&codexUsageUpdatedAt,
			&privacyMode,
			&accountOwnerUserID,
			&account.ShareMode,
			&account.ShareStatus,
			&account.Concurrency,
			&account.Status,
			&account.ExpiresAt,
			&account.AutoPauseOnExpired,
			&account.Schedulable,
			&account.RateLimitResetAt,
			&account.OverloadUntil,
			&account.TempUnschedulableUntil,
		); err != nil {
			return nil, err
		}
		if accountOwnerUserID.Valid {
			account.OwnerUserID = &accountOwnerUserID.Int64
		}
		account.AccountLevel = service.NormalizeAccountLevel(account.AccountLevel)
		account.ShareMode = service.NormalizeAccountShareMode(account.ShareMode)
		account.ShareStatus = service.NormalizeAccountShareStatus(account.ShareStatus)
		account.Extra = map[string]any{}
		setNullStringExtra(account.Extra, "quota_limit", quotaLimit)
		setNullStringExtra(account.Extra, "quota_used", quotaUsed)
		setNullStringExtra(account.Extra, "quota_daily_limit", quotaDailyLimit)
		setNullStringExtra(account.Extra, "quota_daily_used", quotaDailyUsed)
		setNullStringExtra(account.Extra, "quota_daily_start", quotaDailyStart)
		setNullStringExtra(account.Extra, "quota_daily_reset_mode", quotaDailyResetMode)
		setNullStringExtra(account.Extra, "quota_daily_reset_at", quotaDailyResetAt)
		setNullStringExtra(account.Extra, "quota_weekly_limit", quotaWeeklyLimit)
		setNullStringExtra(account.Extra, "quota_weekly_used", quotaWeeklyUsed)
		setNullStringExtra(account.Extra, "quota_weekly_start", quotaWeeklyStart)
		setNullStringExtra(account.Extra, "quota_weekly_reset_mode", quotaWeeklyResetMode)
		setNullStringExtra(account.Extra, "quota_weekly_reset_at", quotaWeeklyResetAt)
		setNullStringExtra(account.Extra, "codex_5h_used_percent", codex5hUsedPercent)
		setNullStringExtra(account.Extra, "codex_5h_reset_after_seconds", codex5hResetAfterSeconds)
		setNullStringExtra(account.Extra, "codex_5h_reset_at", codex5hResetAt)
		setNullStringExtra(account.Extra, "codex_7d_used_percent", codex7dUsedPercent)
		setNullStringExtra(account.Extra, "codex_7d_reset_after_seconds", codex7dResetAfterSeconds)
		setNullStringExtra(account.Extra, "codex_7d_reset_at", codex7dResetAt)
		setNullStringExtra(account.Extra, "codex_usage_updated_at", codexUsageUpdatedAt)
		setNullStringExtra(account.Extra, "privacy_mode", privacyMode)
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *accountRepository) loadQuotaPoolAccountGroupRows(ctx context.Context, accounts []service.Account) error {
	byID := make(map[int64]*service.Account, len(accounts))
	accountIDs := make([]int64, 0, len(accounts))
	for i := range accounts {
		byID[accounts[i].ID] = &accounts[i]
		accountIDs = append(accountIDs, accounts[i].ID)
	}

	return forEachAccountRepositoryIDBatch(accountIDs, func(batch []int64) error {
		rows, err := r.sql.QueryContext(ctx, `
			SELECT
				ag.account_id,
				ag.group_id,
				ag.priority,
				ag.created_at,
				g.name,
				g.platform,
				g.rate_multiplier,
				g.is_exclusive,
				g.status,
				g.owner_user_id,
				g.scope,
				g.subscription_type,
				g.required_account_level,
				g.require_oauth_only,
				g.require_privacy_set
			FROM account_groups ag
			JOIN groups g ON g.id = ag.group_id
			WHERE ag.account_id = ANY($1)
				AND g.deleted_at IS NULL
			ORDER BY ag.account_id, ag.priority
		`, pq.Array(batch))
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var accountID int64
			var group service.Group
			var accountGroup service.AccountGroup
			var groupOwnerUserID sql.NullInt64
			if err := rows.Scan(
				&accountID,
				&group.ID,
				&accountGroup.Priority,
				&accountGroup.CreatedAt,
				&group.Name,
				&group.Platform,
				&group.RateMultiplier,
				&group.IsExclusive,
				&group.Status,
				&groupOwnerUserID,
				&group.Scope,
				&group.SubscriptionType,
				&group.RequiredAccountLevel,
				&group.RequireOAuthOnly,
				&group.RequirePrivacySet,
			); err != nil {
				return err
			}
			if groupOwnerUserID.Valid {
				group.OwnerUserID = &groupOwnerUserID.Int64
			}
			group.Hydrated = true
			group.Scope = service.NormalizeGroupScope(group.Scope)
			group.RequiredAccountLevel = service.NormalizeRequiredAccountLevel(group.RequiredAccountLevel)

			account, ok := byID[accountID]
			if !ok {
				continue
			}
			accountGroup.AccountID = accountID
			accountGroup.GroupID = group.ID
			accountGroup.Group = &group
			account.AccountGroups = append(account.AccountGroups, accountGroup)
			account.GroupIDs = append(account.GroupIDs, group.ID)
			account.Groups = append(account.Groups, &group)
		}
		return rows.Err()
	})
}

func setNullStringExtra(extra map[string]any, key string, value sql.NullString) {
	if !value.Valid || value.String == "" {
		return
	}
	extra[key] = value.String
}
