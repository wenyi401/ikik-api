package securityaudit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrPromptAuditProfileNotFound = errors.New("prompt audit user profile not found")

type PromptAuditUserProfile struct {
	UserID            int64      `json:"user_id"`
	UserEmail         string     `json:"user_email"`
	TotalRequests     int64      `json:"total_requests"`
	RemoteAudits      int64      `json:"remote_audits"`
	FlaggedRequests   int64      `json:"flagged_requests"`
	RiskScore         float64    `json:"risk_score"`
	RiskLevel         string     `json:"risk_level"`
	Blocked           bool       `json:"blocked"`
	CurrentSampleRate int        `json:"current_sample_rate"`
	LastCategory      string     `json:"last_category"`
	LastHitAt         *time.Time `json:"last_hit_at,omitempty"`
	LastAuditedAt     *time.Time `json:"last_audited_at,omitempty"`
	BlockedAt         *time.Time `json:"blocked_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type PromptAuditUserProfilePage struct {
	Items    []PromptAuditUserProfile `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Pages    int                      `json:"pages"`
}

func (r *PostgreSQLRepository) ListPromptAuditUserProfiles(ctx context.Context, page, pageSize int, blockedOnly bool, keyword string) (*PromptAuditUserProfilePage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("prompt audit database unavailable")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	where := []string{"TRUE"}
	args := make([]any, 0, 4)
	if blockedOnly {
		where = append(where, "p.blocked=TRUE")
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = append(where, fmt.Sprintf("(u.email ILIKE $%d OR CAST(p.user_id AS TEXT) ILIKE $%d)", len(args), len(args)))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM prompt_audit_user_profiles p
		JOIN users u ON u.id=p.user_id WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.user_id, COALESCE(u.email,''), p.total_requests, p.remote_audits,
			p.flagged_requests, p.risk_score, p.risk_level, p.blocked,
			p.current_sample_rate, p.last_category, p.last_hit_at,
			p.last_audited_at, p.blocked_at, p.updated_at
		FROM prompt_audit_user_profiles p
		JOIN users u ON u.id=p.user_id
		WHERE `+whereSQL+fmt.Sprintf(" ORDER BY p.blocked DESC, p.risk_score DESC, p.updated_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]PromptAuditUserProfile, 0, pageSize)
	for rows.Next() {
		var item PromptAuditUserProfile
		var lastHit, lastAudited, blockedAt sql.NullTime
		if err := rows.Scan(&item.UserID, &item.UserEmail, &item.TotalRequests, &item.RemoteAudits,
			&item.FlaggedRequests, &item.RiskScore, &item.RiskLevel, &item.Blocked,
			&item.CurrentSampleRate, &item.LastCategory, &lastHit, &lastAudited, &blockedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.LastHitAt = nullableTime(lastHit)
		item.LastAuditedAt = nullableTime(lastAudited)
		item.BlockedAt = nullableTime(blockedAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pages := 0
	if total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &PromptAuditUserProfilePage{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (r *PostgreSQLRepository) UnblockPromptAuditUser(ctx context.Context, userID int64) (*PromptAuditUserProfile, error) {
	if r == nil || r.db == nil || userID <= 0 {
		return nil, ErrPromptAuditProfileNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE prompt_audit_user_profiles SET
			blocked=FALSE, risk_level='watch', risk_score=25,
			current_sample_rate=100, blocked_at=NULL, updated_at=NOW()
		WHERE user_id=$1
		RETURNING user_id, total_requests, remote_audits, flagged_requests,
			risk_score, risk_level, blocked, current_sample_rate, last_category,
			last_hit_at, last_audited_at, blocked_at, updated_at`, userID)
	var item PromptAuditUserProfile
	var lastHit, lastAudited, blockedAt sql.NullTime
	if err := row.Scan(&item.UserID, &item.TotalRequests, &item.RemoteAudits, &item.FlaggedRequests,
		&item.RiskScore, &item.RiskLevel, &item.Blocked, &item.CurrentSampleRate,
		&item.LastCategory, &lastHit, &lastAudited, &blockedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPromptAuditProfileNotFound
		}
		return nil, err
	}
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(email,'') FROM users WHERE id=$1`, userID).Scan(&item.UserEmail)
	item.LastHitAt = nullableTime(lastHit)
	item.LastAuditedAt = nullableTime(lastAudited)
	item.BlockedAt = nullableTime(blockedAt)
	return &item, nil
}

func nullableTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
