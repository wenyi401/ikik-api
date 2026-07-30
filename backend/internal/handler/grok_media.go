package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	pkghttputil "ikik-api/internal/pkg/httputil"
	"ikik-api/internal/pkg/ip"
	"ikik-api/internal/pkg/logger"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

// GrokImages handles xAI image generation/editing through Grok groups.
func (h *OpenAIGatewayHandler) GrokImages(c *gin.Context) {
	endpoint := service.GrokMediaEndpointImagesGenerations
	if strings.Contains(c.Request.URL.Path, "/images/edits") {
		endpoint = service.GrokMediaEndpointImagesEdits
	}
	h.handleGrokMedia(c, endpoint, "")
}

// GrokVideoGeneration handles xAI video generation through Grok groups.
func (h *OpenAIGatewayHandler) GrokVideoGeneration(c *gin.Context) {
	h.handleGrokMedia(c, service.GrokMediaEndpointVideosGenerations, "")
}

// GrokVideoEdit handles asynchronous xAI video edits through Grok groups.
func (h *OpenAIGatewayHandler) GrokVideoEdit(c *gin.Context) {
	h.handleGrokMedia(c, service.GrokMediaEndpointVideosEdits, "")
}

// GrokVideoExtension handles asynchronous xAI video extensions through Grok groups.
func (h *OpenAIGatewayHandler) GrokVideoExtension(c *gin.Context) {
	h.handleGrokMedia(c, service.GrokMediaEndpointVideosExtensions, "")
}

// GrokVideoStatus handles xAI video status retrieval through Grok groups.
func (h *OpenAIGatewayHandler) GrokVideoStatus(c *gin.Context) {
	h.handleGrokMedia(c, service.GrokMediaEndpointVideoStatus, c.Param("request_id"))
}

// GrokVideoContent proxies downloadable video content through the task's upstream account.
func (h *OpenAIGatewayHandler) GrokVideoContent(c *gin.Context) {
	h.handleGrokMedia(c, service.GrokMediaEndpointVideoContent, c.Param("request_id"))
}

func (h *OpenAIGatewayHandler) handleGrokMedia(c *gin.Context, endpoint service.GrokMediaEndpoint, requestID string) {
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
		"handler.openai_gateway.grok_media",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.String("endpoint", string(endpoint)),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	var body []byte
	var err error
	if endpoint.RequiresRequestBody() {
		body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
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
	}

	contentType := c.GetHeader("Content-Type")
	requestInfo := service.ParseGrokMediaRequest(contentType, body)
	requestModel := requestInfo.Model
	routingModel := service.NormalizeGrokMediaModelForEndpoint(endpoint, requestModel, requestInfo.HasInputImage())
	if endpoint.IsGenerationRequest() && strings.TrimSpace(requestModel) == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	if endpoint.IsVideoLookupRequest() && strings.TrimSpace(requestID) == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "request_id is required")
		return
	}

	reqLog = reqLog.With(zap.String("model", requestModel))
	setOpsRequestContext(c, requestModel, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
	imageRouteRequest := isGrokImageRouteRequest(endpoint)
	currentAPIKey := apiKey
	var routeCursor *apiKeyGroupRouteCursor
	if imageRouteRequest {
		routeCursor = newAPIKeyGroupRouteCursor(apiKey)
		routeCandidate, permissionDenied, routeOK := currentGrokImageRoute(routeCursor, reqLog)
		if !routeOK {
			h.grokImageRouteUnavailable(c, permissionDenied)
			return
		}
		currentAPIKey = routeCandidate.APIKey
	}

	if endpoint.IsGenerationRequest() {
		if !imageRouteRequest && !service.GroupAllowsImageGeneration(apiKey.Group) {
			h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
			return
		}
		if moderationBody := requestInfo.ModerationBody(); len(moderationBody) > 0 {
			preFlightDecision := h.runPreFlightHooks(c, reqLog, currentAPIKey, subject, service.ContentModerationProtocolOpenAIImages, requestModel, moderationBody)
			auditDecision := h.checkSecurityAudit(c, reqLog, currentAPIKey, subject, service.ContentModerationProtocolOpenAIImages, requestModel, moderationBody)
			if preFlightDecision != nil && preFlightDecision.Blocked {
				h.errorResponse(c, preFlightStatus(preFlightDecision), preFlightErrorCode(preFlightDecision), preFlightDecision.Message)
				return
			}
			if auditDecision != nil && !auditDecision.AllowNextStage {
				h.openAISecurityAuditError(c, auditDecision)
				return
			}
		}
		imageReleaseFunc, acquired := h.acquireImageGenerationSlot(c, streamStarted)
		if !acquired {
			return
		}
		if imageReleaseFunc != nil {
			defer imageReleaseFunc()
		}
	}

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if !imageRouteRequest {
		if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
			reqLog.Info("grok_media.billing_eligibility_check_failed", zap.Error(err))
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.errorResponse(c, status, code, message)
			return
		}
	}

	sessionSeed := body
	if len(sessionSeed) == 0 && strings.TrimSpace(requestID) != "" {
		sessionSeed = []byte(requestID)
	}
	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, sessionSeed)
	boundLookupAccountID := int64(0)
	if endpoint.IsVideoLookupRequest() {
		sessionHash = service.GrokMediaVideoRequestSessionHash(requestID, subject.UserID, apiKey.ID)
		boundLookupAccountID, err = h.gatewayService.ResolveGrokMediaVideoRequestAccount(
			c.Request.Context(), apiKey.GroupID, requestID, subject.UserID, apiKey.ID,
		)
		if err != nil || boundLookupAccountID <= 0 {
			reqLog.Info("grok_media.video_lookup_owner_binding_missing", zap.Error(err))
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
			return
		}
	}
	baseRequestCtx := c.Request.Context()
	requestCtx := baseRequestCtx
	currentSubscription := subscription
	activeRouteGroupID := int64(0)
	userRPMCounted := false
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *service.UpstreamFailoverError
	var oauth429FailoverState service.OpenAIOAuth429FailoverState
	mediaEligibilityRejected := false
	switchCount := 0
	maxAccountSwitches := h.maxAccountSwitches
	if maxAccountSwitches <= 0 {
		maxAccountSwitches = 3
	}
	routingStart := time.Now()
	requiredCapability := grokMediaRequiredCapability(endpoint)
	resetRouteState := func() {
		failedAccountIDs = make(map[int64]struct{})
		sameAccountRetryCount = make(map[int64]int)
		lastFailoverErr = nil
		oauth429FailoverState = service.OpenAIOAuth429FailoverState{}
		mediaEligibilityRejected = false
		switchCount = 0
		activeRouteGroupID = 0
	}
	switchImageRoute := func(reason string, fields ...zap.Field) bool {
		if !imageRouteRequest || routeCursor == nil || !routeCursor.switchToNext(apiKey.ID, reason, reqLog, fields...) {
			return false
		}
		resetRouteState()
		return true
	}

	for {
		if failoverClientGone(c) {
			return
		}
		if imageRouteRequest {
			routeCandidate, permissionDenied, routeOK := currentGrokImageRoute(routeCursor, reqLog)
			if !routeOK {
				h.grokImageRouteUnavailable(c, permissionDenied)
				return
			}
			currentAPIKey = routeCandidate.APIKey
			if currentAPIKey.GroupID == nil || *currentAPIKey.GroupID <= 0 {
				h.grokImageRouteUnavailable(c, false)
				return
			}
			if activeRouteGroupID != *currentAPIKey.GroupID {
				requestCtx = gatewayRouteContext(baseRequestCtx, currentAPIKey, subject.UserID)
				if userRPMCounted {
					requestCtx = service.WithUserRPMAlreadyCounted(requestCtx)
				}
				currentSubscription, err = h.gatewayService.ResolveRouteSubscription(requestCtx, currentAPIKey, subscription)
				if err != nil {
					status, code, message, retryAfter := billingErrorDetails(err)
					if retryAfter > 0 {
						c.Header("Retry-After", strconv.Itoa(retryAfter))
					}
					h.errorResponse(c, status, code, message)
					return
				}
				if err := h.billingCacheService.CheckBillingEligibility(requestCtx, currentAPIKey.User, currentAPIKey, currentAPIKey.Group, currentSubscription, service.QuotaPlatform(requestCtx, currentAPIKey)); err != nil {
					reqLog.Info("grok_media.billing_eligibility_check_failed", zap.Error(err), zap.Int64p("group_id", currentAPIKey.GroupID))
					status, code, message, retryAfter := billingErrorDetails(err)
					if retryAfter > 0 {
						c.Header("Retry-After", strconv.Itoa(retryAfter))
					}
					h.errorResponse(c, status, code, message)
					return
				}
				userRPMCounted = true
				activeRouteGroupID = *currentAPIKey.GroupID
			}
		}
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
			requestCtx,
			currentAPIKey.GroupID,
			"",
			sessionHash,
			routingModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportHTTPSSE,
			requiredCapability,
			false,
			false,
			false,
			service.PlatformGrok,
		)
		if err != nil {
			if failoverClientGone(c) {
				reqLog.Info("grok_media.account_select_aborted_client_disconnected", zap.Error(err))
				return
			}
			reqLog.Warn("grok_media.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
				zap.Int64p("group_id", currentAPIKey.GroupID),
			)
			if len(failedAccountIDs) == 0 && switchImageRoute("account_select_failed", zap.Error(err)) {
				continue
			}
			if endpoint.IsGenerationRequest() && errors.Is(err, service.ErrNoAvailableAccounts) &&
				(len(failedAccountIDs) == 0 || (mediaEligibilityRejected && lastFailoverErr == nil)) {
				if switchImageRoute("media_eligibility_exhausted", zap.Error(err)) {
					continue
				}
				markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
				h.errorResponse(c, http.StatusServiceUnavailable, "grok_media_no_eligible_account", "No eligible Grok media accounts")
				return
			}
			if len(failedAccountIDs) == 0 {
				cls := classifyNoAccountErrorFromGin(c, h.gatewayService, currentAPIKey, requestModel, routingModel, service.PlatformGrok)
				if !cls.ModelNotFound {
					markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
				}
				h.errorResponse(c, cls.Status, cls.ErrType, cls.Message)
				return
			}
			if lastFailoverErr != nil {
				if shouldSwitchAPIKeyGroupRoute(lastFailoverErr) && switchImageRoute("account_selection_exhausted", zap.Int("upstream_status", lastFailoverErr.StatusCode)) {
					continue
				}
				h.handleFailoverExhausted(c, lastFailoverErr, false)
			} else {
				h.errorResponse(c, http.StatusBadGateway, "api_error", "Upstream request failed")
			}
			return
		}
		if selection == nil || selection.Account == nil {
			if switchImageRoute("account_selection_empty") {
				continue
			}
			if endpoint.IsGenerationRequest() {
				markOpsRoutingCapacityLimited(c)
				h.errorResponse(c, http.StatusServiceUnavailable, "grok_media_no_eligible_account", "No eligible Grok media accounts")
				return
			}
			cls := classifyNoAccountErrorFromGin(c, h.gatewayService, currentAPIKey, requestModel, routingModel, service.PlatformGrok)
			if !cls.ModelNotFound {
				markOpsRoutingCapacityLimited(c)
			}
			h.errorResponse(c, cls.Status, cls.ErrType, cls.Message)
			return
		}
		if boundLookupAccountID > 0 && selection.Account.ID != boundLookupAccountID {
			reqLog.Warn("grok_media.video_lookup_bound_account_unavailable",
				zap.Int64("bound_account_id", boundLookupAccountID),
				zap.Int64("selected_account_id", selection.Account.ID),
			)
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
			return
		}

		reqLog.Debug("grok_media.account_schedule_decision",
			zap.String("layer", scheduleDecision.Layer),
			zap.Bool("sticky_session_hit", scheduleDecision.StickySessionHit),
			zap.Int("candidate_count", scheduleDecision.CandidateCount),
			zap.Int("top_k", scheduleDecision.TopK),
			zap.Int64("latency_ms", scheduleDecision.LatencyMs),
			zap.Float64("load_skew", scheduleDecision.LoadSkew),
		)

		account := selection.Account
		if endpoint.IsGenerationRequest() {
			eligible, eligibilityReason, eligibilityErr := h.ensureGrokMediaAccountEligibility(requestCtx, account)
			if !eligible {
				mediaEligibilityRejected = true
				failedAccountIDs[account.ID] = struct{}{}
				reqLog.Warn("grok_media.account_eligibility_rejected",
					zap.Int64("account_id", account.ID),
					zap.String("reason", eligibilityReason),
					zap.Bool("probe_failed", eligibilityErr != nil),
				)
				if switchCount >= maxAccountSwitches {
					if switchImageRoute("media_eligibility_switch_limit") {
						continue
					}
					markOpsRoutingCapacityLimited(c)
					h.errorResponse(c, http.StatusServiceUnavailable, "grok_media_no_eligible_account", "No eligible Grok media accounts")
					return
				}
				switchCount++
				continue
			}
		}
		sessionHash = ensureOpenAIPoolModeSessionHash(sessionHash, account)
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountReleaseFunc, accountAcquired := h.acquireResponsesAccountSlot(c, currentAPIKey.GroupID, sessionHash, selection, false, &streamStarted, reqLog)
		if !accountAcquired {
			return
		}

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		writerSizeBeforeForward := c.Writer.Size()
		result, err := func() (*service.OpenAIForwardResult, error) {
			defer func() {
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
			}()
			return h.gatewayService.ForwardGrokMedia(requestCtx, c, account, endpoint, requestID, body, contentType)
		}()

		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)

		if err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				if failoverClientGone(c) {
					reqLog.Info("grok_media.failover_aborted_client_disconnected",
						zap.Int64("account_id", account.ID),
						zap.Int("upstream_status", failoverErr.StatusCode),
					)
					return
				}
				if failoverErr.ShouldReportAccountScheduleFailure() {
					h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, grokMediaScheduleModel(account, routingModel, nil), false, nil)
				}
				if c.Writer.Size() != writerSizeBeforeForward {
					h.handleFailoverExhausted(c, failoverErr, true)
					return
				}
				if !failoverErr.ShouldRetryNextAccount() {
					if canSwitchAPIKeyGroupRouteAfterForward(c, routeCursor, failoverErr, streamStarted, writerSizeBeforeForward) &&
						switchImageRoute("upstream_route_retry", zap.Int("upstream_status", failoverErr.StatusCode)) {
						continue
					}
					h.handleFailoverExhausted(c, failoverErr, false)
					return
				}
				if endpoint.IsVideoLookupRequest() {
					h.handleFailoverExhausted(c, failoverErr, false)
					return
				}
				if failoverErr.RetryableOnSameAccount {
					retryLimit := account.GetPoolModeRetryCount()
					if sameAccountRetryCount[account.ID] < retryLimit {
						sameAccountRetryCount[account.ID]++
						reqLog.Warn("grok_media.pool_mode_same_account_retry",
							zap.Int64("account_id", account.ID),
							zap.Int("upstream_status", failoverErr.StatusCode),
							zap.Int("retry_limit", retryLimit),
							zap.Int("retry_count", sameAccountRetryCount[account.ID]),
						)
						select {
						case <-requestCtx.Done():
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
						switchImageRoute("upstream_failover_exhausted", zap.Int("upstream_status", failoverErr.StatusCode)) {
						continue
					}
					h.handleFailoverExhausted(c, failoverErr, false)
					return
				}
				switchCount++
				if h.gatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, switchCount, &oauth429FailoverState) {
					if canSwitchAPIKeyGroupRouteAfterForward(c, routeCursor, failoverErr, streamStarted, writerSizeBeforeForward) &&
						switchImageRoute("oauth_429_failover_exhausted", zap.Int("upstream_status", failoverErr.StatusCode)) {
						continue
					}
					h.handleFailoverExhausted(c, failoverErr, false)
					return
				}
				reqLog.Warn("grok_media.upstream_failover_switching",
					zap.Int64("account_id", account.ID),
					zap.Int("upstream_status", failoverErr.StatusCode),
					zap.Int("switch_count", switchCount),
					zap.Int("max_switches", maxAccountSwitches),
				)
				continue
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, grokMediaScheduleModel(account, routingModel, nil), false, nil)
			if !service.IsResponseCommitted(c) && c.Writer.Size() == writerSizeBeforeForward {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
			}
			reqLog.Warn("grok_media.forward_failed",
				zap.Int64("account_id", account.ID),
				zap.Error(err),
			)
			return
		}

		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, grokMediaScheduleModel(account, routingModel, result), true, nil)
		if imageRouteRequest {
			routeCursor.recordSuccess(apiKey.ID)
		}
		if endpoint.IsGenerationRequest() && strings.TrimSpace(result.ResponseID) != "" {
			if err := h.gatewayService.BindGrokMediaVideoRequestAccount(
				requestCtx, currentAPIKey.GroupID, result.ResponseID, subject.UserID, apiKey.ID, account.ID,
			); err != nil {
				reqLog.Warn("grok_media.bind_video_request_account_failed",
					zap.Int64("account_id", account.ID),
					zap.String("request_id", result.ResponseID),
					zap.Error(err),
				)
			}
		}
		if shouldRecordGrokMediaUsage(endpoint, requestModel) {
			recordGrokMediaUsage(requestCtx, c, h, reqLog, currentAPIKey, subject, currentSubscription, account, result, requestModel, body, requestID)
		}
		reqLog.Debug("grok_media.request_completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount),
		)
		return
	}
}

func isGrokImageRouteRequest(endpoint service.GrokMediaEndpoint) bool {
	return endpoint == service.GrokMediaEndpointImagesGenerations || endpoint == service.GrokMediaEndpointImagesEdits
}

func currentGrokImageRoute(cursor *apiKeyGroupRouteCursor, reqLog *zap.Logger) (apiKeyGroupRouteCandidate, bool, bool) {
	permissionDenied := false
	for {
		candidate, ok := cursor.current()
		if !ok {
			return apiKeyGroupRouteCandidate{}, permissionDenied, false
		}
		if service.PlatformFromAPIKey(candidate.APIKey) != service.PlatformGrok {
			if cursor.skipToNext("grok_image_platform_mismatch", reqLog, zap.Int64("group_id", candidate.Route.GroupID)) {
				continue
			}
			return apiKeyGroupRouteCandidate{}, permissionDenied, false
		}
		if !service.GroupAllowsImageGeneration(candidate.APIKey.Group) {
			permissionDenied = true
			if cursor.skipToNext("image_generation_not_allowed", reqLog, zap.Int64("group_id", candidate.Route.GroupID)) {
				continue
			}
			return apiKeyGroupRouteCandidate{}, true, false
		}
		return candidate, permissionDenied, true
	}
}

func (h *OpenAIGatewayHandler) grokImageRouteUnavailable(c *gin.Context, permissionDenied bool) {
	if permissionDenied {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	h.errorResponse(c, http.StatusServiceUnavailable, "grok_media_no_eligible_route", "No eligible Grok image routes")
}

func (h *OpenAIGatewayHandler) ensureGrokMediaAccountEligibility(ctx context.Context, account *service.Account) (bool, string, error) {
	if account == nil {
		return false, "missing_account", errors.New("grok media account is required")
	}
	eligible, reason := account.GrokMediaGenerationEligibility()
	if eligible || reason != "billing_unobserved" {
		return eligible, reason, nil
	}
	if h == nil || h.grokMediaEligibilityProber == nil {
		return false, "billing_probe_unavailable", errors.New("grok media eligibility probe is not configured")
	}
	return h.grokMediaEligibilityProber.ProbeMediaEligibility(ctx, account.ID)
}

func grokMediaRequiredCapability(endpoint service.GrokMediaEndpoint) service.OpenAIEndpointCapability {
	if endpoint.IsGenerationRequest() {
		return service.OpenAIEndpointCapabilityGrokMediaGeneration
	}
	return ""
}

func grokMediaScheduleModel(account *service.Account, routingModel string, result *service.OpenAIForwardResult) string {
	if result != nil && strings.TrimSpace(result.UpstreamModel) != "" {
		return result.UpstreamModel
	}
	if account == nil {
		return strings.TrimSpace(routingModel)
	}
	return account.GetMappedModel(routingModel)
}

func shouldRecordGrokMediaUsage(endpoint service.GrokMediaEndpoint, requestModel string) bool {
	return endpoint.IsGenerationRequest() && strings.TrimSpace(requestModel) != ""
}

func recordGrokMediaUsage(
	requestCtx context.Context,
	c *gin.Context,
	h *OpenAIGatewayHandler,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	subscription *service.UserSubscription,
	account *service.Account,
	result *service.OpenAIForwardResult,
	requestModel string,
	body []byte,
	requestID string,
) {
	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	payloadForHash := body
	if len(payloadForHash) == 0 && strings.TrimSpace(requestID) != "" {
		payloadForHash = []byte(requestID)
	}
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
	quotaPlatform := service.QuotaPlatform(requestCtx, apiKey)
	channelUsageFields := service.ChannelUsageFields{
		OriginalModel:      clientRequestedModel(c, requestModel),
		ChannelMappedModel: requestModel,
	}
	h.submitOpenAIUsageRecordTask(requestCtx, result, func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            account,
			Subscription:       subscription,
			InboundEndpoint:    inboundEndpoint,
			UpstreamEndpoint:   upstreamEndpoint,
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			RequestPayloadHash: service.HashUsageRequestPayload(payloadForHash),
			APIKeyService:      h.apiKeyService,
			QuotaPlatform:      quotaPlatform,
			ChannelUsageFields: channelUsageFields,
		}); err != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.grok_media"),
				zap.Int64("user_id", subject.UserID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.Any("group_id", apiKey.GroupID),
				zap.String("model", requestModel),
				zap.Int64("account_id", account.ID),
			).Error("grok_media.record_usage_failed", zap.Error(err))
			reqLog.Debug("grok_media.record_usage_failed", zap.Error(err))
		}
	})
}
