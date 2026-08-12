package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/service"
)

func TestUserAccountRefreshRejectsExpiredProxyBeforeOAuthCall(t *testing.T) {
	proxyID := int64(9)
	past := time.Now().Add(-time.Hour)
	account := &service.Account{
		ID:       42,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		ProxyID:  &proxyID,
		Proxy:    &service.Proxy{ID: proxyID, Status: service.StatusActive, ExpiresAt: &past},
	}
	handler := &UserAccountHandler{}

	_, _, err := handler.refreshOwnedAccount(context.Background(), 1001, account)

	require.True(t, errors.Is(err, service.ErrAccountProxyExpired))
}
