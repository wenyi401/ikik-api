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
	"time"

	"ikik-api/internal/config"
	"ikik-api/internal/pkg/tlsfingerprint"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type grokImageRouteAccountRepo struct {
	service.AccountRepository
	mu              sync.Mutex
	accountsByGroup map[int64][]service.Account
	queriedGroups   []int64
}

func (r *grokImageRouteAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for _, accounts := range r.accountsByGroup {
		for i := range accounts {
			if accounts[i].ID == id {
				account := accounts[i]
				return &account, nil
			}
		}
	}
	return nil, service.ErrNoAvailableAccounts
}

func (r *grokImageRouteAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]service.Account, error) {
	return r.accountsForGroup(groupID, platform), nil
}

func (r *grokImageRouteAccountRepo) ListSchedulableByPlatform(ctx context.Context, platform string) ([]service.Account, error) {
	group := service.GroupFromContext(ctx)
	if group == nil {
		return nil, nil
	}
	return r.accountsForGroup(group.ID, platform), nil
}

func (r *grokImageRouteAccountRepo) accountsForGroup(groupID int64, platform string) []service.Account {
	r.mu.Lock()
	r.queriedGroups = append(r.queriedGroups, groupID)
	r.mu.Unlock()

	accounts := r.accountsByGroup[groupID]
	result := make([]service.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Platform == platform {
			result = append(result, account)
		}
	}
	return result
}

func (r *grokImageRouteAccountRepo) groups() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.queriedGroups...)
}

type grokImageRouteHTTPUpstream struct {
	mu         sync.Mutex
	accountIDs []int64
}

func (u *grokImageRouteHTTPUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.accountIDs = append(u.accountIDs, accountID)
	u.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"grok-route-success"},
		},
		Body: io.NopCloser(bytes.NewBufferString(`{"data":[{"url":"https://images.test/route.png"}]}`)),
	}, nil
}

func (u *grokImageRouteHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func (u *grokImageRouteHTTPUpstream) calls() []int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.accountIDs...)
}

func TestGrokImagesGroupRoutesSkipsNonGrokAndFailsOverToNextGrokGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyGroupRouteBreaker = newAPIKeyGroupRouteCircuitBreaker()

	const (
		firstGrokGroupID  int64 = 8101
		openAIGroupID     int64 = 8102
		secondGrokGroupID int64 = 8103
		accountID         int64 = 8201
	)
	group := func(id int64, platform string, allowImages bool) *service.Group {
		return &service.Group{
			ID:                   id,
			Platform:             platform,
			Status:               service.StatusActive,
			Hydrated:             true,
			AllowImageGeneration: allowImages,
		}
	}
	firstGrokGroup := group(firstGrokGroupID, service.PlatformGrok, true)
	openAIGroup := group(openAIGroupID, service.PlatformOpenAI, true)
	secondGrokGroup := group(secondGrokGroupID, service.PlatformGrok, true)
	user := &service.User{ID: 8301}
	apiKey := &service.APIKey{
		ID:     8401,
		UserID: user.ID,
		User:   user,
		GroupRoutes: []service.APIKeyGroupRoute{
			{GroupID: firstGrokGroupID, Priority: 10, Weight: 1, Enabled: true, CooldownSeconds: 30, Group: firstGrokGroup},
			{GroupID: openAIGroupID, Priority: 20, Weight: 1, Enabled: true, CooldownSeconds: 30, Group: openAIGroup},
			{GroupID: secondGrokGroupID, Priority: 30, Weight: 1, Enabled: true, CooldownSeconds: 30, Group: secondGrokGroup},
		},
	}
	apiKey.GroupID = &firstGrokGroup.ID
	apiKey.Group = firstGrokGroup

	accountRepo := &grokImageRouteAccountRepo{accountsByGroup: map[int64][]service.Account{
		secondGrokGroupID: {{
			ID:          accountID,
			Name:        "second-grok-route",
			Platform:    service.PlatformGrok,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Concurrency: 0,
			Credentials: map[string]any{"api_key": "test-key"},
		}},
	}}
	upstream := &grokImageRouteHTTPUpstream{}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	gatewayService := service.NewOpenAIGatewayService(
		accountRepo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil,
		upstream, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	billingService := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingService.Stop)
	h := NewOpenAIGatewayHandler(
		gatewayService,
		service.NewConcurrencyService(nil),
		billingService,
		service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg),
		nil, nil, nil, nil, cfg,
	)

	body := []byte(`{"model":"grok-imagine-image","prompt":"route me"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: user.ID})

	h.GrokImages(c)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s groups=%v upstream=%v", rec.Body.String(), accountRepo.groups(), upstream.calls())
	require.Equal(t, "https://images.test/route.png", gjson.GetBytes(rec.Body.Bytes(), "data.0.url").String())
	require.Equal(t, []int64{firstGrokGroupID, secondGrokGroupID}, accountRepo.groups())
	require.Equal(t, []int64{accountID}, upstream.calls())
	require.False(t, apiKeyGroupRouteBreaker.available(apiKey.ID, firstGrokGroupID, time.Now()))
	require.True(t, apiKeyGroupRouteBreaker.available(apiKey.ID, openAIGroupID, time.Now()))
	require.True(t, apiKeyGroupRouteBreaker.available(apiKey.ID, secondGrokGroupID, time.Now()))
}

func TestGrokImagesGroupRoutesEligibilityLimitFailsOverToNextGrokGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyGroupRouteBreaker = newAPIKeyGroupRouteCircuitBreaker()

	const (
		firstGroupID  int64 = 8601
		secondGroupID int64 = 8602
		successID     int64 = 8605
	)
	newGroup := func(id int64) *service.Group {
		return &service.Group{
			ID:                   id,
			Platform:             service.PlatformGrok,
			Status:               service.StatusActive,
			Hydrated:             true,
			AllowImageGeneration: true,
		}
	}
	firstGroup := newGroup(firstGroupID)
	secondGroup := newGroup(secondGroupID)
	user := &service.User{ID: 8603}
	apiKey := &service.APIKey{
		ID:      8604,
		UserID:  user.ID,
		User:    user,
		GroupID: &firstGroup.ID,
		Group:   firstGroup,
		GroupRoutes: []service.APIKeyGroupRoute{
			{GroupID: firstGroupID, Priority: 10, Weight: 1, Enabled: true, CooldownSeconds: 30, Group: firstGroup},
			{GroupID: secondGroupID, Priority: 20, Weight: 1, Enabled: true, CooldownSeconds: 30, Group: secondGroup},
		},
	}
	accountRepo := &grokImageRouteAccountRepo{accountsByGroup: map[int64][]service.Account{
		firstGroupID: {
			{
				ID: 8611, Name: "unobserved-oauth-1", Platform: service.PlatformGrok,
				Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true,
				Credentials: map[string]any{"access_token": "oauth-1"},
			},
			{
				ID: 8612, Name: "unobserved-oauth-2", Platform: service.PlatformGrok,
				Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true,
				Credentials: map[string]any{"access_token": "oauth-2"},
			},
		},
		secondGroupID: {{
			ID: successID, Name: "eligible-api-key", Platform: service.PlatformGrok,
			Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
			Credentials: map[string]any{"api_key": "success-key"},
		}},
	}}
	upstream := &grokImageRouteHTTPUpstream{}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	gatewayService := service.NewOpenAIGatewayService(
		accountRepo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil,
		upstream, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	billingService := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingService.Stop)
	h := NewOpenAIGatewayHandler(
		gatewayService,
		service.NewConcurrencyService(nil),
		billingService,
		service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg),
		nil, nil, nil, nil, cfg,
	)
	prober := &grokMediaEligibilityProberStub{eligible: false, reason: "billing_ineligible"}
	h.grokMediaEligibilityProber = prober
	h.maxAccountSwitches = 1

	body := []byte(`{"model":"grok-imagine-image","prompt":"route after eligibility rejection"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: user.ID})

	h.GrokImages(c)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s groups=%v upstream=%v", rec.Body.String(), accountRepo.groups(), upstream.calls())
	require.Equal(t, 2, prober.calls)
	require.Equal(t, []int64{firstGroupID, firstGroupID, secondGroupID}, accountRepo.groups())
	require.Equal(t, []int64{successID}, upstream.calls())
	require.False(t, apiKeyGroupRouteBreaker.available(apiKey.ID, firstGroupID, time.Now()))
}

func TestGrokImagesGroupRoutesReturnForbiddenWhenAllGrokRoutesDisallowImages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyGroupRouteBreaker = newAPIKeyGroupRouteCircuitBreaker()

	groupID := int64(8501)
	group := &service.Group{
		ID:       groupID,
		Platform: service.PlatformGrok,
		Status:   service.StatusActive,
		Hydrated: true,
	}
	apiKey := &service.APIKey{
		ID:      8502,
		GroupID: &groupID,
		Group:   group,
		User:    &service.User{ID: 8503},
		GroupRoutes: []service.APIKeyGroupRoute{{
			GroupID: groupID, Priority: 10, Weight: 1, Enabled: true, Group: group,
		}},
	}

	cursor := newAPIKeyGroupRouteCursor(apiKey)
	_, permissionDenied, ok := currentGrokImageRoute(cursor, nil)
	require.False(t, ok)
	require.True(t, permissionDenied)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	(&OpenAIGatewayHandler{}).grokImageRouteUnavailable(c, permissionDenied)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "permission_error", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
}
