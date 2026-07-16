package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/service"
)

type contentModerationRepository struct {
	db *sql.DB
}

func NewContentModerationRepository(db *sql.DB) service.ContentModerationRepository {
	return &contentModerationRepository{db: db}
}

func (r *contentModerationRepository) CreateLog(ctx context.Context, log *service.ContentModerationLog) error {
	if log == nil {
		return nil
	}
	categoryScores, err := json.Marshal(log.CategoryScores)
	if err != nil {
		return fmt.Errorf("marshal moderation category scores: %w", err)
	}
	thresholdSnapshot, err := json.Marshal(log.ThresholdSnapshot)
	if err != nil {
		return fmt.Errorf("marshal moderation thresholds: %w", err)
	}
	var userID any
	if log.UserID != nil {
		userID = *log.UserID
	}
	var apiKeyID any
	if log.APIKeyID != nil {
		apiKeyID = *log.APIKeyID
	}
	var groupID any
	if log.GroupID != nil {
		groupID = *log.GroupID
	}
	var latency any
	if log.UpstreamLatencyMS != nil {
		latency = *log.UpstreamLatencyMS
	}
	err = r.db.QueryRowContext(ctx, `
INSERT INTO content_moderation_logs (
    request_id, user_id, user_email, api_key_id, api_key_name, group_id, group_name,
    endpoint, provider, model, mode, action, flagged, highest_category, highest_score,
    category_scores, threshold_snapshot, input_excerpt, upstream_latency_ms, error,
    violation_count, auto_banned, email_sent, queue_delay_ms, matched_keyword
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12, $13, $14, $15,
    $16::jsonb, $17::jsonb, $18, $19, $20,
    $21, $22, $23, $24, $25
) RETURNING id, created_at`,
		log.RequestID, userID, log.UserEmail, apiKeyID, log.APIKeyName, groupID, log.GroupName,
		log.Endpoint, log.Provider, log.Model, log.Mode, log.Action, log.Flagged, log.HighestCategory, log.HighestScore,
		string(categoryScores), string(thresholdSnapshot), log.InputExcerpt, latency, log.Error,
		log.ViolationCount, log.AutoBanned, log.EmailSent, nullableIntPtr(log.QueueDelayMS), log.MatchedKeyword,
	).Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert content moderation log: %w", err)
	}
	return nil
}

func (r *contentModerationRepository) ListLogs(ctx context.Context, filter service.ContentModerationLogFilter) ([]service.ContentModerationLog, *pagination.PaginationResult, error) {
	where, args := buildContentModerationLogWhere(filter)
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM content_moderation_logs l "+whereSQL, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count content moderation logs: %w", err)
	}

	params := filter.Pagination
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
SELECT
    l.id, l.request_id, l.user_id, l.user_email, l.api_key_id, l.api_key_name, l.group_id, l.group_name,
    l.endpoint, l.provider, l.model, l.mode, l.action, l.flagged, l.highest_category, l.highest_score,
    l.category_scores, l.threshold_snapshot, l.input_excerpt, l.upstream_latency_ms, l.error,
    l.violation_count, l.auto_banned, l.email_sent, COALESCE(u.status, ''), l.queue_delay_ms, l.matched_keyword, l.created_at
FROM content_moderation_logs l
LEFT JOIN users u ON u.id = l.user_id `+whereSQL+`
ORDER BY l.created_at DESC, l.id DESC
LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs)),
		queryArgs...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list content moderation logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.ContentModerationLog, 0)
	for rows.Next() {
		var item service.ContentModerationLog
		var userID, apiKeyID, groupID, latency, queueDelay sql.NullInt64
		var scoresRaw, thresholdsRaw []byte
		if err := rows.Scan(
			&item.ID,
			&item.RequestID,
			&userID,
			&item.UserEmail,
			&apiKeyID,
			&item.APIKeyName,
			&groupID,
			&item.GroupName,
			&item.Endpoint,
			&item.Provider,
			&item.Model,
			&item.Mode,
			&item.Action,
			&item.Flagged,
			&item.HighestCategory,
			&item.HighestScore,
			&scoresRaw,
			&thresholdsRaw,
			&item.InputExcerpt,
			&latency,
			&item.Error,
			&item.ViolationCount,
			&item.AutoBanned,
			&item.EmailSent,
			&item.UserStatus,
			&queueDelay,
			&item.MatchedKeyword,
			&item.CreatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan content moderation log: %w", err)
		}
		if userID.Valid {
			v := userID.Int64
			item.UserID = &v
		}
		if apiKeyID.Valid {
			v := apiKeyID.Int64
			item.APIKeyID = &v
		}
		if groupID.Valid {
			v := groupID.Int64
			item.GroupID = &v
		}
		if latency.Valid {
			v := int(latency.Int64)
			item.UpstreamLatencyMS = &v
		}
		if queueDelay.Valid {
			v := int(queueDelay.Int64)
			item.QueueDelayMS = &v
		}
		item.CategoryScores = map[string]float64{}
		_ = json.Unmarshal(scoresRaw, &item.CategoryScores)
		item.ThresholdSnapshot = map[string]float64{}
		_ = json.Unmarshal(thresholdsRaw, &item.ThresholdSnapshot)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate content moderation logs: %w", err)
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *contentModerationRepository) CountFlaggedByUserSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	if userID <= 0 {
		return 0, nil
	}
	var count int
	err := r.db.QueryRowContext(ctx, `
WITH last_auto_ban AS (
    SELECT MAX(created_at) AS at
    FROM content_moderation_logs
    WHERE user_id = $1 AND auto_banned = TRUE
)
SELECT COUNT(*)
FROM content_moderation_logs
WHERE user_id = $1
  AND flagged = TRUE
  AND action <> 'hash_block'
  AND created_at >= $2
  AND created_at > COALESCE((SELECT at FROM last_auto_ban), '-infinity'::timestamptz)
`, userID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count user content moderation flagged logs: %w", err)
	}
	return count, nil
}

func (r *contentModerationRepository) CleanupExpiredLogs(ctx context.Context, hitBefore time.Time, nonHitBefore time.Time) (*service.ContentModerationCleanupResult, error) {
	result := &service.ContentModerationCleanupResult{FinishedAt: time.Now()}
	if r == nil || r.db == nil {
		return result, nil
	}
	hitExec, err := r.db.ExecContext(ctx, `
DELETE FROM content_moderation_logs
WHERE flagged = TRUE AND created_at < $1
`, hitBefore)
	if err != nil {
		return nil, fmt.Errorf("delete expired hit content moderation logs: %w", err)
	}
	result.DeletedHit, _ = hitExec.RowsAffected()

	nonHitExec, err := r.db.ExecContext(ctx, `
DELETE FROM content_moderation_logs
WHERE flagged = FALSE AND created_at < $1
`, nonHitBefore)
	if err != nil {
		return nil, fmt.Errorf("delete expired non-hit content moderation logs: %w", err)
	}
	result.DeletedNonHit, _ = nonHitExec.RowsAffected()

	if _, err := r.db.ExecContext(ctx, `
DELETE FROM content_moderation_request_events
WHERE created_at < NOW() - INTERVAL '30 days'
`); err != nil {
		return nil, fmt.Errorf("delete expired content moderation request events: %w", err)
	}

	result.FinishedAt = time.Now()
	return result, nil
}

func (r *contentModerationRepository) RecordRiskEvent(ctx context.Context, event service.ContentModerationRiskEvent, policy service.ContentModerationAdaptivePolicy) (*service.ContentModerationRiskProfile, bool, error) {
	if event.UserID <= 0 || strings.TrimSpace(event.RequestID) == "" {
		return nil, false, fmt.Errorf("invalid content moderation risk event")
	}
	policy = normalizedAdaptivePolicy(policy)
	now := event.CreatedAt
	if now.IsZero() {
		now = time.Now()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("begin content moderation risk event: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var apiKeyID any
	if event.APIKeyID > 0 {
		apiKeyID = event.APIKeyID
	}
	result, err := tx.ExecContext(ctx, `
INSERT INTO content_moderation_request_events (
    request_id, user_id, api_key_id, audited, flagged, severity, category,
    score_delta, sample_rate, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (user_id, request_id) DO NOTHING
`, event.RequestID, event.UserID, apiKeyID, event.Audited, event.Flagged, event.Severity,
		event.Category, event.ScoreDelta, event.SampleRate, now)
	if err != nil {
		return nil, false, fmt.Errorf("insert content moderation risk event: %w", err)
	}
	applied, _ := result.RowsAffected()
	if applied == 0 {
		profile, getErr := getRiskProfileWithQuery(ctx, tx, event.UserID, false)
		if getErr != nil {
			return nil, false, getErr
		}
		return profile, false, nil
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO content_moderation_user_risk_profiles (user_id)
VALUES ($1)
ON CONFLICT (user_id) DO NOTHING
`, event.UserID); err != nil {
		return nil, false, fmt.Errorf("ensure content moderation risk profile: %w", err)
	}

	profile, err := getRiskProfileWithQuery(ctx, tx, event.UserID, true)
	if err != nil {
		return nil, false, err
	}
	profile.TotalRequests++
	if event.Audited {
		profile.AuditedRequests++
		profile.LastAuditedAt = timePtr(now)
	}
	if event.Flagged {
		profile.FlaggedRequests++
		profile.LastHitAt = timePtr(now)
		profile.LastCategory = event.Category
	}
	profile.RiskScore = service.DecayContentModerationRiskScore(profile.RiskScore, profile.ScoreUpdatedAt, now, policy.DailyDecayPercent)
	profile.RiskScore = clampRiskScore(profile.RiskScore + event.ScoreDelta)
	profile.LastScoreDelta = event.ScoreDelta
	profile.ScoreUpdatedAt = now
	profile.RiskLevel = policy.RiskLevel(profile)
	profile.CurrentSampleRate = policy.SampleRate(profile)
	profile.UpdatedAt = now

	if _, err := tx.ExecContext(ctx, `
UPDATE content_moderation_user_risk_profiles
SET total_requests = $2,
    audited_requests = $3,
    flagged_requests = $4,
    risk_score = $5,
    risk_level = $6,
    manual_level = $7,
    current_sample_rate = $8,
    last_category = $9,
    last_score_delta = $10,
    last_hit_at = $11,
    last_audited_at = $12,
    score_updated_at = $13,
    updated_at = $14
WHERE user_id = $1
`, profile.UserID, profile.TotalRequests, profile.AuditedRequests, profile.FlaggedRequests,
		profile.RiskScore, profile.RiskLevel, profile.ManualLevel, profile.CurrentSampleRate,
		profile.LastCategory, profile.LastScoreDelta, nullableRiskTime(profile.LastHitAt),
		nullableRiskTime(profile.LastAuditedAt), profile.ScoreUpdatedAt, profile.UpdatedAt); err != nil {
		return nil, false, fmt.Errorf("update content moderation risk profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit content moderation risk event: %w", err)
	}
	profile.UserEmail = event.UserEmail
	return profile, true, nil
}

func (r *contentModerationRepository) GetRiskProfile(ctx context.Context, userID int64) (*service.ContentModerationRiskProfile, error) {
	return getRiskProfileWithQuery(ctx, r.db, userID, false)
}

func (r *contentModerationRepository) ListRiskProfiles(ctx context.Context, filter service.ContentModerationRiskProfileFilter) ([]service.ContentModerationRiskProfile, *pagination.PaginationResult, error) {
	where := []string{"p.user_id IS NOT NULL"}
	args := make([]any, 0, 3)
	if level := strings.ToLower(strings.TrimSpace(filter.Level)); level != "" && level != "all" {
		args = append(args, level)
		where = append(where, fmt.Sprintf("p.risk_level = $%d", len(args)))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+search+"%")
		idx := len(args)
		where = append(where, fmt.Sprintf("(u.email ILIKE $%d OR CAST(p.user_id AS TEXT) ILIKE $%d)", idx, idx))
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM content_moderation_user_risk_profiles p
JOIN users u ON u.id = p.user_id
`+whereSQL, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count content moderation risk profiles: %w", err)
	}

	params := filter.Pagination
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, riskProfileSelectSQL()+`
`+whereSQL+`
ORDER BY p.risk_score DESC, p.last_hit_at DESC NULLS LAST, p.updated_at DESC
LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs)), queryArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("list content moderation risk profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ContentModerationRiskProfile, 0)
	for rows.Next() {
		profile, scanErr := scanRiskProfile(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, *profile)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate content moderation risk profiles: %w", err)
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *contentModerationRepository) GetRiskOverview(ctx context.Context) (*service.ContentModerationRiskOverview, error) {
	var out service.ContentModerationRiskOverview
	err := r.db.QueryRowContext(ctx, `
SELECT
    COUNT(*),
    COUNT(*) FILTER (WHERE risk_level = 'new'),
    COUNT(*) FILTER (WHERE risk_level = 'trusted'),
    COUNT(*) FILTER (WHERE risk_level = 'watch'),
    COUNT(*) FILTER (WHERE risk_level = 'high'),
    COUNT(*) FILTER (WHERE risk_level = 'critical'),
    COALESCE(SUM(audited_requests), 0),
    COALESCE(SUM(flagged_requests), 0),
    COALESCE(AVG(risk_score), 0)
FROM content_moderation_user_risk_profiles
`).Scan(&out.TotalProfiles, &out.NewProfiles, &out.TrustedProfiles, &out.WatchProfiles,
		&out.HighProfiles, &out.CriticalProfiles, &out.AuditedRequests, &out.FlaggedRequests, &out.AverageRiskScore)
	if err != nil {
		return nil, fmt.Errorf("get content moderation risk overview: %w", err)
	}
	return &out, nil
}

func (r *contentModerationRepository) UpdateRiskProfile(ctx context.Context, userID int64, input service.UpdateContentModerationRiskProfileInput, policy service.ContentModerationAdaptivePolicy) (*service.ContentModerationRiskProfile, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin content moderation risk profile override: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	profile, err := getRiskProfileWithQuery(ctx, tx, userID, true)
	if err != nil {
		return nil, err
	}
	if input.ManualLevel != nil {
		profile.ManualLevel = *input.ManualLevel
	}
	if input.ResetScore {
		profile.RiskScore = 0
		profile.LastScoreDelta = 0
		profile.ScoreUpdatedAt = time.Now()
	}
	profile.RiskLevel = policy.RiskLevel(profile)
	profile.CurrentSampleRate = policy.SampleRate(profile)

	_, err = tx.ExecContext(ctx, `
UPDATE content_moderation_user_risk_profiles
SET risk_score = $2,
    risk_level = $3,
    manual_level = $4,
    current_sample_rate = $5,
    last_score_delta = $6,
    score_updated_at = $7,
    updated_at = NOW()
WHERE user_id = $1
`, userID, clampRiskScore(profile.RiskScore), profile.RiskLevel,
		profile.ManualLevel, profile.CurrentSampleRate, profile.LastScoreDelta, profile.ScoreUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update content moderation risk profile override: %w", err)
	}
	updated, err := getRiskProfileWithQuery(ctx, tx, userID, false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit content moderation risk profile override: %w", err)
	}
	return updated, nil
}

func (r *contentModerationRepository) ReserveRiskNotification(ctx context.Context, userID int64, cooldown time.Duration) (bool, error) {
	if cooldown <= 0 {
		cooldown = 24 * time.Hour
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE content_moderation_user_risk_profiles
SET last_notified_at = NOW(), updated_at = NOW()
WHERE user_id = $1
  AND (last_notified_at IS NULL OR last_notified_at <= $2)
`, userID, time.Now().Add(-cooldown))
	if err != nil {
		return false, fmt.Errorf("reserve content moderation risk notification: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (r *contentModerationRepository) DisableAPIKeyForRisk(ctx context.Context, apiKeyID int64, userID int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE api_keys
SET status = $3, updated_at = NOW()
WHERE id = $1 AND user_id = $2 AND status <> $3
`, apiKeyID, userID, service.StatusDisabled)
	if err != nil {
		return false, fmt.Errorf("disable api key for content moderation risk: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

type riskProfileScanner interface {
	Scan(dest ...any) error
}

type riskProfileQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getRiskProfileWithQuery(ctx context.Context, queryer riskProfileQueryer, userID int64, forUpdate bool) (*service.ContentModerationRiskProfile, error) {
	query := riskProfileSelectSQL() + " WHERE p.user_id = $1"
	if forUpdate {
		query += " FOR UPDATE OF p"
	}
	profile, err := scanRiskProfile(queryer.QueryRowContext(ctx, query, userID))
	if err != nil {
		return nil, fmt.Errorf("get content moderation risk profile: %w", err)
	}
	return profile, nil
}

func riskProfileSelectSQL() string {
	return `
SELECT
    p.user_id, COALESCE(u.email, ''), COALESCE(u.status, ''),
    p.total_requests, p.audited_requests, p.flagged_requests,
    p.risk_score, p.risk_level, p.manual_level, p.current_sample_rate,
    p.last_category, p.last_score_delta, p.last_hit_at, p.last_audited_at,
    p.last_notified_at, p.score_updated_at, p.created_at, p.updated_at
FROM content_moderation_user_risk_profiles p
JOIN users u ON u.id = p.user_id`
}

func scanRiskProfile(scanner riskProfileScanner) (*service.ContentModerationRiskProfile, error) {
	var profile service.ContentModerationRiskProfile
	var lastHit, lastAudited, lastNotified sql.NullTime
	if err := scanner.Scan(
		&profile.UserID, &profile.UserEmail, &profile.UserStatus,
		&profile.TotalRequests, &profile.AuditedRequests, &profile.FlaggedRequests,
		&profile.RiskScore, &profile.RiskLevel, &profile.ManualLevel, &profile.CurrentSampleRate,
		&profile.LastCategory, &profile.LastScoreDelta, &lastHit, &lastAudited,
		&lastNotified, &profile.ScoreUpdatedAt, &profile.CreatedAt, &profile.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if lastHit.Valid {
		profile.LastHitAt = timePtr(lastHit.Time)
	}
	if lastAudited.Valid {
		profile.LastAuditedAt = timePtr(lastAudited.Time)
	}
	if lastNotified.Valid {
		profile.LastNotifiedAt = timePtr(lastNotified.Time)
	}
	return &profile, nil
}

func normalizedAdaptivePolicy(policy service.ContentModerationAdaptivePolicy) service.ContentModerationAdaptivePolicy {
	defaults := service.DefaultContentModerationAdaptivePolicy()
	// JSON round-trips normalize the policy in the service. Repository callers in tests may not.
	if policy.FullAuditRequests <= 0 {
		policy.FullAuditRequests = defaults.FullAuditRequests
	}
	if policy.RampAuditRequests < policy.FullAuditRequests {
		policy.RampAuditRequests = defaults.RampAuditRequests
	}
	if policy.DailyDecayPercent <= 0 || policy.DailyDecayPercent >= 100 {
		policy.DailyDecayPercent = defaults.DailyDecayPercent
	}
	return policy
}

func clampRiskScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func timePtr(value time.Time) *time.Time {
	copy := value
	return &copy
}

func nullableRiskTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableIntPtr(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func buildContentModerationLogWhere(filter service.ContentModerationLogFilter) ([]string, []any) {
	where := []string{"l.id IS NOT NULL"}
	args := make([]any, 0)
	add := func(expr string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	switch strings.ToLower(strings.TrimSpace(filter.Result)) {
	case "hit", "flagged":
		where = append(where, "l.flagged = TRUE")
	case "blocked", "block":
		where = append(where, "l.action IN ('block', 'keyword_block', 'hash_block')")
	case "pass", "allow":
		where = append(where, "l.flagged = FALSE AND l.error = ''")
	case "error":
		where = append(where, "l.error <> ''")
	}
	if filter.GroupID != nil {
		add("l.group_id = $%d", *filter.GroupID)
	}
	if endpoint := strings.TrimSpace(filter.Endpoint); endpoint != "" {
		add("l.endpoint = $%d", endpoint)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + search + "%"
		args = append(args, like, like, like, like, like)
		idx := len(args) - 4
		where = append(where, fmt.Sprintf("(l.request_id ILIKE $%d OR l.user_email ILIKE $%d OR l.api_key_name ILIKE $%d OR l.model ILIKE $%d OR l.input_excerpt ILIKE $%d)", idx, idx+1, idx+2, idx+3, idx+4))
	}
	if filter.From != nil && !filter.From.IsZero() {
		add("l.created_at >= $%d", *filter.From)
	}
	if filter.To != nil && !filter.To.IsZero() {
		add("l.created_at <= $%d", *filter.To)
	}
	return where, args
}
