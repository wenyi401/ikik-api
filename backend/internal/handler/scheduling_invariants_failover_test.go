//go:build unit

// Phase-0 TASK-004 failover 不变量测试（INVARIANTS I-5.1 / I-5.2 / I-5.3、I-6.1 部分）。
//
// 固化内容：
//   - I-5.1 三平台换号上限默认值（anthropic=10 / gemini=3 / openai=3，双断言：
//     构造函数默认值 + 硬编码当前值）；anthropic 平台通过完整 Messages 入口驱动
//     真实 failover 循环：连续上游 500 时恰好尝试 maxAccountSwitches+1 个账号后
//     返回 502 + 映射错误体；
//   - I-5.2 失败账号进入排除集合不被重选（同一账号不会被尝试两次）；
//   - I-5.3 可重试错误（池模式）先同账号重试 maxSameAccountRetries 次再换号（完整链）；
//   - I-6.1 转发失败路径下账号/用户槽位获取-释放严格配平（配平计数器）；
//   - chat-completions 兼容路径耗尽错误体（"All available accounts exhausted"）。
//
// 复用同包既有夹具：fakeSchedulerCache / fakeGroupRepo
// （gateway_handler_warmup_intercept_unit_test.go）、mockTempUnscheduler /
// newTestFailoverErr（failover_loop_test.go）。
// 本文件新增的包级辅助类型/函数一律带 schedInv 前缀。
package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ikik-api/internal/config"
	"ikik-api/internal/pkg/ctxkey"
	"ikik-api/internal/pkg/tlsfingerprint"
	middleware "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// schedInv 夹具
// ---------------------------------------------------------------------------

// schedInvUpstream 是固定响应的 HTTPUpstream 桩，记录每次调用的 accountID。
type schedInvUpstream struct {
	mu       sync.Mutex
	status   int
	body     string
	panicOn  bool
	attempts []int64
}

var _ service.HTTPUpstream = (*schedInvUpstream)(nil)

func (u *schedInvUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.attempts = append(u.attempts, accountID)
	shouldPanic := u.panicOn
	status := u.status
	body := u.body
	u.mu.Unlock()

	if req != nil && req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
	}
	if shouldPanic {
		panic("schedInv: simulated upstream panic")
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func (u *schedInvUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (u *schedInvUpstream) attemptedAccounts() []int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.attempts...)
}

// schedInvCountingCache 是带真实容量语义的 ConcurrencyCache，
// 记录账号/用户槽位与等待计数的获取-释放配平。
type schedInvCountingCache struct {
	mu sync.Mutex

	accountHeld map[int64]map[string]struct{}
	userHeld    map[int64]map[string]struct{}

	accountAcquired int
	accountReleased int
	// accountReleasedUnknown 记录对未持有 requestID 的重复/无效释放（幂等释放语义）
	accountReleasedUnknown int
	userAcquired           int
	userReleased           int
	userReleasedUnknown    int

	userWaitInc    int
	userWaitDec    int
	accountWaitInc int
	accountWaitDec int
}

var _ service.ConcurrencyCache = (*schedInvCountingCache)(nil)

func schedInvNewCountingCache() *schedInvCountingCache {
	return &schedInvCountingCache{
		accountHeld: make(map[int64]map[string]struct{}),
		userHeld:    make(map[int64]map[string]struct{}),
	}
}

func (c *schedInvCountingCache) AcquireAccountSlot(_ context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	held := c.accountHeld[accountID]
	if maxConcurrency > 0 && len(held) >= maxConcurrency {
		return false, nil
	}
	if held == nil {
		held = make(map[string]struct{})
		c.accountHeld[accountID] = held
	}
	held[requestID] = struct{}{}
	c.accountAcquired++
	return true, nil
}

func (c *schedInvCountingCache) ReleaseAccountSlot(_ context.Context, accountID int64, requestID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if held := c.accountHeld[accountID]; held != nil {
		if _, ok := held[requestID]; ok {
			delete(held, requestID)
			c.accountReleased++
			return nil
		}
	}
	c.accountReleasedUnknown++
	return nil
}

func (c *schedInvCountingCache) GetAccountConcurrency(_ context.Context, accountID int64) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.accountHeld[accountID]), nil
}

func (c *schedInvCountingCache) GetAccountConcurrencyBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[int64]int, len(accountIDs))
	for _, id := range accountIDs {
		result[id] = len(c.accountHeld[id])
	}
	return result, nil
}

func (c *schedInvCountingCache) IncrementAccountWaitCount(_ context.Context, _ int64, _ int) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accountWaitInc++
	return true, nil
}

func (c *schedInvCountingCache) DecrementAccountWaitCount(_ context.Context, _ int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accountWaitDec++
	return nil
}

func (c *schedInvCountingCache) GetAccountWaitingCount(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

func (c *schedInvCountingCache) AcquireUserSlot(_ context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	held := c.userHeld[userID]
	if maxConcurrency > 0 && len(held) >= maxConcurrency {
		return false, nil
	}
	if held == nil {
		held = make(map[string]struct{})
		c.userHeld[userID] = held
	}
	held[requestID] = struct{}{}
	c.userAcquired++
	return true, nil
}

func (c *schedInvCountingCache) ReleaseUserSlot(_ context.Context, userID int64, requestID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if held := c.userHeld[userID]; held != nil {
		if _, ok := held[requestID]; ok {
			delete(held, requestID)
			c.userReleased++
			return nil
		}
	}
	c.userReleasedUnknown++
	return nil
}

func (c *schedInvCountingCache) GetUserConcurrency(_ context.Context, userID int64) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.userHeld[userID]), nil
}

func (c *schedInvCountingCache) IncrementWaitCount(_ context.Context, _ int64, _ int) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userWaitInc++
	return true, nil
}

func (c *schedInvCountingCache) DecrementWaitCount(_ context.Context, _ int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userWaitDec++
	return nil
}

func (c *schedInvCountingCache) GetAccountsLoadBatch(_ context.Context, accounts []service.AccountWithConcurrency) (map[int64]*service.AccountLoadInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[int64]*service.AccountLoadInfo, len(accounts))
	for _, acc := range accounts {
		held := len(c.accountHeld[acc.ID])
		loadRate := 0
		if acc.MaxConcurrency > 0 {
			loadRate = held * 100 / acc.MaxConcurrency
		}
		result[acc.ID] = &service.AccountLoadInfo{
			AccountID:          acc.ID,
			CurrentConcurrency: held,
			LoadRate:           loadRate,
		}
	}
	return result, nil
}

func (c *schedInvCountingCache) GetUsersLoadBatch(_ context.Context, users []service.UserWithConcurrency) (map[int64]*service.UserLoadInfo, error) {
	result := make(map[int64]*service.UserLoadInfo, len(users))
	for _, u := range users {
		result[u.ID] = &service.UserLoadInfo{UserID: u.ID}
	}
	return result, nil
}

func (c *schedInvCountingCache) CleanupExpiredAccountSlots(_ context.Context, _ int64) error {
	return nil
}
func (c *schedInvCountingCache) CleanupExpiredAccountSlotKeys(_ context.Context) error {
	return nil
}
func (c *schedInvCountingCache) CleanupStaleProcessSlots(_ context.Context, _ string) error {
	return nil
}

// schedInvRequireBalanced 断言所有槽位与等待计数获取-释放严格配平。
func schedInvRequireBalanced(t *testing.T, cc *schedInvCountingCache) {
	t.Helper()
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for accountID, held := range cc.accountHeld {
		require.Empty(t, held, "账号 %d 仍有未释放的槽位", accountID)
	}
	for userID, held := range cc.userHeld {
		require.Empty(t, held, "用户 %d 仍有未释放的槽位", userID)
	}
	require.Equal(t, cc.accountAcquired, cc.accountReleased, "账号槽位获取/释放必须配平")
	require.Equal(t, cc.userAcquired, cc.userReleased, "用户槽位获取/释放必须配平")
	require.Equal(t, cc.userWaitInc, cc.userWaitDec, "用户等待计数增减必须配平")
	require.Equal(t, cc.accountWaitInc, cc.accountWaitDec, "账号等待计数增减必须配平")
}

// schedInvGroup 构造测试分组。
func schedInvGroup(groupID int64) *service.Group {
	return &service.Group{
		ID:       groupID,
		Hydrated: true,
		Platform: service.PlatformAnthropic,
		Status:   service.StatusActive,
	}
}

// schedInvPassthroughAccount 构造 anthropic API Key 透传账号（Forward 路径不依赖真实上游域名）。
func schedInvPassthroughAccount(id, groupID int64, extraCreds map[string]any) *service.Account {
	creds := map[string]any{"api_key": fmt.Sprintf("sk-sched-inv-%d", id)}
	for k, v := range extraCreds {
		creds[k] = v
	}
	return &service.Account{
		ID:            id,
		Name:          fmt.Sprintf("sched-inv-acc-%d", id),
		Platform:      service.PlatformAnthropic,
		Type:          service.AccountTypeAPIKey,
		Credentials:   creds,
		Extra:         map[string]any{"anthropic_passthrough": true},
		Concurrency:   5,
		Priority:      1,
		Status:        service.StatusActive,
		Schedulable:   true,
		AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: groupID}},
	}
}

// schedInvNewHandler 构造完整 GatewayHandler：真实 failover 循环 + 计数并发缓存 + 上游桩。
// cfg 传 nil 给 NewGatewayHandler，以固化默认换号上限（anthropic=10 / gemini=3）。
func schedInvNewHandler(t *testing.T, group *service.Group, accounts []*service.Account, upstream service.HTTPUpstream, cc *schedInvCountingCache) (*GatewayHandler, func()) {
	t.Helper()

	// 隔离环境变量：避免本机 SUB2API_DEBUG_GATEWAY_BODY 在测试期间写调试文件。
	t.Setenv("SUB2API_DEBUG_GATEWAY_BODY", "")

	schedulerSnapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil)
	concurrencySvc := service.NewConcurrencyService(cc)

	gwSvc := service.NewGatewayService(
		nil, // accountRepo（scheduler snapshot 命中，不需要）
		nil, // accountSharePolicyRepo
		&fakeGroupRepo{group: group},
		nil, nil, nil, nil, nil, // usageLogRepo / usageBillingRepo / userRepo / userSubRepo / userGroupRateRepo
		nil, // cache（粘性会话关闭）
		// 空 cfg（非 nil）：API Key 透传分支会校验默认 base_url，
		// validateUpstreamBaseURL 在 cfg=nil 时会空指针。
		&config.Config{},
		schedulerSnapshot,
		concurrencySvc,
		nil,                         // billingService
		&service.RateLimitService{}, // rateLimitService（零值：500 仅记录日志，无副作用）
		nil,                         // billingCacheService
		nil,                         // identityService
		upstream,
		nil,                // deferredService
		nil, nil, nil, nil, // claudeTokenProvider / sessionLimitCache / rpmCache / digestStore
		nil,                // settingService
		nil, nil, nil, nil, // tlsFPProfileService / channelService / resolver / balanceNotifyService
	)

	// RunModeSimple 跳过计费检查，避免引入 repo/cache 依赖。
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, nil,
		&config.Config{RunMode: config.RunModeSimple})

	h := NewGatewayHandler(
		gwSvc,
		nil, // geminiCompatService
		nil, // antigravityGatewayService
		ProvideGatewayPlatformRegistry(gwSvc, nil), // anthropic 账号经 anthropic provider → gwSvc.Forward
		nil, // userService
		concurrencySvc,
		billingCacheSvc,
		nil, // usageService
		nil, // apiKeyService
		nil, // usageRecordWorkerPool
		nil, // errorPassthroughService
		nil, // preFlightHooks
		nil, // userMsgQueueService
		nil, // cfg → 默认换号上限
		nil, // settingService
		nil, // carpoolService
	)
	return h, func() { billingCacheSvc.Stop() }
}

// schedInvNewMessagesContext 构造带认证上下文的 /v1/messages 请求。
// 返回的 cancel 用于模拟请求结束时的 context 取消（生产环境由 net/http 完成）。
func schedInvNewMessagesContext(t *testing.T, group *service.Group, body []byte) (*gin.Context, *httptest.ResponseRecorder, context.CancelFunc) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ctxkey.Group, group)
	ctx, cancel := context.WithCancel(ctx)
	c.Request = req.WithContext(ctx)

	apiKey := &service.APIKey{
		ID:      8301,
		UserID:  8401,
		GroupID: &group.ID,
		Status:  service.StatusActive,
		User: &service.User{
			ID:          8401,
			Concurrency: 10,
			Balance:     100,
		},
		Group: group,
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
	return c, rec, cancel
}

func schedInvMessagesBody() []byte {
	return []byte(`{
		"model": "claude-sonnet-4-5",
		"max_tokens": 256,
		"messages": [{"role":"user","content":[{"type":"text","text":"scheduling invariants probe"}]}]
	}`)
}

// ---------------------------------------------------------------------------
// I-5.1 三平台换号上限默认值（双断言）
// ---------------------------------------------------------------------------

func TestSchedulingInvariant_FailoverSwitchLimit_DefaultValues(t *testing.T) {
	t.Run("anthropic与gemini默认上限", func(t *testing.T) {
		h := NewGatewayHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		require.Equal(t, 10, h.maxAccountSwitches, "anthropic 默认换号上限必须为 10")
		require.Equal(t, 3, h.maxAccountSwitchesGemini, "gemini 默认换号上限必须为 3")
	})

	t.Run("openai默认上限", func(t *testing.T) {
		oh := NewOpenAIGatewayHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		require.Equal(t, 3, oh.maxAccountSwitches, "openai 默认换号上限必须为 3")
	})

	t.Run("配置可覆盖默认上限", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Gateway.MaxAccountSwitches = 5
		cfg.Gateway.MaxAccountSwitchesGemini = 2
		h := NewGatewayHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, cfg, nil, nil)
		require.Equal(t, 5, h.maxAccountSwitches)
		require.Equal(t, 2, h.maxAccountSwitchesGemini)
	})
}

// ---------------------------------------------------------------------------
// I-5.1 + I-5.2 + I-6.1：anthropic 完整 failover 循环
// ---------------------------------------------------------------------------

// TestSchedulingInvariant_FailoverAnthropic_FullLoopExhaustion 通过完整 Messages
// 入口驱动真实 failover 循环：12 个账号 + 恒定上游 500。
// 固化语义：
//  1. 恰好尝试 maxAccountSwitches+1 = 11 个账号（初次 + 10 次换号）；
//  2. 失败账号进入排除集合，同一账号不被尝试两次；
//  3. 耗尽后客户端收到 502 + {"type":"error","error":{"type":"upstream_error",...}}
//     （/v1/messages 路径按 mapUpstreamError 映射 500→502；
//     "server_error/All available accounts exhausted" 错误体属于
//     chat-completions/responses 兼容路径，见下方独立用例）；
//  4. 转发失败路径下账号/用户槽位获取-释放严格配平。
func TestSchedulingInvariant_FailoverAnthropic_FullLoopExhaustion(t *testing.T) {
	groupID := int64(9001)
	group := schedInvGroup(groupID)

	accounts := make([]*service.Account, 0, 12)
	for i := int64(1); i <= 12; i++ {
		accounts = append(accounts, schedInvPassthroughAccount(9100+i, groupID, nil))
	}

	upstream := &schedInvUpstream{
		status: http.StatusInternalServerError,
		body:   `{"type":"error","error":{"type":"api_error","message":"schedInv upstream boom"}}`,
	}
	cc := schedInvNewCountingCache()
	h, cleanup := schedInvNewHandler(t, group, accounts, upstream, cc)
	defer cleanup()

	c, rec, cancel := schedInvNewMessagesContext(t, group, schedInvMessagesBody())
	defer cancel()

	h.Messages(c)

	// 1. 换号次数恰为上限：尝试账号数 = maxAccountSwitches + 1（常量引用 + 硬编码双断言）
	attempts := upstream.attemptedAccounts()
	require.Len(t, attempts, h.maxAccountSwitches+1, "尝试账号数必须等于 maxAccountSwitches+1")
	require.Len(t, attempts, 11, "anthropic 平台：1 次初始尝试 + 10 次换号")

	// 2. 排除集合语义：同一账号不被重选
	seen := make(map[int64]struct{}, len(attempts))
	for _, accountID := range attempts {
		_, dup := seen[accountID]
		require.False(t, dup, "账号 %d 被尝试了两次：失败账号必须进入排除集合", accountID)
		seen[accountID] = struct{}{}
	}

	// 3. 耗尽后错误语义：502 + upstream_error 映射错误体（固化当前实际行为）
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.JSONEq(t,
		`{"type":"error","error":{"type":"upstream_error","message":"Upstream service temporarily unavailable"}}`,
		rec.Body.String())

	// 4. I-6.1 转发失败路径：槽位严格配平
	require.Equal(t, 11, cc.accountAcquired, "每次尝试获取一次账号槽位")
	require.Equal(t, 1, cc.userAcquired, "整个请求只获取一次用户槽位")
	schedInvRequireBalanced(t, cc)
}

// ---------------------------------------------------------------------------
// I-5.3 池模式可重试错误：同账号重试 3 次后换号（完整链）
// ---------------------------------------------------------------------------

// TestSchedulingInvariant_FailoverSameAccountRetry_FullChain 使用池模式账号
// （pool_mode_retry_status_codes=[500]）驱动完整链路：
// 上游恒定 500 → 同账号共尝试 1+maxSameAccountRetries=4 次 → 加入排除集合换号 →
// 无其他账号 → 502。同时验证每次重试均独立获取/释放账号槽位（配平）。
func TestSchedulingInvariant_FailoverSameAccountRetry_FullChain(t *testing.T) {
	groupID := int64(9002)
	group := schedInvGroup(groupID)

	poolAccount := schedInvPassthroughAccount(9201, groupID, map[string]any{
		"pool_mode":                    true,
		"pool_mode_retry_status_codes": []any{float64(500)},
	})

	upstream := &schedInvUpstream{
		status: http.StatusInternalServerError,
		body:   `{"type":"error","error":{"type":"api_error","message":"schedInv pool boom"}}`,
	}
	cc := schedInvNewCountingCache()
	h, cleanup := schedInvNewHandler(t, group, []*service.Account{poolAccount}, upstream, cc)
	defer cleanup()

	c, rec, cancel := schedInvNewMessagesContext(t, group, schedInvMessagesBody())
	defer cancel()

	h.Messages(c)

	// 同账号共尝试 1 + maxSameAccountRetries 次（常量引用 + 硬编码双断言）
	attempts := upstream.attemptedAccounts()
	require.Len(t, attempts, 1+maxSameAccountRetries, "可重试错误：1 次初始 + maxSameAccountRetries 次同账号重试")
	require.Len(t, attempts, 4)
	for _, accountID := range attempts {
		require.Equal(t, poolAccount.ID, accountID, "重试必须发生在同一账号上")
	}

	// 重试耗尽 + 换号后无可用账号 → 502
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.JSONEq(t,
		`{"type":"error","error":{"type":"upstream_error","message":"Upstream service temporarily unavailable"}}`,
		rec.Body.String())

	// 每轮重试独立获取/释放账号槽位
	require.Equal(t, 4, cc.accountAcquired)
	schedInvRequireBalanced(t, cc)
}

// ---------------------------------------------------------------------------
// I-5.1 gemini 换号上限：FailoverState 循环契约
// ---------------------------------------------------------------------------

// TestSchedulingInvariant_FailoverGemini_SwitchLimitLoopContract 按 Messages
// gemini 分支的真实接线（NewFailoverState(h.maxAccountSwitchesGemini, ...)）
// 驱动 FailoverState：连续上游失败时恰好允许 3+1=4 个账号尝试后耗尽。
// （gemini 平台 Forward 的服务层内部 500 重试带秒级退避，完整 e2e 不可在
// 单测时间预算内执行，故此处固化 handler 循环契约层语义。）
func TestSchedulingInvariant_FailoverGemini_SwitchLimitLoopContract(t *testing.T) {
	h := NewGatewayHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.Equal(t, 3, h.maxAccountSwitchesGemini)

	mock := &mockTempUnscheduler{}
	fs := NewFailoverState(h.maxAccountSwitchesGemini, false)

	attempts := 0
	for i := 1; ; i++ {
		require.LessOrEqual(t, i, 10, "防御：循环不应超过 10 次")
		attempts++
		action := fs.HandleFailoverError(context.Background(), mock, int64(i), service.PlatformGemini, newTestFailoverErr(500, false, false))
		if action == FailoverExhausted {
			break
		}
		require.Equal(t, FailoverContinue, action)
	}

	require.Equal(t, h.maxAccountSwitchesGemini+1, attempts, "gemini：尝试账号数 = 上限 + 1")
	require.Equal(t, 4, attempts)
	require.Len(t, fs.FailedAccountIDs, 4, "所有失败账号都进入排除集合")
}

// ---------------------------------------------------------------------------
// chat-completions 兼容路径耗尽错误体
// ---------------------------------------------------------------------------

// TestSchedulingInvariant_FailoverExhausted_ChatCompletionsErrorBody 固化
// chat-completions 兼容路径的耗尽错误体（INVARIANTS 记录的
// "All available accounts exhausted" 属于此路径而非 /v1/messages 路径）。
// 注意当前实际行为：lastErr 非空时状态码透传上游状态码（500→500），
// lastErr 为空时才回退 502。
func TestSchedulingInvariant_FailoverExhausted_ChatCompletionsErrorBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("lastErr存在_状态码透传上游", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

		h := &GatewayHandler{}
		h.handleCCFailoverExhausted(c, &service.UpstreamFailoverError{
			StatusCode:   http.StatusInternalServerError,
			ResponseBody: []byte(`{"error":{"message":"boom"}}`),
		}, false)

		require.Equal(t, http.StatusInternalServerError, rec.Code,
			"当前行为：lastErr 存在时透传上游状态码（非固定 502）")
		require.JSONEq(t,
			`{"error":{"type":"server_error","message":"All available accounts exhausted"}}`,
			rec.Body.String())
	})

	t.Run("lastErr为空_回退502", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

		h := &GatewayHandler{}
		h.handleCCFailoverExhausted(c, nil, false)

		require.Equal(t, http.StatusBadGateway, rec.Code)
		require.JSONEq(t,
			`{"error":{"type":"server_error","message":"All available accounts exhausted"}}`,
			rec.Body.String())
	})
}
