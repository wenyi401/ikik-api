package riskengine

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestShadowRepositoryPersistsObservationAndAggregatesIncident(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewShadowRepository(db)
	event := shadowRepositoryEvent()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO risk_engine_v2_observations (")).
		WithArgs(
			event.RequestID, event.UserID, event.GroupID, event.ConversationIDHash, event.InputHash,
			event.IncidentFingerprint, sqlmock.AnyArg(), sqlmock.AnyArg(), event.Outcome.Recommendation,
			true, true, event.Outcome.ReasonCode, event.Outcome.PolicyVersion,
			event.Adjudication.Model, event.Candidate.KnowledgeVersion, sqlmock.AnyArg(), event.ObservedAt.UTC(),
		).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO risk_engine_v2_incidents (")).
		WithArgs(event.IncidentFingerprint, event.UserID, event.GroupID, event.Adjudication.Category, int64(11), event.ObservedAt.UTC()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "observation_count"}).AddRow(7, 1))
	mock.ExpectCommit()

	result, err := repo.Persist(context.Background(), event)
	require.NoError(t, err)
	require.Equal(t, ShadowPersistResult{ObservationID: 11, IncidentID: 7, IncidentCount: 1, Inserted: true}, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestShadowRepositoryRetryDoesNotIncrementIncident(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	event := shadowRepositoryEvent()
	repo := NewShadowRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO risk_engine_v2_observations (")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM risk_engine_v2_observations")).
		WithArgs(event.RequestID, event.InputHash, event.Outcome.PolicyVersion).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectCommit()

	result, err := repo.Persist(context.Background(), event)
	require.NoError(t, err)
	require.Equal(t, ShadowPersistResult{ObservationID: 11}, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestShadowRepositoryRejectsAnyNonShadowEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	event := shadowRepositoryEvent()
	event.Mode = "enforce"

	_, err = NewShadowRepository(db).Persist(context.Background(), event)
	require.ErrorContains(t, err, "only accepts shadow")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoredDecisionRedactsEvidenceWithoutMutatingEvent(t *testing.T) {
	event := shadowRepositoryEvent()
	event.Adjudication.Evidence[0].Quote = "用户名：root 密码：secret-value"
	original := event.Adjudication.Evidence[0].Quote
	_, raw, err := marshalStoredDecision(event)
	require.NoError(t, err)
	require.Equal(t, original, event.Adjudication.Evidence[0].Quote)
	require.NotContains(t, string(raw), "secret-value")
	var stored Adjudication
	require.NoError(t, json.Unmarshal(raw, &stored))
	require.Contains(t, stored.Evidence[0].Quote, "[SECRET]")
}

func shadowRepositoryEvent() Event {
	text := "修改游戏内存并绕过反作弊"
	return Event{
		SchemaVersion: SchemaVersion, RequestID: "request-1", UserID: 7, GroupID: 6,
		ConversationIDHash: hashOpaque("conversation-1"), InputHash: hashOpaque(text),
		IncidentFingerprint: IncidentFingerprint(7, 6, CategoryCheatAutomation, text),
		Candidate:           Candidate{Review: true, Signals: []string{"shadow_all_traffic"}, Confidence: 1},
		Adjudication: Adjudication{
			SchemaVersion: SchemaVersion, Verdict: VerdictConfirmed, Category: CategoryCheatAutomation,
			Intent: IntentEvasion, Actionability: ActionabilityHigh, Authorization: AuthorizationUnauthorized,
			Confidence: 0.999, Evidence: []Evidence{{Quote: "修改游戏内存", Signal: "requested_action"}},
			ReasonCode: "operational_cheat_request", Model: "risk-model-test",
		},
		Outcome: Outcome{
			Recommendation: RecommendationStrike, WouldProtect: true, WouldStrike: true,
			ReasonCode: "category_strike_candidate", PolicyVersion: 1,
		},
		ObservedAt: time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC), Mode: "shadow",
	}
}
