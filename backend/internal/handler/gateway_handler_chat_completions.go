package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
	"ikik-api/internal/pkg/ip"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

// ChatCompletions handles OpenAI Chat Completions API endpoint for Anthropic platform groups.
// POST /v1/chat/completions
// This converts Chat Completions requests to Anthropic format (via Responses format chain),
// forwards to Anthropic upstream, and converts responses back to Chat Completions format.
func (h *GatewayHandler) ChatCompletions(c *gin.Context) {
	streamStarted := false

	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.chatCompletionsErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.chatCompletionsErrorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.gateway.chat_completions",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	// Read request body
	body, err := readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.chatCompletionsErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	if len(body) == 0 {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	setOpsRequestContext(c, "", false)

	// Validate JSON
	if !gjson.ValidBytes(body) {
		logRequestBodyParseFailure(reqLog, body, nil)
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	// Extract model and stream
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()
	ensureCompositeTargetPlatform(c, apiKey, reqModel)
	if !compositeTargetPlatformResolved(c, apiKey, reqModel) {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Model is not supported by composite groups")
		return
	}
	reqStream, ok := parseOpenAICompatibleStream(body)
	if !ok {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", invalidStreamFieldTypeMessage)
		return
	}
	if service.IsGPTImageGenerationModel(reqModel) {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "This model is not supported on the Chat Completions endpoint")
		return
	}
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))
	if !hasNonClaudeCodeAPIKeyGroupRoute(apiKey) {
		h.chatCompletionsErrorResponse(c, http.StatusForbidden, "permission_error", "This group is restricted to Claude Code clients (/v1/messages only)")
		return
	}

	setOpsRequestContext(c, reqModel, reqStream)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))

	preFlightDecision := h.runPreFlightHooks(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIChat, reqModel, body)
	auditDecision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIChat, reqModel, body)
	if preFlightDecision != nil && preFlightDecision.Blocked {
		h.chatCompletionsErrorResponse(c, preFlightStatus(preFlightDecision), preFlightErrorCode(preFlightDecision), preFlightDecision.Message)
		return
	}
	if auditDecision != nil && !auditDecision.AllowNextStage {
		h.openAISecurityAuditError(c, auditDecision)
		return
	}

	// Error passthrough binding
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	userReleaseFunc, err := h.concurrencyHelper.AcquireUserSlotWithWait(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted)
	if err != nil {
		reqLog.Warn("gateway.cc.user_slot_acquire_failed", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", streamStarted)
		return
	}
	userReleaseFunc = wrapReleaseOnDone(c.Request.Context(), userReleaseFunc)
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	// Parse request for session hash
	bodyRef := service.NewRequestBodyRef(body)
	parsedReq, _ := service.ParseGatewayRequest(bodyRef, "chat_completions")
	if parsedReq == nil {
		parsedReq = &service.ParsedRequest{Model: reqModel, Stream: reqStream, Body: bodyRef}
	}
	parsedReq.SessionContext = &service.SessionContext{
		ClientIP:  ip.GetClientIP(c),
		UserAgent: c.GetHeader("User-Agent"),
		APIKeyID:  apiKey.ID,
	}
	sessionHash := h.gatewayService.GenerateSessionHash(parsedReq)

	// 3. Account selection + failover loop
	routeCursor := newAPIKeyGroupRouteCursor(apiKey)
	if _, ok := routeCursor.current(); !ok {
		h.chatCompletionsErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available API key group routes")
		return
	}
	var fs *FailoverState
	var activeRouteGroupID int64

	for {
		if c.Request.Context().Err() != nil {
			return
		}
		routeCandidate, ok := routeCursor.current()
		if !ok {
			h.chatCompletionsErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available API key group routes")
			return
		}
		currentAPIKey := routeCandidate.APIKey
		if currentAPIKey.Group != nil && currentAPIKey.Group.ClaudeCodeOnly {
			if routeCursor.skipToNext("claude_code_only", reqLog, zap.Int64p("group_id", currentAPIKey.GroupID)) {
				fs = nil
				activeRouteGroupID = 0
				continue
			}
			h.chatCompletionsErrorResponse(c, http.StatusForbidden, "permission_error", "This group is restricted to Claude Code clients (/v1/messages only)")
			return
		}
		routeCtx := gatewayRouteContext(c.Request.Context(), currentAPIKey, subject.UserID)
		currentSubscription, subErr := h.gatewayService.ResolveRouteSubscription(routeCtx, currentAPIKey, subscription)
		if subErr != nil {
			status, code, message, retryAfter := billingErrorDetails(subErr)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.chatCompletionsErrorResponse(c, status, code, message)
			return
		}
		if err := h.billingCacheService.CheckBillingEligibility(routeCtx, currentAPIKey.User, currentAPIKey, currentAPIKey.Group, currentSubscription, service.QuotaPlatform(routeCtx, currentAPIKey)); err != nil {
			reqLog.Info("gateway.cc.billing_check_failed", zap.Error(err), zap.Int64p("group_id", currentAPIKey.GroupID))
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.chatCompletionsErrorResponse(c, status, code, message)
			return
		}
		groupPlatform := effectiveAPIKeyPlatform(c, currentAPIKey)
		if fs == nil || activeRouteGroupID != routeCandidate.Route.GroupID {
			fs = NewFailoverState(h.maxAccountSwitches, false)
			if groupPlatform == service.PlatformGemini {
				fs = NewFailoverState(h.maxAccountSwitchesGemini, false)
			}
			activeRouteGroupID = routeCandidate.Route.GroupID
		}
		selectionSessionHash := sessionHash
		if groupPlatform == service.PlatformGemini && selectionSessionHash != "" {
			selectionSessionHash = "gemini:" + selectionSessionHash
		}
		channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(routeCtx, currentAPIKey.GroupID, reqModel)
		selection, err := h.gatewayService.SelectAccountWithLoadAwareness(routeCtx, currentAPIKey.GroupID, selectionSessionHash, reqModel, fs.FailedAccountIDs, "", int64(0))
		if err != nil {
			if len(fs.FailedAccountIDs) == 0 {
				if routeCursor.switchToNext(apiKey.ID, "account_select_failed", reqLog, zap.Error(err)) {
					fs = nil
					activeRouteGroupID = 0
					continue
				}
				cls := classifyNoAccountErrorFromGin(c, h.gatewayService, currentAPIKey, reqModel, reqModel, groupPlatform)
				if !cls.ModelNotFound {
					markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
				}
				message := cls.Message
				if !cls.ModelNotFound {
					message = "No available accounts: " + err.Error()
				}
				h.chatCompletionsErrorResponse(c, cls.Status, cls.ErrType, message)
				return
			}
			action := fs.HandleSelectionExhausted(c.Request.Context())
			switch action {
			case FailoverContinue:
				continue
			case FailoverCanceled:
				failoverClientGone(c)
				return
			default:
				if fs.LastFailoverErr != nil {
					if shouldSwitchAPIKeyGroupRoute(fs.LastFailoverErr) && routeCursor.switchToNext(apiKey.ID, "account_selection_exhausted", reqLog, zap.Int("upstream_status", fs.LastFailoverErr.StatusCode)) {
						fs = nil
						activeRouteGroupID = 0
						continue
					}
					h.handleCCFailoverExhausted(c, fs.LastFailoverErr, streamStarted)
				} else {
					if routeCursor.switchToNext(apiKey.ID, "account_selection_exhausted", reqLog) {
						fs = nil
						activeRouteGroupID = 0
						continue
					}
					h.chatCompletionsErrorResponse(c, http.StatusBadGateway, "server_error", "All available accounts exhausted")
				}
				return
			}
		}
		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)

		// 4. Acquire account concurrency slot
		accountReleaseFunc := selection.ReleaseFunc
		if !selection.Acquired {
			if selection.WaitPlan == nil {
				if routeCursor.switchToNext(apiKey.ID, "account_concurrency_exhausted", reqLog) {
					fs = nil
					activeRouteGroupID = 0
					continue
				}
				markOpsRoutingCapacityLimited(c)
				h.chatCompletionsErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available accounts")
				return
			}
			accountReleaseFunc, err = h.concurrencyHelper.AcquireAccountSlotWithWaitTimeout(
				c,
				account.ID,
				selection.WaitPlan.MaxConcurrency,
				selection.WaitPlan.Timeout,
				reqStream,
				&streamStarted,
			)
			if err != nil {
				reqLog.Warn("gateway.cc.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				h.handleConcurrencyError(c, err, "account", streamStarted)
				return
			}
		}
		accountReleaseFunc = wrapReleaseOnDone(c.Request.Context(), accountReleaseFunc)

		if groupPlatform == service.PlatformGemini && account.Platform != service.PlatformGemini {
			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}
			fs.FailedAccountIDs[account.ID] = struct{}{}
			continue
		}

		// 5. Forward request
		writerSizeBeforeForward := c.Writer.Size()
		forwardBody := body
		if channelMapping.Mapped {
			forwardBody = h.gatewayService.ReplaceModelInBody(body, channelMapping.MappedModel)
		}
		var result *service.ForwardResult
		if account.Platform == service.PlatformGemini {
			if h.geminiCompatService == nil {
				h.chatCompletionsErrorResponse(c, http.StatusBadGateway, "upstream_error", "Gemini compatibility service is not configured")
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
				return
			}
			result, err = h.geminiCompatService.ForwardAsChatCompletions(routeCtx, c, account, forwardBody)
		} else {
			result, err = h.gatewayService.ForwardAsChatCompletions(routeCtx, c, account, forwardBody, parsedReq)
		}

		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}

		if err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				if c.Writer.Size() != writerSizeBeforeForward {
					h.handleCCFailoverExhausted(c, failoverErr, true)
					return
				}
				action := fs.HandleFailoverError(c.Request.Context(), h.gatewayService, account.ID, account.Platform, account.GetPoolModeRetryCount(), failoverErr)
				switch action {
				case FailoverContinue:
					continue
				case FailoverExhausted:
					if shouldSwitchAPIKeyGroupRoute(fs.LastFailoverErr) && routeCursor.switchToNext(apiKey.ID, "upstream_failover_exhausted", reqLog, zap.Int("upstream_status", fs.LastFailoverErr.StatusCode)) {
						fs = nil
						activeRouteGroupID = 0
						continue
					}
					h.handleCCFailoverExhausted(c, fs.LastFailoverErr, streamStarted)
					return
				case FailoverCanceled:
					failoverClientGone(c)
					return
				}
			}
			upstreamErrorAlreadyCommunicated := gatewayForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, err)
			wroteFallback := false
			if !upstreamErrorAlreadyCommunicated {
				wroteFallback = h.ensureForwardErrorResponse(c, streamStarted)
			}
			reqLog.Error("gateway.cc.forward_failed",
				zap.Int64("account_id", account.ID),
				zap.Bool("fallback_error_response_written", wroteFallback),
				zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
				zap.Error(err),
			)
			return
		}
		routeCursor.recordSuccess(apiKey.ID)

		// 6. Record usage
		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		requestPayloadHash := service.HashUsageRequestPayload(body)
		inboundEndpoint := GetInboundEndpoint(c)
		upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)

		quotaPlatform := service.QuotaPlatform(routeCtx, currentAPIKey)
		h.submitUsageRecordTask(routeCtx, func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
				Result:             result,
				QuotaPlatform:      quotaPlatform,
				APIKey:             currentAPIKey,
				User:               currentAPIKey.User,
				Account:            account,
				Subscription:       currentSubscription,
				InboundEndpoint:    inboundEndpoint,
				UpstreamEndpoint:   upstreamEndpoint,
				UserAgent:          userAgent,
				IPAddress:          clientIP,
				RequestPayloadHash: requestPayloadHash,
				APIKeyService:      h.apiKeyService,
				ChannelUsageFields: clientRequestedUsageFields(c, channelMapping, reqModel, result.UpstreamModel),
			}); err != nil {
				reqLog.Error("gateway.cc.record_usage_failed",
					zap.Int64("account_id", account.ID),
					zap.Error(err),
				)
			}
		})
		return
	}
}

// chatCompletionsErrorResponse writes an error in OpenAI Chat Completions format.
func (h *GatewayHandler) chatCompletionsErrorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// handleCCFailoverExhausted writes a failover-exhausted error in CC format.
func (h *GatewayHandler) handleCCFailoverExhausted(c *gin.Context, lastErr *service.UpstreamFailoverError, streamStarted bool) {
	if streamStarted {
		return
	}
	if lastErr != nil {
		copyFailoverRetryAfter(c, lastErr.ResponseHeaders)
	}
	if lastErr != nil && lastErr.IsCredentialFailure() {
		status, message := credentialFailoverClientResponse(lastErr)
		h.chatCompletionsErrorResponse(c, status, "server_error", message)
		return
	}
	statusCode := http.StatusBadGateway
	if lastErr != nil && lastErr.StatusCode > 0 {
		statusCode = lastErr.StatusCode
	}
	if lastErr != nil && service.IsOpenAISilentRefusalErrorBody(lastErr.ResponseBody) {
		service.SetOpsUpstreamError(c, statusCode, service.OpenAISilentRefusalClientMessage(), "")
		h.chatCompletionsErrorResponse(c, http.StatusBadGateway, "upstream_error", service.OpenAISilentRefusalClientMessage())
		return
	}
	h.chatCompletionsErrorResponse(c, statusCode, "server_error", "All available accounts exhausted")
}
