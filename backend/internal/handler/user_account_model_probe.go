package handler

import (
	"net/http"

	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type userModelProbeListRequest struct {
	Platform string `json:"platform" binding:"required"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key" binding:"required"`
}

type userModelProbeTestRequest struct {
	Platform string   `json:"platform" binding:"required"`
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key" binding:"required"`
	Mode     string   `json:"mode"`
	Models   []string `json:"models" binding:"required"`
}

// ProbeModelList discovers models using temporary credentials without storing them.
// POST /api/v1/accounts/model-probe/list
func (h *UserAccountHandler) ProbeModelList(c *gin.Context) {
	if h.accountTestService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Account test service unavailable")
		return
	}

	var req userModelProbeListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.accountTestService.ProbeModelListForUser(c.Request.Context(), service.ModelProbeListInput{
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

// ProbeModels validates selected models using temporary credentials without storing them.
// POST /api/v1/accounts/model-probe/test
func (h *UserAccountHandler) ProbeModels(c *gin.Context) {
	if h.accountTestService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Account test service unavailable")
		return
	}

	var req userModelProbeTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.accountTestService.ProbeModelsForUser(c.Request.Context(), service.ModelProbeTestInput{
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
