// Package plugin 实现 Caddy 式进程内插件内核。
//
// 内核由四部分组成：
//   - 模块注册表（registry.go）：命名空间化的模块目录，模块包在 init() 期通过
//     RegisterModule 自注册，编译期插装清单（internal/modules/standard）决定哪些模块被编译进来；
//   - 配置子树（config.go）：来自全局配置 `modules:` 的每模块配置（enabled + raw 私有配置）；
//   - 宿主能力面（host.go）：模块可使用的全部宿主能力，只暴露 ports 级接口；
//   - 生命周期驱动器（runtime.go）：按注册表稳定序实例化 enabled 模块并驱动
//     Provision → Validate → Start → Stop（逆序）生命周期。
//
// 注册表只是"目录"，模块的实例化与生命周期统一由 Runtime 驱动。
package plugin

import (
	"context"
	"fmt"
	"strings"
)

// ModuleID 是模块的全局唯一标识，采用点分层级命名，
// 形如 "job.hello"、"gateway.platform.anthropic"。
//
// 最后一段为模块名（Name），之前的部分为命名空间（Namespace）。
// 每一段只允许 [a-z0-9_-] 字符且不能为空。
type ModuleID string

// Namespace 返回模块 ID 的命名空间部分（去掉最后一段）。
//
// 例如 "gateway.platform.anthropic" 的命名空间为 "gateway.platform"；
// 单段 ID（如 "hello"）的命名空间为空字符串。
func (id ModuleID) Namespace() string {
	lastDot := strings.LastIndex(string(id), ".")
	if lastDot < 0 {
		return ""
	}
	return string(id)[:lastDot]
}

// Name 返回模块 ID 的最后一段，即模块名。
//
// 例如 "gateway.platform.anthropic" 的模块名为 "anthropic"。
func (id ModuleID) Name() string {
	lastDot := strings.LastIndex(string(id), ".")
	if lastDot < 0 {
		return string(id)
	}
	return string(id)[lastDot+1:]
}

// validate 校验模块 ID 格式：非空、点分、各段为非空的 [a-z0-9_-]+。
// 校验规则有意保持宽松，不做长度等额外限制。
func (id ModuleID) validate() error {
	if id == "" {
		return fmt.Errorf("module ID is empty")
	}
	for _, segment := range strings.Split(string(id), ".") {
		if segment == "" {
			return fmt.Errorf("module ID %q contains an empty segment", id)
		}
		for _, r := range segment {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '-' {
				return fmt.Errorf("module ID %q contains invalid character %q (allowed: [a-z0-9_-] per segment)", id, r)
			}
		}
	}
	return nil
}

// ModuleInfo 描述一个模块的注册信息（模块目录中的一条记录）。
type ModuleInfo struct {
	// ID 是模块的全局唯一标识，见 ModuleID。
	ID ModuleID

	// New 返回该模块的一个新实例。
	// 每次 Runtime.Build 都会通过 New 创建全新实例，模块不应依赖包级可变状态。
	New func() Module

	// EnabledByDefault 声明该模块在 `modules:` 配置未显式设置 enabled 时的默认启停状态。
	//
	// 设计说明（默认 enabled 的声明方式）：默认值由模块在注册时通过本字段自行声明，
	// 而不是由内核统一约定。这样"缺省配置 = 内置模块全部按各自默认 enabled 值运行"
	// 的语义完全由各模块的注册信息决定；零值 false 意味着新模块默认不启用，
	// 保证引入新模块不会在未配置时改变现有行为（与 Phase-1 零行为变更要求一致）。
	EnabledByDefault bool
}

// Module 是所有插件模块必须实现的最小接口。
//
// 模块通常是一个 struct，在所属包的 init() 中调用 RegisterModule 完成自注册。
// 除 Module 外，模块可按需实现以下可选生命周期接口：
// Provisioner、Validator、Starter、Stopper。
type Module interface {
	// ModuleInfo 返回模块的注册信息。该方法必须无副作用且可在零值实例上调用。
	ModuleInfo() ModuleInfo
}

// Provisioner 是可选生命周期接口：在模块实例化后、校验前装配依赖。
//
// Provision 在 Runtime.Build 阶段被调用，模块应在此从 Host 获取所需能力
// （Logger / ConfigOf / DB / Redis）并完成自身初始化。失败将中止启动。
type Provisioner interface {
	Provision(host *Host) error
}

// Validator 是可选生命周期接口：在 Provision 之后校验模块配置与状态。
//
// Validate 在 Runtime.Build 阶段被调用，失败将中止启动。
type Validator interface {
	Validate() error
}

// Starter 是可选生命周期接口：启动模块的后台工作（goroutine、监听等）。
//
// Start 在 Runtime.Start 阶段按注册表稳定序被调用，失败将中止启动并
// 触发对已启动模块的逆序 Stop 回滚。
//
// 契约：实现必须快速返回——长任务应自行启动 goroutine（参照 hello 模块）。
// Runtime 在持锁状态下调用 Start/Stop，阻塞的实现会同时阻塞 Snapshot
// （即 admin 模块可观测接口）与其他生命周期操作。
type Starter interface {
	Start(ctx context.Context) error
}

// Stopper 是可选生命周期接口：停止模块并释放资源。
//
// Stop 在 Runtime.Stop 阶段按注册表稳定序的逆序被调用，
// 实现必须尊重 ctx 的 deadline（与进程优雅关闭超时协同）。
//
// 契约：与 Start 相同，实现必须快速返回（等待自身 goroutine 退出时
// 用 select 配合 ctx.Done()，参照 hello 模块），原因见 Starter 注释。
type Stopper interface {
	Stop(ctx context.Context) error
}
