package handler

import (
	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

// CapacitySummary GET /api/v1/channel-monitors/capacity-summary
func (h *ChannelMonitorUserHandler) CapacitySummary(c *gin.Context) {
	if !h.featureEnabled(c) {
		response.Success(c, channelMonitorCapacitySummary([]service.GroupCapacitySummary{}))
		return
	}
	if h.groupCapacityService == nil {
		response.Error(c, 500, "Group capacity service is unavailable")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	items, err := h.groupCapacityService.GetUserVisibleGroupCapacity(c.Request.Context(), subject.UserID)
	if err != nil {
		response.Error(c, 500, "Failed to get group capacity summary")
		return
	}
	response.Success(c, channelMonitorCapacitySummary(items))
}
