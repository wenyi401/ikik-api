package riskengine

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestListObservationsKeywordCountUsesSelfContainedPromptEventLookup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewShadowRepository(db)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM risk_engine_v2_observations o")).
		WithArgs("%外挂%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WithArgs("%外挂%", 20, 0).WillReturnRows(sqlmock.NewRows(observationColumns()))

	page, err := repo.ListObservations(context.Background(), ObservationFilter{Keyword: "外挂"}, 1, 20)
	require.NoError(t, err)
	require.Empty(t, page.Items)
	require.Zero(t, page.Total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReviewObservationRejectsInvalidStatusBeforeDatabaseWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = NewShadowRepository(db).ReviewObservation(context.Background(), 4, ObservationReviewInput{Status: "enforce"}, 1)
	require.ErrorContains(t, err, "status is invalid")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReviewObservationRequiresDomainLabelForConfirmedReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = NewShadowRepository(db).ReviewObservation(context.Background(), 4, ObservationReviewInput{Status: ObservationConfirmed}, 1)
	require.ErrorContains(t, err, "topic is invalid")
	require.NoError(t, mock.ExpectationsWereMet())
}

func observationColumns() []string {
	return []string{
		"id", "request_id", "user_id", "group_id", "incident_fingerprint", "candidate", "adjudication",
		"recommendation", "would_protect", "would_strike", "reason_code", "policy_version",
		"adjudicator_model", "knowledge_version", "knowledge_match_ids", "mode", "review_status",
		"review_label", "reviewed_by", "reviewed_at", "review_note", "observed_at", "created_at",
		"event_id", "username_snapshot", "group_name", "model", "redacted_preview", "full_prompt", "decision", "risk_level",
	}
}
