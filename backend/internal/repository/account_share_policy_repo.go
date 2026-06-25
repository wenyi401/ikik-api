package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	dbent "ikik-api/ent"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/service"
)

type accountSharePolicyRepository struct {
	db *sql.DB
}

func NewAccountSharePolicyRepository(_ *dbent.Client, sqlDB *sql.DB) service.AccountSharePolicyRepository {
	return &accountSharePolicyRepository{db: sqlDB}
}

func (r *accountSharePolicyRepository) ListAccountSharePolicies(ctx context.Context, params pagination.PaginationParams, filters service.AccountSharePolicyFilters) ([]service.AccountSharePolicy, *pagination.PaginationResult, error) {
	where, args := accountSharePolicyWhere(filters)
	countQuery := "SELECT COUNT(*) FROM account_share_policies WHERE deleted_at IS NULL" + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	query := `
		SELECT id, scope_type, scope_id, platform, owner_share_ratio::text, invite_share_ratio::text, version, enabled,
			effective_at, created_by_admin_id, created_at, updated_at, deleted_at
		FROM account_share_policies
		WHERE deleted_at IS NULL` + where + `
		ORDER BY effective_at DESC, id DESC
		LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, params.Limit(), params.Offset())

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	policies := make([]service.AccountSharePolicy, 0, params.Limit())
	for rows.Next() {
		policy, err := scanAccountSharePolicy(rows)
		if err != nil {
			return nil, nil, err
		}
		policies = append(policies, *policy)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return policies, paginationResultFromTotal(total, params), nil
}

func (r *accountSharePolicyRepository) GetAccountSharePolicyByID(ctx context.Context, id int64) (*service.AccountSharePolicy, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scope_type, scope_id, platform, owner_share_ratio::text, invite_share_ratio::text, version, enabled,
			effective_at, created_by_admin_id, created_at, updated_at, deleted_at
		FROM account_share_policies
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	policy, err := scanAccountSharePolicy(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func (r *accountSharePolicyRepository) ResolveEnabledAccountSharePolicy(ctx context.Context, accountID int64, groupID *int64, platform string, explicitPolicyID *int64) (*service.AccountSharePolicy, error) {
	policy, found, err := r.queryEnabledAccountSharePolicy(ctx, "scope_type = 'global'")
	if err != nil || found {
		return policy, err
	}
	return nil, nil
}

func (r *accountSharePolicyRepository) queryEnabledAccountSharePolicy(ctx context.Context, predicate string, args ...any) (*service.AccountSharePolicy, bool, error) {
	query := `
		SELECT id, scope_type, scope_id, platform, owner_share_ratio::text, invite_share_ratio::text, version, enabled,
			effective_at, created_by_admin_id, created_at, updated_at, deleted_at
		FROM account_share_policies
		WHERE deleted_at IS NULL
			AND enabled = TRUE
			AND effective_at <= NOW()
			AND ` + predicate + `
		ORDER BY effective_at DESC, version DESC, id DESC
		LIMIT 1
	`
	policy, err := scanAccountSharePolicy(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return policy, true, nil
}

func (r *accountSharePolicyRepository) CreateAccountSharePolicy(ctx context.Context, input service.CreateAccountSharePolicyInput) (*service.AccountSharePolicy, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO account_share_policies (
			scope_type, scope_id, platform, owner_share_ratio, invite_share_ratio, enabled, effective_at, created_by_admin_id
		) VALUES (
			$1, $2, $3, $4::numeric, $5::numeric, $6, $7, $8
		)
		RETURNING id, scope_type, scope_id, platform, owner_share_ratio::text, invite_share_ratio::text, version, enabled,
			effective_at, created_by_admin_id, created_at, updated_at, deleted_at
	`,
		input.ScopeType,
		nullablePtrInt64(input.ScopeID),
		nullableStringPtr(input.Platform),
		strconv.FormatFloat(input.OwnerShareRatio, 'f', 6, 64),
		strconv.FormatFloat(input.InviteShareRatio, 'f', 6, 64),
		*input.Enabled,
		*input.EffectiveAt,
		nullablePtrInt64(input.CreatedByAdminID),
	)
	return scanAccountSharePolicy(row)
}

func (r *accountSharePolicyRepository) UpdateAccountSharePolicy(ctx context.Context, id int64, input service.UpdateAccountSharePolicyInput) (*service.AccountSharePolicy, error) {
	current, err := r.GetAccountSharePolicyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	scopeType := current.ScopeType
	scopeID := current.ScopeID
	platform := current.Platform
	ratio := current.OwnerShareRatio
	inviteRatio := current.InviteShareRatio
	enabled := current.Enabled
	effectiveAt := current.EffectiveAt
	if input.ScopeType != nil {
		scopeType = strings.TrimSpace(*input.ScopeType)
		scopeID = input.ScopeID
		platform = input.Platform
	}
	if input.ScopeID != nil {
		if *input.ScopeID <= 0 {
			scopeID = nil
		} else {
			scopeID = input.ScopeID
		}
	}
	if input.Platform != nil {
		trimmed := strings.TrimSpace(*input.Platform)
		if trimmed == "" {
			platform = nil
		} else {
			platform = &trimmed
		}
	}
	if input.OwnerShareRatio != nil {
		ratio = *input.OwnerShareRatio
	}
	if input.InviteShareRatio != nil {
		inviteRatio = *input.InviteShareRatio
	}
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	if input.EffectiveAt != nil {
		effectiveAt = *input.EffectiveAt
	}

	row := r.db.QueryRowContext(ctx, `
		UPDATE account_share_policies
		SET scope_type = $2,
			scope_id = $3,
			platform = $4,
			owner_share_ratio = $5::numeric,
			invite_share_ratio = $6::numeric,
			enabled = $7,
			effective_at = $8,
			version = version + 1,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, scope_type, scope_id, platform, owner_share_ratio::text, invite_share_ratio::text, version, enabled,
			effective_at, created_by_admin_id, created_at, updated_at, deleted_at
	`, id, scopeType, nullablePtrInt64(scopeID), nullableStringPtr(platform), strconv.FormatFloat(ratio, 'f', 6, 64), strconv.FormatFloat(inviteRatio, 'f', 6, 64), enabled, effectiveAt)
	policy, err := scanAccountSharePolicy(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAccountNotFound
	}
	return policy, err
}

func (r *accountSharePolicyRepository) DeleteAccountSharePolicy(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE account_share_policies
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAccountNotFound
	}
	return nil
}

func accountSharePolicyWhere(filters service.AccountSharePolicyFilters) (string, []any) {
	var where strings.Builder
	args := make([]any, 0, 3)
	add := func(condition string, arg any) {
		args = append(args, arg)
		_, _ = where.WriteString(" AND ")
		_, _ = where.WriteString(fmt.Sprintf(condition, len(args)))
	}
	if scopeType := strings.TrimSpace(filters.ScopeType); scopeType != "" {
		add("scope_type = $%d", scopeType)
	}
	if platform := strings.TrimSpace(filters.Platform); platform != "" {
		add("platform = $%d", platform)
	}
	if filters.Enabled != nil {
		add("enabled = $%d", *filters.Enabled)
	}
	return where.String(), args
}

type sqlScanner interface {
	Scan(dest ...any) error
}

func scanAccountSharePolicy(scanner sqlScanner) (*service.AccountSharePolicy, error) {
	var (
		policy         service.AccountSharePolicy
		scopeID        sql.NullInt64
		platform       sql.NullString
		ratioRaw       string
		inviteRatioRaw string
		createdByAdmin sql.NullInt64
		deletedAt      sql.NullTime
	)
	if err := scanner.Scan(
		&policy.ID,
		&policy.ScopeType,
		&scopeID,
		&platform,
		&ratioRaw,
		&inviteRatioRaw,
		&policy.Version,
		&policy.Enabled,
		&policy.EffectiveAt,
		&createdByAdmin,
		&policy.CreatedAt,
		&policy.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	ratio, err := strconv.ParseFloat(strings.TrimSpace(ratioRaw), 64)
	if err != nil {
		return nil, err
	}
	policy.OwnerShareRatio = ratio
	inviteRatio, err := strconv.ParseFloat(strings.TrimSpace(inviteRatioRaw), 64)
	if err != nil {
		return nil, err
	}
	policy.InviteShareRatio = inviteRatio
	if scopeID.Valid {
		policy.ScopeID = &scopeID.Int64
	}
	if platform.Valid {
		policy.Platform = &platform.String
	}
	if createdByAdmin.Valid {
		policy.CreatedByAdminID = &createdByAdmin.Int64
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		policy.DeletedAt = &t
	}
	return &policy, nil
}

func nullableStringPtr(v *string) any {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
