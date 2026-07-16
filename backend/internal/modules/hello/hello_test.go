package hello

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/plugin"
	"ikik-api/internal/plugin/plugintest"
)

// TestModuleRegisteredInDefaultRegistry 验证 init() 已向包级默认注册表
// 注册 job.hello，且默认 disabled（EnabledByDefault=false）。
func TestModuleRegisteredInDefaultRegistry(t *testing.T) {
	info, ok := plugin.GetModule(ID)
	require.True(t, ok, "job.hello should self-register via init()")
	require.Equal(t, ID, info.ID)
	require.False(t, info.EnabledByDefault, "hello module must be disabled by default")
	require.NotNil(t, info.New)
	require.IsType(t, &Module{}, info.New())
}

// TestProvisionDefaults 验证未提供私有配置时 Provision 使用默认配置且可通过校验。
func TestProvisionDefaults(t *testing.T) {
	m := new(Module)
	require.NoError(t, m.Provision(plugintest.NewHost(t)))
	require.Equal(t, defaultConfig(), m.cfg)
	require.NoError(t, m.Validate())

	// nil host 防御路径：不 panic、使用默认配置。
	m2 := new(Module)
	require.NoError(t, m2.Provision(nil))
	require.Equal(t, defaultConfig(), m2.cfg)
	require.NoError(t, m2.Validate())
}

// TestProvisionDecodesPrivateConfig 验证 Provision 通过 host.ConfigOf
// 解码模块私有配置（interval 时长字符串、greeting 覆盖）。
func TestProvisionDecodesPrivateConfig(t *testing.T) {
	host := plugintest.NewHost(t, plugintest.WithConfig(map[string]map[string]any{
		"job.hello": {"enabled": true, "interval": "15ms", "greeting": "hi"},
	}))

	m := new(Module)
	require.NoError(t, m.Provision(host))
	require.Equal(t, 15*time.Millisecond, m.cfg.Interval)
	require.Equal(t, "hi", m.cfg.Greeting)
	require.NoError(t, m.Validate())
}

// TestProvisionConfigDecodeError 验证私有配置类型错误时 Provision 返回错误。
func TestProvisionConfigDecodeError(t *testing.T) {
	host := plugintest.NewHost(t, plugintest.WithConfig(map[string]map[string]any{
		"job.hello": {"interval": "not-a-duration"},
	}))

	m := new(Module)
	err := m.Provision(host)
	require.Error(t, err)
	require.Contains(t, err.Error(), "job.hello")
}

// TestValidateRejectsInvalidConfig 验证非法配置被 Validate 拒绝。
func TestValidateRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"zero interval", Config{Interval: 0, Greeting: "hi"}, "interval must be positive"},
		{"negative interval", Config{Interval: -time.Second, Greeting: "hi"}, "interval must be positive"},
		{"empty greeting", Config{Interval: time.Second, Greeting: ""}, "greeting must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Module{cfg: tt.cfg}
			err := m.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestStartPeriodicLogAndStop 验证 Start 后周期 debug 日志持续出现，
// Stop 后 goroutine 优雅退出。
func TestStartPeriodicLogAndStop(t *testing.T) {
	logOpt, logs := plugintest.WithObservedLogger()
	host := plugintest.NewHost(t, logOpt, plugintest.WithConfig(map[string]map[string]any{
		"job.hello": {"interval": "5ms"},
	}))

	m := new(Module)
	require.NoError(t, m.Provision(host))
	require.NoError(t, m.Validate())
	require.NoError(t, m.Start(context.Background()))

	require.Eventually(t, func() bool {
		return logs.FilterMessage(m.cfg.Greeting).Len() >= 2
	}, 2*time.Second, 5*time.Millisecond, "periodic debug log should appear repeatedly")

	require.NoError(t, m.Stop(context.Background()))
	select {
	case <-m.done:
		// goroutine 已退出。
	default:
		t.Fatal("worker goroutine should have exited after Stop")
	}
	require.Equal(t, 1, logs.FilterMessage("hello module stopped").Len())
}

// TestStopWithoutStartIsNoop 验证未 Start 时 Stop 为 no-op。
func TestStopWithoutStartIsNoop(t *testing.T) {
	m := new(Module)
	require.NoError(t, m.Provision(plugintest.NewHost(t)))
	require.NoError(t, m.Stop(context.Background()))
}

// TestRuntimeDefaultConfigIsNoop 验证缺省配置（modules: 子树为空）下，
// hello 模块保持 disabled：Build/Start/Stop 全程 no-op，无任何副作用。
func TestRuntimeDefaultConfigIsNoop(t *testing.T) {
	logOpt, logs := plugintest.WithObservedLogger()
	rt := plugintest.RunLifecycle(t, &Module{}, plugintest.NewHost(t, logOpt), nil)

	require.NoError(t, rt.Stop(context.Background()))

	snap := rt.Snapshot()
	require.Len(t, snap, 1)
	require.Equal(t, ID, snap[0].ID)
	require.False(t, snap[0].Enabled)
	require.Equal(t, plugin.StateRegistered, snap[0].State)
	require.Zero(t, logs.Len(), "disabled module must not produce any module logs")
}

// TestRuntimeEnabledHelloLifecycle 验证启用 job.hello 后完整生命周期：
// Build（Provision/Validate）→ Start（周期日志出现）→ Stop（优雅退出）。
func TestRuntimeEnabledHelloLifecycle(t *testing.T) {
	logOpt, logs := plugintest.WithObservedLogger()
	rt := plugintest.RunLifecycle(t, &Module{}, plugintest.NewHost(t, logOpt), map[string]map[string]any{
		"job.hello": {"enabled": true, "interval": "5ms"},
	})

	require.Eventually(t, func() bool {
		return logs.FilterMessage(defaultConfig().Greeting).Len() >= 1
	}, 2*time.Second, 5*time.Millisecond, "periodic debug log should appear after Start")

	require.NoError(t, rt.Stop(context.Background()))
	snap := rt.Snapshot()
	require.Len(t, snap, 1)
	require.True(t, snap[0].Enabled)
	require.Equal(t, plugin.StateStopped, snap[0].State)
}

// TestRuntimeInvalidHelloConfigAbortsBuild 验证 hello 配置非法时
// Validate 失败导致 Build 返回错误（启动中止，fail-fast）。
func TestRuntimeInvalidHelloConfigAbortsBuild(t *testing.T) {
	err := plugintest.BuildExpectingError(t, &Module{}, plugintest.NewHost(t), map[string]map[string]any{
		"job.hello": {"enabled": true, "interval": "-1s"},
	})
	require.Contains(t, err.Error(), `validate module "job.hello"`)
	require.Contains(t, err.Error(), "interval must be positive")
}

// TestRuntimeIgnoresUnknownConfiguredModule 验证配置中出现格式合法但
// 未注册的模块 ID 时，Runtime 按内核语义忽略该项，不影响其他模块。
func TestRuntimeIgnoresUnknownConfiguredModule(t *testing.T) {
	rt := plugintest.RunLifecycle(t, &Module{}, plugintest.NewHost(t), map[string]map[string]any{
		"job.hello":   {"enabled": true, "interval": "5ms"},
		"job.unknown": {"enabled": true},
	})

	require.NoError(t, rt.Stop(context.Background()))

	snap := rt.Snapshot()
	require.Len(t, snap, 1, "snapshot should only contain registered modules")
	require.Equal(t, ID, snap[0].ID)
}
