package handler

import (
	"context"

	"ikik-api/internal/service"
)

func gatewayForwardContext(routeCtx context.Context, switchCount int, bridgeMetadata bool) context.Context {
	if routeCtx == nil {
		routeCtx = context.Background()
	}
	if switchCount > 0 {
		return service.WithAccountSwitchCount(routeCtx, switchCount, bridgeMetadata)
	}
	return routeCtx
}
