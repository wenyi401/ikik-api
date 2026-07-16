package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ikik-api/internal/service"
)

type accountBatchTaskRepository struct {
	db *sql.DB
}

func NewAccountBatchTaskRepository(db *sql.DB) service.AccountBatchTaskRepository {
	return &accountBatchTaskRepository{db: db}
}

func (r *accountBatchTaskRepository) CreateTask(ctx context.Context, input service.CreateAccountBatchTaskInput) (*service.AccountBatchTask, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	task, err := queryAccountBatchTask(ctx, tx, `
		INSERT INTO account_batch_tasks (scope, operation, status, total, created_by, owner_user_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, scope, operation, status, total, processed, success, failed, created_by, owner_user_id,
			error_message, started_at, finished_at, created_at, updated_at
	`, []any{
		input.Scope,
		input.Operation,
		service.AccountBatchTaskStatusPending,
		len(input.AccountIDs),
		input.CreatedBy,
		input.OwnerUserID,
	})
	if err != nil {
		return nil, err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO account_batch_task_items (task_id, account_id, status)
		VALUES ($1, $2, $3)
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stmt.Close() }()
	for _, accountID := range input.AccountIDs {
		if _, err := stmt.ExecContext(ctx, task.ID, accountID, service.AccountBatchTaskStatusPending); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	task.Items, err = r.ListPendingItems(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (r *accountBatchTaskRepository) GetTask(ctx context.Context, id int64) (*service.AccountBatchTask, error) {
	task, err := queryAccountBatchTask(ctx, r.db, `
		SELECT id, scope, operation, status, total, processed, success, failed, created_by, owner_user_id,
			error_message, started_at, finished_at, created_at, updated_at
		FROM account_batch_tasks
		WHERE id = $1
	`, []any{id})
	if err != nil {
		return nil, err
	}
	items, err := r.listItems(ctx, id, "")
	if err != nil {
		return nil, err
	}
	task.Items = items
	return task, nil
}

func (r *accountBatchTaskRepository) ClaimNextPendingTask(ctx context.Context, staleRunningAfterSeconds int64) (*service.AccountBatchTask, error) {
	if staleRunningAfterSeconds <= 0 {
		staleRunningAfterSeconds = int64(30 * 60)
	}
	task, err := queryAccountBatchTask(ctx, r.db, `
		WITH next AS (
			SELECT id
			FROM account_batch_tasks
			WHERE status = $1
				OR (
					status = $2
					AND started_at IS NOT NULL
					AND started_at < NOW() - ($3 * interval '1 second')
				)
			ORDER BY created_at ASC, id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		), reset_running_items AS (
			UPDATE account_batch_task_items AS items
			SET status = $1,
				started_at = NULL,
				updated_at = NOW()
			FROM next
			WHERE items.task_id = next.id
				AND items.status = $2
			RETURNING items.id
		)
		UPDATE account_batch_tasks AS tasks
		SET status = $4,
			started_at = COALESCE(tasks.started_at, NOW()),
			finished_at = NULL,
			error_message = NULL,
			updated_at = NOW()
		FROM next
		WHERE tasks.id = next.id
		RETURNING tasks.id, tasks.scope, tasks.operation, tasks.status, tasks.total, tasks.processed, tasks.success, tasks.failed,
			tasks.created_by, tasks.owner_user_id, tasks.error_message, tasks.started_at, tasks.finished_at, tasks.created_at, tasks.updated_at
	`, []any{
		service.AccountBatchTaskStatusPending,
		service.AccountBatchTaskStatusRunning,
		staleRunningAfterSeconds,
		service.AccountBatchTaskStatusRunning,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (r *accountBatchTaskRepository) ListPendingItems(ctx context.Context, taskID int64) ([]service.AccountBatchTaskItem, error) {
	return r.listItems(ctx, taskID, service.AccountBatchTaskStatusPending)
}

func (r *accountBatchTaskRepository) MarkItemRunning(ctx context.Context, itemID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE account_batch_task_items
		SET status = $1, started_at = COALESCE(started_at, NOW()), updated_at = NOW()
		WHERE id = $2 AND status = $3
	`, service.AccountBatchTaskStatusRunning, itemID, service.AccountBatchTaskStatusPending)
	return err
}

func (r *accountBatchTaskRepository) MarkItemSucceeded(ctx context.Context, itemID int64, result map[string]any) error {
	payload, err := json.Marshal(normalizeJSONMap(result))
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE account_batch_task_items
		SET status = $1, result = $2::jsonb, error_message = NULL, finished_at = NOW(), updated_at = NOW()
		WHERE id = $3
	`, service.AccountBatchTaskStatusSucceeded, payload, itemID)
	return err
}

func (r *accountBatchTaskRepository) MarkItemFailed(ctx context.Context, itemID int64, errorMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE account_batch_task_items
		SET status = $1, error_message = $2, finished_at = NOW(), updated_at = NOW()
		WHERE id = $3
	`, service.AccountBatchTaskStatusFailed, strings.TrimSpace(errorMessage), itemID)
	return err
}

func (r *accountBatchTaskRepository) RefreshTaskProgress(ctx context.Context, taskID int64) (*service.AccountBatchTask, error) {
	task, err := queryAccountBatchTask(ctx, r.db, `
		WITH counts AS (
			SELECT
				COUNT(*) FILTER (WHERE status IN ($2, $3, $4))::integer AS processed,
				COUNT(*) FILTER (WHERE status = $3)::integer AS success,
				COUNT(*) FILTER (WHERE status = $4)::integer AS failed
			FROM account_batch_task_items
			WHERE task_id = $1
		)
		UPDATE account_batch_tasks AS tasks
		SET processed = counts.processed,
			success = counts.success,
			failed = counts.failed,
			updated_at = NOW()
		FROM counts
		WHERE tasks.id = $1
		RETURNING tasks.id, tasks.scope, tasks.operation, tasks.status, tasks.total, tasks.processed, tasks.success, tasks.failed,
			tasks.created_by, tasks.owner_user_id, tasks.error_message, tasks.started_at, tasks.finished_at, tasks.created_at, tasks.updated_at
	`, []any{
		taskID,
		service.AccountBatchTaskStatusCanceled,
		service.AccountBatchTaskStatusSucceeded,
		service.AccountBatchTaskStatusFailed,
	})
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (r *accountBatchTaskRepository) MarkTaskSucceeded(ctx context.Context, taskID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE account_batch_tasks
		SET status = $1, error_message = NULL, finished_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`, service.AccountBatchTaskStatusSucceeded, taskID)
	return err
}

func (r *accountBatchTaskRepository) MarkTaskFailed(ctx context.Context, taskID int64, errorMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE account_batch_tasks
		SET status = $1, error_message = $2, finished_at = NOW(), updated_at = NOW()
		WHERE id = $3
	`, service.AccountBatchTaskStatusFailed, strings.TrimSpace(errorMessage), taskID)
	return err
}

func (r *accountBatchTaskRepository) listItems(ctx context.Context, taskID int64, status string) ([]service.AccountBatchTaskItem, error) {
	query := `
		SELECT id, task_id, account_id, status, error_message, result, started_at, finished_at, created_at, updated_at
		FROM account_batch_task_items
		WHERE task_id = $1
	`
	args := []any{taskID}
	if strings.TrimSpace(status) != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY id ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AccountBatchTaskItem, 0)
	for rows.Next() {
		item, err := scanAccountBatchTaskItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func queryAccountBatchTask(ctx context.Context, q sqlQueryer, query string, args []any) (*service.AccountBatchTask, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	task, err := scanAccountBatchTask(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &task, nil
}

func scanAccountBatchTask(rows *sql.Rows) (service.AccountBatchTask, error) {
	var task service.AccountBatchTask
	var ownerUserID sql.NullInt64
	var errorMessage sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := rows.Scan(
		&task.ID,
		&task.Scope,
		&task.Operation,
		&task.Status,
		&task.Total,
		&task.Processed,
		&task.Success,
		&task.Failed,
		&task.CreatedBy,
		&ownerUserID,
		&errorMessage,
		&startedAt,
		&finishedAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return task, err
	}
	if ownerUserID.Valid {
		v := ownerUserID.Int64
		task.OwnerUserID = &v
	}
	if errorMessage.Valid {
		v := errorMessage.String
		task.ErrorMessage = &v
	}
	if startedAt.Valid {
		v := startedAt.Time
		task.StartedAt = &v
	}
	if finishedAt.Valid {
		v := finishedAt.Time
		task.FinishedAt = &v
	}
	return task, nil
}

func scanAccountBatchTaskItem(rows *sql.Rows) (service.AccountBatchTaskItem, error) {
	var item service.AccountBatchTaskItem
	var errorMessage sql.NullString
	var resultJSON []byte
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := rows.Scan(
		&item.ID,
		&item.TaskID,
		&item.AccountID,
		&item.Status,
		&errorMessage,
		&resultJSON,
		&startedAt,
		&finishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return item, err
	}
	if errorMessage.Valid {
		v := errorMessage.String
		item.ErrorMessage = &v
	}
	if len(resultJSON) > 0 {
		if err := json.Unmarshal(resultJSON, &item.Result); err != nil {
			return item, fmt.Errorf("parse account batch item result: %w", err)
		}
	}
	if startedAt.Valid {
		v := startedAt.Time
		item.StartedAt = &v
	}
	if finishedAt.Valid {
		v := finishedAt.Time
		item.FinishedAt = &v
	}
	return item, nil
}
