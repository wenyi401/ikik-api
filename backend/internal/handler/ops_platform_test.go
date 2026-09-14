package handler

import (
	"context"
	"testing"

	"ikik-api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResolveOpsPlatformPrefersResolvedCompositeTarget(t *testing.T) {
	apiKey := &service.APIKey{Group: &service.Group{Platform: service.PlatformComposite}}
	ctx := service.WithResolvedTargetPlatform(context.Background(), service.PlatformOpenAI)

	require.Equal(t, service.PlatformOpenAI, resolveOpsPlatform(ctx, apiKey, service.PlatformAnthropic))
}
