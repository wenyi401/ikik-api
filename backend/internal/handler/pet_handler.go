package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type PetHandler struct {
	service  *service.PetAssistantService
	activity *service.PetActivityBroker
}

func NewPetHandler(svc *service.PetAssistantService, activity *service.PetActivityBroker) *PetHandler {
	return &PetHandler{service: svc, activity: activity}
}

func petSubject(c *gin.Context) (middleware2.AuthSubject, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
	}
	return subject, ok
}

func (h *PetHandler) ListAssets(c *gin.Context) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	items, err := h.service.ListAssets(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *PetHandler) ImportAsset(c *gin.Context) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.PetZIPMaxBytes+(1<<20))
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Codex Pet ZIP is required")
		return
	}
	if file.Size <= 0 || file.Size > service.PetZIPMaxBytes {
		response.BadRequest(c, "Codex Pet ZIP is too large")
		return
	}
	reader, err := file.Open()
	if err != nil {
		response.BadRequest(c, "Cannot read Codex Pet ZIP")
		return
	}
	defer reader.Close()
	asset, err := h.service.ImportAsset(c.Request.Context(), subject.UserID, reader)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, asset)
}

func (h *PetHandler) ServeAsset(c *gin.Context) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	reader, asset, err := h.service.OpenAsset(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrPetAssetNotFound) {
			response.NotFound(c, "Pet asset not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	defer reader.Close()
	c.Header("Content-Type", "image/webp")
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Header("ETag", `"`+asset.SHA256+`"`)
	c.DataFromReader(http.StatusOK, asset.SizeBytes, "image/webp", reader, nil)
}

func (h *PetHandler) DeleteAsset(c *gin.Context) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	if err := h.service.DeleteAsset(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
		if errors.Is(err, service.ErrPetAssetNotFound) {
			response.NotFound(c, "Pet asset not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *PetHandler) GetPreferences(c *gin.Context) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	prefs, err := h.service.GetPreferences(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prefs)
}

func (h *PetHandler) SavePreferences(c *gin.Context) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	var prefs service.PetPreferences
	if err := c.ShouldBindJSON(&prefs); err != nil {
		response.BadRequest(c, "Invalid pet preferences")
		return
	}
	prefs.UserID = subject.UserID
	saved, err := h.service.SavePreferences(c.Request.Context(), prefs)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, saved)
}

type petAskRequest struct {
	ConversationID string `json:"conversation_id"`
	Question       string `json:"question"`
}

func (h *PetHandler) Ask(c *gin.Context) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	var req petAskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Question is required")
		return
	}
	h.activity.Emit(subject.UserID, service.PetActivityEvent{Type: "assistant.thinking", Status: "started"})
	conversation, message, err := h.service.Ask(c.Request.Context(), subject.UserID, req.ConversationID, req.Question)
	if err != nil {
		h.activity.Emit(subject.UserID, service.PetActivityEvent{Type: "assistant.error", Status: "failed"})
		response.BadRequest(c, err.Error())
		return
	}
	h.activity.Emit(subject.UserID, service.PetActivityEvent{Type: "assistant.done", Status: "completed"})
	response.Success(c, gin.H{"conversation_id": conversation.ID, "message": message})
}

func (h *PetHandler) GetConversation(c *gin.Context) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	conversation, err := h.service.GetConversation(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrPetConversationMissing) {
			response.NotFound(c, "Conversation not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, conversation)
}

func (h *PetHandler) ActivityStream(c *gin.Context) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	events, cleanup, err := h.activity.Subscribe(c.Request.Context(), subject.UserID)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Activity stream unavailable")
		return
	}
	defer cleanup()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ping.C:
			_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		case event, open := <-events:
			if !open {
				return
			}
			c.SSEvent("activity", event)
			flusher.Flush()
		}
	}
}

func (h *PetHandler) GatewayActivity() gin.HandlerFunc {
	if h == nil || h.activity == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if !shouldTrackPetActivity(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}
		subject, ok := middleware2.GetAuthSubjectFromContext(c)
		if !ok {
			c.Next()
			return
		}
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}
		requestID := c.GetHeader("X-Request-ID")
		h.activity.Emit(subject.UserID, service.PetActivityEvent{Type: "api.request", Status: "started", Endpoint: endpoint, RequestID: requestID})
		c.Next()
		status := "completed"
		if c.Writer.Status() >= 400 {
			status = "failed"
		}
		h.activity.Emit(subject.UserID, service.PetActivityEvent{Type: "api.request", Status: status, Endpoint: endpoint, RequestID: requestID})
	}
}

func shouldTrackPetActivity(method, requestPath string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	requestPath = "/" + strings.TrimLeft(requestPath, "/")
	if method == http.MethodGet {
		return requestPath == "/responses" || requestPath == "/v1/responses" ||
			requestPath == "/backend-api/codex/responses" || requestPath == "/realtime" || requestPath == "/v1/realtime"
	}
	if method != http.MethodPost {
		return false
	}
	if strings.Contains(requestPath, "/count_tokens") || strings.Contains(requestPath, ":countTokens") {
		return false
	}
	if strings.Contains(requestPath, "/images/tasks/") || strings.Contains(requestPath, "/images/batches/") {
		return false
	}
	if strings.Contains(requestPath, "/models/") {
		return strings.Contains(requestPath, ":generateContent") || strings.Contains(requestPath, ":streamGenerateContent")
	}
	for _, suffix := range []string{
		"/messages", "/responses", "/chat/completions", "/embeddings", "/alpha/search",
		"/images/generations", "/images/edits", "/images/generations/async", "/images/edits/async", "/images/batches",
		"/videos/generations", "/videos/edits", "/videos/extensions", "/tts", "/stt", "/custom-voices",
		"/web_search", "/x_search",
	} {
		if requestPath == suffix || strings.HasSuffix(requestPath, suffix) {
			return true
		}
	}
	return strings.Contains(requestPath, "/responses/")
}

func (h *PetHandler) AdminListKnowledge(c *gin.Context) {
	items, err := h.service.ListKnowledge(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *PetHandler) AdminCreateKnowledge(c *gin.Context) { h.adminSaveKnowledge(c, 0) }
func (h *PetHandler) AdminUpdateKnowledge(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid document ID")
		return
	}
	h.adminSaveKnowledge(c, id)
}
func (h *PetHandler) adminSaveKnowledge(c *gin.Context, id int64) {
	subject, ok := petSubject(c)
	if !ok {
		return
	}
	var doc service.PetKnowledgeDocument
	if err := c.ShouldBindJSON(&doc); err != nil {
		response.BadRequest(c, "Invalid knowledge document")
		return
	}
	saved, err := h.service.SaveKnowledge(c.Request.Context(), subject.UserID, id, doc)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if id == 0 {
		response.Created(c, saved)
	} else {
		response.Success(c, saved)
	}
}
func (h *PetHandler) AdminDeleteKnowledge(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid document ID")
		return
	}
	if err := h.service.DeleteKnowledge(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrPetKnowledgeNotFound) {
			response.NotFound(c, "Knowledge document not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

var _ = strings.TrimSpace
