package service

import (
	"context"
	"log/slog"
	"time"
)

// TokenRefreshTempUnschedDuration exposes the upstream refresh cooldown to
// product extensions that perform the same recovery workflow.
const TokenRefreshTempUnschedDuration = tokenRefreshTempUnschedDuration

// IsNonRetryableRefreshError exposes the upstream classifier without copying
// its error-matching rules into product handlers.
func IsNonRetryableRefreshError(err error) bool {
	return isNonRetryableRefreshError(err)
}

// SetTempUnschedulable applies the same repository, cache, and runtime state
// updates as the upstream rate-limit paths.
func (s *RateLimitService) SetTempUnschedulable(ctx context.Context, account *Account, until time.Time, reason string) error {
	if s == nil || account == nil {
		return nil
	}
	if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason); err != nil {
		return err
	}
	s.notifyAccountSchedulingBlocked(account, until, "temp_unschedulable")
	if s.tempUnschedCache != nil {
		state := &TempUnschedState{
			UntilUnix:       until.Unix(),
			TriggeredAtUnix: time.Now().Unix(),
			ErrorMessage:    reason,
		}
		if err := s.tempUnschedCache.SetTempUnsched(ctx, account.ID, state); err != nil {
			slog.Warn("temp_unsched_cache_set_failed", "account_id", account.ID, "error", err)
		}
	}
	return nil
}
