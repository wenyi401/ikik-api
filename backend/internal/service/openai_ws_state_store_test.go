package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIWSStateStore_BindGetDeleteResponseAccount(t *testing.T) {
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	ctx := context.Background()
	groupID := int64(7)

	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_abc", 101, time.Minute))

	accountID, err := store.GetResponseAccount(ctx, groupID, "resp_abc")
	require.NoError(t, err)
	require.Equal(t, int64(101), accountID)

	require.NoError(t, store.DeleteResponseAccount(ctx, groupID, "resp_abc"))
	accountID, err = store.GetResponseAccount(ctx, groupID, "resp_abc")
	require.NoError(t, err)
	require.Zero(t, accountID)
}

func TestOpenAIWSStateStore_ResponseConnTTL(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	store.BindResponseConn("resp_conn", "conn_1", 30*time.Millisecond)

	connID, ok := store.GetResponseConn("resp_conn")
	require.True(t, ok)
	require.Equal(t, "conn_1", connID)

	time.Sleep(60 * time.Millisecond)
	_, ok = store.GetResponseConn("resp_conn")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_SessionTurnStateTTL(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	store.BindSessionTurnState(9, "session_hash_1", "turn_state_1", 30*time.Millisecond)

	state, ok := store.GetSessionTurnState(9, "session_hash_1")
	require.True(t, ok)
	require.Equal(t, "turn_state_1", state)

	// group 隔离
	_, ok = store.GetSessionTurnState(10, "session_hash_1")
	require.False(t, ok)

	time.Sleep(60 * time.Millisecond)
	_, ok = store.GetSessionTurnState(9, "session_hash_1")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_SessionConnTTL(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	store.BindSessionConn(9, "session_hash_conn_1", "conn_1", 30*time.Millisecond)

	connID, ok := store.GetSessionConn(9, "session_hash_conn_1")
	require.True(t, ok)
	require.Equal(t, "conn_1", connID)

	// group 隔离
	_, ok = store.GetSessionConn(10, "session_hash_conn_1")
	require.False(t, ok)

	time.Sleep(60 * time.Millisecond)
	_, ok = store.GetSessionConn(9, "session_hash_conn_1")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_BindSessionResponse(t *testing.T) {
	cache := &stubGatewayCache{stringBindings: map[string]string{}}
	store := NewOpenAIWSStateStore(cache)
	ctx := context.Background()
	groupID := int64(19)
	sessionHash := "session_hash_chain_1"
	responseID := "resp_chain_5"

	require.NoError(t, store.BindSessionResponse(ctx, groupID, sessionHash, responseID, time.Minute))

	latest, err := store.GetSessionLatestResponse(ctx, groupID, sessionHash)
	require.NoError(t, err)
	require.Equal(t, responseID, latest)

	ownerSession, err := store.GetResponseSession(ctx, groupID, responseID)
	require.NoError(t, err)
	require.Equal(t, sessionHash, ownerSession)

	cachedLatest := cache.stringBindings[openAIWSSessionLatestResponseCacheKey(sessionHash)]
	cachedOwner := cache.stringBindings[openAIWSResponseSessionCacheKey(responseID)]
	require.Equal(t, responseID, cachedLatest)
	require.Equal(t, sessionHash, cachedOwner)
}

func TestOpenAIWSStateStore_GetSessionLatestResponse_NoStaleAfterCacheMiss(t *testing.T) {
	cache := &stubGatewayCache{stringBindings: map[string]string{}}
	store := NewOpenAIWSStateStore(cache)
	ctx := context.Background()
	groupID := int64(22)
	sessionHash := "session_hash_latest_stale"

	require.NoError(t, store.BindSessionResponse(ctx, groupID, sessionHash, "resp_latest_1", time.Minute))
	latest, err := store.GetSessionLatestResponse(ctx, groupID, sessionHash)
	require.NoError(t, err)
	require.Equal(t, "resp_latest_1", latest)

	delete(cache.stringBindings, openAIWSSessionLatestResponseCacheKey(sessionHash))
	latest, err = store.GetSessionLatestResponse(ctx, groupID, sessionHash)
	require.NoError(t, err)
	require.Empty(t, latest, "多实例 latest_response_id 读取应以 Redis 为准，避免本机旧值误纠偏")
}

func TestOpenAIWSStateStore_BindSessionResponse_LocalFallback(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	ctx := context.Background()
	groupID := int64(20)

	require.NoError(t, store.BindSessionResponse(ctx, groupID, "session_hash_local", "resp_local", time.Minute))

	latest, err := store.GetSessionLatestResponse(ctx, groupID, "session_hash_local")
	require.NoError(t, err)
	require.Equal(t, "resp_local", latest)

	ownerSession, err := store.GetResponseSession(ctx, groupID, "resp_local")
	require.NoError(t, err)
	require.Equal(t, "session_hash_local", ownerSession)
}

func TestOpenAIGatewayService_RepairOpenAIWSPreviousResponseIDForSession_UsesPreviousOwnerSession(t *testing.T) {
	ctx := context.Background()
	groupID := int64(21)
	cache := &stubGatewayCache{}
	svc := &OpenAIGatewayService{cache: cache}
	store := svc.getOpenAIWSStateStore()

	require.NoError(t, store.BindSessionResponse(ctx, groupID, "session_chain_owner", "resp_chain_3", time.Minute))
	require.NoError(t, store.BindSessionResponse(ctx, groupID, "session_chain_owner", "resp_chain_5", time.Minute))

	payload := []byte(`{"type":"response.create","model":"gpt-5.1","previous_response_id":"resp_chain_3","input":"sixth"}`)
	repairedPayload, repairedPreviousResponseID, repaired := svc.RepairOpenAIWSPreviousResponseIDForSession(
		ctx,
		groupID,
		"different_content_hash_session",
		payload,
		true,
	)

	require.True(t, repaired)
	require.Equal(t, "resp_chain_5", repairedPreviousResponseID)
	require.Equal(t, "resp_chain_5", gjson.GetBytes(repairedPayload, "previous_response_id").String())
}

func TestOpenAIWSStateStore_GetResponseAccount_NoStaleAfterCacheMiss(t *testing.T) {
	cache := &stubGatewayCache{sessionBindings: map[string]int64{}}
	store := NewOpenAIWSStateStore(cache)
	ctx := context.Background()
	groupID := int64(17)
	responseID := "resp_cache_stale"
	cacheKey := openAIWSResponseAccountCacheKey(responseID)

	cache.sessionBindings[cacheKey] = 501
	accountID, err := store.GetResponseAccount(ctx, groupID, responseID)
	require.NoError(t, err)
	require.Equal(t, int64(501), accountID)

	delete(cache.sessionBindings, cacheKey)
	accountID, err = store.GetResponseAccount(ctx, groupID, responseID)
	require.NoError(t, err)
	require.Zero(t, accountID, "上游缓存失效后不应继续命中本地陈旧映射")
}

func TestOpenAIWSStateStore_MaybeCleanupRemovesExpiredIncrementally(t *testing.T) {
	raw := NewOpenAIWSStateStore(nil)
	store, ok := raw.(*defaultOpenAIWSStateStore)
	require.True(t, ok)

	expiredAt := time.Now().Add(-time.Minute)
	total := 2048
	store.responseToConnMu.Lock()
	for i := 0; i < total; i++ {
		store.responseToConn[fmt.Sprintf("resp_%d", i)] = openAIWSConnBinding{
			connID:    "conn_incremental",
			expiresAt: expiredAt,
		}
	}
	store.responseToConnMu.Unlock()

	store.lastCleanupUnixNano.Store(time.Now().Add(-2 * openAIWSStateStoreCleanupInterval).UnixNano())
	store.maybeCleanup()

	store.responseToConnMu.RLock()
	remainingAfterFirst := len(store.responseToConn)
	store.responseToConnMu.RUnlock()
	require.Less(t, remainingAfterFirst, total, "单轮 cleanup 应至少有进展")
	require.Greater(t, remainingAfterFirst, 0, "增量清理不要求单轮清空全部键")

	for i := 0; i < 8; i++ {
		store.lastCleanupUnixNano.Store(time.Now().Add(-2 * openAIWSStateStoreCleanupInterval).UnixNano())
		store.maybeCleanup()
	}

	store.responseToConnMu.RLock()
	remaining := len(store.responseToConn)
	store.responseToConnMu.RUnlock()
	require.Zero(t, remaining, "多轮 cleanup 后应逐步清空全部过期键")
}

func TestEnsureBindingCapacity_EvictsOneWhenMapIsFull(t *testing.T) {
	bindings := map[string]int{
		"a": 1,
		"b": 2,
	}

	ensureBindingCapacity(bindings, "c", 2)
	bindings["c"] = 3

	require.Len(t, bindings, 2)
	require.Equal(t, 3, bindings["c"])
}

func TestEnsureBindingCapacity_DoesNotEvictWhenUpdatingExistingKey(t *testing.T) {
	bindings := map[string]int{
		"a": 1,
		"b": 2,
	}

	ensureBindingCapacity(bindings, "a", 2)
	bindings["a"] = 9

	require.Len(t, bindings, 2)
	require.Equal(t, 9, bindings["a"])
}

type openAIWSStateStoreTimeoutProbeCache struct {
	setHasDeadline    bool
	getHasDeadline    bool
	deleteHasDeadline bool
	setStringDeadline bool
	getStringDeadline bool
	setDeadlineDelta  time.Duration
	getDeadlineDelta  time.Duration
	delDeadlineDelta  time.Duration
}

func (c *openAIWSStateStoreTimeoutProbeCache) GetSessionAccountID(ctx context.Context, _ int64, _ string) (int64, error) {
	if deadline, ok := ctx.Deadline(); ok {
		c.getHasDeadline = true
		c.getDeadlineDelta = time.Until(deadline)
	}
	return 123, nil
}

func (c *openAIWSStateStoreTimeoutProbeCache) SetSessionAccountID(ctx context.Context, _ int64, _ string, _ int64, _ time.Duration) error {
	if deadline, ok := ctx.Deadline(); ok {
		c.setHasDeadline = true
		c.setDeadlineDelta = time.Until(deadline)
	}
	return errors.New("set failed")
}

func (c *openAIWSStateStoreTimeoutProbeCache) RefreshSessionTTL(context.Context, int64, string, time.Duration) error {
	return nil
}

func (c *openAIWSStateStoreTimeoutProbeCache) DeleteSessionAccountID(ctx context.Context, _ int64, _ string) error {
	if deadline, ok := ctx.Deadline(); ok {
		c.deleteHasDeadline = true
		c.delDeadlineDelta = time.Until(deadline)
	}
	return nil
}

func (c *openAIWSStateStoreTimeoutProbeCache) GetSessionString(ctx context.Context, _ int64, _ string) (string, error) {
	if _, ok := ctx.Deadline(); ok {
		c.getStringDeadline = true
	}
	return "", errors.New("not found")
}

func (c *openAIWSStateStoreTimeoutProbeCache) SetSessionString(ctx context.Context, _ int64, _ string, _ string, _ time.Duration) error {
	if _, ok := ctx.Deadline(); ok {
		c.setStringDeadline = true
	}
	return nil
}

func (c *openAIWSStateStoreTimeoutProbeCache) DeleteSessionString(context.Context, int64, string) error {
	return nil
}

func TestOpenAIWSStateStore_RedisOpsUseShortTimeout(t *testing.T) {
	probe := &openAIWSStateStoreTimeoutProbeCache{}
	store := NewOpenAIWSStateStore(probe)
	ctx := context.Background()
	groupID := int64(5)

	err := store.BindResponseAccount(ctx, groupID, "resp_timeout_probe", 11, time.Minute)
	require.Error(t, err)

	accountID, getErr := store.GetResponseAccount(ctx, groupID, "resp_timeout_probe")
	require.NoError(t, getErr)
	require.Equal(t, int64(11), accountID, "本地缓存命中应优先返回已绑定账号")

	require.NoError(t, store.DeleteResponseAccount(ctx, groupID, "resp_timeout_probe"))

	require.True(t, probe.setHasDeadline, "SetSessionAccountID 应携带独立超时上下文")
	require.True(t, probe.deleteHasDeadline, "DeleteSessionAccountID 应携带独立超时上下文")
	require.False(t, probe.getHasDeadline, "GetSessionAccountID 本用例应由本地缓存命中，不触发 Redis 读取")
	require.Greater(t, probe.setDeadlineDelta, 2*time.Second)
	require.LessOrEqual(t, probe.setDeadlineDelta, 3*time.Second)
	require.Greater(t, probe.delDeadlineDelta, 2*time.Second)
	require.LessOrEqual(t, probe.delDeadlineDelta, 3*time.Second)

	probe2 := &openAIWSStateStoreTimeoutProbeCache{}
	store2 := NewOpenAIWSStateStore(probe2)
	accountID2, err2 := store2.GetResponseAccount(ctx, groupID, "resp_cache_only")
	require.NoError(t, err2)
	require.Equal(t, int64(123), accountID2)
	require.True(t, probe2.getHasDeadline, "GetSessionAccountID 在缓存未命中时应携带独立超时上下文")
	require.Greater(t, probe2.getDeadlineDelta, 2*time.Second)
	require.LessOrEqual(t, probe2.getDeadlineDelta, 3*time.Second)
}

func TestWithOpenAIWSStateStoreRedisTimeout_WithParentContext(t *testing.T) {
	ctx, cancel := withOpenAIWSStateStoreRedisTimeout(context.Background())
	defer cancel()
	require.NotNil(t, ctx)
	_, ok := ctx.Deadline()
	require.True(t, ok, "应附加短超时")
}
