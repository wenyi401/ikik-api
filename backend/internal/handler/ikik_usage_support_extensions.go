package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"ikik-api/internal/pkg/timezone"

	"github.com/gin-gonic/gin"
)

func parseAccountSharingPagination(c *gin.Context) (page, pageSize int, err error) {
	page = 1
	pageSize = 20

	if raw := strings.TrimSpace(c.Query("account_page")); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 {
			return 0, 0, fmt.Errorf("invalid account_page")
		}
	}

	if raw := strings.TrimSpace(c.Query("account_page_size")); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil || pageSize < 1 || pageSize > 1000 {
			return 0, 0, fmt.Errorf("invalid account_page_size, use 1-1000")
		}
	}

	return page, pageSize, nil
}
func parseUserDashboardTimeRangeStrict(c *gin.Context) (time.Time, time.Time, error) {
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	startTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
	endTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)

	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start_date format, use YYYY-MM-DD")
		}
		startTime = t
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end_date format, use YYYY-MM-DD")
		}
		endTime = t.AddDate(0, 0, 1)
	}
	if !endTime.After(startTime) {
		return time.Time{}, time.Time{}, fmt.Errorf("end_date must be greater than or equal to start_date")
	}
	return startTime, endTime, nil
}
