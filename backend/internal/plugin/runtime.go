package plugin

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// State 是模块在 Runtime 生命周期中的状态。
type State string

// 模块生命周期状态。状态机：
//
//	registered ──Build──▶ provisioned ──Start──▶ running ──Stop──▶ stopped
//	     │（disabled 模块停留在 registered）            任一阶段失败 ──▶ errored
const (
	// StateRegistered 模块已注册但未实例化（disabled 模块始终处于该状态）。
	StateRegistered State = "registered"
	// StateProvisioned 模块已实例化并通过 Provision/Validate，等待启动。
	StateProvisioned State = "provisioned"
	// StateRunning 模块已启动（未实现 Starter 的 enabled 模块在 Start 阶段也进入该状态）。
	StateRunning State = "running"
	// StateStopped 模块已正常停止。
	StateStopped State = "stopped"
	// StateErrored 模块在某个生命周期阶段失败，错误见 ModuleStatus.Err。
	StateErrored State = "errored"
)

// ModuleStatus 是单个模块的状态快照（admin 可观测用）。
type ModuleStatus struct {
	// ID 模块 ID。
	ID ModuleID
	// Enabled 模块最终的启停状态（显式配置优先，否则为注册声明的默认值）。
	Enabled bool
	// State 模块当前生命周期状态。
	State State
	// Err 模块最近一次生命周期失败的错误信息，无错误时为空字符串。
	Err string
}

// moduleRecord 是 Runtime 内部维护的单模块运行时记录。
type moduleRecord struct {
	info     ModuleInfo
	enabled  bool
	instance Module
	state    State
	err      error
}

// Runtime 是模块生命周期驱动器：按注册表稳定序（ID 字典序）实例化 enabled
// 模块并驱动 Provision → Validate → Start → Stop（逆序）生命周期。
//
// 使用方式（单次使用，不支持 Stop 后重新 Start）：
//
//	rt := plugin.NewRuntime(host, cfg)
//	if err := rt.Build(); err != nil { ... }   // 启动中止
//	if err := rt.Start(ctx); err != nil { ... } // 已自动逆序回滚
//	defer rt.Stop(shutdownCtx)                  // 接入既有 cleanup 序列
//
// 所有方法并发安全。
type Runtime struct {
	mu       sync.Mutex
	host     *Host
	config   Config
	registry *Registry
	logger   *zap.Logger

	built          bool
	buildAttempted bool
	started        bool
	records        []*moduleRecord
}

// NewRuntime 创建基于包级默认注册表的 Runtime。
//
// host 为 nil 时使用空 Host；若 host.ConfigOf 未装配，则默认绑定为 cfg.Of，
// 使模块在 Provision 中总能读取到自己的私有配置。
func NewRuntime(host *Host, cfg Config) *Runtime {
	return NewRuntimeWithRegistry(host, cfg, defaultRegistry)
}

// NewRuntimeWithRegistry 创建基于指定注册表的 Runtime（测试隔离用）。
// 其余语义与 NewRuntime 一致。
func NewRuntimeWithRegistry(host *Host, cfg Config, registry *Registry) *Runtime {
	if host == nil {
		host = &Host{}
	}
	if cfg == nil {
		cfg = Config{}
	}
	if registry == nil {
		registry = defaultRegistry
	}
	if host.ConfigOf == nil {
		host.ConfigOf = cfg.Of
	}
	logger := host.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Runtime{
		host:     host,
		config:   cfg,
		registry: registry,
		logger:   logger,
	}
}

// Build 按注册表稳定序实例化全部 enabled 模块并依次执行 Provision、Validate。
//
// 任一模块失败立即返回包含模块 ID 的错误（启动中止），不会继续处理后续模块；
// 已成功 Provision 的模块保持 provisioned 状态，由进程启动失败路径整体退出。
// Build 只能调用一次：无论成败，第二次调用一律拒绝——失败的 Build 不可重试
// （重试会对已 Provision 的旧实例不 Stop 即覆盖，造成资源泄漏），应整体重启进程。
func (r *Runtime) Build() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.buildAttempted {
		if r.built {
			return errors.New("plugin: runtime already built")
		}
		return errors.New("plugin: previous build failed; runtime cannot be rebuilt, restart the process")
	}
	r.buildAttempted = true

	// 先为全部已注册模块建立记录（含 disabled），保证失败后 Snapshot 仍是完整视图。
	infos := r.registry.Modules() // 已按 ID 字典序稳定排序
	r.records = make([]*moduleRecord, 0, len(infos))
	for _, info := range infos {
		r.records = append(r.records, &moduleRecord{
			info:    info,
			enabled: r.config.enabledFor(info),
			state:   StateRegistered,
		})
	}

	for _, rec := range r.records {
		if !rec.enabled {
			continue
		}
		info := rec.info
		rec.instance = info.New()
		if rec.instance == nil {
			rec.err = errors.New("module New func returned nil")
			rec.state = StateErrored
			return fmt.Errorf("plugin: build module %q: New returned nil instance", info.ID)
		}
		if p, ok := rec.instance.(Provisioner); ok {
			if err := p.Provision(r.host); err != nil {
				rec.err = err
				rec.state = StateErrored
				return fmt.Errorf("plugin: provision module %q: %w", info.ID, err)
			}
		}
		if v, ok := rec.instance.(Validator); ok {
			if err := v.Validate(); err != nil {
				rec.err = err
				rec.state = StateErrored
				return fmt.Errorf("plugin: validate module %q: %w", info.ID, err)
			}
		}
		rec.state = StateProvisioned
		r.logger.Debug("plugin module provisioned", zap.String("module", string(info.ID)))
	}
	r.built = true
	return nil
}

// Start 按注册表稳定序启动全部 enabled 模块（对实现 Starter 的模块调用 Start，
// 未实现 Starter 的模块直接视为 running）。
//
// 任一模块启动失败时：对已进入 running 的模块按逆序执行 Stop 回滚
// （回滚沿用传入的 ctx）。返回的错误包含失败模块 ID；若回滚中某些模块的
// Stop 也失败，这些错误会一并 errors.Join 进返回值（同时记日志），
// 让调用方能看到哪些模块的资源未释放干净。
func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.built {
		return errors.New("plugin: runtime must be built before start")
	}
	if r.started {
		return errors.New("plugin: runtime already started")
	}

	startedRecs := make([]*moduleRecord, 0, len(r.records))
	for _, rec := range r.records {
		if !rec.enabled || rec.state != StateProvisioned {
			continue
		}
		if s, ok := rec.instance.(Starter); ok {
			if err := s.Start(ctx); err != nil {
				rec.err = err
				rec.state = StateErrored
				// 逆序回滚已启动模块；回滚失败并入返回错误（见方法注释）。
				rollbackErrs := r.stopRecordsLocked(ctx, startedRecs)
				startErr := fmt.Errorf("plugin: start module %q: %w", rec.info.ID, err)
				if len(rollbackErrs) > 0 {
					return errors.Join(append([]error{startErr}, rollbackErrs...)...)
				}
				return startErr
			}
		}
		rec.state = StateRunning
		startedRecs = append(startedRecs, rec)
		r.logger.Info("plugin module started", zap.String("module", string(rec.info.ID)))
	}
	r.started = true
	return nil
}

// Stop 按注册表稳定序的逆序停止全部 running 模块。
//
// 单个模块 Stop 失败只记日志并继续关闭其余模块（与既有 cleanup 序列的容错
// 纪律一致），最终返回 errors.Join 聚合的失败集合（全部成功时为 nil）。
// ctx 原样传递给各模块 Stop，模块必须自行尊重其 deadline。
func (r *Runtime) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	running := make([]*moduleRecord, 0, len(r.records))
	for _, rec := range r.records {
		if rec.state == StateRunning {
			running = append(running, rec)
		}
	}
	errs := r.stopRecordsLocked(ctx, running)
	r.started = false
	return errors.Join(errs...)
}

// stopRecordsLocked 逆序停止 recs 中的模块（recs 为正向顺序），返回失败集合。
// 调用方必须已持有 r.mu。
func (r *Runtime) stopRecordsLocked(ctx context.Context, recs []*moduleRecord) []error {
	var errs []error
	for i := len(recs) - 1; i >= 0; i-- {
		rec := recs[i]
		if s, ok := rec.instance.(Stopper); ok {
			if err := s.Stop(ctx); err != nil {
				rec.err = err
				rec.state = StateErrored
				r.logger.Error("plugin module stop failed",
					zap.String("module", string(rec.info.ID)), zap.Error(err))
				errs = append(errs, fmt.Errorf("plugin: stop module %q: %w", rec.info.ID, err))
				continue
			}
		}
		rec.state = StateStopped
		r.logger.Info("plugin module stopped", zap.String("module", string(rec.info.ID)))
	}
	return errs
}

// Snapshot 返回全部已注册模块的状态快照，按 ID 字典序稳定排序。
//
// Build 之前调用时，基于注册表与配置即时计算（状态均为 registered）；
// Build 之后返回 Runtime 维护的实际生命周期状态。
func (r *Runtime) Snapshot() []ModuleStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.records == nil {
		infos := r.registry.Modules()
		statuses := make([]ModuleStatus, 0, len(infos))
		for _, info := range infos {
			statuses = append(statuses, ModuleStatus{
				ID:      info.ID,
				Enabled: r.config.enabledFor(info),
				State:   StateRegistered,
			})
		}
		return statuses
	}

	statuses := make([]ModuleStatus, 0, len(r.records))
	for _, rec := range r.records {
		status := ModuleStatus{
			ID:      rec.info.ID,
			Enabled: rec.enabled,
			State:   rec.state,
		}
		if rec.err != nil {
			status.Err = rec.err.Error()
		}
		statuses = append(statuses, status)
	}
	return statuses
}
