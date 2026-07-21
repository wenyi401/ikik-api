package dto

import (
	"time"

	"ikik-api/internal/service"
)

type APIKeyGroupRoute struct {
	ID              int64      `json:"id,omitempty"`
	APIKeyID        int64      `json:"api_key_id,omitempty"`
	GroupID         int64      `json:"group_id"`
	Priority        int        `json:"priority"`
	Weight          int        `json:"weight"`
	Enabled         bool       `json:"enabled"`
	CooldownSeconds int        `json:"cooldown_seconds"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	Group           *Group     `json:"group,omitempty"`
}

func APIKeyGroupRouteFromService(route *service.APIKeyGroupRoute) *APIKeyGroupRoute {
	if route == nil {
		return nil
	}
	out := &APIKeyGroupRoute{
		ID:              route.ID,
		APIKeyID:        route.APIKeyID,
		GroupID:         route.GroupID,
		Priority:        route.Priority,
		Weight:          route.Weight,
		Enabled:         route.Enabled,
		CooldownSeconds: route.CooldownSeconds,
		Group:           GroupFromServiceShallow(route.Group),
	}
	if !route.CreatedAt.IsZero() {
		createdAt := route.CreatedAt
		out.CreatedAt = &createdAt
	}
	if !route.UpdatedAt.IsZero() {
		updatedAt := route.UpdatedAt
		out.UpdatedAt = &updatedAt
	}
	return out
}

func attachAPIKeyGroupRoutes(out *APIKey, key *service.APIKey) {
	if out == nil || key == nil || len(key.GroupRoutes) == 0 {
		return
	}
	out.GroupRoutes = make([]APIKeyGroupRoute, 0, len(key.GroupRoutes))
	for i := range key.GroupRoutes {
		out.GroupRoutes = append(out.GroupRoutes, *APIKeyGroupRouteFromService(&key.GroupRoutes[i]))
	}
}
