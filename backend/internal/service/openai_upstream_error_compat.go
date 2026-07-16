package service

import (
	"context"
	"log/slog"
	"net/http"
)

func (s *OpenAIGatewayService) handleOpenAIAccountUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte, _ ...string) bool {
	if s == nil || account == nil || s.rateLimitService == nil {
		return false
	}
	if account.Platform == PlatformOpenAI && isOpenAIContextWindowError("", responseBody) {
		return false
	}
	if s.shouldSkipPersistentRateLimitForCarpool(ctx, account, statusCode) {
		s.rateLimitService.persistOpenAICodexSnapshot(ctx, account, headers)
		slog.Info("carpool_rate_limit_persist_skipped", "account_id", account.ID, "status_code", statusCode)
		return false
	}
	return s.rateLimitService.HandleUpstreamError(ctx, account, statusCode, headers, responseBody)
}
