package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"log/slog"

	"github.com/gin-gonic/gin"
	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/openai"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"
)

const (
	dataType       = "ikik-api-data"
	legacyDataType = "ikik-api-bundle"
	dataVersion    = 1
	dataPageCap    = 1000
)

type DataPayload = service.AccountDataPayload
type DataProxy = service.AccountDataProxy
type DataAccount = service.AccountDataAccount

type DataImportRequest struct {
	Data                 DataPayload `json:"data"`
	GroupIDs             []int64     `json:"group_ids"`
	SkipDefaultGroupBind *bool       `json:"skip_default_group_bind"`
}

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

type DataImportResult struct {
	ProxyCreated   int               `json:"proxy_created"`
	ProxyReused    int               `json:"proxy_reused"`
	ProxyFailed    int               `json:"proxy_failed"`
	AccountCreated int               `json:"account_created"`
	AccountFailed  int               `json:"account_failed"`
	Errors         []DataImportError `json:"errors,omitempty"`
}

type DataImportError struct {
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	ProxyKey string `json:"proxy_key,omitempty"`
	Message  string `json:"message"`
}

type dataImportPlatformMismatchExample struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

func buildProxyKey(protocol, host string, port int, username, password string) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s", strings.TrimSpace(protocol), strings.TrimSpace(host), port, strings.TrimSpace(username), strings.TrimSpace(password))
}

func (h *AccountHandler) ExportData(c *gin.Context) {
	ctx := c.Request.Context()

	selectedIDs, err := parseAccountIDs(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	accounts, err := h.resolveExportAccounts(ctx, selectedIDs, c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	includeProxies, err := parseIncludeProxies(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var proxies []service.Proxy
	if includeProxies {
		proxies, err = h.resolveExportProxies(ctx, accounts)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	} else {
		proxies = []service.Proxy{}
	}

	response.Success(c, service.BuildAccountDataPayload(accounts, proxies, buildProxyKey))
}

func (h *AccountHandler) ImportData(c *gin.Context) {
	var req DataImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := validateDataHeader(req.Data); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_data", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importData(ctx, req)
	})
}

func (h *AccountHandler) ImportCredentials(c *gin.Context) {
	var req CredentialImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	if req.Priority <= 0 {
		req.Priority = 50
	}

	sources, parseErrors := service.ParseAccountCredentialImportContentsWithOptions(req.Contents, service.AccountCredentialImportOptions{
		KiroConfigImport:  req.KiroConfigImport,
		ClaudeWebImport:   req.ClaudeWebImport,
		ClaudeWebAuthMode: req.ClaudeWebAuthMode,
	})
	if len(sources) == 0 && len(parseErrors) == 0 {
		response.BadRequest(c, "No importable account credentials found")
		return
	}
	if len(sources) > service.MaxAccountCredentialImportItems {
		response.BadRequest(c, fmt.Sprintf("Too many import items; maximum is %d", service.MaxAccountCredentialImportItems))
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_credentials", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importCredentials(ctx, req, sources, parseErrors), nil
	})
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

func (h *AccountHandler) importData(ctx context.Context, req DataImportRequest) (DataImportResult, error) {
	skipDefaultGroupBind := true
	if req.SkipDefaultGroupBind != nil {
		skipDefaultGroupBind = *req.SkipDefaultGroupBind
	}

	dataPayload := req.Data
	result := DataImportResult{}

	if err := h.validateImportTargetGroupPlatforms(ctx, req); err != nil {
		return result, err
	}

	existingProxies, err := h.listAllProxies(ctx)
	if err != nil {
		return result, err
	}

	proxyKeyToID := make(map[string]int64, len(existingProxies))
	proxyNameToID := make(map[string]int64, len(existingProxies))
	for i := range existingProxies {
		p := existingProxies[i]
		key := buildProxyKey(p.Protocol, p.Host, p.Port, p.Username, p.Password)
		proxyKeyToID[key] = p.ID
		if p.Name != "" {
			proxyNameToID[p.Name] = p.ID
		}
	}

	for i := range dataPayload.Proxies {
		item := dataPayload.Proxies[i]
		key := item.ProxyKey
		if key == "" {
			key = buildProxyKey(item.Protocol, item.Host, item.Port, item.Username, item.Password)
		}
		if err := validateDataProxy(item); err != nil {
			result.ProxyFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:     "proxy",
				Name:     item.Name,
				ProxyKey: key,
				Message:  err.Error(),
			})
			continue
		}
		normalizedStatus := normalizeProxyStatus(item.Status)
		expiresAt := dataProxyExpiresAt(item)
		fallbackMode, backupProxyID := dataProxyFallbackTarget(item, proxyNameToID)
		if existingID, ok := proxyKeyToID[key]; ok {
			proxyKeyToID[key] = existingID
			result.ProxyReused++
			if normalizedStatus != "" {
				if proxy, getErr := h.adminService.GetProxy(ctx, existingID); getErr == nil && proxy != nil && proxy.Status != normalizedStatus {
					_, _ = h.adminService.UpdateProxy(ctx, existingID, &service.UpdateProxyInput{
						Name:           proxy.Name,
						Protocol:       proxy.Protocol,
						Host:           proxy.Host,
						Port:           proxy.Port,
						Username:       proxy.Username,
						Password:       proxy.Password,
						Status:         normalizedStatus,
						ExpiresAt:      expiresAt,
						FallbackMode:   fallbackMode,
						BackupProxyID:  backupProxyID,
						ExpiryWarnDays: item.ExpiryWarnDays,
					})
				}
			}
			continue
		}

		created, createErr := h.adminService.CreateProxy(ctx, &service.CreateProxyInput{
			Name:           defaultProxyName(item.Name),
			Protocol:       item.Protocol,
			Host:           item.Host,
			Port:           item.Port,
			Username:       item.Username,
			Password:       item.Password,
			ExpiresAt:      expiresAt,
			FallbackMode:   fallbackMode,
			BackupProxyID:  backupProxyID,
			ExpiryWarnDays: item.ExpiryWarnDays,
		})
		if createErr != nil {
			result.ProxyFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:     "proxy",
				Name:     item.Name,
				ProxyKey: key,
				Message:  createErr.Error(),
			})
			continue
		}
		proxyKeyToID[key] = created.ID
		if created.Name != "" {
			proxyNameToID[created.Name] = created.ID
		}
		result.ProxyCreated++

		if normalizedStatus != "" && normalizedStatus != created.Status {
			_, _ = h.adminService.UpdateProxy(ctx, created.ID, &service.UpdateProxyInput{
				Name:           created.Name,
				Protocol:       created.Protocol,
				Host:           created.Host,
				Port:           created.Port,
				Username:       created.Username,
				Password:       created.Password,
				Status:         normalizedStatus,
				ExpiresAt:      expiresAt,
				FallbackMode:   fallbackMode,
				BackupProxyID:  backupProxyID,
				ExpiryWarnDays: item.ExpiryWarnDays,
			})
		}
	}

	// 收集需要异步设置隐私的 Antigravity OAuth 账号
	var privacyAccounts []*service.Account

	for i := range dataPayload.Accounts {
		item := dataPayload.Accounts[i]
		if err := validateDataAccount(item); err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: err.Error(),
			})
			continue
		}

		var proxyID *int64
		if item.ProxyKey != nil && *item.ProxyKey != "" {
			if id, ok := proxyKeyToID[*item.ProxyKey]; ok {
				proxyID = &id
			} else {
				result.AccountFailed++
				result.Errors = append(result.Errors, DataImportError{
					Kind:     "account",
					Name:     item.Name,
					ProxyKey: *item.ProxyKey,
					Message:  "proxy_key not found",
				})
				continue
			}
		}

		enrichCredentialsFromIDToken(&item)

		accountInput := &service.CreateAccountInput{
			Name:                 item.Name,
			Notes:                item.Notes,
			Platform:             item.Platform,
			Type:                 item.Type,
			Credentials:          item.Credentials,
			Extra:                item.Extra,
			ProxyID:              proxyID,
			Concurrency:          item.Concurrency,
			Priority:             item.Priority,
			RateMultiplier:       item.RateMultiplier,
			OwnerUserID:          item.OwnerUserID,
			ShareMode:            item.ShareMode,
			ShareStatus:          item.ShareStatus,
			SharePolicyID:        item.SharePolicyID,
			GroupIDs:             req.GroupIDs,
			ExpiresAt:            item.ExpiresAt,
			AutoPauseOnExpired:   item.AutoPauseOnExpired,
			SkipDefaultGroupBind: skipDefaultGroupBind,
		}

		created, err := h.adminService.CreateAccount(ctx, accountInput)
		if err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: err.Error(),
			})
			continue
		}
		// 收集 Antigravity OAuth 账号，稍后异步设置隐私
		if created.Platform == service.PlatformAntigravity && created.Type == service.AccountTypeOAuth {
			privacyAccounts = append(privacyAccounts, created)
		}
		h.enqueueOwnedPublicShareValidation(created)
		result.AccountCreated++
	}

	// 异步设置 Antigravity 隐私，避免大量导入时阻塞请求
	if len(privacyAccounts) > 0 {
		adminSvc := h.adminService
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("import_antigravity_privacy_panic", "recover", r)
				}
			}()
			bgCtx := context.Background()
			for _, acc := range privacyAccounts {
				adminSvc.ForceAntigravityPrivacy(bgCtx, acc)
			}
			slog.Info("import_antigravity_privacy_done", "count", len(privacyAccounts))
		}()
	}

	return result, nil
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

func (h *AccountHandler) listAllProxies(ctx context.Context) ([]service.Proxy, error) {
	page := 1
	pageSize := dataPageCap
	var out []service.Proxy
	for {
		items, total, err := h.adminService.ListProxies(ctx, page, pageSize, "", "", "", "created_at", "desc")
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(out) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (h *AccountHandler) listAccountsFiltered(ctx context.Context, platform, accountType, status, search string, groupID, proxyID int64, privacyMode, sortBy, sortOrder string) ([]service.Account, error) {
	page := 1
	pageSize := dataPageCap
	var out []service.Account
	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, pageSize, platform, accountType, status, search, groupID, proxyID, privacyMode, sortBy, sortOrder)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(out) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (h *AccountHandler) resolveExportAccounts(ctx context.Context, ids []int64, c *gin.Context) ([]service.Account, error) {
	if len(ids) > 0 {
		accounts, err := h.adminService.GetAccountsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		out := make([]service.Account, 0, len(accounts))
		for _, acc := range accounts {
			if acc == nil {
				continue
			}
			out = append(out, *acc)
		}
		return out, nil
	}

	platform := c.Query("platform")
	accountType := c.Query("type")
	status := c.Query("status")
	privacyMode := strings.TrimSpace(c.Query("privacy_mode"))
	search := strings.TrimSpace(c.Query("search"))
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")
	if len(search) > 100 {
		search = search[:100]
	}

	groupID := int64(0)
	if groupIDStr := c.Query("group"); groupIDStr != "" {
		if groupIDStr == accountListGroupUngroupedQueryValue {
			groupID = service.AccountListGroupUngrouped
		} else {
			parsedGroupID, parseErr := strconv.ParseInt(groupIDStr, 10, 64)
			if parseErr != nil || parsedGroupID <= 0 {
				return nil, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter")
			}
			groupID = parsedGroupID
		}
	}

	proxyID, err := parseAccountProxyFilter(c)
	if err != nil {
		return nil, err
	}

	return h.listAccountsFiltered(ctx, platform, accountType, status, search, groupID, proxyID, privacyMode, sortBy, sortOrder)
}

func (h *AccountHandler) resolveExportProxies(ctx context.Context, accounts []service.Account) ([]service.Proxy, error) {
	if len(accounts) == 0 {
		return []service.Proxy{}, nil
	}

	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for i := range accounts {
		if accounts[i].ProxyID == nil {
			continue
		}
		id := *accounts[i].ProxyID
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return []service.Proxy{}, nil
	}

	return h.adminService.GetProxiesByIDs(ctx, ids)
}

func parseAccountIDs(c *gin.Context) ([]int64, error) {
	values := c.QueryArray("ids")
	if len(values) == 0 {
		raw := strings.TrimSpace(c.Query("ids"))
		if raw != "" {
			values = []string{raw}
		}
	}
	if len(values) == 0 {
		return nil, nil
	}

	ids := make([]int64, 0, len(values))
	for _, item := range values {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("invalid account id: %s", part)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func parseIncludeProxies(c *gin.Context) (bool, error) {
	raw := strings.TrimSpace(strings.ToLower(c.Query("include_proxies")))
	if raw == "" {
		return true, nil
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return true, fmt.Errorf("invalid include_proxies value: %s", raw)
	}
}

func validateDataHeader(payload DataPayload) error {
	if payload.Type != "" && payload.Type != dataType && payload.Type != legacyDataType {
		return fmt.Errorf("unsupported data type: %s", payload.Type)
	}
	if payload.Version != 0 && payload.Version != dataVersion {
		return fmt.Errorf("unsupported data version: %d", payload.Version)
	}
	if payload.Proxies == nil {
		return errors.New("proxies is required")
	}
	if payload.Accounts == nil {
		return errors.New("accounts is required")
	}
	return nil
}

func validateDataProxy(item DataProxy) error {
	if strings.TrimSpace(item.Protocol) == "" {
		return errors.New("proxy protocol is required")
	}
	if strings.TrimSpace(item.Host) == "" {
		return errors.New("proxy host is required")
	}
	if item.Port <= 0 || item.Port > 65535 {
		return errors.New("proxy port is invalid")
	}
	switch item.Protocol {
	case "http", "https", "socks5", "socks5h":
	default:
		return fmt.Errorf("proxy protocol is invalid: %s", item.Protocol)
	}
	if item.Status != "" {
		normalizedStatus := normalizeProxyStatus(item.Status)
		if normalizedStatus != service.StatusActive && normalizedStatus != "inactive" && normalizedStatus != service.StatusExpired {
			return fmt.Errorf("proxy status is invalid: %s", item.Status)
		}
	}
	return nil
}

func validateDataAccount(item DataAccount) error {
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("account name is required")
	}
	if strings.TrimSpace(item.Platform) == "" {
		return errors.New("account platform is required")
	}
	if strings.TrimSpace(item.Type) == "" {
		return errors.New("account type is required")
	}
	if len(item.Credentials) == 0 {
		return errors.New("account credentials is required")
	}
	switch item.Type {
	case service.AccountTypeOAuth, service.AccountTypeSetupToken, service.AccountTypeAPIKey, service.AccountTypeUpstream:
	default:
		return fmt.Errorf("account type is invalid: %s", item.Type)
	}
	if item.RateMultiplier != nil && *item.RateMultiplier < 0 {
		return errors.New("rate_multiplier must be >= 0")
	}
	if item.Concurrency < 0 {
		return errors.New("concurrency must be >= 0")
	}
	if item.Priority < 0 {
		return errors.New("priority must be >= 0")
	}
	if item.OwnerUserID != nil && *item.OwnerUserID <= 0 {
		return errors.New("owner_user_id must be > 0")
	}
	if shareMode := strings.ToLower(strings.TrimSpace(item.ShareMode)); shareMode != "" {
		switch shareMode {
		case service.AccountShareModePrivate, service.AccountShareModePublic:
		default:
			return fmt.Errorf("share_mode is invalid: %s", item.ShareMode)
		}
	}
	if shareStatus := strings.ToLower(strings.TrimSpace(item.ShareStatus)); shareStatus != "" {
		switch shareStatus {
		case service.AccountShareStatusPending, service.AccountShareStatusApproved, service.AccountShareStatusSuspended:
		default:
			return fmt.Errorf("share_status is invalid: %s", item.ShareStatus)
		}
	}
	if item.SharePolicyID != nil && *item.SharePolicyID <= 0 {
		return errors.New("share_policy_id must be > 0")
	}
	return nil
}

func defaultProxyName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "imported-proxy"
	}
	return name
}

// enrichCredentialsFromIDToken performs best-effort extraction of user info fields
// (email, plan_type, chatgpt_account_id, etc.) from id_token in credentials.
// Only applies to OpenAI OAuth accounts. Skips expired token errors silently.
// Existing credential values are never overwritten — only missing fields are filled.
func enrichCredentialsFromIDToken(item *DataAccount) {
	if item.Credentials == nil {
		return
	}
	// Only enrich OpenAI OAuth accounts
	platform := strings.ToLower(strings.TrimSpace(item.Platform))
	if platform != service.PlatformOpenAI {
		return
	}
	if strings.ToLower(strings.TrimSpace(item.Type)) != service.AccountTypeOAuth {
		return
	}

	idToken, _ := item.Credentials["id_token"].(string)
	if strings.TrimSpace(idToken) == "" {
		return
	}

	// DecodeIDToken skips expiry validation — safe for imported data
	claims, err := openai.DecodeIDToken(idToken)
	if err != nil {
		slog.Debug("import_enrich_id_token_decode_failed", "account", item.Name, "error", err)
		return
	}

	userInfo := claims.GetUserInfo()
	if userInfo == nil {
		return
	}

	// Fill missing fields only (never overwrite existing values)
	setIfMissing := func(key, value string) {
		if value == "" {
			return
		}
		if existing, _ := item.Credentials[key].(string); existing == "" {
			item.Credentials[key] = value
		}
	}

	setIfMissing("email", userInfo.Email)
	setIfMissing("plan_type", userInfo.PlanType)
	setIfMissing("chatgpt_account_id", userInfo.ChatGPTAccountID)
	setIfMissing("chatgpt_user_id", userInfo.ChatGPTUserID)
	setIfMissing("organization_id", userInfo.OrganizationID)
}

func normalizeProxyStatus(status string) string {
	normalized := strings.TrimSpace(strings.ToLower(status))
	switch normalized {
	case "":
		return ""
	case service.StatusActive:
		return service.StatusActive
	case "inactive", service.StatusDisabled:
		return "inactive"
	default:
		return normalized
	}
}
