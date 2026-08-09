package repository

import (
	"context"
	"database/sql"

	"fmt"
	"strings"
	"time"

	"ikik-api/internal/service"
)

func (r *opsRepository) GetLatestRetryAttemptForError(ctx context.Context, sourceErrorID int64) (*service.OpsRetryAttempt, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if sourceErrorID <= 0 {
		return nil, fmt.Errorf("invalid source_error_id")
	}

	q := `
SELECT
  id,
  created_at,
  COALESCE(requested_by_user_id, 0),
  source_error_id,
  COALESCE(mode, ''),
  pinned_account_id,
  COALESCE(status, ''),
  started_at,
  finished_at,
  duration_ms,
  success,
  http_status_code,
  upstream_request_id,
  used_account_id,
  response_preview,
  response_truncated,
  result_request_id,
  result_error_id,
  error_message
FROM ops_retry_attempts
WHERE source_error_id = $1
ORDER BY created_at DESC
LIMIT 1`

	var out service.OpsRetryAttempt
	var pinnedAccountID sql.NullInt64
	var requestedBy sql.NullInt64
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var durationMs sql.NullInt64
	var success sql.NullBool
	var httpStatusCode sql.NullInt64
	var upstreamRequestID sql.NullString
	var usedAccountID sql.NullInt64
	var responsePreview sql.NullString
	var responseTruncated sql.NullBool
	var resultRequestID sql.NullString
	var resultErrorID sql.NullInt64
	var errorMessage sql.NullString

	err := r.db.QueryRowContext(ctx, q, sourceErrorID).Scan(
		&out.ID,
		&out.CreatedAt,
		&requestedBy,
		&out.SourceErrorID,
		&out.Mode,
		&pinnedAccountID,
		&out.Status,
		&startedAt,
		&finishedAt,
		&durationMs,
		&success,
		&httpStatusCode,
		&upstreamRequestID,
		&usedAccountID,
		&responsePreview,
		&responseTruncated,
		&resultRequestID,
		&resultErrorID,
		&errorMessage,
	)
	if err != nil {
		return nil, err
	}
	out.RequestedByUserID = requestedBy.Int64
	if pinnedAccountID.Valid {
		v := pinnedAccountID.Int64
		out.PinnedAccountID = &v
	}
	if startedAt.Valid {
		t := startedAt.Time
		out.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		out.FinishedAt = &t
	}
	if durationMs.Valid {
		v := durationMs.Int64
		out.DurationMs = &v
	}
	if success.Valid {
		v := success.Bool
		out.Success = &v
	}
	if httpStatusCode.Valid {
		v := int(httpStatusCode.Int64)
		out.HTTPStatusCode = &v
	}
	if upstreamRequestID.Valid {
		s := upstreamRequestID.String
		out.UpstreamRequestID = &s
	}
	if usedAccountID.Valid {
		v := usedAccountID.Int64
		out.UsedAccountID = &v
	}
	if responsePreview.Valid {
		s := responsePreview.String
		out.ResponsePreview = &s
	}
	if responseTruncated.Valid {
		v := responseTruncated.Bool
		out.ResponseTruncated = &v
	}
	if resultRequestID.Valid {
		s := resultRequestID.String
		out.ResultRequestID = &s
	}
	if resultErrorID.Valid {
		v := resultErrorID.Int64
		out.ResultErrorID = &v
	}
	if errorMessage.Valid {
		s := errorMessage.String
		out.ErrorMessage = &s
	}

	return &out, nil
}
func (r *opsRepository) InsertRetryAttempt(ctx context.Context, input *service.OpsInsertRetryAttemptInput) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("nil ops repository")
	}
	if input == nil {
		return 0, fmt.Errorf("nil input")
	}
	if input.SourceErrorID <= 0 {
		return 0, fmt.Errorf("invalid source_error_id")
	}
	if strings.TrimSpace(input.Mode) == "" {
		return 0, fmt.Errorf("invalid mode")
	}

	q := `
INSERT INTO ops_retry_attempts (
  requested_by_user_id,
  source_error_id,
  mode,
  pinned_account_id,
  status,
  started_at
) VALUES (
  $1,$2,$3,$4,$5,$6
) RETURNING id`

	var id int64
	err := r.db.QueryRowContext(
		ctx,
		q,
		opsNullInt64(&input.RequestedByUserID),
		input.SourceErrorID,
		strings.TrimSpace(input.Mode),
		opsNullInt64(input.PinnedAccountID),
		strings.TrimSpace(input.Status),
		input.StartedAt,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *opsRepository) ListRetryAttemptsByErrorID(ctx context.Context, sourceErrorID int64, limit int) ([]*service.OpsRetryAttempt, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if sourceErrorID <= 0 {
		return nil, fmt.Errorf("invalid source_error_id")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	q := `
SELECT
  r.id,
  r.created_at,
  COALESCE(r.requested_by_user_id, 0),
  r.source_error_id,
  COALESCE(r.mode, ''),
  r.pinned_account_id,
  COALESCE(pa.name, ''),
  COALESCE(r.status, ''),
  r.started_at,
  r.finished_at,
  r.duration_ms,
  r.success,
  r.http_status_code,
  r.upstream_request_id,
  r.used_account_id,
  COALESCE(ua.name, ''),
  r.response_preview,
  r.response_truncated,
  r.result_request_id,
  r.result_error_id,
  r.error_message
FROM ops_retry_attempts r
LEFT JOIN accounts pa ON r.pinned_account_id = pa.id
LEFT JOIN accounts ua ON r.used_account_id = ua.id
WHERE r.source_error_id = $1
ORDER BY r.created_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, sourceErrorID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]*service.OpsRetryAttempt, 0, 16)
	for rows.Next() {
		var item service.OpsRetryAttempt
		var pinnedAccountID sql.NullInt64
		var pinnedAccountName string
		var requestedBy sql.NullInt64
		var startedAt sql.NullTime
		var finishedAt sql.NullTime
		var durationMs sql.NullInt64
		var success sql.NullBool
		var httpStatusCode sql.NullInt64
		var upstreamRequestID sql.NullString
		var usedAccountID sql.NullInt64
		var usedAccountName string
		var responsePreview sql.NullString
		var responseTruncated sql.NullBool
		var resultRequestID sql.NullString
		var resultErrorID sql.NullInt64
		var errorMessage sql.NullString

		if err := rows.Scan(
			&item.ID,
			&item.CreatedAt,
			&requestedBy,
			&item.SourceErrorID,
			&item.Mode,
			&pinnedAccountID,
			&pinnedAccountName,
			&item.Status,
			&startedAt,
			&finishedAt,
			&durationMs,
			&success,
			&httpStatusCode,
			&upstreamRequestID,
			&usedAccountID,
			&usedAccountName,
			&responsePreview,
			&responseTruncated,
			&resultRequestID,
			&resultErrorID,
			&errorMessage,
		); err != nil {
			return nil, err
		}

		item.RequestedByUserID = requestedBy.Int64
		if pinnedAccountID.Valid {
			v := pinnedAccountID.Int64
			item.PinnedAccountID = &v
		}
		item.PinnedAccountName = pinnedAccountName
		if startedAt.Valid {
			t := startedAt.Time
			item.StartedAt = &t
		}
		if finishedAt.Valid {
			t := finishedAt.Time
			item.FinishedAt = &t
		}
		if durationMs.Valid {
			v := durationMs.Int64
			item.DurationMs = &v
		}
		if success.Valid {
			v := success.Bool
			item.Success = &v
		}
		if httpStatusCode.Valid {
			v := int(httpStatusCode.Int64)
			item.HTTPStatusCode = &v
		}
		if upstreamRequestID.Valid {
			item.UpstreamRequestID = &upstreamRequestID.String
		}
		if usedAccountID.Valid {
			v := usedAccountID.Int64
			item.UsedAccountID = &v
		}
		item.UsedAccountName = usedAccountName
		if responsePreview.Valid {
			item.ResponsePreview = &responsePreview.String
		}
		if responseTruncated.Valid {
			v := responseTruncated.Bool
			item.ResponseTruncated = &v
		}
		if resultRequestID.Valid {
			item.ResultRequestID = &resultRequestID.String
		}
		if resultErrorID.Valid {
			v := resultErrorID.Int64
			item.ResultErrorID = &v
		}
		if errorMessage.Valid {
			item.ErrorMessage = &errorMessage.String
		}
		out = append(out, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
func (r *opsRepository) UpdateRetryAttempt(ctx context.Context, input *service.OpsUpdateRetryAttemptInput) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if input == nil {
		return fmt.Errorf("nil input")
	}
	if input.ID <= 0 {
		return fmt.Errorf("invalid id")
	}

	q := `
UPDATE ops_retry_attempts
SET
  status = $2,
  finished_at = $3,
  duration_ms = $4,
  success = $5,
  http_status_code = $6,
  upstream_request_id = $7,
  used_account_id = $8,
  response_preview = $9,
  response_truncated = $10,
  result_request_id = $11,
  result_error_id = $12,
  error_message = $13
WHERE id = $1`

	_, err := r.db.ExecContext(
		ctx,
		q,
		input.ID,
		strings.TrimSpace(input.Status),
		nullTime(input.FinishedAt),
		input.DurationMs,
		opsRetryNullBool(input.Success),
		nullInt(input.HTTPStatusCode),
		opsNullString(input.UpstreamRequestID),
		nullInt64(input.UsedAccountID),
		opsNullString(input.ResponsePreview),
		opsRetryNullBool(input.ResponseTruncated),
		opsNullString(input.ResultRequestID),
		nullInt64(input.ResultErrorID),
		opsNullString(input.ErrorMessage),
	)
	return err
}

func opsRetryNullBool(v *bool) sql.NullBool {
	if v == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *v, Valid: true}
}
func nullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}
