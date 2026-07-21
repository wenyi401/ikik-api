package handler

import "ikik-api/internal/service"

type APIKeyGroupRouteRequest struct {
	GroupID         int64 `json:"group_id"`
	Priority        int   `json:"priority"`
	Weight          int   `json:"weight"`
	Enabled         *bool `json:"enabled"`
	CooldownSeconds int   `json:"cooldown_seconds"`
}

func apiKeyGroupRouteRequestsToService(routes []APIKeyGroupRouteRequest) []service.APIKeyGroupRoute {
	if len(routes) == 0 {
		return nil
	}
	out := make([]service.APIKeyGroupRoute, 0, len(routes))
	for _, route := range routes {
		enabled := true
		if route.Enabled != nil {
			enabled = *route.Enabled
		}
		out = append(out, service.APIKeyGroupRoute{
			GroupID:         route.GroupID,
			Priority:        route.Priority,
			Weight:          route.Weight,
			Enabled:         enabled,
			CooldownSeconds: route.CooldownSeconds,
		})
	}
	return out
}
