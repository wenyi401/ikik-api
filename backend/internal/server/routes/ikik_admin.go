package routes

import (
	"ikik-api/internal/handler"
	"ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

func registerIkikAdminRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
) {
	admin := v1.Group("/admin")
	admin.Use(gin.HandlerFunc(adminAuth))
	admin.Use(gin.HandlerFunc(auditLog))
	admin.Use(middleware.AdminComplianceGuard(settingService))

	admin.GET("/modules", h.Admin.Module.List)

	carpools := admin.Group("/carpools")
	carpools.GET("", h.Admin.Carpool.List)
	carpools.GET("/:id", h.Admin.Carpool.Get)
	carpools.POST("/:id/close", h.Admin.Carpool.Close)
	carpools.POST("/:id/repair", h.Admin.Carpool.Repair)
	carpools.DELETE("/:id", h.Admin.Carpool.Delete)

	policies := admin.Group("/account-share-policies")
	policies.GET("", h.Admin.AccountSharePolicy.List)
	policies.GET("/:id", h.Admin.AccountSharePolicy.GetByID)
	policies.POST("", h.Admin.AccountSharePolicy.Create)
	policies.PUT("/:id", h.Admin.AccountSharePolicy.Update)
	policies.DELETE("/:id", h.Admin.AccountSharePolicy.Delete)

	broadcasts := admin.Group("/email-broadcasts")
	broadcasts.GET("", h.Admin.EmailBroadcast.List)
	broadcasts.POST("", h.Admin.EmailBroadcast.Create)
	broadcasts.POST("/preview", h.Admin.EmailBroadcast.Preview)
	broadcasts.GET("/recipients/search", h.Admin.EmailBroadcast.SearchRecipients)
	broadcasts.GET("/:id", h.Admin.EmailBroadcast.Get)
	broadcasts.DELETE("/:id", h.Admin.EmailBroadcast.Delete)

	registerIkikAdminAccountRoutes(admin, h)
	registerIkikAdminKiroRoutes(admin, h)
	registerIkikAdminRevenueRoutes(admin, h)
	registerIkikAdminShopRoutes(admin, h)
	registerIkikAdminOpsRoutes(admin, h)
}

func registerIkikAdminAccountRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	users := admin.Group("/users")
	users.POST("/:id/points", h.Admin.User.UpdatePoints)

	groups := admin.Group("/groups")
	groups.GET("/:id/rate-schedules", h.Admin.Group.GetGroupRateSchedules)
	groups.PUT("/:id/rate-schedules", h.Admin.Group.ReplaceGroupRateSchedules)

	accounts := admin.Group("/accounts")
	accounts.GET("/quota-dashboard", h.Admin.Account.GetQuotaDashboard)
	accounts.POST("/model-probe/list", h.Admin.Account.ProbeModelList)
	accounts.POST("/model-probe/test", h.Admin.Account.ProbeModels)
	accounts.POST("/import-credentials", h.Admin.Account.ImportCredentials)
	accounts.POST("/batch-refresh/async", h.Admin.Account.CreateBatchRefreshTask)
	accounts.GET("/batch-tasks/:task_id", h.Admin.Account.GetBatchTask)

	affiliates := admin.Group("/affiliates")
	affiliates.POST("/invite-rewards/extend", h.Admin.Affiliate.ExtendInviteRewards)
	affiliates.POST("/users/:user_id/inviter", h.Admin.Affiliate.BindInviter)
}

func registerIkikAdminKiroRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	kiro := admin.Group("/kiro")
	kiro.POST("/oauth/auth-url", h.Admin.KiroOAuth.GenerateAuthURL)
	kiro.POST("/oauth/idc-auth-url", h.Admin.KiroOAuth.GenerateIDCAuthURL)
	kiro.POST("/oauth/exchange-code", h.Admin.KiroOAuth.ExchangeCode)
	kiro.POST("/oauth/refresh-token", h.Admin.KiroOAuth.RefreshToken)
	kiro.POST("/oauth/import-token", h.Admin.KiroOAuth.ImportToken)
	kiro.POST("/oauth/create-from-oauth", h.Admin.KiroOAuth.CreateAccountFromOAuth)
	kiro.POST("/accounts/:id/refresh", h.Admin.KiroOAuth.RefreshAccountToken)
}

func registerIkikAdminRevenueRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	revenue := admin.Group("/revenue")
	revenue.GET("/summary", h.Admin.Revenue.GetSummary)
	revenue.GET("/share-settlements", h.Admin.Revenue.ListShareSettlements)

	withdrawals := admin.Group("/withdrawals")
	withdrawals.GET("", h.Admin.Withdrawal.List)
	withdrawals.GET("/:id", h.Admin.Withdrawal.Get)
	withdrawals.POST("/:id/settle", h.Admin.Withdrawal.Settle)
	withdrawals.POST("/:id/reject", h.Admin.Withdrawal.Reject)
}

func registerIkikAdminOpsRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	risk := admin.Group("/risk-control")
	risk.GET("/risk-profiles", h.Admin.ContentModeration.ListRiskProfiles)
	risk.PATCH("/risk-profiles/:user_id", h.Admin.ContentModeration.UpdateRiskProfile)

	ops := admin.Group("/ops")
	ops.GET("/errors/:id/retries", h.Admin.Ops.ListRetryAttempts)
	ops.POST("/errors/:id/retry", h.Admin.Ops.RetryErrorRequest)
	ops.POST("/request-errors/:id/retry-client", h.Admin.Ops.RetryRequestErrorClient)
	ops.POST("/request-errors/:id/upstream-errors/:idx/retry", h.Admin.Ops.RetryRequestErrorUpstreamEvent)
	ops.POST("/upstream-errors/:id/retry", h.Admin.Ops.RetryUpstreamError)
}

func registerIkikAdminShopRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	shop := admin.Group("/shop")
	categories := shop.Group("/categories")
	categories.GET("", h.Admin.Shop.ListCategories)
	categories.POST("", h.Admin.Shop.CreateCategory)
	categories.PUT("/:id", h.Admin.Shop.UpdateCategory)
	categories.DELETE("/:id", h.Admin.Shop.DeleteCategory)

	products := shop.Group("/products")
	products.GET("", h.Admin.Shop.ListProducts)
	products.POST("", h.Admin.Shop.CreateProduct)
	products.PUT("/:id", h.Admin.Shop.UpdateProduct)
	products.DELETE("/:id", h.Admin.Shop.DeleteProduct)

	cardKeys := shop.Group("/card-keys")
	cardKeys.GET("", h.Admin.Shop.ListCardKeys)
	cardKeys.POST("", h.Admin.Shop.CreateCardKey)
	cardKeys.POST("/import", h.Admin.Shop.ImportCardKeys)
	cardKeys.POST("/import-files", h.Admin.Shop.ImportFileCardKeys)
	cardKeys.PUT("/:id", h.Admin.Shop.UpdateCardKey)
	cardKeys.DELETE("/:id", h.Admin.Shop.DeleteCardKey)

	orders := shop.Group("/orders")
	orders.GET("/:id", h.Admin.Shop.GetOrder)
	orders.GET("/:id/files/download.zip", h.Admin.Shop.DownloadOrderFilesZip)
	orders.GET("/:id/files/:card_id/download", h.Admin.Shop.DownloadOrderFile)

	storage := shop.Group("/file-card-storage")
	storage.GET("", h.Admin.Shop.GetFileCardStorage)
	storage.PUT("", h.Admin.Shop.UpdateFileCardStorage)
	storage.POST("/test", h.Admin.Shop.TestFileCardStorage)
}
