//go:build unit

// Phase-0 TASK-004 并发槽不变量测试（INVARIANTS I-6.1 / I-6.2）。
//
// 固化内容：
//   - I-6.1 用户槽/账号槽获取-释放严格配平：
//   - 正常路径（上游 200，完整 Messages 链路）；
//   - early-return 路径（预热拦截：选号后转发前直接返回）；
//   - panic 路径（defer 释放用户槽 + context 取消兜底回收账号槽）；
//   - wrapReleaseOnDone 恰好一次语义（显式调用 / context 取消两种触发）；
//   - service 层 AcquireResult.ReleaseFunc 重复调用的当前行为特征化；
//   - I-6.2 AcquireUserSlotWithWait 等待队列：满 → 等待 → 释放 → 唤醒；
//     等待超时返回 ConcurrencyError{IsTimeout}。
//
// 转发失败路径的配平已由 scheduling_invariants_failover_test.go 的完整循环用例覆盖。
// 复用本包 schedInv* 夹具（scheduling_invariants_failover_test.go）。
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// schedInvNewGinContext 构造仅含基础请求的 gin 上下文（供 ConcurrencyHelper 使用）。
func schedInvNewGinContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return c, rec
}

// ---------------------------------------------------------------------------
// I-6.2 用户槽等待队列：满 → 等待 → 释放 → 唤醒
// ---------------------------------------------------------------------------

func TestSchedulingInvariant_UserSlotWaitQueue_FullWaitReleaseWakeup(t *testing.T) {
	cc := schedInvNewCountingCache()
	helper := NewConcurrencyHelper(service.NewConcurrencyService(cc), SSEPingFormatNone, time.Hour)

	const userID = int64(701)

	// 1. 占满唯一槽位
	c1, _ := schedInvNewGinContext(t)
	var ss1 bool
	release1, err := helper.AcquireUserSlotWithWait(c1, userID, 1, false, &ss1)
	require.NoError(t, err)
	require.NotNil(t, release1)

	// 2. 第二个请求进入等待（槽位已满）
	type waitResult struct {
		release func()
		err     error
	}
	done := make(chan waitResult, 1)
	go func() {
		c2, _ := schedInvNewGinContext(t)
		var ss2 bool
		release2, err2 := helper.AcquireUserSlotWithWait(c2, userID, 1, false, &ss2)
		done <- waitResult{release: release2, err: err2}
	}()

	// 槽位未释放期间，等待者不应返回
	select {
	case r := <-done:
		t.Fatalf("槽位未释放时等待者不应获取成功: err=%v", r.err)
	case <-time.After(400 * time.Millisecond):
	}

	// 3. 释放 → 4. 等待者被唤醒并成功获取
	release1()
	select {
	case r := <-done:
		require.NoError(t, r.err, "释放后等待者必须能获取槽位")
		require.NotNil(t, r.release)
		r.release()
	case <-time.After(10 * time.Second):
		t.Fatal("释放槽位后等待者在 10s 内未被唤醒")
	}

	require.Equal(t, 2, cc.userAcquired, "两次成功获取")
	schedInvRequireBalanced(t, cc)
}

// TestSchedulingInvariant_AccountSlotWait_TimeoutReturnsConcurrencyError 固化：
// 账号槽等待超时后返回 ConcurrencyError{IsTimeout:true}，且不产生未配平的获取。
func TestSchedulingInvariant_AccountSlotWait_TimeoutReturnsConcurrencyError(t *testing.T) {
	cc := schedInvNewCountingCache()
	svc := service.NewConcurrencyService(cc)
	helper := NewConcurrencyHelper(svc, SSEPingFormatNone, time.Hour)

	const accountID = int64(801)

	// 占满唯一槽位
	holder, err := svc.AcquireAccountSlot(context.Background(), accountID, 1)
	require.NoError(t, err)
	require.True(t, holder.Acquired)

	c, _ := schedInvNewGinContext(t)
	var ss bool
	release, err := helper.AcquireAccountSlotWithWaitTimeout(c, accountID, 1, 300*time.Millisecond, false, &ss)
	require.Nil(t, release)
	require.Error(t, err)
	var concurrencyErr *ConcurrencyError
	require.ErrorAs(t, err, &concurrencyErr)
	require.True(t, concurrencyErr.IsTimeout, "等待超时必须返回 IsTimeout 的 ConcurrencyError")
	require.Equal(t, "account", concurrencyErr.SlotType)

	holder.ReleaseFunc()
	schedInvRequireBalanced(t, cc)
}

// ---------------------------------------------------------------------------
// I-6.1 正常路径与 early-return 路径配平（完整 Messages 链路）
// ---------------------------------------------------------------------------

// TestSchedulingInvariant_SlotBalance_NormalSuccessPath 固化：上游 200 正常完成时，
// 账号槽与用户槽各获取/释放一次，严格配平。
func TestSchedulingInvariant_SlotBalance_NormalSuccessPath(t *testing.T) {
	groupID := int64(9003)
	group := schedInvGroup(groupID)
	account := schedInvPassthroughAccount(9301, groupID, nil)

	upstream := &schedInvUpstream{
		status: http.StatusOK,
		body: `{"id":"msg_sched_inv","type":"message","role":"assistant","model":"claude-sonnet-4-5",` +
			`"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn",` +
			`"usage":{"input_tokens":3,"output_tokens":2}}`,
	}
	cc := schedInvNewCountingCache()
	h, cleanup := schedInvNewHandler(t, group, []*service.Account{account}, upstream, cc)
	defer cleanup()

	c, rec, cancel := schedInvNewMessagesContext(t, group, schedInvMessagesBody())
	defer cancel()

	h.Messages(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "msg_sched_inv", "上游 200 响应体应透传给客户端")
	require.Len(t, upstream.attemptedAccounts(), 1)
	require.Equal(t, 1, cc.accountAcquired)
	require.Equal(t, 1, cc.userAcquired)
	schedInvRequireBalanced(t, cc)
}

// TestSchedulingInvariant_SlotBalance_InterceptEarlyReturnPath 固化 early-return 路径：
// 预热拦截在选号成功后、转发上游前直接返回 mock 响应，账号槽必须被释放。
func TestSchedulingInvariant_SlotBalance_InterceptEarlyReturnPath(t *testing.T) {
	groupID := int64(9004)
	group := schedInvGroup(groupID)
	account := schedInvPassthroughAccount(9401, groupID, map[string]any{
		"intercept_warmup_requests": true,
	})

	upstream := &schedInvUpstream{status: http.StatusOK, body: `{}`}
	cc := schedInvNewCountingCache()
	h, cleanup := schedInvNewHandler(t, group, []*service.Account{account}, upstream, cc)
	defer cleanup()

	warmupBody := []byte(`{
		"model": "claude-sonnet-4-5",
		"max_tokens": 256,
		"messages": [{"role":"user","content":[{"type":"text","text":"Warmup"}]}]
	}`)
	c, rec, cancel := schedInvNewMessagesContext(t, group, warmupBody)
	defer cancel()

	h.Messages(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "msg_mock_warmup", "预热请求应被拦截返回 mock 响应")
	require.Empty(t, upstream.attemptedAccounts(), "early-return 路径不应触达上游")
	require.Equal(t, 1, cc.accountAcquired, "选号阶段获取过账号槽")
	schedInvRequireBalanced(t, cc)
}

// ---------------------------------------------------------------------------
// I-6.1 panic 路径：defer 释放用户槽 + context 取消兜底回收账号槽
// ---------------------------------------------------------------------------

// TestSchedulingInvariant_SlotBalance_PanicPath 固化当前 panic 语义：
//   - 用户槽通过 defer userReleaseFunc() 在 panic 展开时立即释放；
//   - 账号槽的释放调用位于 Forward 之后（非 defer），panic 时依赖
//     wrapReleaseOnDone 注册的 context.AfterFunc 在请求 context 取消时兜底回收
//     （生产环境由 net/http 在连接结束时取消请求 context）。
func TestSchedulingInvariant_SlotBalance_PanicPath(t *testing.T) {
	groupID := int64(9005)
	group := schedInvGroup(groupID)
	account := schedInvPassthroughAccount(9501, groupID, nil)

	upstream := &schedInvUpstream{status: http.StatusOK, body: `{}`, panicOn: true}
	cc := schedInvNewCountingCache()
	h, cleanup := schedInvNewHandler(t, group, []*service.Account{account}, upstream, cc)
	defer cleanup()

	c, _, cancel := schedInvNewMessagesContext(t, group, schedInvMessagesBody())
	defer cancel()

	func() {
		defer func() {
			require.NotNil(t, recover(), "上游 panic 应穿透 Messages（由外层 gin recovery 兜底）")
		}()
		h.Messages(c)
	}()

	// 用户槽：defer 在 panic 展开时已释放
	cc.mu.Lock()
	userReleased := cc.userReleased
	cc.mu.Unlock()
	require.Equal(t, 1, userReleased, "panic 展开时 defer 必须释放用户槽")

	// 账号槽：panic 跳过了显式释放调用，由请求 context 取消兜底回收
	cancel()
	require.Eventually(t, func() bool {
		cc.mu.Lock()
		defer cc.mu.Unlock()
		return cc.accountReleased == cc.accountAcquired
	}, 5*time.Second, 10*time.Millisecond, "context 取消后账号槽必须被兜底回收")
	schedInvRequireBalanced(t, cc)
}

// ---------------------------------------------------------------------------
// I-6.1 wrapReleaseOnDone 恰好一次语义
// ---------------------------------------------------------------------------

func TestSchedulingInvariant_WrapReleaseOnDone_ExactlyOnce(t *testing.T) {
	t.Run("重复显式调用只释放一次", func(t *testing.T) {
		var calls atomic.Int32
		release := wrapReleaseOnDone(context.Background(), func() { calls.Add(1) })
		release()
		release()
		require.Equal(t, int32(1), calls.Load(), "重复调用不得重复释放")
	})

	t.Run("context取消自动触发释放", func(t *testing.T) {
		var calls atomic.Int32
		ctx, cancel := context.WithCancel(context.Background())
		release := wrapReleaseOnDone(ctx, func() { calls.Add(1) })
		cancel()
		require.Eventually(t, func() bool { return calls.Load() == 1 },
			2*time.Second, 5*time.Millisecond, "context 取消必须自动触发释放")
		// 取消后再显式调用也不重复释放
		release()
		require.Equal(t, int32(1), calls.Load())
	})

	t.Run("nil释放函数返回nil", func(t *testing.T) {
		require.Nil(t, wrapReleaseOnDone(context.Background(), nil))
	})
}

// ---------------------------------------------------------------------------
// service 层 ReleaseFunc 重复调用特征化
// ---------------------------------------------------------------------------

// TestSchedulingCharacterization_ServiceReleaseFuncNotOnceGuarded 固化当前实际行为：
// ConcurrencyService 返回的 AcquireResult.ReleaseFunc 本身没有 once 保护，
// 重复调用会向缓存重复发出释放请求；配平依赖两点：
//  1. 缓存释放按 requestID 幂等（Redis ZREM 不存在的成员是 no-op）；
//  2. handler 层统一经 wrapReleaseOnDone 包装后才暴露。
func TestSchedulingCharacterization_ServiceReleaseFuncNotOnceGuarded(t *testing.T) {
	cc := schedInvNewCountingCache()
	svc := service.NewConcurrencyService(cc)

	result, err := svc.AcquireAccountSlot(context.Background(), 901, 5)
	require.NoError(t, err)
	require.True(t, result.Acquired)

	result.ReleaseFunc()
	result.ReleaseFunc()

	cc.mu.Lock()
	defer cc.mu.Unlock()
	require.Equal(t, 1, cc.accountReleased, "第一次释放生效")
	require.Equal(t, 1, cc.accountReleasedUnknown,
		"当前行为：第二次释放仍会调用缓存（按 requestID 幂等，不破坏配平）")
	require.Empty(t, cc.accountHeld[901])
}
