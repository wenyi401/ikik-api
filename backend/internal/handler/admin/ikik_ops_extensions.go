package admin

import (
	"errors"

	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

// ListRetryAttempts lists retry attempts for an error log.
// GET /api/v1/admin/ops/errors/:id/retries
func (h *OpsHandler) ListRetryAttempts(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid error id")
		return
	}

	limit := 50
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			response.BadRequest(c, "Invalid limit")
			return
		}
		limit = n
	}

	items, err := h.opsService.ListRetryAttemptsByErrorID(c.Request.Context(), id, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// RetryErrorRequest retries a failed request using stored request_body.
// POST /api/v1/admin/ops/errors/:id/retry
func (h *OpsHandler) RetryErrorRequest(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid error id")
		return
	}

	req := opsRetryRequest{Mode: service.OpsRetryModeClient}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Mode) == "" {
		req.Mode = service.OpsRetryModeClient
	}

	_ = req.Force

	if strings.EqualFold(strings.TrimSpace(req.Mode), service.OpsRetryModeUpstream) {
		response.BadRequest(c, "upstream retry is not supported on this endpoint")
		return
	}

	result, err := h.opsService.RetryError(c.Request.Context(), subject.UserID, id, req.Mode, req.PinnedAccountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// RetryRequestErrorClient retries the client request based on stored request body.
// POST /api/v1/admin/ops/request-errors/:id/retry-client
func (h *OpsHandler) RetryRequestErrorClient(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid error id")
		return
	}

	result, err := h.opsService.RetryError(c.Request.Context(), subject.UserID, id, service.OpsRetryModeClient, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// RetryRequestErrorUpstreamEvent retries a specific upstream attempt using captured upstream_request_body.
// POST /api/v1/admin/ops/request-errors/:id/upstream-errors/:idx/retry
func (h *OpsHandler) RetryRequestErrorUpstreamEvent(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid error id")
		return
	}

	idxStr := strings.TrimSpace(c.Param("idx"))
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 {
		response.BadRequest(c, "Invalid upstream idx")
		return
	}

	result, err := h.opsService.RetryUpstreamEvent(c.Request.Context(), subject.UserID, id, idx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// RetryUpstreamError retries upstream error using the original account_id.
// POST /api/v1/admin/ops/upstream-errors/:id/retry
func (h *OpsHandler) RetryUpstreamError(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid error id")
		return
	}

	result, err := h.opsService.RetryError(c.Request.Context(), subject.UserID, id, service.OpsRetryModeUpstream, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
