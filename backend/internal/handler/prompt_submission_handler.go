package handler

import (
	"time"

	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type PromptSubmissionHandler struct {
	service *service.PromptSubmissionService
}

func NewPromptSubmissionHandler(promptSubmissionService *service.PromptSubmissionService) *PromptSubmissionHandler {
	return &PromptSubmissionHandler{service: promptSubmissionService}
}

type createPromptSubmissionRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Content     string `json:"content" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Category    string `json:"category"`
	MediaURL    string `json:"media_url"`
}

type publicPromptSubmissionResponse struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Type        string    `json:"type"`
	Category    string    `json:"category"`
	MediaURL    string    `json:"media_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func (h *PromptSubmissionHandler) Submit(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req createPromptSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	created, err := h.service.Submit(c.Request.Context(), service.CreatePromptSubmissionInput{
		UserID:      subject.UserID,
		Title:       req.Title,
		Description: req.Description,
		Content:     req.Content,
		Type:        req.Type,
		Category:    req.Category,
		MediaURL:    req.MediaURL,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, created)
}

func (h *PromptSubmissionHandler) ListApproved(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.service.ListApproved(c.Request.Context(), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result := make([]publicPromptSubmissionResponse, 0, len(items))
	for i := range items {
		item := items[i]
		result = append(result, publicPromptSubmissionResponse{
			ID:          item.ID,
			Username:    item.Username,
			Title:       item.Title,
			Description: item.Description,
			Content:     item.Content,
			Type:        item.Type,
			Category:    item.Category,
			MediaURL:    item.MediaURL,
			CreatedAt:   item.CreatedAt,
		})
	}
	response.Paginated(c, result, total, page, pageSize)
}
