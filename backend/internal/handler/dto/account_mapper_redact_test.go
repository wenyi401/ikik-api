package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/service"
)

func TestAccountFromServiceShallow_RedactsSensitiveCredentials(t *testing.T) {
	src := &service.Account{
		ID:       42,
		Name:     "demo",
		Platform: "anthropic",
		Type:     "oauth",
		Credentials: map[string]any{
			"access_token":  "at-secret",
			"refresh_token": "rt-secret",
			"id_token":      "id-secret",
			"api_key":       "sk-secret",
			"base_url":      "https://api.example.com",
			"model_mapping": map[string]any{"foo": "bar"},
		},
	}

	got := AccountFromServiceShallow(src)
	require.NotNil(t, got)

	// 敏感键不在 Credentials 里
	require.NotContains(t, got.Credentials, "access_token")
	require.NotContains(t, got.Credentials, "refresh_token")
	require.NotContains(t, got.Credentials, "id_token")
	require.NotContains(t, got.Credentials, "api_key")
	// 非敏感键保留
	require.Equal(t, "https://api.example.com", got.Credentials["base_url"])
	require.Equal(t, map[string]any{"foo": "bar"}, got.Credentials["model_mapping"])

	// 状态 map 标记敏感键存在
	require.True(t, got.CredentialsStatus["has_access_token"])
	require.True(t, got.CredentialsStatus["has_refresh_token"])
	require.True(t, got.CredentialsStatus["has_id_token"])
	require.True(t, got.CredentialsStatus["has_api_key"])

	// JSON 序列化校验：响应体里不会出现敏感子串
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "rt-secret")
	require.NotContains(t, string(raw), "at-secret")
	require.NotContains(t, string(raw), "sk-secret")
	require.NotContains(t, string(raw), "id-secret")
	// 状态标识应序列化进 JSON
	require.Contains(t, string(raw), "credentials_status")
	require.Contains(t, string(raw), "has_refresh_token")

	// 原始 service.Account 不应被改动
	require.Equal(t, "rt-secret", src.Credentials["refresh_token"])
}

func TestAccountFromServiceShallow_NilCredentialsOmitsStatus(t *testing.T) {
	src := &service.Account{ID: 1, Name: "n", Platform: "anthropic", Type: "oauth"}
	got := AccountFromServiceShallow(src)
	require.NotNil(t, got)
	require.Nil(t, got.Credentials)
	require.Nil(t, got.CredentialsStatus)
}

func TestAccountFromServiceShallow_PreservesOwnershipAndShareState(t *testing.T) {
	ownerUserID := int64(1056)
	sharePolicyID := int64(27)
	src := &service.Account{
		ID:            36399,
		Name:          "shared-account",
		Platform:      "openai",
		AccountLevel:  "PLUS",
		Type:          "oauth",
		OwnerUserID:   &ownerUserID,
		ShareMode:     "PUBLIC",
		ShareStatus:   "APPROVED",
		SharePolicyID: &sharePolicyID,
	}

	got := AccountFromServiceShallow(src)
	require.NotNil(t, got)
	require.Equal(t, "plus", got.AccountLevel)
	require.Equal(t, &ownerUserID, got.OwnerUserID)
	require.Equal(t, "public", got.ShareMode)
	require.Equal(t, "approved", got.ShareStatus)
	require.Equal(t, &sharePolicyID, got.SharePolicyID)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "plus", payload["account_level"])
	require.Equal(t, float64(ownerUserID), payload["owner_user_id"])
	require.Equal(t, "public", payload["share_mode"])
	require.Equal(t, "approved", payload["share_status"])
	require.Equal(t, float64(sharePolicyID), payload["share_policy_id"])
}
