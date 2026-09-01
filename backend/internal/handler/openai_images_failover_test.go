//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"ikik-api/internal/config"
	"ikik-api/internal/pkg/ctxkey"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

type openAIImagesFailoverAccountRepo struct {
	service.AccountRepository
	accounts        []service.Account
	accountsByGroup map[int64][]service.Account
}

func (r openAIImagesFailoverAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			account := r.accounts[i]
			return &account, nil
		}
	}
	return nil, service.ErrNoAvailableAccounts
}

func (r openAIImagesFailoverAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]service.Account, error) {
	accounts := r.accounts
	if r.accountsByGroup != nil {
		accounts = r.accountsByGroup[groupID]
	}
	return accountsForOpenAIImagesPlatform(accounts, platform), nil
}

func (r openAIImagesFailoverAccountRepo) ListSchedulableByPlatform(ctx context.Context, platform string) ([]service.Account, error) {
	if r.accountsByGroup != nil {
		if group, _ := ctx.Value(ctxkey.Group).(*service.Group); group != nil {
			return accountsForOpenAIImagesPlatform(r.accountsByGroup[group.ID], platform), nil
		}
	}
	return r.accountsForPlatform(platform), nil
}

func (r openAIImagesFailoverAccountRepo) ListSchedulableUngroupedByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return r.accountsForPlatform(platform), nil
}

func (r openAIImagesFailoverAccountRepo) accountsForPlatform(platform string) []service.Account {
	return accountsForOpenAIImagesPlatform(r.accounts, platform)
}

func accountsForOpenAIImagesPlatform(accounts []service.Account, platform string) []service.Account {
	out := make([]service.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Platform == platform {
			out = append(out, account)
		}
	}
	return out
}

type openAIImagesRouteHTTPUpstream struct {
	service.HTTPUpstream
	mu            sync.Mutex
	accounts      []int64
	failAccountID int64
}

func (u *openAIImagesRouteHTTPUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.accounts = append(u.accounts, accountID)
	u.mu.Unlock()
	if accountID == u.failAccountID {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"req_img_group_route_503"},
			},
			Body: io.NopCloser(bytes.NewBufferString(`{"error":{"type":"server_error","message":"image backend temporarily unavailable"}}`)),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"req_img_group_route"},
		},
		Body: io.NopCloser(bytes.NewBufferString(`{"created":1710000007,"data":[{"b64_json":"aGVsbG8="}]}`)),
	}, nil
}

func (u *openAIImagesRouteHTTPUpstream) calls() []int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.accounts...)
}

func newOpenAIImagesFailoverTestHandler(
	t *testing.T,
	accountRepo service.AccountRepository,
	upstream service.HTTPUpstream,
) *OpenAIGatewayHandler {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	gatewayService := service.NewOpenAIGatewayService(
		accountRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
		nil,
		nil,
		nil,
		nil,
		upstream,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	billingService := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingService.Stop)
	handler := NewOpenAIGatewayHandler(
		gatewayService,
		service.NewConcurrencyService(nil),
		billingService,
		service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg),
		nil,
		nil,
		nil,
		nil,
		cfg,
	)
	handler.maxAccountSwitches = 10
	return handler
}

type openAIImagesFailoverHTTPUpstream struct {
	service.HTTPUpstream
	mu         sync.Mutex
	accountIDs []int64
}

func (u *openAIImagesFailoverHTTPUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.accountIDs = append(u.accountIDs, accountID)
	u.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"X-Request-Id": []string{"req_img_failover"},
		},
		Body: io.NopCloser(bytes.NewBufferString(
			"data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"image backend unavailable\"}}\n\n",
		)),
	}, nil
}

func (u *openAIImagesFailoverHTTPUpstream) calls() []int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.accountIDs...)
}

func TestOpenAIGatewayHandlerImages_ServerErrorFailsOverAndReturnsClearErrorWhenExhausted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(3130)
	accounts := []service.Account{
		{
			ID:          1,
			Name:        "image-account-1",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Status:      service.StatusActive,
			Schedulable: true,
			Concurrency: 0,
			Priority:    0,
			Credentials: map[string]any{"access_token": "token-1"},
		},
		{
			ID:          2,
			Name:        "image-account-2",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Status:      service.StatusActive,
			Schedulable: true,
			Concurrency: 0,
			Priority:    1,
			Credentials: map[string]any{"access_token": "token-2"},
		},
	}
	accountRepo := openAIImagesFailoverAccountRepo{accounts: accounts}
	upstream := &openAIImagesFailoverHTTPUpstream{}
	handler := newOpenAIImagesFailoverTestHandler(t, accountRepo, upstream)

	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","quality":"high","size":"1536x1024"}`)
	core, observedLogs := observer.New(zap.DebugLevel)
	requestCtx := logger.IntoContext(context.Background(), zap.New(core))
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body)).WithContext(requestCtx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      99,
		GroupID: &groupID,
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformOpenAI,
			AllowImageGeneration: true,
		},
		User: &service.User{ID: 100},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 100, Concurrency: 0})

	handler.Images(c)

	accountSelectingLogs := observedLogs.FilterMessage("openai.images.account_selecting").All()
	require.NotEmpty(t, accountSelectingLogs)
	loggedFields := make(map[string]string)
	for _, field := range accountSelectingLogs[0].Context {
		loggedFields[field.Key] = field.String
	}
	require.Equal(t, "high", loggedFields["img_quality"])
	require.Equal(t, "1536x1024", loggedFields["img_size"])
	require.NotContains(t, loggedFields, "prompt")

	require.Equal(t, []int64{1, 2}, upstream.calls())
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Equal(t, "upstream_error", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
	require.Equal(t, "Upstream service temporarily unavailable", gjson.GetBytes(rec.Body.Bytes(), "error.message").String())

	rawEvents, ok := c.Get(service.OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*service.OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 2)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, "failover", events[1].Kind)
}

func TestOpenAIGatewayHandlerImages_GroupRoutesSkipDisabledAndNeverCrossGrok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	disabledGroup := &service.Group{ID: 3211, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
	grokGroup := &service.Group{ID: 3212, Platform: service.PlatformGrok, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true}
	enabledGroup := &service.Group{ID: 3213, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true}
	openAIAccount := service.Account{
		ID:          31,
		Name:        "image-route-openai",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"api_key": "openai-token", "base_url": "https://images.example/v1"},
	}
	grokAccount := service.Account{
		ID:          32,
		Name:        "image-route-grok",
		Platform:    service.PlatformGrok,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"api_key": "grok-token", "base_url": "https://grok.example/v1"},
	}
	accountRepo := openAIImagesFailoverAccountRepo{
		accounts: []service.Account{openAIAccount, grokAccount},
		accountsByGroup: map[int64][]service.Account{
			enabledGroup.ID: {openAIAccount},
			grokGroup.ID:    {grokAccount},
		},
	}
	upstream := &openAIImagesRouteHTTPUpstream{}
	handler := newOpenAIImagesFailoverTestHandler(t, accountRepo, upstream)

	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      3201,
		GroupID: &disabledGroup.ID,
		Group:   disabledGroup,
		GroupRoutes: []service.APIKeyGroupRoute{
			{GroupID: disabledGroup.ID, Priority: 100, Weight: 1, Enabled: true, Group: disabledGroup},
			{GroupID: grokGroup.ID, Priority: 200, Weight: 1, Enabled: true, Group: grokGroup},
			{GroupID: enabledGroup.ID, Priority: 300, Weight: 1, Enabled: true, Group: enabledGroup},
		},
		User: &service.User{ID: 100},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 100})

	handler.Images(c)

	require.Equal(t, []int64{openAIAccount.ID}, upstream.calls())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "aGVsbG8=", gjson.GetBytes(rec.Body.Bytes(), "data.0.b64_json").String())
}

func TestOpenAIGatewayHandlerImages_GroupRoutesFailOverWhenFirstGroupHasNoAccounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	firstGroup := &service.Group{ID: 3221, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true}
	secondGroup := &service.Group{ID: 3222, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true}
	account := service.Account{
		ID:          41,
		Name:        "image-route-fallback",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"api_key": "fallback-token", "base_url": "https://images.example/v1"},
	}
	accountRepo := openAIImagesFailoverAccountRepo{
		accounts: []service.Account{account},
		accountsByGroup: map[int64][]service.Account{
			secondGroup.ID: {account},
		},
	}
	upstream := &openAIImagesRouteHTTPUpstream{}
	handler := newOpenAIImagesFailoverTestHandler(t, accountRepo, upstream)

	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      3202,
		GroupID: &firstGroup.ID,
		Group:   firstGroup,
		GroupRoutes: []service.APIKeyGroupRoute{
			{GroupID: firstGroup.ID, Priority: 100, Weight: 1, Enabled: true, Group: firstGroup},
			{GroupID: secondGroup.ID, Priority: 200, Weight: 1, Enabled: true, Group: secondGroup},
		},
		User: &service.User{ID: 100},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 100})

	handler.Images(c)

	require.Equal(t, []int64{account.ID}, upstream.calls())
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestOpenAIGatewayHandlerImages_GroupRoutesFailOverAfterFirstGroupUpstream503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	firstGroup := &service.Group{ID: 3231, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true}
	secondGroup := &service.Group{ID: 3232, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true}
	firstAccount := service.Account{
		ID:          51,
		Name:        "image-route-upstream-503",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"api_key": "first-token", "base_url": "https://first-images.example/v1"},
	}
	secondAccount := service.Account{
		ID:          52,
		Name:        "image-route-healthy",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"api_key": "second-token", "base_url": "https://second-images.example/v1"},
	}
	accountRepo := openAIImagesFailoverAccountRepo{
		accounts: []service.Account{firstAccount, secondAccount},
		accountsByGroup: map[int64][]service.Account{
			firstGroup.ID:  {firstAccount},
			secondGroup.ID: {secondAccount},
		},
	}
	upstream := &openAIImagesRouteHTTPUpstream{failAccountID: firstAccount.ID}
	handler := newOpenAIImagesFailoverTestHandler(t, accountRepo, upstream)
	handler.maxAccountSwitches = 0

	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      3203,
		GroupID: &firstGroup.ID,
		Group:   firstGroup,
		GroupRoutes: []service.APIKeyGroupRoute{
			{GroupID: firstGroup.ID, Priority: 100, Weight: 1, Enabled: true, Group: firstGroup},
			{GroupID: secondGroup.ID, Priority: 200, Weight: 1, Enabled: true, Group: secondGroup},
		},
		User: &service.User{ID: 100},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 100})

	handler.Images(c)

	require.Equal(t, []int64{firstAccount.ID, secondAccount.ID}, upstream.calls())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "aGVsbG8=", gjson.GetBytes(rec.Body.Bytes(), "data.0.b64_json").String())
}

func TestCanSwitchOpenAIImagesGroupRouteRequiresUnstartedUnwrittenResponse(t *testing.T) {
	apiKeyGroupRouteBreaker = newAPIKeyGroupRouteCircuitBreaker()
	apiKey := routeCursorTestAPIKey()
	failoverErr := &service.UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable}

	newContext := func() (*gin.Context, *httptest.ResponseRecorder) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
		return ctx, recorder
	}

	ctx, _ := newContext()
	writerSizeBeforeForward := service.OpenAIImagesJSONKeepaliveAdjustedWrittenSize(ctx)
	require.True(t, canSwitchOpenAIImagesGroupRoute(ctx, newAPIKeyGroupRouteCursor(apiKey), failoverErr, false, writerSizeBeforeForward))
	require.False(t, canSwitchOpenAIImagesGroupRoute(ctx, newAPIKeyGroupRouteCursor(apiKey), failoverErr, true, writerSizeBeforeForward))

	ctx, _ = newContext()
	writerSizeBeforeForward = service.OpenAIImagesJSONKeepaliveAdjustedWrittenSize(ctx)
	_, err := ctx.Writer.Write([]byte(`{"error":"already written"}`))
	require.NoError(t, err)
	require.False(t, canSwitchOpenAIImagesGroupRoute(ctx, newAPIKeyGroupRouteCursor(apiKey), failoverErr, false, writerSizeBeforeForward))
}
