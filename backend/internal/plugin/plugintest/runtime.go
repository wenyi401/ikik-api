package plugintest

import (
	"context"
	"testing"
	"time"

	"ikik-api/internal/plugin"
)

// stopTimeout 是 t.Cleanup 自动 Stop 的超时上限：模块 Stop 悬挂时，
// 测试以包含模块 ID 的明确错误失败，而不是吊死整个 go test 进程。
const stopTimeout = 10 * time.Second

// RunLifecycle 驱动单个模块的完整生命周期：在隔离注册表中注册 m →
// 解析 raw 配置 → Build（Provision / Validate）→ Start。任一步失败立即
// tb.Fatal（错误信息含模块 ID）。测试结束时 t.Cleanup 自动 Stop 并断言无
// 错误；测试中途已显式 rt.Stop 时，清理阶段的二次 Stop 为 no-op
// （Runtime.Stop 只停 running 模块，幂等）。
//
// 语义说明：
//   - raw 与全局配置 `modules:` 子树同形状，是模块配置的唯一来源
//     （host 上已有的 ConfigOf——包括 WithConfig 设置的——会被重绑定为
//     raw 的解析结果）；nil 表示空配置；
//   - enabled 语义与生产完全一致、不做任何强制：raw 未显式 enabled 且模块
//     默认 disabled 时，Build / Start 全程 no-op（此时会 tb.Logf 提示，
//     便于排查"忘了写 enabled: true"）；
//   - host 为 nil 时等价于 NewHost(tb) 的默认 Host；
//   - Runtime 经 ModuleInfo().New 创建实例驱动生命周期——除非模块的 New
//     返回自身（fake 模块常如此），传入的 m 仅用于注册，生命周期效果应
//     通过日志（WithObservedLogger）与返回 Runtime 的 Snapshot 断言，
//     而不是 m 的内部字段。
//
// 需要断言"Build 应失败"用 BuildExpectingError；更复杂的场景（Start 失败
// 回滚、多模块编排等）直接使用内核 API（plugin.NewRegistry +
// plugin.NewRuntimeWithRegistry），内核 API 保持完全公开。
func RunLifecycle(tb testing.TB, m plugin.Module, host *plugin.Host, raw map[string]map[string]any) *plugin.Runtime {
	tb.Helper()
	rt, id := buildOnly(tb, m, host, raw)
	if err := rt.Start(context.Background()); err != nil {
		tb.Fatalf("plugintest: start module %q: %v", id, err)
	}
	return rt
}

// BuildOnly 与 RunLifecycle 相同但停在 Build 之后（不 Start），用于只验证
// Provision / Validate 行为的测试。返回的 Runtime 可由测试自行 Start；
// 无论是否 Start，t.Cleanup 都会执行兜底 Stop（未启动时为 no-op）并断言无错误。
func BuildOnly(tb testing.TB, m plugin.Module, host *plugin.Host, raw map[string]map[string]any) *plugin.Runtime {
	tb.Helper()
	rt, _ := buildOnly(tb, m, host, raw)
	return rt
}

// BuildExpectingError 与 BuildOnly 同样装配 Runtime，但期望 Build 失败：
// Build 意外成功时 tb.Fatal；失败时返回该错误供测试断言内容
// （内核保证 Build 错误信息包含失败模块 ID）。
//
// 设计说明：单独提供本变体而不是给 RunLifecycle 加 expectError 参数，
// 是为了让成功 / 失败两种期望在调用处一目了然，且失败路径不注册多余的
// cleanup（Build 失败后无 running 模块，无需 Stop）。raw 解析失败仍然
// tb.Fatal——配置字面量笔误属于测试自身缺陷，不属于"期望中的 Build 失败"。
// "Start 应失败"的场景本包不提供夹具（hello 与内核测试均无此样板复用需求，
// 避免为假设需求预铺 API），请直接使用内核 API。
func BuildExpectingError(tb testing.TB, m plugin.Module, host *plugin.Host, raw map[string]map[string]any) error {
	tb.Helper()
	rt, id := newRuntime(tb, m, host, raw)
	err := rt.Build()
	if err == nil {
		tb.Fatalf("plugintest: expected Build of module %q to fail, but it succeeded", id)
	}
	return err
}

// buildOnly 是 RunLifecycle / BuildOnly 的共享实现：装配 Runtime、执行 Build、
// 输出 disabled 提示并注册 t.Cleanup 兜底 Stop。
func buildOnly(tb testing.TB, m plugin.Module, host *plugin.Host, raw map[string]map[string]any) (*plugin.Runtime, plugin.ModuleID) {
	tb.Helper()
	rt, id := newRuntime(tb, m, host, raw)
	if err := rt.Build(); err != nil {
		tb.Fatalf("plugintest: build module %q: %v", id, err)
	}
	for _, status := range rt.Snapshot() {
		if status.ID == id && !status.Enabled {
			tb.Logf("plugintest: module %q resolved to disabled under the given config; its lifecycle is a no-op (set \"enabled\": true in raw to run it)", id)
		}
	}
	tb.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		defer cancel()
		if err := rt.Stop(ctx); err != nil {
			tb.Errorf("plugintest: stop module %q during cleanup: %v", id, err)
		}
	})
	return rt, id
}

// newRuntime 完成各夹具入口共享的装配：隔离注册表注册 m、解析 raw 配置、
// 将 host.ConfigOf 重绑定为解析结果、创建 Runtime。
func newRuntime(tb testing.TB, m plugin.Module, host *plugin.Host, raw map[string]map[string]any) (*plugin.Runtime, plugin.ModuleID) {
	tb.Helper()
	if m == nil {
		tb.Fatalf("plugintest: module under test is nil")
	}
	id := m.ModuleInfo().ID
	cfg, err := plugin.ParseConfig(raw)
	if err != nil {
		tb.Fatalf("plugintest: parse modules config for module %q: %v", id, err)
	}
	if host == nil {
		host = NewHost(tb)
	}
	// raw 是生命周期夹具的唯一配置来源：统一重绑定 ConfigOf，避免 host 上的
	// 旧绑定（NewHost 默认空配置 / WithConfig）与 Runtime 实际持有的 cfg 不一致。
	host.ConfigOf = cfg.Of

	registry := plugin.NewRegistry()
	registry.RegisterModule(m) // 注册非法（ID 格式等）时按内核语义 panic，测试直接失败
	return plugin.NewRuntimeWithRegistry(host, cfg, registry), id
}
