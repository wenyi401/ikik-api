package claudeweb

import (
	"context"
	"testing"

	http "github.com/bogdanfinn/fhttp"
	"github.com/stretchr/testify/require"
)

func TestBrowserSessionSessionKeyModeDoesNotForwardSignedCookies(t *testing.T) {
	session := newBrowserSession(Credentials{
		SessionKey:    "sk-ant-session",
		AuthMode:      AuthModeSessionKey,
		BrowserCookie: "sessionKey=sk-ant-session; routingHint=signed; __cf_bm=stale; _cfuvid=stale",
		OrgUUID:       "org-1",
	})
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://claude.ai/api/organizations", nil)
	require.NoError(t, err)
	session.apply(request)

	cookies := make(map[string]string)
	for _, cookie := range request.Cookies() {
		cookies[cookie.Name] = cookie.Value
	}
	require.Equal(t, "sk-ant-session", cookies["sessionKey"])
	require.NotEmpty(t, cookies["sessionKeyLC"])
	require.NotEmpty(t, cookies["anthropic-device-id"])
	require.Equal(t, "org-1", cookies["lastActiveOrg"])
	require.NotContains(t, cookies, "routingHint")
	require.NotContains(t, cookies, "__cf_bm")
	require.NotContains(t, cookies, "_cfuvid")
	require.NotEmpty(t, request.Header.Get("traceparent"))
}

func TestBrowserSessionFullCookieModeUsesImportedHeader(t *testing.T) {
	const cookieHeader = "sessionKey=sk-ant-session; routingHint=signed; __cf_bm=current"
	session := newBrowserSession(Credentials{
		SessionKey:    "sk-ant-session",
		AuthMode:      AuthModeFullCookie,
		BrowserCookie: cookieHeader,
	})
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://claude.ai/api/organizations", nil)
	require.NoError(t, err)
	session.apply(request)

	require.Equal(t, cookieHeader, request.Header.Get("Cookie"))
}

func TestBrowserSessionIdentityIsStableForSessionKey(t *testing.T) {
	first := newBrowserSession(Credentials{SessionKey: "sk-ant-stable"})
	second := newBrowserSession(Credentials{SessionKey: "sk-ant-stable"})
	require.Equal(t, first.deviceID, second.deviceID)
	require.Equal(t, first.activitySession, second.activitySession)
	require.Equal(t, first.anonymousID, second.anonymousID)
	require.Equal(t, first.ssid, second.ssid)
}
