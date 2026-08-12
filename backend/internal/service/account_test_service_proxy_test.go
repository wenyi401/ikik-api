package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type proxyPreflightAccountRepository struct {
	AccountRepository
	account *Account
}

func (r *proxyPreflightAccountRepository) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, ErrAccountNotFound
}

func newProxyPreflightTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/accounts/42/test", nil)
	return ctx, recorder
}

func TestAccountTestServiceRejectsExpiredProxyBeforeUpstream(t *testing.T) {
	ctx, recorder := newProxyPreflightTestContext()
	proxyID := int64(9)
	past := time.Now().Add(-time.Hour)
	account := &Account{
		ID:          42,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token"},
		ProxyID:     &proxyID,
		Proxy:       &Proxy{ID: proxyID, Status: StatusActive, ExpiresAt: &past},
	}
	repo := &proxyPreflightAccountRepository{account: account}
	svc := &AccountTestService{accountRepo: repo}

	err := svc.TestAccountConnection(ctx, account.ID, "", "", AccountTestModeDefault)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ACCOUNT_PROXY_EXPIRED")
	require.Contains(t, recorder.Body.String(), "ACCOUNT_PROXY_EXPIRED")
}

func TestAccountTestServiceRejectsUnavailableProxyBeforeUpstream(t *testing.T) {
	ctx, recorder := newProxyPreflightTestContext()
	proxyID := int64(9)
	account := &Account{
		ID:          43,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token"},
		ProxyID:     &proxyID,
		Proxy:       &Proxy{ID: proxyID, Status: StatusDisabled},
	}
	repo := &proxyPreflightAccountRepository{account: account}
	svc := &AccountTestService{accountRepo: repo}

	err := svc.TestAccountConnection(ctx, account.ID, "", "", AccountTestModeDefault)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ACCOUNT_PROXY_UNAVAILABLE")
	require.Contains(t, recorder.Body.String(), "ACCOUNT_PROXY_UNAVAILABLE")
}
