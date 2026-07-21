package service

import (
	"context"

	"fmt"
	"strings"

	"time"
)

type AccountListFilters struct {
	Platform    string
	AccountType string
	Status      string
	Search      string
	GroupID     int64
	ProxyID     int64
	PrivacyMode string
}

func (s *AccountService) AutoRepairSuspectedOpenAIFreeAccount(ctx context.Context, accountID int64, maxWeeklyLimitUSD float64, reason string) (*Account, bool, error) {
	if s == nil || s.accountRepo == nil {
		return nil, false, ErrOwnedAccountGroupValidationUnavailable
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, false, fmt.Errorf("get account: %w", err)
	}
	if !ShouldRepairSuspectedOpenAIFreeAccount(account, maxWeeklyLimitUSD, time.Now()) {
		return account, false, nil
	}

	account.AccountLevel = AccountLevelFree
	if account.ShareMode == AccountShareModePublic {
		account.ShareStatus = AccountShareStatusSuspended
	}
	message := strings.TrimSpace(reason)
	if message == "" {
		message = "OpenAI Codex weekly quota exhausted under free-account threshold; public sharing suspended pending review"
	}
	account.ErrorMessage = message

	groupIDs := account.GroupIDs
	if account.OwnerUserID != nil {
		groupIDs, err = s.repairedOpenAIAccountGroupIDs(ctx, account)
		if err != nil {
			return nil, false, err
		}
	}
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, false, fmt.Errorf("update account suspected free repair: %w", err)
	}
	if account.OwnerUserID != nil {
		if err := s.accountRepo.BindGroups(ctx, account.ID, groupIDs); err != nil {
			return nil, false, fmt.Errorf("bind repaired account groups: %w", err)
		}
		account.GroupIDs = append([]int64(nil), groupIDs...)
	}
	return account, true, nil
}
func (s *AccountService) SetUserPrivateGroupProvisioner(provisioner UserPrivateGroupProvisioner) {
	if s == nil {
		return
	}
	s.privateGroupProvisioner = provisioner
}

func ShouldRepairSuspectedOpenAIFreeAccount(account *Account, maxWeeklyLimitUSD float64, now time.Time) bool {
	if account == nil || maxWeeklyLimitUSD <= 0 {
		return false
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return false
	}
	if OpenAISharedPoolLevelRank(account.AccountLevel) <= OpenAISharedPoolLevelRank(AccountLevelFree) {
		return false
	}
	weeklyLimit := account.GetQuotaWeeklyLimit()
	if weeklyLimit <= 0 || weeklyLimit > maxWeeklyLimitUSD {
		return false
	}
	progress := buildCodexUsageProgressFromExtra(account.Extra, "7d", now)
	if progress == nil || progress.Utilization < 100 {
		return false
	}
	if progress.ResetsAt != nil && now.After(*progress.ResetsAt) {
		return false
	}
	return true
}

type accountSubscriptionLookupRepository interface {
	GetActiveByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
}
type accountUserRepository interface {
	GetByID(ctx context.Context, id int64) (*User, error)
}

func hasNonEmptyStringField(values map[string]any, key string) bool {
	if len(values) == 0 {
		return false
	}
	value, ok := values[key]
	if !ok {
		return false
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func isOAuthOnlyGroup(group *Group) bool {
	if group == nil || !group.RequireOAuthOnly {
		return false
	}
	switch group.Platform {
	case PlatformOpenAI, PlatformAntigravity, PlatformAnthropic, PlatformGemini, PlatformGrok:
		return true
	default:
		return false
	}
}
func mergeAccountMap(current map[string]any, updates map[string]any) map[string]any {
	if len(current) == 0 && len(updates) == 0 {
		return nil
	}
	next := make(map[string]any, len(current)+len(updates))
	for key, value := range current {
		next[key] = value
	}
	for key, value := range updates {
		next[key] = value
	}
	return next
}

func normalizeGroupIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrGroupNotFound
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}
func normalizeLoadFactor(value *int) *int {
	if value == nil || *value <= 0 {
		return nil
	}
	normalized := *value
	return &normalized
}

func (s *AccountService) repairedOpenAIAccountGroupIDs(ctx context.Context, account *Account) ([]int64, error) {
	if account == nil || account.OwnerUserID == nil {
		return nil, ErrAccountNotFound
	}
	privateGroup, err := s.getPrivateGroupForOwnedAccount(ctx, *account.OwnerUserID, account.Platform)
	if err != nil {
		return nil, err
	}
	groupIDs := []int64{privateGroup.ID}
	if s.groupRepo == nil {
		return normalizeGroupIDs(groupIDs)
	}
	groups, err := s.groupRepo.ListActiveByPlatform(ctx, account.Platform)
	if err != nil {
		return nil, fmt.Errorf("list public share groups: %w", err)
	}
	for i := range groups {
		group := groups[i]
		if !isOwnedPublicSharePoolGroup(&group, account.Platform) {
			continue
		}
		if NormalizeOpenAISharedPoolRequiredLevel(group.RequiredAccountLevel) == AccountLevelFree {
			groupIDs = append(groupIDs, group.ID)
			break
		}
	}
	return normalizeGroupIDs(groupIDs)
}

func requiresOAuthOnlyGroupCheck(accountType string) bool {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case AccountTypeOAuth, AccountTypeSetupToken:
		return false
	default:
		return true
	}
}
