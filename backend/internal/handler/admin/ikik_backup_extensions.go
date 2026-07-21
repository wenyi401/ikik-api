package admin

import (
	"ikik-api/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"ikik-api/internal/service"
)

func (h *BackupHandler) GetUsageRetention(c *gin.Context) {
	cfg, err := h.backupService.GetUsageRetention(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *BackupHandler) UpdateUsageRetention(c *gin.Context) {
	var req service.UsageRetentionConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.backupService.UpdateUsageRetention(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}
