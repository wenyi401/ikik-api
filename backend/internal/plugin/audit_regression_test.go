package plugin

// Phase-1.5 对抗式审计（2026-06-11）发现的 P2 问题回归测试：
//   A-1: 失败的 Build 不得被静默重跑（会对已 Provision 实例不 Stop 即覆盖）
//   C-1: enabled 空值（YAML null）按"未显式配置"处理，走模块默认值

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRuntimeBuildFailureBlocksRetry 固化审计 A-1 修复：
// Build 失败后，二次 Build 与 Start 均被拒绝，且不重新驱动任何生命周期。
func TestRuntimeBuildFailureBlocksRetry(t *testing.T) {
	log := &lifecycleLog{}
	okMod := &fakeModule{id: "job.a", enabledByDefault: true, log: log}
	badMod := &fakeModule{id: "job.b", enabledByDefault: true, log: log, provisionErr: errors.New("boom")}
	rt := newTestRuntime(t, Config{}, okMod, badMod)

	require.Error(t, rt.Build(), "首次 Build 应因 job.b Provision 失败而报错")
	firstLog := log.snapshot()

	err := rt.Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be rebuilt", "失败后的二次 Build 应被明确拒绝")
	require.Equal(t, firstLog, log.snapshot(), "二次 Build 不得重新 Provision 任何模块")

	require.Error(t, rt.Start(context.Background()), "Build 未成功时 Start 应被拒绝")
}

// TestRuntimeBuildSuccessThenRebuildStillRejected 确认成功路径的原有 guard 语义不变。
func TestRuntimeBuildSuccessThenRebuildStillRejected(t *testing.T) {
	rt := newTestRuntime(t, Config{}, &plainModule{id: "job.ok"})
	require.NoError(t, rt.Build())
	err := rt.Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "already built")
}

// TestParseConfigEnabledNilTreatedAsUnset 固化审计 C-1 修复：
// `enabled:`（YAML 空值 → nil）等价于未配置，三态语义回落到模块默认值，
// 且不影响同条目下私有配置的解析。
func TestParseConfigEnabledNilTreatedAsUnset(t *testing.T) {
	cfg, err := ParseConfig(map[string]map[string]any{
		"job.hello": {"enabled": nil, "greeting": "hi"},
	})
	require.NoError(t, err)

	mc, ok := cfg["job.hello"]
	require.True(t, ok)
	require.Nil(t, mc.Enabled, "enabled 空值应视为未显式配置")
	require.Equal(t, "hi", mc.Raw["greeting"], "私有配置不受 enabled 空值影响")

	require.True(t, cfg.enabledFor(ModuleInfo{ID: "job.hello", EnabledByDefault: true}))
	require.False(t, cfg.enabledFor(ModuleInfo{ID: "job.hello", EnabledByDefault: false}))
}
