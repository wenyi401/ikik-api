package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"ikik-api/internal/config"
	"ikik-api/internal/pkg/tlsfingerprint"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

type userAccountRecoveryRepoStub struct {
	service.AccountRepository
	accounts            map[int64]*service.Account
	clearRateLimitErr   map[int64]error
	clearErrorIDs       []int64
	clearRateLimitIDs   []int64
	clearAntigravityIDs []int64
	clearModelLimitIDs  []int64
	clearTempUnschedIDs []int64
}

type userAccountTestHTTPUpstream struct{}

func (userAccountTestHTTPUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected non-TLS upstream call")
}

func (userAccountTestHTTPUpstream) DoWithTLS(*http.Request, string, int64, int, *tlsfingerprint.Profile) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\"}\n\n")),
	}, nil
}

func (r *userAccountRecoveryRepoStub) GetByID(_ context.Context, id int64) (*service.Account, error) {
	account := r.accounts[id]
	if account == nil {
		return nil, service.ErrAccountNotFound
	}
	return account, nil
}

func (r *userAccountRecoveryRepoStub) ClearError(_ context.Context, id int64) error {
	r.clearErrorIDs = append(r.clearErrorIDs, id)
	r.accounts[id].Status = service.StatusActive
	r.accounts[id].ErrorMessage = ""
	return nil
}

func (r *userAccountRecoveryRepoStub) ClearRateLimit(_ context.Context, id int64) error {
	r.clearRateLimitIDs = append(r.clearRateLimitIDs, id)
	if err := r.clearRateLimitErr[id]; err != nil {
		return err
	}
	r.accounts[id].RateLimitedAt = nil
	r.accounts[id].RateLimitResetAt = nil
	r.accounts[id].OverloadUntil = nil
	return nil
}

func (r *userAccountRecoveryRepoStub) ClearAntigravityQuotaScopes(_ context.Context, id int64) error {
	r.clearAntigravityIDs = append(r.clearAntigravityIDs, id)
	return nil
}

func (r *userAccountRecoveryRepoStub) ClearModelRateLimits(_ context.Context, id int64) error {
	r.clearModelLimitIDs = append(r.clearModelLimitIDs, id)
	return nil
}

func (r *userAccountRecoveryRepoStub) ClearTempUnschedulable(_ context.Context, id int64) error {
	r.clearTempUnschedIDs = append(r.clearTempUnschedIDs, id)
	r.accounts[id].TempUnschedulableUntil = nil
	r.accounts[id].TempUnschedulableReason = ""
	return nil
}

func newUserAccountRecoveryHandler(repo *userAccountRecoveryRepoStub) *UserAccountHandler {
	accountSvc := service.NewAccountService(repo, nil, nil, nil)
	h := NewUserAccountHandler(accountSvc, nil, nil, nil, nil, nil, nil)
	h.SetRateLimitService(service.NewRateLimitService(repo, nil, nil, nil, nil))
	return h
}

func newOwnedAccountRequestContext(t *testing.T, method, target, body string, ownerUserID int64) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: ownerUserID})
	return c, recorder
}

func TestUserAccountHandlerRecoverStateClearsOwnedRuntimeState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	resetAt := time.Now().Add(time.Hour)
	repo := &userAccountRecoveryRepoStub{
		accounts: map[int64]*service.Account{
			42: {
				ID:               42,
				OwnerUserID:      &ownerID,
				Platform:         service.PlatformOpenAI,
				Type:             service.AccountTypeOAuth,
				Status:           service.StatusError,
				ErrorMessage:     "rate limited",
				RateLimitResetAt: &resetAt,
			},
		},
		clearRateLimitErr: map[int64]error{},
	}
	h := newUserAccountRecoveryHandler(repo)
	c, recorder := newOwnedAccountRequestContext(t, http.MethodPost, "/api/v1/accounts/42/recover-state", "", ownerID)
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	h.RecoverState(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int64{42}, repo.clearErrorIDs)
	require.Equal(t, []int64{42}, repo.clearRateLimitIDs)
	require.Nil(t, repo.accounts[42].RateLimitResetAt)
	require.Equal(t, service.StatusActive, repo.accounts[42].Status)
}

func TestUserAccountHandlerRecoverStateRejectsForeignAccountBeforeMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	otherOwnerID := int64(202)
	resetAt := time.Now().Add(time.Hour)
	repo := &userAccountRecoveryRepoStub{
		accounts: map[int64]*service.Account{
			42: {ID: 42, OwnerUserID: &otherOwnerID, Status: service.StatusError, RateLimitResetAt: &resetAt},
		},
		clearRateLimitErr: map[int64]error{},
	}
	h := newUserAccountRecoveryHandler(repo)
	c, recorder := newOwnedAccountRequestContext(t, http.MethodPost, "/api/v1/accounts/42/recover-state", "", ownerID)
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	h.RecoverState(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Empty(t, repo.clearErrorIDs)
	require.Empty(t, repo.clearRateLimitIDs)
}

func TestUserAccountHandlerSuccessfulTestRecoversOwnedRuntimeState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	resetAt := time.Now().Add(time.Hour)
	repo := &userAccountRecoveryRepoStub{
		accounts: map[int64]*service.Account{
			42: {
				ID:               42,
				OwnerUserID:      &ownerID,
				Platform:         service.PlatformOpenAI,
				Type:             service.AccountTypeOAuth,
				Status:           service.StatusError,
				ErrorMessage:     "rate limited",
				RateLimitResetAt: &resetAt,
				Concurrency:      1,
				Credentials:      map[string]any{"access_token": "test-token"},
			},
		},
		clearRateLimitErr: map[int64]error{},
	}
	accountSvc := service.NewAccountService(repo, nil, nil, nil)
	accountTestSvc := service.NewAccountTestService(repo, nil, nil, nil, nil, userAccountTestHTTPUpstream{}, &config.Config{}, nil)
	h := NewUserAccountHandler(accountSvc, nil, accountTestSvc, nil, nil, nil, nil)
	h.SetRateLimitService(service.NewRateLimitService(repo, nil, nil, nil, nil))
	c, recorder := newOwnedAccountRequestContext(t, http.MethodPost, "/api/v1/accounts/42/test", `{}`, ownerID)
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	h.Test(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "test_complete")
	require.Equal(t, []int64{42}, repo.clearErrorIDs)
	require.Equal(t, []int64{42}, repo.clearRateLimitIDs)
	require.Nil(t, repo.accounts[42].RateLimitResetAt)
}

func TestUserAccountHandlerBatchRecoverStatePrevalidatesCompleteOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	otherOwnerID := int64(202)
	resetAt := time.Now().Add(time.Hour)
	repo := &userAccountRecoveryRepoStub{
		accounts: map[int64]*service.Account{
			1: {ID: 1, OwnerUserID: &ownerID, RateLimitResetAt: &resetAt},
			2: {ID: 2, OwnerUserID: &otherOwnerID, RateLimitResetAt: &resetAt},
		},
		clearRateLimitErr: map[int64]error{},
	}
	h := newUserAccountRecoveryHandler(repo)
	c, recorder := newOwnedAccountRequestContext(t, http.MethodPost, "/api/v1/accounts/batch-recover-state", `{"account_ids":[1,2]}`, ownerID)

	h.BatchRecoverState(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Empty(t, repo.clearRateLimitIDs, "no account may be mutated when batch ownership validation fails")
}

func TestUserAccountHandlerBatchRecoverStateReturnsPerAccountResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	resetAt := time.Now().Add(time.Hour)
	repo := &userAccountRecoveryRepoStub{
		accounts: map[int64]*service.Account{
			1: {ID: 1, OwnerUserID: &ownerID, RateLimitResetAt: &resetAt},
			2: {ID: 2, OwnerUserID: &ownerID, RateLimitResetAt: &resetAt},
		},
		clearRateLimitErr: map[int64]error{2: errors.New("database unavailable")},
	}
	h := newUserAccountRecoveryHandler(repo)
	c, recorder := newOwnedAccountRequestContext(t, http.MethodPost, "/api/v1/accounts/batch-recover-state", `{"account_ids":[2,1,1]}`, ownerID)

	h.BatchRecoverState(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":1`)
	require.Contains(t, recorder.Body.String(), `"failed":1`)
	require.Contains(t, recorder.Body.String(), `"success_ids":[1]`)
	require.Contains(t, recorder.Body.String(), `"failed_ids":[2]`)
}

func TestUserAccountHandlerOpenAIQuotaEndpointsEnforceOwnerAndAccountType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	otherOwnerID := int64(202)
	repo := &userAccountRecoveryRepoStub{
		accounts: map[int64]*service.Account{
			1: {ID: 1, OwnerUserID: &otherOwnerID, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth},
			2: {ID: 2, OwnerUserID: &ownerID, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth},
			3: {ID: 3, OwnerUserID: &ownerID, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey},
			4: {ID: 4, OwnerUserID: &ownerID, Platform: service.PlatformOpenAI, Type: service.AccountTypeSetupToken},
		},
		clearRateLimitErr: map[int64]error{},
	}
	h := newUserAccountRecoveryHandler(repo)
	h.SetOpenAIQuotaService(&service.OpenAIQuotaService{})

	for _, tt := range []struct {
		name       string
		accountID  string
		handler    func(*gin.Context)
		wantStatus int
	}{
		{name: "query foreign", accountID: "1", handler: h.QueryOpenAIQuota, wantStatus: http.StatusNotFound},
		{name: "reset foreign", accountID: "1", handler: h.ResetOpenAIQuota, wantStatus: http.StatusNotFound},
		{name: "query non OpenAI", accountID: "2", handler: h.QueryOpenAIQuota, wantStatus: http.StatusBadRequest},
		{name: "reset non OAuth", accountID: "3", handler: h.ResetOpenAIQuota, wantStatus: http.StatusBadRequest},
		{name: "query setup token", accountID: "4", handler: h.QueryOpenAIQuota, wantStatus: http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := newOwnedAccountRequestContext(t, http.MethodPost, "/api/v1/accounts/"+tt.accountID+"/quota", "", ownerID)
			c.Params = gin.Params{{Key: "id", Value: tt.accountID}}

			tt.handler(c)

			require.Equal(t, tt.wantStatus, recorder.Code)
		})
	}
}
