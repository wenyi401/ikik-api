package admin

import (
	"strconv"
	"strings"
	"time"

	"ikik-api/internal/pkg/response"
	tzpkg "ikik-api/internal/pkg/timezone"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type RevenueHandler struct {
	revenueService *service.RevenueService
}

func NewRevenueHandler(revenueService *service.RevenueService) *RevenueHandler {
	return &RevenueHandler{revenueService: revenueService}
}

// GetSummary returns the read-only revenue management dashboard data.
// GET /api/v1/admin/revenue/summary
func (h *RevenueHandler) GetSummary(c *gin.Context) {
	startTime, endTime := parseRevenueTimeRange(c)
	granularity := strings.ToLower(strings.TrimSpace(c.DefaultQuery("granularity", service.RevenueGranularityDay)))
	if granularity != service.RevenueGranularityDay && granularity != service.RevenueGranularityHour {
		response.BadRequest(c, "granularity must be day or hour")
		return
	}
	if !endTime.After(startTime) {
		response.BadRequest(c, "end_date must be after start_date")
		return
	}

	topLimit := 10
	if rawLimit := strings.TrimSpace(c.Query("top_limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "top_limit must be a positive integer")
			return
		}
		topLimit = parsed
	}

	var userID *int64
	if rawUserID := strings.TrimSpace(c.Query("user_id")); rawUserID != "" {
		parsed, err := strconv.ParseInt(rawUserID, 10, 64)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "user_id must be a positive integer")
			return
		}
		userID = &parsed
	}

	stats, err := h.revenueService.GetSummary(c.Request.Context(), service.RevenueQueryParams{
		StartTime:   startTime,
		EndTime:     endTime,
		Granularity: granularity,
		Timezone:    normalizeRevenueTimezone(c.Query("timezone")),
		TopLimit:    topLimit,
		UserID:      userID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

// ListShareSettlements returns auditable account-share settlement entries.
// GET /api/v1/admin/revenue/share-settlements
func (h *RevenueHandler) ListShareSettlements(c *gin.Context) {
	startTime, endTime := parseRevenueTimeRange(c)
	if !endTime.After(startTime) {
		response.BadRequest(c, "end_date must be after start_date")
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := h.revenueService.ListShareSettlements(c.Request.Context(), service.RevenueShareSettlementQueryParams{
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		PageSize:  pageSize,
		Search:    c.Query("search"),
		Status:    c.Query("status"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func parseRevenueTimeRange(c *gin.Context) (time.Time, time.Time) {
	userTZ := c.Query("timezone")
	now := tzpkg.NowInUserLocation(userTZ)
	startTime := tzpkg.StartOfDayInUserLocation(now, userTZ)
	endTime := tzpkg.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)

	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := tzpkg.ParseInUserLocation("2006-01-02", startDate, userTZ); err == nil {
			startTime = parsed
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := tzpkg.ParseInUserLocation("2006-01-02", endDate, userTZ); err == nil {
			endTime = parsed.Add(24 * time.Hour)
		}
	}

	return startTime, endTime
}

func normalizeRevenueTimezone(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		if _, err := time.LoadLocation(raw); err == nil {
			return raw
		}
	}
	name := tzpkg.Name()
	if name != "" && name != "Local" {
		if _, err := time.LoadLocation(name); err == nil {
			return name
		}
	}
	return "UTC"
}
