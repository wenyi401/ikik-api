package admin

import (
	"context"
	"encoding/json"

	"fmt"
	"sort"
	"strconv"
	"strings"

	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/openai"

	"ikik-api/internal/service"
)

type CredentialImportRequest struct {
	Contents                []string `json:"contents" binding:"required"`
	KiroConfigImport        bool     `json:"kiro_config_import"`
	ClaudeWebImport         bool     `json:"claude_web_import"`
	ClaudeWebAuthMode       string   `json:"claude_web_auth_mode" binding:"omitempty,oneof=session_key full_cookie"`
	OwnerUserID             *int64   `json:"owner_user_id"`
	ShareMode               string   `json:"share_mode" binding:"omitempty,oneof=private public"`
	ShareStatus             string   `json:"share_status" binding:"omitempty,oneof=pending approved suspended"`
	SharePolicyID           *int64   `json:"share_policy_id"`
	ProxyID                 *int64   `json:"proxy_id"`
	Concurrency             int      `json:"concurrency"`
	Priority                int      `json:"priority"`
	RateMultiplier          *float64 `json:"rate_multiplier"`
	LoadFactor              *int     `json:"load_factor"`
	GroupIDs                []int64  `json:"group_ids"`
	ExpiresAt               *int64   `json:"expires_at"`
	AutoPauseOnExpired      *bool    `json:"auto_pause_on_expired"`
	SkipDefaultGroupBind    *bool    `json:"skip_default_group_bind"`
	ConfirmMixedChannelRisk *bool    `json:"confirm_mixed_channel_risk"`
}

func (h *AccountHandler) createAccountFromCredentialImportSource(
	ctx context.Context,
	source service.AccountCredentialImportSource,
	defaults CredentialImportRequest,
	sequence int,
) (*service.Account, error) {
	skipDefaultGroupBind := false
	if defaults.SkipDefaultGroupBind != nil {
		skipDefaultGroupBind = *defaults.SkipDefaultGroupBind
	}
	skipMixedChannelCheck := defaults.ConfirmMixedChannelRisk != nil && *defaults.ConfirmMixedChannelRisk

	input := service.CreateAccountInput{
		Name:                  strings.TrimSpace(source.Name),
		Notes:                 source.Notes,
		Platform:              source.Platform,
		Type:                  service.AccountTypeOAuth,
		Credentials:           source.Credentials,
		Extra:                 source.Extra,
		OwnerUserID:           defaults.OwnerUserID,
		ShareMode:             defaults.ShareMode,
		ShareStatus:           defaults.ShareStatus,
		SharePolicyID:         defaults.SharePolicyID,
		ProxyID:               defaults.ProxyID,
		Concurrency:           defaults.Concurrency,
		Priority:              defaults.Priority,
		RateMultiplier:        defaults.RateMultiplier,
		LoadFactor:            defaults.LoadFactor,
		GroupIDs:              defaults.GroupIDs,
		ExpiresAt:             defaults.ExpiresAt,
		AutoPauseOnExpired:    defaults.AutoPauseOnExpired,
		SkipDefaultGroupBind:  skipDefaultGroupBind,
		SkipMixedChannelCheck: skipMixedChannelCheck,
	}
	if input.Concurrency <= 0 {
		input.Concurrency = service.DefaultOAuthAccountConcurrencyForPlatform(input.Platform)
	}

	switch source.Kind {
	case service.AccountCredentialImportKindOAuthCredentials:
		if input.Name == "" {
			input.Name = service.DeriveAccountCredentialImportName(input.Platform, input.Credentials, input.Extra, sequence)
		}
	case service.AccountCredentialImportKindOpenAIRefreshToken:
		proxyURL, err := h.resolveCredentialImportProxyURL(ctx, defaults.ProxyID)
		if err != nil {
			return nil, err
		}
		clientID := strings.TrimSpace(source.ClientID)
		if clientID == "" {
			clientID, _ = openai.OAuthClientConfigByPlatform(service.PlatformOpenAI)
		}
		tokenInfo, err := h.openaiOAuthService.RefreshTokenWithClientID(ctx, source.Token, proxyURL, clientID)
		if err != nil {
			return nil, fmt.Errorf("validate OpenAI refresh token: %w", err)
		}
		input.Platform = service.PlatformOpenAI
		input.Credentials = h.openaiOAuthService.BuildAccountCredentials(tokenInfo)
		input.Extra = service.BuildOpenAIAccountCredentialImportExtra(tokenInfo)
		if input.Concurrency <= 0 || defaults.Concurrency <= 0 {
			input.Concurrency = service.DefaultOAuthAccountConcurrencyForPlatform(input.Platform)
		}
		if input.Name == "" {
			input.Name = strings.TrimSpace(tokenInfo.Email)
		}
		if input.Name == "" {
			input.Name = fmt.Sprintf("OpenAI OAuth Account #%d", sequence)
		}
	case service.AccountCredentialImportKindClaudeSessionKey:
		tokenInfo, err := h.oauthService.CookieAuth(ctx, &service.CookieAuthInput{
			SessionKey: source.Token,
			ProxyID:    defaults.ProxyID,
			Scope:      "full",
		})
		if err != nil {
			return nil, fmt.Errorf("exchange Claude session key: %w", err)
		}
		input.Platform = service.PlatformAnthropic
		input.Credentials = service.BuildClaudeAccountCredentials(tokenInfo)
		input.Extra = service.BuildClaudeAccountCredentialImportExtra(tokenInfo)
		if input.Concurrency <= 0 || defaults.Concurrency <= 0 {
			input.Concurrency = service.DefaultOAuthAccountConcurrencyForPlatform(input.Platform)
		}
		if input.Name == "" {
			input.Name = strings.TrimSpace(tokenInfo.EmailAddress)
		}
		if input.Name == "" {
			input.Name = fmt.Sprintf("Claude OAuth Account #%d", sequence)
		}
	case service.AccountCredentialImportKindClaudeWebSession:
		input.Platform = service.PlatformAnthropic
		input.Credentials = source.Credentials
		input.Extra = source.Extra
		if defaults.Concurrency <= 0 {
			input.Concurrency = 1
		}
		if input.Name == "" {
			input.Name = service.DeriveAccountCredentialImportName(input.Platform, input.Credentials, input.Extra, sequence)
		}
	case service.AccountCredentialImportKindKiroConfig:
		if h.kiroOAuthService == nil {
			return nil, fmt.Errorf("kiro OAuth service is not configured")
		}
		tokenInfo, err := h.kiroOAuthService.RefreshToken(ctx, &service.KiroRefreshTokenInput{
			RefreshToken: source.Token,
			AuthMethod:   source.AuthMethod,
			Provider:     source.Provider,
			ClientID:     source.ClientID,
			ClientSecret: source.ClientSecret,
			StartURL:     source.StartURL,
			Region:       source.Region,
			ProfileArn:   source.ProfileArn,
			ProxyID:      defaults.ProxyID,
		})
		if err != nil {
			return nil, fmt.Errorf("validate Kiro config: %w", err)
		}
		input.Platform = service.PlatformKiro
		input.Credentials = service.MergeCredentials(source.Credentials, h.kiroOAuthService.BuildAccountCredentials(tokenInfo))
		input.Extra = source.Extra
		if input.Concurrency <= 0 || defaults.Concurrency <= 0 {
			input.Concurrency = service.DefaultOAuthAccountConcurrencyForPlatform(input.Platform)
		}
		if input.Name == "" {
			input.Name = strings.TrimSpace(tokenInfo.Email)
		}
		if input.Name == "" {
			input.Name = service.DeriveAccountCredentialImportName(input.Platform, input.Credentials, input.Extra, sequence)
		}
	default:
		return nil, fmt.Errorf("unsupported credential import kind")
	}

	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("account name is required")
	}
	sanitizeExtraBaseRPM(input.Extra)
	account, err := h.adminService.CreateAccount(ctx, &input)
	if err != nil {
		return nil, err
	}
	h.adminService.ForceAntigravityPrivacy(ctx, account)
	h.adminService.ForceOpenAIPrivacy(ctx, account)
	h.enqueueOwnedPublicShareValidation(account)
	return account, nil
}

type dataImportPlatformMismatchExample struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

func (h *AccountHandler) importCredentials(
	ctx context.Context,
	req CredentialImportRequest,
	sources []service.AccountCredentialImportSource,
	parseErrors []service.AccountCredentialImportError,
) service.AccountCredentialImportResult {
	result := service.AccountCredentialImportResult{
		Total:  len(sources) + len(parseErrors),
		Errors: []service.AccountCredentialImportError{},
	}
	result.Errors = append(result.Errors, parseErrors...)

	for idx, source := range sources {
		account, err := h.createAccountFromCredentialImportSource(ctx, source, req, idx+1)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, service.AccountCredentialImportError{
				Index:   len(parseErrors) + idx + 1,
				Kind:    string(source.Kind),
				Name:    source.Name,
				Message: err.Error(),
			})
			continue
		}
		if account != nil {
			result.Created++
		}
	}
	result.Failed += len(parseErrors)
	return result
}

func (h *AccountHandler) resolveCredentialImportProxyURL(ctx context.Context, proxyID *int64) (string, error) {
	if proxyID == nil {
		return "", nil
	}
	proxy, err := h.adminService.GetProxy(ctx, *proxyID)
	if err != nil {
		return "", fmt.Errorf("load proxy: %w", err)
	}
	if proxy == nil {
		return "", fmt.Errorf("proxy not found")
	}
	return proxy.URL(), nil
}

func (h *AccountHandler) validateImportTargetGroupPlatforms(ctx context.Context, req DataImportRequest) error {
	if len(req.GroupIDs) == 0 {
		return nil
	}

	groups := make([]*service.Group, 0, len(req.GroupIDs))
	seen := make(map[int64]struct{}, len(req.GroupIDs))
	for _, groupID := range req.GroupIDs {
		if groupID <= 0 {
			return infraerrors.BadRequest("IMPORT_TARGET_GROUP_INVALID", fmt.Sprintf("import target group %d does not exist", groupID))
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}

		group, err := h.adminService.GetGroup(ctx, groupID)
		if err != nil || group == nil {
			return infraerrors.BadRequest("IMPORT_TARGET_GROUP_INVALID", fmt.Sprintf("import target group %d does not exist", groupID))
		}
		groups = append(groups, group)
	}
	if len(groups) == 0 {
		return nil
	}

	expectedPlatform := strings.TrimSpace(groups[0].Platform)
	platforms := map[string]struct{}{expectedPlatform: {}}
	for _, group := range groups[1:] {
		platforms[strings.TrimSpace(group.Platform)] = struct{}{}
	}
	if len(platforms) > 1 {
		selectedPlatforms := make([]string, 0, len(platforms))
		for platform := range platforms {
			selectedPlatforms = append(selectedPlatforms, platform)
		}
		sort.Strings(selectedPlatforms)
		return infraerrors.BadRequest(
			"IMPORT_TARGET_GROUP_PLATFORM_MISMATCH",
			fmt.Sprintf("import target groups must belong to one platform %s: %d groups mismatch", expectedPlatform, len(groups)-1),
		).WithMetadata(map[string]string{
			"expected_platform":  expectedPlatform,
			"mismatch_count":     strconv.Itoa(len(groups) - 1),
			"selected_platforms": strings.Join(selectedPlatforms, ","),
		})
	}

	var examples []dataImportPlatformMismatchExample
	mismatchCount := 0
	for _, account := range req.Data.Accounts {
		accountPlatform := strings.TrimSpace(account.Platform)
		if accountPlatform == expectedPlatform {
			continue
		}
		mismatchCount++
		if len(examples) < 5 {
			examples = append(examples, dataImportPlatformMismatchExample{
				Name:     account.Name,
				Platform: accountPlatform,
			})
		}
	}
	if mismatchCount == 0 {
		return nil
	}

	examplesJSON, _ := json.Marshal(examples)
	return infraerrors.BadRequest(
		"IMPORT_ACCOUNT_PLATFORM_MISMATCH",
		fmt.Sprintf("imported accounts must match target group platform %s: %d accounts mismatch", expectedPlatform, mismatchCount),
	).WithMetadata(map[string]string{
		"expected_platform": expectedPlatform,
		"mismatch_count":    strconv.Itoa(mismatchCount),
		"mismatch_examples": string(examplesJSON),
	})
}
