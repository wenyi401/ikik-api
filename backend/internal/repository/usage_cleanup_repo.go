package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	dbent "ikik-api/ent"
	dbusagecleanuptask "ikik-api/ent/usagecleanuptask"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/service"
)

const usageSnapshotBusinessTimezone = "Asia/Shanghai"

type usageCleanupRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

type usageLogsArchiveReader struct {
	rows *sql.Rows
	buf  []byte
	line []byte
	done bool
}

func NewUsageCleanupRepository(client *dbent.Client, sqlDB *sql.DB) service.UsageCleanupRepository {
	return newUsageCleanupRepositoryWithSQL(client, sqlDB)
}

func newUsageCleanupRepositoryWithSQL(client *dbent.Client, sqlq sqlExecutor) *usageCleanupRepository {
	return &usageCleanupRepository{client: client, sql: sqlq}
}

func (r *usageCleanupRepository) CreateTask(ctx context.Context, task *service.UsageCleanupTask) error {
	if task == nil {
		return nil
	}
	if r.client != nil {
		return r.createTaskWithEnt(ctx, task)
	}
	return r.createTaskWithSQL(ctx, task)
}

func (r *usageCleanupRepository) ListTasks(ctx context.Context, params pagination.PaginationParams) ([]service.UsageCleanupTask, *pagination.PaginationResult, error) {
	if r.client != nil {
		return r.listTasksWithEnt(ctx, params)
	}
	var total int64
	if err := scanSingleRow(ctx, r.sql, "SELECT COUNT(*) FROM usage_cleanup_tasks", nil, &total); err != nil {
		return nil, nil, err
	}
	if total == 0 {
		return []service.UsageCleanupTask{}, paginationResultFromTotal(0, params), nil
	}

	query := `
		SELECT id, status, filters, COALESCE(created_by, 0), created_source, deleted_rows, error_message,
			canceled_by, canceled_at,
			started_at, finished_at, created_at, updated_at
		FROM usage_cleanup_tasks
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.sql.QueryContext(ctx, query, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	tasks := make([]service.UsageCleanupTask, 0)
	for rows.Next() {
		var task service.UsageCleanupTask
		var filtersJSON []byte
		var errMsg sql.NullString
		var canceledBy sql.NullInt64
		var canceledAt sql.NullTime
		var startedAt sql.NullTime
		var finishedAt sql.NullTime
		if err := rows.Scan(
			&task.ID,
			&task.Status,
			&filtersJSON,
			&task.CreatedBy,
			&task.CreatedSource,
			&task.DeletedRows,
			&errMsg,
			&canceledBy,
			&canceledAt,
			&startedAt,
			&finishedAt,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		if err := json.Unmarshal(filtersJSON, &task.Filters); err != nil {
			return nil, nil, fmt.Errorf("parse cleanup filters: %w", err)
		}
		if errMsg.Valid {
			task.ErrorMsg = &errMsg.String
		}
		if canceledBy.Valid {
			v := canceledBy.Int64
			task.CanceledBy = &v
		}
		if canceledAt.Valid {
			task.CanceledAt = &canceledAt.Time
		}
		if startedAt.Valid {
			task.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			task.FinishedAt = &finishedAt.Time
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return tasks, paginationResultFromTotal(total, params), nil
}

func (r *usageCleanupRepository) ClaimNextPendingTask(ctx context.Context, staleRunningAfterSeconds int64) (*service.UsageCleanupTask, error) {
	if staleRunningAfterSeconds <= 0 {
		staleRunningAfterSeconds = 1800
	}
	query := `
		WITH next AS (
			SELECT id
			FROM usage_cleanup_tasks
			WHERE status = $1
				OR (
					status = $2
					AND started_at IS NOT NULL
					AND started_at < NOW() - ($3 * interval '1 second')
				)
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE usage_cleanup_tasks AS tasks
		SET status = $4,
			started_at = NOW(),
			finished_at = NULL,
			error_message = NULL,
			updated_at = NOW()
		FROM next
		WHERE tasks.id = next.id
		RETURNING tasks.id, tasks.status, tasks.filters, COALESCE(tasks.created_by, 0), tasks.created_source, tasks.deleted_rows, tasks.error_message,
			tasks.started_at, tasks.finished_at, tasks.created_at, tasks.updated_at
	`
	var task service.UsageCleanupTask
	var filtersJSON []byte
	var errMsg sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{
			service.UsageCleanupStatusPending,
			service.UsageCleanupStatusRunning,
			staleRunningAfterSeconds,
			service.UsageCleanupStatusRunning,
		},
		&task.ID,
		&task.Status,
		&filtersJSON,
		&task.CreatedBy,
		&task.CreatedSource,
		&task.DeletedRows,
		&errMsg,
		&startedAt,
		&finishedAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(filtersJSON, &task.Filters); err != nil {
		return nil, fmt.Errorf("parse cleanup filters: %w", err)
	}
	if errMsg.Valid {
		task.ErrorMsg = &errMsg.String
	}
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		task.FinishedAt = &finishedAt.Time
	}
	return &task, nil
}

func (r *usageCleanupRepository) FindOldestUsageLogBefore(ctx context.Context, cutoff time.Time) (*time.Time, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("usage cleanup repository not ready")
	}
	var oldest sql.NullTime
	if err := scanSingleRow(ctx, r.sql, "SELECT MIN(created_at) FROM usage_logs WHERE created_at < $1", []any{cutoff.UTC()}, &oldest); err != nil {
		return nil, err
	}
	if !oldest.Valid {
		return nil, nil
	}
	value := oldest.Time.UTC()
	return &value, nil
}

func (r *usageCleanupRepository) ExportUsageLogs(ctx context.Context, filters service.UsageCleanupFilters) (io.ReadCloser, error) {
	if filters.StartTime.IsZero() || filters.EndTime.IsZero() {
		return nil, fmt.Errorf("usage log export filters missing time range")
	}
	whereClause, args := buildUsageCleanupWhere(filters)
	if whereClause == "" {
		return nil, fmt.Errorf("usage log export filters missing time range")
	}
	query := fmt.Sprintf(`
		SELECT to_jsonb(usage_logs)::text
		FROM usage_logs
		WHERE %s
		ORDER BY created_at ASC, id ASC
	`, whereClause)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &usageLogsArchiveReader{rows: rows}, nil
}

func (r *usageCleanupRepository) SnapshotUsageLogs(ctx context.Context, filters service.UsageCleanupFilters) error {
	if filters.StartTime.IsZero() || filters.EndTime.IsZero() {
		return fmt.Errorf("usage log snapshot filters missing time range")
	}
	if hasUsageCleanupDimensionFilters(filters) {
		return fmt.Errorf("usage log snapshot only supports full-range retention windows")
	}
	whereClause, args := buildUsageCleanupWhere(filters)
	if whereClause == "" {
		return fmt.Errorf("usage log snapshot filters missing time range")
	}

	query := fmt.Sprintf(`
		INSERT INTO usage_daily_dimension_snapshots (
			bucket_date,
			user_id,
			api_key_id,
			account_id,
			group_id,
			model,
			requested_model,
			upstream_model,
			model_mapping_chain,
			request_type,
			stream_state,
			billing_type,
			billing_mode,
			total_requests,
			input_tokens,
			output_tokens,
			cache_creation_tokens,
			cache_read_tokens,
			total_cost,
			actual_cost,
			account_cost,
			total_duration_ms,
			computed_at
		)
		SELECT
			(created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
			COALESCE(user_id, 0) AS user_id,
			COALESCE(api_key_id, 0) AS api_key_id,
			COALESCE(account_id, 0) AS account_id,
			COALESCE(group_id, 0) AS group_id,
			COALESCE(model, '') AS model,
			COALESCE(requested_model, '') AS requested_model,
			COALESCE(upstream_model, '') AS upstream_model,
			COALESCE(model_mapping_chain, '') AS model_mapping_chain,
			COALESCE(request_type, 0)::smallint AS request_type,
			CASE WHEN stream IS TRUE THEN 1 ELSE 0 END::smallint AS stream_state,
			COALESCE(billing_type, -1)::smallint AS billing_type,
			COALESCE(billing_mode, '') AS billing_mode,
			COUNT(*) AS total_requests,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(cache_creation_tokens), 0) AS cache_creation_tokens,
			COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(total_cost), 0) AS total_cost,
			COALESCE(SUM(actual_cost), 0) AS actual_cost,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) AS account_cost,
			COALESCE(SUM(COALESCE(duration_ms, 0)), 0) AS total_duration_ms,
			NOW() AS computed_at
		FROM usage_logs
		WHERE %s
		GROUP BY
			bucket_date,
			COALESCE(user_id, 0),
			COALESCE(api_key_id, 0),
			COALESCE(account_id, 0),
			COALESCE(group_id, 0),
			COALESCE(model, ''),
			COALESCE(requested_model, ''),
			COALESCE(upstream_model, ''),
			COALESCE(model_mapping_chain, ''),
			COALESCE(request_type, 0)::smallint,
			CASE WHEN stream IS TRUE THEN 1 ELSE 0 END::smallint,
			COALESCE(billing_type, -1)::smallint,
			COALESCE(billing_mode, '')
		ON CONFLICT (
			bucket_date,
			user_id,
			api_key_id,
			account_id,
			group_id,
			model,
			requested_model,
			upstream_model,
			model_mapping_chain,
			request_type,
			stream_state,
			billing_type,
			billing_mode
		) DO UPDATE SET
			total_requests = EXCLUDED.total_requests,
			input_tokens = EXCLUDED.input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			cache_creation_tokens = EXCLUDED.cache_creation_tokens,
			cache_read_tokens = EXCLUDED.cache_read_tokens,
			total_cost = EXCLUDED.total_cost,
			actual_cost = EXCLUDED.actual_cost,
			account_cost = EXCLUDED.account_cost,
			total_duration_ms = EXCLUDED.total_duration_ms,
			computed_at = EXCLUDED.computed_at
	`, whereClause)

	if _, err := r.sql.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	if err := r.snapshotRevenueDailyDimensions(ctx, whereClause, args); err != nil {
		return err
	}
	return nil
}

func (r *usageCleanupRepository) snapshotRevenueDailyDimensions(ctx context.Context, whereClause string, args []any) error {
	if strings.TrimSpace(whereClause) == "" {
		return fmt.Errorf("revenue snapshot filters missing time range")
	}
	query := fmt.Sprintf(`
		INSERT INTO revenue_daily_dimension_snapshots (
			bucket_date,
			user_id,
			account_id,
			group_id,
			owner_user_id,
			model,
			requested_model,
			total_requests,
			total_tokens,
			standard_cost,
			consumed_revenue,
			account_cost,
			share_consumer_charge,
			share_account_cost,
			share_owner_credit,
			share_platform_fee,
			computed_at
		)
		SELECT
			(ul.created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
			COALESCE(ul.user_id, 0) AS user_id,
			COALESCE(ul.account_id, 0) AS account_id,
			COALESCE(ul.group_id, 0) AS group_id,
			COALESCE(ase.owner_user_id, 0) AS owner_user_id,
			COALESCE(NULLIF(TRIM(ul.model), ''), '') AS model,
			COALESCE(NULLIF(TRIM(ul.requested_model), ''), '') AS requested_model,
			COUNT(*) AS total_requests,
			COALESCE(SUM(
				COALESCE(ul.input_tokens, 0)
				+ COALESCE(ul.output_tokens, 0)
				+ COALESCE(ul.cache_creation_tokens, 0)
				+ COALESCE(ul.cache_read_tokens, 0)
			), 0) AS total_tokens,
			COALESCE(SUM(ul.total_cost), 0) AS standard_cost,
			COALESCE(SUM(ul.actual_cost), 0) AS consumed_revenue,
			COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS account_cost,
			COALESCE(SUM(COALESCE(ase.consumer_charge, 0)), 0) AS share_consumer_charge,
			COALESCE(SUM(COALESCE(ase.account_cost, 0)), 0) AS share_account_cost,
			COALESCE(SUM(COALESCE(ase.owner_credit, 0)), 0) AS share_owner_credit,
			COALESCE(SUM(COALESCE(ase.platform_fee, 0)), 0) AS share_platform_fee,
			NOW() AS computed_at
		FROM usage_logs ul
		LEFT JOIN account_share_settlement_entries ase ON ase.usage_log_id = ul.id
			AND ase.status = 'applied'
			AND ase.consumer_user_id <> ase.owner_user_id
		WHERE %s
		GROUP BY
			bucket_date,
			COALESCE(ul.user_id, 0),
			COALESCE(ul.account_id, 0),
			COALESCE(ul.group_id, 0),
			COALESCE(ase.owner_user_id, 0),
			COALESCE(NULLIF(TRIM(ul.model), ''), ''),
			COALESCE(NULLIF(TRIM(ul.requested_model), ''), '')
		ON CONFLICT (
			bucket_date,
			user_id,
			account_id,
			group_id,
			owner_user_id,
			model,
			requested_model
		) DO UPDATE SET
			total_requests = EXCLUDED.total_requests,
			total_tokens = EXCLUDED.total_tokens,
			standard_cost = EXCLUDED.standard_cost,
			consumed_revenue = EXCLUDED.consumed_revenue,
			account_cost = EXCLUDED.account_cost,
			share_consumer_charge = EXCLUDED.share_consumer_charge,
			share_account_cost = EXCLUDED.share_account_cost,
			share_owner_credit = EXCLUDED.share_owner_credit,
			share_platform_fee = EXCLUDED.share_platform_fee,
			computed_at = EXCLUDED.computed_at
	`, qualifyUsageCleanupWhereForRevenueSnapshot(whereClause))

	if _, err := r.sql.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("snapshot revenue daily dimensions: %w", err)
	}
	return nil
}

func qualifyUsageCleanupWhereForRevenueSnapshot(whereClause string) string {
	return strings.NewReplacer(
		"created_at ", "ul.created_at ",
		"user_id ", "ul.user_id ",
		"api_key_id ", "ul.api_key_id ",
		"account_id ", "ul.account_id ",
		"group_id ", "ul.group_id ",
		"model ", "ul.model ",
		"request_type ", "ul.request_type ",
		"stream ", "ul.stream ",
		"billing_type ", "ul.billing_type ",
	).Replace(whereClause)
}

func hasUsageCleanupDimensionFilters(filters service.UsageCleanupFilters) bool {
	return filters.UserID != nil ||
		filters.APIKeyID != nil ||
		filters.AccountID != nil ||
		filters.GroupID != nil ||
		filters.Model != nil ||
		filters.RequestType != nil ||
		filters.Stream != nil ||
		filters.BillingType != nil
}

func (r *usageCleanupRepository) GetTaskStatus(ctx context.Context, taskID int64) (string, error) {
	if r.client != nil {
		return r.getTaskStatusWithEnt(ctx, taskID)
	}
	var status string
	if err := scanSingleRow(ctx, r.sql, "SELECT status FROM usage_cleanup_tasks WHERE id = $1", []any{taskID}, &status); err != nil {
		return "", err
	}
	return status, nil
}

func (r *usageLogsArchiveReader) Read(p []byte) (int, error) {
	if r == nil || r.rows == nil {
		return 0, io.EOF
	}
	for len(r.buf) == 0 && !r.done {
		if !r.rows.Next() {
			r.done = true
			if err := r.rows.Err(); err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		var line string
		if err := r.rows.Scan(&line); err != nil {
			return 0, err
		}
		r.line = append(r.line[:0], line...)
		r.line = append(r.line, '\n')
		r.buf = r.line
	}
	if len(r.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func (r *usageLogsArchiveReader) Close() error {
	if r == nil || r.rows == nil {
		return nil
	}
	return r.rows.Close()
}

func (r *usageCleanupRepository) UpdateTaskProgress(ctx context.Context, taskID int64, deletedRows int64) error {
	if r.client != nil {
		return r.updateTaskProgressWithEnt(ctx, taskID, deletedRows)
	}
	query := `
		UPDATE usage_cleanup_tasks
		SET deleted_rows = $1,
			updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.sql.ExecContext(ctx, query, deletedRows, taskID)
	return err
}

func (r *usageCleanupRepository) CancelTask(ctx context.Context, taskID int64, canceledBy int64) (bool, error) {
	if r.client != nil {
		return r.cancelTaskWithEnt(ctx, taskID, canceledBy)
	}
	query := `
		UPDATE usage_cleanup_tasks
		SET status = $1,
			canceled_by = $3,
			canceled_at = NOW(),
			finished_at = NOW(),
			error_message = NULL,
			updated_at = NOW()
		WHERE id = $2
			AND status IN ($4, $5)
		RETURNING id
	`
	var id int64
	err := scanSingleRow(ctx, r.sql, query, []any{
		service.UsageCleanupStatusCanceled,
		taskID,
		canceledBy,
		service.UsageCleanupStatusPending,
		service.UsageCleanupStatusRunning,
	}, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *usageCleanupRepository) MarkTaskSucceeded(ctx context.Context, taskID int64, deletedRows int64) error {
	if r.client != nil {
		return r.markTaskSucceededWithEnt(ctx, taskID, deletedRows)
	}
	query := `
		UPDATE usage_cleanup_tasks
		SET status = $1,
			deleted_rows = $2,
			finished_at = NOW(),
			updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.sql.ExecContext(ctx, query, service.UsageCleanupStatusSucceeded, deletedRows, taskID)
	return err
}

func (r *usageCleanupRepository) MarkTaskFailed(ctx context.Context, taskID int64, deletedRows int64, errorMsg string) error {
	if r.client != nil {
		return r.markTaskFailedWithEnt(ctx, taskID, deletedRows, errorMsg)
	}
	query := `
		UPDATE usage_cleanup_tasks
		SET status = $1,
			deleted_rows = $2,
			error_message = $3,
			finished_at = NOW(),
			updated_at = NOW()
		WHERE id = $4
	`
	_, err := r.sql.ExecContext(ctx, query, service.UsageCleanupStatusFailed, deletedRows, errorMsg, taskID)
	return err
}

func (r *usageCleanupRepository) DeleteUsageLogsBatch(ctx context.Context, filters service.UsageCleanupFilters, limit int) (int64, error) {
	if filters.StartTime.IsZero() || filters.EndTime.IsZero() {
		return 0, fmt.Errorf("cleanup filters missing time range")
	}
	whereClause, args := buildUsageCleanupWhere(filters)
	if whereClause == "" {
		return 0, fmt.Errorf("cleanup filters missing time range")
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
		WITH target AS (
			SELECT id
			FROM usage_logs
			WHERE %s
			ORDER BY created_at ASC, id ASC
			LIMIT $%d
		)
		DELETE FROM usage_logs
		WHERE id IN (SELECT id FROM target)
		RETURNING id
	`, whereClause, len(args))

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	var deleted int64
	for rows.Next() {
		deleted++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return deleted, nil
}

func buildUsageCleanupWhere(filters service.UsageCleanupFilters) (string, []any) {
	conditions := make([]string, 0, 8)
	args := make([]any, 0, 8)
	idx := 1
	if !filters.StartTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, filters.StartTime)
		idx++
	}
	if !filters.EndTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, filters.EndTime)
		idx++
	}
	if filters.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *filters.UserID)
		idx++
	}
	if filters.APIKeyID != nil {
		conditions = append(conditions, fmt.Sprintf("api_key_id = $%d", idx))
		args = append(args, *filters.APIKeyID)
		idx++
	}
	if filters.AccountID != nil {
		conditions = append(conditions, fmt.Sprintf("account_id = $%d", idx))
		args = append(args, *filters.AccountID)
		idx++
	}
	if filters.GroupID != nil {
		conditions = append(conditions, fmt.Sprintf("group_id = $%d", idx))
		args = append(args, *filters.GroupID)
		idx++
	}
	if filters.Model != nil {
		model := strings.TrimSpace(*filters.Model)
		if model != "" {
			conditions = append(conditions, fmt.Sprintf("model = $%d", idx))
			args = append(args, model)
			idx++
		}
	}
	if filters.RequestType != nil {
		condition, conditionArgs := buildRequestTypeFilterCondition(idx, *filters.RequestType)
		conditions = append(conditions, condition)
		args = append(args, conditionArgs...)
		idx += len(conditionArgs)
	} else if filters.Stream != nil {
		conditions = append(conditions, fmt.Sprintf("stream = $%d", idx))
		args = append(args, *filters.Stream)
		idx++
	}
	if filters.BillingType != nil {
		conditions = append(conditions, fmt.Sprintf("billing_type = $%d", idx))
		args = append(args, *filters.BillingType)
	}
	return strings.Join(conditions, " AND "), args
}

func (r *usageCleanupRepository) createTaskWithEnt(ctx context.Context, task *service.UsageCleanupTask) error {
	client := clientFromContext(ctx, r.client)
	filtersJSON, err := json.Marshal(task.Filters)
	if err != nil {
		return fmt.Errorf("marshal cleanup filters: %w", err)
	}
	if strings.TrimSpace(task.CreatedSource) == "" {
		task.CreatedSource = "admin"
	}
	builder := client.UsageCleanupTask.
		Create().
		SetStatus(task.Status).
		SetFilters(json.RawMessage(filtersJSON)).
		SetCreatedSource(task.CreatedSource).
		SetDeletedRows(task.DeletedRows)
	if task.CreatedBy > 0 {
		builder.SetCreatedBy(task.CreatedBy)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	task.ID = created.ID
	task.CreatedAt = created.CreatedAt
	task.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *usageCleanupRepository) createTaskWithSQL(ctx context.Context, task *service.UsageCleanupTask) error {
	filtersJSON, err := json.Marshal(task.Filters)
	if err != nil {
		return fmt.Errorf("marshal cleanup filters: %w", err)
	}
	if strings.TrimSpace(task.CreatedSource) == "" {
		task.CreatedSource = "admin"
	}
	query := `
		INSERT INTO usage_cleanup_tasks (
			status,
			filters,
			created_by,
			created_source,
			deleted_rows
		) VALUES ($1, $2, NULLIF($3, 0), $4, $5)
		RETURNING id, created_at, updated_at
	`
	if err := scanSingleRow(ctx, r.sql, query, []any{task.Status, filtersJSON, task.CreatedBy, task.CreatedSource, task.DeletedRows}, &task.ID, &task.CreatedAt, &task.UpdatedAt); err != nil {
		return err
	}
	return nil
}

func (r *usageCleanupRepository) listTasksWithEnt(ctx context.Context, params pagination.PaginationParams) ([]service.UsageCleanupTask, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)
	query := client.UsageCleanupTask.Query()
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	if total == 0 {
		return []service.UsageCleanupTask{}, paginationResultFromTotal(0, params), nil
	}
	rows, err := query.
		Order(dbent.Desc(dbusagecleanuptask.FieldCreatedAt), dbent.Desc(dbusagecleanuptask.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	tasks := make([]service.UsageCleanupTask, 0, len(rows))
	for _, row := range rows {
		task, err := usageCleanupTaskFromEnt(row)
		if err != nil {
			return nil, nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, paginationResultFromTotal(int64(total), params), nil
}

func (r *usageCleanupRepository) getTaskStatusWithEnt(ctx context.Context, taskID int64) (string, error) {
	client := clientFromContext(ctx, r.client)
	task, err := client.UsageCleanupTask.Query().
		Where(dbusagecleanuptask.IDEQ(taskID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return "", sql.ErrNoRows
		}
		return "", err
	}
	return task.Status, nil
}

func (r *usageCleanupRepository) updateTaskProgressWithEnt(ctx context.Context, taskID int64, deletedRows int64) error {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	_, err := client.UsageCleanupTask.Update().
		Where(dbusagecleanuptask.IDEQ(taskID)).
		SetDeletedRows(deletedRows).
		SetUpdatedAt(now).
		Save(ctx)
	return err
}

func (r *usageCleanupRepository) cancelTaskWithEnt(ctx context.Context, taskID int64, canceledBy int64) (bool, error) {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	affected, err := client.UsageCleanupTask.Update().
		Where(
			dbusagecleanuptask.IDEQ(taskID),
			dbusagecleanuptask.StatusIn(service.UsageCleanupStatusPending, service.UsageCleanupStatusRunning),
		).
		SetStatus(service.UsageCleanupStatusCanceled).
		SetCanceledBy(canceledBy).
		SetCanceledAt(now).
		SetFinishedAt(now).
		ClearErrorMessage().
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *usageCleanupRepository) markTaskSucceededWithEnt(ctx context.Context, taskID int64, deletedRows int64) error {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	_, err := client.UsageCleanupTask.Update().
		Where(dbusagecleanuptask.IDEQ(taskID)).
		SetStatus(service.UsageCleanupStatusSucceeded).
		SetDeletedRows(deletedRows).
		SetFinishedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	return err
}

func (r *usageCleanupRepository) markTaskFailedWithEnt(ctx context.Context, taskID int64, deletedRows int64, errorMsg string) error {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	_, err := client.UsageCleanupTask.Update().
		Where(dbusagecleanuptask.IDEQ(taskID)).
		SetStatus(service.UsageCleanupStatusFailed).
		SetDeletedRows(deletedRows).
		SetErrorMessage(errorMsg).
		SetFinishedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	return err
}

func usageCleanupTaskFromEnt(row *dbent.UsageCleanupTask) (service.UsageCleanupTask, error) {
	task := service.UsageCleanupTask{
		ID:            row.ID,
		Status:        row.Status,
		CreatedSource: row.CreatedSource,
		DeletedRows:   row.DeletedRows,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
	if row.CreatedBy != nil {
		task.CreatedBy = *row.CreatedBy
	}
	if len(row.Filters) > 0 {
		if err := json.Unmarshal(row.Filters, &task.Filters); err != nil {
			return service.UsageCleanupTask{}, fmt.Errorf("parse cleanup filters: %w", err)
		}
	}
	if row.ErrorMessage != nil {
		task.ErrorMsg = row.ErrorMessage
	}
	if row.CanceledBy != nil {
		task.CanceledBy = row.CanceledBy
	}
	if row.CanceledAt != nil {
		task.CanceledAt = row.CanceledAt
	}
	if row.StartedAt != nil {
		task.StartedAt = row.StartedAt
	}
	if row.FinishedAt != nil {
		task.FinishedAt = row.FinishedAt
	}
	return task, nil
}
