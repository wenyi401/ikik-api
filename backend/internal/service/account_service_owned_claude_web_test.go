package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountServiceCreateOwnedAllowsValidatedClaudeWebImport(t *testing.T) {
	sources, parseErrors := ParseAccountCredentialImportContentsWithOptions([]string{`{
		"email":"owned@example.com",
		"org_uuid":"org-owned",
		"cookies":{"sessionKey":"sk-ant-sid02-owned","routingHint":"signed"}
	}`}, AccountCredentialImportOptions{
		ClaudeWebImport:   true,
		ClaudeWebAuthMode: ClaudeWebAuthModeFullCookie,
	})
	require.Empty(t, parseErrors)
	require.Len(t, sources, 1)

	ownerID := int64(101)
	repo := &ownedAccountDuplicateRepoStub{}
	svc := NewAccountService(repo, nil, nil, nil)
	svc.SetUserPrivateGroupProvisioner(&ownedPrivateGroupProvisionerStub{
		group: &Group{ID: 99, Platform: PlatformAnthropic, Status: StatusActive, Scope: GroupScopeUserPrivate},
	})
	account, err := svc.ImportOwned(context.Background(), ownerID, CreateAccountRequest{
		Name:        sources[0].Name,
		Platform:    sources[0].Platform,
		Type:        AccountTypeOAuth,
		Credentials: sources[0].Credentials,
		Extra:       sources[0].Extra,
		ShareMode:   AccountShareModePrivate,
	})

	require.NoError(t, err)
	require.True(t, account.IsClaudeWebSession())
	require.Equal(t, &ownerID, account.OwnerUserID)
	require.Equal(t, AccountShareModePrivate, account.ShareMode)
	require.Equal(t, ClaudeWebAuthModeFullCookie, account.GetCredential(ClaudeWebAuthModeCredential))
}

func TestValidateOwnedClaudeWebRejectsNonStringAllowedCredential(t *testing.T) {
	err := validateOwnedAccountSourceForPlatform(PlatformAnthropic, AccountTypeOAuth, map[string]any{
		ClaudeWebSessionKeyCredential:   "sk-ant-sid02-owned",
		ClaudeWebAuthModeCredential:     ClaudeWebAuthModeSessionKey,
		ClaudeWebOrganizationCredential: map[string]any{"base_url": "https://example.com"},
	}, map[string]any{
		ClaudeWebSessionExtraKey: true,
		"credential_format":      "claude_web",
	})

	require.ErrorIs(t, err, ErrOwnedAccountCredentialsInvalid)
}

func TestValidateOwnedClaudeWebDoesNotBypassGeneralOAuthSafety(t *testing.T) {
	err := validateOwnedAccountSourceForPlatform(PlatformOpenAI, AccountTypeOAuth, map[string]any{
		ClaudeWebSessionKeyCredential: "sk-ant-sid02-owned",
		ClaudeWebAuthModeCredential:   ClaudeWebAuthModeSessionKey,
	}, map[string]any{ClaudeWebSessionExtraKey: true})
	require.Error(t, err)

	err = validateOwnedAccountSourceForPlatform(PlatformAnthropic, AccountTypeOAuth, map[string]any{
		ClaudeWebSessionKeyCredential: "sk-ant-sid02-owned",
		ClaudeWebAuthModeCredential:   ClaudeWebAuthModeSessionKey,
		"custom_base_url":             "https://example.com",
	}, map[string]any{ClaudeWebSessionExtraKey: true})
	require.ErrorIs(t, err, ErrOwnedAccountCredentialsNotAllowed)
}
