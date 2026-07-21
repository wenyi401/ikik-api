package routes

import (
	"ikik-api/internal/handler"
	"ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterIkikRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
) {
	registerIkikUserRoutes(v1, h, jwtAuth, auditLog, settingService)
	registerIkikAdminRoutes(v1, h, adminAuth, auditLog, settingService)
	registerIkikPaymentRoutes(v1, h, jwtAuth, adminAuth, auditLog, settingService)
}
