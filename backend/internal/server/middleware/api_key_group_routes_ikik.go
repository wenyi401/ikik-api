package middleware

import (
	"strings"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

func resolveAPIKeyGroupRouteForRequest(c *gin.Context, apiKey *service.APIKey) *service.APIKey {
	if apiKey == nil || len(apiKey.GroupRoutes) == 0 {
		return apiKey
	}
	compatible := compatibleGroupPlatformsForRequest(c)
	if len(compatible) == 0 {
		return apiKey
	}

	routes := make([]service.APIKeyGroupRoute, 0, len(apiKey.GroupRoutes))
	for _, route := range apiKey.GroupRoutes {
		if !route.Enabled || route.Group == nil || !route.Group.IsActive() {
			continue
		}
		if _, ok := compatible[route.Group.Platform]; ok {
			routes = append(routes, route)
		}
	}
	if len(routes) == 0 {
		return apiKey
	}
	routes = filterGroupRoutesForHandlerFamily(c, routes)
	if len(routes) == 0 {
		return apiKey
	}

	resolved := *apiKey
	groupID := routes[0].GroupID
	resolved.GroupID = &groupID
	resolved.Group = routes[0].Group
	resolved.GroupRoutes = routes
	return &resolved
}

func filterGroupRoutesForHandlerFamily(c *gin.Context, routes []service.APIKeyGroupRoute) []service.APIKeyGroupRoute {
	if len(routes) < 2 || c == nil || c.Request == nil || c.Request.URL == nil || routes[0].Group == nil {
		return routes
	}
	path := strings.ToLower(strings.TrimSpace(c.Request.URL.Path))
	selectedPlatform := routes[0].Group.Platform

	var keep func(string) bool
	switch {
	case strings.Contains(path, "/messages/count_tokens"):
		keep = func(platform string) bool { return platform == selectedPlatform }
	case strings.Contains(path, "/messages"):
		if selectedPlatform == service.PlatformAnthropic {
			keep = func(platform string) bool { return platform == service.PlatformAnthropic }
		} else {
			keep = func(platform string) bool {
				return platform == service.PlatformOpenAI || platform == service.PlatformGrok
			}
		}
	case strings.Contains(path, "/images/"):
		keep = func(platform string) bool { return platform == selectedPlatform }
	default:
		return routes
	}

	filtered := make([]service.APIKeyGroupRoute, 0, len(routes))
	for _, route := range routes {
		if route.Group != nil && keep(route.Group.Platform) {
			filtered = append(filtered, route)
		}
	}
	return filtered
}

func compatibleGroupPlatformsForRequest(c *gin.Context) map[string]struct{} {
	if forcedPlatform, ok := GetForcePlatformFromContext(c); ok && strings.TrimSpace(forcedPlatform) != "" {
		return map[string]struct{}{forcedPlatform: {}}
	}

	path := ""
	if c != nil && c.Request != nil && c.Request.URL != nil {
		path = strings.ToLower(strings.TrimSpace(c.Request.URL.Path))
	}
	switch {
	case strings.Contains(path, "/v1beta/models"):
		return map[string]struct{}{
			service.PlatformGemini:      {},
			service.PlatformAntigravity: {},
		}
	case strings.Contains(path, "/chat/completions"):
		return map[string]struct{}{
			service.PlatformOpenAI: {},
			service.PlatformGrok:   {},
			service.PlatformKiro:   {},
		}
	case strings.Contains(path, "/responses") || strings.Contains(path, "/alpha/search"):
		return map[string]struct{}{
			service.PlatformOpenAI: {},
			service.PlatformGrok:   {},
		}
	case strings.Contains(path, "/messages/count_tokens"):
		return map[string]struct{}{
			service.PlatformAnthropic: {},
			service.PlatformOpenAI:    {},
		}
	case strings.Contains(path, "/messages"):
		return map[string]struct{}{
			service.PlatformAnthropic: {},
			service.PlatformOpenAI:    {},
			service.PlatformGrok:      {},
		}
	case strings.Contains(path, "/embeddings"):
		return map[string]struct{}{service.PlatformOpenAI: {}}
	case strings.Contains(path, "/images/"):
		return map[string]struct{}{
			service.PlatformOpenAI: {},
			service.PlatformGrok:   {},
		}
	case strings.Contains(path, "/videos/"):
		return map[string]struct{}{service.PlatformGrok: {}}
	default:
		return nil
	}
}
