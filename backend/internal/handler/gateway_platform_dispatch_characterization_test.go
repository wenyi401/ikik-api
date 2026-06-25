//go:build unit

// Phase-3 TASK-001 前置特征化测试：/v1/messages 两处平台 Forward 分发点
// （gateway_handler.go :444 与 :794）的路由与参数透传锁定，作为 TASK-002
// Provider 接缝改造（registry 替换分发点）的等价性 gate。
//
// 固化内容（对应 SEAM-DESIGN.md 裁决记录的 T1-T4）：
//   - T1 :794 路由矩阵：antigravity 平台分组下，
//     {OAuth/Upstream（Type != APIKey）账号 → AntigravityGatewayService.Forward，
//     APIKey 账号 → GatewayService.Forward}；通过两个独立的上游记录桩区分命中路径；
//   - T2 :444 路由：gemini 平台分组 + antigravity 账号 → ForwardGemini，
//     参数透传可观测面：model（v1internal 包裹体 model 字段=映射后模型）、
//     上游 action 恒为 streamGenerateContent、stream（客户端响应形态
//     SSE/收集 JSON）、body 原文进入包裹体；
//   - T3 session 选项透传：WithForwardGeminiSession(groupID, "gemini:"+sessionHash)
//     在模型限流切换路径上经 clearStickySession 落到
//     GatewayCache.DeleteSessionAccountID(groupID, sessionKey)，参数逐一断言；
//     粘性绑定账号被优先选中（sessionBoundAccountID → 调度命中）；
//   - T4 错误链：GatewayService.Forward 返回 *BetaBlockedError → handler errors.As
//     命中 → 400 + invalid_request_error + 策略消息；AntigravityGatewayService.Forward
//     返回 *PromptTooLongError（无兜底分组）→ errors.As 命中 → WriteMappedClaudeError
//     输出。改造后 adapter 若包裹/吞掉这两类错误，本组测试必红。
//
// mock 方式取舍说明：GatewayHandler 的 service 字段均为具体类型
// （*service.GatewayService / *service.AntigravityGatewayService），无法以接口桩
// 替换记录调用参数；故采用真实 service + 注入 HTTPUpstream/GatewayCache 记录桩，
// 通过上游收到的请求特征（URL/包裹体）与缓存调用参数区分路径并断言透传。
// isStickySession 实参本身在 handler 层无外部可观测面（其效果 ForceCacheBilling
// 已由 service 层既有测试 TestAntigravityGatewayService_Forward_StickySessionForceCacheBilling /
// _ForwardGemini_StickySessionForceCacheBilling 锁定）；handler 层锁定其数据来源
// hasBoundSession 的判定链：粘性绑定命中 → 绑定账号被优先调度（见 T3 用例）。
//
// 复用同包既有夹具：fakeSchedulerCache / fakeGroupRepo
// （gateway_handler_warmup_intercept_unit_test.go）、schedInvNewCountingCache
// （scheduling_invariants_failover_test.go）、passCharSettingRepo
// （gateway_intercept_characterization_test.go）。
// 本文件新增的包级辅助类型/函数一律带 p3Char 前缀。
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ikik-api/internal/config"
	"ikik-api/internal/pkg/ctxkey"
	"ikik-api/internal/pkg/tlsfingerprint"
	middleware "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// p3Char 夹具
// ---------------------------------------------------------------------------

// p3CharUpstreamCall 记录一次上游调用的可观测特征。
type p3CharUpstreamCall struct {
	AccountID int64
	Method    string
	URL       string
	Header    http.Header
	Body      []byte
}

// p3CharUpstream 是可编程的 HTTPUpstream 记录桩：按 accountID 返回脚本化响应，
// 并记录每次调用的完整请求特征（URL/Header/Body）。
type p3CharUpstream struct {
	mu      sync.Mutex
	respond func(accountID int64) (*http.Response, error)
	calls   []p3CharUpstreamCall
}

var _ service.HTTPUpstream = (*p3CharUpstream)(nil)

func (u *p3CharUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	call := p3CharUpstreamCall{AccountID: accountID}
	if req != nil {
		call.Method = req.Method
		call.URL = req.URL.String()
		call.Header = req.Header.Clone()
		if req.Body != nil {
			body, _ := io.ReadAll(req.Body)
			_ = req.Body.Close()
			call.Body = body
		}
	}
	u.mu.Lock()
	u.calls = append(u.calls, call)
	respond := u.respond
	u.mu.Unlock()

	if respond == nil {
		return nil, fmt.Errorf("p3CharUpstream: unexpected upstream call (account=%d url=%s)", accountID, call.URL)
	}
	return respond(accountID)
}

func (u *p3CharUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (u *p3CharUpstream) recordedCalls() []p3CharUpstreamCall {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]p3CharUpstreamCall(nil), u.calls...)
}

// p3CharSessionCall 记录一次粘性会话缓存调用的参数。
type p3CharSessionCall struct {
	GroupID     int64
	SessionHash string
	AccountID   int64
}

// p3CharStickyCache 是 service.GatewayCache 的内存实现，记录所有调用参数。
// 同一实例同时注入 GatewayService（粘性查询/绑定）与 AntigravityGatewayService
// （模型限流时清除粘性绑定），与生产共用同一 Redis 缓存的接线一致。
type p3CharStickyCache struct {
	mu          sync.Mutex
	bound       map[string]int64
	getCalls    []p3CharSessionCall
	setCalls    []p3CharSessionCall
	deleteCalls []p3CharSessionCall
}

var _ service.GatewayCache = (*p3CharStickyCache)(nil)

func p3CharNewStickyCache() *p3CharStickyCache {
	return &p3CharStickyCache{bound: make(map[string]int64)}
}

func p3CharSessionKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%d|%s", groupID, sessionHash)
}

func (c *p3CharStickyCache) bind(groupID int64, sessionHash string, accountID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bound[p3CharSessionKey(groupID, sessionHash)] = accountID
}

func (c *p3CharStickyCache) GetSessionAccountID(_ context.Context, groupID int64, sessionHash string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getCalls = append(c.getCalls, p3CharSessionCall{GroupID: groupID, SessionHash: sessionHash})
	return c.bound[p3CharSessionKey(groupID, sessionHash)], nil
}

func (c *p3CharStickyCache) SetSessionAccountID(_ context.Context, groupID int64, sessionHash string, accountID int64, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setCalls = append(c.setCalls, p3CharSessionCall{GroupID: groupID, SessionHash: sessionHash, AccountID: accountID})
	c.bound[p3CharSessionKey(groupID, sessionHash)] = accountID
	return nil
}

func (c *p3CharStickyCache) RefreshSessionTTL(_ context.Context, _ int64, _ string, _ time.Duration) error {
	return nil
}

func (c *p3CharStickyCache) DeleteSessionAccountID(_ context.Context, groupID int64, sessionHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteCalls = append(c.deleteCalls, p3CharSessionCall{GroupID: groupID, SessionHash: sessionHash})
	delete(c.bound, p3CharSessionKey(groupID, sessionHash))
	return nil
}

func (c *p3CharStickyCache) GetSessionString(_ context.Context, _ int64, _ string) (string, error) {
	return "", nil
}

func (c *p3CharStickyCache) SetSessionString(_ context.Context, _ int64, _ string, _ string, _ time.Duration) error {
	return nil
}

func (c *p3CharStickyCache) DeleteSessionString(_ context.Context, _ int64, _ string) error {
	return nil
}

func (c *p3CharStickyCache) recordedDeleteCalls() []p3CharSessionCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]p3CharSessionCall(nil), c.deleteCalls...)
}

func (c *p3CharStickyCache) recordedGetCalls() []p3CharSessionCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]p3CharSessionCall(nil), c.getCalls...)
}

// p3CharGroup 构造指定平台的测试分组。
func p3CharGroup(groupID int64, platform string) *service.Group {
	return &service.Group{
		ID:       groupID,
		Hydrated: true,
		Platform: platform,
		Status:   service.StatusActive,
	}
}

// p3CharAntigravityAccount 构造 antigravity 账号夹具。
func p3CharAntigravityAccount(id, groupID int64, accountType string, creds, extra map[string]any) *service.Account {
	credentials := map[string]any{"access_token": fmt.Sprintf("ag-token-%d", id)}
	for k, v := range creds {
		credentials[k] = v
	}
	return &service.Account{
		ID:            id,
		Name:          fmt.Sprintf("p3-char-ag-%d", id),
		Platform:      service.PlatformAntigravity,
		Type:          accountType,
		Credentials:   credentials,
		Extra:         extra,
		Concurrency:   5,
		Priority:      1,
		Status:        service.StatusActive,
		Schedulable:   true,
		AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: groupID}},
	}
}

// p3CharHandlerEnv 聚合一套完整 GatewayHandler 测试环境。
type p3CharHandlerEnv struct {
	handler     *GatewayHandler
	gwUpstream  *p3CharUpstream // GatewayService.Forward 路径的上游
	agUpstream  *p3CharUpstream // AntigravityGatewayService.Forward/ForwardGemini 路径的上游
	stickyCache *p3CharStickyCache
	cleanup     func()
}

// p3CharNewHandler 构造完整 GatewayHandler：真实 GatewayService 与
// AntigravityGatewayService，各自注入独立上游记录桩；共享粘性会话缓存桩。
// settingValues 写入两个 service 共享的 SettingService 内存仓库（beta policy 等）。
func p3CharNewHandler(t *testing.T, group *service.Group, accounts []*service.Account, settingValues map[string]string) *p3CharHandlerEnv {
	t.Helper()
	t.Setenv("SUB2API_DEBUG_GATEWAY_BODY", "")

	gwUpstream := &p3CharUpstream{}
	agUpstream := &p3CharUpstream{}
	stickyCache := p3CharNewStickyCache()

	schedulerSnapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil)
	concurrencySvc := service.NewConcurrencyService(schedInvNewCountingCache())
	settingSvc := service.NewSettingService(&passCharSettingRepo{values: settingValues}, &config.Config{})

	gwSvc := service.NewGatewayService(
		nil, // accountRepo
		nil, // accountSharePolicyRepo
		&fakeGroupRepo{group: group},
		nil, nil, nil, nil, nil, // usageLogRepo / usageBillingRepo / userRepo / userSubRepo / userGroupRateRepo
		stickyCache,
		&config.Config{},
		schedulerSnapshot,
		concurrencySvc,
		nil,                         // billingService
		&service.RateLimitService{}, // 零值：错误码仅记录日志
		nil,                         // billingCacheService
		nil,                         // identityService
		gwUpstream,
		&service.DeferredService{},
		nil, nil, nil, nil, // claudeTokenProvider / sessionLimitCache / rpmCache / digestStore
		settingSvc,
		nil, nil, nil, nil, // tlsFPProfileService / channelService / resolver / balanceNotifyService
	)

	agSvc := service.NewAntigravityGatewayService(
		nil, // accountRepo（模型限流写库不在锁定面，nil → 仅走 handleError 兜底分支）
		stickyCache,
		schedulerSnapshot,
		&service.AntigravityTokenProvider{},
		nil, // rateLimitService
		agUpstream,
		settingSvc,
		nil, // internal500Cache
	)

	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil,
		&config.Config{RunMode: config.RunModeSimple}, nil)

	h := NewGatewayHandler(
		gwSvc,
		nil, // geminiCompatService（:444 非 antigravity 分支误入会 panic → 用例显式失败）
		agSvc,
		ProvideGatewayPlatformRegistry(gwSvc, agSvc), // 与生产 Wire 装配同构的平台分发注册表
		nil, // userService
		concurrencySvc,
		billingCacheSvc,
		nil, nil, nil, nil, nil, nil, // usageService / apiKeyService / workerPool / errorPassthrough / preFlightHooks / userMsgQueue
		nil, // cfg
		nil, // settingService（handler 级版本检查等不在锁定面）
		nil, // carpoolService
	)
	return &p3CharHandlerEnv{
		handler:     h,
		gwUpstream:  gwUpstream,
		agUpstream:  agUpstream,
		stickyCache: stickyCache,
		cleanup:     func() { billingCacheSvc.Stop() },
	}
}

// p3CharMessagesContext 构造带认证上下文的 /v1/messages 请求（可附加 header）。
func p3CharMessagesContext(t *testing.T, group *service.Group, body []byte, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder, context.CancelFunc) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	ctx := context.WithValue(req.Context(), ctxkey.Group, group)
	ctx, cancel := context.WithCancel(ctx)
	c.Request = req.WithContext(ctx)

	apiKey := &service.APIKey{
		ID:      8501,
		UserID:  8601,
		GroupID: &group.ID,
		Status:  service.StatusActive,
		User: &service.User{
			ID:          8601,
			Concurrency: 10,
			Balance:     100,
		},
		Group: group,
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
	return c, rec, cancel
}

// p3CharSessionID 是 metadata.user_id 中的 session 段（36 字符 UUID），
// 用于产生确定性的 sessionHash（GenerateSessionHash 最高优先级来源）。
const p3CharSessionID = "12345678-1234-1234-1234-123456789abc"

// p3CharMetadataUserID 构造 legacy 格式 metadata.user_id。
func p3CharMetadataUserID() string {
	return "user_" + strings.Repeat("a", 64) + "_account__session_" + p3CharSessionID
}

// p3CharClaudeBody 构造 Claude 协议请求体（:794 路径）。
func p3CharClaudeBody(stream bool) []byte {
	return []byte(fmt.Sprintf(`{
		"model": "claude-sonnet-4-5",
		"max_tokens": 64,
		"stream": %t,
		"metadata": {"user_id": %q},
		"messages": [{"role":"user","content":"p3 dispatch probe"}]
	}`, stream, p3CharMetadataUserID()))
}

// p3CharGeminiLoopBody 构造同时满足 /v1/messages 解析（model/stream/metadata）
// 与 ForwardGemini 原文透传（contents）的混合请求体（:444 路径将原始 body
// 原样传给 ForwardGemini）。
func p3CharGeminiLoopBody(stream bool) []byte {
	return []byte(fmt.Sprintf(`{
		"model": "gemini-2.5-flash",
		"stream": %t,
		"metadata": {"user_id": %q},
		"contents": [{"role":"user","parts":[{"text":"p3 gemini dispatch probe"}]}]
	}`, stream, p3CharMetadataUserID()))
}

// p3CharAntigravitySSEResponse 构造 v1internal 包裹的上游流式成功响应。
func p3CharAntigravitySSEResponse() *http.Response {
	sse := "data: {\"response\":{\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"ok\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":8,\"candidatesTokenCount\":3}}}\n\n"
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"X-Request-Id": []string{"rid-p3-ag"},
		},
		Body: io.NopCloser(strings.NewReader(sse)),
	}
}

// p3CharRateLimit429Response 构造触发"模型限流 + 切换账号"的 429 响应
// （RATE_LIMIT_EXCEEDED + retryDelay 60s ≥ 7s 阈值，无智能重试等待）。
func p3CharRateLimit429Response() *http.Response {
	body := `{
		"error": {
			"code": 429,
			"message": "rate limited",
			"status": "RESOURCE_EXHAUSTED",
			"details": [
				{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "RATE_LIMIT_EXCEEDED", "metadata": {"model": "gemini-3-pro-high"}},
				{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "60s"}
			]
		}
	}`
	return &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// ---------------------------------------------------------------------------
// T1 :794 路由矩阵
// ---------------------------------------------------------------------------

// TestPlatformDispatchCharacterization_AntigravityNonAPIKeyRoutesToAntigravityForward
// 固化 :794 分支条件 `Platform == Antigravity && Type != APIKey` 的"命中"侧：
// OAuth 与 Upstream 两种非 APIKey 账号均走 AntigravityGatewayService.Forward
// （上游收到 antigravity v1internal 请求 / 上游透传请求），GatewayService 上游零调用。
func TestPlatformDispatchCharacterization_AntigravityNonAPIKeyRoutesToAntigravityForward(t *testing.T) {
	t.Run("OAuth账号走antigravity转发", func(t *testing.T) {
		group := p3CharGroup(9501, service.PlatformAntigravity)
		account := p3CharAntigravityAccount(9511, group.ID, service.AccountTypeOAuth, map[string]any{
			"model_mapping": map[string]any{"claude-sonnet-4-5": "gemini-3-pro-high"},
		}, nil)

		env := p3CharNewHandler(t, group, []*service.Account{account}, nil)
		defer env.cleanup()
		env.agUpstream.respond = func(int64) (*http.Response, error) {
			return p3CharAntigravitySSEResponse(), nil
		}

		c, rec, cancel := p3CharMessagesContext(t, group, p3CharClaudeBody(false), nil)
		defer cancel()

		env.handler.Messages(c)

		agCalls := env.agUpstream.recordedCalls()
		require.Len(t, agCalls, 1, "antigravity 上游应被调用恰好一次")
		require.Empty(t, env.gwUpstream.recordedCalls(), "GatewayService 上游不应被调用")

		// Claude→Gemini 转换 + v1internal 包裹：URL 恒为 streamGenerateContent，
		// 包裹体 model = 账号映射后的最终模型
		require.Contains(t, agCalls[0].URL, "/v1internal:streamGenerateContent?alt=sse")
		require.Equal(t, "gemini-3-pro-high", gjson.GetBytes(agCalls[0].Body, "model").String())
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Upstream账号走antigravity转发", func(t *testing.T) {
		group := p3CharGroup(9502, service.PlatformAntigravity)
		account := p3CharAntigravityAccount(9512, group.ID, service.AccountTypeUpstream, map[string]any{
			"api_key":       "ag-upstream-key",
			"base_url":      "https://ag-upstream.example.com",
			"model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"},
		}, nil)

		env := p3CharNewHandler(t, group, []*service.Account{account}, nil)
		defer env.cleanup()
		env.agUpstream.respond = func(int64) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"id":"msg_p3_up","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":5,"output_tokens":2}}`)),
			}, nil
		}

		c, rec, cancel := p3CharMessagesContext(t, group, p3CharClaudeBody(false), nil)
		defer cancel()

		env.handler.Messages(c)

		agCalls := env.agUpstream.recordedCalls()
		require.Len(t, agCalls, 1, "antigravity 上游应被调用恰好一次（ForwardUpstream 透传）")
		require.Empty(t, env.gwUpstream.recordedCalls(), "GatewayService 上游不应被调用")
		require.Equal(t, "https://ag-upstream.example.com/v1/messages", agCalls[0].URL)
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

// TestPlatformDispatchCharacterization_AntigravityAPIKeyRoutesToGatewayForward
// 固化 :794 分支条件的"未命中"侧：antigravity 平台的 APIKey 账号走
// GatewayService.Forward（Claude 协议直连上游），antigravity 上游零调用。
func TestPlatformDispatchCharacterization_AntigravityAPIKeyRoutesToGatewayForward(t *testing.T) {
	group := p3CharGroup(9503, service.PlatformAntigravity)
	account := p3CharAntigravityAccount(9513, group.ID, service.AccountTypeAPIKey, map[string]any{
		"api_key":       "ag-apikey-cred",
		"model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"},
	}, nil)

	env := p3CharNewHandler(t, group, []*service.Account{account}, nil)
	defer env.cleanup()
	env.gwUpstream.respond = func(int64) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"id":"msg_p3_gw","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":5,"output_tokens":2}}`)),
		}, nil
	}

	c, rec, cancel := p3CharMessagesContext(t, group, p3CharClaudeBody(false), nil)
	defer cancel()

	env.handler.Messages(c)

	gwCalls := env.gwUpstream.recordedCalls()
	require.Len(t, gwCalls, 1, "GatewayService 上游应被调用恰好一次")
	require.Empty(t, env.agUpstream.recordedCalls(), "antigravity 上游不应被调用")

	// APIKey 账号未配置 base_url 时直连 Anthropic 默认地址，认证走 x-api-key
	// （setHeaderRaw 以小写原样写入 header map，须用原始 key 读取）
	require.Equal(t, "https://api.anthropic.com/v1/messages?beta=true", gwCalls[0].URL)
	require.Equal(t, []string{"ag-apikey-cred"}, gwCalls[0].Header["x-api-key"])
	require.Equal(t, http.StatusOK, rec.Code)
}

// ---------------------------------------------------------------------------
// T2 :444 路由 + 参数透传
// ---------------------------------------------------------------------------

// TestPlatformDispatchCharacterization_GeminiLoopAntigravityRoutesToForwardGemini
// 固化 :444 分支：gemini 平台分组下 antigravity 账号 → ForwardGemini。
// 参数可观测面逐一断言：
//   - model：v1internal 包裹体 model = 账号映射后的模型（请求体 model 经映射进入上游）；
//   - 上游 action 恒为 streamGenerateContent（handler 传入的 action="generateContent"
//     仅决定客户端响应形态合法性，上游不变量见 service 层 action 契约测试）；
//   - stream=false → 网关收集上游流式响应，客户端收到单个 JSON（application/json）；
//   - stream=true → SSE 解包透传（text/event-stream）；
//   - body 原文（contents 文本）进入包裹体 request 字段；
//   - sessionKey 格式 "gemini:"+sessionHash 进入粘性查询。
func TestPlatformDispatchCharacterization_GeminiLoopAntigravityRoutesToForwardGemini(t *testing.T) {
	newEnv := func(t *testing.T, groupID int64) (*p3CharHandlerEnv, *service.Group) {
		group := p3CharGroup(groupID, service.PlatformGemini)
		account := p3CharAntigravityAccount(groupID+10, groupID, service.AccountTypeOAuth, map[string]any{
			"model_mapping": map[string]any{"gemini-2.5-flash": "gemini-3-pro-high"},
		}, map[string]any{"mixed_scheduling": true})
		env := p3CharNewHandler(t, group, []*service.Account{account}, nil)
		env.agUpstream.respond = func(int64) (*http.Response, error) {
			return p3CharAntigravitySSEResponse(), nil
		}
		return env, group
	}

	t.Run("非流式收集为单个JSON", func(t *testing.T) {
		env, group := newEnv(t, 9521)
		defer env.cleanup()

		c, rec, cancel := p3CharMessagesContext(t, group, p3CharGeminiLoopBody(false), nil)
		defer cancel()

		env.handler.Messages(c)

		agCalls := env.agUpstream.recordedCalls()
		require.Len(t, agCalls, 1, "antigravity 上游应被调用恰好一次")
		require.Empty(t, env.gwUpstream.recordedCalls())

		// model 参数：包裹体 model = 映射后模型
		require.Contains(t, agCalls[0].URL, "/v1internal:streamGenerateContent?alt=sse")
		require.Equal(t, "gemini-3-pro-high", gjson.GetBytes(agCalls[0].Body, "model").String())
		// body 透传：原始 contents 文本进入包裹体 request 字段
		require.Contains(t, gjson.GetBytes(agCalls[0].Body, "request").Raw, "p3 gemini dispatch probe")

		// stream=false：收集上游流式响应为单个 JSON
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		require.Equal(t, "ok", gjson.Get(rec.Body.String(), "candidates.0.content.parts.0.text").String())

		// sessionKey 格式：gemini 平台为 "gemini:"+sessionHash
		getCalls := env.stickyCache.recordedGetCalls()
		require.NotEmpty(t, getCalls, "粘性会话查询应发生")
		for _, call := range getCalls {
			require.Equal(t, group.ID, call.GroupID)
			require.Equal(t, "gemini:"+p3CharSessionID, call.SessionHash)
		}
	})

	t.Run("流式SSE解包透传", func(t *testing.T) {
		env, group := newEnv(t, 9522)
		defer env.cleanup()

		c, rec, cancel := p3CharMessagesContext(t, group, p3CharGeminiLoopBody(true), nil)
		defer cancel()

		env.handler.Messages(c)

		agCalls := env.agUpstream.recordedCalls()
		require.Len(t, agCalls, 1)
		require.Contains(t, agCalls[0].URL, "/v1internal:streamGenerateContent?alt=sse")

		// stream=true：SSE 解包透传
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
		require.Contains(t, rec.Body.String(), `data: {"candidates"`)
	})
}

// ---------------------------------------------------------------------------
// T3 session 选项透传 + 粘性绑定语义
// ---------------------------------------------------------------------------

// TestPlatformDispatchCharacterization_GeminiLoopSessionOptionsReachStickyClear
// 固化 WithForwardGeminiSession(groupID, sessionKey) 的端到端透传：
//  1. 粘性绑定账号被优先调度（首次上游调用命中绑定账号）；
//  2. 绑定账号 429（RATE_LIMIT_EXCEEDED + 长 retryDelay）触发模型限流切换时，
//     ForwardGemini 内部以 handler 传入的 (groupID, sessionKey) 调用
//     GatewayCache.DeleteSessionAccountID 清除粘性绑定——参数逐一断言；
//  3. failover 切换第二账号成功，客户端最终 200。
//
// 此场景同时锁定 isStickySession=hasBoundSession 的端到端透传：绑定命中 →
// ForwardGemini(isSticky=true) → 切换信号携带 IsStickySession → handler
// fs.ForceCacheBilling → 第二账号成功后 RecordUsage 应用 force_cache_billing
// （经结构化日志 sink 断言；isSticky 在 service 内的语义另由 service 层既有
// 测试 TestAntigravityGatewayService_ForwardGemini_StickySessionForceCacheBilling 锁定）。
func TestPlatformDispatchCharacterization_GeminiLoopSessionOptionsReachStickyClear(t *testing.T) {
	sink, restoreSink := captureHandlerStructuredLog(t)
	defer restoreSink()

	group := p3CharGroup(9531, service.PlatformGemini)
	boundAccount := p3CharAntigravityAccount(9541, group.ID, service.AccountTypeOAuth, map[string]any{
		"model_mapping": map[string]any{"gemini-2.5-flash": "gemini-3-pro-high"},
	}, map[string]any{"mixed_scheduling": true})
	healthyAccount := p3CharAntigravityAccount(9542, group.ID, service.AccountTypeOAuth, map[string]any{
		"model_mapping": map[string]any{"gemini-2.5-flash": "gemini-3-pro-high"},
	}, map[string]any{"mixed_scheduling": true})

	env := p3CharNewHandler(t, group, []*service.Account{boundAccount, healthyAccount}, nil)
	defer env.cleanup()

	sessionKey := "gemini:" + p3CharSessionID
	env.stickyCache.bind(group.ID, sessionKey, boundAccount.ID)

	env.agUpstream.respond = func(accountID int64) (*http.Response, error) {
		if accountID == boundAccount.ID {
			return p3CharRateLimit429Response(), nil
		}
		return p3CharAntigravitySSEResponse(), nil
	}

	c, rec, cancel := p3CharMessagesContext(t, group, p3CharGeminiLoopBody(false), nil)
	defer cancel()

	env.handler.Messages(c)

	// 1. 粘性绑定账号被优先调度，429 后切换到第二账号
	agCalls := env.agUpstream.recordedCalls()
	require.Len(t, agCalls, 2, "绑定账号 1 次 + failover 后第二账号 1 次")
	require.Equal(t, boundAccount.ID, agCalls[0].AccountID, "首次尝试必须命中粘性绑定账号")
	require.Equal(t, healthyAccount.ID, agCalls[1].AccountID, "failover 切换到第二账号")

	// 2. WithForwardGeminiSession 的 (groupID, sessionKey) 透传到粘性清除
	deleteCalls := env.stickyCache.recordedDeleteCalls()
	require.NotEmpty(t, deleteCalls, "模型限流切换必须清除粘性绑定")
	for _, call := range deleteCalls {
		require.Equal(t, group.ID, call.GroupID, "清除粘性绑定的 groupID 必须等于 handler 传入的分组 ID")
		require.Equal(t, sessionKey, call.SessionHash, "清除粘性绑定的 sessionHash 必须等于 handler 传入的 sessionKey")
	}

	// 3. failover 成功，客户端 200
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, env.gwUpstream.recordedCalls())

	// 4. isStickySession 透传链：hasBoundSession=true → ForwardGemini 切换信号
	//    IsStickySession=true → fs.ForceCacheBilling → RecordUsage 应用
	//    force_cache_billing（input_tokens 转 cache_read）。
	require.True(t,
		sink.ContainsMessageAtLevel("force_cache_billing", "info"),
		"粘性会话切换后第二账号成功，RecordUsage 必须应用 force_cache_billing")
}

// ---------------------------------------------------------------------------
// T4 错误链现状锁定
// ---------------------------------------------------------------------------

// TestErrorChainCharacterization_BetaBlockedErrorReturns400 固化：
// GatewayService.Forward 返回 *service.BetaBlockedError 时，handler 的
// errors.As 命中 → 400 + invalid_request_error + 策略消息原文；不触发 failover、
// 不触上游。改造后 adapter 若包裹该错误且破坏 errors.As 链，本用例必红。
func TestErrorChainCharacterization_BetaBlockedErrorReturns400(t *testing.T) {
	group := p3CharGroup(9551, service.PlatformAnthropic)
	// anthropic APIKey 账号（未启用透传）→ GatewayService.Forward 主路径会评估 beta policy
	account := &service.Account{
		ID:            9561,
		Name:          "p3-char-anthropic-apikey",
		Platform:      service.PlatformAnthropic,
		Type:          service.AccountTypeAPIKey,
		Credentials:   map[string]any{"api_key": "sk-p3-beta"},
		Concurrency:   5,
		Priority:      1,
		Status:        service.StatusActive,
		Schedulable:   true,
		AccountGroups: []service.AccountGroup{{AccountID: 9561, GroupID: group.ID}},
	}

	betaPolicy := `{"rules":[{"beta_token":"p3-blocked-beta","action":"block","scope":"all","error_message":"beta blocked by p3 policy"}]}`
	env := p3CharNewHandler(t, group, []*service.Account{account}, map[string]string{
		"beta_policy_settings": betaPolicy,
	})
	defer env.cleanup()

	c, rec, cancel := p3CharMessagesContext(t, group, p3CharClaudeBody(false), map[string]string{
		"anthropic-beta": "p3-blocked-beta",
	})
	defer cancel()

	env.handler.Messages(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t,
		`{"type":"error","error":{"type":"invalid_request_error","message":"beta blocked by p3 policy"}}`,
		rec.Body.String())
	require.Empty(t, env.gwUpstream.recordedCalls(), "beta policy 拦截发生在触上游之前")
	require.Empty(t, env.agUpstream.recordedCalls())
}

// TestErrorChainCharacterization_PromptTooLongWritesMappedClaudeError 固化：
// AntigravityGatewayService.Forward 返回 *service.PromptTooLongError 且无兜底
// 分组时，handler 的 errors.As 命中 → WriteMappedClaudeError 写回上游状态码 +
// Claude 风格错误体；不触发 failover（单次上游调用）。
func TestErrorChainCharacterization_PromptTooLongWritesMappedClaudeError(t *testing.T) {
	group := p3CharGroup(9552, service.PlatformAntigravity)
	account := p3CharAntigravityAccount(9562, group.ID, service.AccountTypeOAuth, map[string]any{
		"model_mapping": map[string]any{"claude-sonnet-4-5": "gemini-3-pro-high"},
	}, nil)

	env := p3CharNewHandler(t, group, []*service.Account{account}, nil)
	defer env.cleanup()
	env.agUpstream.respond = func(int64) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"rid-p3-ptl"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Prompt is too long"}}`)),
		}, nil
	}

	c, rec, cancel := p3CharMessagesContext(t, group, p3CharClaudeBody(false), nil)
	defer cancel()

	env.handler.Messages(c)

	require.Len(t, env.agUpstream.recordedCalls(), 1, "prompt too long 不触发 failover")
	require.Equal(t, http.StatusBadRequest, rec.Code, "WriteMappedClaudeError 透传上游状态码")

	var errBody map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody))
	require.Equal(t, "error", errBody["type"])
	errObj, ok := errBody["error"].(map[string]any)
	require.True(t, ok, "错误体必须是 Claude 风格 {type, error:{...}}")
	require.Contains(t, fmt.Sprintf("%v", errObj["message"]), "Prompt is too long")
}
