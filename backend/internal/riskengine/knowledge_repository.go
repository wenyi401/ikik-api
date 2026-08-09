package riskengine

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const knowledgeCacheTTL = 30 * time.Second

var ErrKnowledgeEntryNotFound = errors.New("risk knowledge entry not found")

type KnowledgeFilter struct {
	Topic       KnowledgeTopic
	Category    Category
	Disposition KnowledgeDisposition
	Keyword     string
	Enabled     *bool
}

type KnowledgeEntryPage struct {
	Items    []KnowledgeEntry `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Pages    int              `json:"pages"`
	Version  int              `json:"version"`
}

type KnowledgeSummary struct {
	Version                int            `json:"version"`
	Total                  int64          `json:"total"`
	Enabled                int64          `json:"enabled"`
	Risk                   int64          `json:"risk"`
	Safe                   int64          `json:"safe"`
	Review                 int64          `json:"review"`
	UnreviewedObservations int64          `json:"unreviewed_observations"`
	TopicCounts            map[string]int `json:"topic_counts"`
	LastUpdated            *time.Time     `json:"last_updated,omitempty"`
}

type KnowledgeRepository struct {
	db *sql.DB

	cacheMu      sync.RWMutex
	cache        KnowledgeSnapshot
	cacheExpires time.Time
}

func NewKnowledgeRepository(db *sql.DB) *KnowledgeRepository {
	return &KnowledgeRepository{db: db}
}

func (r *KnowledgeRepository) Snapshot(ctx context.Context) (KnowledgeSnapshot, error) {
	if r == nil || r.db == nil {
		return KnowledgeSnapshot{}, errors.New("risk knowledge repository unavailable")
	}
	now := time.Now()
	r.cacheMu.RLock()
	if now.Before(r.cacheExpires) && r.cache.Version > 0 {
		cached := cloneKnowledgeSnapshot(r.cache)
		r.cacheMu.RUnlock()
		return cached, nil
	}
	r.cacheMu.RUnlock()

	version, err := r.currentVersion(ctx)
	if err != nil {
		return KnowledgeSnapshot{}, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,entry_key,topic,category,disposition,intent,actionability,"authorization",
			language,title,example_text,aliases,rationale,enabled,source_type,source_event_id,
			revision,created_by,updated_by,created_at,updated_at
		FROM risk_engine_knowledge_entries
		WHERE enabled=TRUE
		ORDER BY id`)
	if err != nil {
		return KnowledgeSnapshot{}, err
	}
	defer func() { _ = rows.Close() }()
	entries := make([]KnowledgeEntry, 0)
	for rows.Next() {
		entry, scanErr := scanKnowledgeEntry(rows)
		if scanErr != nil {
			return KnowledgeSnapshot{}, scanErr
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return KnowledgeSnapshot{}, err
	}
	snapshot := KnowledgeSnapshot{Version: version, Entries: entries}
	r.cacheMu.Lock()
	r.cache = cloneKnowledgeSnapshot(snapshot)
	r.cacheExpires = now.Add(knowledgeCacheTTL)
	r.cacheMu.Unlock()
	return snapshot, nil
}

func (r *KnowledgeRepository) List(ctx context.Context, filter KnowledgeFilter, page, pageSize int) (*KnowledgeEntryPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("risk knowledge repository unavailable")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if err := ValidateKnowledgeFilter(filter); err != nil {
		return nil, err
	}
	where, args := knowledgeFilterSQL(filter)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM risk_engine_knowledge_entries`+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	query := `SELECT id,entry_key,topic,category,disposition,intent,actionability,"authorization",
		language,title,example_text,aliases,rationale,enabled,source_type,source_event_id,
		revision,created_by,updated_by,created_at,updated_at
		FROM risk_engine_knowledge_entries` + where +
		fmt.Sprintf(" ORDER BY enabled DESC, updated_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]KnowledgeEntry, 0, pageSize)
	for rows.Next() {
		entry, scanErr := scanKnowledgeEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	version, err := r.currentVersion(ctx)
	if err != nil {
		return nil, err
	}
	pages := 0
	if total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &KnowledgeEntryPage{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages, Version: version}, nil
}

func ValidateKnowledgeFilter(filter KnowledgeFilter) error {
	if filter.Topic != "" {
		if _, ok := knowledgeTopics[filter.Topic]; !ok {
			return errors.New("risk knowledge topic filter is invalid")
		}
	}
	if filter.Category != "" {
		if _, ok := domainCategories[filter.Category]; !ok {
			return errors.New("risk knowledge category filter is invalid")
		}
	}
	if filter.Disposition != "" && filter.Disposition != KnowledgeSafe && filter.Disposition != KnowledgeReview && filter.Disposition != KnowledgeRisk {
		return errors.New("risk knowledge disposition filter is invalid")
	}
	return nil
}

func (r *KnowledgeRepository) Summary(ctx context.Context) (*KnowledgeSummary, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("risk knowledge repository unavailable")
	}
	summary := &KnowledgeSummary{TopicCounts: map[string]int{}}
	var updated sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT s.version, COUNT(e.id), COUNT(e.id) FILTER (WHERE e.enabled),
			COUNT(e.id) FILTER (WHERE e.disposition='risk'),
			COUNT(e.id) FILTER (WHERE e.disposition='safe'),
			COUNT(e.id) FILTER (WHERE e.disposition='review'), s.updated_at
		FROM risk_engine_knowledge_state s
		LEFT JOIN risk_engine_knowledge_entries e ON TRUE
		WHERE s.singleton=TRUE
		GROUP BY s.version,s.updated_at`).Scan(
		&summary.Version, &summary.Total, &summary.Enabled, &summary.Risk, &summary.Safe, &summary.Review, &updated,
	)
	if err != nil {
		return nil, err
	}
	if updated.Valid {
		value := updated.Time.UTC()
		summary.LastUpdated = &value
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM risk_engine_v2_observations WHERE review_status='unreviewed'`).Scan(&summary.UnreviewedObservations); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT topic,COUNT(*) FROM risk_engine_knowledge_entries WHERE enabled=TRUE GROUP BY topic ORDER BY topic`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var topic string
		var count int
		if err := rows.Scan(&topic, &count); err != nil {
			return nil, err
		}
		summary.TopicCounts[topic] = count
	}
	return summary, rows.Err()
}

func (r *KnowledgeRepository) Create(ctx context.Context, input KnowledgeWriteInput, actorID int64) (*KnowledgeEntry, int, error) {
	input, err := NormalizeKnowledgeInput(input)
	if err != nil {
		return nil, 0, err
	}
	entryKey, err := generateKnowledgeEntryKey()
	if err != nil {
		return nil, 0, err
	}
	aliases, _ := json.Marshal(input.Aliases)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	version, err := bumpKnowledgeVersion(ctx, tx, actorID)
	if err != nil {
		return nil, 0, err
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO risk_engine_knowledge_entries (
			entry_key,topic,category,disposition,intent,actionability,"authorization",
			language,title,example_text,aliases,rationale,enabled,source_type,source_event_id,
			created_by,updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13,$14,$15,$16,$16)
		RETURNING id,entry_key,topic,category,disposition,intent,actionability,"authorization",
			language,title,example_text,aliases,rationale,enabled,source_type,source_event_id,
			revision,created_by,updated_by,created_at,updated_at`,
		entryKey, input.Topic, input.Category, input.Disposition, input.Intent, input.Actionability, input.Authorization,
		input.Language, input.Title, input.ExampleText, aliases, input.Rationale, input.Enabled, input.SourceType,
		nullableKnowledgeSource(input.SourceEventID), nullablePositiveID(actorID))
	entry, err := scanKnowledgeEntry(row)
	if err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	r.invalidateKnowledgeCache()
	return &entry, version, nil
}

func (r *KnowledgeRepository) Update(ctx context.Context, id int64, input KnowledgeWriteInput, actorID int64) (*KnowledgeEntry, int, error) {
	if id <= 0 {
		return nil, 0, ErrKnowledgeEntryNotFound
	}
	input, err := NormalizeKnowledgeInput(input)
	if err != nil {
		return nil, 0, err
	}
	aliases, _ := json.Marshal(input.Aliases)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var lockedID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM risk_engine_knowledge_entries WHERE id=$1 FOR UPDATE`, id).Scan(&lockedID); errors.Is(err, sql.ErrNoRows) {
		return nil, 0, ErrKnowledgeEntryNotFound
	} else if err != nil {
		return nil, 0, err
	}
	version, err := bumpKnowledgeVersion(ctx, tx, actorID)
	if err != nil {
		return nil, 0, err
	}
	row := tx.QueryRowContext(ctx, `
		UPDATE risk_engine_knowledge_entries SET
			topic=$2,category=$3,disposition=$4,intent=$5,actionability=$6,"authorization"=$7,
			language=$8,title=$9,example_text=$10,aliases=$11::jsonb,rationale=$12,enabled=$13,
			source_type=$14,source_event_id=$15,revision=revision+1,updated_by=$16,updated_at=NOW()
		WHERE id=$1
		RETURNING id,entry_key,topic,category,disposition,intent,actionability,"authorization",
			language,title,example_text,aliases,rationale,enabled,source_type,source_event_id,
			revision,created_by,updated_by,created_at,updated_at`,
		id, input.Topic, input.Category, input.Disposition, input.Intent, input.Actionability, input.Authorization,
		input.Language, input.Title, input.ExampleText, aliases, input.Rationale, input.Enabled, input.SourceType,
		nullableKnowledgeSource(input.SourceEventID), nullablePositiveID(actorID))
	entry, err := scanKnowledgeEntry(row)
	if err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	r.invalidateKnowledgeCache()
	return &entry, version, nil
}

func (r *KnowledgeRepository) currentVersion(ctx context.Context) (int, error) {
	var version int
	err := r.db.QueryRowContext(ctx, `SELECT version FROM risk_engine_knowledge_state WHERE singleton=TRUE`).Scan(&version)
	return version, err
}

func bumpKnowledgeVersion(ctx context.Context, tx *sql.Tx, actorID int64) (int, error) {
	var version int
	err := tx.QueryRowContext(ctx, `
		UPDATE risk_engine_knowledge_state
		SET version=version+1,updated_by=$1,updated_at=NOW()
		WHERE singleton=TRUE RETURNING version`, nullablePositiveID(actorID)).Scan(&version)
	return version, err
}

func knowledgeFilterSQL(filter KnowledgeFilter) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	appendValue := func(column string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf("%s=$%d", column, len(args)))
	}
	if filter.Topic != "" {
		appendValue("topic", filter.Topic)
	}
	if filter.Category != "" {
		appendValue("category", filter.Category)
	}
	if filter.Disposition != "" {
		appendValue("disposition", filter.Disposition)
	}
	if filter.Enabled != nil {
		appendValue("enabled", *filter.Enabled)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		clauses = append(clauses, fmt.Sprintf("(title ILIKE $%d OR example_text ILIKE $%d OR aliases::text ILIKE $%d)", len(args), len(args), len(args)))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

type knowledgeRowScanner interface{ Scan(...any) error }

func scanKnowledgeEntry(row knowledgeRowScanner) (KnowledgeEntry, error) {
	var entry KnowledgeEntry
	var aliases []byte
	var sourceEventID, createdBy, updatedBy sql.NullInt64
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&entry.ID, &entry.EntryKey, &entry.Topic, &entry.Category, &entry.Disposition,
		&entry.Intent, &entry.Actionability, &entry.Authorization, &entry.Language,
		&entry.Title, &entry.ExampleText, &aliases, &entry.Rationale, &entry.Enabled,
		&entry.SourceType, &sourceEventID, &entry.Revision, &createdBy, &updatedBy,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return entry, err
	}
	if len(aliases) > 0 {
		if err := json.Unmarshal(aliases, &entry.Aliases); err != nil {
			return entry, err
		}
	}
	entry.SourceEventID = nullInt64Pointer(sourceEventID)
	entry.CreatedBy = nullInt64Pointer(createdBy)
	entry.UpdatedBy = nullInt64Pointer(updatedBy)
	entry.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	entry.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return entry, nil
}

func nullInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func nullableKnowledgeSource(value *int64) any {
	if value == nil || *value <= 0 {
		return nil
	}
	return *value
}

func generateKnowledgeEntryKey() (string, error) {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "knowledge_" + hex.EncodeToString(buffer), nil
}

func cloneKnowledgeSnapshot(value KnowledgeSnapshot) KnowledgeSnapshot {
	clone := KnowledgeSnapshot{Version: value.Version, Entries: append([]KnowledgeEntry(nil), value.Entries...)}
	for index := range clone.Entries {
		clone.Entries[index].Aliases = append([]string(nil), value.Entries[index].Aliases...)
	}
	return clone
}

func (r *KnowledgeRepository) invalidateKnowledgeCache() {
	r.cacheMu.Lock()
	r.cache = KnowledgeSnapshot{}
	r.cacheExpires = time.Time{}
	r.cacheMu.Unlock()
}
