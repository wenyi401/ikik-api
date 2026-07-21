package handler

import "ikik-api/internal/gatewayhook"

func (h *GatewayHandler) SetPreFlightHooks(hooks *gatewayhook.Chain) *GatewayHandler {
	if h != nil {
		h.preFlightHooks = hooks
	}
	return h
}

func (h *OpenAIGatewayHandler) SetPreFlightHooks(hooks *gatewayhook.Chain) *OpenAIGatewayHandler {
	if h != nil {
		h.preFlightHooks = hooks
	}
	return h
}
