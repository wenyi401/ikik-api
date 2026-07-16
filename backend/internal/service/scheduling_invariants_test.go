//go:build unit

// Phase-0 TASK-004 调度不变量测试（INVARIANTS I-4.1 / I-4.2 / I-4.3 / I-4.5）。
//
// 本文件固化粘性会话与选号过滤的外部可观测行为：
//   - I-4.1 绑定后二次请求命中同账号；session hash 输入优先级
//     （metadata.user_id → cacheable content → IP+UA+APIKeyID+system+messages）；
//   - I-4.2 粘性绑定 TTL = 1 小时（常量引用 + 硬编码双断言 + miniredis 过期语义）；
//   - I-4.3 sticky_escape：粘性账号不可用时能逃逸并重选成功（只断言硬语义，
//     不断言触发阈值细节）；
//   - I-4.5 账号级模型映射白名单（model_mapping）与渠道模型映射/定价限制对选号的影响。
//
// 断言纪律：不断言选号的具体排序结果（负载感知排序属可演化策略），
// 只断言"命中/逃逸/排除/拒绝"等硬语义。
//
// 复用同包既有夹具：mockAccountRepoForPlatform / mockGroupRepoForGateway /
// mockConcurrencyCache（gateway_multiplatform_test.go）、newTestChannelService /
// makeStandardRepo（channel_service_test.go）、mustParseSessionHashRequest /
// anthropicSessionBody / msg（generate_session_hash_test.go）。
// 本文件新增的包级辅助类型/函数一律带 schedInv 前缀。
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"ikik-api/internal/pkg/ctxkey"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// schedInv 夹具
// ---------------------------------------------------------------------------

// schedInvSetCall 记录一次 SetSessionAccountID / RefreshSessionTTL 调用。
type schedInvSetCall struct {
	groupID   int64
	hash      string
	accountID int64
	ttl       time.Duration
}

// schedInvGatewayCache 是 GatewayCache 的内存实现，记录 TTL 与删除调用。
type schedInvGatewayCache struct {
	mu           sync.Mutex
	bindings     map[string]int64
	strings      map[string]string
	setCalls     []schedInvSetCall
	refreshCalls []schedInvSetCall
	deleteCalls  []string
}

var _ GatewayCache = (*schedInvGatewayCache)(nil)

func schedInvNewGatewayCache() *schedInvGatewayCache {
	return &schedInvGatewayCache{
		bindings: make(map[string]int64),
		strings:  make(map[string]string),
	}
}

func schedInvCacheKey(groupID int64, hash string) string {
	return fmt.Sprintf("%d:%s", groupID, hash)
}

func (c *schedInvGatewayCache) GetSessionAccountID(_ context.Context, groupID int64, sessionHash string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if id, ok := c.bindings[schedInvCacheKey(groupID, sessionHash)]; ok {
		return id, nil
	}
	return 0, errors.New("schedInv: session binding not found")
}

func (c *schedInvGatewayCache) SetSessionAccountID(_ context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[schedInvCacheKey(groupID, sessionHash)] = accountID
	c.setCalls = append(c.setCalls, schedInvSetCall{groupID: groupID, hash: sessionHash, accountID: accountID, ttl: ttl})
	return nil
}

func (c *schedInvGatewayCache) RefreshSessionTTL(_ context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshCalls = append(c.refreshCalls, schedInvSetCall{groupID: groupID, hash: sessionHash, ttl: ttl})
	return nil
}

func (c *schedInvGatewayCache) DeleteSessionAccountID(_ context.Context, groupID int64, sessionHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.bindings, schedInvCacheKey(groupID, sessionHash))
	c.deleteCalls = append(c.deleteCalls, schedInvCacheKey(groupID, sessionHash))
	return nil
}

func (c *schedInvGatewayCache) GetSessionString(_ context.Context, groupID int64, sessionHash string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if value, ok := c.strings[schedInvCacheKey(groupID, sessionHash)]; ok {
		return value, nil
	}
	return "", errors.New("schedInv: session string not found")
}

func (c *schedInvGatewayCache) SetSessionString(_ context.Context, groupID int64, sessionHash string, value string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.strings[schedInvCacheKey(groupID, sessionHash)] = value
	return nil
}

func (c *schedInvGatewayCache) DeleteSessionString(_ context.Context, groupID int64, sessionHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.strings, schedInvCacheKey(groupID, sessionHash))
	return nil
}

func (c *schedInvGatewayCache) binding(groupID int64, hash string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bindings[schedInvCacheKey(groupID, hash)]
}

// schedInvRedisGatewayCache 是 miniredis 后端的 GatewayCache，
// 与 repository/gateway_cache.go 的键语义一致，用于验证 TTL 过期行为。
type schedInvRedisGatewayCache struct {
	rdb *redis.Client
}

var _ GatewayCache = (*schedInvRedisGatewayCache)(nil)

func (c *schedInvRedisGatewayCache) key(groupID int64, hash string) string {
	return fmt.Sprintf("sticky_session:%d:%s", groupID, hash)
}

func (c *schedInvRedisGatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	return c.rdb.Get(ctx, c.key(groupID, sessionHash)).Int64()
}

func (c *schedInvRedisGatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.key(groupID, sessionHash), accountID, ttl).Err()
}

func (c *schedInvRedisGatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, c.key(groupID, sessionHash), ttl).Err()
}

func (c *schedInvRedisGatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	return c.rdb.Del(ctx, c.key(groupID, sessionHash)).Err()
}

func (c *schedInvRedisGatewayCache) GetSessionString(ctx context.Context, groupID int64, sessionHash string) (string, error) {
	return c.rdb.Get(ctx, c.key(groupID, sessionHash)).Result()
}

func (c *schedInvRedisGatewayCache) SetSessionString(ctx context.Context, groupID int64, sessionHash string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.key(groupID, sessionHash), value, ttl).Err()
}

func (c *schedInvRedisGatewayCache) DeleteSessionString(ctx context.Context, groupID int64, sessionHash string) error {
	return c.rdb.Del(ctx, c.key(groupID, sessionHash)).Err()
}

// schedInvAccountRepo 构造账号 repo mock（复用 mockAccountRepoForPlatform）。
func schedInvAccountRepo(accounts ...Account) *mockAccountRepoForPlatform {
	repo := &mockAccountRepoForPlatform{
		accounts:     accounts,
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	return repo
}

// schedInvLoadAwareService 构造启用负载感知调度（Layer1.5/Layer2）的 GatewayService。
func schedInvLoadAwareService(repo *mockAccountRepoForPlatform, cache GatewayCache) *GatewayService {
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	return &GatewayService{
		accountRepo:        repo,
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
	}
}

func schedInvAnthropicAccount(id int64, priority int) Account {
	return Account{
		ID:          id,
		Name:        fmt.Sprintf("sched-inv-%d", id),
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Priority:    priority,
		Concurrency: 5,
		Status:      StatusActive,
		Schedulable: true,
	}
}

// ---------------------------------------------------------------------------
// I-4.1 session hash 输入优先级
// ---------------------------------------------------------------------------

// TestSchedulingInvariant_SessionHashSourcePriority 固化 session hash 三级输入优先级：
// metadata.user_id 的 session_xxx > cache_control(ephemeral) 内容 >
// IP+UA+APIKeyID+system+messages 完整摘要兜底。
func TestSchedulingInvariant_SessionHashSourcePriority(t *testing.T) {
	svc := &GatewayService{}
	sessionCtx := &SessionContext{ClientIP: "10.1.2.3", UserAgent: "claude-cli/2.0.0", APIKeyID: 77}
	metadata := "user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_123e4567-e89b-12d3-a456-426614174000"
	cacheableSystem := []any{map[string]any{
		"type":          "text",
		"text":          "long cacheable system prompt",
		"cache_control": map[string]any{"type": "ephemeral"},
	}}
	messages := []any{msg("user", "hello invariants")}

	// 1. 三个来源齐备 → metadata session_id 胜出
	full := mustParseSessionHashRequest(t, anthropicSessionBody(cacheableSystem, messages, metadata), sessionCtx)
	require.Equal(t, "123e4567-e89b-12d3-a456-426614174000", svc.GenerateSessionHash(full),
		"metadata.user_id 的 session_id 必须有最高优先级")

	// 2. 去掉 metadata → cacheable content hash 胜出（与 SessionContext 无关）
	cacheableOnly := mustParseSessionHashRequest(t, anthropicSessionBody(cacheableSystem, messages, ""), sessionCtx)
	cacheableNoCtx := mustParseSessionHashRequest(t, anthropicSessionBody(cacheableSystem, messages, ""), nil)
	h2 := svc.GenerateSessionHash(cacheableOnly)
	require.NotEmpty(t, h2)
	require.Equal(t, svc.GenerateSessionHash(cacheableNoCtx), h2,
		"存在 cacheable content 时 SessionContext 不参与 hash（第 2 级优先于第 3 级）")

	// 3. 无 metadata 且无 cacheable content → 兜底摘要，SessionContext 参与区分
	fallbackA := mustParseSessionHashRequest(t, anthropicSessionBody("plain system", messages, ""), sessionCtx)
	fallbackB := mustParseSessionHashRequest(t, anthropicSessionBody("plain system", messages, ""),
		&SessionContext{ClientIP: "10.9.9.9", UserAgent: "other-agent/1.0", APIKeyID: 88})
	h3a := svc.GenerateSessionHash(fallbackA)
	h3b := svc.GenerateSessionHash(fallbackB)
	require.NotEmpty(t, h3a)
	require.NotEqual(t, h3a, h3b, "兜底级 hash 必须混入 IP+UA+APIKeyID 区分因子")
	require.NotEqual(t, h2, h3a, "第 2 级与第 3 级来源应产生不同 hash")
}

// ---------------------------------------------------------------------------
// I-4.1 绑定后二次请求命中同账号
// ---------------------------------------------------------------------------

func TestSchedulingInvariant_StickySession_SecondSelectionHitsSameAccount(t *testing.T) {
	ctx := context.Background()
	repo := schedInvAccountRepo(
		schedInvAnthropicAccount(1, 1),
		schedInvAnthropicAccount(2, 1),
		schedInvAnthropicAccount(3, 1),
	)
	cache := schedInvNewGatewayCache()
	svc := schedInvLoadAwareService(repo, cache)

	const sessionHash = "sched-inv-sticky-hit"

	// 第一次选号：无绑定 → 任选一个账号并写入粘性绑定
	first, err := svc.SelectAccountWithLoadAwareness(ctx, nil, sessionHash, "claude-sonnet-4-5", nil, "", 0)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NotNil(t, first.Account)
	require.True(t, first.Acquired)
	boundID := cache.binding(0, sessionHash)
	require.Equal(t, first.Account.ID, boundID, "首次选号成功后必须建立粘性绑定")

	// 第二次选号：同 sessionHash 必须命中同一账号（无论负载排序如何演化）
	second, err := svc.SelectAccountWithLoadAwareness(ctx, nil, sessionHash, "claude-sonnet-4-5", nil, "", 0)
	require.NoError(t, err)
	require.NotNil(t, second.Account)
	require.Equal(t, first.Account.ID, second.Account.ID, "二次请求必须命中粘性绑定的同一账号")
}

// ---------------------------------------------------------------------------
// I-4.2 粘性绑定 TTL = 1 小时
// ---------------------------------------------------------------------------

func TestSchedulingInvariant_StickySession_TTLIsOneHour(t *testing.T) {
	// 双断言①：常量引用（常量被改动时此处失败）
	require.Equal(t, time.Hour, stickySessionTTL, "粘性会话 TTL 常量必须保持 1 小时")

	t.Run("BindStickySession传递TTL", func(t *testing.T) {
		cache := schedInvNewGatewayCache()
		svc := &GatewayService{cache: cache}
		require.NoError(t, svc.BindStickySession(context.Background(), nil, "sess-ttl", 42))
		require.Len(t, cache.setCalls, 1)
		// 双断言②：硬编码当前值 + 常量引用
		require.Equal(t, time.Hour, cache.setCalls[0].ttl)
		require.Equal(t, stickySessionTTL, cache.setCalls[0].ttl)
	})

	t.Run("粘性命中刷新TTL为1小时", func(t *testing.T) {
		ctx := context.Background()
		repo := schedInvAccountRepo(schedInvAnthropicAccount(1, 1), schedInvAnthropicAccount(2, 1))
		cache := schedInvNewGatewayCache()
		require.NoError(t, cache.SetSessionAccountID(ctx, 0, "sess-refresh", 1, stickySessionTTL))
		svc := schedInvLoadAwareService(repo, cache)

		result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sess-refresh", "claude-sonnet-4-5", nil, "", 0)
		require.NoError(t, err)
		require.Equal(t, int64(1), result.Account.ID, "应命中粘性账号")
		require.NotEmpty(t, cache.refreshCalls, "粘性命中后必须刷新 TTL")
		require.Equal(t, time.Hour, cache.refreshCalls[0].ttl)
	})

	t.Run("miniredis过期语义", func(t *testing.T) {
		ctx := context.Background()
		mr := miniredis.RunT(t)
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = rdb.Close() })
		svc := &GatewayService{cache: &schedInvRedisGatewayCache{rdb: rdb}}

		require.NoError(t, svc.BindStickySession(ctx, nil, "sess-expiry", 7))

		got, err := svc.GetCachedSessionAccountID(ctx, nil, "sess-expiry")
		require.NoError(t, err)
		require.Equal(t, int64(7), got)

		// TTL 内（59 分钟后）仍然命中
		mr.FastForward(stickySessionTTL - time.Minute)
		got, err = svc.GetCachedSessionAccountID(ctx, nil, "sess-expiry")
		require.NoError(t, err)
		require.Equal(t, int64(7), got)

		// 超过 TTL 后绑定消失（当前实现返回 0 + 底层 miss error，handler 忽略 error）
		mr.FastForward(2 * time.Minute)
		got, _ = svc.GetCachedSessionAccountID(ctx, nil, "sess-expiry")
		require.Equal(t, int64(0), got, "TTL 过期后粘性绑定必须失效")
	})
}

// ---------------------------------------------------------------------------
// I-4.3 sticky_escape：粘性账号不可用时逃逸重选
// ---------------------------------------------------------------------------

func TestSchedulingInvariant_StickyEscape_ReselectsWhenBoundAccountUnavailable(t *testing.T) {
	ctx := context.Background()
	const model = "claude-sonnet-4-5"

	t.Run("绑定账号不可调度_逃逸并重绑", func(t *testing.T) {
		// 账号 1 状态 error → 不在可调度列表；粘性绑定仍指向它
		broken := schedInvAnthropicAccount(1, 1)
		broken.Status = StatusError
		healthy := schedInvAnthropicAccount(2, 1)
		repo := schedInvAccountRepo(broken, healthy)
		cache := schedInvNewGatewayCache()
		require.NoError(t, cache.SetSessionAccountID(ctx, 0, "sess-escape-a", 1, time.Hour))
		svc := schedInvLoadAwareService(repo, cache)

		result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sess-escape-a", model, nil, "", 0)
		require.NoError(t, err, "粘性账号不可用时必须能逃逸重选，而不是失败")
		require.NotNil(t, result.Account)
		require.NotEqual(t, int64(1), result.Account.ID, "不得再选中不可用的粘性账号")
		require.Equal(t, result.Account.ID, cache.binding(0, "sess-escape-a"),
			"逃逸成功后粘性绑定更新为新账号（当前实现行为）")
	})

	t.Run("绑定账号模型限流_清除绑定并逃逸", func(t *testing.T) {
		limited := schedInvAnthropicAccount(1, 1)
		limited.Extra = map[string]any{
			"model_rate_limits": map[string]any{
				model: map[string]any{
					"rate_limit_reset_at": time.Now().Add(30 * time.Minute).Format(time.RFC3339),
				},
			},
		}
		healthy := schedInvAnthropicAccount(2, 1)
		repo := schedInvAccountRepo(limited, healthy)
		cache := schedInvNewGatewayCache()
		require.NoError(t, cache.SetSessionAccountID(ctx, 0, "sess-escape-b", 1, time.Hour))
		svc := schedInvLoadAwareService(repo, cache)

		result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sess-escape-b", model, nil, "", 0)
		require.NoError(t, err)
		require.Equal(t, int64(2), result.Account.ID, "逃逸后应重选到可用账号")
		require.NotEmpty(t, cache.deleteCalls, "模型限流的粘性绑定应被清除（当前实现行为）")
	})

	t.Run("绑定账号在failover排除集合_跳过粘性", func(t *testing.T) {
		repo := schedInvAccountRepo(schedInvAnthropicAccount(1, 1), schedInvAnthropicAccount(2, 1))
		cache := schedInvNewGatewayCache()
		require.NoError(t, cache.SetSessionAccountID(ctx, 0, "sess-escape-c", 1, time.Hour))
		svc := schedInvLoadAwareService(repo, cache)

		excluded := map[int64]struct{}{1: {}}
		result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sess-escape-c", model, excluded, "", 0)
		require.NoError(t, err)
		require.Equal(t, int64(2), result.Account.ID, "排除集合中的粘性账号必须被跳过")
	})
}

// ---------------------------------------------------------------------------
// I-4.5 账号级模型映射白名单对选号的影响
// ---------------------------------------------------------------------------

func TestSchedulingInvariant_AccountModelMappingFiltersSelection(t *testing.T) {
	ctx := context.Background()

	t.Run("不支持请求模型的账号被排除", func(t *testing.T) {
		// 账号 1 配了 model_mapping 白名单（只支持 opus），优先级更高；
		// 账号 2 未配映射（支持所有模型）。请求 sonnet 必须落到账号 2。
		restricted := schedInvAnthropicAccount(1, 0)
		restricted.Credentials = map[string]any{
			"model_mapping": map[string]any{"claude-opus-4-6": "claude-opus-4-6"},
		}
		open := schedInvAnthropicAccount(2, 9)
		repo := schedInvAccountRepo(restricted, open)
		svc := schedInvLoadAwareService(repo, schedInvNewGatewayCache())

		result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "", "claude-sonnet-4-5", nil, "", 0)
		require.NoError(t, err)
		require.Equal(t, int64(2), result.Account.ID,
			"配置了 model_mapping 白名单且不含请求模型的账号必须被排除（即使优先级更高）")
	})

	t.Run("支持模型的账号正常命中映射", func(t *testing.T) {
		restricted := schedInvAnthropicAccount(1, 0)
		restricted.Credentials = map[string]any{
			"model_mapping": map[string]any{"claude-opus-4-6": "claude-opus-4-6"},
		}
		repo := schedInvAccountRepo(restricted)
		svc := schedInvLoadAwareService(repo, schedInvNewGatewayCache())

		result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "", "claude-opus-4-6", nil, "", 0)
		require.NoError(t, err)
		require.Equal(t, int64(1), result.Account.ID)
	})

	t.Run("所有账号都不支持模型_返回无可用账号", func(t *testing.T) {
		a := schedInvAnthropicAccount(1, 1)
		a.Credentials = map[string]any{"model_mapping": map[string]any{"claude-opus-4-6": "claude-opus-4-6"}}
		b := schedInvAnthropicAccount(2, 1)
		b.Credentials = map[string]any{"model_mapping": map[string]any{"claude-haiku-4-5": "claude-haiku-4-5"}}
		repo := schedInvAccountRepo(a, b)
		svc := schedInvLoadAwareService(repo, schedInvNewGatewayCache())

		_, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "", "claude-sonnet-4-5", nil, "", 0)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNoAvailableAccounts)
	})
}

// ---------------------------------------------------------------------------
// I-4.5 渠道模型映射 + 定价限制对选号的影响
// ---------------------------------------------------------------------------

func TestSchedulingInvariant_ChannelMappingPricingRestrictionAffectsSelection(t *testing.T) {
	const groupID = int64(10)
	newSvc := func(pricingModels []string) (*GatewayService, *mockGroupRepoForGateway) {
		ch := Channel{
			ID:                 1,
			Status:             StatusActive,
			GroupIDs:           []int64{groupID},
			RestrictModels:     true,
			BillingModelSource: BillingModelSourceChannelMapped,
			ModelPricing: []ChannelModelPricing{
				{Platform: PlatformAnthropic, Models: pricingModels},
			},
			ModelMapping: map[string]map[string]string{
				PlatformAnthropic: {"claude-sonnet-4-5": "claude-sonnet-4-6"},
			},
		}
		channelSvc := newTestChannelService(makeStandardRepo(ch, map[int64]string{groupID: PlatformAnthropic}))
		repo := schedInvAccountRepo(func() Account {
			a := schedInvAnthropicAccount(1, 1)
			a.AccountGroups = []AccountGroup{{AccountID: 1, GroupID: groupID}}
			return a
		}())
		groupRepo := &mockGroupRepoForGateway{
			groups: map[int64]*Group{
				groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
			},
		}
		svc := &GatewayService{
			accountRepo:    repo,
			groupRepo:      groupRepo,
			channelService: channelSvc,
			cache:          schedInvNewGatewayCache(),
			cfg:            testConfig(),
		}
		return svc, groupRepo
	}
	ctxWithGroup := func(groupRepo *mockGroupRepoForGateway) context.Context {
		return context.WithValue(context.Background(), ctxkey.Group, groupRepo.groups[groupID])
	}

	t.Run("映射目标不在渠道定价_选号被拒绝", func(t *testing.T) {
		// 渠道映射 sonnet-4-5 → sonnet-4-6，但定价列表只有 opus：
		// 限制检查对象是"映射后模型"，因此请求 sonnet-4-5 必须被拒绝。
		gid := groupID
		svc, groupRepo := newSvc([]string{"claude-opus-4-6"})
		_, err := svc.SelectAccountWithLoadAwareness(ctxWithGroup(groupRepo), &gid, "", "claude-sonnet-4-5", nil, "", 0)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNoAvailableAccounts)
		require.Contains(t, err.Error(), "channel pricing restriction",
			"渠道定价限制必须以专属错误语义拒绝选号")
	})

	t.Run("映射目标在渠道定价_正常选号", func(t *testing.T) {
		// 定价列表含映射后的 sonnet-4-6（即使不含原始请求模型 sonnet-4-5）→ 放行。
		// 这证明渠道映射结果（而非原始模型名）决定选号阶段的限制判定。
		gid := groupID
		svc, groupRepo := newSvc([]string{"claude-sonnet-4-6"})
		result, err := svc.SelectAccountWithLoadAwareness(ctxWithGroup(groupRepo), &gid, "", "claude-sonnet-4-5", nil, "", 0)
		require.NoError(t, err)
		require.Equal(t, int64(1), result.Account.ID)
	})
}
