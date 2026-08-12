package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"ikik-api/internal/service"
)

func TestBuildContentModerationLogWhere_BlockedIncludesAllBlockActions(t *testing.T) {
	where, args := buildContentModerationLogWhere(service.ContentModerationLogFilter{Result: "blocked"})

	require.Empty(t, args)
	sql := strings.Join(where, " AND ")
	require.Contains(t, sql, "l.action IN ('block', 'keyword_block', 'hash_block', 'cyber_policy')")
	require.NotContains(t, sql, "l.action = 'block'")
}

func TestContentModerationRepositoryCountFlaggedByUserSince_ExcludesHashBlock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewContentModerationRepository(db)
	since := time.Now().Add(-time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta("AND action <> 'hash_block'")).
		WithArgs(int64(1001), since, false).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	count, err := repo.CountFlaggedByUserSince(context.Background(), 1001, since, false)

	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContentModerationRepositoryGetLogByID_ReturnsFullInput(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 24, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	rows := sqlmock.NewRows([]string{
		"id", "request_id", "user_id", "user_email", "api_key_id", "api_key_name", "group_id", "group_name",
		"endpoint", "provider", "model", "mode", "action", "flagged", "highest_category", "highest_score",
		"category_scores", "threshold_snapshot", "input_excerpt", "input_content", "upstream_latency_ms", "error",
		"violation_count", "auto_banned", "email_sent", "user_status", "queue_delay_ms", "matched_keyword", "created_at",
	}).AddRow(
		int64(42), "req-42", int64(7), "user@example.com", int64(9), "key", int64(16), "shared",
		"/v1/responses", "openai", "gpt-5.5", "adaptive", "allow", true, "gateway_abuse/other", 0.9,
		`{"gateway_abuse/other":0.9}`, `{"gateway_abuse/other":0.85}`, "summary", "complete audited input", 123, "",
		1, false, false, "active", 2, "", createdAt,
	)
	mock.ExpectQuery("FROM content_moderation_logs l").WithArgs(int64(42)).WillReturnRows(rows)
	repo := &contentModerationRepository{db: db}

	log, err := repo.GetLogByID(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, "complete audited input", log.InputContent)
	require.Equal(t, 0.9, log.CategoryScores["gateway_abuse/other"])
	require.Equal(t, "active", log.UserStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContentModerationRepositoryCreateLog_PersistsFullInput(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 24, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	mock.ExpectQuery("INSERT INTO content_moderation_logs").WithArgs(
		"req-42", nil, "user@example.com", nil, "key", nil, "shared",
		"/v1/responses", "openai", "gpt-5.5", "adaptive", "allow", true, "gateway_abuse/other", 0.9,
		`{"gateway_abuse/other":0.9}`, `{"gateway_abuse/other":0.85}`, "summary", "complete audited input", nil, "",
		0, false, false, nil, "",
	).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(42), createdAt))
	repo := &contentModerationRepository{db: db}
	log := &service.ContentModerationLog{
		RequestID:         "req-42",
		UserEmail:         "user@example.com",
		APIKeyName:        "key",
		GroupName:         "shared",
		Endpoint:          "/v1/responses",
		Provider:          "openai",
		Model:             "gpt-5.5",
		Mode:              "adaptive",
		Action:            "allow",
		Flagged:           true,
		HighestCategory:   "gateway_abuse/other",
		HighestScore:      0.9,
		CategoryScores:    map[string]float64{"gateway_abuse/other": 0.9},
		ThresholdSnapshot: map[string]float64{"gateway_abuse/other": 0.85},
		InputExcerpt:      "summary",
		InputContent:      "complete audited input",
	}

	err = repo.CreateLog(context.Background(), log)

	require.NoError(t, err)
	require.Equal(t, int64(42), log.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContentModerationRepositoryApplyUserGroupPenaltyForRisk(t *testing.T) {
	appliedAt := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	columns := []string{
		"user_id", "group_id", "strike_count", "blocked_until", "permanent",
		"last_category", "last_request_id", "last_score", "created_at", "updated_at", "applied",
	}
	tests := []struct {
		name         string
		requestID    string
		strikeCount  int
		blockedUntil any
		permanent    bool
		applied      bool
	}{
		{name: "first strike blocks 24 hours", requestID: "req-1", strikeCount: 1, blockedUntil: appliedAt.Add(24 * time.Hour), applied: true},
		{name: "second strike blocks 36 hours", requestID: "req-2", strikeCount: 2, blockedUntil: appliedAt.Add(36 * time.Hour), applied: true},
		{name: "third strike blocks permanently", requestID: "req-3", strikeCount: 3, blockedUntil: nil, permanent: true, applied: true},
		{name: "duplicate request is idempotent", requestID: "req-3", strikeCount: 3, blockedUntil: nil, permanent: true, applied: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			mock.ExpectQuery("WITH eligible AS").
				WithArgs(int64(7), int64(16), service.RoleAdmin, test.requestID,
					service.ContentModerationRiskCategoryCheatAutomation, 0.96, appliedAt, 24, 36).
				WillReturnRows(sqlmock.NewRows(columns).AddRow(
					int64(7), int64(16), test.strikeCount, test.blockedUntil, test.permanent,
					service.ContentModerationRiskCategoryCheatAutomation, test.requestID, 0.96,
					appliedAt, appliedAt, test.applied,
				))
			repo := &contentModerationRepository{db: db}

			penalty, applied, err := repo.ApplyUserGroupPenaltyForRisk(context.Background(), service.ContentModerationRiskEvent{
				RequestID: test.requestID,
				UserID:    7,
				GroupID:   16,
				Category:  service.ContentModerationRiskCategoryCheatAutomation,
				Score:     0.96,
				CreatedAt: appliedAt,
			}, 24, 36)

			require.NoError(t, err)
			require.Equal(t, test.applied, applied)
			require.NotNil(t, penalty)
			require.Equal(t, test.strikeCount, penalty.StrikeCount)
			require.Equal(t, test.permanent, penalty.Permanent)
			if test.blockedUntil == nil {
				require.Nil(t, penalty.BlockedUntil)
			} else {
				require.Equal(t, test.blockedUntil, *penalty.BlockedUntil)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
