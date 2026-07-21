package handler

import (
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var apiKeyGroupRouteBreaker = newAPIKeyGroupRouteCircuitBreaker()

type apiKeyGroupRouteCandidate struct {
	APIKey *service.APIKey
	Route  service.APIKeyGroupRoute
}

type apiKeyGroupRouteCursor struct {
	candidates []apiKeyGroupRouteCandidate
	index      int
	available  bool
}

type apiKeyGroupRouteCircuitBreaker struct {
	mu     sync.Mutex
	states map[string]apiKeyGroupRouteBreakerState
}

type apiKeyGroupRouteBreakerState struct {
	cooldownUntil time.Time
	failures      int
}

func newAPIKeyGroupRouteCircuitBreaker() *apiKeyGroupRouteCircuitBreaker {
	return &apiKeyGroupRouteCircuitBreaker{states: make(map[string]apiKeyGroupRouteBreakerState)}
}

func apiKeyGroupRouteBreakerKey(apiKeyID, groupID int64) string {
	return strconv.FormatInt(apiKeyID, 10) + ":" + strconv.FormatInt(groupID, 10)
}

func (b *apiKeyGroupRouteCircuitBreaker) available(apiKeyID, groupID int64, now time.Time) bool {
	if b == nil || apiKeyID <= 0 || groupID <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	key := apiKeyGroupRouteBreakerKey(apiKeyID, groupID)
	state, ok := b.states[key]
	if !ok || state.cooldownUntil.IsZero() || !now.Before(state.cooldownUntil) {
		if ok && !state.cooldownUntil.IsZero() {
			delete(b.states, key)
		}
		return true
	}
	return false
}

func (b *apiKeyGroupRouteCircuitBreaker) recordFailure(apiKeyID, groupID int64, cooldownSeconds int) {
	if b == nil || apiKeyID <= 0 || groupID <= 0 {
		return
	}
	if cooldownSeconds <= 0 {
		cooldownSeconds = 30
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	key := apiKeyGroupRouteBreakerKey(apiKeyID, groupID)
	state := b.states[key]
	state.failures++
	multiplier := 1 << min(state.failures-1, 4)
	state.cooldownUntil = time.Now().Add(time.Duration(cooldownSeconds*multiplier) * time.Second)
	b.states[key] = state
}

func (b *apiKeyGroupRouteCircuitBreaker) recordSuccess(apiKeyID, groupID int64) {
	if b == nil || apiKeyID <= 0 || groupID <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.states, apiKeyGroupRouteBreakerKey(apiKeyID, groupID))
}

func newAPIKeyGroupRouteCursor(apiKey *service.APIKey) *apiKeyGroupRouteCursor {
	candidates, available := buildAPIKeyGroupRouteCandidates(apiKey)
	return &apiKeyGroupRouteCursor{candidates: candidates, available: available}
}

func (c *apiKeyGroupRouteCursor) current() (apiKeyGroupRouteCandidate, bool) {
	if c == nil || !c.available || c.index < 0 || c.index >= len(c.candidates) {
		return apiKeyGroupRouteCandidate{}, false
	}
	candidate := c.candidates[c.index]
	return candidate, candidate.APIKey != nil
}

func (c *apiKeyGroupRouteCursor) hasNext() bool {
	return c != nil && c.available && c.index+1 < len(c.candidates)
}

func (c *apiKeyGroupRouteCursor) switchToNext(apiKeyID int64, reason string, reqLog *zap.Logger, fields ...zap.Field) bool {
	if c == nil || !c.hasNext() {
		return false
	}
	current, ok := c.current()
	if !ok {
		return false
	}
	apiKeyGroupRouteBreaker.recordFailure(apiKeyID, current.Route.GroupID, current.Route.CooldownSeconds)
	c.index++
	next, _ := c.current()
	if reqLog != nil {
		logFields := []zap.Field{
			zap.String("reason", reason),
			zap.Int64("from_group_id", current.Route.GroupID),
			zap.Int("from_priority", current.Route.Priority),
			zap.Int64("to_group_id", next.Route.GroupID),
			zap.Int("to_priority", next.Route.Priority),
		}
		logFields = append(logFields, fields...)
		reqLog.Warn("api_key_group_route.switching", logFields...)
	}
	return true
}

func (c *apiKeyGroupRouteCursor) skipToNext(reason string, reqLog *zap.Logger, fields ...zap.Field) bool {
	if c == nil || !c.hasNext() {
		return false
	}
	current, ok := c.current()
	if !ok {
		return false
	}
	c.index++
	next, _ := c.current()
	if reqLog != nil {
		logFields := []zap.Field{
			zap.String("reason", reason),
			zap.Int64("from_group_id", current.Route.GroupID),
			zap.Int64("to_group_id", next.Route.GroupID),
		}
		logFields = append(logFields, fields...)
		reqLog.Debug("api_key_group_route.skipping", logFields...)
	}
	return true
}

func (c *apiKeyGroupRouteCursor) recordSuccess(apiKeyID int64) {
	current, ok := c.current()
	if ok {
		apiKeyGroupRouteBreaker.recordSuccess(apiKeyID, current.Route.GroupID)
	}
}

func canSwitchAPIKeyGroupRouteAfterForward(c *gin.Context, cursor *apiKeyGroupRouteCursor, failoverErr *service.UpstreamFailoverError, streamStarted bool, writerSizeBeforeForward int) bool {
	if cursor == nil || !cursor.hasNext() || !shouldSwitchAPIKeyGroupRoute(failoverErr) || streamStarted {
		return false
	}
	return c == nil || c.Writer == nil || c.Writer.Size() == writerSizeBeforeForward
}

func buildAPIKeyGroupRouteCandidates(apiKey *service.APIKey) ([]apiKeyGroupRouteCandidate, bool) {
	if apiKey == nil {
		return nil, false
	}
	routes := append([]service.APIKeyGroupRoute(nil), apiKey.GroupRoutes...)
	hasConfiguredRoutes := len(routes) > 0
	if len(routes) == 0 && apiKey.GroupID != nil && apiKey.Group != nil {
		routes = []service.APIKeyGroupRoute{{
			GroupID:         *apiKey.GroupID,
			Priority:        100,
			Weight:          1,
			Enabled:         true,
			CooldownSeconds: 30,
			Group:           apiKey.Group,
		}}
	}
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Priority != routes[j].Priority {
			return routes[i].Priority < routes[j].Priority
		}
		if routes[i].Weight != routes[j].Weight {
			return routes[i].Weight > routes[j].Weight
		}
		return routes[i].GroupID < routes[j].GroupID
	})

	now := time.Now()
	candidates := make([]apiKeyGroupRouteCandidate, 0, len(routes))
	for _, route := range routes {
		if !route.Enabled || route.Group == nil || route.GroupID <= 0 {
			continue
		}
		if !apiKeyGroupRouteBreaker.available(apiKey.ID, route.GroupID, now) {
			continue
		}
		candidates = append(candidates, apiKeyGroupRouteCandidate{
			APIKey: cloneAPIKeyWithGroup(apiKey, route.Group),
			Route:  route,
		})
	}
	if len(candidates) == 0 && apiKey.GroupID != nil && apiKey.Group != nil && !hasConfiguredRoutes {
		candidates = append(candidates, apiKeyGroupRouteCandidate{
			APIKey: cloneAPIKeyWithGroup(apiKey, apiKey.Group),
			Route: service.APIKeyGroupRoute{
				GroupID:         *apiKey.GroupID,
				Priority:        100,
				Weight:          1,
				Enabled:         true,
				CooldownSeconds: 30,
				Group:           apiKey.Group,
			},
		})
	}
	if len(candidates) == 0 && apiKey.GroupID == nil {
		candidates = append(candidates, apiKeyGroupRouteCandidate{
			APIKey: apiKey,
			Route: service.APIKeyGroupRoute{
				Priority:        100,
				Weight:          1,
				Enabled:         true,
				CooldownSeconds: 30,
			},
		})
	}
	return candidates, len(candidates) > 0
}

func shouldSwitchAPIKeyGroupRoute(failoverErr *service.UpstreamFailoverError) bool {
	if failoverErr == nil {
		return false
	}
	switch failoverErr.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 529:
		return true
	default:
		return failoverErr.StatusCode >= 500
	}
}
