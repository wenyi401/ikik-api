package admin

import (
	"context"

	"errors"
	"fmt"

	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"time"

	infraerrors "ikik-api/internal/pkg/errors"

	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type ModelProbeListRequest struct {
	Platform string `json:"platform" binding:"required"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key" binding:"required"`
}

type ModelProbeTestRequest struct {
	Platform string   `json:"platform" binding:"required"`
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key" binding:"required"`
	Mode     string   `json:"mode"`
	Models   []string `json:"models" binding:"required"`
}

func adminIsOpenAIUsageLimitReachedValidationError(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" || !strings.Contains(normalized, "usage_limit_reached") {
		return false
	}
	return strings.Contains(normalized, "api returned 429")
}

const (
	adminOwnedPublicShareValidationQueueSize = 1024
)
const (
	adminOwnedPublicShareValidationTestTimeout = 30 * time.Second
)
const (
	adminOwnedPublicShareValidationWorkers = 2
)

func adminPublicShareValidationErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var appErr *infraerrors.ApplicationError
	if errors.As(err, &appErr) && strings.TrimSpace(appErr.Message) != "" {
		return strings.TrimSpace(appErr.Message)
	}
	return strings.TrimSpace(err.Error())
}

func batchRefreshErrorHTTPStatus(err error) int {
	if err == nil {
		return 0
	}
	status := infraerrors.Code(err)
	if status != infraerrors.UnknownCode {
		return status
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "http 401"):
		return http.StatusUnauthorized
	case strings.Contains(msg, "http status 401"):
		return http.StatusUnauthorized
	case strings.Contains(msg, "status 401"):
		return http.StatusUnauthorized
	case strings.Contains(msg, "status=401"):
		return http.StatusUnauthorized
	case strings.Contains(msg, "status: 401"):
		return http.StatusUnauthorized
	default:
		return 0
	}
}
func currentAdminUserID(c *gin.Context) (int64, bool) {
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		return subject.UserID, true
	}
	return 0, false
}
func (h *AccountHandler) enqueueOwnedPublicShareValidation(account *service.Account) {
	if h == nil || !shouldQueueOwnedPublicShareValidation(account) {
		return
	}
	if h.accountService == nil || h.accountTestService == nil {
		slog.Warn("admin_public_share_validation_not_ready",
			"account_id", account.ID,
			"has_account_service", h.accountService != nil,
			"has_account_test_service", h.accountTestService != nil,
		)
		return
	}
	if h.publicShareValidation == nil {
		h.publicShareValidation = make(chan ownedPublicShareValidationJob, adminOwnedPublicShareValidationQueueSize)
	}
	h.startOwnedPublicShareValidationWorkers()
	job := ownedPublicShareValidationJob{AccountID: account.ID, OwnerUserID: *account.OwnerUserID}
	select {
	case h.publicShareValidation <- job:
	default:
		slog.Warn("admin_public_share_validation_queue_full", "account_id", account.ID, "owner_user_id", *account.OwnerUserID)
	}
}
func (h *AccountHandler) executeAdminRefreshCredentialsTaskItem(ctx context.Context, task *service.AccountBatchTask, item service.AccountBatchTaskItem) (map[string]any, error) {
	account, err := h.adminService.GetAccount(ctx, item.AccountID)
	if err != nil {
		return nil, err
	}
	updated, warning, err := h.refreshSingleAccount(ctx, account)
	if err != nil {
		h.persistManualRefreshFailureState(ctx, account, err)
		return nil, err
	}
	result := map[string]any{"account_id": updated.ID}
	if strings.TrimSpace(warning) != "" {
		result["warning"] = warning
	}
	return result, nil
}

func (h *AccountHandler) markBatchRefreshAccountError(ctx context.Context, accountID int64, err error) error {
	if batchRefreshErrorHTTPStatus(err) != http.StatusUnauthorized {
		return nil
	}
	return h.adminService.SetAccountError(ctx, accountID, err.Error())
}

type ownedPublicShareValidationJob struct {
	AccountID   int64
	OwnerUserID int64
}

func parseAccountProxyFilter(c *gin.Context) (int64, error) {
	raw := strings.TrimSpace(c.Query("proxy_id"))
	if raw == "" {
		raw = strings.TrimSpace(c.Query("proxy"))
	}
	if raw == "" {
		return 0, nil
	}

	proxyID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || proxyID == 0 || proxyID < service.AccountListProxyUnassigned {
		return 0, infraerrors.BadRequest("INVALID_PROXY_FILTER", "invalid proxy filter")
	}
	return proxyID, nil
}

func (h *AccountHandler) persistManualRefreshFailureState(ctx context.Context, account *service.Account, refreshErr error) {
	if account == nil || refreshErr == nil {
		return
	}

	if service.IsNonRetryableRefreshError(refreshErr) {
		errorMsg := fmt.Sprintf("Token refresh failed (non-retryable): %v", refreshErr)
		if err := h.adminService.SetAccountError(ctx, account.ID, errorMsg); err != nil {
			slog.Warn("manual_token_refresh_set_error_failed", "account_id", account.ID, "error", err)
		}
		return
	}

	if h.rateLimitService == nil {
		return
	}

	until := time.Now().Add(service.TokenRefreshTempUnschedDuration)
	reason := fmt.Sprintf("token refresh retry exhausted: %v", refreshErr)
	if err := h.rateLimitService.SetTempUnschedulable(ctx, account, until, reason); err != nil {
		slog.Warn("manual_token_refresh_set_temp_unschedulable_failed", "account_id", account.ID, "error", err)
	}
}
func (h *AccountHandler) recoverAccountStateAfterRefresh(ctx context.Context, accountID int64) (*service.Account, error) {
	if h.rateLimitService == nil {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "RATE_LIMIT_SERVICE_UNAVAILABLE", "rate limit service unavailable")
	}

	if _, err := h.rateLimitService.RecoverAccountState(ctx, accountID, service.AccountRecoveryOptions{
		InvalidateToken: true,
	}); err != nil {
		return nil, fmt.Errorf("failed to recover account state after refreshing credentials: %w", err)
	}

	account, err := h.adminService.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return account, nil
}
func (h *AccountHandler) registerAccountBatchExecutors() {
	if h == nil || h.accountBatchTaskService == nil {
		return
	}
	h.accountBatchTaskService.RegisterExecutor(service.AccountBatchTaskOperationAdminRefreshCredentials, h.executeAdminRefreshCredentialsTaskItem)
}

func (h *AccountHandler) runOwnedPublicShareValidationWorker() {
	for job := range h.publicShareValidation {
		h.validateOwnedPublicShare(job)
	}
}
func shouldQueueOwnedPublicShareValidation(account *service.Account) bool {
	return account != nil &&
		account.ID > 0 &&
		account.OwnerUserID != nil &&
		*account.OwnerUserID > 0 &&
		service.NormalizeAccountShareMode(account.ShareMode) == service.AccountShareModePublic &&
		service.NormalizeAccountShareStatus(account.ShareStatus) == service.AccountShareStatusPending
}

func (h *AccountHandler) startOwnedPublicShareValidationWorkers() {
	h.publicShareValidationOnce.Do(func() {
		for i := 0; i < adminOwnedPublicShareValidationWorkers; i++ {
			go h.runOwnedPublicShareValidationWorker()
		}
	})
}

func (h *AccountHandler) validateOwnedPublicShare(job ownedPublicShareValidationJob) {
	ctx, cancel := context.WithTimeout(context.Background(), adminOwnedPublicShareValidationTestTimeout+30*time.Second)
	defer cancel()

	account, err := h.accountService.GetOwnedByID(ctx, job.OwnerUserID, job.AccountID)
	if err != nil {
		slog.Warn("admin_public_share_validation_account_load_failed", "account_id", job.AccountID, "owner_user_id", job.OwnerUserID, "error", err)
		return
	}
	if !shouldQueueOwnedPublicShareValidation(account) {
		return
	}

	reason := ""
	allowRateLimitedApproval := false
	testCtx, testCancel := context.WithTimeout(ctx, adminOwnedPublicShareValidationTestTimeout)
	result, err := h.accountTestService.RunTestBackground(testCtx, account.ID, "")
	testCancel()
	switch {
	case err != nil:
		reason = adminPublicShareValidationErrorMessage(err)
	case result == nil:
		reason = "account test did not return a result"
	case strings.TrimSpace(result.Status) != "success":
		reason = strings.TrimSpace(result.ErrorMessage)
		if reason == "" {
			reason = "account test failed"
		}
	}
	if adminIsOpenAIUsageLimitReachedValidationError(reason) {
		reason = ""
		allowRateLimitedApproval = true
	}
	if reason != "" {
		if _, err := h.accountService.MarkOwnedPublicSharePending(ctx, job.OwnerUserID, account.ID, reason); err != nil {
			slog.Warn("admin_public_share_validation_mark_pending_failed", "account_id", account.ID, "owner_user_id", job.OwnerUserID, "reason", reason, "error", err)
		}
		return
	}

	if _, err := h.accountService.ApproveOwnedPublicShareWithOptions(ctx, job.OwnerUserID, account.ID, service.OwnedPublicShareApprovalOptions{
		AllowRateLimited: allowRateLimitedApproval,
	}); err != nil {
		reason := adminPublicShareValidationErrorMessage(err)
		if _, markErr := h.accountService.MarkOwnedPublicSharePending(ctx, job.OwnerUserID, account.ID, reason); markErr != nil {
			slog.Warn("admin_public_share_validation_approve_failed_mark_pending_failed", "account_id", account.ID, "owner_user_id", job.OwnerUserID, "reason", reason, "approve_error", err, "mark_error", markErr)
		}
		return
	}
	slog.Info("admin_public_share_validation_approved", "account_id", account.ID, "owner_user_id", job.OwnerUserID)
}
