package plugin

import (
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	ent "ikik-api/ent"
)

// Host 是模块可使用的宿主能力面（纯数据结构，由宿主装配后注入 Provision）。
//
// 边界纪律：Host 只暴露 ports 级基础能力，禁止暴露任何具体 Service；
// 本阶段仅 Logger / ConfigOf / DB / Redis 四项，后续每新增一项
// 必须在插件化 ROADMAP 决策记录中登记，防止 Host 膨胀为上帝对象。
type Host struct {
	// Logger 是宿主提供的结构化日志器。模块应通过它输出日志，
	// 而不是自建全局 logger。
	Logger *zap.Logger

	// ConfigOf 将 `modules:` 配置子树中 id 对应的 raw 私有配置解码到 out
	// （out 必须是指向模块自定义配置 struct 的指针）。
	// 模块未配置时不修改 out（模块应自带默认值）；解码类型错误时返回错误。
	//
	// 该字段通常由宿主装配为 Config.Of；若装配时留空，
	// NewRuntime 会默认绑定到 Runtime 自身持有的 Config。
	ConfigOf func(id ModuleID, out any) error

	// DB 是 Ent ORM 客户端（数据库访问能力）。
	DB *ent.Client

	// Redis 是 go-redis 客户端（缓存/队列能力）。
	// 使用 UniversalClient 接口以兼容单机与集群部署形态。
	Redis redis.UniversalClient
}
