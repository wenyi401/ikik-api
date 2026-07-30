package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"ikik-api/internal/service"
)

func TestResolveAPIKeyGroupRouteForRequestFiltersHandlerFamily(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		path      string
		platforms []string
		want      []string
	}{
		{
			name:      "anthropic messages do not fail over into openai handler",
			path:      "/v1/messages",
			platforms: []string{service.PlatformAnthropic, service.PlatformOpenAI, service.PlatformGrok},
			want:      []string{service.PlatformAnthropic},
		},
		{
			name:      "openai messages can fail over to grok bridge",
			path:      "/v1/messages",
			platforms: []string{service.PlatformOpenAI, service.PlatformGrok, service.PlatformAnthropic},
			want:      []string{service.PlatformOpenAI, service.PlatformGrok},
		},
		{
			name:      "image handlers keep exact platform",
			path:      "/v1/images/generations",
			platforms: []string{service.PlatformGrok, service.PlatformOpenAI},
			want:      []string{service.PlatformGrok},
		},
		{
			name:      "chat handler keeps all supported openai compatible platforms",
			path:      "/v1/chat/completions",
			platforms: []string{service.PlatformOpenAI, service.PlatformKiro, service.PlatformGrok},
			want:      []string{service.PlatformOpenAI, service.PlatformKiro, service.PlatformGrok},
		},
		{
			name:      "grok count tokens skips an incompatible primary route",
			path:      "/v1/messages/count_tokens",
			platforms: []string{service.PlatformKiro, service.PlatformGrok, service.PlatformOpenAI},
			want:      []string{service.PlatformGrok},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiKey := testAPIKeyWithGroupRoutes(tt.platforms)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", tt.path, nil)

			resolved := resolveAPIKeyGroupRouteForRequest(c, apiKey)
			require.NotNil(t, resolved.Group)
			require.Equal(t, tt.want[0], resolved.Group.Platform)
			got := make([]string, 0, len(resolved.GroupRoutes))
			for _, route := range resolved.GroupRoutes {
				got = append(got, route.Group.Platform)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResolveAPIKeyGroupRouteForRequestHonorsForcedPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKey := testAPIKeyWithGroupRoutes([]string{service.PlatformOpenAI, service.PlatformGrok})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Set(string(ContextKeyForcePlatform), service.PlatformGrok)

	resolved := resolveAPIKeyGroupRouteForRequest(c, apiKey)
	require.NotNil(t, resolved.Group)
	require.Equal(t, service.PlatformGrok, resolved.Group.Platform)
	require.Len(t, resolved.GroupRoutes, 1)
}

func TestResolveAPIKeyGroupRouteForImagesSkipsDisabledRoutesWithinSelectedPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKey := testAPIKeyWithImageGroupRoutes([]imageGroupRouteTestCase{
		{platform: service.PlatformOpenAI, allowImages: false},
		{platform: service.PlatformOpenAI, allowImages: true},
		{platform: service.PlatformGrok, allowImages: true},
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/edits", nil)

	resolved := resolveAPIKeyGroupRouteForRequest(c, apiKey)

	require.NotNil(t, resolved.Group)
	require.Equal(t, service.PlatformOpenAI, resolved.Group.Platform)
	require.True(t, resolved.Group.AllowImageGeneration)
	require.Len(t, resolved.GroupRoutes, 1)
	require.Equal(t, int64(2), resolved.GroupRoutes[0].GroupID)
}

func TestResolveAPIKeyGroupRouteForImagesDoesNotCrossProviderForPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKey := testAPIKeyWithImageGroupRoutes([]imageGroupRouteTestCase{
		{platform: service.PlatformOpenAI, allowImages: false},
		{platform: service.PlatformGrok, allowImages: true},
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/edits", nil)

	resolved := resolveAPIKeyGroupRouteForRequest(c, apiKey)

	require.NotNil(t, resolved.Group)
	require.Equal(t, service.PlatformOpenAI, resolved.Group.Platform)
	require.False(t, resolved.Group.AllowImageGeneration)
	require.Len(t, resolved.GroupRoutes, 1)
}

func TestFilterGroupRoutesForHandlerFamilyDoesNotApplyImageGateToTasksOrBatches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{
		"/v1/images/tasks/imgtask_123",
		"/v1/images/batches",
		"/v1/images/batches/123/items",
	} {
		t.Run(path, func(t *testing.T) {
			apiKey := testAPIKeyWithImageGroupRoutes([]imageGroupRouteTestCase{
				{platform: service.PlatformOpenAI, allowImages: false},
				{platform: service.PlatformGrok, allowImages: true},
				{platform: service.PlatformOpenAI, allowImages: true},
			})
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", path, nil)

			resolved := resolveAPIKeyGroupRouteForRequest(c, apiKey)

			require.NotNil(t, resolved.Group)
			require.Equal(t, int64(1), resolved.Group.ID)
			require.Len(t, resolved.GroupRoutes, 2)
			require.Equal(t, int64(1), resolved.GroupRoutes[0].GroupID)
			require.Equal(t, int64(3), resolved.GroupRoutes[1].GroupID)
			for _, route := range resolved.GroupRoutes {
				require.Equal(t, service.PlatformOpenAI, route.Group.Platform)
			}
		})
	}
}

type imageGroupRouteTestCase struct {
	platform    string
	allowImages bool
}

func testAPIKeyWithImageGroupRoutes(groups []imageGroupRouteTestCase) *service.APIKey {
	routes := make([]service.APIKeyGroupRoute, 0, len(groups))
	for i, item := range groups {
		groupID := int64(i + 1)
		group := &service.Group{
			ID:                   groupID,
			Platform:             item.platform,
			Status:               service.StatusActive,
			AllowImageGeneration: item.allowImages,
		}
		routes = append(routes, service.APIKeyGroupRoute{
			GroupID:         groupID,
			Priority:        (i + 1) * 100,
			Weight:          1,
			Enabled:         true,
			CooldownSeconds: 30,
			Group:           group,
		})
	}
	return &service.APIKey{ID: 99, GroupRoutes: routes}
}

func testAPIKeyWithGroupRoutes(platforms []string) *service.APIKey {
	routes := make([]service.APIKeyGroupRoute, 0, len(platforms))
	for i, platform := range platforms {
		groupID := int64(i + 1)
		group := &service.Group{ID: groupID, Platform: platform, Status: service.StatusActive}
		routes = append(routes, service.APIKeyGroupRoute{
			GroupID:         groupID,
			Priority:        (i + 1) * 100,
			Weight:          1,
			Enabled:         true,
			CooldownSeconds: 30,
			Group:           group,
		})
	}
	return &service.APIKey{ID: 99, GroupRoutes: routes}
}
