package handler

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"ikik-api/internal/handler/dto"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/pkg/xai"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

const userGrokSSOImportConcurrency = 3

type userGrokSSOImportRequest struct {
	SSOTokens          []string `json:"sso_tokens"`
	SSOToken           string   `json:"sso_token"`
	Name               string   `json:"name"`
	Notes              *string  `json:"notes"`
	ProxyID            *int64   `json:"proxy_id"`
	ShareMode          string   `json:"share_mode" binding:"omitempty,oneof=private public"`
	Concurrency        int      `json:"concurrency"`
	LoadFactor         *int     `json:"load_factor"`
	Priority           int      `json:"priority"`
	ExpiresAt          *int64   `json:"expires_at"`
	AutoPauseOnExpired *bool    `json:"auto_pause_on_expired"`
}

type userGrokSSOImportItem struct {
	Index   int          `json:"index"`
	Name    string       `json:"name,omitempty"`
	Email   string       `json:"email,omitempty"`
	Account *dto.Account `json:"account,omitempty"`
	Error   string       `json:"error,omitempty"`
}

type userGrokSSOImportResponse struct {
	Created []userGrokSSOImportItem `json:"created"`
	Failed  []userGrokSSOImportItem `json:"failed"`
}

type userGrokSSOImportWorkerResult struct {
	created bool
	item    userGrokSSOImportItem
}

func (h *UserAccountHandler) ImportGrokSSO(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	if h.grokOAuthService == nil {
		response.InternalError(c, "Grok OAuth service is not configured")
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userGrokSSOImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	tokens := normalizeUserGrokSSOTokens(req.SSOTokens, req.SSOToken)
	if len(tokens) == 0 {
		response.BadRequest(c, "sso_tokens is required")
		return
	}
	if len(tokens) > service.MaxAccountCredentialImportItems {
		response.BadRequest(c, fmt.Sprintf("Too many import items; maximum is %d", service.MaxAccountCredentialImportItems))
		return
	}
	if req.Concurrency <= 0 {
		req.Concurrency = userOwnedDefaultConcurrency
	}
	if req.Priority <= 0 {
		req.Priority = userOwnedDefaultPriority
	}

	items := make([]userGrokSSOImportWorkerResult, len(tokens))
	jobs := make(chan int)
	workerCount := min(userGrokSSOImportConcurrency, len(tokens))
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				items[index] = h.importOwnedGrokSSOToken(c.Request.Context(), subject.UserID, proxyID, req, tokens[index], index+1, len(tokens))
			}
		}()
	}
	for index := range tokens {
		jobs <- index
	}
	close(jobs)
	workers.Wait()

	result := userGrokSSOImportResponse{
		Created: make([]userGrokSSOImportItem, 0, len(tokens)),
		Failed:  make([]userGrokSSOImportItem, 0),
	}
	for _, workerResult := range items {
		if workerResult.created {
			result.Created = append(result.Created, workerResult.item)
		} else {
			result.Failed = append(result.Failed, workerResult.item)
		}
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) importOwnedGrokSSOToken(
	ctx context.Context,
	ownerUserID int64,
	proxyID *int64,
	req userGrokSSOImportRequest,
	token string,
	index int,
	total int,
) userGrokSSOImportWorkerResult {
	item := userGrokSSOImportItem{Index: index}
	tokenInfo, err := h.grokOAuthService.ConvertFromSSO(ctx, token, proxyID)
	if err != nil {
		item.Error = err.Error()
		return userGrokSSOImportWorkerResult{item: item}
	}
	item.Email = strings.TrimSpace(tokenInfo.Email)
	item.Name = userGrokSSOAccountName(req.Name, item.Email, index, total)
	expiresAt := req.ExpiresAt
	autoPause := req.AutoPauseOnExpired
	if expiresAt == nil && strings.TrimSpace(tokenInfo.RefreshToken) == "" && tokenInfo.ExpiresAt > 0 {
		expiresAt = &tokenInfo.ExpiresAt
		value := true
		autoPause = &value
	}
	account, err := h.accountService.CreateOwned(ctx, ownerUserID, service.CreateAccountRequest{
		Name:        item.Name,
		Notes:       req.Notes,
		Platform:    service.PlatformGrok,
		Type:        service.AccountTypeOAuth,
		Credentials: h.grokOAuthService.BuildAccountCredentials(tokenInfo),
		Extra: map[string]any{
			"email":              tokenInfo.Email,
			"subscription_tier":  tokenInfo.SubscriptionTier,
			"entitlement_status": tokenInfo.EntitlementStatus,
		},
		ShareMode:          req.ShareMode,
		ProxyID:            proxyID,
		Concurrency:        req.Concurrency,
		LoadFactor:         req.LoadFactor,
		Priority:           req.Priority,
		ExpiresAt:          userUnixSecondsToTime(expiresAt),
		AutoPauseOnExpired: autoPause,
	})
	if err == nil {
		account, err = h.activateOwnedPublicShareIfRequested(ctx, ownerUserID, account)
	}
	if err != nil {
		item.Error = err.Error()
		return userGrokSSOImportWorkerResult{item: item}
	}
	item.Account = dto.AccountFromService(account)
	return userGrokSSOImportWorkerResult{created: true, item: item}
}

func normalizeUserGrokSSOTokens(values []string, single string) []string {
	candidates := append(append([]string(nil), values...), single)
	seen := make(map[string]struct{}, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		token := xai.NormalizeSSOToken(candidate)
		if token == "" {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

func userGrokSSOAccountName(base, email string, index, total int) string {
	name := strings.TrimSpace(base)
	if name == "" {
		name = strings.TrimSpace(email)
	}
	if name == "" {
		name = "Grok OAuth Account"
	}
	if total > 1 {
		return fmt.Sprintf("%s #%d", name, index)
	}
	return name
}
