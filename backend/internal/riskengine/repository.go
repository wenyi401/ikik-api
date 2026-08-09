package riskengine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type ShadowPersistResult struct {
	ObservationID int64 `json:"observation_id"`
	IncidentID    int64 `json:"incident_id,omitempty"`
	IncidentCount int   `json:"incident_count,omitempty"`
	Inserted      bool  `json:"inserted"`
}

type ShadowRepository struct {
	db *sql.DB
}

func NewShadowRepository(db *sql.DB) *ShadowRepository {
	return &ShadowRepository{db: db}
}

func (r *ShadowRepository) Persist(ctx context.Context, event Event) (ShadowPersistResult, error) {
	if r == nil || r.db == nil {
		return ShadowPersistResult{}, errors.New("risk engine shadow repository unavailable")
	}
	if event.Mode != "shadow" {
		return ShadowPersistResult{}, errors.New("risk engine repository only accepts shadow events")
	}
	if strings.TrimSpace(event.RequestID) == "" || !validHash(event.InputHash) || !validHash(event.IncidentFingerprint) {
		return ShadowPersistResult{}, errors.New("risk engine shadow event identity is invalid")
	}
	candidate, adjudication, err := marshalStoredDecision(event)
	if err != nil {
		return ShadowPersistResult{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ShadowPersistResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var observationID int64
	matchIDs := make([]int64, 0, len(event.Candidate.KnowledgeMatches))
	for _, match := range event.Candidate.KnowledgeMatches {
		if match.EntryID > 0 {
			matchIDs = append(matchIDs, match.EntryID)
		}
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO risk_engine_v2_observations (
			request_id, user_id, group_id, conversation_id_hash, input_hash,
			incident_fingerprint, candidate, adjudication, recommendation,
			would_protect, would_strike, reason_code, policy_version,
			adjudicator_model, knowledge_version, knowledge_match_ids, mode, observed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'shadow',$17)
		ON CONFLICT (request_id, input_hash, policy_version) DO NOTHING
		RETURNING id`,
		event.RequestID, nullablePositiveID(event.UserID), nullablePositiveID(event.GroupID),
		event.ConversationIDHash, event.InputHash, event.IncidentFingerprint,
		candidate, adjudication, event.Outcome.Recommendation, event.Outcome.WouldProtect,
		event.Outcome.WouldStrike, event.Outcome.ReasonCode, event.Outcome.PolicyVersion,
		event.Adjudication.Model, event.Candidate.KnowledgeVersion, pq.Array(matchIDs), event.ObservedAt.UTC()).Scan(&observationID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM risk_engine_v2_observations
			WHERE request_id=$1 AND input_hash=$2 AND policy_version=$3`,
			event.RequestID, event.InputHash, event.Outcome.PolicyVersion).Scan(&observationID)
		if err != nil {
			return ShadowPersistResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return ShadowPersistResult{}, err
		}
		return ShadowPersistResult{ObservationID: observationID}, nil
	}
	if err != nil {
		return ShadowPersistResult{}, err
	}

	result := ShadowPersistResult{ObservationID: observationID, Inserted: true}
	if event.Adjudication.Category != CategoryNone && event.Outcome.Recommendation != RecommendationAllow {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO risk_engine_v2_incidents (
				incident_fingerprint, user_id, group_id, category, observation_count,
				first_observation_id, latest_observation_id, first_seen_at, last_seen_at
			) VALUES ($1,$2,$3,$4,1,$5,$5,$6,$6)
			ON CONFLICT (incident_fingerprint) DO UPDATE SET
				observation_count=risk_engine_v2_incidents.observation_count+1,
				latest_observation_id=EXCLUDED.latest_observation_id,
				last_seen_at=EXCLUDED.last_seen_at,
				updated_at=NOW()
			RETURNING id, observation_count`,
			event.IncidentFingerprint, nullablePositiveID(event.UserID), nullablePositiveID(event.GroupID),
			event.Adjudication.Category, observationID, event.ObservedAt.UTC()).
			Scan(&result.IncidentID, &result.IncidentCount)
		if err != nil {
			return ShadowPersistResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ShadowPersistResult{}, err
	}
	return result, nil
}

func marshalStoredDecision(event Event) ([]byte, []byte, error) {
	candidate := event.Candidate
	candidate.Signals = append([]string(nil), event.Candidate.Signals...)
	candidate.KnowledgeMatches = append([]KnowledgeMatch(nil), event.Candidate.KnowledgeMatches...)
	if len(candidate.Signals) > 16 {
		candidate.Signals = candidate.Signals[:16]
	}
	for index := range candidate.Signals {
		candidate.Signals[index] = SanitizeTrainingText(candidate.Signals[index])
		candidate.Signals[index], _ = limitRunes(candidate.Signals[index], 128)
	}
	adjudication := event.Adjudication
	adjudication.Evidence = append([]Evidence(nil), event.Adjudication.Evidence...)
	for index := range adjudication.Evidence {
		adjudication.Evidence[index].Quote = SanitizeTrainingText(adjudication.Evidence[index].Quote)
		adjudication.Evidence[index].Quote, _ = limitRunes(adjudication.Evidence[index].Quote, 512)
		adjudication.Evidence[index].Signal = strings.TrimSpace(adjudication.Evidence[index].Signal)
	}
	candidateJSON, err := json.Marshal(candidate)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal risk candidate: %w", err)
	}
	adjudicationJSON, err := json.Marshal(adjudication)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal risk adjudication: %w", err)
	}
	return candidateJSON, adjudicationJSON, nil
}

func nullablePositiveID(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func validHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
