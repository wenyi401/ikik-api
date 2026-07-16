package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
