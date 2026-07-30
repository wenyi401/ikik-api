package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateOwnedOpenAIAgentIdentityCredentials(t *testing.T) {
	credentials := ownedAgentIdentityCredentials(t)
	require.NoError(t, validateOwnedAccountSourceForPlatform(PlatformOpenAI, AccountTypeOAuth, credentials, nil))

	credentials["api_key"] = "must-not-be-accepted"
	err := validateOwnedAccountSourceForPlatform(PlatformOpenAI, AccountTypeOAuth, credentials, nil)
	require.ErrorIs(t, err, ErrOwnedAccountCredentialsNotAllowed)
}

func TestOwnedAgentIdentityDuplicateKeyUsesTeamAndUser(t *testing.T) {
	credentials := ownedAgentIdentityCredentials(t)
	keys := accountDuplicateIdentityKeys(&Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: credentials,
	})
	require.Equal(t, []ownedAccountDuplicateKey{{
		Name:  "openai.agent_identity",
		Value: "team-owned|user-owned",
	}}, keys)
}

func TestOwnedAgentIdentityDuplicateKeyDistinguishesTeamMembers(t *testing.T) {
	first := ownedAgentIdentityCredentials(t)
	second := ownedAgentIdentityCredentials(t)
	second["chatgpt_user_id"] = "user-owned-2"

	firstKeys := accountDuplicateIdentityKeys(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: first})
	secondKeys := accountDuplicateIdentityKeys(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: second})

	require.NotEqual(t, firstKeys, secondKeys)
}

func TestOwnedAgentIdentityDuplicateKeyDistinguishesTeams(t *testing.T) {
	first := ownedAgentIdentityCredentials(t)
	second := ownedAgentIdentityCredentials(t)
	second["chatgpt_account_id"] = "team-owned-2"

	firstKeys := accountDuplicateIdentityKeys(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: first})
	secondKeys := accountDuplicateIdentityKeys(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: second})

	require.NotEqual(t, firstKeys, secondKeys)
}

func TestOwnedAgentIdentityDuplicateKeyIgnoresRuntimeRotation(t *testing.T) {
	first := ownedAgentIdentityCredentials(t)
	second := ownedAgentIdentityCredentials(t)
	second["agent_runtime_id"] = "runtime-owned-rotated"

	firstKeys := accountDuplicateIdentityKeys(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: first})
	secondKeys := accountDuplicateIdentityKeys(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: second})

	require.Equal(t, firstKeys, secondKeys)
}

func ownedAgentIdentityCredentials(t *testing.T) map[string]any {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	return map[string]any{
		"auth_mode":                  OpenAIAuthModeAgentIdentity,
		"agent_runtime_id":           "runtime-owned",
		"agent_private_key":          base64.StdEncoding.EncodeToString(der),
		"task_id":                    "task-owned",
		"chatgpt_account_id":         "team-owned",
		"chatgpt_user_id":            "user-owned",
		"chatgpt_account_is_fedramp": false,
	}
}
