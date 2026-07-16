package plugintest_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/plugin"
	"ikik-api/internal/plugin/plugintest"
)

// fixtureConfig 是 fakeModule 的私有配置，用于验证 raw 配置经 host.ConfigOf 抵达模块。
type fixtureConfig struct {
	Greeting string `mapstructure:"greeting"`
}

// fakeModule 是自测用模块：New 返回自身以便直接断言生命周期调用计数，
// 可按需注入各阶段错误。生命周期由 Runtime 在测试 goroutine 上同步驱动
// （cleanup 与 t.Run 完成均有 happens-before 保证），计数无需加锁。
type fakeModule struct {
	id               plugin.ModuleID
	enabledByDefault bool

	provisionErr error
	validateErr  error
	startErr     error
	stopErr      error

	provisions int
	validates  int
	starts     int
	stops      int
	host       *plugin.Host
	cfg        fixtureConfig
}

func (m *fakeModule) ModuleInfo() plugin.ModuleInfo {
	return plugin.ModuleInfo{
		ID:               m.id,
		EnabledByDefault: m.enabledByDefault,
		New:              func() plugin.Module { return m },
	}
}

func (m *fakeModule) Provision(host *plugin.Host) error {
	m.provisions++
	m.host = host
	if host != nil && host.ConfigOf != nil {
		if err := host.ConfigOf(m.id, &m.cfg); err != nil {
			return err
		}
	}
	return m.provisionErr
}

func (m *fakeModule) Validate() error { m.validates++; return m.validateErr }

func (m *fakeModule) Start(context.Context) error { m.starts++; return m.startErr }

func (m *fakeModule) Stop(context.Context) error { m.stops++; return m.stopErr }

// errFatalCalled 是 recordingTB.Fatalf 用于模拟"立即中止"语义的 panic 哨兵。
var errFatalCalled = errors.New("recordingTB: Fatalf called")

// recordingTB 包装真实 *testing.T，捕获夹具的 Fatalf / Errorf / Logf / Cleanup
// 调用，用于验证夹具在错误路径上不吞错。Fatalf 记录信息后 panic(errFatalCalled)
// 模拟真实 t.Fatalf 的中止语义，由 expectFatal 捕获恢复。
type recordingTB struct {
	testing.TB
	fatals   []string
	errs     []string
	logLines []string
	cleanups []func()
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Logf(format string, args ...any) {
	r.logLines = append(r.logLines, fmt.Sprintf(format, args...))
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
	panic(errFatalCalled)
}

func (r *recordingTB) Cleanup(f func()) { r.cleanups = append(r.cleanups, f) }

// runCleanups 按注册逆序（与真实 testing 一致）执行捕获的 cleanup。
func (r *recordingTB) runCleanups() {
	for i := len(r.cleanups) - 1; i >= 0; i-- {
		r.cleanups[i]()
	}
}

// expectFatal 执行 fn 并断言它经 recordingTB.Fatalf 中止，返回最后一条 fatal 信息。
func expectFatal(t *testing.T, rec *recordingTB, fn func()) (msg string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			if r != errFatalCalled {
				panic(r) // 非哨兵 panic 原样上抛
			}
			require.NotEmpty(t, rec.fatals)
			msg = rec.fatals[len(rec.fatals)-1]
		}
	}()
	fn()
	t.Fatal("expected the fixture to call Fatalf, but it returned normally")
	return ""
}

func TestNewHostDefaults(t *testing.T) {
	host := plugintest.NewHost(t)
	require.NotNil(t, host.Logger, "默认应提供 nop logger 而非 nil")
	require.NotNil(t, host.ConfigOf, "默认应提供空 ConfigOf 而非 nil")
	require.Nil(t, host.DB)
	require.Nil(t, host.Redis)

	// 空 ConfigOf：对任意模块 ID 不报错、不修改 out（模块默认值得以保留）。
	cfg := fixtureConfig{Greeting: "default"}
	require.NoError(t, host.ConfigOf("job.anything", &cfg))
	require.Equal(t, "default", cfg.Greeting)
}

func TestWithRedis(t *testing.T) {
	host := plugintest.NewHost(t, plugintest.WithRedis(t))
	require.NotNil(t, host.Redis)

	ctx := context.Background()
	require.NoError(t, host.Redis.Set(ctx, "k", "v", 0).Err())
	got, err := host.Redis.Get(ctx, "k").Result()
	require.NoError(t, err)
	require.Equal(t, "v", got)
}

func TestWithConfig(t *testing.T) {
	host := plugintest.NewHost(t, plugintest.WithConfig(map[string]map[string]any{
		"job.fixture": {"enabled": true, "greeting": "hi"},
	}))

	var cfg fixtureConfig
	require.NoError(t, host.ConfigOf("job.fixture", &cfg))
	require.Equal(t, "hi", cfg.Greeting, "私有配置应可经 ConfigOf 解码（enabled 键被内核剥离）")
}

func TestWithConfigParseFailureFatals(t *testing.T) {
	rec := &recordingTB{TB: t}
	msg := expectFatal(t, rec, func() {
		plugintest.NewHost(rec, plugintest.WithConfig(map[string]map[string]any{
			"INVALID ID": {},
		}))
	})
	require.Contains(t, msg, "parse modules config")
}

func TestWithObservedLogger(t *testing.T) {
	logOpt, logs := plugintest.WithObservedLogger()
	host := plugintest.NewHost(t, logOpt)

	host.Logger.Debug("observed-ping")
	require.Equal(t, 1, logs.FilterMessage("observed-ping").Len())
}

func TestRunLifecycleSuccessAndAutoStop(t *testing.T) {
	m := &fakeModule{id: "job.fixture"}
	t.Run("lifecycle", func(t *testing.T) {
		rt := plugintest.RunLifecycle(t, m, plugintest.NewHost(t), map[string]map[string]any{
			"job.fixture": {"enabled": true, "greeting": "hi"},
		})

		require.Equal(t, 1, m.provisions)
		require.Equal(t, 1, m.validates)
		require.Equal(t, 1, m.starts)
		require.Zero(t, m.stops)
		require.Equal(t, "hi", m.cfg.Greeting, "raw 配置应经 host.ConfigOf 抵达模块")
		require.NotNil(t, m.host)

		snap := rt.Snapshot()
		require.Len(t, snap, 1, "隔离注册表应只含被测模块")
		require.Equal(t, plugin.StateRunning, snap[0].State)
	})
	require.Equal(t, 1, m.stops, "t.Cleanup 应自动 Stop 模块")
}

func TestRunLifecycleRawOverridesHostConfig(t *testing.T) {
	m := &fakeModule{id: "job.fixture"}
	host := plugintest.NewHost(t, plugintest.WithConfig(map[string]map[string]any{
		"job.fixture": {"greeting": "from-with-config"},
	}))
	plugintest.RunLifecycle(t, m, host, map[string]map[string]any{
		"job.fixture": {"enabled": true, "greeting": "from-raw"},
	})
	require.Equal(t, "from-raw", m.cfg.Greeting,
		"RunLifecycle 的 raw 是配置唯一来源，应覆盖 WithConfig 的旧绑定")
}

func TestRunLifecycleNilHostUsesDefaultHost(t *testing.T) {
	m := &fakeModule{id: "job.fixture"}
	plugintest.RunLifecycle(t, m, nil, map[string]map[string]any{
		"job.fixture": {"enabled": true},
	})
	require.NotNil(t, m.host, "nil host 应被替换为默认 Host")
	require.NotNil(t, m.host.Logger)
	require.NotNil(t, m.host.ConfigOf)
}

func TestRunLifecycleExplicitStopThenCleanupIsIdempotent(t *testing.T) {
	m := &fakeModule{id: "job.fixture"}
	t.Run("lifecycle", func(t *testing.T) {
		rt := plugintest.RunLifecycle(t, m, nil, map[string]map[string]any{
			"job.fixture": {"enabled": true},
		})
		require.NoError(t, rt.Stop(context.Background()))
		require.Equal(t, 1, m.stops)
	})
	require.Equal(t, 1, m.stops, "测试已显式 Stop 时，cleanup 的二次 Stop 应为 no-op")
}

func TestRunLifecycleDisabledModuleIsNoop(t *testing.T) {
	m := &fakeModule{id: "job.fixture"} // EnabledByDefault=false 且 raw 未显式 enabled
	rt := plugintest.RunLifecycle(t, m, nil, nil)

	require.Zero(t, m.provisions, "disabled 模块不应经历任何生命周期（语义与生产一致，不强制 enable）")
	snap := rt.Snapshot()
	require.Len(t, snap, 1)
	require.False(t, snap[0].Enabled)
	require.Equal(t, plugin.StateRegistered, snap[0].State)
}

func TestBuildOnlyDoesNotStart(t *testing.T) {
	m := &fakeModule{id: "job.fixture"}
	t.Run("build", func(t *testing.T) {
		rt := plugintest.BuildOnly(t, m, nil, map[string]map[string]any{
			"job.fixture": {"enabled": true},
		})
		require.Equal(t, 1, m.provisions)
		require.Equal(t, 1, m.validates)
		require.Zero(t, m.starts)
		require.Equal(t, plugin.StateProvisioned, rt.Snapshot()[0].State)
	})
	require.Zero(t, m.stops, "未 Start 的模块在 cleanup 兜底 Stop 中不应被调用 Stop")
}

func TestBuildOnlyLogsDisabledHint(t *testing.T) {
	rec := &recordingTB{TB: t}
	m := &fakeModule{id: "job.fixture"}
	plugintest.BuildOnly(rec, m, nil, nil)

	require.NotEmpty(t, rec.logLines, "disabled 模块应输出提示日志")
	require.Contains(t, rec.logLines[0], "disabled")
	require.Contains(t, rec.logLines[0], `"job.fixture"`)
	rec.runCleanups()
	require.Empty(t, rec.errs)
}

func TestRunLifecycleBuildFailureFatalsWithModuleID(t *testing.T) {
	rec := &recordingTB{TB: t}
	m := &fakeModule{id: "job.fixture", enabledByDefault: true, provisionErr: errors.New("provision boom")}
	msg := expectFatal(t, rec, func() {
		plugintest.RunLifecycle(rec, m, nil, nil)
	})
	require.Contains(t, msg, `"job.fixture"`, "Build 失败信息必须包含模块 ID")
	require.Contains(t, msg, "provision boom")
}

func TestRunLifecycleStartFailureFatalsWithModuleID(t *testing.T) {
	rec := &recordingTB{TB: t}
	m := &fakeModule{id: "job.fixture", enabledByDefault: true, startErr: errors.New("start boom")}
	msg := expectFatal(t, rec, func() {
		plugintest.RunLifecycle(rec, m, nil, nil)
	})
	require.Contains(t, msg, `"job.fixture"`, "Start 失败信息必须包含模块 ID")
	require.Contains(t, msg, "start boom")

	// Start 失败的模块未进入 running，cleanup 兜底 Stop 应为无错误的 no-op。
	rec.runCleanups()
	require.Empty(t, rec.errs)
	require.Zero(t, m.stops)
}

func TestCleanupStopFailureReportedWithModuleID(t *testing.T) {
	rec := &recordingTB{TB: t}
	m := &fakeModule{id: "job.fixture", enabledByDefault: true, stopErr: errors.New("stop boom")}
	plugintest.RunLifecycle(rec, m, nil, nil)
	require.Empty(t, rec.fatals)
	require.NotEmpty(t, rec.cleanups)

	rec.runCleanups()
	require.Len(t, rec.errs, 1, "cleanup Stop 失败必须经 Errorf 报告（不吞错）")
	require.Contains(t, rec.errs[0], `"job.fixture"`)
	require.Contains(t, rec.errs[0], "stop boom")
}

func TestBuildExpectingErrorReturnsBuildError(t *testing.T) {
	m := &fakeModule{id: "job.fixture", enabledByDefault: true, validateErr: errors.New("validate boom")}
	err := plugintest.BuildExpectingError(t, m, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"job.fixture"`)
	require.Contains(t, err.Error(), "validate boom")
}

func TestBuildExpectingErrorFatalsOnUnexpectedSuccess(t *testing.T) {
	rec := &recordingTB{TB: t}
	m := &fakeModule{id: "job.fixture", enabledByDefault: true}
	msg := expectFatal(t, rec, func() {
		_ = plugintest.BuildExpectingError(rec, m, nil, nil)
	})
	require.Contains(t, msg, "expected Build")
	require.Contains(t, msg, `"job.fixture"`)
}
