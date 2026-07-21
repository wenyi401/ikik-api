package admin

import (
	"strconv"

	"ikik-api/internal/pkg/response"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

// GetGroupRateSchedules handles listing time-range rate schedules for a group.
// GET /api/v1/admin/groups/:id/rate-schedules
func (h *GroupHandler) GetGroupRateSchedules(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	if h.groupRateScheduleService == nil {
		response.Success(c, []service.GroupRateSchedule{})
		return
	}

	schedules, err := h.groupRateScheduleService.List(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if schedules == nil {
		schedules = []service.GroupRateSchedule{}
	}
	response.Success(c, schedules)
}

// ReplaceGroupRateSchedules handles replacing time-range rate schedules for a group.
// PUT /api/v1/admin/groups/:id/rate-schedules
func (h *GroupHandler) ReplaceGroupRateSchedules(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	if h.groupRateScheduleService == nil {
		response.Error(c, 500, "Group rate schedule service is not configured")
		return
	}

	var req ReplaceGroupRateSchedulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	inputs := make([]service.GroupRateScheduleInput, 0, len(req.Entries))
	for _, entry := range req.Entries {
		enabled := true
		if entry.Enabled != nil {
			enabled = *entry.Enabled
		}
		inputs = append(inputs, service.GroupRateScheduleInput{
			TargetUserID:   entry.TargetUserID,
			StartMinute:    entry.StartMinute,
			EndMinute:      entry.EndMinute,
			RateMultiplier: entry.RateMultiplier,
			Enabled:        enabled,
		})
	}
	schedules, err := h.groupRateScheduleService.Replace(c.Request.Context(), groupID, inputs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if schedules == nil {
		schedules = []service.GroupRateSchedule{}
	}
	response.Success(c, schedules)
}
