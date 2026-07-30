package handler

import (
	"net/http"

	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type ServiceStatusHandler struct {
	openAIStatusService             *service.OpenAIStatusService
	providerStatusService           *service.ProviderStatusService
	promptLibraryTranslationService *service.PromptLibraryTranslationService
}

func NewServiceStatusHandler(
	openAIStatusService *service.OpenAIStatusService,
	providerStatusService *service.ProviderStatusService,
	promptLibraryTranslationService *service.PromptLibraryTranslationService,
) *ServiceStatusHandler {
	return &ServiceStatusHandler{
		openAIStatusService:             openAIStatusService,
		providerStatusService:           providerStatusService,
		promptLibraryTranslationService: promptLibraryTranslationService,
	}
}

func (h *ServiceStatusHandler) GetOpenAI(c *gin.Context) {
	snapshot, err := h.openAIStatusService.Get(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "OpenAI status is temporarily unavailable")
		return
	}
	response.Success(c, snapshot)
}

func (h *ServiceStatusHandler) GetProviders(c *gin.Context) {
	snapshot, err := h.providerStatusService.Get(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Provider status is temporarily unavailable")
		return
	}
	response.Success(c, snapshot)
}
