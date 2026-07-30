package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"ikik-api/internal/service"
)

type promptSubmissionRepository struct {
	db *sql.DB
}

func NewPromptSubmissionRepository(db *sql.DB) service.PromptSubmissionRepository {
	return &promptSubmissionRepository{db: db}
}

func (r *promptSubmissionRepository) CreatePromptSubmission(ctx context.Context, input service.CreatePromptSubmissionInput) (*service.PromptSubmission, error) {
	const query = `
		INSERT INTO prompt_library_submissions (
			user_id, title, description, content, type, category, media_url
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id, user_id, ''::text, ''::text, title, description, content, type,
			category, media_url, status, review_note, reviewed_by, reviewed_at,
			created_at, updated_at
	`
	item, err := scanPromptSubmission(r.db.QueryRowContext(
		ctx,
		query,
		input.UserID,
		input.Title,
		input.Description,
		input.Content,
		input.Type,
		input.Category,
		input.MediaURL,
	))
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *promptSubmissionRepository) CountPendingPromptSubmissions(ctx context.Context, userID int64) (int, error) {
	var count int
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM prompt_library_submissions WHERE user_id = $1 AND status = 'pending'`,
		userID,
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *promptSubmissionRepository) ListApprovedPromptSubmissions(ctx context.Context, page, pageSize int) ([]service.PromptSubmission, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM prompt_library_submissions WHERE status = 'approved'`,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	const query = `
		SELECT
			s.id, s.user_id,
			COALESCE(NULLIF(u.username, ''), split_part(u.email, '@', 1), 'user'),
			''::text,
			s.title, s.description, s.content, s.type, s.category, s.media_url,
			s.status, s.review_note, s.reviewed_by, s.reviewed_at,
			s.created_at, s.updated_at
		FROM prompt_library_submissions s
		JOIN users u ON u.id = s.user_id
		WHERE s.status = 'approved'
		ORDER BY s.reviewed_at DESC NULLS LAST, s.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.QueryContext(ctx, query, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := scanPromptSubmissionRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *promptSubmissionRepository) ListPromptSubmissionsAdmin(ctx context.Context, filters service.PromptSubmissionAdminFilters) ([]service.PromptSubmission, int64, error) {
	where := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if filters.Status != "" {
		args = append(args, filters.Status)
		where = append(where, fmt.Sprintf("s.status = $%d", len(args)))
	}
	if filters.Search != "" {
		args = append(args, "%"+filters.Search+"%")
		placeholder := fmt.Sprintf("$%d", len(args))
		where = append(where, "(s.title ILIKE "+placeholder+" OR s.description ILIKE "+placeholder+" OR s.content ILIKE "+placeholder+" OR u.email ILIKE "+placeholder+" OR u.username ILIKE "+placeholder+")")
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	countQuery := `
		SELECT COUNT(*)
		FROM prompt_library_submissions s
		JOIN users u ON u.id = s.user_id` + whereSQL
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataArgs := append([]any(nil), args...)
	dataArgs = append(dataArgs, filters.PageSize, (filters.Page-1)*filters.PageSize)
	limitArg := len(dataArgs) - 1
	offsetArg := len(dataArgs)
	query := `
		SELECT
			s.id, s.user_id,
			COALESCE(NULLIF(u.username, ''), split_part(u.email, '@', 1), 'user'),
			COALESCE(u.email, ''),
			s.title, s.description, s.content, s.type, s.category, s.media_url,
			s.status, s.review_note, s.reviewed_by, s.reviewed_at,
			s.created_at, s.updated_at
		FROM prompt_library_submissions s
		JOIN users u ON u.id = s.user_id` + whereSQL + fmt.Sprintf(`
		ORDER BY CASE WHEN s.status = 'pending' THEN 0 ELSE 1 END, s.created_at DESC
		LIMIT $%d OFFSET $%d`, limitArg, offsetArg)

	rows, err := r.db.QueryContext(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := scanPromptSubmissionRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *promptSubmissionRepository) ReviewPromptSubmission(ctx context.Context, id, reviewerID int64, status, note string) (*service.PromptSubmission, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE prompt_library_submissions
		SET status = $2,
			review_note = $3,
			reviewed_by = $4,
			reviewed_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, id, status, note, reviewerID)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, service.ErrPromptSubmissionNotFound
	}
	return r.getPromptSubmissionByID(ctx, id)
}

func (r *promptSubmissionRepository) getPromptSubmissionByID(ctx context.Context, id int64) (*service.PromptSubmission, error) {
	const query = `
		SELECT
			s.id, s.user_id,
			COALESCE(NULLIF(u.username, ''), split_part(u.email, '@', 1), 'user'),
			COALESCE(u.email, ''),
			s.title, s.description, s.content, s.type, s.category, s.media_url,
			s.status, s.review_note, s.reviewed_by, s.reviewed_at,
			s.created_at, s.updated_at
		FROM prompt_library_submissions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1
	`
	item, err := scanPromptSubmission(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPromptSubmissionNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

type promptSubmissionScanner interface {
	Scan(dest ...any) error
}

func scanPromptSubmission(scanner promptSubmissionScanner) (*service.PromptSubmission, error) {
	var item service.PromptSubmission
	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.Username,
		&item.UserEmail,
		&item.Title,
		&item.Description,
		&item.Content,
		&item.Type,
		&item.Category,
		&item.MediaURL,
		&item.Status,
		&item.ReviewNote,
		&reviewedBy,
		&reviewedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if reviewedBy.Valid {
		value := reviewedBy.Int64
		item.ReviewedBy = &value
	}
	if reviewedAt.Valid {
		value := reviewedAt.Time
		item.ReviewedAt = &value
	}
	return &item, nil
}

func scanPromptSubmissionRows(rows *sql.Rows) ([]service.PromptSubmission, error) {
	items := make([]service.PromptSubmission, 0)
	for rows.Next() {
		item, err := scanPromptSubmission(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
