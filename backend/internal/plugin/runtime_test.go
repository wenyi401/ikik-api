package plugin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// lifecycleLog 记录全部 fake 模块的生命周期调用序列（"模块ID:阶段"）。
type lifecycleLog struct {
	mu     sync.Mutex
	events []string
}

func (l *lifecycleLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *lifecycleLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

// fakeModule 是实现全部可选生命周期接口的测试模块，按配置注入各阶段错误。
type fakeModule struct {
	id               ModuleID
	enabledByDefault bool
	log              *lifecycleLog

	provisionErr error
	validateErr  error
	startErr     error
	stopErr      error
	stopFn       func(ctx context.Context) error // 非 nil 时覆盖默认 Stop 行为

	host *Host // Provision 时记录，供断言
}

func (m *fakeModule) ModuleInfo() ModuleInfo {
	return ModuleInfo{
		ID:               m.id,
		EnabledByDefault: m.enabledByDefault,
		New:              func() Module { return m },
	}
}

func (m *fakeModule) Provision(host *Host) error {
	m.host = host
	m.log.add(string(m.id) + ":provision")
	return m.provisionErr
}

func (m *fakeModule) Validate() error {
	m.log.add(string(m.id) + ":validate")
	return m.validateErr
}

func (m *fakeModule) Start(ctx context.Context) error {
	m.log.add(string(m.id) + ":start")
	return m.startErr
}

func (m *fakeModule) Stop(ctx context.Context) error {
	m.log.add(string(m.id) + ":stop")
	if m.stopFn != nil {
		return m.stopFn(ctx)
	}
	return m.stopErr
}

// plainModule 只实现 Module（无任何生命周期接口）。
type plainModule struct {
	id ModuleID
}

func (m *plainModule) ModuleInfo() ModuleInfo {
	return ModuleInfo{
		ID:               m.id,
		EnabledByDefault: true,
		New:              func() Module { return m },
	}
}

// newTestRuntime 构造隔离注册表 + Runtime。
func newTestRuntime(t *testing.T, cfg Config, mods ...Module) *Runtime {
	t.Helper()
	reg := NewRegistry()
	for _, m := range mods {
		reg.RegisterModule(m)
	}
	return NewRuntimeWithRegistry(&Host{}, cfg, reg)
}

func statusByID(t *testing.T, statuses []ModuleStatus, id ModuleID) ModuleStatus {
	t.Helper()
	for _, s := range statuses {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("module %q not found in snapshot", id)
	return ModuleStatus{}
}

func TestRuntimeBuildLifecycleOrderAndEnabledTriState(t *testing.T) {
	log := &lifecycleLog{}
	on, off := true, false
	cfg := Config{
		"job.c-default-off-enabled": {Enabled: &on},
		"job.d-default-on-disabled": {Enabled: &off},
	}
	rt := newTestRuntime(t, cfg,
		// 故意乱序注册，验证 Build 按 ID 字典序执行。
		&fakeModule{id: "job.b-default-on", enabledByDefault: true, log: log},
		&fakeModule{id: "job.a-default-on", enabledByDefault: true, log: log},
		&fakeModule{id: "job.c-default-off-enabled", enabledByDefault: false, log: log},
		&fakeModule{id: "job.d-default-on-disabled", enabledByDefault: true, log: log},
		&fakeModule{id: "job.e-default-off", enabledByDefault: false, log: log},
	)

	require.NoError(t, rt.Build())

	// enabled 三态：默认 on（a、b）、显式 enable 默认 off 的 c、显式 disable 默认 on 的 d、默认 off 的 e。
	require.Equal(t, []string{
		"job.a-default-on:provision", "job.a-default-on:validate",
		"job.b-default-on:provision", "job.b-default-on:validate",
		"job.c-default-off-enabled:provision", "job.c-default-off-enabled:validate",
	}, log.snapshot())

	statuses := rt.Snapshot()
	require.Equal(t, StateProvisioned, statusByID(t, statuses, "job.a-default-on").State)
	require.Equal(t, StateProvisioned, statusByID(t, statuses, "job.c-default-off-enabled").State)
	require.False(t, statusByID(t, statuses, "job.d-default-on-disabled").Enabled)
	require.Equal(t, StateRegistered, statusByID(t, statuses, "job.d-default-on-disabled").State)
	require.False(t, statusByID(t, statuses, "job.e-default-off").Enabled)
	require.Equal(t, StateRegistered, statusByID(t, statuses, "job.e-default-off").State)
}

func TestRuntimeBuildProvisionFailureAborts(t *testing.T) {
	log := &lifecycleLog{}
	provisionErr := errors.New("provision boom")
	rt := newTestRuntime(t, Config{},
		&fakeModule{id: "job.a", enabledByDefault: true, log: log},
		&fakeModule{id: "job.b", enabledByDefault: true, log: log, provisionErr: provisionErr},
		&fakeModule{id: "job.c", enabledByDefault: true, log: log},
	)

	err := rt.Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "job.b", "Build 错误必须包含失败模块 ID")
	require.ErrorIs(t, err, provisionErr)

	// 失败即中止：job.c 不会被 Provision；job.b 的 Validate 不会执行。
	require.Equal(t, []string{
		"job.a:provision", "job.a:validate",
		"job.b:provision",
	}, log.snapshot())

	statuses := rt.Snapshot()
	require.Equal(t, StateProvisioned, statusByID(t, statuses, "job.a").State)
	bStatus := statusByID(t, statuses, "job.b")
	require.Equal(t, StateErrored, bStatus.State)
	require.Contains(t, bStatus.Err, "provision boom")
	require.Equal(t, StateRegistered, statusByID(t, statuses, "job.c").State)

	// Build 失败后 Start 必须被拒绝。
	require.Error(t, rt.Start(context.Background()))
}

func TestRuntimeBuildValidateFailureAborts(t *testing.T) {
	log := &lifecycleLog{}
	rt := newTestRuntime(t, Config{},
		&fakeModule{id: "job.a", enabledByDefault: true, log: log, validateErr: errors.New("validate boom")},
		&fakeModule{id: "job.b", enabledByDefault: true, log: log},
	)

	err := rt.Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "job.a")
	require.Equal(t, []string{"job.a:provision", "job.a:validate"}, log.snapshot())
	require.Equal(t, StateErrored, statusByID(t, rt.Snapshot(), "job.a").State)
}

func TestRuntimeStartOrderAndStopReverseOrder(t *testing.T) {
	log := &lifecycleLog{}
	rt := newTestRuntime(t, Config{},
		&fakeModule{id: "job.b", enabledByDefault: true, log: log},
		&fakeModule{id: "job.a", enabledByDefault: true, log: log},
		&fakeModule{id: "job.c", enabledByDefault: true, log: log},
	)
	require.NoError(t, rt.Build())
	require.NoError(t, rt.Start(context.Background()))

	for _, s := range rt.Snapshot() {
		require.Equal(t, StateRunning, s.State)
	}

	require.NoError(t, rt.Stop(context.Background()))

	require.Equal(t, []string{
		"job.a:provision", "job.a:validate",
		"job.b:provision", "job.b:validate",
		"job.c:provision", "job.c:validate",
		"job.a:start", "job.b:start", "job.c:start",
		"job.c:stop", "job.b:stop", "job.a:stop", // 逆序关闭
	}, log.snapshot())

	for _, s := range rt.Snapshot() {
		require.Equal(t, StateStopped, s.State)
	}
}

func TestRuntimeStartHalfwayFailureRollsBackInReverse(t *testing.T) {
	log := &lifecycleLog{}
	startErr := errors.New("start boom")
	rt := newTestRuntime(t, Config{},
		&fakeModule{id: "job.a", enabledByDefault: true, log: log},
		&fakeModule{id: "job.b", enabledByDefault: true, log: log},
		&fakeModule{id: "job.c", enabledByDefault: true, log: log, startErr: startErr},
		&fakeModule{id: "job.d", enabledByDefault: true, log: log},
	)
	require.NoError(t, rt.Build())

	err := rt.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "job.c", "Start 错误必须包含失败模块 ID")
	require.ErrorIs(t, err, startErr)

	// a、b 已启动；c 启动失败；回滚必须按 b → a 逆序 Stop；d 不得被 Start。
	require.Equal(t, []string{
		"job.a:provision", "job.a:validate",
		"job.b:provision", "job.b:validate",
		"job.c:provision", "job.c:validate",
		"job.d:provision", "job.d:validate",
		"job.a:start", "job.b:start", "job.c:start",
		"job.b:stop", "job.a:stop",
	}, log.snapshot())

	statuses := rt.Snapshot()
	require.Equal(t, StateStopped, statusByID(t, statuses, "job.a").State)
	require.Equal(t, StateStopped, statusByID(t, statuses, "job.b").State)
	cStatus := statusByID(t, statuses, "job.c")
	require.Equal(t, StateErrored, cStatus.State)
	require.Contains(t, cStatus.Err, "start boom")
	require.Equal(t, StateProvisioned, statusByID(t, statuses, "job.d").State)
}

func TestRuntimeStopToleratesSingleFailure(t *testing.T) {
	log := &lifecycleLog{}
	stopErr := errors.New("stop boom")
	rt := newTestRuntime(t, Config{},
		&fakeModule{id: "job.a", enabledByDefault: true, log: log},
		&fakeModule{id: "job.b", enabledByDefault: true, log: log, stopErr: stopErr},
		&fakeModule{id: "job.c", enabledByDefault: true, log: log},
	)
	require.NoError(t, rt.Build())
	require.NoError(t, rt.Start(context.Background()))

	err := rt.Stop(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, stopErr)
	require.Contains(t, err.Error(), "job.b")

	// 单个失败不中断其余模块关闭：c → b（失败）→ a 全部被调用。
	stops := log.snapshot()[9:]
	require.Equal(t, []string{"job.c:stop", "job.b:stop", "job.a:stop"}, stops)

	statuses := rt.Snapshot()
	require.Equal(t, StateStopped, statusByID(t, statuses, "job.a").State)
	require.Equal(t, StateErrored, statusByID(t, statuses, "job.b").State)
	require.Contains(t, statusByID(t, statuses, "job.b").Err, "stop boom")
	require.Equal(t, StateStopped, statusByID(t, statuses, "job.c").State)
}

func TestRuntimeStopRespectsContextDeadline(t *testing.T) {
	log := &lifecycleLog{}
	blocking := &fakeModule{
		id: "job.blocking", enabledByDefault: true, log: log,
		stopFn: func(ctx context.Context) error {
			<-ctx.Done() // 模拟尊重 ctx deadline 的慢关闭
			return ctx.Err()
		},
	}
	rt := newTestRuntime(t, Config{}, blocking,
		&fakeModule{id: "job.a", enabledByDefault: true, log: log},
	)
	require.NoError(t, rt.Build())
	require.NoError(t, rt.Start(context.Background()))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	begin := time.Now()
	err := rt.Stop(ctx)
	require.Less(t, time.Since(begin), 2*time.Second, "Stop 必须随 ctx deadline 返回，不得无限阻塞")
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// 即使 blocking 超时失败，job.a 仍被关闭（容错继续）。
	require.Equal(t, StateStopped, statusByID(t, rt.Snapshot(), "job.a").State)
}

func TestRuntimeModuleWithoutLifecycleInterfaces(t *testing.T) {
	rt := newTestRuntime(t, Config{}, &plainModule{id: "job.plain"})
	require.NoError(t, rt.Build())
	require.NoError(t, rt.Start(context.Background()))
	require.Equal(t, StateRunning, statusByID(t, rt.Snapshot(), "job.plain").State,
		"未实现 Starter 的 enabled 模块在 Start 后视为 running")
	require.NoError(t, rt.Stop(context.Background()))
	require.Equal(t, StateStopped, statusByID(t, rt.Snapshot(), "job.plain").State)
}

func TestRuntimeGuards(t *testing.T) {
	rt := newTestRuntime(t, Config{})
	require.Error(t, rt.Start(context.Background()), "未 Build 不得 Start")
	require.NoError(t, rt.Build())
	require.Error(t, rt.Build(), "Build 只能调用一次")
	require.NoError(t, rt.Start(context.Background()))
	require.Error(t, rt.Start(context.Background()), "Start 只能调用一次")
}

func TestRuntimeSnapshotBeforeBuild(t *testing.T) {
	log := &lifecycleLog{}
	off := false
	rt := newTestRuntime(t, Config{"job.b": {Enabled: &off}},
		&fakeModule{id: "job.b", enabledByDefault: true, log: log},
		&fakeModule{id: "job.a", enabledByDefault: true, log: log},
	)

	statuses := rt.Snapshot()
	require.Len(t, statuses, 2)
	require.Equal(t, ModuleID("job.a"), statuses[0].ID, "Snapshot 按 ID 字典序")
	require.Equal(t, ModuleID("job.b"), statuses[1].ID)
	for _, s := range statuses {
		require.Equal(t, StateRegistered, s.State)
		require.Empty(t, s.Err)
	}
	require.True(t, statuses[0].Enabled)
	require.False(t, statuses[1].Enabled, "Build 前 Snapshot 也要正确解析 enabled 三态")
}

func TestRuntimeHostConfigOfDefaultBinding(t *testing.T) {
	log := &lifecycleLog{}
	cfg, err := ParseConfig(map[string]map[string]any{
		"job.hello": {"enabled": true, "greeting": "from-config"},
	})
	require.NoError(t, err)

	mod := &fakeModule{id: "job.hello", log: log}
	rt := newTestRuntime(t, cfg, mod)
	require.NoError(t, rt.Build())

	// NewRuntime 未显式装配 ConfigOf 时默认绑定到自身 Config.Of。
	require.NotNil(t, mod.host)
	require.NotNil(t, mod.host.ConfigOf)
	var out helloModuleConfig
	require.NoError(t, mod.host.ConfigOf("job.hello", &out))
	require.Equal(t, "from-config", out.Greeting)
}

// 审计 B2 修复的回归测试：Start 半途失败触发回滚时，若已启动模块的回滚 Stop
// 也失败，该失败必须并入 Start 的返回错误（而不是只记日志被吞掉）。
func TestRuntimeStartRollbackStopFailureJoinedIntoError(t *testing.T) {
	log := &lifecycleLog{}
	stopBoom := errors.New("stop-a-boom")
	rt := newTestRuntime(t, Config{},
		&fakeModule{id: "job.a", enabledByDefault: true, log: log, stopErr: stopBoom},
		&fakeModule{id: "job.b", enabledByDefault: true, log: log, startErr: errors.New("start-b-boom")},
	)
	require.NoError(t, rt.Build())

	err := rt.Start(context.Background())
	require.Error(t, err)
	// 启动失败与回滚失败都必须在返回错误中可见（errors.Join 可穿透）。
	require.ErrorIs(t, err, stopBoom)
	require.Contains(t, err.Error(), `start module "job.b"`)
	require.Contains(t, err.Error(), `stop module "job.a"`)

	statuses := rt.Snapshot()
	require.Equal(t, StateErrored, statusByID(t, statuses, "job.a").State, "回滚失败的模块应为 errored")
	require.Equal(t, StateErrored, statusByID(t, statuses, "job.b").State)
}
