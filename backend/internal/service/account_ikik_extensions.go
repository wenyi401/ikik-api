package service

import (
	"fmt"
	"strings"
	"time"

	"ikik-api/internal/domain"
)

const (
	PlatformCustom = domain.PlatformCustom
)

const (
	AccountLevelUnknown = domain.AccountLevelUnknown
	AccountLevelFree    = domain.AccountLevelFree
	AccountLevelPlus    = domain.AccountLevelPlus
	AccountLevelPro     = domain.AccountLevelPro
	AccountLevelTeam    = domain.AccountLevelTeam
	AccountLevelK12     = domain.AccountLevelK12
)

const (
	GroupScopePublic      = domain.GroupScopePublic
	GroupScopeUserPrivate = domain.GroupScopeUserPrivate
	GroupScopeUserCarpool = domain.GroupScopeUserCarpool
)

const (
	AccountShareModePrivate = "private"
	AccountShareModePublic  = "public"

	AccountShareStatusPending   = "pending"
	AccountShareStatusApproved  = "approved"
	AccountShareStatusSuspended = "suspended"
)

const (
	OAuthAccountDefaultConcurrency = 3
	OpenAIPlusDefaultConcurrency   = 3
	AccountMaxLoadFactor           = 10000
)

func NormalizeAccountLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case AccountLevelFree:
		return AccountLevelFree
	case AccountLevelPlus:
		return AccountLevelPlus
	case AccountLevelPro:
		return AccountLevelPro
	case AccountLevelTeam:
		return AccountLevelTeam
	case AccountLevelK12:
		return AccountLevelK12
	default:
		return AccountLevelUnknown
	}
}

func IsConcreteAccountLevel(level string) bool {
	switch NormalizeAccountLevel(level) {
	case AccountLevelFree, AccountLevelPlus, AccountLevelPro, AccountLevelTeam, AccountLevelK12:
		return true
	default:
		return false
	}
}

func IsOpenAIPlusAccount(platform, accountLevel string) bool {
	return platform == PlatformOpenAI && NormalizeAccountLevel(accountLevel) == AccountLevelPlus
}

func NormalizeOpenAIAccountLevel(platform, accountLevel string, credentials, extra map[string]any) string {
	level := NormalizeAccountLevel(accountLevel)
	if platform != PlatformOpenAI || IsConcreteAccountLevel(level) {
		return level
	}
	if inferred := InferOpenAIAccountLevel(credentials, extra); IsConcreteAccountLevel(inferred) {
		return inferred
	}
	return level
}

func InferOpenAIAccountLevel(credentials, extra map[string]any) string {
	for _, values := range []map[string]any{credentials, extra} {
		for _, key := range []string{"plan_type", "chatgpt_plan_type", "subscription_plan"} {
			raw, ok := values[key].(string)
			if !ok {
				continue
			}
			if inferred := NormalizeOpenAIPlanAccountLevel(raw); inferred != AccountLevelUnknown {
				return inferred
			}
		}
	}
	return AccountLevelUnknown
}

func NormalizeOpenAIPlanAccountLevel(planType string) string {
	if level := NormalizeAccountLevel(planType); IsConcreteAccountLevel(level) {
		return level
	}
	normalized := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(strings.ToLower(strings.TrimSpace(planType)))
	switch {
	case normalized == "chatgptfree":
		return AccountLevelFree
	case normalized == "chatgptplus" || strings.HasPrefix(normalized, "plus"):
		return AccountLevelPlus
	case normalized == "chatgptpro":
		return AccountLevelPro
	case normalized == "chatgptteam":
		return AccountLevelTeam
	case normalized == "chatgptk12" || normalized == "chatgptk":
		return AccountLevelK12
	default:
		return AccountLevelUnknown
	}
}

func NormalizeOpenAISharedPoolAccountLevel(level string) string {
	if normalized := NormalizeAccountLevel(level); normalized != AccountLevelUnknown {
		return normalized
	}
	return AccountLevelFree
}

func NormalizeRequiredAccountLevel(level string) string {
	normalized := NormalizeAccountLevel(level)
	if normalized == AccountLevelUnknown {
		return ""
	}
	return normalized
}

func IsValidRequiredAccountLevel(level string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(level))
	return trimmed == "" || IsConcreteAccountLevel(trimmed)
}

func NormalizeOpenAISharedPoolRequiredLevel(level string) string {
	return NormalizeRequiredAccountLevel(level)
}

func OpenAISharedPoolLevelRank(level string) int {
	switch NormalizeOpenAISharedPoolAccountLevel(level) {
	case AccountLevelFree:
		return 1
	case AccountLevelPlus:
		return 2
	case AccountLevelPro:
		return 3
	default:
		return 0
	}
}

func CanOpenAIAccountJoinSharedPool(accountLevel, requiredLevel string) bool {
	required := NormalizeOpenAISharedPoolRequiredLevel(requiredLevel)
	if required == "" {
		return true
	}
	account := NormalizeOpenAISharedPoolAccountLevel(accountLevel)
	if required == AccountLevelTeam || required == AccountLevelK12 {
		return account == required
	}
	accountRank := OpenAISharedPoolLevelRank(account)
	requiredRank := OpenAISharedPoolLevelRank(required)
	return accountRank > 0 && requiredRank > 0 && accountRank >= requiredRank
}

func OpenAISharedPoolAllowedAccountLevels(requiredLevel string) []string {
	required := NormalizeOpenAISharedPoolRequiredLevel(requiredLevel)
	if required == "" {
		return nil
	}
	if required == AccountLevelTeam || required == AccountLevelK12 {
		return []string{required}
	}
	if OpenAISharedPoolLevelRank(required) == 0 {
		return nil
	}
	levels := make([]string, 0, 5)
	if required == AccountLevelFree {
		levels = append(levels, AccountLevelUnknown)
	}
	for _, level := range []string{AccountLevelFree, AccountLevelPlus, AccountLevelPro, AccountLevelTeam, AccountLevelK12} {
		if CanOpenAIAccountJoinSharedPool(level, required) {
			levels = append(levels, level)
		}
	}
	return levels
}

func DefaultOAuthAccountConcurrencyForPlatform(platform string) int {
	if platform == PlatformOpenAI {
		return OpenAIPlusDefaultConcurrency
	}
	if platform == PlatformGrok {
		return 1
	}
	return OAuthAccountDefaultConcurrency
}

func NormalizeOpenAIPlusConcurrency(platform, accountLevel string, concurrency int) (int, error) {
	if IsOpenAIPlusAccount(platform, accountLevel) && concurrency <= 0 {
		return OpenAIPlusDefaultConcurrency, nil
	}
	return concurrency, nil
}

func ValidateOpenAIPlusConcurrency(platform, accountLevel string, concurrency int) error {
	if IsOpenAIPlusAccount(platform, accountLevel) && concurrency <= 0 {
		return fmt.Errorf("openai plus account concurrency must be > 0")
	}
	return nil
}

func ValidateAccountLoadFactor(loadFactor *int) error {
	if loadFactor != nil && *loadFactor > AccountMaxLoadFactor {
		return fmt.Errorf("load_factor must be <= %d", AccountMaxLoadFactor)
	}
	return nil
}

func NormalizeAccountShareMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), AccountShareModePublic) {
		return AccountShareModePublic
	}
	return AccountShareModePrivate
}

func NormalizeAccountShareStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case AccountShareStatusPending:
		return AccountShareStatusPending
	case AccountShareStatusSuspended:
		return AccountShareStatusSuspended
	default:
		return AccountShareStatusApproved
	}
}

func (a *Account) IsPublicShareApproved() bool {
	return a != nil && a.OwnerUserID != nil && NormalizeAccountShareMode(a.ShareMode) == AccountShareModePublic && NormalizeAccountShareStatus(a.ShareStatus) == AccountShareStatusApproved
}

func (a *Account) IsVisibleToConsumer(userID int64) bool {
	if a == nil {
		return false
	}
	if a.OwnerUserID == nil || (userID > 0 && *a.OwnerUserID == userID) {
		return true
	}
	return a.IsPublicShareApproved()
}

func (a *Account) IsRateLimitedAt(now time.Time) bool {
	return a != nil && a.RateLimitResetAt != nil && now.Before(*a.RateLimitResetAt)
}

func (a *Account) IsOverloadedAt(now time.Time) bool {
	return a != nil && a.OverloadUntil != nil && now.Before(*a.OverloadUntil)
}

func (a *Account) IsQuotaExceededAt(now time.Time) bool {
	if a == nil {
		return false
	}
	if limit := a.GetQuotaLimit(); limit > 0 && a.GetQuotaUsed() >= limit {
		return true
	}
	if limit := a.GetQuotaDailyLimit(); limit > 0 && !a.IsDailyQuotaPeriodExpiredAt(now) && a.GetQuotaDailyUsed() >= limit {
		return true
	}
	if limit := a.GetQuotaWeeklyLimit(); limit > 0 && !a.IsWeeklyQuotaPeriodExpiredAt(now) && a.GetQuotaWeeklyUsed() >= limit {
		return true
	}
	return false
}

func (a *Account) IsDailyQuotaPeriodExpiredAt(now time.Time) bool {
	start := a.getExtraTime("quota_daily_start")
	if a.GetQuotaDailyResetMode() == "fixed" {
		return a.isFixedDailyPeriodExpiredAt(start, now)
	}
	return isPeriodExpiredAt(start, 24*time.Hour, now)
}

func (a *Account) IsWeeklyQuotaPeriodExpiredAt(now time.Time) bool {
	start := a.getExtraTime("quota_weekly_start")
	if a.GetQuotaWeeklyResetMode() == "fixed" {
		return a.isFixedWeeklyPeriodExpiredAt(start, now)
	}
	return isPeriodExpiredAt(start, 7*24*time.Hour, now)
}

func (a *Account) isFixedDailyPeriodExpiredAt(periodStart, now time.Time) bool {
	if periodStart.IsZero() {
		return true
	}
	tz, err := time.LoadLocation(a.GetQuotaResetTimezone())
	if err != nil {
		tz = time.UTC
	}
	return periodStart.Before(lastFixedDailyReset(a.GetQuotaDailyResetHour(), tz, now))
}

func (a *Account) isFixedWeeklyPeriodExpiredAt(periodStart, now time.Time) bool {
	if periodStart.IsZero() {
		return true
	}
	tz, err := time.LoadLocation(a.GetQuotaResetTimezone())
	if err != nil {
		tz = time.UTC
	}
	return periodStart.Before(lastFixedWeeklyReset(a.GetQuotaWeeklyResetDay(), a.GetQuotaWeeklyResetHour(), tz, now))
}

func isPeriodExpiredAt(periodStart time.Time, duration time.Duration, now time.Time) bool {
	return periodStart.IsZero() || !now.Before(periodStart.Add(duration))
}
