package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"

	"ikik-api/internal/service"
)

type promptLibraryTranslationRepository struct {
	db *sql.DB
}

func NewPromptLibraryTranslationRepository(db *sql.DB) service.PromptLibraryTranslationRepository {
	return &promptLibraryTranslationRepository{db: db}
}

func (r *promptLibraryTranslationRepository) GetByPromptIDs(
	ctx context.Context,
	locale string,
	promptIDs []string,
) (map[string]service.PromptLibraryTranslation, error) {
	result := make(map[string]service.PromptLibraryTranslation, len(promptIDs))
	if len(promptIDs) == 0 {
		return result, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT prompt_id, locale, source_hash, title, description, category
		FROM prompt_library_translations
		WHERE locale = $1 AND prompt_id = ANY($2)
	`, locale, pq.Array(promptIDs))
	if err != nil {
		return nil, fmt.Errorf("query prompt library translations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item service.PromptLibraryTranslation
		if err := rows.Scan(&item.PromptID, &item.Locale, &item.SourceHash, &item.Title, &item.Description, &item.Category); err != nil {
			return nil, fmt.Errorf("scan prompt library translation: %w", err)
		}
		result[item.PromptID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prompt library translations: %w", err)
	}
	return result, nil
}

func (r *promptLibraryTranslationRepository) Upsert(
	ctx context.Context,
	items []service.PromptLibraryTranslation,
) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin prompt library translation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO prompt_library_translations (
			prompt_id, locale, source_hash, title, description, category, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (prompt_id, locale) DO UPDATE SET
			source_hash = EXCLUDED.source_hash,
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			category = EXCLUDED.category,
			updated_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("prepare prompt library translation upsert: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		if _, err := stmt.ExecContext(
			ctx,
			item.PromptID,
			item.Locale,
			item.SourceHash,
			item.Title,
			item.Description,
			item.Category,
		); err != nil {
			return fmt.Errorf("upsert prompt library translation %s: %w", item.PromptID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit prompt library translations: %w", err)
	}
	return nil
}

func (r *promptLibraryTranslationRepository) Count(ctx context.Context, locale string) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM prompt_library_translations WHERE locale = $1`,
		locale,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count prompt library translations: %w", err)
	}
	return count, nil
}
