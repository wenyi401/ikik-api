package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/service"
)

func TestAPIKeyGroupRouteCursorOrdersAndSwitchesRoutes(t *testing.T) {
	apiKeyGroupRouteBreaker = newAPIKeyGroupRouteCircuitBreaker()
	apiKey := routeCursorTestAPIKey()

	cursor := newAPIKeyGroupRouteCursor(apiKey)
	current, ok := cursor.current()
	require.True(t, ok)
	require.Equal(t, int64(20), current.Route.GroupID)

	require.True(t, cursor.switchToNext(apiKey.ID, "test", nil))
	current, ok = cursor.current()
	require.True(t, ok)
	require.Equal(t, int64(30), current.Route.GroupID)

	// The failed first route is skipped by a new cursor until its cooldown ends.
	newCursor := newAPIKeyGroupRouteCursor(apiKey)
	current, ok = newCursor.current()
	require.True(t, ok)
	require.Equal(t, int64(30), current.Route.GroupID)
}

func TestAPIKeyGroupRouteCircuitBreakerSuccessClearsCooldown(t *testing.T) {
	breaker := newAPIKeyGroupRouteCircuitBreaker()
	breaker.recordFailure(7, 8, 30)
	require.False(t, breaker.available(7, 8, time.Now()))

	breaker.recordSuccess(7, 8)
	require.True(t, breaker.available(7, 8, time.Now()))
}

func TestShouldSwitchAPIKeyGroupRouteOnlyForTransientFailures(t *testing.T) {
	require.True(t, shouldSwitchAPIKeyGroupRoute(&service.UpstreamFailoverError{StatusCode: http.StatusTooManyRequests}))
	require.True(t, shouldSwitchAPIKeyGroupRoute(&service.UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable}))
	require.False(t, shouldSwitchAPIKeyGroupRoute(&service.UpstreamFailoverError{StatusCode: http.StatusBadRequest}))
	require.False(t, shouldSwitchAPIKeyGroupRoute(&service.UpstreamFailoverError{StatusCode: http.StatusUnauthorized}))
}

func TestCloneAPIKeyWithGroupClearsOnlyForeignGroupRPMOverride(t *testing.T) {
	primaryGroupID := int64(10)
	override := 0
	apiKey := &service.APIKey{
		GroupID: &primaryGroupID,
		Group:   &service.Group{ID: primaryGroupID},
		User:    &service.User{ID: 42, UserGroupRPMOverride: &override},
	}

	primary := cloneAPIKeyWithGroup(apiKey, &service.Group{ID: primaryGroupID})
	require.Same(t, apiKey.User, primary.User)
	require.NotNil(t, primary.User.UserGroupRPMOverride)

	fallback := cloneAPIKeyWithGroup(apiKey, &service.Group{ID: 20})
	require.NotSame(t, apiKey.User, fallback.User)
	require.Nil(t, fallback.User.UserGroupRPMOverride)
	require.NotNil(t, apiKey.User.UserGroupRPMOverride, "cloning must not mutate the auth snapshot")
}

func routeCursorTestAPIKey() *service.APIKey {
	group := func(id int64) *service.Group {
		return &service.Group{ID: id, Platform: service.PlatformOpenAI, Status: service.StatusActive}
	}
	return &service.APIKey{
		ID: 7,
		GroupRoutes: []service.APIKeyGroupRoute{
			{GroupID: 10, Priority: 200, Weight: 1, Enabled: true, CooldownSeconds: 30, Group: group(10)},
			{GroupID: 30, Priority: 100, Weight: 1, Enabled: true, CooldownSeconds: 30, Group: group(30)},
			{GroupID: 20, Priority: 100, Weight: 3, Enabled: true, CooldownSeconds: 30, Group: group(20)},
		},
	}
}
