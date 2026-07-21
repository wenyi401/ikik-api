package admin

import (
	"context"
	"fmt"

	"net/http"
	"strconv"

	"ikik-api/internal/pkg/response"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type accountQuotaDashboardService interface {
	GetAccountQuotaDashboard(ctx context.Context) (*service.AccountQuotaDashboard, error)
}

// CreateBatchRefreshTask creates an async account credential refresh task.
// POST /api/v1/admin/accounts/batch-refresh/async
func (h *AccountHandler) CreateBatchRefreshTask(c *gin.Context) {
	if h.accountBatchTaskService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Account batch task service is unavailable")
		return
	}
	var req struct {
		AccountIDs []int64 `json:"account_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	accountIDs := normalizeInt64IDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}
	accounts, err := h.adminService.GetAccountsByIDs(c.Request.Context(), accountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	found := make(map[int64]struct{}, len(accounts))
	for _, account := range accounts {
		if account != nil {
			found[account.ID] = struct{}{}
		}
	}
	for _, accountID := range accountIDs {
		if _, ok := found[accountID]; !ok {
			response.BadRequest(c, fmt.Sprintf("account not found: %d", accountID))
			return
		}
	}
	createdBy, ok := currentAdminUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Invalid admin identity")
		return
	}
	task, err := h.accountBatchTaskService.CreateTask(c.Request.Context(), service.CreateAccountBatchTaskInput{
		Scope:      service.AccountBatchTaskScopeAdmin,
		Operation:  service.AccountBatchTaskOperationAdminRefreshCredentials,
		AccountIDs: accountIDs,
		CreatedBy:  createdBy,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Accepted(c, task)
}

// GetBatchTask returns an admin account batch task with item results.
// GET /api/v1/admin/accounts/batch-tasks/:task_id
func (h *AccountHandler) GetBatchTask(c *gin.Context) {
	if h.accountBatchTaskService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Account batch task service is unavailable")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("task_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid task ID")
		return
	}
	task, err := h.accountBatchTaskService.GetTask(c.Request.Context(), taskID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if task.Scope != service.AccountBatchTaskScopeAdmin {
		response.NotFound(c, "Account batch task not found")
		return
	}
	response.Success(c, task)
}

// GetQuotaDashboard returns account quota summaries grouped by platform and account type.
// GET /api/v1/admin/accounts/quota-dashboard
func (h *AccountHandler) GetQuotaDashboard(c *gin.Context) {
	dashboardService, ok := h.adminService.(accountQuotaDashboardService)
	if !ok {
		response.Error(c, http.StatusServiceUnavailable, "Account quota dashboard is unavailable")
		return
	}
	dashboard, err := dashboardService.GetAccountQuotaDashboard(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dashboard)
}

// ProbeModelList discovers upstream models with a temporary URL and API key.
// POST /api/v1/admin/accounts/model-probe/list
func (h *AccountHandler) ProbeModelList(c *gin.Context) {
	if h.accountTestService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Account test service unavailable")
		return
	}

	var req ModelProbeListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.accountTestService.ProbeModelList(c.Request.Context(), service.ModelProbeListInput{
		Platform: req.Platform,
		BaseURL:  req.BaseURL,
		APIKey:   req.APIKey,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// ProbeModels validates selected models with a minimal upstream request.
// POST /api/v1/admin/accounts/model-probe/test
func (h *AccountHandler) ProbeModels(c *gin.Context) {
	if h.accountTestService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Account test service unavailable")
		return
	}

	var req ModelProbeTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.accountTestService.ProbeModels(c.Request.Context(), service.ModelProbeTestInput{
		Platform: req.Platform,
		BaseURL:  req.BaseURL,
		APIKey:   req.APIKey,
		Mode:     req.Mode,
		Models:   req.Models,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}
