package routes

import (
	"ikik-api/internal/handler"
	"ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterDeveloperRoutes(
	developer *gin.RouterGroup,
	h *handler.Handlers,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
) {
	if h == nil || h.Developer == nil || h.UserAccount == nil {
		return
	}
	developer.Use(h.Developer.Authenticate())
	developer.Use(middleware.BackendModeUserGuard(settingService))
	developer.Use(gin.HandlerFunc(auditLog))

	developer.GET("/health", h.Developer.Health)

	bot := developer.Group("/bot")
	bot.Use(h.Developer.RequireScope(service.DeveloperScopeBotAccess))
	if h.User != nil {
		bot.GET("/profile", h.User.GetProfile)
	}
	if h.Usage != nil {
		bot.GET("/usage/stats", h.Usage.DashboardStats)
		bot.GET("/usage/trend", h.Usage.DashboardTrend)
		bot.GET("/sharing", h.Usage.DashboardAccountSharing)
	}
	if h.ServiceStatus != nil {
		bot.GET("/channels/openai", h.ServiceStatus.GetOpenAI)
		bot.GET("/channels/providers", h.ServiceStatus.GetProviders)
	}
	if h.ChannelMonitor != nil {
		bot.GET("/channels", h.ChannelMonitor.BotSummary)
	}
	bot.GET("/accounts/summary", h.Developer.BotAccountSummary)

	developer.POST(
		"/account-imports",
		h.Developer.RequireScope(service.DeveloperScopeAccountsWrite),
		h.Developer.ImportAccounts,
	)
	developer.GET(
		"/account-tasks/:task_id",
		h.Developer.RequireScope(service.DeveloperScopeAccountsRead),
		h.UserAccount.GetBatchTask,
	)

	accounts := developer.Group("/accounts")
	accounts.GET("", h.Developer.RequireScope(service.DeveloperScopeAccountsRead), h.UserAccount.List)
	accounts.GET("/:id", h.Developer.RequireScope(service.DeveloperScopeAccountsRead), h.UserAccount.GetByID)
	accounts.PUT("/:id/sharing", h.Developer.RequireScope(service.DeveloperScopeAccountsShare), h.Developer.SetAccountSharing)
	accounts.DELETE("/:id", h.Developer.RequireScope(service.DeveloperScopeAccountsWrite), h.Developer.DeleteAccount)
}
