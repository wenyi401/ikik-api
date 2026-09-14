package securityaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "ikik-api/internal/pkg/errors"
	servermiddleware "ikik-api/internal/server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"ikik-api/internal/riskengine"
)

type fakePromptAdminService struct {
	config            PublicConfig
	save              func(context.Context, UpdateConfigRequest, int64) (PublicConfig, error)
	probe             func(context.Context, ProbeRequest) ProbeResult
	testPrompt        func(context.Context, string) (*PromptAuditTestResult, error)
	runtime           RuntimeSnapshot
	list              func(context.Context, EventFilter, int, int) (*EventPage, error)
	get               func(context.Context, int64) (*Event, error)
	deleteOne         func(context.Context, int64) (*DeleteResult, error)
	deleteIDs         func(context.Context, []int64) (*DeleteResult, error)
	preview           func(context.Context, EventFilter, int64) (*DeletePreview, error)
	deleteFilter      func(context.Context, DeleteByFilterRequest, int64) (*DeleteResult, error)
	listProfiles      func(context.Context, int, int, bool, string) (*PromptAuditUserProfilePage, error)
	unblockProfile    func(context.Context, int64) (*PromptAuditUserProfile, error)
	knowledgeSummary  func(context.Context) (*riskengine.KnowledgeSummary, error)
	listKnowledge     func(context.Context, riskengine.KnowledgeFilter, int, int) (*riskengine.KnowledgeEntryPage, error)
	createKnowledge   func(context.Context, riskengine.KnowledgeWriteInput, int64) (*riskengine.KnowledgeEntry, int, error)
	updateKnowledge   func(context.Context, int64, riskengine.KnowledgeWriteInput, int64) (*riskengine.KnowledgeEntry, int, error)
	listObservations  func(context.Context, riskengine.ObservationFilter, int, int) (*riskengine.ObservationPage, error)
	reviewObservation func(context.Context, int64, riskengine.ObservationReviewInput, int64) (*riskengine.Observation, error)
}

func (s *fakePromptAdminService) GetConfig() (PublicConfig, error) {
	return s.config, nil
}
func (s *fakePromptAdminService) SaveConfig(ctx context.Context, req UpdateConfigRequest, actorID int64) (PublicConfig, error) {
	if s.save == nil {
		return PublicConfig{}, errors.New("unexpected SaveConfig call")
	}
	return s.save(ctx, req, actorID)
}
func (s *fakePromptAdminService) Probe(ctx context.Context, req ProbeRequest) ProbeResult {
	if s.probe == nil {
		return ProbeResult{}
	}
	return s.probe(ctx, req)
}
func (s *fakePromptAdminService) TestPrompt(ctx context.Context, prompt string) (*PromptAuditTestResult, error) {
	if s.testPrompt == nil {
		return nil, errors.New("unexpected TestPrompt call")
	}
	return s.testPrompt(ctx, prompt)
}
func (s *fakePromptAdminService) Runtime(context.Context) RuntimeSnapshot { return s.runtime }
func (s *fakePromptAdminService) ListEvents(ctx context.Context, filter EventFilter, page, pageSize int) (*EventPage, error) {
	if s.list == nil {
		return &EventPage{}, nil
	}
	return s.list(ctx, filter, page, pageSize)
}
func (s *fakePromptAdminService) GetEvent(ctx context.Context, id int64) (*Event, error) {
	if s.get == nil {
		return nil, ErrEventNotFound
	}
	return s.get(ctx, id)
}
func (s *fakePromptAdminService) DeleteEvent(ctx context.Context, id int64) (*DeleteResult, error) {
	if s.deleteOne == nil {
		return &DeleteResult{}, nil
	}
	return s.deleteOne(ctx, id)
}
func (s *fakePromptAdminService) DeleteEventsByIDs(ctx context.Context, ids []int64) (*DeleteResult, error) {
	if s.deleteIDs == nil {
		return &DeleteResult{}, nil
	}
	return s.deleteIDs(ctx, ids)
}
func (s *fakePromptAdminService) PreviewDelete(ctx context.Context, filter EventFilter, actorID int64) (*DeletePreview, error) {
	if s.preview == nil {
		return &DeletePreview{}, nil
	}
	return s.preview(ctx, filter, actorID)
}
func (s *fakePromptAdminService) DeleteByFilter(ctx context.Context, req DeleteByFilterRequest, actorID int64) (*DeleteResult, error) {
	if s.deleteFilter == nil {
		return &DeleteResult{}, nil
	}
	return s.deleteFilter(ctx, req, actorID)
}
func (s *fakePromptAdminService) ListPromptAuditUserProfiles(ctx context.Context, page, pageSize int, blockedOnly bool, keyword string) (*PromptAuditUserProfilePage, error) {
	if s.listProfiles == nil {
		return &PromptAuditUserProfilePage{}, nil
	}
	return s.listProfiles(ctx, page, pageSize, blockedOnly, keyword)
}
func (s *fakePromptAdminService) UnblockPromptAuditUser(ctx context.Context, userID int64) (*PromptAuditUserProfile, error) {
	if s.unblockProfile == nil {
		return nil, ErrPromptAuditProfileNotFound
	}
	return s.unblockProfile(ctx, userID)
}
func (s *fakePromptAdminService) KnowledgeSummary(ctx context.Context) (*riskengine.KnowledgeSummary, error) {
	if s.knowledgeSummary == nil {
		return &riskengine.KnowledgeSummary{}, nil
	}
	return s.knowledgeSummary(ctx)
}
func (s *fakePromptAdminService) ListKnowledge(ctx context.Context, filter riskengine.KnowledgeFilter, page, pageSize int) (*riskengine.KnowledgeEntryPage, error) {
	if s.listKnowledge == nil {
		return &riskengine.KnowledgeEntryPage{}, nil
	}
	return s.listKnowledge(ctx, filter, page, pageSize)
}
func (s *fakePromptAdminService) CreateKnowledge(ctx context.Context, input riskengine.KnowledgeWriteInput, actorID int64) (*riskengine.KnowledgeEntry, int, error) {
	if s.createKnowledge == nil {
		return &riskengine.KnowledgeEntry{}, 1, nil
	}
	return s.createKnowledge(ctx, input, actorID)
}
func (s *fakePromptAdminService) UpdateKnowledge(ctx context.Context, id int64, input riskengine.KnowledgeWriteInput, actorID int64) (*riskengine.KnowledgeEntry, int, error) {
	if s.updateKnowledge == nil {
		return &riskengine.KnowledgeEntry{ID: id}, 1, nil
	}
	return s.updateKnowledge(ctx, id, input, actorID)
}
func (s *fakePromptAdminService) ListKnowledgeObservations(ctx context.Context, filter riskengine.ObservationFilter, page, pageSize int) (*riskengine.ObservationPage, error) {
	if s.listObservations == nil {
		return &riskengine.ObservationPage{}, nil
	}
	return s.listObservations(ctx, filter, page, pageSize)
}
func (s *fakePromptAdminService) ReviewKnowledgeObservation(ctx context.Context, id int64, input riskengine.ObservationReviewInput, actorID int64) (*riskengine.Observation, error) {
	if s.reviewObservation == nil {
		return &riskengine.Observation{ID: id, ReviewStatus: input.Status}, nil
	}
	return s.reviewObservation(ctx, id, input, actorID)
}

func promptAdminRouter(service PromptAdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 42})
		c.Set(string(servermiddleware.ContextKeyUserRole), "admin")
		c.Next()
	})
	handler := NewPromptAdminHandler(service)
	group := router.Group("/admin/prompt-audit")
	group.GET("/config", handler.GetConfig)
	group.PUT("/config", handler.UpdateConfig)
	group.POST("/endpoints/probe", handler.ProbeEndpoint)
	group.POST("/test", handler.TestPrompt)
	group.GET("/runtime", handler.GetRuntime)
	group.GET("/knowledge/summary", handler.GetKnowledgeSummary)
	group.GET("/knowledge/observations", handler.ListKnowledgeObservations)
	group.PUT("/knowledge/observations/:id/review", handler.ReviewKnowledgeObservation)
	group.GET("/knowledge", handler.ListKnowledge)
	group.POST("/knowledge", handler.CreateKnowledge)
	group.PUT("/knowledge/:id", handler.UpdateKnowledge)
	group.GET("/events", handler.ListEvents)
	group.GET("/profiles", handler.ListProfiles)
	group.POST("/profiles/:user_id/unblock", handler.UnblockProfile)
	group.GET("/events/:id", handler.GetEvent)
	group.DELETE("/events/:id", handler.DeleteEvent)
	group.POST("/events/batch-delete", handler.BatchDelete)
	group.POST("/events/delete-preview", handler.DeletePreview)
	group.POST("/events/delete-by-filter", handler.DeleteByFilter)
	return router
}

func promptAdminRequest(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestPromptAdminProfilesListAndUnblock(t *testing.T) {
	t.Run("list forwards filters", func(t *testing.T) {
		service := &fakePromptAdminService{listProfiles: func(_ context.Context, page, pageSize int, blockedOnly bool, keyword string) (*PromptAuditUserProfilePage, error) {
			require.Equal(t, 2, page)
			require.Equal(t, 10, pageSize)
			require.True(t, blockedOnly)
			require.Equal(t, "user@example.com", keyword)
			return &PromptAuditUserProfilePage{Items: []PromptAuditUserProfile{{UserID: 7, Blocked: true}}, Total: 1, Page: 2, PageSize: 10, Pages: 1}, nil
		}}
		response := promptAdminRequest(t, promptAdminRouter(service), http.MethodGet, "/admin/prompt-audit/profiles?page=2&page_size=10&blocked_only=true&keyword=user%40example.com", nil)
		require.Equal(t, http.StatusOK, response.Code)
		require.Contains(t, response.Body.String(), `"user_id":7`)
	})

	t.Run("unblock validates and returns profile", func(t *testing.T) {
		service := &fakePromptAdminService{unblockProfile: func(_ context.Context, userID int64) (*PromptAuditUserProfile, error) {
			require.Equal(t, int64(7), userID)
			return &PromptAuditUserProfile{UserID: userID, Blocked: false}, nil
		}}
		response := promptAdminRequest(t, promptAdminRouter(service), http.MethodPost, "/admin/prompt-audit/profiles/7/unblock", nil)
		require.Equal(t, http.StatusOK, response.Code)
		require.Contains(t, response.Body.String(), `"blocked":false`)
	})
}

func TestPromptAdminConfigRequiresVersionMapsConflictAndNeverEchoesToken(t *testing.T) {
	const canary = "prompt-admin-token-canary"

	t.Run("missing expected version", func(t *testing.T) {
		router := promptAdminRouter(&fakePromptAdminService{})
		response := promptAdminRequest(t, router, http.MethodPut, "/admin/prompt-audit/config", map[string]any{})
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Contains(t, response.Body.String(), "prompt_audit_invalid_config_request")
	})

	t.Run("CAS conflict", func(t *testing.T) {
		service := &fakePromptAdminService{save: func(context.Context, UpdateConfigRequest, int64) (PublicConfig, error) {
			return PublicConfig{}, infraerrors.Conflict(ErrorCodeConfigConflict, "配置已被更新")
		}}
		response := promptAdminRequest(t, promptAdminRouter(service), http.MethodPut, "/admin/prompt-audit/config", validHandlerUpdateRequest(canary))
		require.Equal(t, http.StatusConflict, response.Code)
		require.Contains(t, response.Body.String(), ErrorCodeConfigConflict)
		require.NotContains(t, response.Body.String(), canary)
	})

	t.Run("success public DTO", func(t *testing.T) {
		service := &fakePromptAdminService{save: func(_ context.Context, req UpdateConfigRequest, actorID int64) (PublicConfig, error) {
			require.Equal(t, int64(42), actorID)
			require.Equal(t, canary, req.Endpoints[0].Token)
			return PublicConfig{ConfigVersion: 8, Endpoints: []PublicEndpoint{{ID: "guard-1", HasToken: true, TokenStatus: "configured"}}}, nil
		}}
		response := promptAdminRequest(t, promptAdminRouter(service), http.MethodPut, "/admin/prompt-audit/config", validHandlerUpdateRequest(canary))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		require.NotContains(t, body, canary)
		require.NotContains(t, body, "token_ciphertext")
		require.NotContains(t, body, `"token":`)
		require.Contains(t, body, `"has_token":true`)
	})
}

func TestPromptAdminGetConfigReturnsSecretFreeUnavailableError(t *testing.T) {
	const canary = "persisted-config-secret-canary"
	repository := &switchableSettingRepository{loadErr: errors.New("failed to load token " + canary)}
	manager := NewConfigManager(nil, repository, nil, prefixEncryptor{}, testTotpKeyConfig())
	require.Error(t, manager.Reload(context.Background()))
	service := &PromptService{config: manager}

	response := promptAdminRequest(t, promptAdminRouter(service), http.MethodGet, "/admin/prompt-audit/config", nil)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Contains(t, response.Body.String(), ErrorCodeConfigUnavailable)
	require.NotContains(t, response.Body.String(), canary)
	require.NotContains(t, response.Body.String(), `"config_version"`)
	require.NotContains(t, response.Body.String(), `"token"`)
}

func TestPromptAdminProbeSupportsTemporaryOrSavedTokenWithoutEcho(t *testing.T) {
	const canary = "probe-token-canary"
	for _, tc := range []struct {
		name         string
		token        string
		tokenApplied bool
	}{
		{name: "temporary token", token: canary, tokenApplied: true},
		{name: "saved token", token: "", tokenApplied: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakePromptAdminService{probe: func(_ context.Context, req ProbeRequest) ProbeResult {
				require.Equal(t, tc.token, req.Endpoint.Token)
				return ProbeResult{OK: true, Status: "healthy", Message: "ok", TokenApplied: tc.tokenApplied}
			}}
			endpoint := validHandlerUpdateRequest(tc.token).Endpoints[0]
			response := promptAdminRequest(t, promptAdminRouter(service), http.MethodPost, "/admin/prompt-audit/endpoints/probe", ProbeRequest{Endpoint: endpoint})
			require.Equal(t, http.StatusOK, response.Code)
			require.NotContains(t, response.Body.String(), canary)
			require.NotContains(t, response.Body.String(), `"token":`)
			require.Contains(t, response.Body.String(), `"token_applied":true`)
		})
	}
}

func TestPromptAdminTestPromptValidatesAndDoesNotEchoInput(t *testing.T) {
	const prompt = "测试外挂识别，不要把原文写入响应"
	service := &fakePromptAdminService{testPrompt: func(_ context.Context, got string) (*PromptAuditTestResult, error) {
		require.Equal(t, prompt, got)
		return &PromptAuditTestResult{
			Result: &NormalizedResult{
				Decision: EventCritical, RiskLevel: RiskCritical, Action: ActionBlock,
				Categories: []string{"cheat_automation"}, Shadow: true,
			},
			ChunkTotal: 1,
			LatencyMS:  5,
		}, nil
	}}
	response := promptAdminRequest(t, promptAdminRouter(service), http.MethodPost, "/admin/prompt-audit/test", PromptAuditTestRequest{Prompt: prompt})
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"decision":"critical"`)
	require.Contains(t, response.Body.String(), `"categories":["cheat_automation"]`)
	require.NotContains(t, response.Body.String(), prompt)

	response = promptAdminRequest(t, promptAdminRouter(service), http.MethodPost, "/admin/prompt-audit/test", PromptAuditTestRequest{Prompt: "  "})
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "prompt_audit_test_prompt_required")
}

func TestPromptAdminRejectsInvalidEventIDsTimesAndPagination(t *testing.T) {
	router := promptAdminRouter(&fakePromptAdminService{})
	for _, tc := range []struct {
		method string
		path   string
		body   any
		reason string
	}{
		{http.MethodGet, "/admin/prompt-audit/events/not-a-number", nil, "prompt_audit_invalid_event_id"},
		{http.MethodDelete, "/admin/prompt-audit/events/-1", nil, "prompt_audit_invalid_event_id"},
		{http.MethodGet, "/admin/prompt-audit/events?group_id=bad", nil, "prompt_audit_invalid_filter_id"},
		{http.MethodGet, "/admin/prompt-audit/events?start_at=not-time", nil, "prompt_audit_invalid_time"},
		{http.MethodGet, "/admin/prompt-audit/events?page=0", nil, "prompt_audit_invalid_pagination"},
		{http.MethodPost, "/admin/prompt-audit/events/batch-delete", map[string]any{"ids": []int64{1, -2}}, "prompt_audit_invalid_event_id"},
	} {
		response := promptAdminRequest(t, router, tc.method, tc.path, tc.body)
		require.Equalf(t, http.StatusBadRequest, response.Code, "%s %s", tc.method, tc.path)
		require.Contains(t, response.Body.String(), tc.reason)
	}
}

func validHandlerUpdateRequest(token string) UpdateConfigRequest {
	return UpdateConfigRequest{
		ExpectedConfigVersion: 7,
		Strategy:              "priority",
		WorkerCount:           1,
		QueueCapacity:         10,
		Scanners:              []string{"pii"},
		AllGroups:             true,
		Endpoints: []UpdateEndpoint{{
			ID: "guard-1", Name: "Guard One", Protocol: "openai_compatible",
			BaseURL: "http://127.0.0.1:18080", Model: DefaultGuardModel, Token: token,
			TimeoutMS: 1000, InputLimit: 1024, Enabled: true,
		}},
	}
}

func TestPromptAdminDeleteConfirmationErrorsStayGeneric(t *testing.T) {
	service := &fakePromptAdminService{deleteFilter: func(context.Context, DeleteByFilterRequest, int64) (*DeleteResult, error) {
		return nil, errors.New("sensitive-token-or-filter-detail")
	}}
	response := promptAdminRequest(t, promptAdminRouter(service), http.MethodPost, "/admin/prompt-audit/events/delete-by-filter", DeleteByFilterRequest{
		SnapshotMaxID: 3, FilterHash: strings.Repeat("a", 64), ConfirmationToken: "secret-confirmation", Confirm: true,
	})
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "prompt_audit_delete_confirmation_invalid")
	require.NotContains(t, response.Body.String(), "sensitive-token")
	require.NotContains(t, response.Body.String(), "secret-confirmation")
}

func TestPromptKnowledgeAdminWritesCasesAndShadowReviewLabelsOnly(t *testing.T) {
	created := false
	reviewed := false
	service := &fakePromptAdminService{
		createKnowledge: func(_ context.Context, input riskengine.KnowledgeWriteInput, actorID int64) (*riskengine.KnowledgeEntry, int, error) {
			require.EqualValues(t, 42, actorID)
			require.Equal(t, riskengine.TopicCheatDevelopment, input.Topic)
			require.Equal(t, riskengine.KnowledgeRisk, input.Disposition)
			created = true
			return &riskengine.KnowledgeEntry{ID: 12, Title: input.Title}, 4, nil
		},
		reviewObservation: func(_ context.Context, id int64, input riskengine.ObservationReviewInput, actorID int64) (*riskengine.Observation, error) {
			require.EqualValues(t, 9, id)
			require.EqualValues(t, 42, actorID)
			require.Equal(t, riskengine.ObservationConfirmed, input.Status)
			require.Equal(t, riskengine.TopicCheatDevelopment, input.Topic)
			require.Equal(t, riskengine.CategoryCheatAutomation, input.Category)
			reviewed = true
			return &riskengine.Observation{ID: id, Mode: "shadow", ReviewStatus: input.Status}, nil
		},
	}
	router := promptAdminRouter(service)
	entry := riskengine.KnowledgeWriteInput{
		Topic: riskengine.TopicCheatDevelopment, Category: riskengine.CategoryCheatAutomation,
		Disposition: riskengine.KnowledgeRisk, Intent: riskengine.IntentOperational,
		Actionability: riskengine.ActionabilityHigh, Authorization: riskengine.AuthorizationUnknown,
		Language: "zh", Title: "外挂开发", ExampleText: "读取游戏内存并自动瞄准", Enabled: true,
	}
	response := promptAdminRequest(t, router, http.MethodPost, "/admin/prompt-audit/knowledge", entry)
	require.Equal(t, http.StatusOK, response.Code)
	require.True(t, created)
	require.Contains(t, response.Body.String(), `"version":4`)

	review := riskengine.ObservationReviewInput{
		Status: riskengine.ObservationConfirmed, Topic: riskengine.TopicCheatDevelopment,
		Category: riskengine.CategoryCheatAutomation, Note: "人工确认",
	}
	response = promptAdminRequest(t, router, http.MethodPut, "/admin/prompt-audit/knowledge/observations/9/review", review)
	require.Equal(t, http.StatusOK, response.Code)
	require.True(t, reviewed)
	require.Contains(t, response.Body.String(), `"mode":"shadow"`)
	require.NotContains(t, response.Body.String(), "blocked")
	require.NotContains(t, response.Body.String(), "penalty")
}

func TestPromptKnowledgeAdminParsesObservationFilters(t *testing.T) {
	service := &fakePromptAdminService{listObservations: func(_ context.Context, filter riskengine.ObservationFilter, page, pageSize int) (*riskengine.ObservationPage, error) {
		require.Equal(t, riskengine.ObservationUnreviewed, filter.ReviewStatus)
		require.Equal(t, riskengine.CategoryCheatAutomation, filter.Category)
		require.EqualValues(t, 7, *filter.UserID)
		require.Equal(t, "外挂", filter.Keyword)
		require.Equal(t, 2, page)
		require.Equal(t, 10, pageSize)
		return &riskengine.ObservationPage{Page: page, PageSize: pageSize}, nil
	}}
	response := promptAdminRequest(t, promptAdminRouter(service), http.MethodGet,
		"/admin/prompt-audit/knowledge/observations?review_status=unreviewed&category=cheat_automation&user_id=7&keyword=%E5%A4%96%E6%8C%82&page=2&page_size=10", nil)
	require.Equal(t, http.StatusOK, response.Code)
}
