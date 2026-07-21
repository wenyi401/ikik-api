package admin

import "ikik-api/internal/service"

// ConfigureIkikExtensions attaches product-specific account services without
// changing the upstream account-handler constructor.
func (h *AccountHandler) ConfigureIkikExtensions(
	accountService *service.AccountService,
	kiroOAuthService *service.KiroOAuthService,
	accountBatchTaskService *service.AccountBatchTaskService,
) {
	if h == nil {
		return
	}
	h.accountService = accountService
	h.kiroOAuthService = kiroOAuthService
	h.accountBatchTaskService = accountBatchTaskService
	if h.publicShareValidation == nil {
		h.publicShareValidation = make(chan ownedPublicShareValidationJob, adminOwnedPublicShareValidationQueueSize)
	}
	h.registerAccountBatchExecutors()
}

// ConfigureIkikExtensions attaches product-specific group services without
// changing the upstream group-handler constructor.
func (h *GroupHandler) ConfigureIkikExtensions(groupRateScheduleService *service.GroupRateScheduleService) {
	if h == nil {
		return
	}
	h.groupRateScheduleService = groupRateScheduleService
}
