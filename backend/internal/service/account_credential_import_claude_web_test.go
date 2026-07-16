package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeWebImportStoresBrowserModesAndIdentity(t *testing.T) {
	contents := []string{`{
		"email":"person@example.com",
		"cookies":{
			"sessionKey":"sk-ant-sid02-example",
			"routingHint":"signed-routing",
			"__cf_bm":"browser-cookie"
		}
	}`}
	sources, parseErrors := ParseAccountCredentialImportContentsWithOptions(contents, AccountCredentialImportOptions{
		ClaudeWebImport:   true,
		ClaudeWebAuthMode: ClaudeWebAuthModeFullCookie,
	})
	require.Empty(t, parseErrors)
	require.Len(t, sources, 1)
	credentials := sources[0].Credentials
	require.Equal(t, ClaudeWebAuthModeFullCookie, credentials[ClaudeWebAuthModeCredential])
	require.Contains(t, credentials[ClaudeWebBrowserCookieCredential], "sessionKey=sk-ant-sid02-example")
	require.Contains(t, credentials[ClaudeWebBrowserCookieCredential], "routingHint=signed-routing")
	require.NotEmpty(t, credentials[ClaudeWebSessionKeyLCCredential])
	require.NotEmpty(t, credentials[ClaudeWebDeviceIDCredential])
	require.NotEmpty(t, credentials[ClaudeWebActivitySessionCredential])
	require.NotEmpty(t, credentials[ClaudeWebAnonymousIDCredential])
	require.NotEmpty(t, credentials[ClaudeWebSSIDCredential])
	require.NotContains(t, credentials, ClaudeWebOrganizationCredential)
}

func TestClaudeWebImportDefaultsToSessionKeyMode(t *testing.T) {
	sources, parseErrors := ParseAccountCredentialImportContentsWithOptions([]string{`{
		"org_uuid":"org-1",
		"cookies":{"sessionKey":"sk-ant-sid02-example","routingHint":"signed","__cf_bm":"stale"}
	}`}, AccountCredentialImportOptions{ClaudeWebImport: true})
	require.Empty(t, parseErrors)
	require.Len(t, sources, 1)
	credentials := sources[0].Credentials
	require.Equal(t, ClaudeWebAuthModeSessionKey, credentials[ClaudeWebAuthModeCredential])
	require.NotContains(t, credentials, ClaudeWebBrowserCookieCredential)
	require.NotContains(t, credentials, ClaudeWebRoutingHintCredential)
	require.NotContains(t, credentials, ClaudeWebCFBMCredential)
}
