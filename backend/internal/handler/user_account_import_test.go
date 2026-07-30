package handler

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"ikik-api/internal/pkg/pagination"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

type userAccountImportRepoStub struct {
	*userAccountBatchRepoStub
	created []*service.Account
	bound   map[int64][]int64
}

func newUserAccountImportRepoStub() *userAccountImportRepoStub {
	return &userAccountImportRepoStub{
		userAccountBatchRepoStub: &userAccountBatchRepoStub{accounts: map[int64]*service.Account{}},
		bound:                    map[int64][]int64{},
	}
}

func (s *userAccountImportRepoStub) Create(_ context.Context, account *service.Account) error {
	account.ID = int64(len(s.created) + 1)
	cp := *account
	s.created = append(s.created, &cp)
	s.accounts[account.ID] = &cp
	return nil
}

func (s *userAccountImportRepoStub) BindGroups(_ context.Context, accountID int64, groupIDs []int64) error {
	s.bound[accountID] = append([]int64(nil), groupIDs...)
	return nil
}

func (s *userAccountImportRepoStub) ListOwnedWithFilters(
	_ context.Context,
	ownerUserID int64,
	params pagination.PaginationParams,
	platform, accountType, _, _ string,
	_, _ int64,
	_ string,
) ([]service.Account, *pagination.PaginationResult, error) {
	accounts := make([]service.Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		if account == nil || account.OwnerUserID == nil || *account.OwnerUserID != ownerUserID {
			continue
		}
		if platform != "" && account.Platform != platform {
			continue
		}
		if accountType != "" && account.Type != accountType {
			continue
		}
		accounts = append(accounts, *account)
	}
	total := int64(len(accounts))
	start := params.Offset()
	if start >= len(accounts) {
		return nil, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.Limit()}, nil
	}
	end := start + params.Limit()
	if end > len(accounts) {
		end = len(accounts)
	}
	return accounts[start:end], &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.Limit()}, nil
}

type userAccountImportPrivateGroupProvisioner struct {
	group service.Group
}

func (p *userAccountImportPrivateGroupProvisioner) ProvisionUserPrivateGroups(context.Context, int64) error {
	return nil
}

func (p *userAccountImportPrivateGroupProvisioner) GetActiveUserPrivateGroup(_ context.Context, _ int64, platform string) (*service.Group, error) {
	cp := p.group
	cp.Platform = platform
	return &cp, nil
}

func TestUserAccountImportCredentialsCreatesOwnedClaudeWebAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousCoordinator := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(nil)
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previousCoordinator) })

	ownerID := int64(101)
	repo := newUserAccountImportRepoStub()
	accountService := service.NewAccountService(repo, nil, nil, nil)
	accountService.SetUserPrivateGroupProvisioner(&userAccountImportPrivateGroupProvisioner{
		group: service.Group{ID: 99, Status: service.StatusActive, Scope: service.GroupScopeUserPrivate},
	})
	h := NewUserAccountHandler(accountService, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/accounts/import-credentials", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: ownerID})
		h.ImportCredentials(c)
	})

	body := []byte(`{
		"contents":["{\"email\":\"owned@example.com\",\"cookies\":{\"sessionKey\":\"sk-ant-sid02-owned\"}}"],
		"claude_web_import":true,
		"claude_web_auth_mode":"session_key",
		"share_mode":"private"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/accounts/import-credentials", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Created int `json:"created"`
			Skipped int `json:"skipped"`
			Failed  int `json:"failed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Zero(t, envelope.Code)
	require.Equal(t, 1, envelope.Data.Created)
	require.Zero(t, envelope.Data.Failed)
	require.Len(t, repo.created, 1)
	require.Equal(t, &ownerID, repo.created[0].OwnerUserID)
	require.Equal(t, service.AccountShareModePrivate, repo.created[0].ShareMode)
	require.True(t, repo.created[0].IsClaudeWebSession())
	require.Equal(t, []int64{99}, repo.bound[repo.created[0].ID])
}

func TestUserAccountImportCredentialsAutoDetectsAgentIdentityK12(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousCoordinator := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(nil)
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previousCoordinator) })

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	ownerID := int64(102)
	repo := newUserAccountImportRepoStub()
	accountService := service.NewAccountService(repo, nil, nil, nil)
	accountService.SetUserPrivateGroupProvisioner(&userAccountImportPrivateGroupProvisioner{
		group: service.Group{ID: 100, Status: service.StatusActive, Scope: service.GroupScopeUserPrivate},
	})
	h := NewUserAccountHandler(accountService, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/accounts/import-credentials", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: ownerID})
		h.ImportCredentials(c)
	})

	credential := map[string]any{
		"auth_mode": "agentIdentity",
		"agent_identity": map[string]any{
			"agent_runtime_id":  "runtime-k12",
			"agent_private_key": base64.StdEncoding.EncodeToString(privateKeyDER),
			"task_id":           "task-k12",
			"account_id":        "account-k12",
			"chatgpt_user_id":   "user-k12",
			"email":             "k12@example.com",
			"plan_type":         "k12",
		},
	}
	credentialJSON, err := json.Marshal(credential)
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{
		"contents":   []string{string(credentialJSON)},
		"share_mode": "private",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/accounts/import-credentials", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Created int `json:"created"`
			Failed  int `json:"failed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Zero(t, envelope.Code)
	require.Equal(t, 1, envelope.Data.Created)
	require.Zero(t, envelope.Data.Failed)
	require.Len(t, repo.created, 1)
	require.Equal(t, &ownerID, repo.created[0].OwnerUserID)
	require.Equal(t, service.AccountLevelK12, repo.created[0].AccountLevel)
	require.Equal(t, service.OpenAIAuthModeAgentIdentity, repo.created[0].Credentials["auth_mode"])
	require.Equal(t, service.AccountShareModePrivate, repo.created[0].ShareMode)
	require.Equal(t, []int64{100}, repo.bound[repo.created[0].ID])
}

func TestUserAccountImportCredentialsAcceptsSub2APIDataAgentIdentityExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousCoordinator := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(nil)
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previousCoordinator) })

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	ownerID := int64(103)
	repo := newUserAccountImportRepoStub()
	accountService := service.NewAccountService(repo, nil, nil, nil)
	accountService.SetUserPrivateGroupProvisioner(&userAccountImportPrivateGroupProvisioner{
		group: service.Group{ID: 101, Status: service.StatusActive, Scope: service.GroupScopeUserPrivate},
	})
	h := NewUserAccountHandler(accountService, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/accounts/import-credentials", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: ownerID})
		h.ImportCredentials(c)
	})

	exportContent, err := json.Marshal(map[string]any{
		"type":    "sub2api-data",
		"version": 1,
		"accounts": []any{
			map[string]any{
				"name":     "Owned K12 export one",
				"platform": service.PlatformOpenAI,
				"type":     service.AccountTypeOAuth,
				"credentials": map[string]any{
					"auth_mode":         service.OpenAIAuthModeAgentIdentity,
					"agent_runtime_id":  "runtime-export-k12-one",
					"agent_private_key": base64.StdEncoding.EncodeToString(privateKeyDER),
					"task_id":           "task-export-k12-one",
					"account_id":        "account-export-k12",
					"chatgpt_user_id":   "user-export-k12-one",
					"email":             "export-k12-one@example.com",
					"plan_type":         "k12",
				},
				"extra": map[string]any{"source": "agent_identity_import"},
			},
			map[string]any{
				"name":     "Owned K12 export two",
				"platform": service.PlatformOpenAI,
				"type":     service.AccountTypeOAuth,
				"credentials": map[string]any{
					"auth_mode":         service.OpenAIAuthModeAgentIdentity,
					"agent_runtime_id":  "runtime-export-k12-two",
					"agent_private_key": base64.StdEncoding.EncodeToString(privateKeyDER),
					"task_id":           "task-export-k12-two",
					"account_id":        "account-export-k12",
					"chatgpt_user_id":   "user-export-k12-two",
					"email":             "export-k12-two@example.com",
					"plan_type":         "k12",
				},
				"extra": map[string]any{"source": "agent_identity_import"},
			},
		},
	})
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{
		"contents":   []string{string(exportContent)},
		"share_mode": "private",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/accounts/import-credentials", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Created int `json:"created"`
			Failed  int `json:"failed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Zero(t, envelope.Code)
	require.Equal(t, 2, envelope.Data.Created)
	require.Zero(t, envelope.Data.Failed)
	require.Len(t, repo.created, 2)
	for _, account := range repo.created {
		require.Equal(t, &ownerID, account.OwnerUserID)
		require.Equal(t, service.AccountLevelK12, account.AccountLevel)
		require.Equal(t, service.AccountShareModePrivate, account.ShareMode)
		require.Equal(t, []int64{101}, repo.bound[account.ID])
	}
	require.Equal(t, "Owned K12 export one #1", repo.created[0].Name)
	require.Equal(t, "Owned K12 export two #2", repo.created[1].Name)

	retryRepo := newUserAccountImportRepoStub()
	existing := *repo.created[0]
	existing.ID = 37024
	retryRepo.accounts[existing.ID] = &existing
	retryService := service.NewAccountService(retryRepo, nil, nil, nil)
	retryService.SetUserPrivateGroupProvisioner(&userAccountImportPrivateGroupProvisioner{
		group: service.Group{ID: 101, Status: service.StatusActive, Scope: service.GroupScopeUserPrivate},
	})
	retryHandler := NewUserAccountHandler(retryService, nil, nil, nil, nil, nil, nil)
	retryRouter := gin.New()
	retryRouter.POST("/accounts/import-credentials", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: ownerID})
		retryHandler.ImportCredentials(c)
	})
	retryRec := httptest.NewRecorder()
	retryReq := httptest.NewRequest(http.MethodPost, "/accounts/import-credentials", bytes.NewReader(body))
	retryReq.Header.Set("Content-Type", "application/json")
	retryRouter.ServeHTTP(retryRec, retryReq)

	require.Equal(t, http.StatusOK, retryRec.Code, retryRec.Body.String())
	var retryEnvelope struct {
		Code int `json:"code"`
		Data struct {
			Created int `json:"created"`
			Skipped int `json:"skipped"`
			Failed  int `json:"failed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(retryRec.Body.Bytes(), &retryEnvelope))
	require.Zero(t, retryEnvelope.Code)
	require.Equal(t, 1, retryEnvelope.Data.Created, retryRec.Body.String())
	require.Equal(t, 1, retryEnvelope.Data.Skipped, retryRec.Body.String())
	require.Zero(t, retryEnvelope.Data.Failed, retryRec.Body.String())
	require.Len(t, retryRepo.created, 1)
	require.Equal(t, "user-export-k12-two", retryRepo.created[0].Credentials["chatgpt_user_id"])
}

func TestUserAccountImportDataRejectsRemoteSourceURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUserAccountHandler(nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/accounts/data", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 101})
		h.ImportData(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/accounts/data", bytes.NewBufferString(`{"source_url":"http://127.0.0.1/internal"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "source_url is not supported")
}
