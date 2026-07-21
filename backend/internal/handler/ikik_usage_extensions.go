package handler

import (
	"strings"
	"time"

	"ikik-api/internal/pkg/response"

	middleware2 "ikik-api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// DashboardAccountSharing handles owned-account self usage and public-share settlement statistics.
// GET /api/v1/usage/dashboard/account-sharing
func (h *UsageHandler) DashboardAccountSharing(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	startTime, endTime, err := parseUserDashboardTimeRangeStrict(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	granularity := strings.TrimSpace(c.DefaultQuery("granularity", "day"))
	switch granularity {
	case "hour", "day", "week", "month":
	default:
		response.BadRequest(c, "Invalid granularity, use hour, day, week, or month")
		return
	}

	accountPage, accountPageSize, err := parseAccountSharingPagination(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	stats, err := h.usageService.GetUserAccountSharingDashboard(c.Request.Context(), subject.UserID, startTime, endTime, granularity, accountPage, accountPageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// PublicTodayStats handles public homepage usage counters.
// GET /api/v1/public/usage/today
func (h *UsageHandler) PublicTodayStats(c *gin.Context) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		response.InternalError(c, "Failed to load timezone")
		return
	}

	now := time.Now().In(loc)
	startTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	stats, err := h.usageService.GetPublicTodayStats(c.Request.Context(), startTime, now)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"today_requests":         stats.TodayRequests,
		"today_tokens":           stats.TodayTokens,
		"success_count":          stats.SuccessCount,
		"error_count":            stats.ErrorCount,
		"success_rate":           stats.SuccessRate,
		"average_duration_ms":    stats.AverageDurationMs,
		"average_first_token_ms": stats.AverageFirstTokenMs,
		"timezone":               "Asia/Shanghai",
	})
}
