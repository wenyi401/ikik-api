//go:build unit

package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"ikik-api/internal/service"
)

func TestValidateAPIKeyGroupAllowedRejectsBlockedGroup(t *testing.T) {
	groupID := int64(42)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		User: &service.User{
			BlockedGroups: []int64{groupID},
		},
		Group: &service.Group{
			ID:               groupID,
			SubscriptionType: service.SubscriptionTypeSubscription,
		},
	}

	require.False(t, validateAPIKeyGroupAllowed(apiKey))
}

func TestResolveAPIKeyGroupRouteSkipsBlockedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKey := &service.APIKey{
		User: &service.User{BlockedGroups: []int64{1}},
		GroupRoutes: []service.APIKeyGroupRoute{
			{GroupID: 1, Enabled: true, Group: &service.Group{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive}},
			{GroupID: 2, Enabled: true, Group: &service.Group{ID: 2, Platform: service.PlatformGrok, Status: service.StatusActive}},
		},
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	resolved := resolveAPIKeyGroupRouteForRequest(c, apiKey)
	require.NotNil(t, resolved.Group)
	require.Equal(t, int64(2), resolved.Group.ID)
	require.Len(t, resolved.GroupRoutes, 1)
}
