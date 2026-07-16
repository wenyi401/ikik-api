// Package plugintest 提供插件模块的测试夹具（test fixtures）。
//
// 本包是模块作者的测试入口：NewHost 一行构造可按需附带 miniredis、
// observed logger、配置子树的宿主能力面；RunLifecycle 一行驱动
// "隔离注册 → 解析配置 → Build → Start → t.Cleanup 自动 Stop" 的完整
// 生命周期。命名与用法风格对齐标准库 net/http/httptest。
//
// 使用约束：本包 import "testing"，仅供 _test.go 文件 import，
// 禁止进入任何生产代码的依赖面。该约束以本声明 + 代码评审人工约定保证
// （未配置 depguard 规则；若被非 _test 文件 import，testing 包会被
// 连带编入生产二进制，评审时一票否决）。
//
// 夹具纪律：夹具自身绝不吞错——Build/Start/Stop 或配置解析失败一律经
// tb.Fatalf / tb.Errorf 报告，且错误信息包含被测模块 ID。
package plugintest

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"ikik-api/internal/plugin"
)

// Option 定制 NewHost 构造的 Host。Option 在 NewHost 内部被依次应用，
// tb 即传给 NewHost 的测试句柄；Option 内部的失败一律经 tb 报告（不吞错）。
type Option func(tb testing.TB, h *plugin.Host)

// NewHost 构造模块测试用的宿主能力面，默认零依赖即可使用：
// nop logger、空 ConfigOf（对任何模块 ID 都不报错、不修改 out）、
// nil DB、nil Redis。按需叠加 WithRedis / WithConfig / WithObservedLogger。
//
// Host 是纯数据结构：本包未覆盖的少见场景（如注入 enttest 客户端、
// 自定义 logger）可在 NewHost 返回后直接对字段赋值，无需新增 Option。
func NewHost(tb testing.TB, opts ...Option) *plugin.Host {
	tb.Helper()
	host := &plugin.Host{
		Logger:   zap.NewNop(),
		ConfigOf: plugin.Config{}.Of,
	}
	for _, opt := range opts {
		opt(tb, host)
	}
	return host
}

// WithRedis 为 Host 附带一个由 miniredis 支撑的 go-redis 客户端。
//
// miniredis 实例与 redis 客户端的生命周期均挂在 tb.Cleanup 上自动清理；
// tb 通常就是传给 NewHost 的同一个测试句柄。
func WithRedis(tb testing.TB) Option {
	return func(_ testing.TB, h *plugin.Host) {
		tb.Helper()
		mr := miniredis.RunT(tb)
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		tb.Cleanup(func() {
			if err := rdb.Close(); err != nil {
				tb.Errorf("plugintest: close redis client: %v", err)
			}
		})
		h.Redis = rdb
	}
}

// WithConfig 将 Host.ConfigOf 绑定为 raw（与全局配置 `modules:` 子树同形状）
// 的解析结果，供直接调用 Provision 等不经 Runtime 的测试读取模块私有配置。
// raw 解析失败（模块 ID 格式非法、enabled 类型错误）立即 tb.Fatal。
//
// 注意：经 RunLifecycle / BuildOnly 驱动生命周期时，模块配置以其 raw 参数
// 为唯一来源（会覆盖本 Option 设置的 ConfigOf），两者不要混用。
func WithConfig(raw map[string]map[string]any) Option {
	return func(tb testing.TB, h *plugin.Host) {
		tb.Helper()
		cfg, err := plugin.ParseConfig(raw)
		if err != nil {
			tb.Fatalf("plugintest: parse modules config: %v", err)
		}
		h.ConfigOf = cfg.Of
	}
}

// WithObservedLogger 将 Host.Logger 替换为 debug 级别的 zap observer logger，
// 并把可断言日志记录的句柄作为第二返回值一并返回：
//
//	logOpt, logs := plugintest.WithObservedLogger()
//	host := plugintest.NewHost(t, logOpt)
//	// ... 驱动模块 ...
//	require.Equal(t, 1, logs.FilterMessage("hello").Len())
//
// 设计说明：observer 句柄随 Option 直接返回，而非提供 Logs(host) 式的查表
// 函数——调用方一行解构即可拿到句柄，包内也无需维护 Host → observer 的
// 映射状态（与 PHASE_PLAN 草案中 Logs(t) 形态的差异已在任务交付说明登记）。
func WithObservedLogger() (Option, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	return func(_ testing.TB, h *plugin.Host) {
		h.Logger = logger
	}, logs
}
