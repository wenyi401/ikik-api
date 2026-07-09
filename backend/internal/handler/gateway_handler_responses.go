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
	pkghttputil "ikik-api/internal/pkg/httputil"
	"ikik-api/internal/pkg/ip"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

// Responses handles OpenAI Responses API endpoint for Anthropic platform groups.
// POST /v1/responses
// This converts Responses API requests to Anthropic format, forwards to Anthropic
// upstream, and converts responses back to Responses format.
func (h *GatewayHandler) Responses(c *gin.Context) {
	streamStarted := false

	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.responsesErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.responsesErrorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.gateway.responses",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	// Read request body
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.responsesErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	if len(body) == 0 {
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	setOpsRequestContext(c, "", false, body)

	// Validate JSON
	if !gjson.ValidBytes(body) {
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	// Extract model and stream using gjson (like OpenAI handler)
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()
	reqStream := gjson.GetBytes(body, "stream").Bool()
	requestedModel := reqModel
	autoDecision := h.gatewayService.ResolveAutoModel(c.Request.Context(), apiKey.GroupID, reqModel, body, service.AutoModelProtocolOpenAIResponses)
	if autoDecision.Matched {
		reqModel = autoDecision.ResolvedModel
		body = h.gatewayService.ReplaceModelInBody(body, reqModel)
		body = service.StripAutoRouterPluginFromBody(body)
	}
	reqLog = reqLog.With(
		zap.String("model", requestedModel),
		zap.String("routing_model", reqModel),
		zap.Bool("stream", reqStream),
		zap.Bool("auto_model", autoDecision.Matched),
	)

	setOpsRequestContext(c, requestedModel, reqStream, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))

	if decision := h.runPreFlightHooks(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, reqModel, body); decision != nil && decision.Blocked {
		h.responsesErrorResponse(c, preFlightStatus(decision), preFlightErrorCode(decision), decision.Message)
		return
	}

	// 解析渠道级模型映射

	// Error passthrough binding
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	// 1. Acquire user concurrency slot
	maxWait := service.CalculateMaxWait(subject.Concurrency)
	canWait, err := h.concurrencyHelper.IncrementWaitCount(c.Request.Context(), subject.UserID, maxWait)
	waitCounted := false
	if err != nil {
		reqLog.Warn("gateway.responses.user_wait_counter_increment_failed", zap.Error(err))
	} else if !canWait {
		h.responsesErrorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later")
		return
	}
	if err == nil && canWait {
		waitCounted = true
	}
	defer func() {
		if waitCounted {
			h.concurrencyHelper.DecrementWaitCount(c.Request.Context(), subject.UserID)
		}
	}()

	userReleaseFunc, err := h.concurrencyHelper.AcquireUserSlotWithWait(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted)
	if err != nil {
		reqLog.Warn("gateway.responses.user_slot_acquire_failed", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", streamStarted)
		return
	}
	if waitCounted {
		h.concurrencyHelper.DecrementWaitCount(c.Request.Context(), subject.UserID)
		waitCounted = false
	}
	userReleaseFunc = wrapReleaseOnDone(c.Request.Context(), userReleaseFunc)
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	// Parse request for session hash
	bodyRef := service.NewRequestBodyRef(body)
	parsedReq, _ := service.ParseGatewayRequest(bodyRef, "responses")
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
		h.responsesErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available API key group routes")
		return
	}

routeLoop:
	for {
		routeCandidate, ok := routeCursor.current()
		if !ok {
			h.responsesErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available API key group routes")
			return
		}
		currentAPIKey := routeCandidate.APIKey
		routeCtx := gatewayRouteContext(c.Request.Context(), currentAPIKey, subject.UserID)
		currentSubscription, subErr := h.gatewayService.ResolveRouteSubscription(routeCtx, currentAPIKey, subscription)
		if subErr != nil {
			status, code, message, retryAfter := billingErrorDetails(subErr)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.responsesErrorResponse(c, status, code, message)
			return
		}
		channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(routeCtx, currentAPIKey.GroupID, reqModel)
		if currentAPIKey.Group != nil && currentAPIKey.Group.ClaudeCodeOnly {
			if routeCursor.skipToNext("responses_claude_code_only", reqLog, zap.Int64p("group_id", currentAPIKey.GroupID)) {
				continue routeLoop
			}
			h.responsesErrorResponse(c, http.StatusForbidden, "permission_error",
				"This group is restricted to Claude Code clients (/v1/messages only)")
			return
		}
		if err := h.billingCacheService.CheckBillingEligibility(routeCtx, currentAPIKey.User, currentAPIKey, currentAPIKey.Group, currentSubscription); err != nil {
			reqLog.Info("gateway.responses.billing_check_failed",
				zap.Error(err),
				zap.Int64p("group_id", currentAPIKey.GroupID),
			)
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.responsesErrorResponse(c, status, code, message)
			return
		}
		fs := NewFailoverState(h.maxAccountSwitches, false)

		for {
			selection, err := h.gatewayService.SelectAccountWithLoadAwareness(routeCtx, currentAPIKey.GroupID, sessionHash, reqModel, fs.FailedAccountIDs, "", int64(0))
			if err != nil {
				if len(fs.FailedAccountIDs) == 0 {
					if routeCursor.switchToNext(apiKey.ID, "account_select_failed", reqLog, zap.Error(err)) {
						continue routeLoop
					}
					cls := classifyNoAccountErrorFromGin(c, h.gatewayService, currentAPIKey, reqModel, requestedModel, openAICompatibleRequestPlatform(currentAPIKey))
					if cls.ModelNotFound {
						h.responsesErrorResponse(c, cls.Status, cls.ErrType, cls.Message)
						return
					}
					h.responsesErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available accounts: "+err.Error())
					return
				}
				action := fs.HandleSelectionExhausted(c.Request.Context())
				switch action {
				case FailoverContinue:
					continue
				case FailoverCanceled:
					return
				default:
					if fs.LastFailoverErr != nil {
						if !streamStarted && shouldSwitchAPIKeyGroupRoute(fs.LastFailoverErr) &&
							routeCursor.switchToNext(apiKey.ID, "account_selection_exhausted", reqLog, zap.Int("upstream_status", fs.LastFailoverErr.StatusCode)) {
							continue routeLoop
						}
						h.handleResponsesFailoverExhausted(c, fs.LastFailoverErr, streamStarted)
					} else {
						h.responsesErrorResponse(c, http.StatusBadGateway, "server_error", "All available accounts exhausted")
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
					h.responsesErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available accounts")
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
					reqLog.Warn("gateway.responses.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
					h.handleConcurrencyError(c, err, "account", streamStarted)
					return
				}
			}
			accountReleaseFunc = wrapReleaseOnDone(c.Request.Context(), accountReleaseFunc)

			// 5. Forward request
			writerSizeBeforeForward := c.Writer.Size()
			forwardBody := body
			if channelMapping.Mapped {
				forwardBody = h.gatewayService.ReplaceModelInBody(body, channelMapping.MappedModel)
			}
			forwardCtx := gatewayForwardContext(routeCtx, 0, h.metadataBridgeEnabled())
			result, err := h.gatewayService.ForwardAsResponses(forwardCtx, c, account, forwardBody, parsedReq)

			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}

			if err != nil {
				var failoverErr *service.UpstreamFailoverError
				if errors.As(err, &failoverErr) {
					// Can't failover if streaming content already sent
					if c.Writer.Size() != writerSizeBeforeForward {
						h.handleResponsesFailoverExhausted(c, failoverErr, true)
						return
					}
					action := fs.HandleFailoverError(c.Request.Context(), h.gatewayService, account.ID, account.Platform, failoverErr)
					switch action {
					case FailoverContinue:
						continue
					case FailoverExhausted:
						if canSwitchAPIKeyGroupRouteAfterForward(c, routeCursor, fs.LastFailoverErr, streamStarted, writerSizeBeforeForward) &&
							routeCursor.switchToNext(apiKey.ID, "upstream_failover_exhausted", reqLog, zap.Int("upstream_status", fs.LastFailoverErr.StatusCode)) {
							continue routeLoop
						}
						h.handleResponsesFailoverExhausted(c, fs.LastFailoverErr, streamStarted)
						return
					case FailoverCanceled:
						return
					}
				}
				h.ensureForwardErrorResponse(c, streamStarted)
				reqLog.Error("gateway.responses.forward_failed",
					zap.Int64("account_id", account.ID),
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

			h.submitUsageRecordTask(func(ctx context.Context) {
				if err := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
					Result:             result,
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
					ChannelUsageFields: service.BuildAutoModelUsageFields(autoDecision, channelMapping, result.UpstreamModel),
				}); err != nil {
					reqLog.Error("gateway.responses.record_usage_failed",
						zap.Int64("account_id", account.ID),
						zap.Error(err),
					)
				}
			})
			return
		}
	}
}

// responsesErrorResponse writes an error in OpenAI Responses API format.
func (h *GatewayHandler) responsesErrorResponse(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}

// handleResponsesFailoverExhausted writes a failover-exhausted error in Responses format.
func (h *GatewayHandler) handleResponsesFailoverExhausted(c *gin.Context, lastErr *service.UpstreamFailoverError, streamStarted bool) {
	if streamStarted {
		return // Can't write error after stream started
	}
	statusCode := http.StatusBadGateway
	if lastErr != nil && lastErr.StatusCode > 0 {
		statusCode = lastErr.StatusCode
	}
	h.responsesErrorResponse(c, statusCode, "server_error", "All available accounts exhausted")
}
