package service

import (
	"context"
)

func DisableOpenAITraining(ctx context.Context, clientFactory PrivacyClientFactory, accessToken, proxyURL string) string {
	return disableOpenAITraining(ctx, clientFactory, accessToken, proxyURL)
}
