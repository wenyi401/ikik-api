package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "ikik-api/ent"
	"ikik-api/internal/service"

	"github.com/lib/pq"
)

type carpoolRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewCarpoolRepository(client *dbent.Client, sqlDB *sql.DB) service.CarpoolRepository {
	return &carpoolRepository{client: client, db: sqlDB}
}

func (r *carpoolRepository) CreatePool(ctx context.Context, input service.CreateCarpoolPoolInput) (*service.CarpoolPool, error) {
	const query = `
		INSERT INTO carpool_pools (
			owner_user_id, invite_code, name, platform, status, visibility,
			target_seats, duration_days, seat_price, extra_fee, extra_fee_description,
			system_proxy_enabled, risk_control_enabled, notes,
			total_five_hour_limit_usd, total_weekly_limit_usd,
			per_member_five_hour_limit_usd, per_member_weekly_limit_usd, quota_snapshot_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14,
			$15, $16, $17, $18, $19
		)
		RETURNING
			id, owner_user_id, group_id, invite_code, name, platform, status, visibility,
			target_seats, duration_days, seat_price, extra_fee, extra_fee_description,
			system_proxy_enabled, risk_control_enabled, notes,
			total_five_hour_limit_usd, total_weekly_limit_usd,
			per_member_five_hour_limit_usd, per_member_weekly_limit_usd,
			quota_snapshot_at, created_at, updated_at
	`
	row := r.db.QueryRowContext(
		ctx,
		query,
		input.OwnerUserID,
		input.InviteCode,
		input.Name,
		input.Platform,
		service.CarpoolPoolStatusRecruiting,
		service.NormalizeCarpoolPoolVisibility(input.Visibility),
		input.TargetSeats,
		input.DurationDays,
		input.SeatPrice,
		input.ExtraFee,
		input.ExtraFeeDescription,
		input.SystemProxyEnabled,
		input.RiskControlEnabled,
		input.Notes,
		input.InitialQuotaSnapshot.TotalFiveHourLimitUSD,
		input.InitialQuotaSnapshot.TotalWeeklyLimitUSD,
		input.InitialQuotaSnapshot.PerMemberFiveHourLimitUSD,
		input.InitialQuotaSnapshot.PerMemberWeeklyLimitUSD,
		input.InitialQuotaSnapshot.SnapshotAt,
	)
	pool, err := scanCarpoolPool(row)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *carpoolRepository) UpdatePoolGroupAndQuota(ctx context.Context, poolID int64, groupID *int64, totals service.CarpoolQuotaSnapshot) (*service.CarpoolPool, error) {
	const query = `
		UPDATE carpool_pools
		SET
			group_id = $2,
			total_five_hour_limit_usd = $3,
			total_weekly_limit_usd = $4,
			per_member_five_hour_limit_usd = $5,
			per_member_weekly_limit_usd = $6,
			quota_snapshot_at = $7,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
			RETURNING
				id, owner_user_id, group_id, invite_code, name, platform, status, visibility,
				target_seats, duration_days, seat_price, extra_fee, extra_fee_description,
				system_proxy_enabled, risk_control_enabled, notes,
				total_five_hour_limit_usd, total_weekly_limit_usd,
				per_member_five_hour_limit_usd, per_member_weekly_limit_usd,
				quota_snapshot_at, created_at, updated_at
	`
	pool, err := scanCarpoolPool(
		r.db.QueryRowContext(ctx, query, poolID, groupID, totals.TotalFiveHourLimitUSD, totals.TotalWeeklyLimitUSD, totals.PerMemberFiveHourLimitUSD, totals.PerMemberWeeklyLimitUSD, totals.SnapshotAt),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolPoolNotFound
	}
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *carpoolRepository) UpdatePoolStatus(ctx context.Context, poolID int64, status string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE carpool_pools
		SET status = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, poolID, status)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrCarpoolPoolNotFound
	}
	return nil
}

func (r *carpoolRepository) DeletePool(ctx context.Context, poolID int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE carpool_pools
		SET status = 'closed', deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, poolID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrCarpoolPoolNotFound
	}
	return nil
}

func (r *carpoolRepository) GetPoolByID(ctx context.Context, poolID int64) (*service.CarpoolPool, error) {
	const query = `
		SELECT
			id, owner_user_id, group_id, invite_code, name, platform, status, visibility,
			target_seats, duration_days, seat_price, extra_fee, extra_fee_description,
			system_proxy_enabled, risk_control_enabled, notes,
			total_five_hour_limit_usd, total_weekly_limit_usd,
			per_member_five_hour_limit_usd, per_member_weekly_limit_usd,
			quota_snapshot_at, created_at, updated_at
		FROM carpool_pools
		WHERE id = $1 AND deleted_at IS NULL
	`
	pool, err := scanCarpoolPool(r.db.QueryRowContext(ctx, query, poolID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolPoolNotFound
	}
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *carpoolRepository) GetPoolByGroupID(ctx context.Context, groupID int64) (*service.CarpoolPool, error) {
	const query = `
		SELECT
			id, owner_user_id, group_id, invite_code, name, platform, status, visibility,
			target_seats, duration_days, seat_price, extra_fee, extra_fee_description,
			system_proxy_enabled, risk_control_enabled, notes,
			total_five_hour_limit_usd, total_weekly_limit_usd,
			per_member_five_hour_limit_usd, per_member_weekly_limit_usd,
			quota_snapshot_at, created_at, updated_at
		FROM carpool_pools
		WHERE group_id = $1 AND deleted_at IS NULL
	`
	pool, err := scanCarpoolPool(r.db.QueryRowContext(ctx, query, groupID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolPoolNotFound
	}
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *carpoolRepository) GetPoolByInviteCode(ctx context.Context, inviteCode string) (*service.CarpoolPool, error) {
	code := strings.ToUpper(strings.TrimSpace(inviteCode))
	if code == "" {
		return nil, service.ErrCarpoolInviteCodeRequired
	}
	const query = `
		SELECT
			id, owner_user_id, group_id, invite_code, name, platform, status, visibility,
			target_seats, duration_days, seat_price, extra_fee, extra_fee_description,
			system_proxy_enabled, risk_control_enabled, notes,
			total_five_hour_limit_usd, total_weekly_limit_usd,
			per_member_five_hour_limit_usd, per_member_weekly_limit_usd,
			quota_snapshot_at, created_at, updated_at
		FROM carpool_pools
		WHERE UPPER(invite_code) = $1 AND deleted_at IS NULL
	`
	pool, err := scanCarpoolPool(r.db.QueryRowContext(ctx, query, code))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolPoolNotFound
	}
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *carpoolRepository) ListAdminPools(ctx context.Context, filters service.AdminCarpoolPoolFilters) ([]service.AdminCarpoolPoolSummary, int64, error) {
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.PageSize <= 0 {
		filters.PageSize = 20
	}
	if filters.PageSize > 1000 {
		filters.PageSize = 1000
	}

	where := []string{"p.deleted_at IS NULL"}
	args := make([]any, 0, 6)
	addArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filters.Platform != "" {
		where = append(where, "p.platform = "+addArg(filters.Platform))
	}
	if filters.Status != "" {
		where = append(where, "p.status = "+addArg(filters.Status))
	}
	if filters.OwnerUserID > 0 {
		where = append(where, "p.owner_user_id = "+addArg(filters.OwnerUserID))
	}
	if search := strings.TrimSpace(filters.Search); search != "" {
		placeholder := addArg("%" + search + "%")
		where = append(where, fmt.Sprintf(
			"(p.name ILIKE %s OR p.invite_code ILIKE %s OR COALESCE(g.name, '') ILIKE %s OR COALESCE(u.email, '') ILIKE %s OR COALESCE(u.username, '') ILIKE %s)",
			placeholder,
			placeholder,
			placeholder,
			placeholder,
			placeholder,
		))
	}

	whereSQL := strings.Join(where, " AND ")
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM carpool_pools p
		LEFT JOIN groups g ON g.id = p.group_id
		LEFT JOIN users u ON u.id = p.owner_user_id
		WHERE %s
	`, whereSQL)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (filters.Page - 1) * filters.PageSize
	limitPlaceholder := addArg(filters.PageSize)
	offsetPlaceholder := addArg(offset)
	query := fmt.Sprintf(`
		SELECT
			p.id, p.owner_user_id, p.group_id, p.invite_code, p.name, p.platform, p.status, p.visibility,
			p.target_seats, p.duration_days, p.seat_price, p.extra_fee, p.extra_fee_description,
			p.system_proxy_enabled, p.risk_control_enabled, p.notes,
			p.total_five_hour_limit_usd, p.total_weekly_limit_usd,
			p.per_member_five_hour_limit_usd, p.per_member_weekly_limit_usd,
			p.quota_snapshot_at, p.created_at, p.updated_at,
			COALESCE(g.name, '') AS group_name,
			COALESCE((SELECT COUNT(*) FROM carpool_members m WHERE m.pool_id = p.id AND m.deleted_at IS NULL AND m.status = 'active'), 0) AS active_members,
			COALESCE((SELECT COUNT(*) FROM carpool_join_requests jr WHERE jr.pool_id = p.id AND jr.deleted_at IS NULL AND jr.status IN ('pending', 'approved')), 0) AS pending_applications,
			COALESCE((SELECT COUNT(*) FROM carpool_pool_accounts pa WHERE pa.pool_id = p.id), 0) AS bound_account_count,
			TRUE AS is_owner,
			'owner' AS current_user_status,
			NULL::bigint AS current_user_request_id,
			COALESCE(u.email, '') AS owner_email,
			COALESCE(u.username, '') AS owner_username
		FROM carpool_pools p
		LEFT JOIN groups g ON g.id = p.group_id
		LEFT JOIN users u ON u.id = p.owner_user_id
		WHERE %s
		ORDER BY p.id DESC
		LIMIT %s OFFSET %s
	`, whereSQL, limitPlaceholder, offsetPlaceholder)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AdminCarpoolPoolSummary, 0)
	for rows.Next() {
		item, err := scanAdminCarpoolPoolSummaryFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *carpoolRepository) ListOwnedPools(ctx context.Context, ownerUserID int64) ([]service.CarpoolPoolSummary, error) {
	return r.listPoolSummaries(ctx, `
		SELECT
			p.id, p.owner_user_id, p.group_id, p.invite_code, p.name, p.platform, p.status, p.visibility,
			p.target_seats, p.duration_days, p.seat_price, p.extra_fee, p.extra_fee_description,
			p.system_proxy_enabled, p.risk_control_enabled, p.notes,
			p.total_five_hour_limit_usd, p.total_weekly_limit_usd,
			p.per_member_five_hour_limit_usd, p.per_member_weekly_limit_usd,
			p.quota_snapshot_at, p.created_at, p.updated_at,
			COALESCE(g.name, '') AS group_name,
			COALESCE((SELECT COUNT(*) FROM carpool_members m WHERE m.pool_id = p.id AND m.deleted_at IS NULL AND m.status = 'active'), 0) AS active_members,
			COALESCE((SELECT COUNT(*) FROM carpool_join_requests jr WHERE jr.pool_id = p.id AND jr.deleted_at IS NULL AND jr.status IN ('pending', 'approved')), 0) AS pending_applications,
			COALESCE((SELECT COUNT(*) FROM carpool_pool_accounts pa WHERE pa.pool_id = p.id), 0) AS bound_account_count,
			TRUE AS is_owner,
			'owner' AS current_user_status,
			NULL::bigint AS current_user_request_id
		FROM carpool_pools p
		LEFT JOIN groups g ON g.id = p.group_id
		WHERE p.owner_user_id = $1 AND p.deleted_at IS NULL
		ORDER BY p.id DESC
	`, ownerUserID)
}

func (r *carpoolRepository) ListJoinedPools(ctx context.Context, userID int64) ([]service.CarpoolPoolSummary, error) {
	return r.listPoolSummaries(ctx, `
		SELECT
			p.id, p.owner_user_id, p.group_id, p.invite_code, p.name, p.platform, p.status, p.visibility,
			p.target_seats, p.duration_days, p.seat_price, p.extra_fee, p.extra_fee_description,
			p.system_proxy_enabled, p.risk_control_enabled, p.notes,
			p.total_five_hour_limit_usd, p.total_weekly_limit_usd,
			p.per_member_five_hour_limit_usd, p.per_member_weekly_limit_usd,
			p.quota_snapshot_at, p.created_at, p.updated_at,
			COALESCE(g.name, '') AS group_name,
			COALESCE((SELECT COUNT(*) FROM carpool_members x WHERE x.pool_id = p.id AND x.deleted_at IS NULL AND x.status = 'active'), 0) AS active_members,
			COALESCE((SELECT COUNT(*) FROM carpool_join_requests jr WHERE jr.pool_id = p.id AND jr.deleted_at IS NULL AND jr.status IN ('pending', 'approved')), 0) AS pending_applications,
			COALESCE((SELECT COUNT(*) FROM carpool_pool_accounts pa WHERE pa.pool_id = p.id), 0) AS bound_account_count,
			FALSE AS is_owner,
			m.status AS current_user_status,
			NULL::bigint AS current_user_request_id
		FROM carpool_members m
		INNER JOIN carpool_pools p ON p.id = m.pool_id
		LEFT JOIN groups g ON g.id = p.group_id
		WHERE m.user_id = $1
			AND m.deleted_at IS NULL
			AND m.status = 'active'
			AND p.deleted_at IS NULL
			AND p.owner_user_id <> $1
		ORDER BY p.id DESC
	`, userID)
}

func (r *carpoolRepository) ListHallPools(ctx context.Context, userID int64) ([]service.CarpoolPoolSummary, error) {
	return r.listPoolSummaries(ctx, `
		SELECT
			p.id, p.owner_user_id, p.group_id, p.invite_code, p.name, p.platform, p.status, p.visibility,
			p.target_seats, p.duration_days, p.seat_price, p.extra_fee, p.extra_fee_description,
			p.system_proxy_enabled, p.risk_control_enabled, p.notes,
			p.total_five_hour_limit_usd, p.total_weekly_limit_usd,
			p.per_member_five_hour_limit_usd, p.per_member_weekly_limit_usd,
			p.quota_snapshot_at, p.created_at, p.updated_at,
			COALESCE(g.name, '') AS group_name,
			COALESCE((SELECT COUNT(*) FROM carpool_members x WHERE x.pool_id = p.id AND x.deleted_at IS NULL AND x.status = 'active'), 0) AS active_members,
			COALESCE((SELECT COUNT(*) FROM carpool_join_requests jr WHERE jr.pool_id = p.id AND jr.deleted_at IS NULL AND jr.status IN ('pending', 'approved')), 0) AS pending_applications,
			COALESCE((SELECT COUNT(*) FROM carpool_pool_accounts pa WHERE pa.pool_id = p.id), 0) AS bound_account_count,
			FALSE AS is_owner,
			COALESCE(
				(SELECT m.status FROM carpool_members m WHERE m.pool_id = p.id AND m.user_id = $1 AND m.deleted_at IS NULL LIMIT 1),
				(SELECT jr.status FROM carpool_join_requests jr WHERE jr.pool_id = p.id AND jr.user_id = $1 AND jr.deleted_at IS NULL AND jr.status IN ('pending', 'approved') ORDER BY jr.id DESC LIMIT 1),
				''
			) AS current_user_status,
			(SELECT jr.id FROM carpool_join_requests jr WHERE jr.pool_id = p.id AND jr.user_id = $1 AND jr.deleted_at IS NULL AND jr.status IN ('pending', 'approved') ORDER BY jr.id DESC LIMIT 1) AS current_user_request_id
		FROM carpool_pools p
		LEFT JOIN groups g ON g.id = p.group_id
		WHERE p.deleted_at IS NULL
			AND p.visibility = 'public'
			AND p.status = 'recruiting'
			AND p.owner_user_id <> $1
		ORDER BY p.id DESC
	`, userID)
}

func (r *carpoolRepository) ListPoolAccounts(ctx context.Context, poolID int64) ([]service.CarpoolPoolAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			pa.id,
			pa.pool_id,
			pa.account_id,
			a.name,
			a.platform,
			a.type,
			a.account_level,
			a.status,
			pa.external_5h_used_usd,
			pa.external_weekly_used_usd,
			pa.external_5h_reset_at,
			pa.external_weekly_reset_at,
			pa.external_checked_at,
			pa.external_overage_notified_at,
			pa.created_at
		FROM carpool_pool_accounts pa
		INNER JOIN accounts a ON a.id = pa.account_id
		WHERE pa.pool_id = $1 AND a.deleted_at IS NULL
		ORDER BY pa.id ASC
	`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.CarpoolPoolAccount, 0)
	for rows.Next() {
		var item service.CarpoolPoolAccount
		var fiveHourResetAt, weeklyResetAt, checkedAt, notifiedAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.PoolID,
			&item.AccountID,
			&item.Name,
			&item.Platform,
			&item.Type,
			&item.AccountLevel,
			&item.Status,
			&item.ExternalFiveHourUsedUSD,
			&item.ExternalWeeklyUsedUSD,
			&fiveHourResetAt,
			&weeklyResetAt,
			&checkedAt,
			&notifiedAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if fiveHourResetAt.Valid {
			item.ExternalFiveHourResetAt = &fiveHourResetAt.Time
		}
		if weeklyResetAt.Valid {
			item.ExternalWeeklyResetAt = &weeklyResetAt.Time
		}
		if checkedAt.Valid {
			item.ExternalCheckedAt = &checkedAt.Time
		}
		if notifiedAt.Valid {
			item.ExternalOverageNotifiedAt = &notifiedAt.Time
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *carpoolRepository) FindActivePoolByAccountID(ctx context.Context, accountID, excludePoolID int64) (*service.CarpoolPool, error) {
	const query = `
		SELECT
			p.id, p.owner_user_id, p.group_id, p.invite_code, p.name, p.platform, p.status, p.visibility,
			p.target_seats, p.duration_days, p.seat_price, p.extra_fee, p.extra_fee_description,
			p.system_proxy_enabled, p.risk_control_enabled, p.notes,
			p.total_five_hour_limit_usd, p.total_weekly_limit_usd,
			p.per_member_five_hour_limit_usd, p.per_member_weekly_limit_usd,
			p.quota_snapshot_at, p.created_at, p.updated_at
		FROM carpool_pool_accounts pa
		INNER JOIN carpool_pools p ON p.id = pa.pool_id
		WHERE pa.account_id = $1
			AND p.deleted_at IS NULL
			AND p.status <> 'closed'
			AND p.id <> $2
		ORDER BY p.id DESC
		LIMIT 1
	`
	pool, err := scanCarpoolPool(r.db.QueryRowContext(ctx, query, accountID, excludePoolID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *carpoolRepository) ReplacePoolAccounts(ctx context.Context, poolID int64, accountIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM carpool_pool_accounts WHERE pool_id = $1`, poolID); err != nil {
		return err
	}
	for _, accountID := range uniquePositiveInt64s(accountIDs) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO carpool_pool_accounts (pool_id, account_id)
			VALUES ($1, $2)
		`, poolID, accountID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *carpoolRepository) ListPoolMembers(ctx context.Context, poolID int64) ([]service.CarpoolMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pool_id, user_id, subscription_id, role, status, paid_confirmed_at,
			quota_share_ratio, five_hour_limit_usd, five_hour_used_usd, weekly_limit_usd, five_hour_window_start, created_at, updated_at
		FROM carpool_members
		WHERE pool_id = $1 AND deleted_at IS NULL
		ORDER BY CASE WHEN role = 'owner' THEN 0 ELSE 1 END, id ASC
	`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.CarpoolMember, 0)
	for rows.Next() {
		member, err := scanCarpoolMemberFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *member)
	}
	return out, rows.Err()
}

func (r *carpoolRepository) ListPoolJoinRequests(ctx context.Context, poolID int64) ([]service.CarpoolJoinRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pool_id, user_id, status, note, review_note, reviewed_at, activated_at, created_at, updated_at
		FROM carpool_join_requests
		WHERE pool_id = $1 AND deleted_at IS NULL
		ORDER BY CASE WHEN status = 'pending' THEN 0 WHEN status = 'approved' THEN 1 ELSE 2 END, id DESC
	`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.CarpoolJoinRequest, 0)
	for rows.Next() {
		item, err := scanCarpoolJoinRequestFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *carpoolRepository) GetJoinRequestByID(ctx context.Context, requestID int64) (*service.CarpoolJoinRequest, error) {
	item, err := scanCarpoolJoinRequest(
		r.db.QueryRowContext(ctx, `
			SELECT id, pool_id, user_id, status, note, review_note, reviewed_at, activated_at, created_at, updated_at
			FROM carpool_join_requests
			WHERE id = $1 AND deleted_at IS NULL
		`, requestID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolJoinRequestNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *carpoolRepository) GetOpenJoinRequestByPoolAndUser(ctx context.Context, poolID, userID int64) (*service.CarpoolJoinRequest, error) {
	item, err := scanCarpoolJoinRequest(
		r.db.QueryRowContext(ctx, `
			SELECT id, pool_id, user_id, status, note, review_note, reviewed_at, activated_at, created_at, updated_at
			FROM carpool_join_requests
			WHERE pool_id = $1 AND user_id = $2 AND deleted_at IS NULL AND status IN ('pending', 'approved')
			ORDER BY id DESC
			LIMIT 1
		`, poolID, userID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *carpoolRepository) CreateJoinRequest(ctx context.Context, poolID, userID int64, note string) (*service.CarpoolJoinRequest, error) {
	item, err := scanCarpoolJoinRequest(
		r.db.QueryRowContext(ctx, `
			INSERT INTO carpool_join_requests (pool_id, user_id, status, note, review_note)
			VALUES ($1, $2, 'pending', $3, '')
			RETURNING id, pool_id, user_id, status, note, review_note, reviewed_at, activated_at, created_at, updated_at
		`, poolID, userID, note),
	)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *carpoolRepository) UpdateJoinRequestStatus(ctx context.Context, requestID int64, status, reviewNote string, reviewedAt time.Time) (*service.CarpoolJoinRequest, error) {
	item, err := scanCarpoolJoinRequest(
		r.db.QueryRowContext(ctx, `
			UPDATE carpool_join_requests
			SET status = $2, review_note = $3, reviewed_at = $4, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
			RETURNING id, pool_id, user_id, status, note, review_note, reviewed_at, activated_at, created_at, updated_at
		`, requestID, status, reviewNote, reviewedAt),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolJoinRequestNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *carpoolRepository) ActivateJoinRequest(ctx context.Context, requestID int64, activatedAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE carpool_join_requests
		SET status = 'activated', activated_at = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, requestID, activatedAt)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrCarpoolJoinRequestNotFound
	}
	return nil
}

func (r *carpoolRepository) UpsertMember(ctx context.Context, input service.UpsertCarpoolMemberInput) (*service.CarpoolMember, error) {
	item, err := scanCarpoolMember(
		r.db.QueryRowContext(ctx, `
			INSERT INTO carpool_members (
				pool_id, user_id, subscription_id, role, status, paid_confirmed_at,
				quota_share_ratio, five_hour_limit_usd, five_hour_used_usd, weekly_limit_usd, five_hour_window_start, deleted_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, $9, NULL, NULL)
			ON CONFLICT (pool_id, user_id) DO UPDATE
			SET
				subscription_id = EXCLUDED.subscription_id,
				role = EXCLUDED.role,
				status = EXCLUDED.status,
				paid_confirmed_at = EXCLUDED.paid_confirmed_at,
				quota_share_ratio = EXCLUDED.quota_share_ratio,
				five_hour_limit_usd = EXCLUDED.five_hour_limit_usd,
				five_hour_used_usd = CASE WHEN $10 THEN 0 ELSE carpool_members.five_hour_used_usd END,
				weekly_limit_usd = EXCLUDED.weekly_limit_usd,
				five_hour_window_start = CASE WHEN $10 THEN NULL ELSE carpool_members.five_hour_window_start END,
				deleted_at = NULL,
				updated_at = NOW()
			RETURNING id, pool_id, user_id, subscription_id, role, status, paid_confirmed_at,
				quota_share_ratio, five_hour_limit_usd, five_hour_used_usd, weekly_limit_usd, five_hour_window_start, created_at, updated_at
		`, input.PoolID, input.UserID, input.SubscriptionID, input.Role, input.Status, input.PaidConfirmedAt, input.QuotaShareRatio, input.FiveHourLimitUSD, input.WeeklyLimitUSD, input.ResetFiveHourUsage),
	)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *carpoolRepository) UpdateMembersFiveHourLimit(ctx context.Context, poolID int64, limitUSD float64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE carpool_members
		SET five_hour_limit_usd = $2, updated_at = NOW()
		WHERE pool_id = $1 AND deleted_at IS NULL
	`, poolID, limitUSD)
	return err
}

func (r *carpoolRepository) UpdateMembersQuotaFromSnapshot(ctx context.Context, poolID int64, snapshot service.CarpoolQuotaSnapshot, defaultShareRatio float64) error {
	if defaultShareRatio < 0 {
		defaultShareRatio = 0
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE carpool_members
		SET
			quota_share_ratio = CASE WHEN quota_share_ratio > 0 THEN quota_share_ratio ELSE $4 END,
			five_hour_limit_usd = $2 * CASE WHEN quota_share_ratio > 0 THEN quota_share_ratio ELSE $4 END,
			weekly_limit_usd = $3 * CASE WHEN quota_share_ratio > 0 THEN quota_share_ratio ELSE $4 END,
			updated_at = NOW()
		WHERE pool_id = $1 AND deleted_at IS NULL AND status = 'active'
	`, poolID, snapshot.TotalFiveHourLimitUSD, snapshot.TotalWeeklyLimitUSD, defaultShareRatio)
	return err
}

func (r *carpoolRepository) UpdateMemberAllocations(ctx context.Context, poolID int64, updates []service.CarpoolMemberAllocationUpdate) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, update := range updates {
		res, err := tx.ExecContext(ctx, `
			UPDATE carpool_members
			SET
				quota_share_ratio = $3,
				five_hour_limit_usd = $4,
				weekly_limit_usd = $5,
				updated_at = NOW()
			WHERE id = $2
				AND pool_id = $1
				AND deleted_at IS NULL
				AND status = 'active'
		`, poolID, update.MemberID, update.QuotaShareRatio, update.FiveHourLimitUSD, update.WeeklyLimitUSD)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return service.ErrCarpoolMemberNotFound
		}
	}
	return tx.Commit()
}

func (r *carpoolRepository) ResetPoolMembersFiveHourUsage(ctx context.Context, poolID int64, windowStart *time.Time) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		UPDATE carpool_members
		SET
			five_hour_used_usd = 0,
			five_hour_window_start = $2,
			updated_at = NOW()
		WHERE pool_id = $1
			AND deleted_at IS NULL
			AND status = 'active'
		RETURNING user_id
	`, poolID, windowStart)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	userIDs := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

func (r *carpoolRepository) ResetPoolMemberWeeklyUsage(ctx context.Context, poolID int64, windowStart time.Time) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		UPDATE user_subscriptions AS us
		SET
			weekly_usage_usd = 0,
			weekly_window_start = $2,
			updated_at = NOW()
		FROM carpool_members AS m
		WHERE m.pool_id = $1
			AND m.subscription_id = us.id
			AND m.deleted_at IS NULL
			AND m.status = 'active'
			AND us.deleted_at IS NULL
		RETURNING us.user_id
	`, poolID, windowStart)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	userIDs := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

func (r *carpoolRepository) IncrementOwnerMemberFiveHourUsage(ctx context.Context, poolID, ownerUserID int64, costUSD float64, occurredAt time.Time) (*service.CarpoolMember, error) {
	if poolID <= 0 || ownerUserID <= 0 || costUSD <= 0 {
		return r.GetMemberByPoolAndUser(ctx, poolID, ownerUserID)
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	item, err := scanCarpoolMember(
		r.db.QueryRowContext(ctx, `
			UPDATE carpool_members AS m
			SET
				five_hour_used_usd = CASE
					WHEN m.five_hour_window_start IS NULL OR m.five_hour_window_start + INTERVAL '5 hours' <= $4
						THEN $3
					ELSE COALESCE(m.five_hour_used_usd, 0) + $3
				END,
				five_hour_window_start = CASE
					WHEN m.five_hour_window_start IS NULL OR m.five_hour_window_start + INTERVAL '5 hours' <= $4
						THEN $4
					ELSE m.five_hour_window_start
				END,
				updated_at = NOW()
			WHERE m.pool_id = $1
				AND m.user_id = $2
				AND m.role = 'owner'
				AND m.deleted_at IS NULL
				AND m.status = 'active'
			RETURNING id, pool_id, user_id, subscription_id, role, status, paid_confirmed_at,
				quota_share_ratio, five_hour_limit_usd, five_hour_used_usd, weekly_limit_usd, five_hour_window_start, created_at, updated_at
		`, poolID, ownerUserID, costUSD, occurredAt),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *carpoolRepository) GetMemberByPoolAndUser(ctx context.Context, poolID, userID int64) (*service.CarpoolMember, error) {
	item, err := scanCarpoolMember(
		r.db.QueryRowContext(ctx, `
			SELECT id, pool_id, user_id, subscription_id, role, status, paid_confirmed_at,
				quota_share_ratio, five_hour_limit_usd, five_hour_used_usd, weekly_limit_usd, five_hour_window_start, created_at, updated_at
			FROM carpool_members
			WHERE pool_id = $1 AND user_id = $2 AND deleted_at IS NULL
		`, poolID, userID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *carpoolRepository) GetMemberByID(ctx context.Context, memberID int64) (*service.CarpoolMember, error) {
	item, err := scanCarpoolMember(
		r.db.QueryRowContext(ctx, `
			SELECT id, pool_id, user_id, subscription_id, role, status, paid_confirmed_at,
				quota_share_ratio, five_hour_limit_usd, five_hour_used_usd, weekly_limit_usd, five_hour_window_start, created_at, updated_at
			FROM carpool_members
			WHERE id = $1 AND deleted_at IS NULL
		`, memberID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *carpoolRepository) UpdateMemberStatus(ctx context.Context, memberID int64, status string, removedAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE carpool_members
		SET status = $2, updated_at = NOW(), deleted_at = CASE WHEN $2 = 'removed' THEN $3 ELSE NULL END
		WHERE id = $1
	`, memberID, status, removedAt)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrCarpoolMemberNotFound
	}
	return nil
}

func (r *carpoolRepository) GetRuntimeMemberLimitByGroupAndUser(ctx context.Context, groupID, userID int64, _ time.Time) (*service.CarpoolRuntimeMemberLimit, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			m.pool_id,
			m.id,
			m.five_hour_limit_usd,
			m.five_hour_used_usd,
			m.five_hour_window_start,
			m.weekly_limit_usd,
			COALESCE(us.weekly_usage_usd, 0)
		FROM carpool_members m
		INNER JOIN carpool_pools p ON p.id = m.pool_id
		LEFT JOIN user_subscriptions us ON us.id = m.subscription_id AND us.deleted_at IS NULL
		WHERE p.group_id = $1
			AND m.user_id = $2
			AND p.deleted_at IS NULL
			AND m.deleted_at IS NULL
			AND m.status = 'active'
		LIMIT 1
	`, groupID, userID)
	var out service.CarpoolRuntimeMemberLimit
	var windowStart sql.NullTime
	err := row.Scan(&out.PoolID, &out.MemberID, &out.FiveHourLimitUSD, &out.FiveHourUsedUSD, &windowStart, &out.WeeklyLimitUSD, &out.WeeklyUsageUSD)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if windowStart.Valid {
		out.FiveHourWindowStart = &windowStart.Time
	}
	return &out, nil
}

func (r *carpoolRepository) ListPoolApplicantUsageStats(ctx context.Context, poolID int64) (map[int64]service.CarpoolApplicantUsageStats, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH req_users AS (
			SELECT DISTINCT user_id
			FROM carpool_join_requests
			WHERE pool_id = $1 AND deleted_at IS NULL
		)
		SELECT
			u.user_id,
			COALESCE(COUNT(ul.id), 0) AS total_requests,
			COALESCE(SUM(COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0) + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0)), 0) AS total_tokens,
			COALESCE(COUNT(ul.id) FILTER (WHERE ul.created_at >= NOW() - INTERVAL '7 days'), 0) AS last7d_requests,
			COALESCE(SUM(COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0) + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0)) FILTER (WHERE ul.created_at >= NOW() - INTERVAL '7 days'), 0) AS last7d_tokens,
			COALESCE(COUNT(ul.id) FILTER (WHERE ul.created_at >= NOW() - INTERVAL '30 days'), 0) AS last30d_requests,
			COALESCE(SUM(COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0) + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0)) FILTER (WHERE ul.created_at >= NOW() - INTERVAL '30 days'), 0) AS last30d_tokens
		FROM req_users u
		LEFT JOIN usage_logs ul ON ul.user_id = u.user_id
		GROUP BY u.user_id
	`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int64]service.CarpoolApplicantUsageStats)
	for rows.Next() {
		var userID int64
		var stats service.CarpoolApplicantUsageStats
		if err := rows.Scan(&userID, &stats.TotalRequests, &stats.TotalTokens, &stats.Last7dRequests, &stats.Last7dTokens, &stats.Last30dRequests, &stats.Last30dTokens); err != nil {
			return nil, err
		}
		out[userID] = stats
	}
	return out, rows.Err()
}

func (r *carpoolRepository) ListPoolMemberUsageStats(ctx context.Context, groupID int64, userIDs []int64) (map[int64]service.CarpoolMemberUsageStats, error) {
	out := make(map[int64]service.CarpoolMemberUsageStats, len(userIDs))
	if groupID <= 0 || len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			user_id,
			COALESCE(SUM(COALESCE(input_tokens, 0) + COALESCE(output_tokens, 0) + COALESCE(cache_creation_tokens, 0) + COALESCE(cache_read_tokens, 0)), 0) AS total_tokens,
			COALESCE(SUM(COALESCE(actual_cost, 0)), 0) AS total_cost_usd
		FROM usage_logs
		WHERE group_id = $1
			AND user_id = ANY($2)
		GROUP BY user_id
	`, groupID, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID int64
		var stats service.CarpoolMemberUsageStats
		if err := rows.Scan(&userID, &stats.TotalTokens, &stats.TotalCostUSD); err != nil {
			return nil, err
		}
		out[userID] = stats
	}
	return out, rows.Err()
}

func (r *carpoolRepository) UpdatePoolAccountExternalUsage(ctx context.Context, poolID, accountID int64, update service.CarpoolPoolAccountExternalUsageUpdate) error {
	checkedAt := update.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE carpool_pool_accounts
		SET
			external_5h_used_usd = $3,
			external_weekly_used_usd = $4,
			external_5h_reset_at = $5,
			external_weekly_reset_at = $6,
			external_checked_at = $7
		WHERE pool_id = $1 AND account_id = $2
	`, poolID, accountID, update.ExternalFiveHourUsedUSD, update.ExternalWeeklyUsedUSD, update.ExternalFiveHourResetAt, update.ExternalWeeklyResetAt, checkedAt)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrCarpoolPoolNotFound
	}
	return nil
}

func (r *carpoolRepository) MarkPoolAccountExternalOverageNotified(ctx context.Context, poolID, accountID int64, notifiedAt time.Time) error {
	if notifiedAt.IsZero() {
		notifiedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE carpool_pool_accounts
		SET external_overage_notified_at = $3
		WHERE pool_id = $1 AND account_id = $2
	`, poolID, accountID, notifiedAt)
	return err
}

func (r *carpoolRepository) listPoolSummaries(ctx context.Context, query string, args ...any) ([]service.CarpoolPoolSummary, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.CarpoolPoolSummary, 0)
	for rows.Next() {
		item, err := scanCarpoolPoolSummaryFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

type carpoolPoolScanner interface {
	Scan(dest ...any) error
}

func scanCarpoolPool(scanner carpoolPoolScanner) (*service.CarpoolPool, error) {
	var pool service.CarpoolPool
	var groupID sql.NullInt64
	var quotaSnapshotAt sql.NullTime
	err := scanner.Scan(
		&pool.ID,
		&pool.OwnerUserID,
		&groupID,
		&pool.InviteCode,
		&pool.Name,
		&pool.Platform,
		&pool.Status,
		&pool.Visibility,
		&pool.TargetSeats,
		&pool.DurationDays,
		&pool.SeatPrice,
		&pool.ExtraFee,
		&pool.ExtraFeeDescription,
		&pool.SystemProxyEnabled,
		&pool.RiskControlEnabled,
		&pool.Notes,
		&pool.TotalFiveHourLimitUSD,
		&pool.TotalWeeklyLimitUSD,
		&pool.PerMemberFiveHourLimitUSD,
		&pool.PerMemberWeeklyLimitUSD,
		&quotaSnapshotAt,
		&pool.CreatedAt,
		&pool.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if groupID.Valid {
		pool.GroupID = &groupID.Int64
	}
	if quotaSnapshotAt.Valid {
		pool.QuotaSnapshotAt = &quotaSnapshotAt.Time
	}
	return &pool, nil
}

func scanCarpoolPoolSummaryFromRows(rows *sql.Rows) (*service.CarpoolPoolSummary, error) {
	var summary service.CarpoolPoolSummary
	var groupID sql.NullInt64
	var quotaSnapshotAt sql.NullTime
	var currentStatus sql.NullString
	var currentRequestID sql.NullInt64
	err := rows.Scan(
		&summary.Pool.ID,
		&summary.Pool.OwnerUserID,
		&groupID,
		&summary.Pool.InviteCode,
		&summary.Pool.Name,
		&summary.Pool.Platform,
		&summary.Pool.Status,
		&summary.Pool.Visibility,
		&summary.Pool.TargetSeats,
		&summary.Pool.DurationDays,
		&summary.Pool.SeatPrice,
		&summary.Pool.ExtraFee,
		&summary.Pool.ExtraFeeDescription,
		&summary.Pool.SystemProxyEnabled,
		&summary.Pool.RiskControlEnabled,
		&summary.Pool.Notes,
		&summary.Pool.TotalFiveHourLimitUSD,
		&summary.Pool.TotalWeeklyLimitUSD,
		&summary.Pool.PerMemberFiveHourLimitUSD,
		&summary.Pool.PerMemberWeeklyLimitUSD,
		&quotaSnapshotAt,
		&summary.Pool.CreatedAt,
		&summary.Pool.UpdatedAt,
		&summary.GroupName,
		&summary.ActiveMembers,
		&summary.PendingApplications,
		&summary.BoundAccountCount,
		&summary.IsOwner,
		&currentStatus,
		&currentRequestID,
	)
	if err != nil {
		return nil, err
	}
	if groupID.Valid {
		summary.Pool.GroupID = &groupID.Int64
	}
	if quotaSnapshotAt.Valid {
		summary.Pool.QuotaSnapshotAt = &quotaSnapshotAt.Time
	}
	if currentStatus.Valid {
		summary.CurrentUserStatus = currentStatus.String
	}
	if currentRequestID.Valid {
		summary.CurrentUserRequestID = &currentRequestID.Int64
	}
	return &summary, nil
}

func scanAdminCarpoolPoolSummaryFromRows(rows *sql.Rows) (*service.AdminCarpoolPoolSummary, error) {
	var item service.AdminCarpoolPoolSummary
	var groupID sql.NullInt64
	var quotaSnapshotAt sql.NullTime
	var currentStatus sql.NullString
	var currentRequestID sql.NullInt64
	var ownerEmail sql.NullString
	var ownerUsername sql.NullString
	err := rows.Scan(
		&item.Pool.ID,
		&item.Pool.OwnerUserID,
		&groupID,
		&item.Pool.InviteCode,
		&item.Pool.Name,
		&item.Pool.Platform,
		&item.Pool.Status,
		&item.Pool.Visibility,
		&item.Pool.TargetSeats,
		&item.Pool.DurationDays,
		&item.Pool.SeatPrice,
		&item.Pool.ExtraFee,
		&item.Pool.ExtraFeeDescription,
		&item.Pool.SystemProxyEnabled,
		&item.Pool.RiskControlEnabled,
		&item.Pool.Notes,
		&item.Pool.TotalFiveHourLimitUSD,
		&item.Pool.TotalWeeklyLimitUSD,
		&item.Pool.PerMemberFiveHourLimitUSD,
		&item.Pool.PerMemberWeeklyLimitUSD,
		&quotaSnapshotAt,
		&item.Pool.CreatedAt,
		&item.Pool.UpdatedAt,
		&item.GroupName,
		&item.ActiveMembers,
		&item.PendingApplications,
		&item.BoundAccountCount,
		&item.IsOwner,
		&currentStatus,
		&currentRequestID,
		&ownerEmail,
		&ownerUsername,
	)
	if err != nil {
		return nil, err
	}
	if groupID.Valid {
		item.Pool.GroupID = &groupID.Int64
	}
	if quotaSnapshotAt.Valid {
		item.Pool.QuotaSnapshotAt = &quotaSnapshotAt.Time
	}
	if currentStatus.Valid {
		item.CurrentUserStatus = currentStatus.String
	}
	if currentRequestID.Valid {
		item.CurrentUserRequestID = &currentRequestID.Int64
	}
	if ownerEmail.Valid {
		item.OwnerEmail = ownerEmail.String
	}
	if ownerUsername.Valid {
		item.OwnerUsername = ownerUsername.String
	}
	return &item, nil
}

func scanCarpoolJoinRequest(scanner carpoolPoolScanner) (*service.CarpoolJoinRequest, error) {
	var item service.CarpoolJoinRequest
	var reviewedAt sql.NullTime
	var activatedAt sql.NullTime
	err := scanner.Scan(
		&item.ID,
		&item.PoolID,
		&item.UserID,
		&item.Status,
		&item.Note,
		&item.ReviewNote,
		&reviewedAt,
		&activatedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	if activatedAt.Valid {
		item.ActivatedAt = &activatedAt.Time
	}
	return &item, nil
}

func scanCarpoolJoinRequestFromRows(rows *sql.Rows) (*service.CarpoolJoinRequest, error) {
	return scanCarpoolJoinRequest(rows)
}

func scanCarpoolMember(scanner carpoolPoolScanner) (*service.CarpoolMember, error) {
	var item service.CarpoolMember
	var subscriptionID sql.NullInt64
	var paidAt sql.NullTime
	var windowStart sql.NullTime
	err := scanner.Scan(
		&item.ID,
		&item.PoolID,
		&item.UserID,
		&subscriptionID,
		&item.Role,
		&item.Status,
		&paidAt,
		&item.QuotaShareRatio,
		&item.FiveHourLimitUSD,
		&item.FiveHourUsedUSD,
		&item.WeeklyLimitUSD,
		&windowStart,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if subscriptionID.Valid {
		item.SubscriptionID = &subscriptionID.Int64
	}
	if paidAt.Valid {
		item.PaidConfirmedAt = &paidAt.Time
	}
	if windowStart.Valid {
		item.FiveHourWindowStart = &windowStart.Time
	}
	return &item, nil
}

func scanCarpoolMemberFromRows(rows *sql.Rows) (*service.CarpoolMember, error) {
	return scanCarpoolMember(rows)
}
