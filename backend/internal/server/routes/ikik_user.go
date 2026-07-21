package routes

import (
	"ikik-api/internal/handler"
	"ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

func registerIkikUserRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
) {
	publicUsage := v1.Group("/public/usage")
	publicUsage.GET("/today", h.Usage.PublicTodayStats)

	shopPublic := v1.Group("/shop")
	shopPublic.GET("/categories", h.Shop.ListCategories)
	shopPublic.GET("/products", h.Shop.ListProducts)
	shopPublic.GET("/products/:id", h.Shop.GetProduct)

	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	authenticated.Use(gin.HandlerFunc(auditLog))

	shop := authenticated.Group("/shop")
	shop.GET("/draw-progress", h.Shop.ListDrawProgress)
	shop.POST("/orders", h.Shop.CreateOrder)
	shop.GET("/orders/:id", h.Shop.GetOrder)
	shop.GET("/orders/:id/files/download.zip", h.Shop.DownloadOrderFilesZip)
	shop.GET("/orders/:id/files/:card_id/download", h.Shop.DownloadOrderFile)

	user := authenticated.Group("/user")
	user.GET("/receipt-code", h.ReceiptCode.Get)
	user.POST("/receipt-code", h.ReceiptCode.Upload)
	user.DELETE("/receipt-code", h.ReceiptCode.Delete)
	user.GET("/withdrawals", h.Withdrawal.ListMine)
	user.POST("/withdrawals", h.Withdrawal.Submit)
	user.POST("/withdrawals/:id/cancel", h.Withdrawal.Cancel)

	registerIkikUserAccountRoutes(authenticated, h)

	playground := authenticated.Group("/playground")
	playground.POST("/chat/completions", h.Playground.ChatCompletions)

	usage := authenticated.Group("/usage")
	usage.GET("/dashboard/account-sharing", h.Usage.DashboardAccountSharing)

	monitors := authenticated.Group("/channel-monitors")
	monitors.GET("/capacity-summary", h.ChannelMonitor.CapacitySummary)
}

func registerIkikUserAccountRoutes(authenticated *gin.RouterGroup, h *handler.Handlers) {
	accounts := authenticated.Group("/accounts")
	accounts.GET("", h.UserAccount.List)
	accounts.GET("/carpools", h.UserAccount.ListCarpools)
	accounts.GET("/carpools/hall", h.UserAccount.ListCarpoolHall)
	accounts.GET("/carpools/invite/:invite_code", h.UserAccount.GetCarpoolDetailByInviteCode)
	accounts.POST("/carpools/invite/:invite_code/apply", h.UserAccount.ApplyCarpoolByInviteCode)
	accounts.GET("/carpools/:pool_id", h.UserAccount.GetCarpoolDetail)
	accounts.POST("/carpools", h.UserAccount.CreateCarpool)
	accounts.DELETE("/carpools/:pool_id", h.UserAccount.DeleteCarpool)
	accounts.PUT("/carpools/:pool_id/accounts", h.UserAccount.BindCarpoolAccounts)
	accounts.POST("/carpools/:pool_id/accounts/:account_id/reset-local-limit", h.UserAccount.ResetCarpoolAccountLocalLimit)
	accounts.POST("/carpools/:pool_id/apply", h.UserAccount.ApplyCarpool)
	accounts.POST("/carpools/:pool_id/requests/:request_id/approve", h.UserAccount.ApproveCarpoolJoinRequest)
	accounts.POST("/carpools/:pool_id/requests/:request_id/reject", h.UserAccount.RejectCarpoolJoinRequest)
	accounts.POST("/carpools/:pool_id/requests/:request_id/confirm-paid", h.UserAccount.ConfirmCarpoolJoinPaid)
	accounts.POST("/carpools/:pool_id/members/:member_id/remove", h.UserAccount.RemoveCarpoolMember)
	accounts.PUT("/carpools/:pool_id/members/allocation", h.UserAccount.UpdateCarpoolMemberAllocations)
	accounts.GET("/quota-dashboard", h.UserAccount.GetQuotaPoolDashboard)
	accounts.GET("/data", h.UserAccount.ExportData)
	accounts.POST("/today-stats/batch", h.UserAccount.GetBatchTodayStats)
	accounts.POST("", h.UserAccount.Create)
	accounts.POST("/import", h.UserAccount.Import)
	accounts.POST("/import-credentials", h.UserAccount.ImportCredentials)
	accounts.POST("/bulk-update", h.UserAccount.BulkUpdate)
	accounts.POST("/bulk-delete", h.UserAccount.BulkDelete)
	accounts.POST("/batch-refresh/async", h.UserAccount.CreateBatchRefreshTask)
	accounts.POST("/batch-revalidate-public-share/async", h.UserAccount.CreateBatchRevalidatePublicShareTask)
	accounts.GET("/batch-tasks/:task_id", h.UserAccount.GetBatchTask)
	accounts.GET("/:id/usage", h.UserAccount.GetUsage)
	accounts.GET("/:id/stats", h.UserAccount.GetStats)
	accounts.GET("/:id/today-stats", h.UserAccount.GetTodayStats)
	accounts.GET("/:id", h.UserAccount.GetByID)
	accounts.POST("/:id/test", h.UserAccount.Test)
	accounts.POST("/:id/refresh", h.UserAccount.Refresh)
	accounts.POST("/:id/set-privacy", h.UserAccount.SetPrivacy)
	accounts.POST("/:id/revalidate-public-share", h.UserAccount.RevalidatePublicShare)
	accounts.PUT("/:id", h.UserAccount.Update)
	accounts.DELETE("/:id", h.UserAccount.Delete)

	proxies := authenticated.Group("/account-proxies")
	proxies.GET("", h.UserAccount.ListProxies)
	proxies.POST("", h.UserAccount.CreateProxy)
	proxies.PUT("/:id", h.UserAccount.UpdateProxy)
	proxies.DELETE("/:id", h.UserAccount.DeleteProxy)
	proxies.POST("/:id/test", h.UserAccount.TestProxy)
	proxies.POST("/:id/quality-check", h.UserAccount.CheckProxyQuality)

	oauth := authenticated.Group("/account-oauth")
	oauth.POST("/anthropic/auth-url", h.UserAccount.GenerateAnthropicOAuthURL)
	oauth.POST("/anthropic/exchange-code", h.UserAccount.ExchangeAnthropicOAuthCode)
	oauth.POST("/anthropic/setup-token/auth-url", h.UserAccount.GenerateAnthropicSetupTokenURL)
	oauth.POST("/anthropic/setup-token/exchange-code", h.UserAccount.ExchangeAnthropicSetupTokenCode)
	oauth.POST("/anthropic/cookie-auth", h.UserAccount.AnthropicCookieAuth)
	oauth.POST("/anthropic/setup-token-cookie-auth", h.UserAccount.AnthropicSetupTokenCookieAuth)
	oauth.POST("/openai/auth-url", h.UserAccount.GenerateOpenAIOAuthURL)
	oauth.POST("/openai/exchange-code", h.UserAccount.ExchangeOpenAIOAuthCode)
	oauth.POST("/openai/refresh-token", h.UserAccount.RefreshOpenAIToken)
	oauth.GET("/gemini/capabilities", h.UserAccount.GetGeminiOAuthCapabilities)
	oauth.POST("/gemini/auth-url", h.UserAccount.GenerateGeminiOAuthURL)
	oauth.POST("/gemini/exchange-code", h.UserAccount.ExchangeGeminiOAuthCode)
	oauth.POST("/antigravity/auth-url", h.UserAccount.GenerateAntigravityOAuthURL)
	oauth.POST("/antigravity/exchange-code", h.UserAccount.ExchangeAntigravityOAuthCode)
	oauth.POST("/antigravity/refresh-token", h.UserAccount.RefreshAntigravityToken)
	oauth.POST("/grok/auth-url", h.UserAccount.GenerateGrokOAuthURL)
	oauth.POST("/grok/exchange-code", h.UserAccount.ExchangeGrokOAuthCode)
	oauth.POST("/grok/refresh-token", h.UserAccount.RefreshGrokToken)
	oauth.POST("/grok/sso-to-oauth", h.UserAccount.ImportGrokSSO)
	oauth.POST("/kiro/auth-url", h.UserAccount.GenerateKiroOAuthURL)
	oauth.POST("/kiro/idc-auth-url", h.UserAccount.GenerateKiroIDCAuthURL)
	oauth.POST("/kiro/exchange-code", h.UserAccount.ExchangeKiroOAuthCode)
	oauth.POST("/kiro/refresh-token", h.UserAccount.RefreshKiroToken)
	oauth.POST("/kiro/import-token", h.UserAccount.ImportKiroToken)
}
