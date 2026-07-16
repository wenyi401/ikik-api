package handler

import (
	"context"

	"ikik-api/internal/pkg/ctxkey"
	"ikik-api/internal/service"
)

func gatewayRouteContext(ctx context.Context, apiKey *service.APIKey, userID int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if userID > 0 {
		if existing, ok := ctx.Value(ctxkey.AuthenticatedUserID).(int64); !ok || existing != userID {
			ctx = context.WithValue(ctx, ctxkey.AuthenticatedUserID, userID)
		}
	}
	if apiKey != nil && service.IsGroupContextValid(apiKey.Group) {
		if existing, ok := ctx.Value(ctxkey.Group).(*service.Group); !ok || existing == nil || existing.ID != apiKey.Group.ID || !service.IsGroupContextValid(existing) {
			ctx = context.WithValue(ctx, ctxkey.Group, apiKey.Group)
		}
	}
	return ctx
}
