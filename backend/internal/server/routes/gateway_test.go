package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"ikik-api/internal/config"
	"ikik-api/internal/handler"
	servermiddleware "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

type gatewayRouteSettingRepo struct {
	values map[string]string
}

func (r *gatewayRouteSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (r *gatewayRouteSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (r *gatewayRouteSettingRepo) Set(context.Context, string, string) error { return nil }

func (r *gatewayRouteSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *gatewayRouteSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }

func (r *gatewayRouteSettingRepo) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *gatewayRouteSettingRepo) Delete(context.Context, string) error { return nil }

func newGatewayRoutesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	settingSvc := service.NewSettingService(&gatewayRouteSettingRepo{values: map[string]string{}}, &config.Config{})

	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: &handler.OpenAIGatewayHandler{},
		},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{Platform: service.PlatformOpenAI},
			})
			c.Next()
		}),
		nil,
		nil,
		nil,
		settingSvc,
		&config.Config{},
	)

	return router
}

func TestGatewayRoutesOpenAIResponsesCompactPathIsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{
		"/v1/responses/compact",
		"/responses/compact",
		"/backend-api/codex/responses",
		"/backend-api/codex/responses/compact",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should hit OpenAI responses handler", path)
	}
}

func TestGatewayRoutesOpenAIImagesPathsAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{
		"/v1/images/generations",
		"/v1/images/edits",
		"/images/generations",
		"/images/edits",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should hit OpenAI images handler", path)
	}
}

func TestPrivateGroupRouteResolverFiltersRoutesByEndpoint(t *testing.T) {
	groupID := int64(1)
	routes := []service.APIKeyGroupRoute{
		privateRoute(1, service.PlatformAnthropic),
		privateRoute(2, service.PlatformOpenAI),
		privateRoute(3, service.PlatformGemini),
		privateRoute(4, service.PlatformKiro),
		privateRoute(5, service.PlatformGrok),
	}

	tests := []struct {
		name          string
		path          string
		wantPrimary   int64
		wantPlatforms []string
	}{
		{
			name:          "messages uses anthropic private group",
			path:          "/v1/messages",
			wantPrimary:   1,
			wantPlatforms: []string{service.PlatformAnthropic},
		},
		{
			name:          "chat completions uses openai-compatible text groups",
			path:          "/v1/chat/completions",
			wantPrimary:   2,
			wantPlatforms: []string{service.PlatformOpenAI, service.PlatformKiro},
		},
		{
			name:          "responses includes grok-compatible group",
			path:          "/v1/responses",
			wantPrimary:   2,
			wantPlatforms: []string{service.PlatformOpenAI, service.PlatformKiro, service.PlatformGrok},
		},
		{
			name:          "gemini native uses gemini private group",
			path:          "/v1beta/models/gemini-2.5-pro:generateContent",
			wantPrimary:   3,
			wantPlatforms: []string{service.PlatformGemini},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
					ID:          9,
					GroupID:     &groupID,
					Group:       routes[0].Group,
					GroupRoutes: routes,
				})
				c.Next()
			})
			router.Use(privateGroupRouteResolverMiddleware())
			router.Any("/*path", func(c *gin.Context) {
				apiKey, ok := servermiddleware.GetAPIKeyFromContext(c)
				require.True(t, ok)
				require.NotNil(t, apiKey.GroupID)
				require.Equal(t, tt.wantPrimary, *apiKey.GroupID)
				gotPlatforms := make([]string, 0, len(apiKey.GroupRoutes))
				for _, route := range apiKey.GroupRoutes {
					require.NotNil(t, route.Group)
					gotPlatforms = append(gotPlatforms, route.Group.Platform)
				}
				require.Equal(t, tt.wantPlatforms, gotPlatforms)
			})

			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(`{}`))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func privateRoute(id int64, platform string) service.APIKeyGroupRoute {
	return service.APIKeyGroupRoute{
		GroupID:         id,
		Priority:        int(100 + id),
		Weight:          1,
		Enabled:         true,
		CooldownSeconds: 30,
		Group: &service.Group{
			ID:       id,
			Name:     platform,
			Platform: platform,
			Scope:    service.GroupScopeUserPrivate,
			Status:   service.StatusActive,
		},
	}
}
