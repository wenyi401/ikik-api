package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ikik-api/internal/service"
)

type petRepository struct {
	db *sql.DB
}

func NewPetRepository(db *sql.DB) service.PetRepository {
	return &petRepository{db: db}
}

func (r *petRepository) ListAssets(ctx context.Context, userID int64) ([]service.PetAsset, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id::text, owner_user_id, pet_key, display_name, description, sprite_version,
		       storage_key, sha256, size_bytes, width, height, license, created_at
		FROM pet_assets
		WHERE owner_user_id IS NULL OR owner_user_id = $1
		ORDER BY owner_user_id NULLS FIRST,
		         CASE WHEN pet_key = 'shinobu-kocho--wangfan002' THEN 0 ELSE 1 END,
		         display_name ASC, created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.PetAsset, 0)
	for rows.Next() {
		item, scanErr := scanPetAsset(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *petRepository) CreateAsset(ctx context.Context, asset *service.PetAsset) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO pet_assets (id, owner_user_id, pet_key, display_name, description, sprite_version, storage_key, sha256, size_bytes, width, height, license)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at`, asset.ID, asset.OwnerUserID, asset.PetKey, asset.DisplayName, asset.Description, asset.SpriteVersion, asset.StorageKey, asset.SHA256, asset.SizeBytes, asset.Width, asset.Height, asset.License).Scan(&asset.CreatedAt)
}

func (r *petRepository) GetAsset(ctx context.Context, userID int64, assetID string) (*service.PetAsset, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id::text, owner_user_id, pet_key, display_name, description, sprite_version,
		       storage_key, sha256, size_bytes, width, height, license, created_at
		FROM pet_assets WHERE id = $1::uuid AND (owner_user_id IS NULL OR owner_user_id = $2)`, assetID, userID)
	item, err := scanPetAsset(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPetAssetNotFound
	}
	return item, err
}

func (r *petRepository) DeleteAsset(ctx context.Context, userID int64, assetID string) (*service.PetAsset, error) {
	row := r.db.QueryRowContext(ctx, `
		DELETE FROM pet_assets WHERE id = $1::uuid AND owner_user_id = $2
		RETURNING id::text, owner_user_id, pet_key, display_name, description, sprite_version,
		          storage_key, sha256, size_bytes, width, height, license, created_at`, assetID, userID)
	item, err := scanPetAsset(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPetAssetNotFound
	}
	return item, err
}

type petAssetScanner interface{ Scan(dest ...any) error }

func scanPetAsset(row petAssetScanner) (*service.PetAsset, error) {
	var item service.PetAsset
	var owner sql.NullInt64
	if err := row.Scan(&item.ID, &owner, &item.PetKey, &item.DisplayName, &item.Description, &item.SpriteVersion, &item.StorageKey, &item.SHA256, &item.SizeBytes, &item.Width, &item.Height, &item.License, &item.CreatedAt); err != nil {
		return nil, err
	}
	if owner.Valid {
		item.OwnerUserID = &owner.Int64
	} else {
		item.IsBuiltIn = true
	}
	return &item, nil
}

func defaultPetPreferences(userID int64) *service.PetPreferences {
	defaultAssetID := service.DefaultPetAssetID
	return &service.PetPreferences{
		UserID: userID, SelectedAssetID: &defaultAssetID, Enabled: false,
		Size: "medium", Anchor: "bottom-right", ActivityReactions: true,
	}
}

func (r *petRepository) GetPreferences(ctx context.Context, userID int64) (*service.PetPreferences, error) {
	prefs := defaultPetPreferences(userID)
	var selected sql.NullString
	var positionX, positionY sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `
		SELECT selected_asset_id::text, assistant_group_id, assistant_model, enabled, size, anchor, reduced_motion, activity_reactions,
		       position_x, position_y
		FROM pet_user_preferences WHERE user_id = $1`, userID).Scan(
		&selected, &prefs.AssistantGroupID, &prefs.AssistantModel, &prefs.Enabled, &prefs.Size, &prefs.Anchor, &prefs.ReducedMotion,
		&prefs.ActivityReactions, &positionX, &positionY,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return prefs, nil
	}
	if err != nil {
		return nil, err
	}
	if selected.Valid {
		prefs.SelectedAssetID = &selected.String
	}
	if positionX.Valid && positionY.Valid {
		prefs.PositionX = &positionX.Float64
		prefs.PositionY = &positionY.Float64
	}
	return prefs, nil
}

func (r *petRepository) UpsertPreferences(ctx context.Context, prefs service.PetPreferences) (*service.PetPreferences, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pet_user_preferences (user_id, selected_asset_id, assistant_group_id, assistant_model, enabled, size, anchor, reduced_motion, activity_reactions, position_x, position_y, updated_at)
		VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
		  selected_asset_id = EXCLUDED.selected_asset_id, assistant_group_id = EXCLUDED.assistant_group_id,
		  assistant_model = EXCLUDED.assistant_model, enabled = EXCLUDED.enabled, size = EXCLUDED.size,
		  anchor = EXCLUDED.anchor, reduced_motion = EXCLUDED.reduced_motion,
		  activity_reactions = EXCLUDED.activity_reactions, position_x = EXCLUDED.position_x,
		  position_y = EXCLUDED.position_y, updated_at = NOW()`,
		prefs.UserID, prefs.SelectedAssetID, prefs.AssistantGroupID, prefs.AssistantModel, prefs.Enabled, prefs.Size, prefs.Anchor,
		prefs.ReducedMotion, prefs.ActivityReactions, prefs.PositionX, prefs.PositionY)
	if err != nil {
		return nil, err
	}
	return &prefs, nil
}

func (r *petRepository) ListPublishedKnowledge(ctx context.Context, limit int) ([]service.PetKnowledgeDocument, error) {
	if limit <= 0 || limit > service.PetKnowledgeMaxDocs {
		limit = service.PetKnowledgeMaxDocs
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, slug, title, content_md, status, version, updated_by, created_at, updated_at
		FROM pet_knowledge_documents WHERE status = 'published' ORDER BY updated_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPetKnowledgeRows(rows)
}

func (r *petRepository) ListKnowledge(ctx context.Context) ([]service.PetKnowledgeDocument, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, slug, title, content_md, status, version, updated_by, created_at, updated_at
		FROM pet_knowledge_documents ORDER BY updated_at DESC LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPetKnowledgeRows(rows)
}

func scanPetKnowledgeRows(rows *sql.Rows) ([]service.PetKnowledgeDocument, error) {
	items := make([]service.PetKnowledgeDocument, 0)
	for rows.Next() {
		var item service.PetKnowledgeDocument
		var actor sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Slug, &item.Title, &item.ContentMD, &item.Status, &item.Version, &actor, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if actor.Valid {
			item.UpdatedBy = &actor.Int64
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *petRepository) CreateKnowledge(ctx context.Context, doc *service.PetKnowledgeDocument) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO pet_knowledge_documents (slug, title, content_md, status, updated_by)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, version, created_at, updated_at`,
		doc.Slug, doc.Title, doc.ContentMD, doc.Status, doc.UpdatedBy).Scan(&doc.ID, &doc.Version, &doc.CreatedAt, &doc.UpdatedAt)
}

func (r *petRepository) UpdateKnowledge(ctx context.Context, doc *service.PetKnowledgeDocument) error {
	err := r.db.QueryRowContext(ctx, `
		UPDATE pet_knowledge_documents SET slug = $2, title = $3, content_md = $4, status = $5,
		       version = version + 1, updated_by = $6, updated_at = NOW()
		WHERE id = $1 RETURNING version, created_at, updated_at`,
		doc.ID, doc.Slug, doc.Title, doc.ContentMD, doc.Status, doc.UpdatedBy).Scan(&doc.Version, &doc.CreatedAt, &doc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrPetKnowledgeNotFound
	}
	return err
}

func (r *petRepository) DeleteKnowledge(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM pet_knowledge_documents WHERE id = $1`, id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return service.ErrPetKnowledgeNotFound
	}
	return nil
}

func (r *petRepository) CreateConversation(ctx context.Context, conversation *service.PetConversation) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO pet_conversations (id, user_id, title) VALUES ($1::uuid, $2, $3)
		RETURNING created_at, updated_at`, conversation.ID, conversation.UserID, conversation.Title).Scan(&conversation.CreatedAt, &conversation.UpdatedAt)
}

func (r *petRepository) GetConversation(ctx context.Context, userID int64, id string) (*service.PetConversation, error) {
	var conversation service.PetConversation
	err := r.db.QueryRowContext(ctx, `
		SELECT id::text, user_id, title, created_at, updated_at FROM pet_conversations
		WHERE id = $1::uuid AND user_id = $2`, id, userID).Scan(&conversation.ID, &conversation.UserID, &conversation.Title, &conversation.CreatedAt, &conversation.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPetConversationMissing
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, conversation_id::text, role, content, citations, refused, created_at
		FROM pet_messages WHERE conversation_id = $1::uuid ORDER BY created_at ASC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	conversation.Messages = make([]service.PetMessage, 0)
	for rows.Next() {
		var message service.PetMessage
		var citations []byte
		if err := rows.Scan(&message.ID, &message.ConversationID, &message.Role, &message.Content, &citations, &message.Refused, &message.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(citations, &message.Citations); err != nil {
			return nil, fmt.Errorf("decode pet citations: %w", err)
		}
		conversation.Messages = append(conversation.Messages, message)
	}
	return &conversation, rows.Err()
}

func (r *petRepository) AppendMessage(ctx context.Context, message *service.PetMessage) error {
	citations, err := json.Marshal(message.Citations)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	err = tx.QueryRowContext(ctx, `
		INSERT INTO pet_messages (conversation_id, role, content, citations, refused)
		VALUES ($1::uuid, $2, $3, $4::jsonb, $5) RETURNING id, created_at`,
		message.ConversationID, message.Role, message.Content, citations, message.Refused).Scan(&message.ID, &message.CreatedAt)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE pet_conversations SET updated_at = NOW() WHERE id = $1::uuid`, message.ConversationID); err != nil {
		return err
	}
	return tx.Commit()
}

var _ = time.Time{}
