//go:build unit

// Phase-2 TASK-001 特征化测试：按"格式类"锁定内容审核拦截后的客户端可观测输出
// （钩子链改造 TASK-003 的实施前置 gate，调用点清单与格式类定义见
// issues/plugin-refactor/phase-2_pilots/CALLSITE-INVENTORY.md）。
//
// 覆盖（已有测试不重复）：
//   - B  格式 chat_completions（无顶层 type）          —— gateway_handler_chat_completions.go:98
//   - C  格式 responses（error.code 字符串字段）        —— gateway_handler_responses.go:107
//   - D  格式 gemini googleError（code int + status 串）—— gemini_v1beta_handler.go:190
//   - A' 格式 openai 网关 anthropic（顶层 type:"error"）—— openai_gateway_handler.go:676（格式化 :934）
//   - B' 格式 openai images（与 B 字节级同形，归并覆盖 openai_gateway_handler.go:244 / openai_chat_completions.go:88，
//     三点共用 openai_gateway_handler.go:1920 同一格式化函数）—— openai_images.go:89
//   - fail-open 非 anthropic 协议（chat_completions 审核服务 500 → 放行至账号调度阶段）
//   - WS turn-2 每消息审核 close-error 路径             —— openai_gateway_handler.go:1426
//
// 特征化纪律：所有断言均为先驱动真实链路跑出实际行为后固化，不写应然值。
// 复用 gateway_intercept_characterization_test.go 的 passChar* 夹具与
// openai_gateway_handler_test.go 的 WS 测试基建；新增辅助统一 p2Char 前缀。
package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ikik-api/internal/config"
	"ikik-api/internal/pkg/ctxkey"
	middleware "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// p2CharFlagAllModerationServer 返回对任意输入都判定命中的 mock 审核 API。
func p2CharFlagAllModerationServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"flagged":true,"category_scores":{"sexual":0.99}}]}`))
	}))
}

// p2CharNewContext 构造带认证上下文的网关请求（任意路径/分组/请求体）。
func p2CharNewContext(t *testing.T, path string, group *service.Group, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.Group, group))
	c.Request = req

	apiKey := &service.APIKey{
		ID:      7311,
		UserID:  7411,
		GroupID: &group.ID,
		Status:  service.StatusActive,
		User: &service.User{
			ID:          7411,
			Concurrency: 10,
			Balance:     100,
		},
		Group: group,
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
	return c, rec
}

// p2CharAnthropicGroup 返回 anthropic 平台分组（GatewayHandler 系端点用）。
func p2CharAnthropicGroup() *service.Group {
	return &service.Group{
		ID:       7001,
		Hydrated: true,
		Platform: service.PlatformAnthropic,
		Status:   service.StatusActive,
	}
}

// TestP2Characterization_ModerationBlock_ChatCompletionsFormat 锁定 B 格式：
// GatewayHandler.ChatCompletions（/v1/chat/completions）审核命中时返回 403 +
// {"error":{"type":"content_policy_violation","message":<拦截文案>}}——无顶层 type 字段。
func TestP2Characterization_ModerationBlock_ChatCompletionsFormat(t *testing.T) {
	moderationSrv := p2CharFlagAllModerationServer(t)
	defer moderationSrv.Close()

	group := p2CharAnthropicGroup()
	h, cleanup := newTestGatewayHandler(t, group, nil)
	defer cleanup()
	h.preFlightHooks = ProvideGatewayHookChain(passCharModerationService(t, moderationSrv.URL))

	body := []byte(`{"model":"claude-sonnet-4-5","stream":false,"messages":[{"role":"user","content":"bad words"}]}`)
	c, rec := p2CharNewContext(t, "/v1/chat/completions", group, body)
	h.ChatCompletions(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t,
		`{"error":{"type":"content_policy_violation","message":"内容审计命中风险规则，请调整输入后重试"}}`,
		rec.Body.String())
	// 关键差异点：chat_completions 格式没有顶层 type:"error"（区别于 anthropic 格式）。
	require.False(t, gjson.GetBytes(rec.Body.Bytes(), "type").Exists(),
		"chat_completions 拦截体不应有顶层 type 字段")
}

// TestP2Characterization_ModerationBlock_ResponsesFormat 锁定 C 格式：
// GatewayHandler.Responses（/v1/responses）审核命中时返回 403 +
// {"error":{"code":"content_policy_violation","message":<拦截文案>}}——
// error 内是 code 字段（字符串）而非 type 字段。
func TestP2Characterization_ModerationBlock_ResponsesFormat(t *testing.T) {
	moderationSrv := p2CharFlagAllModerationServer(t)
	defer moderationSrv.Close()

	group := p2CharAnthropicGroup()
	h, cleanup := newTestGatewayHandler(t, group, nil)
	defer cleanup()
	h.preFlightHooks = ProvideGatewayHookChain(passCharModerationService(t, moderationSrv.URL))

	body := []byte(`{"model":"claude-sonnet-4-5","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"bad words"}]}]}`)
	c, rec := p2CharNewContext(t, "/v1/responses", group, body)
	h.Responses(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t,
		`{"error":{"code":"content_policy_violation","message":"内容审计命中风险规则，请调整输入后重试"}}`,
		rec.Body.String())
	// 关键差异点：responses 格式用 error.code（字符串），无 error.type、无顶层 type。
	require.False(t, gjson.GetBytes(rec.Body.Bytes(), "type").Exists())
	require.False(t, gjson.GetBytes(rec.Body.Bytes(), "error.type").Exists(),
		"responses 拦截体 error 内不应有 type 字段（用 code）")
	require.Equal(t, gjson.String, gjson.GetBytes(rec.Body.Bytes(), "error.code").Type,
		"responses 拦截体 error.code 应为字符串")
}

// TestP2Characterization_ModerationBlock_GeminiGoogleErrorFormat 锁定 D 格式：
// GatewayHandler.GeminiV1BetaModels（/v1beta/models/{model}:{action}）审核命中时返回 403 +
// {"error":{"code":403,"message":<拦截文案>,"status":"PERMISSION_DENIED"}}。
// 注意：该调用点不消费 contentModerationErrorCode（content_policy_violation 被丢弃）。
func TestP2Characterization_ModerationBlock_GeminiGoogleErrorFormat(t *testing.T) {
	moderationSrv := p2CharFlagAllModerationServer(t)
	defer moderationSrv.Close()

	group := &service.Group{
		ID:       7002,
		Hydrated: true,
		Platform: service.PlatformGemini,
		Status:   service.StatusActive,
	}
	h, cleanup := newTestGatewayHandler(t, group, nil)
	defer cleanup()
	h.preFlightHooks = ProvideGatewayHookChain(passCharModerationService(t, moderationSrv.URL))

	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"bad words"}]}]}`)
	c, rec := p2CharNewContext(t, "/v1beta/models/gemini-2.5-pro:generateContent", group, body)
	c.Params = gin.Params{{Key: "modelAction", Value: "/gemini-2.5-pro:generateContent"}}
	h.GeminiV1BetaModels(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t,
		`{"error":{"code":403,"message":"内容审计命中风险规则，请调整输入后重试","status":"PERMISSION_DENIED"}}`,
		rec.Body.String())
	// 关键差异点：googleError 三字段——code 是 int HTTP 状态码、status 是 google 状态串；
	// 无顶层 type、无 error.type，审核错误码 content_policy_violation 不出现。
	parsed := gjson.ParseBytes(rec.Body.Bytes())
	require.False(t, parsed.Get("type").Exists(), "googleError 不应有顶层 type 字段")
	require.False(t, parsed.Get("error.type").Exists(), "googleError 的 error 内不应有 type 字段")
	require.Equal(t, gjson.Number, parsed.Get("error.code").Type, "googleError 的 error.code 应为数字")
	require.NotContains(t, rec.Body.String(), "content_policy_violation",
		"gemini 调用点不消费 contentModerationErrorCode")
}

// p2CharOpenAIGatewayHandler 构造满足 ensureResponsesDependencies 的最小 OpenAIGatewayHandler。
// 审核拦截发生在账号调度/转发之前，故各 service 用零值即可。
// HTTP 调用点走 preFlightHooks 链；contentModerationService 同时装配以镜像生产接线
// （WS 调用点仍直接消费该字段）。
func p2CharOpenAIGatewayHandler(t *testing.T, moderationBaseURL string) *OpenAIGatewayHandler {
	t.Helper()
	moderationSvc := passCharModerationService(t, moderationBaseURL)
	return &OpenAIGatewayHandler{
		gatewayService:           &service.OpenAIGatewayService{},
		billingCacheService:      &service.BillingCacheService{},
		apiKeyService:            &service.APIKeyService{},
		contentModerationService: moderationSvc,
		preFlightHooks:           ProvideGatewayHookChain(moderationSvc),
		concurrencyHelper:        NewConcurrencyHelper(service.NewConcurrencyService(&concurrencyCacheMock{}), SSEPingFormatNone, time.Second),
	}
}

// TestP2Characterization_ModerationBlock_OpenAIGatewayAnthropicFormat 锁定 A' 格式：
// OpenAIGatewayHandler.Messages（openai 平台分组的 /v1/messages dispatch）审核命中时返回 403 +
// {"type":"error","error":{"type":"content_policy_violation","message":<拦截文案>}}
// （openai_gateway_handler.go:934 anthropicErrorResponse 一族，与 anthropic 主格式字节级同形）。
func TestP2Characterization_ModerationBlock_OpenAIGatewayAnthropicFormat(t *testing.T) {
	moderationSrv := p2CharFlagAllModerationServer(t)
	defer moderationSrv.Close()

	group := &service.Group{
		ID:                    7003,
		Hydrated:              true,
		Platform:              service.PlatformOpenAI,
		Status:                service.StatusActive,
		AllowMessagesDispatch: true,
	}
	h := p2CharOpenAIGatewayHandler(t, moderationSrv.URL)

	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"bad words"}]}]}`)
	c, rec := p2CharNewContext(t, "/v1/messages", group, body)
	h.Messages(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t,
		`{"type":"error","error":{"type":"content_policy_violation","message":"内容审计命中风险规则，请调整输入后重试"}}`,
		rec.Body.String())
	// 关键差异点：openai 网关 anthropic 格式保留顶层 type:"error"。
	require.Equal(t, "error", gjson.GetBytes(rec.Body.Bytes(), "type").String())
}

// TestP2Characterization_ModerationBlock_OpenAIImagesFormat 锁定 B' 格式：
// OpenAIGatewayHandler.Images（/v1/images/generations，审核入参为 parsed.ModerationBody()）
// 审核命中时返回 403 + {"error":{"type":"content_policy_violation","message":<拦截文案>}}。
// 该格式与 B（chat_completions）字节级同形；openai 网关 /v1/responses（openai_gateway_handler.go:244）
// 与 /v1/chat/completions（openai_chat_completions.go:88）共用同一格式化函数
// （openai_gateway_handler.go:1920），由本测试归并覆盖（见 CALLSITE-INVENTORY.md）。
func TestP2Characterization_ModerationBlock_OpenAIImagesFormat(t *testing.T) {
	moderationSrv := p2CharFlagAllModerationServer(t)
	defer moderationSrv.Close()

	group := &service.Group{
		ID:                   7004,
		Hydrated:             true,
		Platform:             service.PlatformOpenAI,
		Status:               service.StatusActive,
		AllowImageGeneration: true,
	}
	h := p2CharOpenAIGatewayHandler(t, moderationSrv.URL)

	body := []byte(`{"model":"gpt-image-1","prompt":"bad words"}`)
	c, rec := p2CharNewContext(t, "/v1/images/generations", group, body)
	h.Images(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t,
		`{"error":{"type":"content_policy_violation","message":"内容审计命中风险规则，请调整输入后重试"}}`,
		rec.Body.String())
	require.False(t, gjson.GetBytes(rec.Body.Bytes(), "type").Exists(),
		"openai 网关通用错误格式不应有顶层 type 字段")
}

// TestP2Characterization_ModerationFailOpen_ChatCompletionsFormat 锁定非 anthropic 协议的
// fail-open 行为：chat_completions 链路上审核 API 自身故障（HTTP 500）时请求必须放行——
// 请求穿过审核 gate 继续进入账号调度阶段（夹具无可用账号，故观测到 503 No available accounts，
// 而不是 403 content_policy_violation）。
func TestP2Characterization_ModerationFailOpen_ChatCompletionsFormat(t *testing.T) {
	moderationSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "moderation backend exploded", http.StatusInternalServerError)
	}))
	defer moderationSrv.Close()

	group := p2CharAnthropicGroup()
	h, cleanup := newTestGatewayHandler(t, group, nil)
	defer cleanup()
	h.preFlightHooks = ProvideGatewayHookChain(passCharModerationService(t, moderationSrv.URL))

	body := []byte(`{"model":"claude-sonnet-4-5","stream":false,"messages":[{"role":"user","content":"bad words"}]}`)
	c, rec := p2CharNewContext(t, "/v1/chat/completions", group, body)
	h.ChatCompletions(c)

	// fail-open：未被审核拦截（非 403 + content_policy_violation），
	// 请求推进到账号调度阶段（夹具无账号 → 503 api_error）。
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.NotContains(t, rec.Body.String(), "content_policy_violation")
	require.Equal(t, "api_error", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
	require.Contains(t, gjson.GetBytes(rec.Body.Bytes(), "error.message").String(), "No available accounts")
}

// p2CharConditionalModerationServer 返回仅当审核请求中含 marker 才判定命中的 mock 审核 API。
func p2CharConditionalModerationServer(t *testing.T, marker string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(raw), marker) {
			_, _ = w.Write([]byte(`{"results":[{"flagged":true,"category_scores":{"sexual":0.99}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"flagged":false,"category_scores":{"sexual":0.0}}]}`))
	}))
}

// TestP2Characterization_OpenAIResponsesWSTurn2ModerationCloseError 锁定 E 格式 turn-2 路径
// （openai_gateway_handler.go:1426，OpenAIWSIngressHooks.BeforeRequest 的 turn≥2 每消息审核）：
// turn-1 干净消息正常转发并完成；turn-2 消息审核命中后客户端先收到
// writeContentModerationWSError 错误帧，随后连接以 close(1008 StatusPolicyViolation,
// reason=<拦截文案>) 关闭，且 turn-2 消息不会到达上游。
// 复用 TestOpenAIResponsesWebSocket_ContentModerationBlocksFirstFrame 的 WS 基建
// （newOpenAIWSHandlerTestServer 路由形态 + passthrough 上游夹具）。
func TestP2Characterization_OpenAIResponsesWSTurn2ModerationCloseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const turn2Marker = "p2char-turn2-bad"
	moderationSrv := p2CharConditionalModerationServer(t, turn2Marker)
	defer moderationSrv.Close()

	upstreamFrames := make(chan []byte, 4)
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		for {
			readCtx, cancelRead := context.WithTimeout(r.Context(), 5*time.Second)
			_, payload, readErr := conn.Read(readCtx)
			cancelRead()
			if readErr != nil {
				return
			}
			upstreamFrames <- payload
			writeCtx, cancelWrite := context.WithTimeout(r.Context(), 3*time.Second)
			writeErr := conn.Write(writeCtx, coderws.MessageText, []byte(
				`{"type":"response.completed","response":{"id":"resp_p2char_turn1","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`,
			))
			cancelWrite()
			if writeErr != nil {
				return
			}
		}
	}))
	defer upstreamServer.Close()

	account := service.Account{
		ID:          9911,
		Name:        "p2char-ws-turn2",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-p2char",
			"base_url": upstreamServer.URL,
		},
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
			"openai_apikey_responses_websockets_v2_mode":    service.OpenAIWSIngressModePassthrough,
		},
	}

	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	accountRepo := &openAIWSUsageHandlerAccountRepoStub{account: account}
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 4)}
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, nil, cfg)
	gatewaySvc := service.NewOpenAIGatewayService(
		accountRepo,                         // accountRepo
		nil,                                 // accountSharePolicyRepo
		usageRepo,                           // usageLogRepo
		nil,                                 // usageBillingRepo
		nil,                                 // userRepo
		nil,                                 // userSubRepo
		nil,                                 // userGroupRateRepo
		nil,                                 // cache (GatewayCache)
		cfg,                                 // cfg
		nil,                                 // schedulerSnapshot
		nil,                                 // concurrencyService
		service.NewBillingService(cfg, nil), // billingService
		nil,                                 // rateLimitService
		billingCacheSvc,                     // billingCacheService
		nil,                                 // httpUpstream
		&service.DeferredService{},          // deferredService
		nil,                                 // openAITokenProvider
		nil,                                 // resolver
		nil,                                 // channelService
		nil,                                 // balanceNotifyService
		nil,                                 // settingService
		nil,                                 // accountService
	)

	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
		acquireAccountSlotFn: func(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
			return true, nil
		},
	}
	h := &OpenAIGatewayHandler{
		gatewayService:           gatewaySvc,
		billingCacheService:      billingCacheSvc,
		apiKeyService:            &service.APIKeyService{},
		contentModerationService: passCharModerationService(t, moderationSrv.URL),
		concurrencyHelper:        NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}
	wsServer := newOpenAIWSHandlerTestServer(t, h, middleware.AuthSubject{UserID: 1, Concurrency: 1})
	defer wsServer.Close()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(
		dialCtx,
		"ws"+strings.TrimPrefix(wsServer.URL, "http")+"/openai/v1/responses",
		&coderws.DialOptions{CompressionMode: coderws.CompressionContextTakeover},
	)
	cancelDial()
	require.NoError(t, err)
	defer func() { _ = clientConn.CloseNow() }()

	// turn-1：干净消息正常转发，收到上游 response.completed。
	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
	err = clientConn.Write(writeCtx, coderws.MessageText, []byte(
		`{"type":"response.create","model":"gpt-5.1","stream":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"clean prompt"}]}]}`,
	))
	cancelWrite()
	require.NoError(t, err)

	readCtx, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
	_, turn1Event, err := clientConn.Read(readCtx)
	cancelRead()
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(turn1Event, "type").String())

	select {
	case <-upstreamFrames:
	case <-time.After(3 * time.Second):
		t.Fatal("等待上游收到 turn-1 帧超时")
	}

	// turn-2：命中审核的消息被 BeforeRequest 拦截。
	writeCtx, cancelWrite = context.WithTimeout(context.Background(), 3*time.Second)
	err = clientConn.Write(writeCtx, coderws.MessageText, []byte(
		`{"type":"response.create","model":"gpt-5.1","stream":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"`+turn2Marker+`"}]}]}`,
	))
	cancelWrite()
	require.NoError(t, err)

	// 实际行为：先收到 writeContentModerationWSError 错误帧，再收到 close(1008) 关闭。
	readCtx, cancelRead = context.WithTimeout(context.Background(), 5*time.Second)
	_, errFrame, readErr := clientConn.Read(readCtx)
	cancelRead()
	require.NoError(t, readErr, "turn-2 拦截后应先收到错误帧")
	require.JSONEq(t,
		`{"event_id":"evt_content_moderation_blocked","type":"error","error":{"type":"invalid_request_error","code":"content_policy_violation","message":"内容审计命中风险规则，请调整输入后重试"}}`,
		string(errFrame))

	readCtx, cancelRead = context.WithTimeout(context.Background(), 5*time.Second)
	_, _, closeReadErr := clientConn.Read(readCtx)
	cancelRead()
	require.Error(t, closeReadErr)
	var closeErr coderws.CloseError
	require.ErrorAs(t, closeReadErr, &closeErr)
	require.Equal(t, coderws.StatusPolicyViolation, closeErr.Code)
	require.Equal(t, "内容审计命中风险规则，请调整输入后重试", closeErr.Reason)

	// turn-2 消息不得到达上游。
	select {
	case frame := <-upstreamFrames:
		t.Fatalf("turn-2 被拦截的消息不应转发到上游，实际收到: %s", frame)
	case <-time.After(300 * time.Millisecond):
	}
}

// openAIWSUsageHandlerAccountRepoStub 仅实现 WS 调度路径需要的最小账号查询。
// 其余方法经内嵌 nil AccountRepository 接口在意外调用时 panic（特征化测试纪律：
// 不掩饰未预期的 service 调用，直接 fail）。
type openAIWSUsageHandlerAccountRepoStub struct {
	service.AccountRepository
	account service.Account
}

func (r *openAIWSUsageHandlerAccountRepoStub) GetByID(_ context.Context, _ int64) (*service.Account, error) {
	return &r.account, nil
}

func (r *openAIWSUsageHandlerAccountRepoStub) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	if r.account.Platform != "" && r.account.Platform != platform {
		return nil, nil
	}
	return []service.Account{r.account}, nil
}

func (r *openAIWSUsageHandlerAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]service.Account, error) {
	if r.account.Platform != "" && r.account.Platform != platform {
		return nil, nil
	}
	return []service.Account{r.account}, nil
}

// openAIWSUsageHandlerUsageLogRepoStub 仅实现 Create 将日志发入 channel 供测试
// 断言；其余方法经内嵌 nil UsageLogRepository 接口 panic。
type openAIWSUsageHandlerUsageLogRepoStub struct {
	service.UsageLogRepository
	created chan *service.UsageLog
}

func (r *openAIWSUsageHandlerUsageLogRepoStub) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	r.created <- log
	return true, nil
}
