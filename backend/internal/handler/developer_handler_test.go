package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type developerHandlerTokenRepoStub struct {
	token   *service.DeveloperToken
	getErr  error
	touched int64
}

func (s *developerHandlerTokenRepoStub) Create(context.Context, *service.DeveloperToken) error {
	panic("unexpected Create call")
}

func (s *developerHandlerTokenRepoStub) ListByUserID(context.Context, int64) ([]service.DeveloperToken, error) {
	panic("unexpected ListByUserID call")
}

func (s *developerHandlerTokenRepoStub) CountByUserID(context.Context, int64) (int, error) {
	panic("unexpected CountByUserID call")
}

func (s *developerHandlerTokenRepoStub) GetByTokenHash(context.Context, string) (*service.DeveloperToken, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.token == nil {
		return nil, service.ErrDeveloperTokenNotFound
	}
	copy := *s.token
	return &copy, nil
}

func (s *developerHandlerTokenRepoStub) DeleteOwned(context.Context, int64, int64) error {
	panic("unexpected DeleteOwned call")
}

func (s *developerHandlerTokenRepoStub) Touch(_ context.Context, tokenID int64, _ time.Time, _ string) error {
	s.touched = tokenID
	return nil
}

func TestDeveloperAuthenticationMiddlewareSetsUserAndEnforcesScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &developerHandlerTokenRepoStub{token: &service.DeveloperToken{
		ID:     31,
		Status: service.StatusActive,
		Scopes: []string{service.DeveloperScopeAccountsRead},
		User: &service.User{
			ID: 9, Role: service.RoleUser, Status: service.StatusActive,
			DeveloperAPIEnabled: true, Concurrency: 3,
		},
	}}
	h := NewDeveloperHandler(service.NewDeveloperTokenService(repo, nil), nil)
	router := gin.New()
	router.GET("/allowed", h.Authenticate(), h.RequireScope(service.DeveloperScopeAccountsRead), func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(9), subject.UserID)
		c.Status(http.StatusNoContent)
	})
	router.GET("/denied", h.Authenticate(), h.RequireScope(service.DeveloperScopeAccountsWrite), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	allowed := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodGet, "/allowed", nil)
	allowedRequest.Header.Set("Authorization", "Bearer "+service.DeveloperTokenPrefix+"secret")
	router.ServeHTTP(allowed, allowedRequest)
	require.Equal(t, http.StatusNoContent, allowed.Code)
	require.Equal(t, int64(31), repo.touched)

	denied := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodGet, "/denied", nil)
	deniedRequest.Header.Set("Authorization", "Bearer "+service.DeveloperTokenPrefix+"secret")
	router.ServeHTTP(denied, deniedRequest)
	require.Equal(t, http.StatusForbidden, denied.Code)
	require.Contains(t, denied.Body.String(), "DEVELOPER_TOKEN_SCOPE_REQUIRED")
}

func TestDeveloperAuthenticationMiddlewareRejectsMissingBearerAndHidesRepositoryError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing bearer", func(t *testing.T) {
		h := NewDeveloperHandler(service.NewDeveloperTokenService(&developerHandlerTokenRepoStub{}, nil), nil)
		router := gin.New()
		router.GET("/health", h.Authenticate(), h.Health)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		require.Contains(t, recorder.Body.String(), "INVALID_DEVELOPER_TOKEN")
	})

	t.Run("repository outage", func(t *testing.T) {
		repo := &developerHandlerTokenRepoStub{getErr: errors.New("postgres password=do-not-leak")}
		h := NewDeveloperHandler(service.NewDeveloperTokenService(repo, nil), nil)
		router := gin.New()
		router.GET("/health", h.Authenticate(), h.Health)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/health", nil)
		request.Header.Set("Authorization", "Bearer "+service.DeveloperTokenPrefix+"secret")
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusInternalServerError, recorder.Code)
		require.NotContains(t, recorder.Body.String(), "do-not-leak")
	})
}

func TestDeveloperAccountImportValidationRejectsServerManagedFields(t *testing.T) {
	for _, field := range []string{
		"owner_user_id", "account_level", "group_ids", "share_status", "share_policy_id",
		"rate_multiplier", "concurrency", "priority",
	} {
		t.Run(field, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"platform":    "openai",
				"type":        "oauth",
				"credentials": map[string]any{"access_token": "secret"},
				field:         true,
			})
			require.NoError(t, err)
			_, err = validateDeveloperAccountImportItem(raw)
			require.Error(t, err)
			require.Equal(t, "ACCOUNT_IMPORT_FIELD_NOT_ALLOWED", developerImportErrorCode(err))
		})
	}
}

func TestDeveloperAccountImportValidationRequiresStructuredOAuthCredentials(t *testing.T) {
	tests := []struct {
		name string
		item map[string]any
	}{
		{
			name: "missing credentials",
			item: map[string]any{
				"platform": "openai",
				"type":     "oauth",
			},
		},
		{
			name: "manual session credential",
			item: map[string]any{
				"platform":    "anthropic",
				"type":        "oauth",
				"session_key": "sk-ant-sid-example",
			},
		},
		{
			name: "non-object credentials",
			item: map[string]any{
				"platform":    "openai",
				"type":        "oauth",
				"credentials": "refresh-token",
			},
		},
		{
			name: "non-string external id",
			item: map[string]any{
				"external_id": 7,
				"platform":    "openai",
				"type":        "oauth",
				"credentials": map[string]any{"access_token": "secret"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal(test.item)
			require.NoError(t, err)
			_, err = validateDeveloperAccountImportItem(raw)
			require.Error(t, err)
		})
	}
}

func TestDeveloperAccountImportRequiresIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewDeveloperHandler(nil, nil)
	router := gin.New()
	router.POST("/imports", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 19})
		h.ImportAccounts(c)
	})
	body := []byte(`{"accounts":[{"name":"test","platform":"openai","type":"oauth","credentials":{"access_token":"secret"}}]}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/imports", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "IDEMPOTENCY_KEY_REQUIRED")
}

func TestDeveloperPublicImportCreatesShareReviewTask(t *testing.T) {
	ownerID := int64(23)
	repo := newUserAccountImportRepoStub()
	accountService := service.NewAccountService(repo, nil, nil, nil)
	accountService.SetUserPrivateGroupProvisioner(&userAccountImportPrivateGroupProvisioner{
		group: service.Group{ID: 99, Status: service.StatusActive, Scope: service.GroupScopeUserPrivate},
	})
	batchService := service.NewAccountBatchTaskService(repo, nil)
	accountHandler := NewUserAccountHandler(accountService, nil, nil, nil, nil, nil, nil, batchService)
	h := NewDeveloperHandler(nil, accountHandler)

	result, err := h.importAccounts(context.Background(), ownerID, developerAccountImportRequest{
		ShareMode: service.AccountShareModePublic,
		Accounts: []json.RawMessage{json.RawMessage(`{
			"external_id":"source-1",
			"name":"Developer OpenAI",
			"platform":"openai",
			"type":"oauth",
			"credentials":{"access_token":"secret-access-token"}
		}`)},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Zero(t, result.Failed)
	require.NotNil(t, result.ShareTask)
	require.Equal(t, service.AccountBatchTaskOperationUserSetPublicShare, repo.createdTask.Operation)
	require.Equal(t, ownerID, repo.createdTask.CreatedBy)
	require.Equal(t, []int64{1}, repo.createdTask.AccountIDs)
	require.Len(t, repo.created, 1)
	require.Equal(t, service.AccountShareModePrivate, repo.created[0].ShareMode)
}

func TestDeveloperImportErrorMessageDoesNotExposeUnknownErrors(t *testing.T) {
	require.Equal(t, "account import failed", developerImportErrorMessage(errors.New("password=secret")))
}
