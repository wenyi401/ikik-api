package plugin

import (
	"fmt"
	"sort"
	"sync"
)

// Registry 是模块注册表：从 ModuleID 到 ModuleInfo 的并发安全目录。
//
// 注册表只负责"有哪些模块"，不负责实例化与生命周期（那是 Runtime 的职责）。
// 常规代码使用包级默认注册表（RegisterModule 等包级函数）；
// 测试可通过 NewRegistry 创建相互隔离的实例。
type Registry struct {
	mu      sync.RWMutex
	modules map[ModuleID]ModuleInfo
}

// NewRegistry 创建一个空的、与包级默认注册表相互隔离的注册表实例。
func NewRegistry() *Registry {
	return &Registry{modules: make(map[ModuleID]ModuleInfo)}
}

// RegisterModule 将模块注册进本注册表。
//
// 与包级 RegisterModule 相同的 panic 语义：模块为 nil、ID 为空、ID 格式非法、
// New 为 nil 或 ID 重复时 panic。注册错误属于编译期插装错误，应尽早暴露。
func (r *Registry) RegisterModule(m Module) {
	if m == nil {
		panic("plugin: RegisterModule called with nil module")
	}
	info := m.ModuleInfo()
	if err := info.ID.validate(); err != nil {
		panic(fmt.Sprintf("plugin: RegisterModule: invalid module ID: %v", err))
	}
	if info.New == nil {
		panic(fmt.Sprintf("plugin: RegisterModule: module %q has a nil New function", info.ID))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.modules[info.ID]; dup {
		panic(fmt.Sprintf("plugin: RegisterModule: module %q is already registered (duplicate registration; check the instrumentation list in internal/modules/standard)", info.ID))
	}
	r.modules[info.ID] = info
}

// GetModule 按 ID 查询模块注册信息，第二个返回值表示是否存在。
func (r *Registry) GetModule(id ModuleID) (ModuleInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.modules[id]
	return info, ok
}

// GetModulesInNamespace 返回命名空间恰好为 ns 的全部模块，按 ID 字典序稳定排序。
//
// 匹配为精确匹配（非前缀匹配）：ns 为 "gateway.platform" 时返回
// "gateway.platform.anthropic" 等直接子模块，但不包含 "gateway.platform.x.y"。
// ns 为空字符串时返回所有单段 ID 的顶层模块。
func (r *Registry) GetModulesInNamespace(ns string) []ModuleInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ModuleInfo
	for id, info := range r.modules {
		if id.Namespace() == ns {
			result = append(result, info)
		}
	}
	sortModuleInfos(result)
	return result
}

// Modules 返回注册表中的全部模块，按 ID 字典序稳定排序。
// Runtime 以该顺序作为模块生命周期的确定性顺序。
func (r *Registry) Modules() []ModuleInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ModuleInfo, 0, len(r.modules))
	for _, info := range r.modules {
		result = append(result, info)
	}
	sortModuleInfos(result)
	return result
}

// sortModuleInfos 按 ID 字典序原地排序，保证遍历顺序的确定性。
func sortModuleInfos(infos []ModuleInfo) {
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })
}

// defaultRegistry 是包级默认注册表，模块包在 init() 期向其自注册。
var defaultRegistry = NewRegistry()

// RegisterModule 将模块注册进包级默认注册表，必须在 init() 期调用。
//
// 模块为 nil、ID 为空、ID 格式非法、New 为 nil 或 ID 重复时 panic：
// 注册错误属于编译期插装错误（插装清单 import 错误、模块 ID 冲突等），
// 必须在进程启动最早期暴露，而不是延迟到运行时。
func RegisterModule(m Module) {
	defaultRegistry.RegisterModule(m)
}

// GetModule 按 ID 在包级默认注册表中查询模块注册信息。
func GetModule(id ModuleID) (ModuleInfo, bool) {
	return defaultRegistry.GetModule(id)
}

// GetModulesInNamespace 在包级默认注册表中查询命名空间恰好为 ns 的全部模块，
// 按 ID 字典序稳定排序。匹配语义见 (*Registry).GetModulesInNamespace。
func GetModulesInNamespace(ns string) []ModuleInfo {
	return defaultRegistry.GetModulesInNamespace(ns)
}
