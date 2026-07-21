package admin

import (
	"fmt"

	"strings"

	"ikik-api/internal/service"
)

// GroupRateScheduleRequest represents one group time-range multiplier rule.
type GroupRateScheduleRequest struct {
	TargetUserID   *int64  `json:"target_user_id"`
	StartMinute    int     `json:"start_minute"`
	EndMinute      int     `json:"end_minute"`
	RateMultiplier float64 `json:"rate_multiplier"`
	Enabled        *bool   `json:"enabled"`
}

// ReplaceGroupRateSchedulesRequest represents replacing all rate schedules for a group.
type ReplaceGroupRateSchedulesRequest struct {
	Entries []GroupRateScheduleRequest `json:"entries"`
}

func parseAdminGroupScope(scope string, includePrivate bool) (string, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		if includePrivate {
			return "all", nil
		}
		return service.GroupScopePublic, nil
	}
	switch scope {
	case "all", service.GroupScopePublic, service.GroupScopeUserPrivate, service.GroupScopeUserCarpool:
		return scope, nil
	default:
		return "", fmt.Errorf("invalid group scope: %s", scope)
	}
}
