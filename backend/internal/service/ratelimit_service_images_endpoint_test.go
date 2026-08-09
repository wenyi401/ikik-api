package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageEndpointRateLimitRepo struct {
	AccountRepository
	calls []imageEndpointRateLimitCall
}

type imageEndpointRateLimitCall struct {
	scope  string
	reason string
}

func (r *imageEndpointRateLimitRepo) SetModelRateLimit(_ context.Context, _ int64, scope string, _ time.Time, reason ...string) error {
	call := imageEndpointRateLimitCall{scope: scope}
	if len(reason) > 0 {
		call.reason = reason[0]
	}
	r.calls = append(r.calls, call)
	return nil
}

func TestRateLimitServiceCodexPlanGatedImageCooldownDependsOnEndpoint(t *testing.T) {
	account := &Account{
		ID:       8801,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"image-alias": "gpt-image-2"},
		},
	}
	body := []byte(`{"detail":"The 'gpt-image-2' model is not supported when using Codex with a ChatGPT account."}`)

	t.Run("text endpoint does not cool the image capability", func(t *testing.T) {
		repo := &imageEndpointRateLimitRepo{}
		svc := &RateLimitService{accountRepo: repo}

		handled := svc.HandleUpstreamModelNotFound(context.Background(), account, "image-alias", http.StatusBadRequest, body)

		require.True(t, handled)
		require.Empty(t, repo.calls)
	})

	t.Run("dedicated images endpoint still cools an unsupported account", func(t *testing.T) {
		repo := &imageEndpointRateLimitRepo{}
		svc := &RateLimitService{accountRepo: repo}

		handled := svc.HandleUpstreamModelNotFound(WithOpenAIImagesEndpoint(context.Background()), account, "image-alias", http.StatusBadRequest, body)

		require.True(t, handled)
		require.Equal(t, []imageEndpointRateLimitCall{{scope: "gpt-image-2", reason: upstreamCodexPlanGatedModelReason}}, repo.calls)
	})
}
