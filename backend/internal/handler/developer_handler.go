package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

const developerTokenContextKey = "developer_token"
const maxDeveloperAccountImportItems = 50

type DeveloperHandler struct {
	tokens   *service.DeveloperTokenService
	accounts *UserAccountHandler
}

func NewDeveloperHandler(tokens *service.DeveloperTokenService, accounts *UserAccountHandler) *DeveloperHandler {
	return &DeveloperHandler{tokens: tokens, accounts: accounts}
}

type createDeveloperTokenRequest struct {
	Name      string     `json:"name" binding:"required"`
	Scopes    []string   `json:"scopes" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type developerAccountImportRequest struct {
	Accounts  []json.RawMessage `json:"accounts" binding:"required"`
	ShareMode string            `json:"share_mode" binding:"omitempty,oneof=private public"`
	ProxyID   *int64            `json:"proxy_id"`
}

type developerAccountImportItem struct {
	Index      int    `json:"index"`
	ExternalID string `json:"external_id,omitempty"`
	AccountID  *int64 `json:"account_id,omitempty"`
	Status     string `json:"status"`
	ErrorCode  string `json:"error_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

type developerAccountImportResult struct {
	Total      int                          `json:"total"`
	Created    int                          `json:"created"`
	Failed     int                          `json:"failed"`
	Items      []developerAccountImportItem `json:"items"`
	ShareTask  *service.AccountBatchTask    `json:"share_task,omitempty"`
	ShareError string                       `json:"share_error,omitempty"`
}

type developerAccountSharingRequest struct {
	Mode string `json:"mode" binding:"required,oneof=private public"`
}

func (h *DeveloperHandler) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(authorization, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.ErrorFrom(c, service.ErrDeveloperTokenInvalid)
			c.Abort()
			return
		}
		token, err := h.tokens.Authenticate(c.Request.Context(), strings.TrimSpace(parts[1]))
		if err != nil {
			response.ErrorFrom(c, err)
			c.Abort()
			return
		}
		c.Set(developerTokenContextKey, token)
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{
			UserID:      token.User.ID,
			Concurrency: token.User.Concurrency,
		})
		c.Set(string(middleware2.ContextKeyUserRole), token.User.Role)
		h.tokens.Touch(c.Request.Context(), token.ID, middleware2.SecurityClientIP(c))
		c.Next()
	}
}

func (h *DeveloperHandler) RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := developerTokenFromContext(c)
		if !ok || !token.HasScope(scope) {
			response.ErrorFrom(c, service.ErrDeveloperTokenScope.WithMetadata(map[string]string{"scope": scope}))
			c.Abort()
			return
		}
		c.Next()
	}
}

func developerTokenFromContext(c *gin.Context) (*service.DeveloperToken, bool) {
	value, ok := c.Get(developerTokenContextKey)
	if !ok {
		return nil, false
	}
	token, ok := value.(*service.DeveloperToken)
	return token, ok && token != nil
}

func (h *DeveloperHandler) ListTokens(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	tokens, err := h.tokens.List(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokens)
}

func (h *DeveloperHandler) CreateToken(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createDeveloperTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	created, err := h.tokens.Create(c.Request.Context(), subject.UserID, service.CreateDeveloperTokenInput{
		Name: req.Name, Scopes: req.Scopes, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, created)
}

func (h *DeveloperHandler) DeleteToken(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	tokenID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid developer token ID")
		return
	}
	if err := h.tokens.Delete(c.Request.Context(), subject.UserID, tokenID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Developer token revoked"})
}

func (h *DeveloperHandler) ImportAccounts(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.ErrorFrom(c, service.ErrDeveloperTokenInvalid)
		return
	}
	var req developerAccountImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.Accounts) == 0 || len(req.Accounts) > maxDeveloperAccountImportItems {
		response.BadRequest(c, fmt.Sprintf("accounts must contain between 1 and %d items", maxDeveloperAccountImportItems))
		return
	}
	shareMode := service.NormalizeAccountShareMode(req.ShareMode)
	if shareMode == service.AccountShareModePublic {
		token, _ := developerTokenFromContext(c)
		if token == nil || !token.HasScope(service.DeveloperScopeAccountsShare) {
			response.ErrorFrom(c, service.ErrDeveloperTokenScope.WithMetadata(map[string]string{"scope": service.DeveloperScopeAccountsShare}))
			return
		}
	}
	if err := requireDeveloperIdempotencyKey(c); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	executeUserIdempotentJSON(c, "developer.account_imports.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importAccounts(ctx, subject.UserID, req)
	})
}

func (h *DeveloperHandler) importAccounts(ctx context.Context, userID int64, req developerAccountImportRequest) (developerAccountImportResult, error) {
	result := developerAccountImportResult{
		Total: len(req.Accounts),
		Items: make([]developerAccountImportItem, 0, len(req.Accounts)),
	}
	proxyID, proxyURL, err := h.accounts.resolveOwnedCredentialImportProxy(ctx, userID, req.ProxyID)
	if err != nil {
		return result, err
	}
	req.ProxyID = proxyID
	createdIDs := make([]int64, 0, len(req.Accounts))
	for index, raw := range req.Accounts {
		item := developerAccountImportItem{Index: index + 1, Status: "failed"}
		accountValue, err := validateDeveloperAccountImportItem(raw)
		if err == nil {
			item.ExternalID, _ = accountValue["external_id"].(string)
			delete(accountValue, "external_id")
			var normalized []byte
			normalized, err = json.Marshal(accountValue)
			if err == nil {
				sources, parseErrors := service.ParseAccountCredentialImportContents([]string{string(normalized)})
				if len(parseErrors) > 0 {
					err = infraerrors.BadRequest("ACCOUNT_IMPORT_CREDENTIALS_INVALID", parseErrors[0].Message)
				} else if len(sources) != 1 {
					err = infraerrors.BadRequest("ACCOUNT_IMPORT_CREDENTIALS_INVALID", "account item must resolve to exactly one credential source")
				} else {
					defaults := importUserAccountCredentialsRequest{
						ShareMode:   service.AccountShareModePrivate,
						ProxyID:     req.ProxyID,
						Concurrency: userOwnedDefaultConcurrency,
						Priority:    userOwnedDefaultPriority,
					}
					var account *service.Account
					account, err = h.accounts.createOwnedAccountFromCredentialImportSource(ctx, userID, sources[0], defaults, proxyURL, index+1)
					if err == nil {
						id := account.ID
						item.AccountID = &id
						item.Status = "created"
						createdIDs = append(createdIDs, id)
						result.Created++
					}
				}
			}
		}
		if err != nil {
			item.ErrorCode = developerImportErrorCode(err)
			item.Error = developerImportErrorMessage(err)
			result.Failed++
		}
		result.Items = append(result.Items, item)
	}

	if service.NormalizeAccountShareMode(req.ShareMode) == service.AccountShareModePublic && len(createdIDs) > 0 {
		task, err := h.accounts.createSetPublicShareTask(ctx, userID, createdIDs)
		if err != nil {
			result.ShareError = developerImportErrorMessage(err)
		} else {
			result.ShareTask = task
		}
	}
	return result, nil
}

func validateDeveloperAccountImportItem(raw json.RawMessage) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, infraerrors.BadRequest("ACCOUNT_IMPORT_ITEM_INVALID", "account import item must be a JSON object")
	}
	allowedFields := map[string]struct{}{
		"external_id": {},
		"name":        {},
		"notes":       {},
		"platform":    {},
		"type":        {},
		"credentials": {},
		"extra":       {},
	}
	for field := range value {
		if _, allowed := allowedFields[field]; !allowed {
			return nil, infraerrors.BadRequest("ACCOUNT_IMPORT_FIELD_NOT_ALLOWED", "account import item contains an unsupported or server-managed field").WithMetadata(map[string]string{"field": field})
		}
	}
	platform, _ := value["platform"].(string)
	if strings.TrimSpace(platform) == "" {
		return nil, infraerrors.BadRequest("ACCOUNT_IMPORT_PLATFORM_REQUIRED", "account platform is required")
	}
	accountType, _ := value["type"].(string)
	if !strings.EqualFold(strings.TrimSpace(accountType), service.AccountTypeOAuth) {
		return nil, infraerrors.BadRequest("ACCOUNT_IMPORT_TYPE_NOT_ALLOWED", "developer account imports currently support OAuth accounts only")
	}
	credentials, ok := value["credentials"].(map[string]any)
	if !ok || len(credentials) == 0 {
		return nil, infraerrors.BadRequest("ACCOUNT_IMPORT_CREDENTIALS_INVALID", "account credentials must be a non-empty JSON object")
	}
	if extra, exists := value["extra"]; exists && extra != nil {
		if _, ok := extra.(map[string]any); !ok {
			return nil, infraerrors.BadRequest("ACCOUNT_IMPORT_EXTRA_INVALID", "account extra must be a JSON object")
		}
	}
	if rawExternalID, exists := value["external_id"]; exists {
		externalID, ok := rawExternalID.(string)
		if !ok {
			return nil, infraerrors.BadRequest("ACCOUNT_IMPORT_EXTERNAL_ID_INVALID", "external_id must be a string")
		}
		externalID = strings.TrimSpace(externalID)
		if len(externalID) > 128 || strings.ContainsAny(externalID, "\r\n\x00") {
			return nil, infraerrors.BadRequest("ACCOUNT_IMPORT_EXTERNAL_ID_INVALID", "external_id must not exceed 128 characters")
		}
		value["external_id"] = externalID
	}
	for _, field := range []string{"name", "notes"} {
		if rawValue, exists := value[field]; exists && rawValue != nil {
			if _, ok := rawValue.(string); !ok {
				return nil, infraerrors.BadRequest("ACCOUNT_IMPORT_ITEM_INVALID", field+" must be a string")
			}
		}
	}
	return value, nil
}

func developerImportErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if reason := infraerrors.Reason(err); reason != "" {
		return reason
	}
	return "ACCOUNT_IMPORT_FAILED"
}

func developerImportErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if message := infraerrors.Message(err); message != "" && message != infraerrors.UnknownMessage {
		return message
	}
	return "account import failed"
}

func requireDeveloperIdempotencyKey(c *gin.Context) error {
	key, err := service.NormalizeIdempotencyKey(c.GetHeader("Idempotency-Key"))
	if err != nil {
		return err
	}
	if key == "" {
		return service.ErrIdempotencyKeyRequired
	}
	return nil
}

func (h *DeveloperHandler) SetAccountSharing(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.ErrorFrom(c, service.ErrDeveloperTokenInvalid)
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req developerAccountSharingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := requireDeveloperIdempotencyKey(c); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	executeUserIdempotentJSON(c, "developer.accounts.sharing", map[string]any{"account_id": accountID, "mode": req.Mode}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if _, err := h.accounts.accountService.GetOwnedByID(ctx, subject.UserID, accountID); err != nil {
			return nil, err
		}
		if req.Mode == service.AccountShareModePrivate {
			mode := service.AccountShareModePrivate
			account, err := h.accounts.accountService.UpdateOwned(ctx, subject.UserID, accountID, service.UpdateAccountRequest{ShareMode: &mode})
			if err != nil {
				return nil, err
			}
			return gin.H{"account": h.accounts.accountResponseFromService(account)}, nil
		}
		task, err := h.accounts.createSetPublicShareTask(ctx, subject.UserID, []int64{accountID})
		if err != nil {
			return nil, err
		}
		return gin.H{"task": task}, nil
	})
}

func (h *DeveloperHandler) DeleteAccount(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.ErrorFrom(c, service.ErrDeveloperTokenInvalid)
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if err := requireDeveloperIdempotencyKey(c); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	executeUserIdempotentJSON(c, "developer.accounts.delete", map[string]any{"account_id": accountID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if err := h.accounts.accountService.DeleteOwned(ctx, subject.UserID, accountID); err != nil {
			return nil, err
		}
		return gin.H{"deleted": true, "account_id": accountID}, nil
	})
}

func (h *DeveloperHandler) Health(c *gin.Context) {
	response.Success(c, gin.H{"status": "ok", "version": "v1"})
}

func (h *DeveloperHandler) BotAccountSummary(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.ErrorFrom(c, service.ErrDeveloperTokenInvalid)
		return
	}
	params := pagination.PaginationParams{
		Page:      1,
		PageSize:  1,
		SortBy:    "created_at",
		SortOrder: "desc",
	}
	_, allResult, err := h.accounts.accountService.ListOwned(c.Request.Context(), subject.UserID, params, service.AccountListFilters{})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	_, activeResult, err := h.accounts.accountService.ListOwned(c.Request.Context(), subject.UserID, params, service.AccountListFilters{Status: service.StatusActive})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	total := allResult.Total
	invalid := total - activeResult.Total
	if invalid < 0 {
		invalid = 0
	}
	response.Success(c, gin.H{
		"total_accounts":   total,
		"invalid_accounts": invalid,
	})
}
