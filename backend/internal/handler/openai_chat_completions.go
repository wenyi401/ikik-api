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
	"ikik-api/internal/pkg/logger"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

// ChatCompletions handles OpenAI Chat Completions API requests.
// POST /v1/chat/completions
func (h *OpenAIGatewayHandler) ChatCompletions(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.openai_gateway.chat_completions",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	if !gjson.ValidBytes(body) {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()
	reqStream := gjson.GetBytes(body, "stream").Bool()
	requestedModel := reqModel
	autoDecision := h.gatewayService.ResolveAutoModel(c.Request.Context(), apiKey.GroupID, reqModel, body, service.AutoModelProtocolOpenAIChat)
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

	if decision := h.runPreFlightHooks(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIChat, reqModel, body); decision != nil && decision.Blocked {
		h.errorResponse(c, preFlightStatus(decision), preFlightErrorCode(decision), decision.Message)
		return
	}

	// 解析渠道级模型映射
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	sessionHash := h.gatewayService.GenerateSessionHash(c, body)
	promptCacheKey := h.gatewayService.ExtractSessionID(c, body)
	routeCursor := newAPIKeyGroupRouteCursor(apiKey)
	if _, ok := routeCursor.current(); !ok {
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available API key group routes", streamStarted)
		return
	}

	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *service.UpstreamFailoverError

	for {
		routeCandidate, ok := routeCursor.current()
		if !ok {
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available API key group routes", streamStarted)
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
			h.handleStreamingAwareError(c, status, code, message, streamStarted)
			return
		}
		channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(routeCtx, currentAPIKey.GroupID, reqModel)
		if err := h.billingCacheService.CheckBillingEligibility(routeCtx, currentAPIKey.User, currentAPIKey, currentAPIKey.Group, currentSubscription); err != nil {
			reqLog.Info("openai_chat_completions.billing_eligibility_check_failed",
				zap.Error(err),
				zap.Int64p("group_id", currentAPIKey.GroupID),
			)
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.handleStreamingAwareError(c, status, code, message, streamStarted)
			return
		}

		c.Set("openai_chat_completions_fallback_model", "")
		requestPlatform := openAICompatibleRequestPlatform(currentAPIKey)
		reqLog.Debug("openai_chat_completions.account_selecting",
			zap.Int("excluded_account_count", len(failedAccountIDs)),
			zap.Int64p("group_id", currentAPIKey.GroupID),
		)
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
			routeCtx,
			currentAPIKey.GroupID,
			"",
			sessionHash,
			reqModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportAny,
			service.OpenAIEndpointCapabilityChatCompletions,
			false,
			requestPlatform,
		)
		if err != nil {
			reqLog.Warn("openai_chat_completions.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
				zap.Int64p("group_id", currentAPIKey.GroupID),
			)
			if len(failedAccountIDs) == 0 {
				defaultModel := ""
				if currentAPIKey.Group != nil {
					defaultModel = currentAPIKey.Group.DefaultMappedModel
				}
				if defaultModel != "" && defaultModel != reqModel {
					reqLog.Info("openai_chat_completions.fallback_to_default_model",
						zap.String("default_mapped_model", defaultModel),
					)
					selection, scheduleDecision, err = h.gatewayService.SelectAccountWithSchedulerForCapability(
						routeCtx,
						currentAPIKey.GroupID,
						"",
						sessionHash,
						defaultModel,
						failedAccountIDs,
						service.OpenAIUpstreamTransportAny,
						service.OpenAIEndpointCapabilityChatCompletions,
						false,
						requestPlatform,
					)
					if err == nil && selection != nil {
						c.Set("openai_chat_completions_fallback_model", defaultModel)
					}
				}
				if err != nil {
					if routeCursor.switchToNext(apiKey.ID, "account_select_failed", reqLog, zap.Error(err)) {
						failedAccountIDs = make(map[int64]struct{})
						sameAccountRetryCount = make(map[int64]int)
						switchCount = 0
						lastFailoverErr = nil
						continue
					}
					h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable", streamStarted)
					return
				}
			} else {
				if lastFailoverErr != nil {
					if !streamStarted && shouldSwitchAPIKeyGroupRoute(lastFailoverErr) &&
						routeCursor.switchToNext(apiKey.ID, "account_selection_exhausted", reqLog, zap.Int("upstream_status", lastFailoverErr.StatusCode)) {
						failedAccountIDs = make(map[int64]struct{})
						sameAccountRetryCount = make(map[int64]int)
						switchCount = 0
						lastFailoverErr = nil
						continue
					}
					h.handleFailoverExhausted(c, lastFailoverErr, streamStarted)
				} else {
					h.handleStreamingAwareError(c, http.StatusBadGateway, "api_error", "Upstream request failed", streamStarted)
				}
				return
			}
		}
		if selection == nil || selection.Account == nil {
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", streamStarted)
			return
		}
		account := selection.Account
		sessionHash = ensureOpenAIPoolModeSessionHash(sessionHash, account)
		reqLog.Debug("openai_chat_completions.account_selected",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name),
			zap.Int64p("group_id", currentAPIKey.GroupID),
		)
		_ = scheduleDecision
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountReleaseFunc, acquired := h.acquireResponsesAccountSlot(c, currentAPIKey.GroupID, sessionHash, selection, reqStream, &streamStarted, reqLog)
		if !acquired {
			return
		}

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		writerSizeBeforeForward := c.Writer.Size()

		defaultMappedModel := resolveOpenAIForwardDefaultMappedModel(currentAPIKey, c.GetString("openai_chat_completions_fallback_model"))
		forwardBody := body
		if channelMapping.Mapped {
			forwardBody = h.gatewayService.ReplaceModelInBody(body, channelMapping.MappedModel)
		}
		forwardCtx := gatewayForwardContext(routeCtx, 0, h.metadataBridgeEnabled())
		result, err := h.gatewayService.ForwardAsChatCompletions(forwardCtx, c, account, forwardBody, promptCacheKey, defaultMappedModel)

		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)
		if err == nil && result != nil && result.FirstTokenMs != nil {
			service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, int64(*result.FirstTokenMs))
		}
		if err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
				// Pool mode: retry on the same account
				if failoverErr.RetryableOnSameAccount {
					retryLimit := account.GetPoolModeRetryCount()
					if sameAccountRetryCount[account.ID] < retryLimit {
						sameAccountRetryCount[account.ID]++
						reqLog.Warn("openai_chat_completions.pool_mode_same_account_retry",
							zap.Int64("account_id", account.ID),
							zap.Int("upstream_status", failoverErr.StatusCode),
							zap.Int("retry_limit", retryLimit),
							zap.Int("retry_count", sameAccountRetryCount[account.ID]),
						)
						select {
						case <-c.Request.Context().Done():
							return
						case <-time.After(sameAccountRetryDelay):
						}
						continue
					}
				}
				h.gatewayService.RecordOpenAIAccountSwitch()
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= maxAccountSwitches {
					if canSwitchAPIKeyGroupRouteAfterForward(c, routeCursor, failoverErr, streamStarted, writerSizeBeforeForward) &&
						routeCursor.switchToNext(apiKey.ID, "upstream_failover_exhausted", reqLog, zap.Int("upstream_status", failoverErr.StatusCode)) {
						failedAccountIDs = make(map[int64]struct{})
						sameAccountRetryCount = make(map[int64]int)
						switchCount = 0
						lastFailoverErr = nil
						continue
					}
					h.handleFailoverExhausted(c, failoverErr, streamStarted)
					return
				}
				switchCount++
				reqLog.Warn("openai_chat_completions.upstream_failover_switching",
					zap.Int64("account_id", account.ID),
					zap.Int("upstream_status", failoverErr.StatusCode),
					zap.Int("switch_count", switchCount),
					zap.Int("max_switches", maxAccountSwitches),
				)
				continue
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
			wroteFallback := h.ensureForwardErrorResponse(c, streamStarted)
			reqLog.Warn("openai_chat_completions.forward_failed",
				zap.Int64("account_id", account.ID),
				zap.Bool("fallback_error_response_written", wroteFallback),
				zap.Error(err),
			)
			return
		}
		if result != nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
		} else {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, nil)
		}
		routeCursor.recordSuccess(apiKey.ID)

		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)

		h.submitUsageRecordTask(func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
				Result:             result,
				APIKey:             currentAPIKey,
				User:               currentAPIKey.User,
				Account:            account,
				Subscription:       currentSubscription,
				InboundEndpoint:    GetInboundEndpoint(c),
				UpstreamEndpoint:   GetUpstreamEndpoint(c, account.Platform),
				UserAgent:          userAgent,
				IPAddress:          clientIP,
				APIKeyService:      h.apiKeyService,
				ChannelUsageFields: service.BuildAutoModelUsageFields(autoDecision, channelMapping, result.UpstreamModel),
			}); err != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.chat_completions"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", currentAPIKey.ID),
					zap.Any("group_id", currentAPIKey.GroupID),
					zap.String("model", reqModel),
					zap.Int64("account_id", account.ID),
				).Error("openai_chat_completions.record_usage_failed", zap.Error(err))
			}
		})
		reqLog.Debug("openai_chat_completions.request_completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount),
		)
		return
	}
}
