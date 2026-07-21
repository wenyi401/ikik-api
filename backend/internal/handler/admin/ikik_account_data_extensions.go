package admin

import (
	"context"

	"fmt"

	"github.com/gin-gonic/gin"

	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"
)

func (h *AccountHandler) ImportCredentials(c *gin.Context) {
	var req CredentialImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	if req.Priority <= 0 {
		req.Priority = 50
	}

	sources, parseErrors := service.ParseAccountCredentialImportContentsWithOptions(req.Contents, service.AccountCredentialImportOptions{
		KiroConfigImport:  req.KiroConfigImport,
		ClaudeWebImport:   req.ClaudeWebImport,
		ClaudeWebAuthMode: req.ClaudeWebAuthMode,
	})
	if len(sources) == 0 && len(parseErrors) == 0 {
		response.BadRequest(c, "No importable account credentials found")
		return
	}
	if len(sources) > service.MaxAccountCredentialImportItems {
		response.BadRequest(c, fmt.Sprintf("Too many import items; maximum is %d", service.MaxAccountCredentialImportItems))
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_credentials", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importCredentials(ctx, req, sources, parseErrors), nil
	})
}
