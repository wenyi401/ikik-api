package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"ikik-api/internal/pkg/openai"

	"github.com/google/uuid"
)

const MaxAccountCredentialImportItems = 200

const codexManagerOpenAIIssuer = "https://auth.openai.com"

type AccountCredentialImportKind string

const (
	AccountCredentialImportKindOAuthCredentials   AccountCredentialImportKind = "oauth_credentials"
	AccountCredentialImportKindOpenAIRefreshToken AccountCredentialImportKind = "openai_refresh_token"
	AccountCredentialImportKindClaudeSessionKey   AccountCredentialImportKind = "claude_session_key"
	AccountCredentialImportKindClaudeWebSession   AccountCredentialImportKind = "claude_web_session"
	AccountCredentialImportKindKiroConfig         AccountCredentialImportKind = "kiro_config"
)

type AccountCredentialImportSource struct {
	Kind         AccountCredentialImportKind
	Name         string
	Notes        *string
	Platform     string
	Credentials  map[string]any
	Extra        map[string]any
	Token        string
	ClientID     string
	ClientSecret string
	AuthMethod   string
	Provider     string
	StartURL     string
	Region       string
	ProfileArn   string
}

type AccountCredentialImportError struct {
	Index   int    `json:"index"`
	Kind    string `json:"kind,omitempty"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

type AccountCredentialImportResult struct {
	Total   int                            `json:"total"`
	Created int                            `json:"created"`
	Failed  int                            `json:"failed"`
	Errors  []AccountCredentialImportError `json:"errors"`
}

type AccountCredentialImportOptions struct {
	KiroConfigImport  bool
	ClaudeWebImport   bool
	ClaudeWebAuthMode string
}

func ParseAccountCredentialImportContents(contents []string) ([]AccountCredentialImportSource, []AccountCredentialImportError) {
	return ParseAccountCredentialImportContentsWithOptions(contents, AccountCredentialImportOptions{})
}

func ParseAccountCredentialImportContentsWithOptions(contents []string, opts AccountCredentialImportOptions) ([]AccountCredentialImportSource, []AccountCredentialImportError) {
	sources := make([]AccountCredentialImportSource, 0)
	errs := make([]AccountCredentialImportError, 0)
	nextIndex := 1

	for _, content := range contents {
		items, err := parseAccountCredentialImportContent(content)
		if err != nil {
			errs = append(errs, AccountCredentialImportError{
				Index:   nextIndex,
				Message: err.Error(),
			})
			nextIndex++
			continue
		}
		for _, item := range items {
			itemSources, itemErr := accountCredentialImportSourcesFromValue(item, opts)
			if itemErr != nil {
				errs = append(errs, AccountCredentialImportError{
					Index:   nextIndex,
					Message: itemErr.Error(),
				})
				nextIndex++
				continue
			}
			for _, source := range itemSources {
				sources = append(sources, source)
				nextIndex++
			}
		}
	}
	return sources, errs
}

func BuildOpenAIAccountCredentialImportExtra(tokenInfo *OpenAITokenInfo) map[string]any {
	extra := map[string]any{}
	if tokenInfo == nil {
		return extra
	}
	if strings.TrimSpace(tokenInfo.Email) != "" {
		extra["email"] = strings.TrimSpace(tokenInfo.Email)
	}
	if strings.TrimSpace(tokenInfo.PrivacyMode) != "" {
		extra["privacy_mode"] = strings.TrimSpace(tokenInfo.PrivacyMode)
	}
	return extra
}

func BuildClaudeAccountCredentialImportExtra(tokenInfo *TokenInfo) map[string]any {
	extra := map[string]any{}
	if tokenInfo == nil {
		return extra
	}
	if strings.TrimSpace(tokenInfo.OrgUUID) != "" {
		extra["org_uuid"] = strings.TrimSpace(tokenInfo.OrgUUID)
	}
	if strings.TrimSpace(tokenInfo.AccountUUID) != "" {
		extra["account_uuid"] = strings.TrimSpace(tokenInfo.AccountUUID)
	}
	if strings.TrimSpace(tokenInfo.EmailAddress) != "" {
		extra["email_address"] = strings.TrimSpace(tokenInfo.EmailAddress)
	}
	return extra
}

func DeriveAccountCredentialImportName(platform string, credentials, extra map[string]any, sequence int) string {
	for _, source := range []map[string]any{credentials, extra} {
		if name := importStringField(source, "name", "email", "email_address"); name != "" {
			return name
		}
	}
	switch platform {
	case PlatformAnthropic:
		return fmt.Sprintf("Claude OAuth Account #%d", sequence)
	case PlatformGemini:
		return fmt.Sprintf("Gemini OAuth Account #%d", sequence)
	case PlatformAntigravity:
		return fmt.Sprintf("Antigravity OAuth Account #%d", sequence)
	case PlatformKiro:
		return fmt.Sprintf("Kiro OAuth Account #%d", sequence)
	default:
		return fmt.Sprintf("OpenAI OAuth Account #%d", sequence)
	}
}

func parseAccountCredentialImportContent(content string) ([]any, error) {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil, nil
	}

	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return decodeAccountCredentialImportJSONValues(text)
	}

	items := make([]any, 0)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
			values, err := decodeAccountCredentialImportJSONValues(line)
			if err != nil {
				return nil, err
			}
			items = append(items, values...)
			continue
		}
		items = append(items, line)
	}
	return items, nil
}

func decodeAccountCredentialImportJSONValues(text string) ([]any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	values := make([]any, 0)
	for {
		var value any
		err := decoder.Decode(&value)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid JSON import content: %w", err)
		}
		if array, ok := value.([]any); ok {
			values = append(values, array...)
			continue
		}
		values = append(values, value)
	}
	return values, nil
}

func accountCredentialImportSourcesFromValue(value any, opts AccountCredentialImportOptions) ([]AccountCredentialImportSource, error) {
	switch typed := value.(type) {
	case string:
		source, err := accountCredentialImportSourceFromString(typed, "", nil)
		if err != nil {
			return nil, err
		}
		return []AccountCredentialImportSource{source}, nil
	case []any:
		out := make([]AccountCredentialImportSource, 0, len(typed))
		for _, item := range typed {
			sources, err := accountCredentialImportSourcesFromValue(item, opts)
			if err != nil {
				return nil, err
			}
			out = append(out, sources...)
		}
		return out, nil
	case map[string]any:
		if accounts, ok := importArrayField(typed, "accounts"); ok {
			out := make([]AccountCredentialImportSource, 0, len(accounts))
			for _, account := range accounts {
				sources, err := accountCredentialImportSourcesFromValue(account, opts)
				if err != nil {
					return nil, err
				}
				out = append(out, sources...)
			}
			return out, nil
		}
		source, err := accountCredentialImportSourceFromMap(typed, opts)
		if err != nil {
			return nil, err
		}
		return []AccountCredentialImportSource{source}, nil
	default:
		return nil, fmt.Errorf("invalid import item")
	}
}

func accountCredentialImportSourceFromMap(item map[string]any, opts AccountCredentialImportOptions) (AccountCredentialImportSource, error) {
	if opts.KiroConfigImport {
		if source, handled, err := accountCredentialImportSourceFromKiroConfig(item); handled || err != nil {
			return source, err
		}
	}
	if opts.ClaudeWebImport {
		if source, handled, err := accountCredentialImportSourceFromClaudeWeb(item, opts.ClaudeWebAuthMode); handled || err != nil {
			return source, err
		}
	}

	if source, handled, err := accountCredentialImportSourceFromCodexManagerExport(item); handled || err != nil {
		return source, err
	}

	if field, found := findDisallowedCredentialImportField(item); found {
		return AccountCredentialImportSource{}, fmt.Errorf("disallowed credential field: %s", field)
	}

	name := credentialImportFirstNonEmptyString(
		importStringField(item, "name"),
		importStringField(item, "label"),
		importStringField(item, "email"),
	)
	notes := importOptionalStringField(item, "notes", "note", "description")
	extra := importMapField(item, "extra", "metadata")
	platform := normalizeCredentialImportPlatform(importStringField(item, "platform", "provider", "service"))

	if sessionKey := importStringField(item, "session_key", "sessionKey", "session_token", "claude_session_key", "claudeSessionKey"); sessionKey != "" {
		source, err := accountCredentialImportSourceFromString(sessionKey, name, notes)
		if err != nil {
			return AccountCredentialImportSource{}, err
		}
		return source, nil
	}

	credentials := importMapField(item, "credentials")
	if len(credentials) > 0 {
		accountType := strings.ToLower(strings.TrimSpace(importStringField(item, "type", "account_type", "accountType")))
		if accountType != "" && accountType != AccountTypeOAuth {
			return AccountCredentialImportSource{}, fmt.Errorf("credential import only supports OAuth account credentials")
		}
		if platform == "" {
			platform = inferOAuthCredentialPlatform(credentials, extra)
		}
		if platform == "" {
			return AccountCredentialImportSource{}, fmt.Errorf("account platform is required")
		}
		if accessToken := importStringField(credentials, "access_token", "accessToken"); accessToken != "" {
			credentials["access_token"] = accessToken
			if platform == PlatformOpenAI {
				enrichOpenAIImportCredentialsFromJWT(credentials)
			}
			return AccountCredentialImportSource{
				Kind:        AccountCredentialImportKindOAuthCredentials,
				Name:        name,
				Notes:       notes,
				Platform:    platform,
				Credentials: credentials,
				Extra:       extra,
			}, nil
		}
		if refreshToken := importStringField(credentials, "refresh_token", "refreshToken"); refreshToken != "" && platform == PlatformOpenAI {
			return AccountCredentialImportSource{
				Kind:     AccountCredentialImportKindOpenAIRefreshToken,
				Name:     name,
				Notes:    notes,
				Token:    refreshToken,
				ClientID: importStringField(credentials, "client_id", "clientId"),
			}, nil
		}
		return AccountCredentialImportSource{}, fmt.Errorf("OAuth credentials must include access_token")
	}

	if tokens := importMapField(item, "tokens", "token"); len(tokens) > 0 {
		tokenName := credentialImportFirstNonEmptyString(name, importStringField(tokens, "email", "email_address"))
		if refreshToken := importStringField(tokens, "refresh_token", "refreshToken"); refreshToken != "" && importStringField(tokens, "access_token", "accessToken") == "" {
			return AccountCredentialImportSource{
				Kind:     AccountCredentialImportKindOpenAIRefreshToken,
				Name:     tokenName,
				Notes:    notes,
				Token:    refreshToken,
				ClientID: importStringField(tokens, "client_id", "clientId"),
			}, nil
		}
		if accessToken := importStringField(tokens, "access_token", "accessToken"); accessToken != "" {
			tokens["access_token"] = accessToken
			if idToken := importStringField(tokens, "id_token", "idToken"); idToken != "" {
				tokens["id_token"] = idToken
			}
			if refreshToken := importStringField(tokens, "refresh_token", "refreshToken"); refreshToken != "" {
				tokens["refresh_token"] = refreshToken
			}
			enrichOpenAIImportCredentialsFromJWT(tokens)
			return AccountCredentialImportSource{
				Kind:        AccountCredentialImportKindOAuthCredentials,
				Name:        tokenName,
				Notes:       notes,
				Platform:    PlatformOpenAI,
				Credentials: tokens,
				Extra:       extra,
			}, nil
		}
	}

	if refreshToken := importStringField(item, "refresh_token", "refreshToken"); refreshToken != "" {
		if platform != "" && platform != PlatformOpenAI {
			return AccountCredentialImportSource{}, fmt.Errorf("refresh-token credential import currently supports OpenAI only; use OAuth JSON for this platform")
		}
		return AccountCredentialImportSource{
			Kind:     AccountCredentialImportKindOpenAIRefreshToken,
			Name:     name,
			Notes:    notes,
			Token:    refreshToken,
			ClientID: importStringField(item, "client_id", "clientId"),
		}, nil
	}

	if accessToken := importStringField(item, "access_token", "accessToken"); accessToken != "" {
		if platform == "" {
			platform = inferOAuthCredentialPlatform(item, extra)
		}
		if platform == "" {
			platform = PlatformOpenAI
		}
		credentials := copyImportMap(item)
		credentials["access_token"] = accessToken
		if platform == PlatformOpenAI {
			enrichOpenAIImportCredentialsFromJWT(credentials)
		}
		return AccountCredentialImportSource{
			Kind:        AccountCredentialImportKindOAuthCredentials,
			Name:        name,
			Notes:       notes,
			Platform:    platform,
			Credentials: credentials,
			Extra:       extra,
		}, nil
	}

	if value := importStringField(item, "value", "token", "credential"); value != "" {
		return accountCredentialImportSourceFromString(value, name, notes)
	}
	return AccountCredentialImportSource{}, fmt.Errorf("unsupported credential import item")
}

func accountCredentialImportSourceFromClaudeWeb(item map[string]any, requestedAuthMode string) (AccountCredentialImportSource, bool, error) {
	cookies := importMapField(item, "cookies")
	browserCookie := strings.TrimSpace(importStringField(item, "cookie", "browser_cookie", "browserCookie"))
	if len(cookies) == 0 && browserCookie == "" {
		return AccountCredentialImportSource{}, false, nil
	}
	if browserCookie == "" {
		browserCookie = claudeWebCookieHeader(cookies)
	}

	sessionKey := credentialImportFirstNonEmptyString(
		importStringField(cookies, "sessionKey", "session_key"),
		claudeWebCookieValue(browserCookie, "sessionKey"),
	)
	if sessionKey == "" {
		return AccountCredentialImportSource{}, true, fmt.Errorf("claude Web cookies must include sessionKey")
	}
	if _, ok := extractClaudeSessionKey(sessionKey); !ok {
		return AccountCredentialImportSource{}, true, fmt.Errorf("invalid Claude Web sessionKey")
	}

	email := credentialImportFirstNonEmptyString(
		importStringField(item, "email_address", "emailAddress"),
		importStringField(item, "email"),
	)
	accountUUID := importStringField(item, "uuid", "account_uuid", "accountUUID")
	orgUUID := credentialImportFirstNonEmptyString(
		importStringField(item, "org_uuid", "orgUUID", "organization_uuid", "organizationUUID"),
		claudeWebCookieValue(browserCookie, "lastActiveOrg"),
	)
	authMode := normalizeClaudeWebImportAuthMode(requestedAuthMode, importStringField(item, "auth_mode", "authMode", "cookie_mode", "cookieMode"))
	if authMode == ClaudeWebAuthModeFullCookie && browserCookie == "" {
		return AccountCredentialImportSource{}, true, fmt.Errorf("claude Web full-cookie mode requires browser cookies")
	}

	credentials := map[string]any{
		ClaudeWebSessionKeyCredential: sessionKey,
		ClaudeWebAuthModeCredential:   authMode,
	}
	setString := func(key, value string) {
		if value = strings.TrimSpace(value); value != "" {
			credentials[key] = value
		}
	}
	sessionKeyLC := credentialImportFirstNonEmptyString(
		importStringField(cookies, "sessionKeyLC", "session_key_lc"),
		claudeWebCookieValue(browserCookie, "sessionKeyLC"),
	)
	if sessionKeyLC == "" {
		sessionKeyLC = fmt.Sprintf("%d", time.Now().UnixMilli())
	}
	setString(ClaudeWebSessionKeyLCCredential, sessionKeyLC)
	if authMode == ClaudeWebAuthModeFullCookie {
		setString(ClaudeWebRoutingHintCredential, importStringField(cookies, "routingHint", "routing_hint"))
		setString(ClaudeWebCFBMCredential, importStringField(cookies, "__cf_bm", "cf_bm"))
		setString(ClaudeWebCFUVIDCredential, importStringField(cookies, "_cfuvid", "cfuvid"))
		setString(ClaudeWebBrowserCookieCredential, browserCookie)
	}
	setString(ClaudeWebOrganizationCredential, orgUUID)
	setString(ClaudeWebAccountUUIDCredential, accountUUID)
	setString(ClaudeWebEmailCredential, email)
	setString(ClaudeWebDeviceIDCredential, credentialImportFirstNonEmptyString(
		importStringField(cookies, "anthropic-device-id", "anthropic_device_id"),
		claudeWebCookieValue(browserCookie, "anthropic-device-id"),
		uuid.NewString(),
	))
	setString(ClaudeWebActivitySessionCredential, credentialImportFirstNonEmptyString(
		importStringField(cookies, "activitySessionId", "activity_session_id"),
		claudeWebCookieValue(browserCookie, "activitySessionId"),
		uuid.NewString(),
	))
	setString(ClaudeWebAnonymousIDCredential, credentialImportFirstNonEmptyString(
		importStringField(cookies, "ajs_anonymous_id", "anonymous_id"),
		claudeWebCookieValue(browserCookie, "ajs_anonymous_id"),
		"claudeai.v1."+uuid.NewString(),
	))
	setString(ClaudeWebSSIDCredential, credentialImportFirstNonEmptyString(
		importStringField(cookies, "__ssid", "ssid"),
		claudeWebCookieValue(browserCookie, "__ssid"),
		uuid.NewString(),
	))

	extra := map[string]any{
		ClaudeWebSessionExtraKey: true,
		"credential_format":      "claude_web",
	}
	if value := importStringField(item, "org_name", "orgName", "organization_name", "organizationName"); value != "" {
		extra["org_name"] = value
	}
	if value := importStringField(item, "saved_at", "savedAt"); value != "" {
		extra["saved_at"] = value
	}

	name := credentialImportFirstNonEmptyString(email, importStringField(item, "name"), importStringField(item, "org_name", "orgName"))
	return AccountCredentialImportSource{
		Kind:        AccountCredentialImportKindClaudeWebSession,
		Name:        name,
		Platform:    PlatformAnthropic,
		Credentials: credentials,
		Extra:       extra,
	}, true, nil
}

func normalizeClaudeWebImportAuthMode(requested, embedded string) string {
	for _, value := range []string{requested, embedded} {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case ClaudeWebAuthModeFullCookie, "cookie", "full":
			return ClaudeWebAuthModeFullCookie
		case ClaudeWebAuthModeSessionKey, "session", "bearer":
			return ClaudeWebAuthModeSessionKey
		}
	}
	return ClaudeWebAuthModeSessionKey
}

func claudeWebCookieHeader(cookies map[string]any) string {
	keys := make([]string, 0, len(cookies))
	for key, value := range cookies {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, strings.TrimSpace(key)+"="+strings.TrimSpace(fmt.Sprint(cookies[key])))
	}
	return strings.Join(parts, "; ")
}

func claudeWebCookieValue(cookieHeader, name string) string {
	for _, part := range strings.Split(cookieHeader, ";") {
		keyValue := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(keyValue) == 2 && keyValue[0] == name {
			return strings.TrimSpace(keyValue[1])
		}
	}
	return ""
}

func accountCredentialImportSourceFromCodexManagerExport(item map[string]any) (AccountCredentialImportSource, bool, error) {
	tokens := importMapField(item, "tokens")
	meta := importMapField(item, "meta")
	if len(tokens) == 0 || len(meta) == 0 {
		return AccountCredentialImportSource{}, false, nil
	}

	topLevel := copyImportMap(item)
	removeImportMapField(topLevel, "tokens")
	removeImportMapField(topLevel, "meta")
	if field, found := findDisallowedCredentialImportField(topLevel); found {
		return AccountCredentialImportSource{}, true, fmt.Errorf("disallowed credential field: %s", field)
	}
	if field, found := findDisallowedCredentialImportField(tokens); found {
		return AccountCredentialImportSource{}, true, fmt.Errorf("disallowed credential field: %s", field)
	}
	metaForSafety := copyImportMap(meta)
	removeImportMapField(metaForSafety, "issuer")
	if field, found := findDisallowedCredentialImportField(metaForSafety); found {
		return AccountCredentialImportSource{}, true, fmt.Errorf("disallowed credential field: %s", field)
	}

	issuer := strings.TrimRight(strings.TrimSpace(importStringField(meta, "issuer")), "/")
	if issuer != "" && !strings.EqualFold(issuer, codexManagerOpenAIIssuer) {
		return AccountCredentialImportSource{}, true, fmt.Errorf("unsupported Codex-Manager issuer: %s", issuer)
	}

	accessToken := importStringField(tokens, "access_token", "accessToken")
	if accessToken == "" {
		return AccountCredentialImportSource{}, true, fmt.Errorf("OAuth credentials must include access_token")
	}

	credentials := map[string]any{
		"access_token": accessToken,
	}
	if idToken := importStringField(tokens, "id_token", "idToken"); idToken != "" {
		credentials["id_token"] = idToken
	}
	if refreshToken := importStringField(tokens, "refresh_token", "refreshToken"); refreshToken != "" {
		credentials["refresh_token"] = refreshToken
	}
	if chatgptAccountID := importStringField(meta, "chatgpt_account_id", "chatgptAccountId"); chatgptAccountID != "" {
		credentials["chatgpt_account_id"] = chatgptAccountID
	}
	if chatgptUserID := importStringField(meta, "chatgpt_user_id", "chatgptUserId"); chatgptUserID != "" {
		credentials["chatgpt_user_id"] = chatgptUserID
	}
	if organizationID := importStringField(meta, "organization_id", "organizationId", "org_id", "orgId"); organizationID != "" {
		credentials["organization_id"] = organizationID
	}
	if planType := importStringField(meta, "plan_type", "planType", "chatgpt_plan_type", "chatgptPlanType", "subscription_plan", "subscriptionPlan"); planType != "" {
		credentials["plan_type"] = planType
	}
	if email := importStringField(meta, "email", "email_address", "emailAddress"); email != "" {
		credentials["email"] = email
	}
	if workspaceID := importStringField(meta, "workspace_id", "workspaceId"); workspaceID != "" {
		credentials["workspace_id"] = workspaceID
	}
	enrichOpenAIImportCredentialsFromJWT(credentials)

	notes := importOptionalStringField(meta, "note", "notes", "description")
	if notes == nil {
		notes = importOptionalStringField(item, "note", "notes", "description")
	}

	return AccountCredentialImportSource{
		Kind:        AccountCredentialImportKindOAuthCredentials,
		Name:        credentialImportFirstNonEmptyString(importStringField(meta, "label", "name"), importStringField(item, "name", "label")),
		Notes:       notes,
		Platform:    PlatformOpenAI,
		Credentials: credentials,
		Extra:       map[string]any{},
	}, true, nil
}

func accountCredentialImportSourceFromString(value, name string, notes *string) (AccountCredentialImportSource, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return AccountCredentialImportSource{}, fmt.Errorf("empty credential")
	}
	if sessionKey, ok := extractClaudeSessionKey(text); ok {
		return AccountCredentialImportSource{
			Kind:  AccountCredentialImportKindClaudeSessionKey,
			Name:  name,
			Notes: notes,
			Token: sessionKey,
		}, nil
	}
	if reason, blocked := disallowedRawCredentialReason(text); blocked {
		return AccountCredentialImportSource{}, fmt.Errorf("disallowed credential content: %s", reason)
	}
	return AccountCredentialImportSource{
		Kind:  AccountCredentialImportKindOpenAIRefreshToken,
		Name:  name,
		Notes: notes,
		Token: text,
	}, nil
}

func extractClaudeSessionKey(value string) (string, bool) {
	text := strings.TrimSpace(strings.Trim(value, `"'`))
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "sk-ant-sid") {
		return text, true
	}
	for _, prefix := range []string{"sessionkey=", "session_key=", "claude_session_key="} {
		idx := strings.Index(lower, prefix)
		if idx < 0 {
			continue
		}
		candidate := strings.TrimSpace(text[idx+len(prefix):])
		if cut := strings.IndexAny(candidate, ";\r\n\t "); cut >= 0 {
			candidate = candidate[:cut]
		}
		candidate = strings.Trim(candidate, `"'`)
		if strings.HasPrefix(strings.ToLower(candidate), "sk-ant-sid") {
			return candidate, true
		}
	}
	return "", false
}

func disallowedRawCredentialReason(value string) (string, bool) {
	return disallowedCredentialStringReason("", value, credentialSafetyOptions{})
}

func findDisallowedCredentialImportField(value any) (string, bool) {
	return findDisallowedCredentialContent(value, credentialSafetyOptions{
		AllowClaudeSessionKeyFields: true,
		AllowOAuthTokenValues:       true,
		AllowOAuthMetadataURLs:      true,
	})
}

func importMapField(values map[string]any, keys ...string) map[string]any {
	value, ok := importAnyField(values, keys...)
	if !ok {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return copyImportMap(typed)
	}
	return nil
}

func importArrayField(values map[string]any, keys ...string) ([]any, bool) {
	value, ok := importAnyField(values, keys...)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]any)
	return typed, ok
}

func importStringField(values map[string]any, keys ...string) string {
	value, ok := importAnyField(values, keys...)
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func importOptionalStringField(values map[string]any, keys ...string) *string {
	value := importStringField(values, keys...)
	if value == "" {
		return nil
	}
	return &value
}

func importAnyField(values map[string]any, keys ...string) (any, bool) {
	if len(values) == 0 {
		return nil, false
	}
	for _, key := range keys {
		normalizedTarget := normalizeCredentialImportKey(key)
		for existingKey, value := range values {
			if normalizeCredentialImportKey(existingKey) == normalizedTarget {
				return value, true
			}
		}
	}
	return nil, false
}

func copyImportMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func removeImportMapField(values map[string]any, key string) {
	normalizedTarget := normalizeCredentialImportKey(key)
	for existingKey := range values {
		if normalizeCredentialImportKey(existingKey) == normalizedTarget {
			delete(values, existingKey)
		}
	}
}

func normalizeCredentialImportKey(key string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(strings.TrimSpace(key)))
}

func normalizeCredentialImportPlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "anthropic", "claude":
		return PlatformAnthropic
	case "openai", "chatgpt", "codex":
		return PlatformOpenAI
	case "gemini", "google":
		return PlatformGemini
	case "antigravity":
		return PlatformAntigravity
	case "kiro":
		return PlatformKiro
	default:
		return ""
	}
}

func inferOAuthCredentialPlatform(credentials, extra map[string]any) string {
	if importStringField(credentials, "org_uuid", "account_uuid", "email_address") != "" ||
		importStringField(extra, "org_uuid", "account_uuid", "email_address") != "" {
		return PlatformAnthropic
	}
	if importStringField(credentials, "project_id", "oauth_type", "tier_id") != "" {
		return PlatformGemini
	}
	if importStringField(credentials, "chatgpt_account_id", "chatgpt_user_id", "organization_id", "id_token") != "" {
		return PlatformOpenAI
	}
	return ""
}

func enrichOpenAIImportCredentialsFromJWT(credentials map[string]any) {
	if len(credentials) == 0 {
		return
	}
	for _, token := range []string{
		importStringField(credentials, "id_token", "idToken"),
		importStringField(credentials, "access_token", "accessToken"),
	} {
		if token == "" {
			continue
		}
		claims, err := openai.DecodeIDToken(token)
		if err != nil {
			continue
		}
		info := claims.GetUserInfo()
		if info == nil {
			continue
		}
		setImportStringIfMissing(credentials, "email", info.Email)
		setImportStringIfMissing(credentials, "chatgpt_account_id", info.ChatGPTAccountID)
		setImportStringIfMissing(credentials, "chatgpt_user_id", info.ChatGPTUserID)
		setImportStringIfMissing(credentials, "organization_id", info.OrganizationID)
		setImportStringIfMissing(credentials, "plan_type", info.PlanType)
	}
}

func setImportStringIfMissing(values map[string]any, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" || importStringField(values, key) != "" {
		return
	}
	values[key] = value
}

func credentialImportFirstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
