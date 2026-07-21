package service

import (
	"context"
	"fmt"
	"time"
)

type APIKeyGroupRoute struct {
	ID              int64
	APIKeyID        int64
	GroupID         int64
	Priority        int
	Weight          int
	Enabled         bool
	CooldownSeconds int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Group           *Group
}

func normalizeAPIKeyGroupRoutes(routes []APIKeyGroupRoute) ([]APIKeyGroupRoute, error) {
	if len(routes) == 0 {
		return nil, nil
	}

	normalized := make([]APIKeyGroupRoute, 0, len(routes))
	seen := make(map[int64]struct{}, len(routes))
	for _, route := range routes {
		if route.GroupID <= 0 {
			return nil, ErrAPIKeyGroupRouteInvalid
		}
		if _, exists := seen[route.GroupID]; exists {
			return nil, ErrAPIKeyGroupRouteInvalid
		}
		seen[route.GroupID] = struct{}{}
		if route.Priority <= 0 {
			route.Priority = 100
		}
		if route.Weight <= 0 {
			route.Weight = 1
		}
		if route.CooldownSeconds <= 0 {
			route.CooldownSeconds = 30
		}
		normalized = append(normalized, route)
	}
	return normalized, nil
}

func defaultAPIKeyGroupRoute(groupID *int64) []APIKeyGroupRoute {
	if groupID == nil || *groupID <= 0 {
		return nil
	}
	return []APIKeyGroupRoute{{
		GroupID:         *groupID,
		Priority:        100,
		Weight:          1,
		Enabled:         true,
		CooldownSeconds: 30,
	}}
}

func primaryGroupIDFromRoutes(routes []APIKeyGroupRoute) *int64 {
	var selected *APIKeyGroupRoute
	for i := range routes {
		route := &routes[i]
		if !route.Enabled {
			continue
		}
		if selected == nil ||
			route.Priority < selected.Priority ||
			(route.Priority == selected.Priority && route.Weight > selected.Weight) ||
			(route.Priority == selected.Priority && route.Weight == selected.Weight && route.GroupID < selected.GroupID) {
			selected = route
		}
	}
	if selected == nil || selected.GroupID <= 0 {
		return nil
	}
	groupID := selected.GroupID
	return &groupID
}

func (s *APIKeyService) isUngroupedKeySchedulingAllowed(ctx context.Context) bool {
	return s.settingService != nil && s.settingService.IsUngroupedKeySchedulingAllowed(ctx)
}

func (s *APIKeyService) validateAPIKeyGroupRoutes(ctx context.Context, user *User, routes []APIKeyGroupRoute) error {
	for i := range routes {
		group, err := s.groupRepo.GetByID(ctx, routes[i].GroupID)
		if err != nil {
			return fmt.Errorf("get group: %w", err)
		}
		if !s.canUserBindAPIKeyGroup(ctx, user, group) {
			return ErrGroupNotAllowed
		}
		routes[i].Group = group
	}
	return nil
}

func (s *APIKeyService) canUserBindAPIKeyGroup(ctx context.Context, user *User, group *Group) bool {
	if canUserBindStandardGroup(user, group) {
		return true
	}
	if user == nil || group == nil || !group.IsSubscriptionType() {
		return false
	}
	if (group.IsUserPrivateScope() || group.IsUserCarpoolScope()) && !isGroupOwnedByUser(group, user.ID) {
		return false
	}
	_, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, user.ID, group.ID)
	return err == nil
}

func (s *APIKeyService) prepareAPIKeyGroupRoutesForCreate(ctx context.Context, user *User, req *CreateAPIKeyRequest) ([]APIKeyGroupRoute, error) {
	routes, err := normalizeAPIKeyGroupRoutes(req.GroupRoutes)
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		routes = defaultAPIKeyGroupRoute(req.GroupID)
	}
	if len(routes) == 0 {
		if s.isUngroupedKeySchedulingAllowed(ctx) {
			return nil, nil
		}
		return nil, ErrAPIKeyGroupRequired
	}
	if err := s.validateAPIKeyGroupRoutes(ctx, user, routes); err != nil {
		return nil, err
	}
	primaryGroupID := primaryGroupIDFromRoutes(routes)
	if primaryGroupID == nil {
		return nil, ErrAPIKeyGroupRouteInvalid
	}
	req.GroupID = primaryGroupID
	return routes, nil
}

func (s *APIKeyService) applyAPIKeyGroupRoutesUpdate(ctx context.Context, userID int64, apiKey *APIKey, req UpdateAPIKeyRequest) error {
	if req.GroupID == nil && req.GroupRoutes == nil {
		return nil
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	var routes []APIKeyGroupRoute
	if req.GroupRoutes != nil {
		routes, err = normalizeAPIKeyGroupRoutes(*req.GroupRoutes)
		if err != nil {
			return err
		}
	} else {
		routes = defaultAPIKeyGroupRoute(req.GroupID)
	}
	if len(routes) == 0 {
		if !s.isUngroupedKeySchedulingAllowed(ctx) {
			return ErrAPIKeyGroupRequired
		}
		apiKey.GroupID = nil
		apiKey.Group = nil
		apiKey.GroupRoutes = nil
		return nil
	}
	if err := s.validateAPIKeyGroupRoutes(ctx, user, routes); err != nil {
		return err
	}
	primaryGroupID := primaryGroupIDFromRoutes(routes)
	if primaryGroupID == nil {
		return ErrAPIKeyGroupRouteInvalid
	}
	apiKey.GroupID = primaryGroupID
	apiKey.GroupRoutes = routes
	for i := range routes {
		if routes[i].GroupID == *primaryGroupID {
			apiKey.Group = routes[i].Group
			break
		}
	}
	return nil
}
