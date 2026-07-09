package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"ikik-api/internal/handler/dto"
	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/pkg/timezone"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type UserAccountHandler struct {
	accountService          *service.AccountService
	carpoolService          *service.CarpoolService
	settingService          *service.SettingService
	accountUsageService     *service.AccountUsageService
	accountTestService      *service.AccountTestService
	oauthService            *service.OAuthService
	openaiOAuthService      *service.OpenAIOAuthService
	geminiOAuthService      *service.GeminiOAuthService
	antigravityOAuthService *service.AntigravityOAuthService
	kiroOAuthService        *service.KiroOAuthService
	accountBatchTaskService *service.AccountBatchTaskService
}

func NewUserAccountHandler(
	accountService *service.AccountService,
	accountUsageService *service.AccountUsageService,
	accountTestService *service.AccountTestService,
	oauthService *service.OAuthService,
	openaiOAuthService *service.OpenAIOAuthService,
	geminiOAuthService *service.GeminiOAuthService,
	antigravityOAuthService *service.AntigravityOAuthService,
	accountBatchTaskServices ...*service.AccountBatchTaskService,
) *UserAccountHandler {
	var accountBatchTaskService *service.AccountBatchTaskService
	if len(accountBatchTaskServices) > 0 {
		accountBatchTaskService = accountBatchTaskServices[0]
	}
	h := &UserAccountHandler{
		accountService:          accountService,
		accountUsageService:     accountUsageService,
		accountTestService:      accountTestService,
		oauthService:            oauthService,
		openaiOAuthService:      openaiOAuthService,
		geminiOAuthService:      geminiOAuthService,
		antigravityOAuthService: antigravityOAuthService,
		accountBatchTaskService: accountBatchTaskService,
	}
	h.registerAccountBatchExecutors()
	return h
}

func (h *UserAccountHandler) SetCarpoolService(carpoolService *service.CarpoolService) {
	if h == nil {
		return
	}
	h.carpoolService = carpoolService
}

func (h *UserAccountHandler) SetSettingService(settingService *service.SettingService) {
	if h == nil {
		return
	}
	h.settingService = settingService
}

func (h *UserAccountHandler) SetKiroOAuthService(kiroOAuthService *service.KiroOAuthService) {
	if h == nil {
		return
	}
	h.kiroOAuthService = kiroOAuthService
}

type createUserAccountRequest struct {
	Name               string         `json:"name" binding:"required"`
	Notes              *string        `json:"notes"`
	Platform           string         `json:"platform" binding:"required"`
	AccountLevel       string         `json:"account_level" binding:"omitempty,oneof=unknown team k12"`
	Type               string         `json:"type" binding:"required,oneof=oauth apikey"`
	Credentials        map[string]any `json:"credentials" binding:"required"`
	Extra              map[string]any `json:"extra"`
	ShareMode          string         `json:"share_mode" binding:"omitempty,oneof=private public"`
	ProxyID            *int64         `json:"proxy_id"`
	Concurrency        int            `json:"concurrency"`
	LoadFactor         *int           `json:"load_factor"`
	Priority           int            `json:"priority"`
	GroupIDs           []int64        `json:"group_ids"`
	ExpiresAt          *int64         `json:"expires_at"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired"`
}

type importUserAccountCredentialsRequest struct {
	Contents           []string `json:"contents" binding:"required"`
	KiroConfigImport   bool     `json:"kiro_config_import"`
	ShareMode          string   `json:"share_mode" binding:"omitempty,oneof=private public"`
	Concurrency        int      `json:"concurrency"`
	LoadFactor         *int     `json:"load_factor"`
	Priority           int      `json:"priority"`
	GroupIDs           []int64  `json:"group_ids"`
	ExpiresAt          *int64   `json:"expires_at"`
	AutoPauseOnExpired *bool    `json:"auto_pause_on_expired"`
}

type updateUserAccountRequest struct {
	Name               *string         `json:"name"`
	Notes              *string         `json:"notes"`
	AccountLevel       *string         `json:"account_level" binding:"omitempty,oneof=unknown team k12"`
	Credentials        *map[string]any `json:"credentials"`
	Extra              *map[string]any `json:"extra"`
	ShareMode          *string         `json:"share_mode" binding:"omitempty,oneof=private public"`
	ProxyID            *int64          `json:"proxy_id"`
	Concurrency        *int            `json:"concurrency"`
	LoadFactor         *int            `json:"load_factor"`
	Priority           *int            `json:"priority"`
	Status             *string         `json:"status" binding:"omitempty,oneof=active disabled inactive"`
	Schedulable        *bool           `json:"schedulable"`
	GroupIDs           *[]int64        `json:"group_ids"`
	ExpiresAt          *int64          `json:"expires_at"`
	AutoPauseOnExpired *bool           `json:"auto_pause_on_expired"`
}

type bulkUpdateUserAccountsRequest struct {
	AccountIDs     []int64        `json:"account_ids"`
	ProxyID        *int64         `json:"proxy_id"`
	Concurrency    *int           `json:"concurrency"`
	LoadFactor     *int           `json:"load_factor"`
	Priority       *int           `json:"priority"`
	RateMultiplier *float64       `json:"rate_multiplier"`
	Status         string         `json:"status" binding:"omitempty,oneof=active disabled inactive"`
	Schedulable    *bool          `json:"schedulable"`
	AccountLevel   *string        `json:"account_level" binding:"omitempty,oneof=unknown team k12"`
	ShareMode      *string        `json:"share_mode" binding:"omitempty,oneof=private public"`
	GroupIDs       *[]int64       `json:"group_ids"`
	Credentials    map[string]any `json:"credentials"`
	Extra          map[string]any `json:"extra"`
}

type bulkUpdateUserAccountsAsyncResponse struct {
	Async bool                      `json:"async"`
	Task  *service.AccountBatchTask `json:"task"`
}

type bulkDeleteUserAccountsRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

type userAccountBatchTaskRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

const userOwnedDefaultConcurrency = 3
const userOwnedDefaultPriority = 1

type userOAuthProxyRequest struct {
	ProxyID *int64 `json:"proxy_id"`
}

type userExchangeCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
}

type userOpenAIGenerateAuthURLRequest struct {
	ProxyID     *int64 `json:"proxy_id"`
	RedirectURI string `json:"redirect_uri"`
}

type userOpenAIExchangeCodeRequest struct {
	SessionID   string `json:"session_id" binding:"required"`
	Code        string `json:"code" binding:"required"`
	State       string `json:"state" binding:"required"`
	RedirectURI string `json:"redirect_uri"`
	ProxyID     *int64 `json:"proxy_id"`
}

type userGeminiGenerateAuthURLRequest struct {
	ProxyID   *int64 `json:"proxy_id"`
	ProjectID string `json:"project_id"`
	OAuthType string `json:"oauth_type"`
	TierID    string `json:"tier_id"`
}

type userGeminiExchangeCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	State     string `json:"state" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
	OAuthType string `json:"oauth_type"`
	TierID    string `json:"tier_id"`
}

type userAntigravityGenerateAuthURLRequest struct {
	ProxyID *int64 `json:"proxy_id"`
}

type userAntigravityExchangeCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	State     string `json:"state" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
}

type userKiroGenerateAuthURLRequest struct {
	ProxyID  *int64 `json:"proxy_id"`
	Provider string `json:"provider"`
}

type userKiroGenerateIDCAuthURLRequest struct {
	ProxyID  *int64 `json:"proxy_id"`
	StartURL string `json:"start_url"`
	Region   string `json:"region"`
}

type userKiroExchangeCodeRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	State        string `json:"state" binding:"required"`
	Code         string `json:"code" binding:"required"`
	CallbackPath string `json:"callback_path"`
	LoginOption  string `json:"login_option"`
	ProxyID      *int64 `json:"proxy_id"`
}

type userBatchTodayStatsRequest struct {
	AccountIDs []int64 `json:"account_ids" binding:"required"`
}

type userTestAccountRequest struct {
	ModelID string `json:"model_id"`
	Prompt  string `json:"prompt"`
	Mode    string `json:"mode"`
}

type createUserPrivateProxyRequest struct {
	Name     string `json:"name" binding:"required"`
	Protocol string `json:"protocol" binding:"required,oneof=http https socks5 socks5h"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type updateUserPrivateProxyRequest struct {
	Name     *string `json:"name"`
	Protocol *string `json:"protocol" binding:"omitempty,oneof=http https socks5 socks5h"`
	Host     *string `json:"host"`
	Port     *int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username *string `json:"username"`
	Password *string `json:"password"`
	Status   *string `json:"status" binding:"omitempty,oneof=active inactive disabled"`
}

const userPublicShareValidationTimeout = 30 * time.Second

func bindOptionalJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		response.BadRequest(c, "Invalid request: "+err.Error())
		return false
	}
	return true
}

func requireUserAccountAuth(c *gin.Context) bool {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return false
	}
	return true
}

func rejectUserManualCredentialAuth(c *gin.Context) {
	response.BadRequest(c, "manual credential account creation is not allowed for user accounts; use official OAuth or import OAuth credentials")
}

func (h *UserAccountHandler) resolveUserProxyID(c *gin.Context, ownerUserID int64, proxyID *int64) (*int64, bool) {
	id, err := h.accountService.ValidateOwnedProxyID(c.Request.Context(), ownerUserID, proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	return id, true
}

func (h *UserAccountHandler) resolveUserOAuthProxyID(c *gin.Context, ownerUserID int64, proxyID *int64) (*int64, bool) {
	id, err := h.accountService.ValidateOwnedOAuthProxyID(c.Request.Context(), ownerUserID, proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	return id, true
}

func userUnixSecondsToTime(value *int64) *time.Time {
	if value == nil || *value <= 0 {
		return nil
	}
	t := time.Unix(*value, 0).UTC()
	return &t
}

func normalizeUserAccountIDList(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}

	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func removeInt64(values []int64, target int64) []int64 {
	if len(values) == 0 {
		return values
	}
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func normalizeUserAccountStatus(status *string) *string {
	if status == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(*status))
	if normalized == "inactive" {
		normalized = service.StatusDisabled
	}
	return &normalized
}

func isUserBulkPublicShareOnlyUpdate(req bulkUpdateUserAccountsRequest, normalizedStatus string) bool {
	return req.Concurrency == nil &&
		req.LoadFactor == nil &&
		req.Priority == nil &&
		normalizedStatus == "" &&
		req.Schedulable == nil &&
		req.AccountLevel == nil &&
		req.ShareMode != nil &&
		req.GroupIDs == nil &&
		len(req.Credentials) == 0 &&
		len(req.Extra) == 0
}

func publicShareValidationErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var appErr *infraerrors.ApplicationError
	if errors.As(err, &appErr) && strings.TrimSpace(appErr.Message) != "" {
		return strings.TrimSpace(appErr.Message)
	}
	return strings.TrimSpace(err.Error())
}

func isOpenAIUsageLimitReachedValidationError(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" || !strings.Contains(normalized, "usage_limit_reached") {
		return false
	}
	return strings.Contains(normalized, "api returned 429")
}

func (h *UserAccountHandler) activateOwnedPublicShareIfRequested(ctx context.Context, ownerUserID int64, account *service.Account) (*service.Account, error) {
	if account == nil || service.NormalizeAccountShareMode(account.ShareMode) != service.AccountShareModePublic {
		return account, nil
	}
	if service.NormalizeAccountShareStatus(account.ShareStatus) == service.AccountShareStatusApproved {
		return account, nil
	}

	reason := ""
	allowRateLimitedApproval := false
	if h.accountTestService == nil {
		reason = "account test service is unavailable"
	} else {
		testCtx, cancel := context.WithTimeout(ctx, userPublicShareValidationTimeout)
		defer cancel()
		result, err := h.accountTestService.RunTestBackground(testCtx, account.ID, "")
		switch {
		case err != nil:
			reason = publicShareValidationErrorMessage(err)
		case result == nil:
			reason = "account test did not return a result"
		case strings.TrimSpace(result.Status) != "success":
			reason = strings.TrimSpace(result.ErrorMessage)
			if reason == "" {
				reason = "account test failed"
			}
		}
	}
	if isOpenAIUsageLimitReachedValidationError(reason) {
		reason = ""
		allowRateLimitedApproval = true
	}
	if reason != "" {
		return h.accountService.MarkOwnedPublicSharePending(ctx, ownerUserID, account.ID, reason)
	}

	approved, err := h.accountService.ApproveOwnedPublicShareWithOptions(ctx, ownerUserID, account.ID, service.OwnedPublicShareApprovalOptions{
		AllowRateLimited: allowRateLimitedApproval,
	})
	if err != nil {
		return h.accountService.MarkOwnedPublicSharePending(ctx, ownerUserID, account.ID, publicShareValidationErrorMessage(err))
	}
	return approved, nil
}

func (h *UserAccountHandler) registerAccountBatchExecutors() {
	if h == nil || h.accountBatchTaskService == nil {
		return
	}
	h.accountBatchTaskService.RegisterExecutor(service.AccountBatchTaskOperationUserRefreshCredentials, h.executeUserRefreshCredentialsTaskItem)
	h.accountBatchTaskService.RegisterExecutor(service.AccountBatchTaskOperationUserRevalidateShare, h.executeUserRevalidateShareTaskItem)
	h.accountBatchTaskService.RegisterExecutor(service.AccountBatchTaskOperationUserSetPublicShare, h.executeUserSetPublicShareTaskItem)
}

func (h *UserAccountHandler) executeUserRefreshCredentialsTaskItem(ctx context.Context, task *service.AccountBatchTask, item service.AccountBatchTaskItem) (map[string]any, error) {
	if task == nil || task.OwnerUserID == nil {
		return nil, service.ErrAccountNotFound
	}
	account, err := h.accountService.GetOwnedByID(ctx, *task.OwnerUserID, item.AccountID)
	if err != nil {
		return nil, err
	}
	updated, warning, err := h.refreshOwnedAccount(ctx, *task.OwnerUserID, account)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"account_id": updated.ID}
	if strings.TrimSpace(warning) != "" {
		result["warning"] = warning
	}
	return result, nil
}

func (h *UserAccountHandler) executeUserRevalidateShareTaskItem(ctx context.Context, task *service.AccountBatchTask, item service.AccountBatchTaskItem) (map[string]any, error) {
	if task == nil || task.OwnerUserID == nil {
		return nil, service.ErrAccountNotFound
	}
	account, err := h.accountService.GetOwnedByID(ctx, *task.OwnerUserID, item.AccountID)
	if err != nil {
		return nil, err
	}
	if service.NormalizeAccountShareMode(account.ShareMode) != service.AccountShareModePublic {
		return nil, fmt.Errorf("only public shared accounts can be revalidated")
	}
	updated, err := h.activateOwnedPublicShareIfRequested(ctx, *task.OwnerUserID, account)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"account_id":   updated.ID,
		"share_status": updated.ShareStatus,
	}, nil
}

func (h *UserAccountHandler) executeUserSetPublicShareTaskItem(ctx context.Context, task *service.AccountBatchTask, item service.AccountBatchTaskItem) (map[string]any, error) {
	if task == nil || task.OwnerUserID == nil {
		return nil, service.ErrAccountNotFound
	}
	shareMode := service.AccountShareModePublic
	account, err := h.accountService.UpdateOwned(ctx, *task.OwnerUserID, item.AccountID, service.UpdateAccountRequest{
		ShareMode: &shareMode,
	})
	if err != nil {
		return nil, err
	}
	updated, err := h.activateOwnedPublicShareIfRequested(ctx, *task.OwnerUserID, account)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"account_id":   updated.ID,
		"share_mode":   updated.ShareMode,
		"share_status": updated.ShareStatus,
	}, nil
}

func (h *UserAccountHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filters := service.AccountListFilters{
		Platform:    strings.TrimSpace(c.Query("platform")),
		AccountType: strings.TrimSpace(c.Query("type")),
		Status:      strings.TrimSpace(c.Query("status")),
		Search:      strings.TrimSpace(c.Query("search")),
		PrivacyMode: strings.TrimSpace(c.Query("privacy_mode")),
	}
	if groupIDStr := strings.TrimSpace(c.Query("group_id")); groupIDStr != "" {
		groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		filters.GroupID = groupID
	}

	accounts, result, err := h.accountService.ListOwned(c.Request.Context(), subject.UserID, params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, *dto.AccountFromService(&accounts[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func (h *UserAccountHandler) ListProxies(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	proxies, err := h.accountService.ListOwnedProxies(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.ProxyWithAccountCount, 0, len(proxies))
	for i := range proxies {
		out = append(out, *dto.ProxyWithAccountCountFromService(&proxies[i]))
	}
	response.Success(c, out)
}

func (h *UserAccountHandler) CreateProxy(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createUserPrivateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	executeUserIdempotentJSON(c, "user.proxies.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		proxy, err := h.accountService.CreateOwnedProxy(ctx, subject.UserID, service.CreateProxyRequest{
			Name:     strings.TrimSpace(req.Name),
			Protocol: strings.TrimSpace(req.Protocol),
			Host:     strings.TrimSpace(req.Host),
			Port:     req.Port,
			Username: strings.TrimSpace(req.Username),
			Password: strings.TrimSpace(req.Password),
		})
		if err != nil {
			return nil, err
		}
		return dto.ProxyFromService(proxy), nil
	})
}

func (h *UserAccountHandler) UpdateProxy(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}
	var req updateUserPrivateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	proxy, err := h.accountService.UpdateOwnedProxy(c.Request.Context(), subject.UserID, proxyID, service.UpdateProxyRequest{
		Name:     trimOptionalString(req.Name),
		Protocol: trimOptionalString(req.Protocol),
		Host:     trimOptionalString(req.Host),
		Port:     req.Port,
		Username: trimOptionalString(req.Username),
		Password: trimOptionalString(req.Password),
		Status:   trimOptionalString(req.Status),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ProxyFromService(proxy))
}

func (h *UserAccountHandler) DeleteProxy(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}
	if err := h.accountService.DeleteOwnedProxy(c.Request.Context(), subject.UserID, proxyID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Proxy deleted successfully"})
}

func (h *UserAccountHandler) TestProxy(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}
	result, err := h.accountService.TestOwnedProxy(c.Request.Context(), subject.UserID, proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) CheckProxyQuality(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}
	result, err := h.accountService.CheckOwnedProxyQuality(c.Request.Context(), subject.UserID, proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) GetQuotaPoolDashboard(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	dashboard, err := h.accountService.GetQuotaPoolDashboard(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dashboard)
}

func (h *UserAccountHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(account))
}

func (h *UserAccountHandler) GetUsage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	if _, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	source := c.DefaultQuery("source", "active")
	var usage *service.UsageInfo
	if source == "passive" {
		usage, err = h.accountUsageService.GetPassiveUsage(c.Request.Context(), accountID)
	} else {
		usage, err = h.accountUsageService.GetUsage(c.Request.Context(), accountID)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if source != "passive" && h.carpoolService != nil {
		if syncErr := h.carpoolService.SyncExternalUsageForAccount(c.Request.Context(), accountID); syncErr != nil {
			slog.Warn("carpool_external_usage_sync_after_user_usage_failed", "account_id", accountID, "error", syncErr)
		}
	}
	response.Success(c, usage)
}

func (h *UserAccountHandler) GetStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if _, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 && parsedDays <= 90 {
			days = parsedDays
		}
	}

	now := timezone.Now()
	endTime := timezone.StartOfDay(now.AddDate(0, 0, 1))
	startTime := timezone.StartOfDay(now.AddDate(0, 0, -days+1))

	stats, err := h.accountUsageService.GetAccountUsageStats(c.Request.Context(), accountID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *UserAccountHandler) GetTodayStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	if _, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	stats, err := h.accountUsageService.GetTodayStats(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *UserAccountHandler) GetBatchTodayStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req userBatchTodayStatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	accountIDs := normalizeUserAccountIDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	for _, accountID := range accountIDs {
		if _, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	stats, err := h.accountUsageService.GetTodayStatsBatch(c.Request.Context(), accountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"stats": stats})
}

func (h *UserAccountHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createUserAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Concurrency <= 0 {
		req.Concurrency = userOwnedDefaultConcurrency
	}
	if req.Priority <= 0 {
		req.Priority = userOwnedDefaultPriority
	}

	executeUserIdempotentJSON(c, "user.accounts.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		account, err := h.accountService.CreateOwned(ctx, subject.UserID, service.CreateAccountRequest{
			Name:               req.Name,
			Notes:              req.Notes,
			Platform:           req.Platform,
			AccountLevel:       req.AccountLevel,
			Type:               req.Type,
			Credentials:        req.Credentials,
			Extra:              req.Extra,
			ShareMode:          req.ShareMode,
			ProxyID:            req.ProxyID,
			Concurrency:        req.Concurrency,
			LoadFactor:         req.LoadFactor,
			Priority:           req.Priority,
			GroupIDs:           req.GroupIDs,
			ExpiresAt:          userUnixSecondsToTime(req.ExpiresAt),
			AutoPauseOnExpired: req.AutoPauseOnExpired,
		})
		if err != nil {
			return nil, err
		}
		account, err = h.activateOwnedPublicShareIfRequested(ctx, subject.UserID, account)
		if err != nil {
			return nil, err
		}
		return dto.AccountFromService(account), nil
	})
}

func (h *UserAccountHandler) Import(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createUserAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Concurrency <= 0 {
		req.Concurrency = userOwnedDefaultConcurrency
	}
	if req.Priority <= 0 {
		req.Priority = userOwnedDefaultPriority
	}

	executeUserIdempotentJSON(c, "user.accounts.import", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		account, err := h.accountService.ImportOwned(ctx, subject.UserID, service.CreateAccountRequest{
			Name:               req.Name,
			Notes:              req.Notes,
			Platform:           req.Platform,
			AccountLevel:       req.AccountLevel,
			Type:               req.Type,
			Credentials:        req.Credentials,
			Extra:              req.Extra,
			ShareMode:          req.ShareMode,
			ProxyID:            req.ProxyID,
			Concurrency:        req.Concurrency,
			LoadFactor:         req.LoadFactor,
			Priority:           req.Priority,
			GroupIDs:           req.GroupIDs,
			ExpiresAt:          userUnixSecondsToTime(req.ExpiresAt),
			AutoPauseOnExpired: req.AutoPauseOnExpired,
		})
		if err != nil {
			return nil, err
		}
		account, err = h.activateOwnedPublicShareIfRequested(ctx, subject.UserID, account)
		if err != nil {
			return nil, err
		}
		return dto.AccountFromService(account), nil
	})
}

func (h *UserAccountHandler) ImportCredentials(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req importUserAccountCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Priority <= 0 {
		req.Priority = userOwnedDefaultPriority
	}
	if req.Concurrency <= 0 {
		req.Concurrency = userOwnedDefaultConcurrency
	}

	sources, parseErrors := service.ParseAccountCredentialImportContentsWithOptions(req.Contents, service.AccountCredentialImportOptions{
		KiroConfigImport: req.KiroConfigImport,
	})
	if len(sources) == 0 && len(parseErrors) == 0 {
		response.BadRequest(c, "No importable account credentials found")
		return
	}
	if len(sources) > service.MaxAccountCredentialImportItems {
		response.BadRequest(c, fmt.Sprintf("Too many import items; maximum is %d", service.MaxAccountCredentialImportItems))
		return
	}

	result := service.AccountCredentialImportResult{
		Total:  len(sources) + len(parseErrors),
		Errors: []service.AccountCredentialImportError{},
	}
	result.Errors = append(result.Errors, parseErrors...)

	for idx, source := range sources {
		account, err := h.createOwnedAccountFromCredentialImportSource(c.Request.Context(), subject.UserID, source, req, idx+1)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, service.AccountCredentialImportError{
				Index:   len(parseErrors) + idx + 1,
				Kind:    string(source.Kind),
				Name:    source.Name,
				Message: err.Error(),
			})
			continue
		}
		if account != nil {
			result.Created++
		}
	}
	result.Failed += len(parseErrors)
	response.Success(c, result)
}

func (h *UserAccountHandler) createOwnedAccountFromCredentialImportSource(
	ctx context.Context,
	ownerUserID int64,
	source service.AccountCredentialImportSource,
	defaults importUserAccountCredentialsRequest,
	sequence int,
) (*service.Account, error) {
	req := service.CreateAccountRequest{
		Name:               strings.TrimSpace(source.Name),
		Notes:              source.Notes,
		Platform:           source.Platform,
		Type:               service.AccountTypeOAuth,
		Credentials:        source.Credentials,
		Extra:              source.Extra,
		ShareMode:          defaults.ShareMode,
		ProxyID:            nil,
		Concurrency:        defaults.Concurrency,
		LoadFactor:         defaults.LoadFactor,
		Priority:           defaults.Priority,
		GroupIDs:           defaults.GroupIDs,
		ExpiresAt:          userUnixSecondsToTime(defaults.ExpiresAt),
		AutoPauseOnExpired: defaults.AutoPauseOnExpired,
	}
	if req.Concurrency <= 0 {
		req.Concurrency = userOwnedDefaultConcurrency
	}

	switch source.Kind {
	case service.AccountCredentialImportKindOAuthCredentials:
		if req.Name == "" {
			req.Name = service.DeriveAccountCredentialImportName(req.Platform, req.Credentials, req.Extra, sequence)
		}
	case service.AccountCredentialImportKindOpenAIRefreshToken:
		tokenInfo, err := h.openaiOAuthService.RefreshTokenWithClientID(ctx, source.Token, "", source.ClientID)
		if err != nil {
			return nil, fmt.Errorf("validate OpenAI refresh token: %w", err)
		}
		req.Platform = service.PlatformOpenAI
		req.Credentials = h.openaiOAuthService.BuildAccountCredentials(tokenInfo)
		req.Extra = service.BuildOpenAIAccountCredentialImportExtra(tokenInfo)
		if defaults.Concurrency <= 0 {
			req.Concurrency = userOwnedDefaultConcurrency
		}
		if req.Name == "" {
			req.Name = strings.TrimSpace(tokenInfo.Email)
		}
		if req.Name == "" {
			req.Name = fmt.Sprintf("OpenAI OAuth Account #%d", sequence)
		}
	case service.AccountCredentialImportKindClaudeSessionKey:
		tokenInfo, err := h.oauthService.CookieAuth(ctx, &service.CookieAuthInput{
			SessionKey: source.Token,
			ProxyID:    nil,
			Scope:      "full",
		})
		if err != nil {
			return nil, fmt.Errorf("exchange Claude session key: %w", err)
		}
		req.Platform = service.PlatformAnthropic
		req.Credentials = service.BuildClaudeAccountCredentials(tokenInfo)
		req.Extra = service.BuildClaudeAccountCredentialImportExtra(tokenInfo)
		if defaults.Concurrency <= 0 {
			req.Concurrency = userOwnedDefaultConcurrency
		}
		if req.Name == "" {
			req.Name = strings.TrimSpace(tokenInfo.EmailAddress)
		}
		if req.Name == "" {
			req.Name = fmt.Sprintf("Claude OAuth Account #%d", sequence)
		}
	case service.AccountCredentialImportKindKiroConfig:
		if h.kiroOAuthService == nil {
			return nil, fmt.Errorf("Kiro OAuth service is not configured")
		}
		tokenInfo, err := h.kiroOAuthService.RefreshToken(ctx, &service.KiroRefreshTokenInput{
			RefreshToken: source.Token,
			AuthMethod:   source.AuthMethod,
			Provider:     source.Provider,
			ClientID:     source.ClientID,
			ClientSecret: source.ClientSecret,
			StartURL:     source.StartURL,
			Region:       source.Region,
			ProfileArn:   source.ProfileArn,
			ProxyID:      nil,
		})
		if err != nil {
			return nil, fmt.Errorf("validate Kiro config: %w", err)
		}
		req.Platform = service.PlatformKiro
		req.Credentials = service.MergeCredentials(source.Credentials, h.kiroOAuthService.BuildAccountCredentials(tokenInfo))
		req.Extra = source.Extra
		if defaults.Concurrency <= 0 {
			req.Concurrency = userOwnedDefaultConcurrency
		}
		if req.Name == "" {
			req.Name = strings.TrimSpace(tokenInfo.Email)
		}
		if req.Name == "" {
			req.Name = service.DeriveAccountCredentialImportName(req.Platform, req.Credentials, req.Extra, sequence)
		}
	default:
		return nil, fmt.Errorf("unsupported credential import kind")
	}

	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("account name is required")
	}
	account, err := h.accountService.ImportOwned(ctx, ownerUserID, req)
	if err != nil {
		return nil, err
	}
	return h.activateOwnedPublicShareIfRequested(ctx, ownerUserID, account)
}

func (h *UserAccountHandler) Update(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req updateUserAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	status := normalizeUserAccountStatus(req.Status)
	account, err := h.accountService.UpdateOwned(c.Request.Context(), subject.UserID, accountID, service.UpdateAccountRequest{
		Name:               req.Name,
		Notes:              req.Notes,
		AccountLevel:       req.AccountLevel,
		Credentials:        req.Credentials,
		Extra:              req.Extra,
		ShareMode:          req.ShareMode,
		ProxyID:            req.ProxyID,
		Concurrency:        req.Concurrency,
		LoadFactor:         req.LoadFactor,
		Priority:           req.Priority,
		Status:             status,
		Schedulable:        req.Schedulable,
		GroupIDs:           req.GroupIDs,
		ExpiresAt:          userUnixSecondsToTime(req.ExpiresAt),
		ClearExpiresAt:     req.ExpiresAt != nil && *req.ExpiresAt <= 0,
		AutoPauseOnExpired: req.AutoPauseOnExpired,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.ShareMode != nil && service.NormalizeAccountShareMode(*req.ShareMode) == service.AccountShareModePublic {
		account, err = h.activateOwnedPublicShareIfRequested(c.Request.Context(), subject.UserID, account)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	response.Success(c, dto.AccountFromService(account))
}

func (h *UserAccountHandler) RevalidatePublicShare(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if service.NormalizeAccountShareMode(account.ShareMode) != service.AccountShareModePublic {
		response.BadRequest(c, "Only public shared accounts can be revalidated")
		return
	}
	account, err = h.activateOwnedPublicShareIfRequested(c.Request.Context(), subject.UserID, account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(account))
}

func (h *UserAccountHandler) CreateBatchRefreshTask(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.accountBatchTaskService == nil {
		response.Error(c, 503, "Account batch task service is unavailable")
		return
	}
	var req userAccountBatchTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	accountIDs := normalizeUserAccountIDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}
	for _, accountID := range accountIDs {
		if _, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	ownerUserID := subject.UserID
	task, err := h.accountBatchTaskService.CreateTask(c.Request.Context(), service.CreateAccountBatchTaskInput{
		Scope:       service.AccountBatchTaskScopeUser,
		Operation:   service.AccountBatchTaskOperationUserRefreshCredentials,
		AccountIDs:  accountIDs,
		CreatedBy:   subject.UserID,
		OwnerUserID: &ownerUserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

func (h *UserAccountHandler) CreateBatchRevalidatePublicShareTask(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.accountBatchTaskService == nil {
		response.Error(c, 503, "Account batch task service is unavailable")
		return
	}
	var req userAccountBatchTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	accountIDs := normalizeUserAccountIDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}
	for _, accountID := range accountIDs {
		account, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if service.NormalizeAccountShareMode(account.ShareMode) != service.AccountShareModePublic {
			response.BadRequest(c, "Only public shared accounts can be revalidated")
			return
		}
	}
	ownerUserID := subject.UserID
	task, err := h.accountBatchTaskService.CreateTask(c.Request.Context(), service.CreateAccountBatchTaskInput{
		Scope:       service.AccountBatchTaskScopeUser,
		Operation:   service.AccountBatchTaskOperationUserRevalidateShare,
		AccountIDs:  accountIDs,
		CreatedBy:   subject.UserID,
		OwnerUserID: &ownerUserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

func (h *UserAccountHandler) createSetPublicShareTask(ctx context.Context, ownerUserID int64, accountIDs []int64) (*service.AccountBatchTask, error) {
	if h.accountBatchTaskService == nil {
		return nil, infraerrors.ServiceUnavailable("ACCOUNT_BATCH_TASK_UNAVAILABLE", "Account batch task service is unavailable")
	}
	ownerID := ownerUserID
	return h.accountBatchTaskService.CreateTask(ctx, service.CreateAccountBatchTaskInput{
		Scope:       service.AccountBatchTaskScopeUser,
		Operation:   service.AccountBatchTaskOperationUserSetPublicShare,
		AccountIDs:  accountIDs,
		CreatedBy:   ownerUserID,
		OwnerUserID: &ownerID,
	})
}

func (h *UserAccountHandler) GetBatchTask(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.accountBatchTaskService == nil {
		response.Error(c, 503, "Account batch task service is unavailable")
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
	if task.Scope != service.AccountBatchTaskScopeUser || task.OwnerUserID == nil || *task.OwnerUserID != subject.UserID {
		response.NotFound(c, "Account batch task not found")
		return
	}
	response.Success(c, task)
}

func (h *UserAccountHandler) BulkUpdate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req bulkUpdateUserAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	accountIDs := normalizeUserAccountIDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}
	if req.RateMultiplier != nil {
		response.BadRequest(c, "rate_multiplier is not allowed for user accounts")
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "inactive" {
		status = service.StatusDisabled
	}
	if req.Concurrency != nil && *req.Concurrency <= 0 {
		response.BadRequest(c, "concurrency must be > 0")
		return
	}
	if req.Priority != nil && *req.Priority <= 0 {
		response.BadRequest(c, "priority must be > 0")
		return
	}
	if req.LoadFactor != nil && *req.LoadFactor > 10000 {
		response.BadRequest(c, "load_factor must be <= 10000")
		return
	}

	hasUpdates := req.Concurrency != nil ||
		req.LoadFactor != nil ||
		req.Priority != nil ||
		status != "" ||
		req.Schedulable != nil ||
		req.AccountLevel != nil ||
		req.ShareMode != nil ||
		req.GroupIDs != nil ||
		req.ProxyID != nil ||
		len(req.Credentials) > 0 ||
		len(req.Extra) > 0
	if !hasUpdates {
		response.BadRequest(c, "No updates provided")
		return
	}

	if req.ShareMode != nil && service.NormalizeAccountShareMode(*req.ShareMode) == service.AccountShareModePublic && isUserBulkPublicShareOnlyUpdate(req, status) {
		for _, accountID := range accountIDs {
			if _, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
		task, err := h.createSetPublicShareTask(c.Request.Context(), subject.UserID, accountIDs)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, bulkUpdateUserAccountsAsyncResponse{
			Async: true,
			Task:  task,
		})
		return
	}

	result, err := h.accountService.BulkUpdateOwned(c.Request.Context(), subject.UserID, &service.BulkUpdateOwnedAccountsInput{
		AccountIDs:   accountIDs,
		Concurrency:  req.Concurrency,
		LoadFactor:   req.LoadFactor,
		Priority:     req.Priority,
		Status:       status,
		Schedulable:  req.Schedulable,
		AccountLevel: req.AccountLevel,
		ShareMode:    req.ShareMode,
		ProxyID:      req.ProxyID,
		GroupIDs:     req.GroupIDs,
		Credentials:  req.Credentials,
		Extra:        req.Extra,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.ShareMode != nil && service.NormalizeAccountShareMode(*req.ShareMode) == service.AccountShareModePublic {
		for i := range result.Results {
			entry := &result.Results[i]
			if !entry.Success {
				continue
			}
			account, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, entry.AccountID)
			if err == nil {
				_, err = h.activateOwnedPublicShareIfRequested(c.Request.Context(), subject.UserID, account)
			}
			if err != nil {
				entry.Success = false
				entry.Error = err.Error()
				result.Success--
				result.Failed++
				result.SuccessIDs = removeInt64(result.SuccessIDs, entry.AccountID)
				result.FailedIDs = append(result.FailedIDs, entry.AccountID)
			}
		}
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if err := h.accountService.DeleteOwned(c.Request.Context(), subject.UserID, accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Account deleted successfully"})
}

func (h *UserAccountHandler) BulkDelete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req bulkDeleteUserAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	accountIDs := normalizeUserAccountIDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}
	result, err := h.accountService.BulkDeleteOwned(c.Request.Context(), subject.UserID, accountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) Test(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if _, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	var req userTestAccountRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.accountTestService.TestAccountConnection(c, accountID, req.ModelID, req.Prompt, req.Mode); err != nil {
		return
	}
}

func (h *UserAccountHandler) refreshOwnedAccount(ctx context.Context, ownerUserID int64, account *service.Account) (*service.Account, string, error) {
	if account == nil {
		return nil, "", service.ErrAccountNotFound
	}
	if !account.IsOAuth() {
		return nil, "", infraerrors.BadRequest("NOT_OAUTH", "cannot refresh non-OAuth account")
	}

	var newCredentials map[string]any
	switch {
	case account.IsOpenAI():
		tokenInfo, err := h.openaiOAuthService.RefreshAccountToken(ctx, account)
		if err != nil {
			return nil, "", err
		}
		newCredentials = h.openaiOAuthService.BuildAccountCredentials(tokenInfo)
		for k, v := range account.Credentials {
			if _, exists := newCredentials[k]; !exists {
				newCredentials[k] = v
			}
		}
	case account.Platform == service.PlatformGemini:
		tokenInfo, err := h.geminiOAuthService.RefreshAccountToken(ctx, account)
		if err != nil {
			return nil, "", fmt.Errorf("failed to refresh credentials: %w", err)
		}
		newCredentials = h.geminiOAuthService.BuildAccountCredentials(tokenInfo)
		for k, v := range account.Credentials {
			if _, exists := newCredentials[k]; !exists {
				newCredentials[k] = v
			}
		}
	case account.Platform == service.PlatformAntigravity:
		tokenInfo, err := h.antigravityOAuthService.RefreshAccountToken(ctx, account)
		if err != nil {
			return nil, "", err
		}
		newCredentials = h.antigravityOAuthService.BuildAccountCredentials(tokenInfo)
		for k, v := range account.Credentials {
			if _, exists := newCredentials[k]; !exists {
				newCredentials[k] = v
			}
		}
		if newProjectID, _ := newCredentials["project_id"].(string); newProjectID == "" {
			if oldProjectID := strings.TrimSpace(account.GetCredential("project_id")); oldProjectID != "" {
				newCredentials["project_id"] = oldProjectID
			}
		}
		if tokenInfo.ProjectIDMissing {
			updatedAccount, updateErr := h.accountService.UpdateOwned(ctx, ownerUserID, account.ID, service.UpdateAccountRequest{
				Credentials: &newCredentials,
			})
			if updateErr != nil {
				return nil, "", fmt.Errorf("failed to update credentials: %w", updateErr)
			}
			_, _ = h.setOwnedAccountPrivacy(ctx, ownerUserID, updatedAccount)
			return updatedAccount, "missing_project_id_temporary", nil
		}
	default:
		tokenInfo, err := h.oauthService.RefreshAccountToken(ctx, account)
		if err != nil {
			return nil, "", err
		}
		newCredentials = make(map[string]any)
		for k, v := range account.Credentials {
			newCredentials[k] = v
		}
		newCredentials["access_token"] = tokenInfo.AccessToken
		newCredentials["token_type"] = tokenInfo.TokenType
		newCredentials["expires_in"] = strconv.FormatInt(tokenInfo.ExpiresIn, 10)
		newCredentials["expires_at"] = strconv.FormatInt(tokenInfo.ExpiresAt, 10)
		if strings.TrimSpace(tokenInfo.RefreshToken) != "" {
			newCredentials["refresh_token"] = tokenInfo.RefreshToken
		}
		if strings.TrimSpace(tokenInfo.Scope) != "" {
			newCredentials["scope"] = tokenInfo.Scope
		}
	}

	updatedAccount, err := h.accountService.UpdateOwned(ctx, ownerUserID, account.ID, service.UpdateAccountRequest{
		Credentials: &newCredentials,
	})
	if err != nil {
		return nil, "", err
	}

	_, _ = h.setOwnedAccountPrivacy(ctx, ownerUserID, updatedAccount)
	return updatedAccount, "", nil
}

func (h *UserAccountHandler) Refresh(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updatedAccount, warning, err := h.refreshOwnedAccount(c.Request.Context(), subject.UserID, account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if warning == "missing_project_id_temporary" {
		response.Success(c, gin.H{
			"account": dto.AccountFromService(updatedAccount),
			"message": "Token refreshed successfully, but project_id could not be retrieved (will retry automatically)",
			"warning": "missing_project_id_temporary",
		})
		return
	}
	response.Success(c, dto.AccountFromService(updatedAccount))
}

func (h *UserAccountHandler) setOwnedAccountPrivacy(ctx context.Context, ownerUserID int64, account *service.Account) (string, error) {
	if account == nil {
		return "", service.ErrAccountNotFound
	}
	if account.Type != service.AccountTypeOAuth {
		return "", infraerrors.BadRequest("PRIVACY_UNSUPPORTED", "Only OAuth accounts support privacy setting")
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	mode := ""
	switch account.Platform {
	case service.PlatformOpenAI:
		if h.openaiOAuthService == nil || h.openaiOAuthService.PrivacyClientFactory() == nil {
			return "", infraerrors.BadRequest("PRIVACY_UNAVAILABLE", "privacy client is unavailable")
		}
		token, _ := account.Credentials["access_token"].(string)
		if token == "" {
			return "", infraerrors.BadRequest("PRIVACY_TOKEN_MISSING", "Cannot set privacy: missing access_token")
		}
		mode = service.DisableOpenAITraining(ctx, h.openaiOAuthService.PrivacyClientFactory(), token, proxyURL)
	case service.PlatformAntigravity:
		token, _ := account.Credentials["access_token"].(string)
		if token == "" {
			return "", infraerrors.BadRequest("PRIVACY_TOKEN_MISSING", "Cannot set privacy: missing access_token")
		}
		projectID, _ := account.Credentials["project_id"].(string)
		mode = service.SetAntigravityPrivacy(ctx, token, projectID, proxyURL)
	default:
		return "", infraerrors.BadRequest("PRIVACY_UNSUPPORTED", "Only OpenAI and Antigravity OAuth accounts support privacy setting")
	}
	if mode == "" {
		return "", infraerrors.BadRequest("PRIVACY_FAILED", "Cannot set privacy")
	}

	extra := make(map[string]any, len(account.Extra)+1)
	for k, v := range account.Extra {
		extra[k] = v
	}
	extra["privacy_mode"] = mode
	if _, err := h.accountService.UpdateOwned(ctx, ownerUserID, account.ID, service.UpdateAccountRequest{Extra: &extra}); err != nil {
		return "", err
	}
	return mode, nil
}

func (h *UserAccountHandler) SetPrivacy(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	mode, err := h.setOwnedAccountPrivacy(c.Request.Context(), subject.UserID, account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updated, err := h.accountService.GetOwnedByID(c.Request.Context(), subject.UserID, accountID)
	if err != nil {
		if account.Extra == nil {
			account.Extra = make(map[string]any)
		}
		account.Extra["privacy_mode"] = mode
		response.Success(c, dto.AccountFromService(account))
		return
	}
	response.Success(c, dto.AccountFromService(updated))
}

func (h *UserAccountHandler) GenerateAnthropicOAuthURL(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userOAuthProxyRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	result, err := h.oauthService.GenerateAuthURL(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) GenerateAnthropicSetupTokenURL(c *gin.Context) {
	rejectUserManualCredentialAuth(c)
}

func (h *UserAccountHandler) ExchangeAnthropicOAuthCode(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	tokenInfo, err := h.oauthService.ExchangeCode(c.Request.Context(), &service.ExchangeCodeInput{
		SessionID: req.SessionID,
		Code:      req.Code,
		ProxyID:   proxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

func (h *UserAccountHandler) ExchangeAnthropicSetupTokenCode(c *gin.Context) {
	rejectUserManualCredentialAuth(c)
}

func (h *UserAccountHandler) AnthropicCookieAuth(c *gin.Context) {
	rejectUserManualCredentialAuth(c)
}

func (h *UserAccountHandler) AnthropicSetupTokenCookieAuth(c *gin.Context) {
	rejectUserManualCredentialAuth(c)
}

func (h *UserAccountHandler) GenerateOpenAIOAuthURL(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userOpenAIGenerateAuthURLRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	result, err := h.openaiOAuthService.GenerateAuthURL(
		c.Request.Context(),
		proxyID,
		req.RedirectURI,
		service.PlatformOpenAI,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) ExchangeOpenAIOAuthCode(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userOpenAIExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	tokenInfo, err := h.openaiOAuthService.ExchangeCode(c.Request.Context(), &service.OpenAIExchangeCodeInput{
		SessionID:   req.SessionID,
		Code:        req.Code,
		State:       req.State,
		RedirectURI: req.RedirectURI,
		ProxyID:     proxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

func (h *UserAccountHandler) RefreshOpenAIToken(c *gin.Context) {
	rejectUserManualCredentialAuth(c)
}

func (h *UserAccountHandler) GetGeminiOAuthCapabilities(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	response.Success(c, h.geminiOAuthService.GetOAuthConfig())
}

func (h *UserAccountHandler) GenerateGeminiOAuthURL(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userGeminiGenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}

	oauthType := strings.TrimSpace(req.OAuthType)
	if oauthType == "" {
		oauthType = "code_assist"
	}
	if oauthType != "code_assist" && oauthType != "google_one" && oauthType != "ai_studio" {
		response.BadRequest(c, "Invalid oauth_type: must be 'code_assist', 'google_one', or 'ai_studio'")
		return
	}

	result, err := h.geminiOAuthService.GenerateAuthURL(
		c.Request.Context(),
		proxyID,
		deriveUserGeminiRedirectURI(c),
		req.ProjectID,
		oauthType,
		req.TierID,
	)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "OAuth client not configured") ||
			strings.Contains(msg, "requires your own OAuth Client") ||
			strings.Contains(msg, "requires a custom OAuth Client") ||
			strings.Contains(msg, "GEMINI_CLI_OAUTH_CLIENT_SECRET_MISSING") ||
			strings.Contains(msg, "built-in Gemini CLI OAuth client_secret is not configured") {
			response.BadRequest(c, "Failed to generate auth URL: "+msg)
			return
		}
		response.InternalError(c, "Failed to generate auth URL: "+msg)
		return
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) ExchangeGeminiOAuthCode(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userGeminiExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}

	oauthType := strings.TrimSpace(req.OAuthType)
	if oauthType == "" {
		oauthType = "code_assist"
	}
	if oauthType != "code_assist" && oauthType != "google_one" && oauthType != "ai_studio" {
		response.BadRequest(c, "Invalid oauth_type: must be 'code_assist', 'google_one', or 'ai_studio'")
		return
	}

	tokenInfo, err := h.geminiOAuthService.ExchangeCode(c.Request.Context(), &service.GeminiExchangeCodeInput{
		SessionID: req.SessionID,
		State:     req.State,
		Code:      req.Code,
		ProxyID:   proxyID,
		OAuthType: oauthType,
		TierID:    req.TierID,
	})
	if err != nil {
		response.BadRequest(c, "Failed to exchange code: "+err.Error())
		return
	}
	response.Success(c, tokenInfo)
}

func (h *UserAccountHandler) GenerateAntigravityOAuthURL(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userAntigravityGenerateAuthURLRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	result, err := h.antigravityOAuthService.GenerateAuthURL(c.Request.Context(), proxyID)
	if err != nil {
		response.InternalError(c, "Failed to generate auth URL: "+err.Error())
		return
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) ExchangeAntigravityOAuthCode(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userAntigravityExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	tokenInfo, err := h.antigravityOAuthService.ExchangeCode(c.Request.Context(), &service.AntigravityExchangeCodeInput{
		SessionID: req.SessionID,
		State:     req.State,
		Code:      req.Code,
		ProxyID:   proxyID,
	})
	if err != nil {
		response.BadRequest(c, "Failed to exchange code: "+err.Error())
		return
	}
	response.Success(c, tokenInfo)
}

func (h *UserAccountHandler) RefreshAntigravityToken(c *gin.Context) {
	rejectUserManualCredentialAuth(c)
}

func (h *UserAccountHandler) GenerateKiroOAuthURL(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	if h.kiroOAuthService == nil {
		response.InternalError(c, "Kiro OAuth service is not configured")
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userKiroGenerateAuthURLRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	result, err := h.kiroOAuthService.GenerateAuthURL(c.Request.Context(), &service.KiroGenerateAuthURLInput{
		ProxyID:  proxyID,
		Provider: req.Provider,
	})
	if err != nil {
		response.BadRequest(c, "Failed to generate Kiro auth URL: "+err.Error())
		return
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) GenerateKiroIDCAuthURL(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	if h.kiroOAuthService == nil {
		response.InternalError(c, "Kiro OAuth service is not configured")
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userKiroGenerateIDCAuthURLRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	result, err := h.kiroOAuthService.GenerateIDCAuthURL(c.Request.Context(), &service.KiroGenerateIDCAuthURLInput{
		ProxyID:  proxyID,
		StartURL: req.StartURL,
		Region:   req.Region,
	})
	if err != nil {
		response.BadRequest(c, "Failed to generate Kiro IDC auth URL: "+err.Error())
		return
	}
	response.Success(c, result)
}

func (h *UserAccountHandler) ExchangeKiroOAuthCode(c *gin.Context) {
	if !requireUserAccountAuth(c) {
		return
	}
	if h.kiroOAuthService == nil {
		response.InternalError(c, "Kiro OAuth service is not configured")
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	var req userKiroExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	proxyID, ok := h.resolveUserOAuthProxyID(c, subject.UserID, req.ProxyID)
	if !ok {
		return
	}
	tokenInfo, err := h.kiroOAuthService.ExchangeCode(c.Request.Context(), &service.KiroExchangeCodeInput{
		SessionID:    req.SessionID,
		State:        req.State,
		Code:         req.Code,
		CallbackPath: req.CallbackPath,
		LoginOption:  req.LoginOption,
		ProxyID:      proxyID,
	})
	if err != nil {
		response.BadRequest(c, "Failed to exchange Kiro code: "+err.Error())
		return
	}
	response.Success(c, tokenInfo)
}

func (h *UserAccountHandler) RefreshKiroToken(c *gin.Context) {
	rejectUserManualCredentialAuth(c)
}

func (h *UserAccountHandler) ImportKiroToken(c *gin.Context) {
	rejectUserManualCredentialAuth(c)
}

func deriveUserGeminiRedirectURI(c *gin.Context) string {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin != "" {
		return strings.TrimRight(origin, "/") + "/auth/callback"
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xfProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xfProto != "" {
		scheme = strings.TrimSpace(strings.Split(xfProto, ",")[0])
	}

	host := strings.TrimSpace(c.Request.Host)
	if xfHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); xfHost != "" {
		host = strings.TrimSpace(strings.Split(xfHost, ",")[0])
	}

	return fmt.Sprintf("%s://%s/auth/callback", scheme, host)
}
