package service

import (
	"context"
)

func SetAntigravityPrivacy(ctx context.Context, accessToken, projectID, proxyURL string) string {
	return setAntigravityPrivacy(ctx, accessToken, projectID, proxyURL)
}
