package plugin

import (
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"

	ent "ikik-api/ent"
	"ikik-api/internal/config"
	"ikik-api/internal/pkg/logger"
)

// ProviderSet 提供插件内核的依赖：modules: 配置子树、宿主能力面 Host
// 与模块生命周期驱动器 Runtime。
//
// 边界约定（见 Phase-1 计划）：Wire 只负责装配 Host 与 Runtime，
// 模块目录由注册表（插装清单 internal/modules/standard）负责，
// 模块内部依赖一律经 Host 获取，不进入 Wire graph。
var ProviderSet = wire.NewSet(
	ProvideModuleConfig,
	ProvideHost,
	ProvideModuleRuntime,
)

// ProvideModuleConfig 解析全局配置中 `modules:` 子树为插件模块配置。
//
// 解析失败（模块 ID 格式非法、enabled 类型错误）返回错误，
// 使应用启动 fail-fast，配置笔误在启动时即暴露。
func ProvideModuleConfig(cfg *config.Config) (Config, error) {
	return ParseConfig(cfg.Modules)
}

// ProvideHost 装配模块宿主能力面：聚合全局结构化日志器、模块私有配置
// 解码能力（ConfigOf 绑定为 modules: 配置子树的 Config.Of）、
// Ent 客户端与 Redis 客户端。
//
// 日志器复用项目全局 logger（main 在依赖装配前已完成 logger.Init）。
func ProvideHost(moduleCfg Config, entClient *ent.Client, rdb *redis.Client) *Host {
	return &Host{
		Logger:   logger.L(),
		ConfigOf: moduleCfg.Of,
		DB:       entClient,
		Redis:    rdb,
	}
}

// ProvideModuleRuntime 创建基于包级默认注册表的模块生命周期驱动器。
//
// 生命周期驱动由 main 负责：HTTP server 启动前 Build + Start，
// 关闭时 Stop 接入 cleanup 序列（先于 Redis/Ent 等基础设施关闭）。
func ProvideModuleRuntime(host *Host, moduleCfg Config) *Runtime {
	return NewRuntime(host, moduleCfg)
}
