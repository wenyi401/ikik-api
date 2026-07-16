package gatewayhook

import (
	"context"

	"go.uber.org/zap"
	"ikik-api/internal/pkg/logger"
)

// Chain 按固定顺序执行 pre-flight 钩子。
//
// 执行语义（SEAM-DESIGN 裁决 H 链层三件必补）：
//  1. 每钩子 recover()：panic 记 error 日志后按 fail-open 继续下一钩子；
//  2. 钩子返回 error 默认 fail-open：记日志后继续下一钩子（与内容审核现状一致）；
//  3. 首个 Blocked Decision 即短路返回；非 Blocked 的 Decision 视为放行并继续。
//
// Run 永不向调用方返回 error：返回 nil 即放行。
type Chain struct {
	hooks []PreFlightHook
	log   *zap.Logger
}

// NewChain 构造钩子链。log 传 nil 时延迟回退到进程全局 logger；
// nil 钩子被忽略。
func NewChain(log *zap.Logger, hooks ...PreFlightHook) *Chain {
	filtered := make([]PreFlightHook, 0, len(hooks))
	for _, hook := range hooks {
		if hook != nil {
			filtered = append(filtered, hook)
		}
	}
	return &Chain{hooks: filtered, log: log}
}

// IsEmpty 报告链上是否没有任何钩子（nil 链视为空链）。
// 调用侧可据此零开销跳过钩子请求的构造。
func (c *Chain) IsEmpty() bool {
	return c == nil || len(c.hooks) == 0
}

// Run 按序执行钩子，返回首个 Blocked Decision；全部放行（或链为空）返回 nil。
func (c *Chain) Run(ctx context.Context, req *Request) *Decision {
	if c.IsEmpty() {
		return nil
	}
	for _, hook := range c.hooks {
		if decision := c.runHook(ctx, hook, req); decision != nil && decision.Blocked {
			return decision
		}
	}
	return nil
}

// runHook 执行单个钩子并隔离其 panic / error（均按 fail-open 降级为放行）。
func (c *Chain) runHook(ctx context.Context, hook PreFlightHook, req *Request) (decision *Decision) {
	hookID := ""
	defer func() {
		if r := recover(); r != nil {
			c.logger().Error("gatewayhook.hook_panic",
				zap.String("hook_id", hookID),
				zap.Any("panic_value", r),
				zap.Stack("stack"))
			decision = nil // fail-open：panic 钩子视为放行
		}
	}()
	hookID = hook.HookID()
	d, err := hook.CheckPreFlight(ctx, req)
	if err != nil {
		c.logger().Warn("gatewayhook.hook_error",
			zap.String("hook_id", hookID),
			zap.Error(err))
		return nil // fail-open：钩子自身故障视为放行
	}
	return d
}

func (c *Chain) logger() *zap.Logger {
	if c != nil && c.log != nil {
		return c.log
	}
	return logger.L()
}
