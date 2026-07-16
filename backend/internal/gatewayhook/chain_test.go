package gatewayhook

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// stubHook 是可编程的测试钩子，记录自己是否被执行。
type stubHook struct {
	id       string
	decision *Decision
	err      error
	panicVal any
	calls    *[]string
}

func (h *stubHook) HookID() string { return h.id }

func (h *stubHook) CheckPreFlight(ctx context.Context, req *Request) (*Decision, error) {
	if h.calls != nil {
		*h.calls = append(*h.calls, h.id)
	}
	if h.panicVal != nil {
		panic(h.panicVal)
	}
	return h.decision, h.err
}

func TestChainRun_ExecutesHooksInOrder(t *testing.T) {
	var calls []string
	chain := NewChain(zap.NewNop(),
		&stubHook{id: "first", calls: &calls},
		&stubHook{id: "second", calls: &calls},
		&stubHook{id: "third", calls: &calls},
	)

	decision := chain.Run(context.Background(), &Request{})

	require.Nil(t, decision, "全部放行时 Run 必须返回 nil")
	require.Equal(t, []string{"first", "second", "third"}, calls, "钩子必须按注册顺序执行")
}

func TestChainRun_FirstBlockedShortCircuits(t *testing.T) {
	var calls []string
	blocked := &Decision{Blocked: true, StatusCode: 403, ErrorType: "content_policy_violation", Message: "blocked"}
	chain := NewChain(zap.NewNop(),
		&stubHook{id: "pass", calls: &calls},
		&stubHook{id: "block", decision: blocked, calls: &calls},
		&stubHook{id: "never", calls: &calls},
	)

	decision := chain.Run(context.Background(), &Request{})

	require.Same(t, blocked, decision, "必须原样返回首个 Blocked Decision")
	require.Equal(t, []string{"pass", "block"}, calls, "Blocked 之后的钩子不得执行")
}

func TestChainRun_NonBlockedDecisionContinues(t *testing.T) {
	var calls []string
	chain := NewChain(zap.NewNop(),
		&stubHook{id: "flag-only", decision: &Decision{Blocked: false, Message: "flagged"}, calls: &calls},
		&stubHook{id: "after", calls: &calls},
	)

	decision := chain.Run(context.Background(), &Request{})

	require.Nil(t, decision, "非 Blocked 的 Decision 视为放行")
	require.Equal(t, []string{"flag-only", "after"}, calls)
}

func TestChainRun_PanicIsolatedAndFailOpen(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	var calls []string
	blocked := &Decision{Blocked: true, StatusCode: 403, Message: "blocked"}
	chain := NewChain(zap.New(core),
		&stubHook{id: "boomer", panicVal: "boom", calls: &calls},
		&stubHook{id: "blocker", decision: blocked, calls: &calls},
	)

	decision := chain.Run(context.Background(), &Request{})

	require.Same(t, blocked, decision, "panic 钩子 fail-open 后链必须继续执行后续钩子")
	require.Equal(t, []string{"boomer", "blocker"}, calls)

	entries := logs.FilterMessage("gatewayhook.hook_panic").All()
	require.Len(t, entries, 1, "panic 必须记一条 error 日志")
	fields := entries[0].ContextMap()
	require.Equal(t, "boomer", fields["hook_id"])
	require.Equal(t, "boom", fields["panic_value"])

	// 仅 panic 钩子的链：整体放行。
	onlyPanic := NewChain(zap.New(core), &stubHook{id: "boomer2", panicVal: errors.New("kaboom")})
	require.Nil(t, onlyPanic.Run(context.Background(), &Request{}))
}

func TestChainRun_HookErrorFailOpen(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	var calls []string
	chain := NewChain(zap.New(core),
		// 即使 error 钩子同时返回了 Blocked Decision，error 优先按 fail-open 丢弃。
		&stubHook{id: "broken", decision: &Decision{Blocked: true}, err: errors.New("hook exploded"), calls: &calls},
		&stubHook{id: "after", calls: &calls},
	)

	decision := chain.Run(context.Background(), &Request{})

	require.Nil(t, decision, "钩子 error 必须 fail-open 放行")
	require.Equal(t, []string{"broken", "after"}, calls, "error 钩子之后链必须继续")

	entries := logs.FilterMessage("gatewayhook.hook_error").All()
	require.Len(t, entries, 1)
	require.Equal(t, "broken", entries[0].ContextMap()["hook_id"])
}

func TestChainRun_EmptyChain(t *testing.T) {
	require.Nil(t, NewChain(zap.NewNop()).Run(context.Background(), &Request{}))
	require.True(t, NewChain(zap.NewNop()).IsEmpty())

	var nilChain *Chain
	require.True(t, nilChain.IsEmpty(), "nil 链视为空链")
	require.Nil(t, nilChain.Run(context.Background(), &Request{}), "nil 链 Run 必须安全放行")

	// nil 钩子在构造时被过滤。
	require.True(t, NewChain(zap.NewNop(), nil, nil).IsEmpty())
}
