package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAccountCredentialImportContentsEnrichesOpenAIJWTIdentity(t *testing.T) {
	token := importTestJWT(t, map[string]any{
		"email": "seat@example.com",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "team-account",
			"chatgpt_user_id":    "user-seat",
			"chatgpt_plan_type":  "team",
			"organizations": []map[string]any{
				{"id": "org-team", "is_default": true},
			},
		},
	})
	content := `{"tokens":{"access_token":"` + token + `"}}`

	sources, errs := ParseAccountCredentialImportContents([]string{content})

	require.Empty(t, errs)
	require.Len(t, sources, 1)
	require.Equal(t, PlatformOpenAI, sources[0].Platform)
	require.Equal(t, AccountCredentialImportKindOAuthCredentials, sources[0].Kind)
	require.Equal(t, "seat@example.com", sources[0].Credentials["email"])
	require.Equal(t, "team-account", sources[0].Credentials["chatgpt_account_id"])
	require.Equal(t, "user-seat", sources[0].Credentials["chatgpt_user_id"])
	require.Equal(t, "org-team", sources[0].Credentials["organization_id"])
	require.Equal(t, "team", sources[0].Credentials["plan_type"])
}

func TestParseAccountCredentialImportContentsKiroConfigRequiresSwitch(t *testing.T) {
	content := `{
		"clientId":"client-id",
		"clientSecret":"client-secret",
		"refreshToken":"refresh-token",
		"email":"kiro@example.com",
		"provider":"BuilderId",
		"region":"us-east-1",
		"subscription":"KIRO FREE",
		"creditLimit":50,
		"creditUsed":1
	}`

	sources, errs := ParseAccountCredentialImportContents([]string{content})
	require.Empty(t, errs)
	require.Len(t, sources, 1)
	require.NotEqual(t, AccountCredentialImportKindKiroConfig, sources[0].Kind)

	sources, errs = ParseAccountCredentialImportContentsWithOptions([]string{content}, AccountCredentialImportOptions{
		KiroConfigImport: true,
	})
	require.Empty(t, errs)
	require.Len(t, sources, 1)
	require.Equal(t, AccountCredentialImportKindKiroConfig, sources[0].Kind)
	require.Equal(t, PlatformKiro, sources[0].Platform)
	require.Equal(t, "kiro@example.com", sources[0].Name)
	require.Equal(t, "refresh-token", sources[0].Token)
	require.Equal(t, "client-id", sources[0].ClientID)
	require.Equal(t, "client-secret", sources[0].ClientSecret)
	require.Equal(t, "idc", sources[0].AuthMethod)
	require.Equal(t, "BuilderId", sources[0].Provider)
	require.Equal(t, "us-east-1", sources[0].Region)
	require.Equal(t, "KIRO FREE", sources[0].Credentials["plan_type"])
	require.Equal(t, json.Number("50"), sources[0].Credentials["credit_limit"])
	require.Equal(t, json.Number("1"), sources[0].Credentials["credit_used"])
	require.Equal(t, "kiro_config", sources[0].Extra["import_source"])
}

func importTestJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
