package riskengine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
)

var ErrObservationNotFound = errors.New("risk engine observation not found")

type ObservationReviewStatus string

const (
	ObservationUnreviewed    ObservationReviewStatus = "unreviewed"
	ObservationConfirmed     ObservationReviewStatus = "confirmed"
	ObservationFalsePositive ObservationReviewStatus = "false_positive"
	ObservationInconclusive  ObservationReviewStatus = "inconclusive"
)

type ObservationReviewLabel struct {
	Topic       KnowledgeTopic       `json:"topic,omitempty"`
	Category    Category             `json:"category,omitempty"`
	Disposition KnowledgeDisposition `json:"disposition,omitempty"`
}

type ObservationAuditContext struct {
	EventID         *int64 `json:"event_id,omitempty"`
	Username        string `json:"username"`
	GroupName       string `json:"group_name"`
	Model           string `json:"model"`
	RedactedPreview string `json:"redacted_preview"`
	FullPrompt      string `json:"full_prompt"`
	GuardDecision   string `json:"guard_decision"`
	GuardRiskLevel  string `json:"guard_risk_level"`
}

type Observation struct {
	ID                  int64                   `json:"id"`
	RequestID           string                  `json:"request_id"`
	UserID              *int64                  `json:"user_id,omitempty"`
	GroupID             *int64                  `json:"group_id,omitempty"`
	IncidentFingerprint string                  `json:"incident_fingerprint"`
	Candidate           Candidate               `json:"candidate"`
	Adjudication        Adjudication            `json:"adjudication"`
	Recommendation      Recommendation          `json:"recommendation"`
	WouldProtect        bool                    `json:"would_protect"`
	WouldStrike         bool                    `json:"would_strike"`
	ReasonCode          string                  `json:"reason_code"`
	PolicyVersion       int                     `json:"policy_version"`
	AdjudicatorModel    string                  `json:"adjudicator_model"`
	KnowledgeVersion    int                     `json:"knowledge_version"`
	KnowledgeMatchIDs   []int64                 `json:"knowledge_match_ids"`
	Mode                string                  `json:"mode"`
	ReviewStatus        ObservationReviewStatus `json:"review_status"`
	ReviewLabel         *ObservationReviewLabel `json:"review_label,omitempty"`
	ReviewedBy          *int64                  `json:"reviewed_by,omitempty"`
	ReviewedAt          *time.Time              `json:"reviewed_at,omitempty"`
	ReviewNote          string                  `json:"review_note"`
	ObservedAt          time.Time               `json:"observed_at"`
	CreatedAt           time.Time               `json:"created_at"`
	Audit               ObservationAuditContext `json:"audit"`
}

type ObservationFilter struct {
	ReviewStatus ObservationReviewStatus
	Category     Category
	UserID       *int64
	GroupID      *int64
	Keyword      string
}

type ObservationPage struct {
	Items    []Observation `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Pages    int           `json:"pages"`
}

type ObservationReviewInput struct {
	Status   ObservationReviewStatus `json:"status"`
	Topic    KnowledgeTopic          `json:"topic,omitempty"`
	Category Category                `json:"category,omitempty"`
	Note     string                  `json:"note"`
}

func (r *ShadowRepository) ListObservations(ctx context.Context, filter ObservationFilter, page, pageSize int) (*ObservationPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("risk engine shadow repository unavailable")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where, args, err := observationFilterSQL(filter)
	if err != nil {
		return nil, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM risk_engine_v2_observations o`+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	query := observationSelectSQL() + where + fmt.Sprintf(" ORDER BY o.observed_at DESC,o.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]Observation, 0, pageSize)
	for rows.Next() {
		item, scanErr := scanObservation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pages := 0
	if total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &ObservationPage{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (r *ShadowRepository) ReviewObservation(ctx context.Context, id int64, input ObservationReviewInput, actorID int64) (*Observation, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("risk engine shadow repository unavailable")
	}
	if id <= 0 {
		return nil, ErrObservationNotFound
	}
	input.Note = strings.TrimSpace(SanitizeTrainingText(input.Note))
	if utf8.RuneCountInString(input.Note) > 500 {
		return nil, errors.New("risk observation review note must not exceed 500 characters")
	}
	label, err := normalizeObservationReview(input)
	if err != nil {
		return nil, err
	}
	labelJSON, err := json.Marshal(label)
	if err != nil {
		return nil, err
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE risk_engine_v2_observations SET
			review_status=$2,review_label=$3::jsonb,reviewed_by=$4,reviewed_at=NOW(),review_note=$5
		WHERE id=$1`, id, input.Status, labelJSON, nullablePositiveID(actorID), input.Note)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, ErrObservationNotFound
	}
	return r.GetObservation(ctx, id)
}

func (r *ShadowRepository) GetObservation(ctx context.Context, id int64) (*Observation, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("risk engine shadow repository unavailable")
	}
	item, err := scanObservation(r.db.QueryRowContext(ctx, observationSelectSQL()+" WHERE o.id=$1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrObservationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func normalizeObservationReview(input ObservationReviewInput) (*ObservationReviewLabel, error) {
	switch input.Status {
	case ObservationConfirmed, ObservationFalsePositive:
		if _, ok := knowledgeTopics[input.Topic]; !ok {
			return nil, errors.New("risk observation review topic is invalid")
		}
		if _, ok := domainCategories[input.Category]; !ok || input.Category == CategoryNone {
			return nil, errors.New("risk observation review category is invalid")
		}
		disposition := KnowledgeRisk
		if input.Status == ObservationFalsePositive {
			disposition = KnowledgeSafe
		}
		return &ObservationReviewLabel{Topic: input.Topic, Category: input.Category, Disposition: disposition}, nil
	case ObservationInconclusive:
		return &ObservationReviewLabel{Disposition: KnowledgeReview}, nil
	default:
		return nil, errors.New("risk observation review status is invalid")
	}
}

func ValidateObservationReviewInput(input ObservationReviewInput) error {
	if utf8.RuneCountInString(strings.TrimSpace(input.Note)) > 500 {
		return errors.New("risk observation review note must not exceed 500 characters")
	}
	_, err := normalizeObservationReview(input)
	return err
}

func ValidateObservationFilter(filter ObservationFilter) error {
	_, _, err := observationFilterSQL(filter)
	return err
}

func observationFilterSQL(filter ObservationFilter) (string, []any, error) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	add := func(expression string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(expression, len(args)))
	}
	if filter.ReviewStatus != "" {
		switch filter.ReviewStatus {
		case ObservationUnreviewed, ObservationConfirmed, ObservationFalsePositive, ObservationInconclusive:
			add("o.review_status=$%d", filter.ReviewStatus)
		default:
			return "", nil, errors.New("risk observation review status filter is invalid")
		}
	}
	if filter.Category != "" {
		if _, ok := domainCategories[filter.Category]; !ok {
			return "", nil, errors.New("risk observation category filter is invalid")
		}
		add("o.adjudication->>'category'=$%d", filter.Category)
	}
	if filter.UserID != nil {
		add("o.user_id=$%d", *filter.UserID)
	}
	if filter.GroupID != nil {
		add("o.group_id=$%d", *filter.GroupID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		index := len(args)
		clauses = append(clauses, fmt.Sprintf(`(o.request_id ILIKE $%[1]d OR EXISTS (
			SELECT 1 FROM prompt_audit_events pe
			WHERE pe.request_id=o.request_id
			AND (pe.redacted_preview ILIKE $%[1]d OR pe.username_snapshot ILIKE $%[1]d)
		))`, index))
	}
	if len(clauses) == 0 {
		return "", args, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

func observationSelectSQL() string {
	return `SELECT
		o.id,o.request_id,o.user_id,o.group_id,o.incident_fingerprint,o.candidate,o.adjudication,
		o.recommendation,o.would_protect,o.would_strike,o.reason_code,o.policy_version,
		o.adjudicator_model,o.knowledge_version,o.knowledge_match_ids,o.mode,o.review_status,
		o.review_label,o.reviewed_by,o.reviewed_at,o.review_note,o.observed_at,o.created_at,
		e.id,e.username_snapshot,e.group_name,e.model,e.redacted_preview,e.full_prompt,e.decision,e.risk_level
	FROM risk_engine_v2_observations o
	LEFT JOIN LATERAL (
		SELECT id,username_snapshot,group_name,model,redacted_preview,full_prompt,decision,risk_level
		FROM prompt_audit_events
		WHERE request_id=o.request_id
		ORDER BY id DESC LIMIT 1
	) e ON TRUE`
}

type observationRowScanner interface{ Scan(...any) error }

func scanObservation(row observationRowScanner) (Observation, error) {
	var item Observation
	var userID, groupID, reviewedBy, eventID sql.NullInt64
	var reviewedAt sql.NullTime
	var candidateJSON, adjudicationJSON, reviewLabelJSON []byte
	var knowledgeMatchIDs pq.Int64Array
	var username, groupName, model, preview, fullPrompt, decision, riskLevel sql.NullString
	err := row.Scan(
		&item.ID, &item.RequestID, &userID, &groupID, &item.IncidentFingerprint,
		&candidateJSON, &adjudicationJSON, &item.Recommendation, &item.WouldProtect,
		&item.WouldStrike, &item.ReasonCode, &item.PolicyVersion, &item.AdjudicatorModel,
		&item.KnowledgeVersion, &knowledgeMatchIDs, &item.Mode, &item.ReviewStatus,
		&reviewLabelJSON, &reviewedBy, &reviewedAt, &item.ReviewNote, &item.ObservedAt,
		&item.CreatedAt, &eventID, &username, &groupName, &model, &preview, &fullPrompt,
		&decision, &riskLevel,
	)
	if err != nil {
		return item, err
	}
	if err := json.Unmarshal(candidateJSON, &item.Candidate); err != nil {
		return item, err
	}
	if err := json.Unmarshal(adjudicationJSON, &item.Adjudication); err != nil {
		return item, err
	}
	if len(reviewLabelJSON) > 0 && string(reviewLabelJSON) != "null" {
		var label ObservationReviewLabel
		if err := json.Unmarshal(reviewLabelJSON, &label); err != nil {
			return item, err
		}
		item.ReviewLabel = &label
	}
	item.UserID = nullInt64Pointer(userID)
	item.GroupID = nullInt64Pointer(groupID)
	item.ReviewedBy = nullInt64Pointer(reviewedBy)
	if reviewedAt.Valid {
		value := reviewedAt.Time.UTC()
		item.ReviewedAt = &value
	}
	item.KnowledgeMatchIDs = append([]int64(nil), knowledgeMatchIDs...)
	item.ObservedAt = item.ObservedAt.UTC()
	item.CreatedAt = item.CreatedAt.UTC()
	item.Audit = ObservationAuditContext{
		EventID: nullInt64Pointer(eventID), Username: username.String, GroupName: groupName.String,
		Model: model.String, RedactedPreview: preview.String, FullPrompt: fullPrompt.String,
		GuardDecision: decision.String, GuardRiskLevel: riskLevel.String,
	}
	return item, nil
}
