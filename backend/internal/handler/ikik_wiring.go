package handler

import "ikik-api/internal/service"

func (h *ChannelMonitorUserHandler) configureIkikExtensions(
	groupCapacityService *service.GroupCapacityService,
	accountService *service.AccountService,
) {
	if h != nil {
		h.groupCapacityService = groupCapacityService
		h.accountService = accountService
	}
}

func (h *PaymentHandler) configureIkikExtensions(channelService *service.ChannelService) {
	if h != nil {
		h.channelService = channelService
	}
}
