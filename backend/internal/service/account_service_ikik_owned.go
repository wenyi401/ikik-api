package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"time"

	"ikik-api/internal/pkg/antigravity"
	"ikik-api/internal/pkg/claude"
	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/geminicli"
	"ikik-api/internal/pkg/kiro"
	"ikik-api/internal/pkg/openai"
	"ikik-api/internal/pkg/openai_compat"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/pkg/xai"
)

func (s *AccountService) ApproveOwnedPublicShare(ctx context.Context, ownerUserID, accountID int64) (*Account, error) {
	return s.ApproveOwnedPublicShareWithOptions(ctx, ownerUserID, accountID, OwnedPublicShareApprovalOptions{})
}

func (s *AccountService) ApproveOwnedPublicShareWithOptions(ctx context.Context, ownerUserID, accountID int64, opts OwnedPublicShareApprovalOptions) (*Account, error) {
	account, err := s.GetOwnedByID(ctx, ownerUserID, accountID)
	if err != nil {
		return nil, err
	}
	if ownedAccountForcesPrivateShare(account.Type) {
		return nil, ErrOwnedAccountAPIKeyPublicShareNotAllowed
	}
	if err := validateOwnedAccountSourceForPlatform(account.Platform, account.Type, account.Credentials, account.Extra); err != nil {
		return nil, err
	}
	if !isOwnedAccountPublicShareApprovable(account, opts.AllowRateLimited) {
		return nil, ErrOwnedAccountPublicValidationFailed.WithMetadata(map[string]string{
			"reason": "account is not active or schedulable",
		})
	}

	publicGroup, err := s.resolveOwnedPublicShareGroup(ctx, account)
	if err != nil {
		return nil, err
	}
	if err := s.validateOwnedPublicSharePolicy(ctx, account, publicGroup); err != nil {
		return nil, err
	}
	groupIDs, err := s.publicOwnedAccountGroupIDs(ctx, ownerUserID, account, publicGroup)
	if err != nil {
		return nil, err
	}

	account.ShareMode = AccountShareModePublic
	account.ShareStatus = AccountShareStatusApproved
	account.ErrorMessage = ""
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, fmt.Errorf("update account public share status: %w", err)
	}
	if err := s.accountRepo.BindGroups(ctx, account.ID, groupIDs); err != nil {
		return nil, fmt.Errorf("bind public account groups: %w", err)
	}
	account.GroupIDs = append([]int64(nil), groupIDs...)
	return account, nil
}

// Delete 删除账号
// 优化：使用 ExistsByID 替代 GetByID 进行存在性检查，
// 避免加载完整账号对象及其关联数据，提升删除操作的性能
func (s *AccountService) BulkDeleteOwned(ctx context.Context, ownerUserID int64, accountIDs []int64) (*BulkUpdateAccountsResult, error) {
	if ownerUserID <= 0 {
		return nil, ErrUserNotFound
	}
	ids := normalizeOwnedBulkAccountIDs(accountIDs)
	result := &BulkUpdateAccountsResult{
		SuccessIDs: make([]int64, 0, len(ids)),
		FailedIDs:  make([]int64, 0, len(ids)),
		Results:    make([]BulkUpdateAccountResult, 0, len(ids)),
	}
	for _, accountID := range ids {
		entry := BulkUpdateAccountResult{AccountID: accountID}
		if err := s.DeleteOwned(ctx, ownerUserID, accountID); err != nil {
			entry.Error = err.Error()
			result.Failed++
			result.FailedIDs = append(result.FailedIDs, accountID)
		} else {
			entry.Success = true
			result.Success++
			result.SuccessIDs = append(result.SuccessIDs, accountID)
		}
		result.Results = append(result.Results, entry)
	}
	return result, nil
}

func (s *AccountService) BulkUpdateOwned(ctx context.Context, ownerUserID int64, input *BulkUpdateOwnedAccountsInput) (*BulkUpdateAccountsResult, error) {
	if ownerUserID <= 0 {
		return nil, ErrUserNotFound
	}
	if input == nil {
		return nil, ErrAccountNilInput
	}

	accountIDs := normalizeOwnedBulkAccountIDs(input.AccountIDs)
	result := &BulkUpdateAccountsResult{
		SuccessIDs: make([]int64, 0, len(accountIDs)),
		FailedIDs:  make([]int64, 0, len(accountIDs)),
		Results:    make([]BulkUpdateAccountResult, 0, len(accountIDs)),
	}
	if len(accountIDs) == 0 {
		return result, nil
	}

	if input.Concurrency != nil && *input.Concurrency <= 0 {
		return nil, fmt.Errorf("concurrency must be > 0")
	}
	if input.Priority != nil && *input.Priority <= 0 {
		return nil, fmt.Errorf("priority must be > 0")
	}
	if err := ValidateAccountLoadFactor(input.LoadFactor); err != nil {
		return nil, err
	}
	if input.GroupIDs != nil {
		return nil, ErrGroupNotAllowed
	}
	if input.AccountLevel != nil {
		return nil, ErrOwnedAccountLevelNotAllowed
	}
	status, err := normalizeOwnedBulkStatus(input.Status)
	if err != nil {
		return nil, err
	}
	shareMode := ""
	if input.ShareMode != nil {
		shareMode = NormalizeAccountShareMode(*input.ShareMode)
	}

	accounts, err := s.accountRepo.GetByIDs(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("get accounts: %w", err)
	}
	accountsByID := make(map[int64]*Account, len(accounts))
	for _, account := range accounts {
		if account != nil {
			accountsByID[account.ID] = account
		}
	}

	if input.Concurrency != nil {
		input.Concurrency = nil
	}
	if input.LoadFactor != nil {
		input.LoadFactor = nil
	}
	if input.Credentials == nil {
		input.Credentials = map[string]any{}
	}
	if input.Extra == nil {
		input.Extra = map[string]any{}
	}
	updatedIdentityAccounts := make([]*Account, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		account := accountsByID[accountID]
		if account == nil || account.OwnerUserID == nil || *account.OwnerUserID != ownerUserID {
			return nil, ErrAccountNotFound
		}

		nextCredentials := mergeAccountMap(account.Credentials, input.Credentials)
		nextExtra := mergeAccountMap(account.Extra, input.Extra)
		nextCredentials, nextExtra = applyOwnedPersonalAccountTemplateToMaps(account.Platform, nextCredentials, nextExtra)
		nextAccount := *account
		nextAccount.Credentials = nextCredentials
		nextAccount.Extra = nextExtra
		if err := validateOwnedAccountSourceForPlatform(account.Platform, account.Type, nextCredentials, nextExtra); err != nil {
			return nil, err
		}
		nextConcurrency := ownedPersonalDefaultConcurrency
		nextLoadFactor := (*int)(nil)
		nextAccountLevel := NormalizeOpenAIAccountLevel(account.Platform, account.AccountLevel, nextCredentials, nextExtra)
		if err := ValidateOpenAIPlusConcurrency(account.Platform, nextAccountLevel, nextConcurrency); err != nil {
			return nil, err
		}
		if err := ValidateAccountLoadFactor(nextLoadFactor); err != nil {
			return nil, err
		}
		if len(input.Credentials) > 0 || len(input.Extra) > 0 {
			if err := s.ensureOwnedAccountNotDuplicate(ctx, ownerUserID, &nextAccount, accountIDs...); err != nil {
				return nil, err
			}
			updatedIdentityAccounts = append(updatedIdentityAccounts, &nextAccount)
		}
	}
	if err := ensureOwnedAccountBatchNotDuplicate(updatedIdentityAccounts); err != nil {
		return nil, err
	}

	requiresPerAccountUpdate := input.ProxyID != nil || shareMode != "" || len(input.Credentials) > 0 || len(input.Extra) > 0
	if requiresPerAccountUpdate {
		for _, accountID := range accountIDs {
			account := accountsByID[accountID]
			entry := BulkUpdateAccountResult{AccountID: accountID}
			updateReq := UpdateAccountRequest{
				Concurrency: nil,
				LoadFactor:  nil,
				Priority:    input.Priority,
				Schedulable: input.Schedulable,
				ProxyID:     input.ProxyID,
			}
			if status != "" {
				updateReq.Status = &status
			}
			if shareMode != "" {
				updateReq.ShareMode = &shareMode
			}
			if len(input.Credentials) > 0 {
				credentials := mergeAccountMap(account.Credentials, input.Credentials)
				credentials, _ = applyOwnedPersonalAccountTemplateToMaps(account.Platform, credentials, account.Extra)
				updateReq.Credentials = &credentials
			}
			if len(input.Extra) > 0 {
				extra := mergeAccountMap(account.Extra, input.Extra)
				_, extra = applyOwnedPersonalAccountTemplateToMaps(account.Platform, account.Credentials, extra)
				updateReq.Extra = &extra
			}
			if _, err := s.UpdateOwned(ctx, ownerUserID, accountID, updateReq); err != nil {
				entry.Error = err.Error()
				result.Failed++
				result.FailedIDs = append(result.FailedIDs, accountID)
				result.Results = append(result.Results, entry)
				continue
			}
			entry.Success = true
			result.Success++
			result.SuccessIDs = append(result.SuccessIDs, accountID)
			result.Results = append(result.Results, entry)
		}
		return result, nil
	}

	repoUpdates := AccountBulkUpdate{
		Concurrency: nil,
		Priority:    input.Priority,
		LoadFactor:  nil,
		Schedulable: input.Schedulable,
		Credentials: map[string]any{},
		Extra:       map[string]any{},
	}
	if status != "" {
		repoUpdates.Status = &status
	}

	updated, err := s.accountRepo.BulkUpdate(ctx, accountIDs, repoUpdates)
	if err != nil {
		return nil, fmt.Errorf("bulk update owned accounts: %w", err)
	}
	if updated != int64(len(accountIDs)) {
		return nil, ErrAccountNotFound
	}
	for _, accountID := range accountIDs {
		entry := BulkUpdateAccountResult{AccountID: accountID, Success: true}
		result.Success++
		result.SuccessIDs = append(result.SuccessIDs, accountID)
		result.Results = append(result.Results, entry)
	}

	return result, nil
}

type BulkUpdateOwnedAccountsInput struct {
	AccountIDs   []int64
	Concurrency  *int
	Priority     *int
	LoadFactor   *int
	Status       string
	Schedulable  *bool
	AccountLevel *string
	ShareMode    *string
	GroupIDs     *[]int64
	Credentials  map[string]any
	Extra        map[string]any
	ProxyID      *int64
}

func (s *AccountService) CreateOwned(ctx context.Context, ownerUserID int64, req CreateAccountRequest) (*Account, error) {
	return s.createOwned(ctx, ownerUserID, req)
}

func (s *AccountService) DeleteOwned(ctx context.Context, ownerUserID, accountID int64) error {
	if _, err := s.GetOwnedByID(ctx, ownerUserID, accountID); err != nil {
		return err
	}
	if err := s.accountRepo.Delete(ctx, accountID); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

var (
	ErrOwnedAccountAPIKeyPublicShareNotAllowed = infraerrors.BadRequest("OWNED_ACCOUNT_APIKEY_PUBLIC_SHARE_NOT_ALLOWED", "user API key accounts can only be private")
)
var (
	ErrOwnedAccountAlreadyExists = infraerrors.Conflict("OWNED_ACCOUNT_ALREADY_EXISTS", "account already exists")
)
var (
	ErrOwnedAccountCredentialsInvalid = infraerrors.BadRequest("OWNED_ACCOUNT_CREDENTIALS_INVALID", "account credentials are invalid")
)
var (
	ErrOwnedAccountCredentialsNotAllowed = infraerrors.BadRequest("OWNED_ACCOUNT_CREDENTIALS_NOT_ALLOWED", "user OAuth accounts cannot include API keys, custom URLs, upstream endpoints, cookies or manual session credentials")
)
var (
	ErrOwnedAccountGroupPlatformMismatch = infraerrors.BadRequest("OWNED_ACCOUNT_GROUP_PLATFORM_MISMATCH", "account group platform does not match account platform")
)
var (
	ErrOwnedAccountGroupValidationUnavailable = infraerrors.InternalServer("OWNED_ACCOUNT_GROUP_VALIDATION_UNAVAILABLE", "owned account group validation is unavailable")
)
var (
	ErrOwnedAccountLevelNotAllowed = infraerrors.BadRequest("OWNED_ACCOUNT_LEVEL_NOT_ALLOWED", "user account level is detected automatically and cannot be set manually")
)
var (
	ErrOwnedAccountPublicPolicyUnavailable = infraerrors.BadRequest("OWNED_ACCOUNT_PUBLIC_POLICY_UNAVAILABLE", "account share policy is not configured for this public account pool")
)
var (
	ErrOwnedAccountPublicPoolUnavailable = infraerrors.BadRequest("OWNED_ACCOUNT_PUBLIC_POOL_UNAVAILABLE", "public shared account pool group is not configured for this account platform")
)
var (
	ErrOwnedAccountPublicValidationFailed = infraerrors.BadRequest("OWNED_ACCOUNT_PUBLIC_VALIDATION_FAILED", "public account validation failed")
)
var (
	ErrOwnedAccountTypeNotAllowed = infraerrors.BadRequest("OWNED_ACCOUNT_TYPE_NOT_ALLOWED", "unsupported user account type")
)

func (s *AccountService) GetOwnedByID(ctx context.Context, ownerUserID, accountID int64) (*Account, error) {
	if ownerUserID <= 0 {
		return nil, ErrUserNotFound
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	if account.OwnerUserID == nil || *account.OwnerUserID != ownerUserID {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

func (s *AccountService) ImportOwned(ctx context.Context, ownerUserID int64, req CreateAccountRequest) (*Account, error) {
	return s.createOwned(ctx, ownerUserID, req)
}
func (s *AccountService) ListOwned(ctx context.Context, ownerUserID int64, params pagination.PaginationParams, filters AccountListFilters) ([]Account, *pagination.PaginationResult, error) {
	if ownerUserID <= 0 {
		return nil, nil, ErrUserNotFound
	}
	repo, ok := s.accountRepo.(ownedAccountFilterRepository)
	if !ok {
		return nil, nil, fmt.Errorf("owned account listing is not supported by repository")
	}
	accounts, result, err := repo.ListOwnedWithFilters(ctx, ownerUserID, params, filters.Platform, filters.AccountType, filters.Status, filters.Search, filters.GroupID, filters.ProxyID, filters.PrivacyMode)
	if err != nil {
		return nil, nil, fmt.Errorf("list owned accounts: %w", err)
	}
	return accounts, result, nil
}

func (s *AccountService) ListOwnedProxies(ctx context.Context, ownerUserID int64) ([]ProxyWithAccountCount, error) {
	if ownerUserID <= 0 {
		return nil, ErrUserNotFound
	}
	repo, err := s.userPrivateProxyRepo()
	if err != nil {
		return nil, err
	}
	return repo.ListOwnedByUserID(ctx, ownerUserID)
}

func (s *AccountService) MarkOwnedPublicSharePending(ctx context.Context, ownerUserID, accountID int64, reason string) (*Account, error) {
	account, err := s.GetOwnedByID(ctx, ownerUserID, accountID)
	if err != nil {
		return nil, err
	}
	groupIDs, err := s.initialOwnedAccountGroupIDs(ctx, ownerUserID, account, nil)
	if err != nil {
		return nil, err
	}
	account.ShareMode = AccountShareModePublic
	account.ShareStatus = AccountShareStatusPending
	account.ErrorMessage = strings.TrimSpace(reason)
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, fmt.Errorf("update account public share status: %w", err)
	}
	if err := s.accountRepo.BindGroups(ctx, account.ID, groupIDs); err != nil {
		return nil, fmt.Errorf("bind pending account groups: %w", err)
	}
	account.GroupIDs = append([]int64(nil), groupIDs...)
	return account, nil
}

type OwnedPublicShareApprovalOptions struct {
	AllowRateLimited bool
}

func (s *AccountService) SetAccountSharePolicyRepository(repo AccountSharePolicyRepository) {
	if s == nil {
		return
	}
	s.accountSharePolicyRepo = repo
}

func (s *AccountService) UpdateOwned(ctx context.Context, ownerUserID, accountID int64, req UpdateAccountRequest) (*Account, error) {
	if req.AccountLevel != nil {
		return nil, ErrOwnedAccountLevelNotAllowed
	}
	account, err := s.GetOwnedByID(ctx, ownerUserID, accountID)
	if err != nil {
		return nil, err
	}
	applyOwnedPersonalAccountTemplateToUpdate(account, &req)

	if req.Name != nil {
		account.Name = *req.Name
	}
	if req.Notes != nil {
		account.Notes = normalizeAccountNotes(req.Notes)
	}
	if req.Credentials != nil {
		account.Credentials = *req.Credentials
	}
	if req.Extra != nil {
		account.Extra = *req.Extra
	}
	if req.ProxyID != nil {
		proxyID, err := s.ValidateOwnedProxyID(ctx, ownerUserID, req.ProxyID)
		if err != nil {
			return nil, err
		}
		account.ProxyID = proxyID
	}
	if req.Concurrency != nil {
		account.Concurrency = *req.Concurrency
	}
	if req.LoadFactor != nil {
		account.LoadFactor = normalizeLoadFactor(req.LoadFactor)
	}
	account.AccountLevel = NormalizeOpenAIAccountLevel(account.Platform, account.AccountLevel, account.Credentials, account.Extra)
	if err := ValidateOpenAIPlusConcurrency(account.Platform, account.AccountLevel, account.Concurrency); err != nil {
		return nil, err
	}
	if err := ValidateAccountLoadFactor(account.LoadFactor); err != nil {
		return nil, err
	}
	if req.Priority != nil {
		account.Priority = *req.Priority
	}
	if req.Status != nil {
		switch *req.Status {
		case StatusActive, StatusDisabled:
			account.Status = *req.Status
		default:
			return nil, fmt.Errorf("invalid account status: %s", *req.Status)
		}
	}
	if req.Schedulable != nil {
		account.Schedulable = *req.Schedulable
	}
	if req.ClearExpiresAt {
		account.ExpiresAt = nil
	} else if req.ExpiresAt != nil {
		account.ExpiresAt = req.ExpiresAt
	}
	if req.AutoPauseOnExpired != nil {
		account.AutoPauseOnExpired = *req.AutoPauseOnExpired
	}
	shouldBindGroups := false
	var groupIDs []int64
	if ownedAccountForcesPrivateShare(account.Type) && req.ShareMode != nil && NormalizeAccountShareMode(*req.ShareMode) == AccountShareModePublic {
		return nil, ErrOwnedAccountAPIKeyPublicShareNotAllowed
	}
	if ownedAccountForcesPrivateShare(account.Type) && NormalizeAccountShareMode(account.ShareMode) == AccountShareModePublic {
		managedGroupIDs, err := s.managedOwnedAccountGroupIDsForShareMode(ctx, ownerUserID, account, AccountShareModePrivate)
		if err != nil {
			return nil, err
		}
		account.ShareMode = AccountShareModePrivate
		account.ShareStatus = AccountShareStatusApproved
		account.ErrorMessage = ""
		groupIDs = managedGroupIDs
		shouldBindGroups = true
	}
	if !shouldBindGroups && req.ShareMode != nil {
		nextMode := NormalizeAccountShareMode(*req.ShareMode)
		managedGroupIDs, err := s.managedOwnedAccountGroupIDsForShareMode(ctx, ownerUserID, account, nextMode)
		if err != nil {
			return nil, err
		}
		if nextMode == AccountShareModePrivate {
			account.ShareMode = AccountShareModePrivate
			account.ShareStatus = AccountShareStatusApproved
			account.ErrorMessage = ""
		} else if account.IsPublicShareApproved() {
			account.ShareMode = AccountShareModePublic
		} else {
			account.ShareMode = AccountShareModePublic
			account.ShareStatus = AccountShareStatusPending
		}
		groupIDs = managedGroupIDs
		shouldBindGroups = true
	}
	if err := validateOwnedAccountSourceForPlatform(account.Platform, account.Type, account.Credentials, account.Extra); err != nil {
		return nil, err
	}
	if req.Credentials != nil || req.Extra != nil {
		if err := s.ensureOwnedAccountNotDuplicate(ctx, ownerUserID, account, account.ID); err != nil {
			return nil, err
		}
	}

	if !shouldBindGroups && req.GroupIDs != nil {
		return nil, ErrGroupNotAllowed
	}
	if !shouldBindGroups && account.IsPublicShareApproved() && (req.AccountLevel != nil || req.Credentials != nil || req.Extra != nil) {
		publicGroup, err := s.resolveOwnedPublicShareGroup(ctx, account)
		if err != nil {
			return nil, err
		}
		groupIDs, err = s.publicOwnedAccountGroupIDs(ctx, ownerUserID, account, publicGroup)
		if err != nil {
			return nil, err
		}
		shouldBindGroups = true
	}
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, fmt.Errorf("update account: %w", err)
	}
	if shouldBindGroups {
		if err := s.accountRepo.BindGroups(ctx, account.ID, groupIDs); err != nil {
			return nil, fmt.Errorf("bind groups: %w", err)
		}
		account.GroupIDs = append([]int64(nil), groupIDs...)
	}
	return account, nil
}

func accountDuplicateIdentityKeys(account *Account) []ownedAccountDuplicateKey {
	if account == nil {
		return nil
	}
	keys := make([]ownedAccountDuplicateKey, 0, 3)
	add := func(name, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		keys = append(keys, ownedAccountDuplicateKey{Name: name, Value: value})
	}
	addFolded := func(name, value string) {
		add(name, strings.ToLower(strings.TrimSpace(value)))
	}
	switch account.Platform {
	case PlatformOpenAI:
		if account.Type != AccountTypeOAuth {
			return nil
		}
		if account.IsOpenAIAgentIdentity() {
			accountID := strings.TrimSpace(account.GetChatGPTAccountID())
			userID := strings.TrimSpace(account.GetChatGPTUserID())
			if accountID != "" && userID != "" {
				add("openai.agent_identity", accountID+"|"+userID)
			} else if userID != "" {
				add("openai.chatgpt_user_id", userID)
			} else {
				add("openai.chatgpt_account_id", accountID)
			}
			return keys
		}
		if chatgptUserID := account.GetChatGPTUserID(); chatgptUserID != "" {
			add("openai.chatgpt_user_id", chatgptUserID)
		} else if email := account.GetCredential("email"); email != "" {
			addFolded("openai.email", email)
		} else {
			add("openai.chatgpt_account_id", account.GetChatGPTAccountID())
		}
	case PlatformAnthropic:
		if account.Type != AccountTypeOAuth {
			return nil
		}
		orgUUID := strings.ToLower(strings.TrimSpace(account.GetClaudeOrgUUID()))
		accountUUID := strings.ToLower(strings.TrimSpace(account.GetClaudeAccountUUID()))
		if orgUUID != "" && accountUUID != "" {
			add("anthropic.org_account", orgUUID+"|"+accountUUID)
		} else if accountUUID != "" {
			add("anthropic.account_uuid", accountUUID)
		} else {
			add("anthropic.org_uuid", orgUUID)
		}
		if len(keys) == 0 {
			addFolded("anthropic.email_address", account.GetCredential("email_address"))
		}
	case PlatformGemini:
		if account.Type != AccountTypeOAuth {
			return nil
		}
		projectID := strings.ToLower(strings.TrimSpace(account.GetCredential("project_id")))
		oauthType := strings.TrimSpace(account.GeminiOAuthType())
		if projectID != "" {
			if oauthType == "" {
				oauthType = "code_assist"
			}
			add("gemini.project", strings.ToLower(oauthType)+"|"+projectID)
		}
	case PlatformAntigravity:
		if account.Type != AccountTypeOAuth {
			return nil
		}
		addFolded("antigravity.project_id", account.GetCredential("project_id"))
		if len(keys) == 0 {
			addFolded("antigravity.email", account.GetCredential("email"))
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return keys
}
func applyOwnedPersonalAccountTemplateToCreate(req *CreateAccountRequest) {
	if req == nil {
		return
	}
	if req.Concurrency <= 0 {
		req.Concurrency = ownedPersonalDefaultConcurrency
	}
	if req.Priority <= 0 {
		req.Priority = ownedPersonalDefaultPriority
	}
	if req.AutoPauseOnExpired == nil {
		autoPause := true
		req.AutoPauseOnExpired = &autoPause
	}
	req.GroupIDs = nil
	req.Credentials, req.Extra = applyOwnedPersonalAccountTemplateToMaps(req.Platform, req.Credentials, req.Extra)
}
func applyOwnedPersonalAccountTemplateToMaps(platform string, credentials, extra map[string]any) (map[string]any, map[string]any) {
	nextCredentials := make(map[string]any, len(credentials)+1)
	for key, value := range credentials {
		nextCredentials[key] = value
	}
	if !hasOwnedPersonalModelMapping(nextCredentials) {
		nextCredentials["model_mapping"] = ownedPersonalDefaultModelMapping(platform)
	}
	delete(nextCredentials, "compact_model_mapping")

	nextExtra := make(map[string]any, len(extra)+6)
	for key, value := range extra {
		nextExtra[key] = value
	}
	if platform == PlatformOpenAI {
		nextExtra["openai_oauth_responses_websockets_v2_mode"] = ownedPersonalDefaultOpenAIWSMode
		nextExtra["openai_oauth_responses_websockets_v2_enabled"] = false
		nextExtra["openai_passthrough"] = false
		nextExtra["openai_oauth_passthrough"] = false
		nextExtra["codex_cli_only"] = false
		nextExtra["openai_compact_mode"] = ownedPersonalDefaultOpenAICompactMode
		delete(nextExtra, "responses_websockets_v2_enabled")
		delete(nextExtra, "openai_ws_enabled")
	}
	if platform == PlatformKiro {
		nextExtra[openai_compat.ExtraKeyResponsesSupported] = false
	}
	return nextCredentials, nextExtra
}

func applyOwnedPersonalAccountTemplateToUpdate(account *Account, req *UpdateAccountRequest) {
	if account == nil || req == nil {
		return
	}
	nextShareMode := NormalizeAccountShareMode(account.ShareMode)
	if req.ShareMode != nil {
		nextShareMode = NormalizeAccountShareMode(*req.ShareMode)
	}
	if nextShareMode == AccountShareModePublic {
		concurrency := ownedPersonalDefaultConcurrency
		req.Concurrency = &concurrency
		req.LoadFactor = nil
	}
	if req.AutoPauseOnExpired == nil {
		autoPause := true
		req.AutoPauseOnExpired = &autoPause
	}
	req.GroupIDs = nil
	priority := ownedPersonalDefaultPriority
	if req.Priority != nil && *req.Priority > 0 {
		priority = *req.Priority
	}
	req.Priority = &priority
	credentials := account.Credentials
	if req.Credentials != nil {
		credentials = mergeAccountMap(account.Credentials, *req.Credentials)
	}
	extra := account.Extra
	if req.Extra != nil {
		extra = mergeAccountMap(account.Extra, *req.Extra)
	}
	nextCredentials, nextExtra := applyOwnedPersonalAccountTemplateToMaps(account.Platform, credentials, extra)
	req.Credentials = &nextCredentials
	req.Extra = &nextExtra
}

func (s *AccountService) canUserBindOwnedAccountGroup(ctx context.Context, user *User, group *Group) (bool, error) {
	if user == nil || group == nil {
		return false, nil
	}
	if user.IsGroupBlocked(group.ID) {
		return false, nil
	}
	if group.IsSubscriptionType() {
		if s.userSubRepo == nil {
			return false, ErrOwnedAccountGroupValidationUnavailable
		}
		_, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, user.ID, group.ID)
		if err == nil {
			return true, nil
		}
		if errors.Is(err, ErrSubscriptionNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("get active subscription: %w", err)
	}
	return user.CanBindGroup(group.ID, group.IsExclusive), nil
}
func (s *AccountService) createOwned(ctx context.Context, ownerUserID int64, req CreateAccountRequest) (*Account, error) {
	return s.createOwnedWithOptions(ctx, ownerUserID, req, ownedAccountCreateOptions{
		validateProxyID: s.ValidateOwnedProxyID,
		schedulable:     true,
	})
}

type ownedAccountProxyValidator func(context.Context, int64, *int64) (*int64, error)

type ownedAccountCreateOptions struct {
	validateProxyID ownedAccountProxyValidator
	schedulable     bool
}

func (s *AccountService) createOwnedWithOptions(
	ctx context.Context,
	ownerUserID int64,
	req CreateAccountRequest,
	opts ownedAccountCreateOptions,
) (*Account, error) {
	if ownerUserID <= 0 {
		return nil, ErrUserNotFound
	}

	req.AccountLevel = AccountLevelUnknown
	applyOwnedPersonalAccountTemplateToCreate(&req)
	if err := validateOwnedAccountSourceForPlatform(req.Platform, req.Type, req.Credentials, req.Extra); err != nil {
		return nil, err
	}
	proxyID, err := opts.validateProxyID(ctx, ownerUserID, req.ProxyID)
	if err != nil {
		return nil, err
	}
	req.ProxyID = proxyID
	shareMode := NormalizeAccountShareMode(req.ShareMode)
	if ownedAccountForcesPrivateShare(req.Type) {
		shareMode = AccountShareModePrivate
	}
	if shareMode == AccountShareModePublic {
		req.Concurrency = ownedPersonalDefaultConcurrency
		req.LoadFactor = nil
		req.Priority = ownedPersonalDefaultPriority
	}

	shareStatus := AccountShareStatusApproved
	if shareMode == AccountShareModePublic {
		shareStatus = AccountShareStatusPending
	}

	account := &Account{
		Name:               req.Name,
		Notes:              normalizeAccountNotes(req.Notes),
		Platform:           req.Platform,
		AccountLevel:       NormalizeOpenAIAccountLevel(req.Platform, req.AccountLevel, req.Credentials, req.Extra),
		Type:               req.Type,
		Credentials:        req.Credentials,
		Extra:              req.Extra,
		OwnerUserID:        &ownerUserID,
		ShareMode:          shareMode,
		ShareStatus:        shareStatus,
		ProxyID:            req.ProxyID,
		Concurrency:        req.Concurrency,
		LoadFactor:         normalizeLoadFactor(req.LoadFactor),
		Priority:           req.Priority,
		Status:             StatusActive,
		ExpiresAt:          req.ExpiresAt,
		AutoPauseOnExpired: true,
		Schedulable:        opts.schedulable,
	}
	if req.AutoPauseOnExpired != nil {
		account.AutoPauseOnExpired = *req.AutoPauseOnExpired
	}
	concurrency, err := NormalizeOpenAIPlusConcurrency(account.Platform, account.AccountLevel, account.Concurrency)
	if err != nil {
		return nil, err
	}
	account.Concurrency = concurrency
	if err := ValidateAccountLoadFactor(account.LoadFactor); err != nil {
		return nil, err
	}
	if err := s.ensureOwnedAccountNotDuplicate(ctx, ownerUserID, account, 0); err != nil {
		return nil, err
	}

	groupIDs, err := s.initialOwnedAccountGroupIDs(ctx, ownerUserID, account, req.GroupIDs)
	if err != nil {
		return nil, err
	}

	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}
	if len(groupIDs) > 0 {
		if err := s.accountRepo.BindGroups(ctx, account.ID, groupIDs); err != nil {
			return nil, fmt.Errorf("bind groups: %w", err)
		}
		account.GroupIDs = append([]int64(nil), groupIDs...)
	}
	return account, nil
}

func duplicateOwnedAccountError(platform string, key ownedAccountDuplicateKey, existingAccountID int64) error {
	return ErrOwnedAccountAlreadyExists.WithMetadata(map[string]string{
		"platform":            platform,
		"identity":            key.Name,
		"existing_account_id": fmt.Sprintf("%d", existingAccountID),
	})
}

func ensureOwnedAccountBatchNotDuplicate(accounts []*Account) error {
	seen := make(map[ownedAccountDuplicateKey]int64)
	for _, account := range accounts {
		if account == nil {
			continue
		}
		for _, key := range accountDuplicateIdentityKeys(account) {
			if existingID, ok := seen[key]; ok && existingID != account.ID {
				return duplicateOwnedAccountError(account.Platform, key, existingID)
			}
			seen[key] = account.ID
		}
	}
	return nil
}
func (s *AccountService) ensureOwnedAccountNotDuplicate(ctx context.Context, ownerUserID int64, candidate *Account, skipAccountIDs ...int64) error {
	candidateKeys := accountDuplicateIdentityKeys(candidate)
	if len(candidateKeys) == 0 {
		return nil
	}
	skipIDs := make(map[int64]struct{}, len(skipAccountIDs))
	for _, id := range skipAccountIDs {
		if id > 0 {
			skipIDs[id] = struct{}{}
		}
	}
	repo, ok := s.accountRepo.(ownedAccountFilterRepository)
	if !ok {
		return ErrOwnedAccountGroupValidationUnavailable
	}
	page := 1
	for {
		accounts, result, err := repo.ListOwnedWithFilters(
			ctx,
			ownerUserID,
			pagination.PaginationParams{Page: page, PageSize: 1000, SortBy: "id", SortOrder: pagination.SortOrderAsc},
			candidate.Platform,
			candidate.Type,
			"",
			"",
			0,
			0,
			"",
		)
		if err != nil {
			return fmt.Errorf("check owned account duplicate: %w", err)
		}
		for i := range accounts {
			existing := &accounts[i]
			if _, ok := skipIDs[existing.ID]; ok {
				continue
			}
			existingKeys := accountDuplicateIdentityKeys(existing)
			for _, candidateKey := range candidateKeys {
				for _, existingKey := range existingKeys {
					if existingKey.Name == candidateKey.Name && existingKey.Value == candidateKey.Value {
						return duplicateOwnedAccountError(candidate.Platform, candidateKey, existing.ID)
					}
				}
			}
		}
		if result == nil || int64(page*1000) >= result.Total || len(accounts) == 0 {
			return nil
		}
		page++
	}
}
func findDisallowedOwnedAccountField(values map[string]any) (string, bool) {
	return findDisallowedCredentialContent(values, credentialSafetyOptions{
		AllowOAuthTokenValues:  true,
		AllowOAuthMetadataURLs: true,
	})
}

func (s *AccountService) getPrivateGroupForOwnedAccount(ctx context.Context, ownerUserID int64, platform string) (*Group, error) {
	if s.privateGroupProvisioner == nil {
		return nil, ErrOwnedAccountGroupValidationUnavailable
	}
	group, err := s.privateGroupProvisioner.GetActiveUserPrivateGroup(ctx, ownerUserID, platform)
	if err == nil {
		return group, nil
	}
	if !errors.Is(err, ErrGroupNotFound) && !errors.Is(err, ErrGroupNotAllowed) {
		return nil, err
	}
	if provisionErr := s.privateGroupProvisioner.ProvisionUserPrivateGroups(ctx, ownerUserID); provisionErr != nil {
		return nil, provisionErr
	}
	group, err = s.privateGroupProvisioner.GetActiveUserPrivateGroup(ctx, ownerUserID, platform)
	if err != nil {
		return nil, err
	}
	return group, nil
}
func hasOwnedPersonalModelMapping(credentials map[string]any) bool {
	if len(credentials) == 0 {
		return false
	}
	value, ok := credentials["model_mapping"]
	if !ok || value == nil {
		return false
	}
	switch value.(type) {
	case map[string]any, map[string]string:
		return true
	default:
		return false
	}
}

func (s *AccountService) initialOwnedAccountGroupIDs(ctx context.Context, ownerUserID int64, account *Account, requestedGroupIDs []int64) ([]int64, error) {
	if account == nil {
		return nil, ErrAccountNotFound
	}
	privateGroup, err := s.getPrivateGroupForOwnedAccount(ctx, ownerUserID, account.Platform)
	if err != nil {
		return nil, err
	}
	return []int64{privateGroup.ID}, nil
}
func isAllowedOwnedAccountType(accountType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(accountType))
	switch normalized {
	case AccountTypeOAuth, AccountTypeSetupToken, AccountTypeAPIKey, AccountTypeBedrock, AccountTypeServiceAccount:
		return true
	default:
		return false
	}
}

func isOwnedAccountPublicShareApprovable(account *Account, allowRateLimited bool) bool {
	if account == nil {
		return false
	}
	if account.IsSchedulable() {
		return true
	}
	if !allowRateLimited || account.RateLimitResetAt == nil || !time.Now().Before(*account.RateLimitResetAt) {
		return false
	}
	copy := *account
	copy.RateLimitedAt = nil
	copy.RateLimitResetAt = nil
	return copy.IsSchedulable()
}

func isOwnedPublicSharePoolGroup(group *Group, platform string) bool {
	if group == nil || !group.IsActive() {
		return false
	}
	if group.OwnerUserID != nil || NormalizeGroupScope(group.Scope) != GroupScopePublic {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(group.Platform), strings.TrimSpace(platform)) {
		return false
	}
	return true
}

func supportsOwnedPublicSharePoolPlatform(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity, PlatformGrok, PlatformKiro:
		return true
	default:
		return false
	}
}
func (s *AccountService) managedOwnedAccountGroupIDsForShareMode(ctx context.Context, ownerUserID int64, account *Account, nextMode string) ([]int64, error) {
	if account == nil {
		return nil, ErrAccountNotFound
	}
	if NormalizeAccountShareMode(nextMode) == AccountShareModePublic && account.IsPublicShareApproved() {
		publicGroup, err := s.resolveOwnedPublicShareGroup(ctx, account)
		if err != nil {
			return nil, err
		}
		return s.publicOwnedAccountGroupIDs(ctx, ownerUserID, account, publicGroup)
	}
	return s.initialOwnedAccountGroupIDs(ctx, ownerUserID, account, nil)
}
func normalizeOwnedBulkAccountIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeOwnedBulkStatus(status string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return "", nil
	}
	if normalized == "inactive" {
		normalized = StatusDisabled
	}
	switch normalized {
	case StatusActive, StatusDisabled:
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid account status: %s", status)
	}
}

type ownedAccountDuplicateKey struct {
	Name  string
	Value string
}
type ownedAccountFilterRepository interface {
	ListOwnedWithFilters(ctx context.Context, ownerUserID int64, params pagination.PaginationParams, platform, accountType, status, search string, groupID, proxyID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error)
}

func ownedAccountForcesPrivateShare(accountType string) bool {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case AccountTypeAPIKey, AccountTypeBedrock, AccountTypeServiceAccount:
		return true
	default:
		return false
	}
}

const ownedPersonalDefaultConcurrency = 10

func ownedPersonalDefaultModelMapping(platform string) map[string]any {
	models := make([]string, 0)
	switch platform {
	case PlatformOpenAI:
		models = append(models, openai.DefaultModelIDs()...)
		models = append(models, "gpt-5.2-2025-12-11", "gpt-5.2-chat-latest", "gpt-5.2-pro", "gpt-5.2-pro-2025-12-11", "gpt-4o-audio-preview", "gpt-4o-realtime-preview")
	case PlatformAnthropic:
		models = append(models, claude.DefaultModelIDs()...)
		models = append(models, "claude-3-5-sonnet-20241022", "claude-3-5-sonnet-20240620", "claude-3-5-haiku-20241022", "claude-3-7-sonnet-20250219", "claude-sonnet-4-20250514", "claude-opus-4-20250514", "claude-opus-4-1-20250805")
	case PlatformGemini:
		for _, model := range geminicli.DefaultModels {
			models = append(models, model.ID)
		}
	case PlatformAntigravity:
		for _, model := range antigravity.DefaultModels() {
			models = append(models, model.ID)
		}
	case PlatformGrok:
		for _, model := range xai.DefaultModels() {
			models = append(models, model.ID)
		}
	case PlatformKiro:
		defaults := kiro.DefaultModelMapping()
		mapping := make(map[string]any, len(defaults))
		for from, to := range defaults {
			mapping[from] = to
		}
		return mapping
	}
	if len(models) == 0 {
		return map[string]any{}
	}
	mapping := make(map[string]any, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || strings.Contains(model, "*") {
			continue
		}
		mapping[model] = model
	}
	return mapping
}

const ownedPersonalDefaultOpenAICompactMode = "force_on"
const ownedPersonalDefaultOpenAIWSMode = OpenAIWSIngressModeOff
const ownedPersonalDefaultPriority = 1

func (s *AccountService) publicOwnedAccountGroupIDs(ctx context.Context, ownerUserID int64, account *Account, publicGroup *Group) ([]int64, error) {
	if account == nil || publicGroup == nil {
		return nil, ErrOwnedAccountPublicPoolUnavailable
	}
	privateGroup, err := s.getPrivateGroupForOwnedAccount(ctx, ownerUserID, account.Platform)
	if err != nil {
		return nil, err
	}
	return normalizeGroupIDs([]int64{privateGroup.ID, publicGroup.ID})
}
func (s *AccountService) resolveOwnedPublicShareGroup(ctx context.Context, account *Account) (*Group, error) {
	if s == nil || s.groupRepo == nil || account == nil {
		return nil, ErrOwnedAccountGroupValidationUnavailable
	}
	platform := strings.TrimSpace(account.Platform)
	if platform == "" {
		return nil, ErrOwnedAccountGroupPlatformMismatch
	}
	groups, err := s.groupRepo.ListActiveByPlatform(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("list public share groups: %w", err)
	}
	if account.Platform == PlatformOpenAI {
		accountLevel := NormalizeOpenAISharedPoolAccountLevel(NormalizeOpenAIAccountLevel(account.Platform, account.AccountLevel, account.Credentials, account.Extra))
		if OpenAISharedPoolLevelRank(accountLevel) == 0 && accountLevel != AccountLevelTeam && accountLevel != AccountLevelK12 {
			return nil, ErrOwnedAccountPublicPoolUnavailable.WithMetadata(map[string]string{
				"platform":      platform,
				"account_level": accountLevel,
			})
		}
		var matchedGroup *Group
		bestRank := 0
		for i := range groups {
			group := groups[i]
			requiredLevel := NormalizeOpenAISharedPoolRequiredLevel(group.RequiredAccountLevel)
			if requiredLevel == "" || !isOwnedPublicSharePoolGroup(&group, platform) || !CanOpenAIAccountJoinSharedPool(accountLevel, requiredLevel) {
				continue
			}
			requiredRank := OpenAISharedPoolLevelRank(requiredLevel)
			if matchedGroup == nil || requiredRank > bestRank {
				candidate := group
				matchedGroup = &candidate
				bestRank = requiredRank
			}
		}
		if matchedGroup != nil {
			return matchedGroup, nil
		}
		return nil, ErrOwnedAccountPublicPoolUnavailable.WithMetadata(map[string]string{
			"platform":      platform,
			"account_level": accountLevel,
		})
	}
	if !supportsOwnedPublicSharePoolPlatform(platform) {
		return nil, ErrOwnedAccountPublicPoolUnavailable.WithMetadata(map[string]string{
			"platform": platform,
		})
	}
	for i := range groups {
		group := groups[i]
		if group.IsSharedPool && isOwnedPublicSharePoolGroup(&group, platform) && NormalizeRequiredAccountLevel(group.RequiredAccountLevel) == "" {
			return &group, nil
		}
	}
	return nil, ErrOwnedAccountPublicPoolUnavailable.WithMetadata(map[string]string{
		"platform": platform,
	})
}

func (s *AccountService) validateOwnedAccountGroupBinding(ctx context.Context, ownerUserID int64, platform, accountType string, groupIDs []int64) ([]int64, error) {
	groupIDs, err := normalizeGroupIDs(groupIDs)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return nil, nil
	}
	if s.groupRepo == nil || s.userRepo == nil {
		return nil, ErrOwnedAccountGroupValidationUnavailable
	}

	user, err := s.userRepo.GetByID(ctx, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil || user.ID <= 0 {
		return nil, ErrUserNotFound
	}

	accountPlatform := strings.TrimSpace(platform)
	if accountPlatform == "" {
		return nil, ErrOwnedAccountGroupPlatformMismatch
	}
	for _, groupID := range groupIDs {
		group, err := s.groupRepo.GetByID(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf("get group: %w", err)
		}
		if group == nil || group.ID <= 0 {
			return nil, ErrGroupNotFound
		}
		if !group.IsActive() {
			return nil, ErrGroupNotAllowed
		}
		groupPlatform := strings.TrimSpace(group.Platform)
		if groupPlatform == "" || !strings.EqualFold(groupPlatform, accountPlatform) {
			return nil, ErrOwnedAccountGroupPlatformMismatch
		}
		if requiresOAuthOnlyGroupCheck(accountType) && isOAuthOnlyGroup(group) {
			return nil, ErrGroupNotAllowed
		}
		allowed, err := s.canUserBindOwnedAccountGroup(ctx, user, group)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrGroupNotAllowed
		}
	}
	return groupIDs, nil
}
func validateOwnedAccountSource(accountType string, credentials, extra map[string]any) error {
	return validateOwnedAccountSourceForPlatform("", accountType, credentials, extra)
}

func validateOwnedAccountSourceForPlatform(platform, accountType string, credentials, extra map[string]any) error {
	if !isAllowedOwnedAccountType(accountType) {
		return ErrOwnedAccountTypeNotAllowed
	}
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case AccountTypeOAuth, AccountTypeSetupToken:
		if strings.EqualFold(strings.TrimSpace(platform), PlatformAnthropic) &&
			strings.EqualFold(strings.TrimSpace(accountType), AccountTypeOAuth) &&
			isOwnedClaudeWebSessionExtra(extra) {
			return validateOwnedClaudeWebCredentials(credentials, extra)
		}
		if strings.EqualFold(strings.TrimSpace(platform), PlatformOpenAI) &&
			strings.EqualFold(strings.TrimSpace(accountType), AccountTypeOAuth) &&
			strings.EqualFold(strings.TrimSpace(stringMapValue(credentials, "auth_mode")), OpenAIAuthModeAgentIdentity) {
			return validateOwnedOpenAIAgentIdentityCredentials(credentials, extra)
		}
		if !hasNonEmptyStringField(credentials, "access_token") {
			return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
				"field": "access_token",
			})
		}
		if field, ok := findDisallowedOwnedAccountField(credentials); ok {
			return ErrOwnedAccountCredentialsNotAllowed.WithMetadata(map[string]string{
				"section": "credentials",
				"field":   field,
			})
		}
		if field, ok := findDisallowedOwnedAccountField(extra); ok {
			return ErrOwnedAccountCredentialsNotAllowed.WithMetadata(map[string]string{
				"section": "extra",
				"field":   field,
			})
		}
	case AccountTypeAPIKey:
		if !hasNonEmptyStringField(credentials, "api_key") {
			return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
				"field": "api_key",
			})
		}
	case AccountTypeBedrock:
		authMode, _ := credentials["auth_mode"].(string)
		switch strings.ToLower(strings.TrimSpace(authMode)) {
		case "sigv4":
			if !hasNonEmptyStringField(credentials, "aws_access_key_id") || !hasNonEmptyStringField(credentials, "aws_secret_access_key") {
				return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
					"field": "aws_credentials",
				})
			}
		case "api_key":
			if !hasNonEmptyStringField(credentials, "api_key") {
				return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
					"field": "api_key",
				})
			}
		default:
			return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
				"field": "auth_mode",
			})
		}
	case AccountTypeServiceAccount:
		if !hasNonEmptyStringField(credentials, "service_account_json") {
			return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
				"field": "service_account_json",
			})
		}
	}
	return nil
}

func validateOwnedOpenAIAgentIdentityCredentials(credentials, extra map[string]any) error {
	required := []string{"agent_runtime_id", "agent_private_key", "chatgpt_account_id", "chatgpt_user_id"}
	for _, field := range required {
		if !hasNonEmptyStringField(credentials, field) {
			return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{"field": field})
		}
	}
	if err := ValidateOpenAIAgentIdentityPrivateKey(stringMapValue(credentials, "agent_private_key")); err != nil {
		return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{"field": "agent_private_key"})
	}

	allowedFields := map[string]struct{}{
		"auth_mode":                  {},
		"agent_runtime_id":           {},
		"agent_private_key":          {},
		"task_id":                    {},
		"chatgpt_account_id":         {},
		"chatgpt_user_id":            {},
		"chatgpt_account_is_fedramp": {},
		"email":                      {},
		"plan_type":                  {},
	}
	remaining := make(map[string]any, len(credentials))
	for key, value := range credentials {
		if _, ok := allowedFields[key]; ok {
			switch key {
			case "chatgpt_account_is_fedramp":
				if _, ok := value.(bool); !ok {
					return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{"field": key})
				}
			default:
				text, ok := value.(string)
				if !ok || strings.ContainsAny(text, "\r\n\x00") || len(text) > 64*1024 {
					return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{"field": key})
				}
			}
			continue
		}
		remaining[key] = value
	}
	if field, ok := findDisallowedOwnedAccountField(remaining); ok {
		return ErrOwnedAccountCredentialsNotAllowed.WithMetadata(map[string]string{
			"section": "credentials",
			"field":   field,
		})
	}
	if field, ok := findDisallowedOwnedAccountField(extra); ok {
		return ErrOwnedAccountCredentialsNotAllowed.WithMetadata(map[string]string{
			"section": "extra",
			"field":   field,
		})
	}
	return nil
}

func isOwnedClaudeWebSessionExtra(extra map[string]any) bool {
	if extra == nil {
		return false
	}
	switch value := extra[ClaudeWebSessionExtraKey].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func validateOwnedClaudeWebCredentials(credentials, extra map[string]any) error {
	sessionKey, _ := credentials[ClaudeWebSessionKeyCredential].(string)
	sessionKey = strings.TrimSpace(sessionKey)
	extracted, valid := extractClaudeSessionKey(sessionKey)
	if !valid || extracted != sessionKey {
		return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
			"field": ClaudeWebSessionKeyCredential,
		})
	}

	authMode, _ := credentials[ClaudeWebAuthModeCredential].(string)
	authMode = strings.ToLower(strings.TrimSpace(authMode))
	switch authMode {
	case ClaudeWebAuthModeSessionKey:
		if strings.TrimSpace(stringMapValue(credentials, ClaudeWebBrowserCookieCredential)) != "" {
			return ErrOwnedAccountCredentialsNotAllowed.WithMetadata(map[string]string{
				"section": "credentials",
				"field":   ClaudeWebBrowserCookieCredential,
			})
		}
	case ClaudeWebAuthModeFullCookie:
		browserCookie := strings.TrimSpace(stringMapValue(credentials, ClaudeWebBrowserCookieCredential))
		if browserCookie == "" || strings.ContainsAny(browserCookie, "\r\n\x00") || len(browserCookie) > 64*1024 {
			return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
				"field": ClaudeWebBrowserCookieCredential,
			})
		}
		cookieSessionKey, ok := extractClaudeSessionKey(claudeWebCookieValue(browserCookie, "sessionKey"))
		if !ok || cookieSessionKey != sessionKey {
			return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
				"field": ClaudeWebBrowserCookieCredential,
			})
		}
	default:
		return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{
			"field": ClaudeWebAuthModeCredential,
		})
	}

	allowedCredentialFields := map[string]struct{}{
		ClaudeWebSessionKeyCredential:      {},
		ClaudeWebSessionKeyLCCredential:    {},
		ClaudeWebRoutingHintCredential:     {},
		ClaudeWebCFBMCredential:            {},
		ClaudeWebCFUVIDCredential:          {},
		ClaudeWebOrganizationCredential:    {},
		ClaudeWebAccountUUIDCredential:     {},
		ClaudeWebEmailCredential:           {},
		ClaudeWebAuthModeCredential:        {},
		ClaudeWebBrowserCookieCredential:   {},
		ClaudeWebDeviceIDCredential:        {},
		ClaudeWebActivitySessionCredential: {},
		ClaudeWebAnonymousIDCredential:     {},
		ClaudeWebSSIDCredential:            {},
	}
	remainingCredentials := make(map[string]any, len(credentials))
	for key, value := range credentials {
		if _, allowed := allowedCredentialFields[key]; allowed {
			text, ok := value.(string)
			if !ok || strings.ContainsAny(text, "\r\n\x00") || len(text) > 64*1024 {
				return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{"field": key})
			}
			continue
		}
		remainingCredentials[key] = value
	}
	if field, ok := findDisallowedOwnedAccountField(remainingCredentials); ok {
		return ErrOwnedAccountCredentialsNotAllowed.WithMetadata(map[string]string{
			"section": "credentials",
			"field":   field,
		})
	}

	remainingExtra := make(map[string]any, len(extra))
	for key, value := range extra {
		switch key {
		case ClaudeWebSessionExtraKey:
			if !isOwnedClaudeWebSessionExtra(map[string]any{ClaudeWebSessionExtraKey: value}) {
				return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{"field": key})
			}
			continue
		case "credential_format", "org_name", "saved_at":
			text, ok := value.(string)
			if !ok || strings.ContainsAny(text, "\r\n\x00") || len(text) > 4096 {
				return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{"field": key})
			}
			if key == "credential_format" && !strings.EqualFold(strings.TrimSpace(text), "claude_web") {
				return ErrOwnedAccountCredentialsInvalid.WithMetadata(map[string]string{"field": key})
			}
			continue
		default:
			remainingExtra[key] = value
		}
	}
	if field, ok := findDisallowedOwnedAccountField(remainingExtra); ok {
		return ErrOwnedAccountCredentialsNotAllowed.WithMetadata(map[string]string{
			"section": "extra",
			"field":   field,
		})
	}
	return nil
}

func stringMapValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func (s *AccountService) validateOwnedPublicSharePolicy(ctx context.Context, account *Account, group *Group) error {
	if s == nil || s.accountSharePolicyRepo == nil {
		return ErrOwnedAccountPublicPolicyUnavailable
	}
	if account == nil || group == nil || group.ID <= 0 {
		return ErrOwnedAccountPublicPolicyUnavailable
	}
	groupID := group.ID
	policy, err := s.accountSharePolicyRepo.ResolveEnabledAccountSharePolicy(ctx, account.ID, &groupID, account.Platform, account.SharePolicyID)
	if err != nil {
		return fmt.Errorf("resolve account share policy: %w", err)
	}
	if policy == nil || policy.OwnerShareRatio <= 0 {
		return ErrOwnedAccountPublicPolicyUnavailable.WithMetadata(map[string]string{
			"platform": account.Platform,
			"group_id": fmt.Sprintf("%d", group.ID),
		})
	}
	return nil
}
