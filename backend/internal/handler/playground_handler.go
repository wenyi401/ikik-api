package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"ikik-api/internal/config"
	"ikik-api/internal/pkg/ctxkey"
	"ikik-api/internal/pkg/ip"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

const playgroundAPIKeyPrefix = "Playground #"

// PlaygroundHandler exposes authenticated in-app testing endpoints while
// keeping scheduling, billing, moderation, and usage logging on the normal gateway path.
type PlaygroundHandler struct {
	apiKeyService       *service.APIKeyService
	subscriptionService *service.SubscriptionService
	gateway             *GatewayHandler
	openaiGateway       *OpenAIGatewayHandler
	cfg                 *config.Config
}

type playgroundModelsResponse struct {
	GroupID      int64    `json:"group_id"`
	Models       []string `json:"models"`
	DefaultModel string   `json:"default_model"`
}

func NewPlaygroundHandler(
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	gateway *GatewayHandler,
	openaiGateway *OpenAIGatewayHandler,
	cfg *config.Config,
) *PlaygroundHandler {
	return &PlaygroundHandler{
		apiKeyService:       apiKeyService,
		subscriptionService: subscriptionService,
		gateway:             gateway,
		openaiGateway:       openaiGateway,
		cfg:                 cfg,
	}
}

// ChatCompletions handles the user playground chat request.
//
// POST /api/v1/playground/chat/completions
func (h *PlaygroundHandler) ChatCompletions(c *gin.Context) {
	if h == nil || h.apiKeyService == nil || h.gateway == nil || h.openaiGateway == nil {
		playgroundError(c, http.StatusServiceUnavailable, "api_error", "Playground is not available")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		playgroundError(c, http.StatusUnauthorized, "authentication_error", "User not authenticated")
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		playgroundError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		playgroundError(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	groupID, cleanBody, err := sanitizePlaygroundChatBody(body)
	if err != nil {
		playgroundError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	group, err := h.resolveAvailableGroup(c.Request.Context(), subject.UserID, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if group == nil {
		playgroundError(c, http.StatusForbidden, "permission_error", "Group is not available")
		return
	}

	apiKey, err := h.getOrCreatePlaygroundKey(c.Request.Context(), subject.UserID, group)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if apiKey == nil || apiKey.User == nil || apiKey.Group == nil {
		playgroundError(c, http.StatusInternalServerError, "api_error", "Failed to prepare playground API key")
		return
	}

	subscription, ok := h.validateAPIKeyAccess(c, apiKey)
	if !ok {
		return
	}

	h.bindGatewayContext(c, apiKey, subscription, cleanBody)

	if apiKey.Group.Platform == service.PlatformOpenAI || apiKey.Group.Platform == service.PlatformGrok || apiKey.Group.Platform == service.PlatformKiro {
		h.openaiGateway.ChatCompletions(c)
		return
	}
	h.gateway.ChatCompletions(c)
}

// Models lists text models currently available to the authenticated user in a
// selected group. The result uses the same account pool and custom model-list
// rules as the billed playground request path.
//
// GET /api/v1/playground/models?group_id=123
func (h *PlaygroundHandler) Models(c *gin.Context) {
	if h == nil || h.apiKeyService == nil || h.gateway == nil {
		playgroundError(c, http.StatusServiceUnavailable, "api_error", "Playground is not available")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		playgroundError(c, http.StatusUnauthorized, "authentication_error", "User not authenticated")
		return
	}
	groupID, err := strconv.ParseInt(strings.TrimSpace(c.Query("group_id")), 10, 64)
	if err != nil || groupID <= 0 {
		playgroundError(c, http.StatusBadRequest, "invalid_request_error", "group_id is invalid")
		return
	}
	group, err := h.resolveAvailableGroup(c.Request.Context(), subject.UserID, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if group == nil {
		playgroundError(c, http.StatusForbidden, "permission_error", "Group is not available")
		return
	}
	if group.ClaudeCodeOnly {
		playgroundError(c, http.StatusBadRequest, "invalid_request_error", "Selected group only supports Claude Code requests")
		return
	}
	models := h.availableTextModels(c.Request.Context(), group)
	if len(models) == 0 {
		playgroundError(c, http.StatusNotFound, "not_found_error", "Selected group has no text model available")
		return
	}
	defaultModel := h.preferredTextModel(group, models)
	c.JSON(http.StatusOK, playgroundModelsResponse{
		GroupID: group.ID, Models: models, DefaultModel: defaultModel,
	})
}

func sanitizePlaygroundChatBody(body []byte) (int64, []byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, nil, fmt.Errorf("failed to parse request body")
	}

	groupRaw, ok := raw["group_id"]
	if !ok || len(bytes.TrimSpace(groupRaw)) == 0 {
		return 0, nil, fmt.Errorf("group_id is required")
	}
	var groupID int64
	if err := json.Unmarshal(groupRaw, &groupID); err != nil || groupID <= 0 {
		return 0, nil, fmt.Errorf("group_id is invalid")
	}

	modelRaw, ok := raw["model"]
	if !ok || len(bytes.TrimSpace(modelRaw)) == 0 {
		return 0, nil, fmt.Errorf("model is required")
	}
	var model string
	if err := json.Unmarshal(modelRaw, &model); err != nil || strings.TrimSpace(model) == "" {
		return 0, nil, fmt.Errorf("model is required")
	}

	if messagesRaw, ok := raw["messages"]; !ok || len(bytes.TrimSpace(messagesRaw)) == 0 {
		return 0, nil, fmt.Errorf("messages is required")
	}

	delete(raw, "group_id")
	cleanBody, err := json.Marshal(raw)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to normalize request body")
	}
	return groupID, cleanBody, nil
}

func (h *PlaygroundHandler) resolveAvailableGroup(ctx context.Context, userID, groupID int64) (*service.Group, error) {
	groups, err := h.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if groups[i].ID == groupID {
			group := groups[i]
			return &group, nil
		}
	}
	return nil, nil
}

func (h *PlaygroundHandler) availableTextModels(ctx context.Context, group *service.Group) []string {
	if h == nil || h.gateway == nil || group == nil {
		return nil
	}
	var models []string
	if group.Platform == service.PlatformComposite {
		models = h.gateway.compositeAvailableModels(ctx, &group.ID)
	} else if h.gateway.gatewayService != nil {
		models = h.gateway.gatewayService.GetAvailableModels(ctx, &group.ID, group.Platform)
	}
	fallback := defaultModelIDsForPlatform(group.Platform)
	if group.CustomModelsListEnabled() {
		models = filterModelsByCustomList(customModelsListSource(group.Platform, models, fallback), fallback, group.ModelsListConfig.Models)
	} else if len(models) == 0 {
		models = fallback
	}
	models = playgroundTextModels(models)
	sort.SliceStable(models, func(i, j int) bool {
		left, right := playgroundModelScore(models[i]), playgroundModelScore(models[j])
		if left == right {
			return models[i] < models[j]
		}
		return left < right
	})
	return models
}

func (h *PlaygroundHandler) preferredTextModel(group *service.Group, models []string) string {
	if group != nil {
		if preferred := strings.TrimSpace(group.DefaultMappedModel); preferred != "" {
			for _, model := range models {
				if model == preferred {
					return model
				}
			}
		}
	}
	if len(models) == 0 {
		return ""
	}
	return models[0]
}

func playgroundTextModels(models []string) []string {
	result := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		lower := strings.ToLower(model)
		if model == "" || strings.Contains(model, "*") {
			continue
		}
		if strings.Contains(lower, "image") || strings.Contains(lower, "video") ||
			strings.Contains(lower, "embedding") || strings.Contains(lower, "whisper") ||
			strings.Contains(lower, "tts") || strings.Contains(lower, "audio") {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		result = append(result, model)
	}
	return result
}

func playgroundModelScore(model string) int {
	lower := strings.ToLower(model)
	score := 20
	for _, marker := range []string{"flash", "mini", "nano", "haiku", "air", "fast"} {
		if strings.Contains(lower, marker) {
			score -= 8
			break
		}
	}
	for _, marker := range []string{"opus", "pro", "max"} {
		if strings.Contains(lower, marker) {
			score += 5
			break
		}
	}
	return score
}

func (h *PlaygroundHandler) getOrCreatePlaygroundKey(ctx context.Context, userID int64, group *service.Group) (*service.APIKey, error) {
	name := playgroundKeyName(group)
	groupID := group.ID
	keys, _, err := h.apiKeyService.List(ctx, userID, pagination.PaginationParams{
		Page:      1,
		PageSize:  50,
		SortBy:    "id",
		SortOrder: "desc",
	}, service.APIKeyListFilters{
		Search:  name,
		GroupID: &groupID,
	})
	if err != nil {
		return nil, err
	}
	for i := range keys {
		if keys[i].Name == name {
			return h.apiKeyService.GetByID(ctx, keys[i].ID)
		}
	}

	created, err := h.apiKeyService.Create(ctx, userID, service.CreateAPIKeyRequest{
		Name:    name,
		GroupID: &groupID,
	})
	if err != nil {
		return nil, err
	}
	return h.apiKeyService.GetByID(ctx, created.ID)
}

func (h *PlaygroundHandler) validateAPIKeyAccess(c *gin.Context, apiKey *service.APIKey) (*service.UserSubscription, bool) {
	if apiKey == nil || apiKey.User == nil || apiKey.Group == nil {
		playgroundError(c, http.StatusInternalServerError, "api_error", "Failed to prepare playground API key")
		return nil, false
	}

	if h.cfg != nil && h.cfg.RunMode == config.RunModeSimple {
		_ = h.apiKeyService.TouchLastUsed(c.Request.Context(), apiKey.ID)
		return nil, true
	}

	if !apiKey.IsActive() &&
		apiKey.Status != service.StatusAPIKeyExpired &&
		apiKey.Status != service.StatusAPIKeyQuotaExhausted {
		playgroundError(c, http.StatusUnauthorized, "API_KEY_DISABLED", "API key is disabled")
		return nil, false
	}
	if len(apiKey.IPWhitelist) > 0 || len(apiKey.IPBlacklist) > 0 {
		clientIP := ip.GetTrustedClientIP(c)
		if h.cfg != nil && h.cfg.TrustForwardedIPForAPIKeyACL() {
			clientIP = ip.GetClientIP(c)
		}
		allowed, _ := ip.CheckIPRestrictionWithCompiledRules(clientIP, apiKey.CompiledIPWhitelist, apiKey.CompiledIPBlacklist)
		if !allowed {
			playgroundError(c, http.StatusForbidden, "ACCESS_DENIED", "Access denied")
			return nil, false
		}
	}
	if !apiKey.User.IsActive() {
		playgroundError(c, http.StatusUnauthorized, "USER_INACTIVE", "User account is not active")
		return nil, false
	}

	switch apiKey.Status {
	case service.StatusAPIKeyQuotaExhausted:
		playgroundError(c, http.StatusTooManyRequests, "API_KEY_QUOTA_EXHAUSTED", "API key 额度已用完")
		return nil, false
	case service.StatusAPIKeyExpired:
		playgroundError(c, http.StatusForbidden, "API_KEY_EXPIRED", "API key 已过期")
		return nil, false
	}
	if apiKey.IsExpired() {
		playgroundError(c, http.StatusForbidden, "API_KEY_EXPIRED", "API key 已过期")
		return nil, false
	}
	if apiKey.IsQuotaExhausted() {
		playgroundError(c, http.StatusTooManyRequests, "API_KEY_QUOTA_EXHAUSTED", "API key 额度已用完")
		return nil, false
	}

	var subscription *service.UserSubscription
	if apiKey.Group.IsSubscriptionType() && h.subscriptionService != nil {
		sub, err := h.subscriptionService.GetActiveSubscription(c.Request.Context(), apiKey.User.ID, apiKey.Group.ID)
		if err != nil {
			playgroundError(c, http.StatusForbidden, "SUBSCRIPTION_NOT_FOUND", "No active subscription found for this group")
			return nil, false
		}
		subscription = sub
	}

	if subscription != nil {
		needsMaintenance, err := h.subscriptionService.ValidateAndCheckLimits(subscription, apiKey.Group)
		if err != nil {
			code := "SUBSCRIPTION_INVALID"
			status := http.StatusForbidden
			if errors.Is(err, service.ErrDailyLimitExceeded) ||
				errors.Is(err, service.ErrWeeklyLimitExceeded) ||
				errors.Is(err, service.ErrMonthlyLimitExceeded) {
				code = "USAGE_LIMIT_EXCEEDED"
				status = http.StatusTooManyRequests
			}
			playgroundError(c, status, code, err.Error())
			return nil, false
		}
		if needsMaintenance {
			maintenanceCopy := *subscription
			h.subscriptionService.DoWindowMaintenance(&maintenanceCopy)
		}
	} else if !service.HasUsageBillingFunds(apiKey.User) {
		playgroundError(c, http.StatusForbidden, "INSUFFICIENT_BALANCE", "Insufficient account balance")
		return nil, false
	}

	_ = h.apiKeyService.TouchLastUsed(c.Request.Context(), apiKey.ID)
	return subscription, true
}

func playgroundKeyName(group *service.Group) string {
	if group == nil {
		return playgroundAPIKeyPrefix + "unknown"
	}
	name := fmt.Sprintf("%s%d", playgroundAPIKeyPrefix, group.ID)
	groupName := strings.TrimSpace(group.Name)
	if groupName == "" {
		return name
	}
	withGroup := name + " " + groupName
	if utf8.RuneCountInString(withGroup) <= 100 {
		return withGroup
	}
	runes := []rune(withGroup)
	return string(runes[:100])
}

func (h *PlaygroundHandler) bindGatewayContext(c *gin.Context, apiKey *service.APIKey, subscription *service.UserSubscription, body []byte) {
	if subscription != nil {
		c.Set(string(middleware2.ContextKeySubscription), subscription)
	}
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{
		UserID:      apiKey.User.ID,
		Concurrency: apiKey.User.Concurrency,
	})
	if apiKey.User != nil {
		c.Set(string(middleware2.ContextKeyUserRole), apiKey.User.Role)
	}
	if c.Request != nil {
		ctx := context.WithValue(c.Request.Context(), ctxkey.AuthenticatedUserID, apiKey.User.ID)
		if service.IsGroupContextValid(apiKey.Group) {
			ctx = context.WithValue(ctx, ctxkey.Group, apiKey.Group)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
		if c.Request.URL != nil {
			c.Request.URL.Path = EndpointChatCompletions
			c.Request.URL.RawPath = ""
		}
	}
	c.Set(ctxKeyInboundEndpoint, EndpointChatCompletions)
}

func playgroundError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}
