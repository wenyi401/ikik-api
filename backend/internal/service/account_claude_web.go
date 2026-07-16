package service

import (
	"strings"

	"ikik-api/internal/pkg/claudeweb"
)

const (
	ClaudeWebSessionExtraKey           = "claude_web_session"
	ClaudeWebSessionKeyCredential      = "session_key"
	ClaudeWebSessionKeyLCCredential    = "session_key_lc"
	ClaudeWebRoutingHintCredential     = "routing_hint"
	ClaudeWebCFBMCredential            = "cf_bm"
	ClaudeWebCFUVIDCredential          = "cfuvid"
	ClaudeWebOrganizationCredential    = "org_uuid"
	ClaudeWebAccountUUIDCredential     = "account_uuid"
	ClaudeWebEmailCredential           = "email_address"
	ClaudeWebAuthModeCredential        = "auth_mode"
	ClaudeWebBrowserCookieCredential   = "browser_cookie"
	ClaudeWebDeviceIDCredential        = "anthropic_device_id"
	ClaudeWebActivitySessionCredential = "activity_session_id"
	ClaudeWebAnonymousIDCredential     = "anonymous_id"
	ClaudeWebSSIDCredential            = "ssid"
	ClaudeWebDefaultTestModel          = claudeweb.DefaultModel
	ClaudeWebAuthModeSessionKey        = string(claudeweb.AuthModeSessionKey)
	ClaudeWebAuthModeFullCookie        = string(claudeweb.AuthModeFullCookie)
)

func (a *Account) IsClaudeWebSession() bool {
	if a == nil || a.Platform != PlatformAnthropic || a.Type != AccountTypeOAuth || a.Extra == nil {
		return false
	}
	switch value := a.Extra[ClaudeWebSessionExtraKey].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func (a *Account) ClaudeWebCredentials() claudeweb.Credentials {
	if a == nil {
		return claudeweb.Credentials{}
	}
	return claudeweb.Credentials{
		SessionKey:        a.GetCredential(ClaudeWebSessionKeyCredential),
		SessionKeyLC:      a.GetCredential(ClaudeWebSessionKeyLCCredential),
		RoutingHint:       a.GetCredential(ClaudeWebRoutingHintCredential),
		CFBM:              a.GetCredential(ClaudeWebCFBMCredential),
		CFUVID:            a.GetCredential(ClaudeWebCFUVIDCredential),
		OrgUUID:           a.GetCredential(ClaudeWebOrganizationCredential),
		AuthMode:          claudeweb.AuthMode(a.GetCredential(ClaudeWebAuthModeCredential)),
		BrowserCookie:     a.GetCredential(ClaudeWebBrowserCookieCredential),
		DeviceID:          a.GetCredential(ClaudeWebDeviceIDCredential),
		ActivitySessionID: a.GetCredential(ClaudeWebActivitySessionCredential),
		AnonymousID:       a.GetCredential(ClaudeWebAnonymousIDCredential),
		SSID:              a.GetCredential(ClaudeWebSSIDCredential),
	}
}

func ClaudeWebSupportedModels() []string {
	return claudeweb.SupportedModels()
}

func IsClaudeWebModelSupported(model string) bool {
	return claudeweb.ValidateModel(model) == nil
}

func (a *Account) ClaudeWebProxyURL() string {
	if a == nil || a.ProxyID == nil || a.Proxy == nil {
		return ""
	}
	return a.Proxy.URL()
}
