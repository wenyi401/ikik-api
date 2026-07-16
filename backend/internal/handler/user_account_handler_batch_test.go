package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ikik-api/internal/pkg/pagination"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userAccountBatchRepoStub struct {
	accounts         map[int64]*service.Account
	createdTask      service.CreateAccountBatchTaskInput
	createTaskCalled int
}

func (s *userAccountBatchRepoStub) Create(_ context.Context, account *service.Account) error {
	s.accounts[account.ID] = account
	return nil
}

func (s *userAccountBatchRepoStub) GetByID(_ context.Context, id int64) (*service.Account, error) {
	account := s.accounts[id]
	if account == nil {
		return nil, service.ErrAccountNotFound
	}
	cp := *account
	return &cp, nil
}

func (s *userAccountBatchRepoStub) GetByIDs(_ context.Context, ids []int64) ([]*service.Account, error) {
	out := make([]*service.Account, 0, len(ids))
	for _, id := range ids {
		if account := s.accounts[id]; account != nil {
			cp := *account
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *userAccountBatchRepoStub) CreateTask(_ context.Context, input service.CreateAccountBatchTaskInput) (*service.AccountBatchTask, error) {
	s.createTaskCalled++
	s.createdTask = input
	return &service.AccountBatchTask{
		ID:          77,
		Scope:       input.Scope,
		Operation:   input.Operation,
		Status:      service.AccountBatchTaskStatusPending,
		Total:       len(input.AccountIDs),
		CreatedBy:   input.CreatedBy,
		OwnerUserID: input.OwnerUserID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (s *userAccountBatchRepoStub) ExistsByID(context.Context, int64) (bool, error) {
	panic("unexpected ExistsByID call")
}
func (s *userAccountBatchRepoStub) GetByCRSAccountID(context.Context, string) (*service.Account, error) {
	panic("unexpected GetByCRSAccountID call")
}
func (s *userAccountBatchRepoStub) FindByExtraField(context.Context, string, any) ([]service.Account, error) {
	panic("unexpected FindByExtraField call")
}
func (s *userAccountBatchRepoStub) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	panic("unexpected ListCRSAccountIDs call")
}
func (s *userAccountBatchRepoStub) Update(context.Context, *service.Account) error {
	panic("unexpected Update call")
}
func (s *userAccountBatchRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *userAccountBatchRepoStub) List(context.Context, pagination.PaginationParams) ([]service.Account, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *userAccountBatchRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, int64, string) ([]service.Account, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *userAccountBatchRepoStub) ListByGroup(context.Context, int64) ([]service.Account, error) {
	panic("unexpected ListByGroup call")
}
func (s *userAccountBatchRepoStub) ListActive(context.Context) ([]service.Account, error) {
	panic("unexpected ListActive call")
}
func (s *userAccountBatchRepoStub) ListByPlatform(context.Context, string) ([]service.Account, error) {
	panic("unexpected ListByPlatform call")
}
func (s *userAccountBatchRepoStub) UpdateLastUsed(context.Context, int64) error {
	panic("unexpected UpdateLastUsed call")
}
func (s *userAccountBatchRepoStub) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	panic("unexpected BatchUpdateLastUsed call")
}
func (s *userAccountBatchRepoStub) SetError(context.Context, int64, string) error {
	panic("unexpected SetError call")
}
func (s *userAccountBatchRepoStub) ClearError(context.Context, int64) error {
	panic("unexpected ClearError call")
}
func (s *userAccountBatchRepoStub) SetSchedulable(context.Context, int64, bool) error {
	panic("unexpected SetSchedulable call")
}
func (s *userAccountBatchRepoStub) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	panic("unexpected AutoPauseExpiredAccounts call")
}
func (s *userAccountBatchRepoStub) BindGroups(context.Context, int64, []int64) error {
	panic("unexpected BindGroups call")
}
func (s *userAccountBatchRepoStub) ListSchedulable(context.Context) ([]service.Account, error) {
	panic("unexpected ListSchedulable call")
}
func (s *userAccountBatchRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]service.Account, error) {
	panic("unexpected ListSchedulableByGroupID call")
}
func (s *userAccountBatchRepoStub) ListSchedulableByPlatform(context.Context, string) ([]service.Account, error) {
	panic("unexpected ListSchedulableByPlatform call")
}
func (s *userAccountBatchRepoStub) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]service.Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatform call")
}
func (s *userAccountBatchRepoStub) ListSchedulableByPlatforms(context.Context, []string) ([]service.Account, error) {
	panic("unexpected ListSchedulableByPlatforms call")
}
func (s *userAccountBatchRepoStub) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]service.Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatforms call")
}
func (s *userAccountBatchRepoStub) ListSchedulableUngroupedByPlatform(context.Context, string) ([]service.Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatform call")
}
func (s *userAccountBatchRepoStub) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]service.Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatforms call")
}
func (s *userAccountBatchRepoStub) SetRateLimited(context.Context, int64, time.Time) error {
	panic("unexpected SetRateLimited call")
}
func (s *userAccountBatchRepoStub) SetModelRateLimit(context.Context, int64, string, time.Time) error {
	panic("unexpected SetModelRateLimit call")
}
func (s *userAccountBatchRepoStub) SetOverloaded(context.Context, int64, time.Time) error {
	panic("unexpected SetOverloaded call")
}
func (s *userAccountBatchRepoStub) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	panic("unexpected SetTempUnschedulable call")
}
func (s *userAccountBatchRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	panic("unexpected ClearTempUnschedulable call")
}
func (s *userAccountBatchRepoStub) ClearRateLimit(context.Context, int64) error {
	panic("unexpected ClearRateLimit call")
}
func (s *userAccountBatchRepoStub) ClearAntigravityQuotaScopes(context.Context, int64) error {
	panic("unexpected ClearAntigravityQuotaScopes call")
}
func (s *userAccountBatchRepoStub) ClearModelRateLimits(context.Context, int64) error {
	panic("unexpected ClearModelRateLimits call")
}
func (s *userAccountBatchRepoStub) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	panic("unexpected UpdateSessionWindow call")
}
func (s *userAccountBatchRepoStub) UpdateExtra(context.Context, int64, map[string]any) error {
	panic("unexpected UpdateExtra call")
}
func (s *userAccountBatchRepoStub) BulkUpdate(context.Context, []int64, service.AccountBulkUpdate) (int64, error) {
	panic("unexpected BulkUpdate call")
}
func (s *userAccountBatchRepoStub) IncrementQuotaUsed(context.Context, int64, float64) error {
	panic("unexpected IncrementQuotaUsed call")
}
func (s *userAccountBatchRepoStub) ResetQuotaUsed(context.Context, int64) error {
	panic("unexpected ResetQuotaUsed call")
}

func (s *userAccountBatchRepoStub) GetTask(context.Context, int64) (*service.AccountBatchTask, error) {
	panic("unexpected GetTask call")
}
func (s *userAccountBatchRepoStub) ClaimNextPendingTask(context.Context, int64) (*service.AccountBatchTask, error) {
	return nil, nil
}
func (s *userAccountBatchRepoStub) ListPendingItems(context.Context, int64) ([]service.AccountBatchTaskItem, error) {
	panic("unexpected ListPendingItems call")
}
func (s *userAccountBatchRepoStub) MarkItemRunning(context.Context, int64) error {
	panic("unexpected MarkItemRunning call")
}
func (s *userAccountBatchRepoStub) MarkItemSucceeded(context.Context, int64, map[string]any) error {
	panic("unexpected MarkItemSucceeded call")
}
func (s *userAccountBatchRepoStub) MarkItemFailed(context.Context, int64, string) error {
	panic("unexpected MarkItemFailed call")
}
func (s *userAccountBatchRepoStub) RefreshTaskProgress(context.Context, int64) (*service.AccountBatchTask, error) {
	panic("unexpected RefreshTaskProgress call")
}
func (s *userAccountBatchRepoStub) MarkTaskSucceeded(context.Context, int64) error {
	panic("unexpected MarkTaskSucceeded call")
}
func (s *userAccountBatchRepoStub) MarkTaskFailed(context.Context, int64, string) error {
	panic("unexpected MarkTaskFailed call")
}

func TestUserAccountHandlerBulkPublicShareOnlyCreatesAsyncTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(101)
	repo := &userAccountBatchRepoStub{
		accounts: map[int64]*service.Account{
			1: {ID: 1, OwnerUserID: &ownerID, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Credentials: map[string]any{"access_token": "token-1"}},
			2: {ID: 2, OwnerUserID: &ownerID, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Credentials: map[string]any{"access_token": "token-2"}},
		},
	}
	accountSvc := service.NewAccountService(repo, nil, nil, nil)
	batchSvc := service.NewAccountBatchTaskService(repo, nil)
	handler := NewUserAccountHandler(accountSvc, nil, nil, nil, nil, nil, nil, batchSvc)
	router := gin.New()
	router.POST("/accounts/bulk-update", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: ownerID})
		handler.BulkUpdate(c)
	})

	body := []byte(`{"account_ids":[1,2],"share_mode":"public"}`)
	req := httptest.NewRequest(http.MethodPost, "/accounts/bulk-update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, repo.createTaskCalled)
	require.Equal(t, service.AccountBatchTaskOperationUserSetPublicShare, repo.createdTask.Operation)
	require.Equal(t, []int64{1, 2}, repo.createdTask.AccountIDs)
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Async bool `json:"async"`
			Task  struct {
				ID        int64  `json:"id"`
				Operation string `json:"operation"`
				Total     int    `json:"total"`
			} `json:"task"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)
	require.True(t, envelope.Data.Async)
	require.Equal(t, int64(77), envelope.Data.Task.ID)
	require.Equal(t, service.AccountBatchTaskOperationUserSetPublicShare, envelope.Data.Task.Operation)
	require.Equal(t, 2, envelope.Data.Task.Total)
}
