//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/pkg/xai"
)

type grokOAuthClientStub struct {
	refreshResponse *xai.TokenResponse
	ssoResponse     *xai.TokenResponse
	exchangeCalls   int
}

func (s *grokOAuthClientStub) ExchangeCode(context.Context, string, string, string, string, string) (*xai.TokenResponse, error) {
	s.exchangeCalls++
	return &xai.TokenResponse{}, nil
}

func (s *grokOAuthClientStub) RefreshToken(context.Context, string, string, string) (*xai.TokenResponse, error) {
	return s.refreshResponse, nil
}

func (s *grokOAuthClientStub) ConvertSSOToBuild(context.Context, string, string) (*xai.TokenResponse, error) {
	return s.ssoResponse, nil
}

func TestGrokOAuthServiceRefreshTokenPreservesOriginalRefreshTokenWhenNotRotated(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		refreshResponse: &xai.TokenResponse{
			AccessToken: "new-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		},
	})
	defer svc.Stop()

	info, err := svc.RefreshToken(context.Background(), "original-refresh-token", "", "client-id")
	require.NoError(t, err)
	require.Equal(t, "new-access-token", info.AccessToken)
	require.Equal(t, "original-refresh-token", info.RefreshToken)
	require.Equal(t, "client-id", info.ClientID)
}

func TestGrokOAuthServiceExchangeCodeRequiresStateForCallbackURLAndConsumesSession(t *testing.T) {
	client := &grokOAuthClientStub{}
	svc := NewGrokOAuthService(nil, client)
	defer svc.Stop()

	auth, err := svc.GenerateAuthURL(context.Background(), nil, "")
	require.NoError(t, err)

	_, err = svc.ExchangeCode(context.Background(), &GrokExchangeCodeInput{
		SessionID: auth.SessionID,
		Code:      "http://127.0.0.1:56121/callback?code=code-without-state",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GROK_OAUTH_STATE_REQUIRED")
	require.Zero(t, client.exchangeCalls)

	_, err = svc.ExchangeCode(context.Background(), &GrokExchangeCodeInput{
		SessionID: auth.SessionID,
		Code:      "code-with-state",
		State:     auth.State,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GROK_OAUTH_SESSION_NOT_FOUND")
	require.Zero(t, client.exchangeCalls)
}

func TestGrokOAuthServiceBuildAccountCredentialsDefaultsToSubscriptionProxy(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{})
	defer svc.Stop()

	credentials := svc.BuildAccountCredentials(&GrokTokenInfo{
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	})

	require.Equal(t, xai.DefaultCLIBaseURL, credentials["base_url"])
}

func TestGrokOAuthServiceConvertFromSSOExtractsBuildClaims(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		ssoResponse: &xai.TokenResponse{
			AccessToken:  makeGrokOAuthJWT(map[string]any{"sub": "user-sub", "team_id": "team-1", "tier": 5}),
			RefreshToken: "refresh-token",
			IDToken:      makeGrokOAuthJWT(map[string]any{"email": "user@example.com"}),
			ExpiresIn:    3600,
		},
	})
	defer svc.Stop()

	info, err := svc.ConvertFromSSO(context.Background(), "sso-token", nil)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", info.Email)
	require.Equal(t, "user-sub", info.Subject)
	require.Equal(t, "team-1", info.TeamID)
	require.Equal(t, "supergrok_heavy", info.SubscriptionTier)

	credentials := svc.BuildAccountCredentials(info)
	require.Equal(t, "user@example.com", credentials["email"])
	require.Equal(t, "user-sub", credentials["sub"])
	require.Equal(t, "team-1", credentials["team_id"])
	require.Equal(t, "supergrok_heavy", credentials["subscription_tier"])
	require.NotContains(t, credentials, "sso_token")
}

func TestGrokOAuthServiceRefreshAccountTokenOverwritesStaleTierFromNewJWT(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		refreshResponse: &xai.TokenResponse{
			AccessToken: makeGrokOAuthJWT(map[string]any{"sub": "user-sub", "tier": 0}),
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		},
	})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token":     "refresh-token",
			"client_id":         "client-id",
			"subscription_tier": "supergrok_heavy",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "free", info.SubscriptionTier)

	credentials := svc.BuildAccountCredentials(info)
	require.Equal(t, "free", credentials["subscription_tier"])
}

func TestGrokOAuthServiceRefreshAccountTokenIgnoresIDTokenTierWhenAccessTokenHasNone(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		refreshResponse: &xai.TokenResponse{
			AccessToken: "opaque-access-token",
			IDToken:     makeGrokOAuthJWT(map[string]any{"tier": 5}),
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		},
	})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token":     "refresh-token",
			"subscription_tier": "supergrok_lite",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "supergrok_lite", info.SubscriptionTier)
}

func TestGrokOAuthServiceRefreshAccountTokenKeepsStoredTierWhenJWTHasNoClaim(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		refreshResponse: &xai.TokenResponse{
			AccessToken: "opaque-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		},
	})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token":     "refresh-token",
			"subscription_tier": "supergrok_lite",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "supergrok_lite", info.SubscriptionTier)
}

func TestGrokOAuthServiceValidateSSOTokenReturnsOAuthTokensWithoutPersistingSSO(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		ssoResponse: &xai.TokenResponse{
			AccessToken:  "access-from-sso",
			RefreshToken: "refresh-from-sso",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		},
	})
	defer svc.Stop()

	info, err := svc.ValidateSSOToken(context.Background(), "sso-token", nil)
	require.NoError(t, err)
	require.Equal(t, "access-from-sso", info.AccessToken)
	require.Equal(t, "refresh-from-sso", info.RefreshToken)

	creds := svc.BuildAccountCredentials(info)
	require.NotContains(t, creds, "sso_token")
	require.NotContains(t, creds, "password")
}

func TestGrokOAuthServiceAuthorizePasswordUsesLoginThenSSOAuthorize(t *testing.T) {
	client := &grokOAuthClientStub{
		loginResult: &GrokPasswordLoginResult{
			Email:    "user@example.com",
			SSOToken: "password-derived-sso",
		},
		ssoResponse: &xai.TokenResponse{
			AccessToken:  "access-from-password",
			RefreshToken: "refresh-from-password",
			ExpiresIn:    3600,
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.Grok.PasswordAuthEnabled = true
	svc := NewGrokOAuthService(nil, client, cfg)
	defer svc.Stop()

	require.True(t, svc.GetCapabilities().PasswordAuthEnabled)
	info, err := svc.AuthorizePassword(context.Background(), " user@example.com ", "  super-secret  ", nil)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", info.Email)
	require.Equal(t, "access-from-password", info.AccessToken)
	creds := svc.BuildAccountCredentials(info)
	require.NotContains(t, creds, "password")
	require.NotContains(t, creds, "sso_token")
	require.Equal(t, "user@example.com", client.loginEmail)
	require.Equal(t, "  super-secret  ", client.loginPassword)
}

func TestGrokOAuthServiceAuthorizePasswordDisabledByDefault(t *testing.T) {
	client := &grokOAuthClientStub{}
	svc := NewGrokOAuthService(nil, client)
	defer svc.Stop()

	require.False(t, svc.GetCapabilities().PasswordAuthEnabled)
	_, err := svc.AuthorizePassword(context.Background(), "user@example.com", "secret", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "GROK_OAUTH_PASSWORD_AUTH_DISABLED")
	require.Empty(t, client.loginEmail)
}

func makeGrokOAuthJWT(claims map[string]any) string {
	payload, _ := json.Marshal(claims)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
