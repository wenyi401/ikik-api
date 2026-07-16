// Package hello 实现示例插件模块 job.hello。
//
// 该模块是插件内核的最小可运行示例：实现全部四个可选生命周期接口
// （Provision / Validate / Start / Stop），启动后按配置间隔周期性输出
// 一条 debug 日志。默认 disabled——仅在 `modules:` 配置中显式 enabled
// 后才会运行，用于验证插件链路连通性，并作为编写新模块的参考实现。
package hello

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"ikik-api/internal/plugin"
)

// ID 是 hello 模块的模块 ID。
const ID plugin.ModuleID = "job.hello"

func init() {
	plugin.RegisterModule(&Module{})
}

// Config 是 hello 模块的私有配置
// （`modules:` 子树中 "job.hello" 项除 enabled 外的部分）。
type Config struct {
	// Interval 是周期日志的输出间隔，必须大于 0。
	Interval time.Duration `mapstructure:"interval"`

	// Greeting 是周期日志输出的内容，不能为空。
	Greeting string `mapstructure:"greeting"`
}

// defaultConfig 返回 hello 模块的默认配置（用户未配置的字段保持默认值）。
func defaultConfig() Config {
	return Config{
		Interval: 30 * time.Second,
		Greeting: "hello from job.hello",
	}
}

// Module 是 job.hello 模块实例。零值即可用，依赖在 Provision 中装配；
// 每次 Runtime.Build 都会通过 ModuleInfo.New 创建全新实例。
type Module struct {
	cfg    Config
	logger *zap.Logger

	cancel context.CancelFunc
	done   chan struct{}
}

// 编译期断言：Module 实现 plugin.Module 与全部四个可选生命周期接口。
var (
	_ plugin.Module      = (*Module)(nil)
	_ plugin.Provisioner = (*Module)(nil)
	_ plugin.Validator   = (*Module)(nil)
	_ plugin.Starter     = (*Module)(nil)
	_ plugin.Stopper     = (*Module)(nil)
)

// ModuleInfo 返回模块注册信息。EnabledByDefault 为 false：
// 未在 `modules:` 中显式 enabled 时模块不实例化、无任何副作用。
func (*Module) ModuleInfo() plugin.ModuleInfo {
	return plugin.ModuleInfo{
		ID:               ID,
		New:              func() plugin.Module { return new(Module) },
		EnabledByDefault: false,
	}
}

// Provision 从宿主获取 logger，并在默认配置之上解码模块私有配置。
func (m *Module) Provision(host *plugin.Host) error {
	m.cfg = defaultConfig()
	if host != nil {
		m.logger = host.Logger
		if host.ConfigOf != nil {
			if err := host.ConfigOf(ID, &m.cfg); err != nil {
				return err
			}
		}
	}
	if m.logger == nil {
		m.logger = zap.NewNop()
	}
	return nil
}

// Validate 校验模块配置：interval 必须大于 0，greeting 不能为空。
func (m *Module) Validate() error {
	if m.cfg.Interval <= 0 {
		return fmt.Errorf("hello: interval must be positive, got %s", m.cfg.Interval)
	}
	if m.cfg.Greeting == "" {
		return errors.New("hello: greeting must not be empty")
	}
	return nil
}

// Start 启动周期日志 goroutine。
//
// 模块持有独立于启动 ctx 的生命周期上下文，由 Stop 负责取消，
// 因此模块的运行寿命不受启动流程上下文的影响。
func (m *Module) Start(_ context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.done = make(chan struct{})
	go m.run(ctx)
	m.logger.Info("hello module started",
		zap.String("module", string(ID)),
		zap.Duration("interval", m.cfg.Interval))
	return nil
}

// run 按配置间隔输出周期 debug 日志，直到 ctx 被取消。
func (m *Module) run(ctx context.Context) {
	defer close(m.done)
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.logger.Debug(m.cfg.Greeting, zap.String("module", string(ID)))
		}
	}
}

// Stop 取消周期 goroutine 并等待其退出，尊重 ctx 的 deadline。
// 未 Start 过（或 Start 之前失败）时调用为 no-op。
func (m *Module) Stop(ctx context.Context) error {
	if m.cancel == nil {
		return nil
	}
	m.cancel()
	// 先非阻塞探测 worker 是否已退出：当传入的 ctx 已取消/超时而 worker 其实
	// 已干净退出时，select 双臂就绪的随机选取可能把成功停止误报为失败（审计 B-1）。
	select {
	case <-m.done:
		m.logger.Info("hello module stopped", zap.String("module", string(ID)))
		return nil
	default:
	}
	select {
	case <-m.done:
		m.logger.Info("hello module stopped", zap.String("module", string(ID)))
		return nil
	case <-ctx.Done():
		return fmt.Errorf("hello: stop interrupted while waiting for worker exit: %w", ctx.Err())
	}
}
