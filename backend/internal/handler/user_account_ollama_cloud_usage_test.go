package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userOllamaCloudUsageRepoStub struct {
	service.AccountRepository
	accounts map[int64]*service.Account
}

func (r *userOllamaCloudUsageRepoStub) GetByID(_ context.Context, id int64) (*service.Account, error) {
	account := r.accounts[id]
	if account == nil {
		return nil, service.ErrAccountNotFound
	}
	return account, nil
}

func (r *userOllamaCloudUsageRepoStub) ListOllamaCloudUsageGroupAccounts(_ context.Context, accounts []*service.Account) ([]service.Account, error) {
	result := make([]service.Account, 0, len(accounts))
	for _, account := range accounts {
		if account != nil {
			result = append(result, *account)
		}
	}
	return result, nil
}

func (r *userOllamaCloudUsageRepoStub) SaveOllamaCloudUsageSession(context.Context, *service.Account, string, bool) error {
	return nil
}

func (r *userOllamaCloudUsageRepoStub) DeleteOllamaCloudUsageSession(context.Context, *service.Account) error {
	return nil
}

func (r *userOllamaCloudUsageRepoStub) SetOllamaCloudUsageAutoRefresh(context.Context, *service.Account, bool) error {
	return nil
}

func (r *userOllamaCloudUsageRepoStub) UpdateOllamaCloudUsageSnapshot(context.Context, *service.Account, *service.OllamaCloudUsageSnapshot) error {
	return nil
}

func (r *userOllamaCloudUsageRepoStub) DisableOllamaCloudUsageAutoRefresh(context.Context, *service.Account) error {
	return nil
}

func (r *userOllamaCloudUsageRepoStub) ListDueOllamaCloudUsageAccounts(context.Context, time.Time, int) ([]service.Account, error) {
	return nil, nil
}

func newUserOllamaCloudUsageHandler(repo *userOllamaCloudUsageRepoStub, encryptionKeyConfigured bool) *UserAccountHandler {
	accountService := service.NewAccountService(repo, nil, nil, nil)
	usageService := service.NewOllamaCloudUsageService(repo, nil, nil, nil, encryptionKeyConfigured)
	handler := NewUserAccountHandler(accountService, nil, nil, nil, nil, nil, nil)
	handler.SetOllamaCloudUsageService(usageService)
	return handler
}

func TestUserOllamaCloudUsageRejectsForeignAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	foreignOwnerID := int64(202)
	repo := &userOllamaCloudUsageRepoStub{accounts: map[int64]*service.Account{
		7: {
			ID: 7, OwnerUserID: &foreignOwnerID, Platform: service.PlatformOpenAI,
			Type: service.AccountTypeAPIKey, Credentials: map[string]any{
				"base_url": "https://ollama.com", "api_key": "foreign-key",
			}, Extra: map[string]any{}, Status: service.StatusActive,
		},
	}}
	handler := newUserOllamaCloudUsageHandler(repo, true)
	ctx, recorder := newOwnedAccountRequestContext(t, http.MethodGet, "/api/v1/accounts/7/ollama-cloud-usage", "", ownerID)
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}

	handler.GetOllamaCloudUsage(ctx)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestUserOllamaCloudUsageReturnsOwnedAccountState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	repo := &userOllamaCloudUsageRepoStub{accounts: map[int64]*service.Account{
		7: {
			ID: 7, OwnerUserID: &ownerID, Platform: service.PlatformAnthropic,
			Type: service.AccountTypeAPIKey, Credentials: map[string]any{
				"base_url": "https://ollama.com/v1", "api_key": "owned-key",
			}, Extra: map[string]any{}, Status: service.StatusActive,
		},
	}}
	handler := newUserOllamaCloudUsageHandler(repo, true)
	ctx, recorder := newOwnedAccountRequestContext(t, http.MethodGet, "/api/v1/accounts/7/ollama-cloud-usage", "", ownerID)
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}

	handler.GetOllamaCloudUsage(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"account_id":7`)
	require.Contains(t, recorder.Body.String(), `"eligible":true`)
	require.Contains(t, recorder.Body.String(), `"encryption_key_configured":true`)
	require.NotContains(t, recorder.Body.String(), "owned-key")
}

func TestUserOllamaCloudUsageValidatesOwnedMutationPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	repo := &userOllamaCloudUsageRepoStub{accounts: map[int64]*service.Account{
		7: {
			ID: 7, OwnerUserID: &ownerID, Platform: service.PlatformOpenAI,
			Type: service.AccountTypeAPIKey, Credentials: map[string]any{
				"base_url": "https://ollama.com", "api_key": "owned-key",
			}, Extra: map[string]any{}, Status: service.StatusActive,
		},
	}}
	handler := newUserOllamaCloudUsageHandler(repo, true)
	ctx, recorder := newOwnedAccountRequestContext(t, http.MethodPut, "/api/v1/accounts/7/ollama-cloud-usage/auto-refresh", `{}`, ownerID)
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}

	handler.SetOllamaCloudUsageAutoRefresh(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
