package handler

import (
	"context"
	"testing"

	"ikik-api/internal/service"
)

func TestGatewayForwardContext_PreservesRouteMetadataAndAddsSwitchCount(t *testing.T) {
	group := &service.Group{
		ID:       88,
		Name:     "carpool-group",
		Platform: service.PlatformOpenAI,
		Status:   service.StatusActive,
		Hydrated: true,
	}
	apiKey := &service.APIKey{Group: group}

	routeCtx := gatewayRouteContext(context.Background(), apiKey, 123)
	forwardCtx := gatewayForwardContext(routeCtx, 2, false)

	if got := service.AuthenticatedUserIDFromContext(forwardCtx); got != 123 {
		t.Fatalf("AuthenticatedUserIDFromContext = %d, want 123", got)
	}
	gotGroup := service.GroupFromContext(forwardCtx)
	if gotGroup == nil || gotGroup.ID != group.ID {
		t.Fatalf("GroupFromContext = %#v, want group id %d", gotGroup, group.ID)
	}
	switchCount, ok := service.AccountSwitchCountFromContext(forwardCtx)
	if !ok || switchCount != 2 {
		t.Fatalf("AccountSwitchCountFromContext = (%d, %v), want (2, true)", switchCount, ok)
	}
}

func TestGatewayForwardContext_ReturnsRouteContextWhenNoSwitchCount(t *testing.T) {
	group := &service.Group{
		ID:       99,
		Name:     "carpool-group",
		Platform: service.PlatformOpenAI,
		Status:   service.StatusActive,
		Hydrated: true,
	}
	apiKey := &service.APIKey{Group: group}

	routeCtx := gatewayRouteContext(context.Background(), apiKey, 456)
	forwardCtx := gatewayForwardContext(routeCtx, 0, false)

	if got := service.AuthenticatedUserIDFromContext(forwardCtx); got != 456 {
		t.Fatalf("AuthenticatedUserIDFromContext = %d, want 456", got)
	}
	gotGroup := service.GroupFromContext(forwardCtx)
	if gotGroup == nil || gotGroup.ID != group.ID {
		t.Fatalf("GroupFromContext = %#v, want group id %d", gotGroup, group.ID)
	}
	if _, ok := service.AccountSwitchCountFromContext(forwardCtx); ok {
		t.Fatal("AccountSwitchCountFromContext should be absent when switchCount is 0")
	}
}
