package handler

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

type importUserAccountDataRequest struct {
	Data            service.AccountDataPayload              `json:"data"`
	SourceURL       string                                  `json:"source_url"`
	GroupIDs        []int64                                 `json:"group_ids"`
	AccountDefaults *service.OwnedAccountDataImportDefaults `json:"account_defaults"`
}

func (h *UserAccountHandler) ImportData(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req importUserAccountDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.SourceURL) != "" {
		response.BadRequest(c, "source_url is not supported for personal account imports")
		return
	}

	executeUserIdempotentJSON(c, "user.accounts.import_data", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.accountService.ImportOwnedData(ctx, subject.UserID, req.Data, service.OwnedAccountDataImportOptions{
			GroupIDs:        append([]int64(nil), req.GroupIDs...),
			AccountDefaults: req.AccountDefaults,
		})
	})
}
