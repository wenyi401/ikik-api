package handler

import "ikik-api/internal/service"

func (h *ChannelMonitorUserHandler) configureIkikExtensions(groupCapacityService *service.GroupCapacityService) {
	if h != nil {
		h.groupCapacityService = groupCapacityService
	}
}

func (h *PaymentHandler) configureIkikExtensions(channelService *service.ChannelService) {
	if h != nil {
		h.channelService = channelService
	}
}
