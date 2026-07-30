package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	adminhandler "ikik-api/internal/handler/admin"
	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type userAgentIdentityImportRequest struct {
	Content     string   `json:"content"`
	Contents    []string `json:"contents"`
	Name        string   `json:"name"`
	Notes       *string  `json:"notes"`
	ShareMode   string   `json:"share_mode" binding:"omitempty,oneof=private public"`
	ProxyID     *int64   `json:"proxy_id"`
	Concurrency int      `json:"concurrency"`
	LoadFactor  *int     `json:"load_factor"`
	Priority    int      `json:"priority"`
}

type userAgentIdentityImportItem struct {
	Index     int    `json:"index"`
	Name      string `json:"name,omitempty"`
	Action    string `json:"action"`
	AccountID int64  `json:"account_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

type userAgentIdentityImportResult struct {
	Total    int                                      `json:"total"`
	Created  int                                      `json:"created"`
	Updated  int                                      `json:"updated"`
	Skipped  int                                      `json:"skipped"`
	Failed   int                                      `json:"failed"`
	Items    []userAgentIdentityImportItem            `json:"items,omitempty"`
	Warnings []adminhandler.CodexSessionImportMessage `json:"warnings,omitempty"`
	Errors   []adminhandler.CodexSessionImportMessage `json:"errors,omitempty"`
}

func (h *UserAccountHandler) ImportOpenAIAgentIdentity(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req userAgentIdentityImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	contents := make([]string, 0, len(req.Contents)+1)
	if strings.TrimSpace(req.Content) != "" {
		contents = append(contents, req.Content)
	}
	contents = append(contents, req.Contents...)
	items, parseErrors, err := adminhandler.ParseCodexAgentIdentityImport(contents)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(items) == 0 && len(parseErrors) == 0 {
		response.BadRequest(c, "No Agent Identity credentials found")
		return
	}
	if len(items)+len(parseErrors) > service.MaxAccountCredentialImportItems {
		response.BadRequest(c, fmt.Sprintf("Too many import items; maximum is %d", service.MaxAccountCredentialImportItems))
		return
	}
	if req.Concurrency <= 0 {
		req.Concurrency = userOwnedDefaultConcurrency
	}
	if req.Priority <= 0 {
		req.Priority = userOwnedDefaultPriority
	}

	executeUserIdempotentJSON(c, "user.accounts.import_agent_identity", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importOwnedOpenAIAgentIdentities(ctx, subject.UserID, req, items, parseErrors), nil
	})
}

func (h *UserAccountHandler) importOwnedOpenAIAgentIdentities(
	ctx context.Context,
	ownerUserID int64,
	req userAgentIdentityImportRequest,
	items []adminhandler.CodexAgentIdentityImportAccount,
	parseErrors []adminhandler.CodexSessionImportMessage,
) userAgentIdentityImportResult {
	result := userAgentIdentityImportResult{
		Total:  len(items) + len(parseErrors),
		Items:  make([]userAgentIdentityImportItem, 0, len(items)),
		Errors: append([]adminhandler.CodexSessionImportMessage(nil), parseErrors...),
		Failed: len(parseErrors),
	}

	for index, item := range items {
		itemIndex := item.Index
		if itemIndex <= 0 {
			itemIndex = index + 1
		}
		name := buildUserAgentIdentityAccountName(req.Name, item.Name, itemIndex, len(items))
		account, err := h.accountService.CreateOwned(ctx, ownerUserID, service.CreateAccountRequest{
			Name:         name,
			Notes:        req.Notes,
			Platform:     service.PlatformOpenAI,
			AccountLevel: service.AccountLevelUnknown,
			Type:         service.AccountTypeOAuth,
			Credentials:  item.Credentials,
			Extra:        item.Extra,
			ShareMode:    req.ShareMode,
			ProxyID:      req.ProxyID,
			Concurrency:  req.Concurrency,
			LoadFactor:   req.LoadFactor,
			Priority:     req.Priority,
		})
		if err != nil {
			if errors.Is(err, service.ErrOwnedAccountAlreadyExists) {
				result.Skipped++
				result.Items = append(result.Items, userAgentIdentityImportItem{
					Index:   itemIndex,
					Name:    name,
					Action:  "skipped",
					Message: err.Error(),
				})
				continue
			}
			result.Failed++
			result.Errors = append(result.Errors, adminhandler.CodexSessionImportMessage{
				Index:   itemIndex,
				Name:    name,
				Message: err.Error(),
			})
			result.Items = append(result.Items, userAgentIdentityImportItem{
				Index:   itemIndex,
				Name:    name,
				Action:  "failed",
				Message: err.Error(),
			})
			continue
		}

		account, err = h.activateOwnedPublicShareIfRequested(ctx, ownerUserID, account)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, adminhandler.CodexSessionImportMessage{
				Index:   itemIndex,
				Name:    name,
				Message: err.Error(),
			})
			result.Items = append(result.Items, userAgentIdentityImportItem{
				Index:     itemIndex,
				Name:      name,
				Action:    "failed",
				AccountID: account.ID,
				Message:   err.Error(),
			})
			continue
		}

		result.Created++
		result.Items = append(result.Items, userAgentIdentityImportItem{
			Index:     itemIndex,
			Name:      name,
			Action:    "created",
			AccountID: account.ID,
		})
		if item.Warning != "" {
			result.Warnings = append(result.Warnings, adminhandler.CodexSessionImportMessage{
				Index:   itemIndex,
				Name:    name,
				Message: item.Warning,
			})
		}
	}
	return result
}

func buildUserAgentIdentityAccountName(base, fallback string, index, total int) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = strings.TrimSpace(fallback)
	}
	if base == "" {
		base = "OpenAI Agent Identity"
	}
	if total > 1 {
		return fmt.Sprintf("%s #%d", base, index)
	}
	return base
}
