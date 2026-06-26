package service

import (
	"ikik-api/internal/pkg/antigravity"
	"ikik-api/internal/pkg/claude"
	"ikik-api/internal/pkg/geminicli"
	"ikik-api/internal/pkg/openai"
	"ikik-api/internal/pkg/xai"
)

// DefaultModelIDsForPlatform 返回平台默认模型 ID 列表（未知平台回退 Claude 默认集）。
//
// 由 handler 的 /v1/models 自定义模型过滤与 admin 的默认模型候选两处共同消费——
// 此前两处各持一份同构 switch，必须人工保持同步（Phase-3 TASK-003 收敛为单一来源）。
// 仅共享"ID 列表"这一形状；需要完整模型结构体的调用方仍直接使用各平台包的导出值。
func DefaultModelIDsForPlatform(platform string) []string {
	switch platform {
	case PlatformOpenAI:
		return openai.DefaultModelIDs()
	case PlatformGemini:
		ids := make([]string, 0, len(geminicli.DefaultModels))
		for _, model := range geminicli.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformAntigravity:
		models := antigravity.DefaultModels()
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformGrok:
		return xai.DefaultModelIDs()
	default:
		ids := make([]string, 0, len(claude.DefaultModels))
		for _, model := range claude.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	}
}
