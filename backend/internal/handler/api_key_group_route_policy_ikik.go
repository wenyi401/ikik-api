package handler

import "ikik-api/internal/service"

func hasNonClaudeCodeAPIKeyGroupRoute(apiKey *service.APIKey) bool {
	if apiKey == nil {
		return false
	}
	if apiKey.Group == nil || !apiKey.Group.ClaudeCodeOnly {
		return true
	}
	for i := range apiKey.GroupRoutes {
		route := &apiKey.GroupRoutes[i]
		if route.Enabled && route.Group != nil && !route.Group.ClaudeCodeOnly {
			return true
		}
	}
	return false
}
