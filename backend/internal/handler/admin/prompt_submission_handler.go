package admin

import (
	"strconv"

	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type PromptSubmissionHandler struct {
	service            *service.PromptSubmissionService
	translationService *service.PromptLibraryTranslationService
}

func NewPromptSubmissionHandler(
	promptSubmissionService *service.PromptSubmissionService,
	translationService *service.PromptLibraryTranslationService,
) *PromptSubmissionHandler {
	return &PromptSubmissionHandler{
		service:            promptSubmissionService,
		translationService: translationService,
	}
}

type reviewPromptSubmissionRequest struct {
	Status string `json:"status" binding:"required"`
	Note   string `json:"note"`
}

type updatePromptLibraryTranslationConfigRequest struct {
	Enabled bool   `json:"enabled"`
	GroupID int64  `json:"group_id"`
	Model   string `json:"model"`
}

func (h *PromptSubmissionHandler) GetTranslationConfig(c *gin.Context) {
	config, err := h.translationService.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func (h *PromptSubmissionHandler) UpdateTranslationConfig(c *gin.Context) {
	var req updatePromptLibraryTranslationConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	config, err := h.translationService.UpdateConfig(c.Request.Context(), service.PromptLibraryTranslationConfig{
		Enabled: req.Enabled,
		GroupID: req.GroupID,
		Model:   req.Model,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func (h *PromptSubmissionHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.service.AdminList(c.Request.Context(), service.PromptSubmissionAdminFilters{
		Page:     page,
		PageSize: pageSize,
		Status:   c.Query("status"),
		Search:   c.Query("search"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *PromptSubmissionHandler) Review(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid prompt submission ID")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req reviewPromptSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.service.Review(c.Request.Context(), id, subject.UserID, req.Status, req.Note)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
