package admin

import (
	"sort"

	"ikik-api/internal/pkg/response"
	"ikik-api/internal/plugin"
	"ikik-api/internal/util/logredact"

	"github.com/gin-gonic/gin"
)

// moduleStatusSource 提供插件模块状态快照能力（*plugin.Runtime 实现该接口，测试可注入桩）。
type moduleStatusSource interface {
	Snapshot() []plugin.ModuleStatus
}

// ModuleHandler handles plugin module observability operations (read-only)
type ModuleHandler struct {
	runtime moduleStatusSource
}

// NewModuleHandler creates a new ModuleHandler
func NewModuleHandler(runtime *plugin.Runtime) *ModuleHandler {
	return &ModuleHandler{runtime: runtime}
}

// ModuleStatusItem 是单个插件模块状态的响应条目
type ModuleStatusItem struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	State     string `json:"state"`
	Error     string `json:"error"`
}

// List 返回全部已注册插件模块的状态清单（含未启用模块），按模块 ID 字典序排序
// GET /api/v1/admin/modules
func (h *ModuleHandler) List(c *gin.Context) {
	statuses := h.runtime.Snapshot()
	modules := make([]ModuleStatusItem, 0, len(statuses))
	for _, status := range statuses {
		errText := status.Err
		if errText != "" {
			// 错误文本可能包含敏感信息（如上游返回的凭据片段），输出前统一脱敏。
			errText = logredact.RedactText(errText)
		}
		modules = append(modules, ModuleStatusItem{
			ID:        string(status.ID),
			Namespace: status.ID.Namespace(),
			Name:      status.ID.Name(),
			Enabled:   status.Enabled,
			State:     string(status.State),
			Error:     errText,
		})
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].ID < modules[j].ID })
	response.Success(c, gin.H{"modules": modules})
}
