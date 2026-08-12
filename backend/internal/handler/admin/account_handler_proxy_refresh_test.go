package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/service"
)

func TestAdminAccountRefreshRejectsUnavailableProxyBeforeOAuthCall(t *testing.T) {
	proxyID := int64(9)
	account := &service.Account{
		ID:       42,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		ProxyID:  &proxyID,
		Proxy:    &service.Proxy{ID: proxyID, Status: service.StatusDisabled},
	}
	handler := &AccountHandler{}

	_, _, err := handler.refreshSingleAccount(context.Background(), account)

	require.True(t, errors.Is(err, service.ErrAccountProxyUnavailable))
}
