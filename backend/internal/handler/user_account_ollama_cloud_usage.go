package handler

import (
	"strconv"

	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type userOllamaCloudUsageSessionRequest struct {
	Session string `json:"session" binding:"required"`
}

type userOllamaCloudUsageAutoRefreshRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

func (h *UserAccountHandler) GetOllamaCloudUsage(c *gin.Context) {
	accountID, ok := h.requireOwnedOllamaCloudUsageAccount(c)
	if !ok {
		return
	}
	state, err := h.ollamaCloudUsage.GetState(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func (h *UserAccountHandler) SaveOllamaCloudUsageSession(c *gin.Context) {
	accountID, ok := h.requireOwnedOllamaCloudUsageAccount(c)
	if !ok {
		return
	}
	var req userOllamaCloudUsageSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	state, err := h.ollamaCloudUsage.SaveSession(c.Request.Context(), accountID, req.Session)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func (h *UserAccountHandler) DeleteOllamaCloudUsageSession(c *gin.Context) {
	accountID, ok := h.requireOwnedOllamaCloudUsageAccount(c)
	if !ok {
		return
	}
	state, err := h.ollamaCloudUsage.DeleteSession(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func (h *UserAccountHandler) SetOllamaCloudUsageAutoRefresh(c *gin.Context) {
	accountID, ok := h.requireOwnedOllamaCloudUsageAccount(c)
	if !ok {
		return
	}
	var req userOllamaCloudUsageAutoRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	state, err := h.ollamaCloudUsage.SetAutoRefresh(c.Request.Context(), accountID, *req.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func (h *UserAccountHandler) RefreshOllamaCloudUsage(c *gin.Context) {
	accountID, ok := h.requireOwnedOllamaCloudUsageAccount(c)
	if !ok {
		return
	}
	state, err := h.ollamaCloudUsage.Refresh(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func (h *UserAccountHandler) requireOwnedOllamaCloudUsageAccount(c *gin.Context) (int64, bool) {
	if h == nil || h.ollamaCloudUsage == nil || h.accountService == nil {
		response.ErrorFrom(c, service.ErrOllamaCloudUsageUnavailable)
		return 0, false
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return 0, false
	}
	if _, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID); err != nil {
		response.ErrorFrom(c, err)
		return 0, false
	}
	return accountID, true
}
