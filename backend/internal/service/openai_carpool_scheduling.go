package service

import (
	"context"
	"net/http"
)

func (s *OpenAIGatewayService) isCarpoolSchedulingAccount(ctx context.Context, account *Account) bool {
	if s == nil || account == nil {
		return false
	}
	return isCarpoolSchedulingAccountAllowed(ctx, s.carpoolRepo, currentRequestGroupID(ctx), account)
}

func (s *OpenAIGatewayService) isAccountSchedulableForSchedulingRequest(ctx context.Context, account *Account) bool {
	if account == nil {
		return false
	}
	if account.IsSchedulable() {
		return true
	}
	if s.isCarpoolSchedulingAccount(ctx, account) {
		return isCarpoolAccountSchedulable(account)
	}
	return false
}

func (s *OpenAIGatewayService) isOpenAIAccountEligibleForSchedulingRequest(ctx context.Context, account *Account, platform string, requestedModel string, requireCompact bool, requiredCapability OpenAIEndpointCapability) bool {
	platform = normalizeOpenAICompatiblePlatform(platform)
	if account == nil || account.Platform != platform || !s.isAccountSchedulableForSchedulingRequest(ctx, account) || !account.IsOpenAICompatible() {
		return false
	}
	if account.IsGrok() {
		if paused, _ := shouldAutoPauseGrokAccountByQuota(account); paused {
			return false
		}
	}
	if requestedModel != "" && !account.IsModelSupported(requestedModel) {
		return false
	}
	if !account.SupportsOpenAIEndpointCapability(requiredCapability) {
		return false
	}
	if requireCompact && openAICompactSupportTier(account) == 0 {
		return false
	}
	return true
}

func (s *OpenAIGatewayService) shouldClearStickySessionForSchedulingRequest(ctx context.Context, account *Account, requestedModel string) bool {
	if account == nil {
		return false
	}
	if s.isCarpoolSchedulingAccount(ctx, account) {
		return !isCarpoolAccountSchedulable(account)
	}
	return shouldClearStickySession(account, requestedModel)
}

func (s *OpenAIGatewayService) shouldSkipPersistentRateLimitForCarpool(ctx context.Context, account *Account, statusCode int) bool {
	return statusCode == http.StatusTooManyRequests && s.isCarpoolSchedulingAccount(ctx, account)
}
