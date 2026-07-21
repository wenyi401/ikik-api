package routes

import (
	"ikik-api/internal/handler"
	"ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

func registerIkikPaymentRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
) {
	payment := v1.Group("/payment")
	payment.Use(gin.HandlerFunc(jwtAuth))
	payment.Use(middleware.BackendModeUserGuard(settingService))
	payment.GET("/channels", h.Payment.GetChannels)

	adminPayment := v1.Group("/admin/payment")
	adminPayment.Use(gin.HandlerFunc(adminAuth))
	adminPayment.Use(gin.HandlerFunc(auditLog))
	adminPayment.Use(middleware.AdminComplianceGuard(settingService))
	adminPayment.POST("/orders/:id/manual-fulfill", h.Admin.Payment.ManualFulfillOrder)
}
