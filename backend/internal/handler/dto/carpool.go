package dto

import (
	"time"

	"ikik-api/internal/service"
)

type CarpoolMineOverview struct {
	Owned  []CarpoolPoolSummary `json:"owned"`
	Joined []CarpoolPoolSummary `json:"joined"`
}

type CarpoolPool struct {
	ID                        int64      `json:"id"`
	OwnerUserID               int64      `json:"owner_user_id"`
	GroupID                   *int64     `json:"group_id,omitempty"`
	InviteCode                string     `json:"invite_code"`
	Name                      string     `json:"name"`
	Platform                  string     `json:"platform"`
	Status                    string     `json:"status"`
	Visibility                string     `json:"visibility"`
	TargetSeats               int        `json:"target_seats"`
	DurationDays              int        `json:"duration_days"`
	SeatPrice                 float64    `json:"seat_price"`
	ExtraFee                  float64    `json:"extra_fee"`
	ExtraFeeDescription       string     `json:"extra_fee_description"`
	SystemProxyEnabled        bool       `json:"system_proxy_enabled"`
	RiskControlEnabled        bool       `json:"risk_control_enabled"`
	Notes                     string     `json:"notes"`
	TotalFiveHourLimitUSD     float64    `json:"total_five_hour_limit_usd"`
	TotalWeeklyLimitUSD       float64    `json:"total_weekly_limit_usd"`
	PerMemberFiveHourLimitUSD float64    `json:"per_member_five_hour_limit_usd"`
	PerMemberWeeklyLimitUSD   float64    `json:"per_member_weekly_limit_usd"`
	QuotaSnapshotAt           *time.Time `json:"quota_snapshot_at,omitempty"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type CarpoolPoolSummary struct {
	Pool                 CarpoolPool `json:"pool"`
	GroupName            string      `json:"group_name"`
	ActiveMembers        int         `json:"active_members"`
	PendingApplications  int         `json:"pending_applications"`
	BoundAccountCount    int         `json:"bound_account_count"`
	IsOwner              bool        `json:"is_owner"`
	CurrentUserStatus    string      `json:"current_user_status"`
	CurrentUserRequestID *int64      `json:"current_user_request_id,omitempty"`
}

type AdminCarpoolPoolSummary struct {
	CarpoolPoolSummary
	OwnerEmail    string `json:"owner_email"`
	OwnerUsername string `json:"owner_username"`
}

type CarpoolPoolAccount struct {
	ID           int64     `json:"id"`
	PoolID       int64     `json:"pool_id"`
	AccountID    int64     `json:"account_id"`
	Name         string    `json:"name"`
	Platform     string    `json:"platform"`
	Type         string    `json:"type"`
	AccountLevel string    `json:"account_level"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type CarpoolMember struct {
	ID                  int64      `json:"id"`
	PoolID              int64      `json:"pool_id"`
	UserID              int64      `json:"user_id"`
	SubscriptionID      *int64     `json:"subscription_id,omitempty"`
	Role                string     `json:"role"`
	Status              string     `json:"status"`
	PaidConfirmedAt     *time.Time `json:"paid_confirmed_at,omitempty"`
	QuotaShareRatio     float64    `json:"quota_share_ratio"`
	FiveHourLimitUSD    float64    `json:"five_hour_limit_usd"`
	FiveHourUsedUSD     float64    `json:"five_hour_used_usd"`
	WeeklyLimitUSD      float64    `json:"weekly_limit_usd"`
	FiveHourWindowStart *time.Time `json:"five_hour_window_start,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type CarpoolMemberProfile struct {
	Member         CarpoolMember        `json:"member"`
	MaskedEmail    string               `json:"masked_email"`
	Username       string               `json:"username"`
	WeeklyLimitUSD float64              `json:"weekly_limit_usd"`
	WeeklyUsageUSD float64              `json:"weekly_usage_usd"`
	WeeklyResetAt  *time.Time           `json:"weekly_reset_at,omitempty"`
	UsageWindows   []CarpoolUsageWindow `json:"usage_windows"`
	TotalTokens    int64                `json:"total_tokens"`
	TotalCostUSD   float64              `json:"total_cost_usd"`
}

type CarpoolUsageWindow struct {
	Window          string     `json:"window"`
	UsedPoints      float64    `json:"used_points"`
	LimitPoints     float64    `json:"limit_points"`
	RemainingPoints float64    `json:"remaining_points"`
	Utilization     float64    `json:"utilization"`
	ResetAt         *time.Time `json:"reset_at,omitempty"`
}

type CarpoolApplicantUsageStats struct {
	TotalRequests   int64 `json:"total_requests"`
	TotalTokens     int64 `json:"total_tokens"`
	Last7dRequests  int64 `json:"last_7d_requests"`
	Last7dTokens    int64 `json:"last_7d_tokens"`
	Last30dRequests int64 `json:"last_30d_requests"`
	Last30dTokens   int64 `json:"last_30d_tokens"`
}

type CarpoolJoinRequest struct {
	ID          int64      `json:"id"`
	PoolID      int64      `json:"pool_id"`
	UserID      int64      `json:"user_id"`
	Status      string     `json:"status"`
	Note        string     `json:"note"`
	ReviewNote  string     `json:"review_note"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CarpoolJoinRequestProfile struct {
	Request     CarpoolJoinRequest         `json:"request"`
	MaskedEmail string                     `json:"masked_email"`
	Username    string                     `json:"username"`
	Usage       CarpoolApplicantUsageStats `json:"usage"`
}

type CarpoolPoolDetail struct {
	Pool             CarpoolPool                 `json:"pool"`
	Group            *Group                      `json:"group,omitempty"`
	Summary          CarpoolPoolSummary          `json:"summary"`
	Accounts         []CarpoolPoolAccount        `json:"accounts"`
	PoolUsageWindows []CarpoolUsageWindow        `json:"pool_usage_windows"`
	Members          []CarpoolMemberProfile      `json:"members"`
	JoinRequests     []CarpoolJoinRequestProfile `json:"join_requests"`
}

func CarpoolMineOverviewFromService(v *service.CarpoolMineOverview) *CarpoolMineOverview {
	if v == nil {
		return nil
	}
	out := &CarpoolMineOverview{
		Owned:  make([]CarpoolPoolSummary, 0, len(v.Owned)),
		Joined: make([]CarpoolPoolSummary, 0, len(v.Joined)),
	}
	for i := range v.Owned {
		out.Owned = append(out.Owned, *CarpoolPoolSummaryFromService(&v.Owned[i]))
	}
	for i := range v.Joined {
		out.Joined = append(out.Joined, *CarpoolPoolSummaryFromService(&v.Joined[i]))
	}
	return out
}

func CarpoolPoolFromService(v *service.CarpoolPool) *CarpoolPool {
	if v == nil {
		return nil
	}
	return &CarpoolPool{
		ID:                        v.ID,
		OwnerUserID:               v.OwnerUserID,
		GroupID:                   v.GroupID,
		InviteCode:                v.InviteCode,
		Name:                      v.Name,
		Platform:                  v.Platform,
		Status:                    v.Status,
		Visibility:                v.Visibility,
		TargetSeats:               v.TargetSeats,
		DurationDays:              v.DurationDays,
		SeatPrice:                 v.SeatPrice,
		ExtraFee:                  v.ExtraFee,
		ExtraFeeDescription:       v.ExtraFeeDescription,
		SystemProxyEnabled:        v.SystemProxyEnabled,
		RiskControlEnabled:        v.RiskControlEnabled,
		Notes:                     v.Notes,
		TotalFiveHourLimitUSD:     v.TotalFiveHourLimitUSD,
		TotalWeeklyLimitUSD:       v.TotalWeeklyLimitUSD,
		PerMemberFiveHourLimitUSD: v.PerMemberFiveHourLimitUSD,
		PerMemberWeeklyLimitUSD:   v.PerMemberWeeklyLimitUSD,
		QuotaSnapshotAt:           v.QuotaSnapshotAt,
		CreatedAt:                 v.CreatedAt,
		UpdatedAt:                 v.UpdatedAt,
	}
}

func CarpoolPoolSummaryFromService(v *service.CarpoolPoolSummary) *CarpoolPoolSummary {
	if v == nil {
		return nil
	}
	return &CarpoolPoolSummary{
		Pool:                 *CarpoolPoolFromService(&v.Pool),
		GroupName:            v.GroupName,
		ActiveMembers:        v.ActiveMembers,
		PendingApplications:  v.PendingApplications,
		BoundAccountCount:    v.BoundAccountCount,
		IsOwner:              v.IsOwner,
		CurrentUserStatus:    v.CurrentUserStatus,
		CurrentUserRequestID: v.CurrentUserRequestID,
	}
}

func AdminCarpoolPoolSummaryFromService(v *service.AdminCarpoolPoolSummary) *AdminCarpoolPoolSummary {
	if v == nil {
		return nil
	}
	return &AdminCarpoolPoolSummary{
		CarpoolPoolSummary: *CarpoolPoolSummaryFromService(&v.CarpoolPoolSummary),
		OwnerEmail:         v.OwnerEmail,
		OwnerUsername:      v.OwnerUsername,
	}
}

func CarpoolPoolAccountFromService(v *service.CarpoolPoolAccount) *CarpoolPoolAccount {
	if v == nil {
		return nil
	}
	return &CarpoolPoolAccount{
		ID:           v.ID,
		PoolID:       v.PoolID,
		AccountID:    v.AccountID,
		Name:         v.Name,
		Platform:     v.Platform,
		Type:         v.Type,
		AccountLevel: v.AccountLevel,
		Status:       v.Status,
		CreatedAt:    v.CreatedAt,
	}
}

func CarpoolMemberFromService(v *service.CarpoolMember) *CarpoolMember {
	if v == nil {
		return nil
	}
	return &CarpoolMember{
		ID:                  v.ID,
		PoolID:              v.PoolID,
		UserID:              v.UserID,
		SubscriptionID:      v.SubscriptionID,
		Role:                v.Role,
		Status:              v.Status,
		PaidConfirmedAt:     v.PaidConfirmedAt,
		QuotaShareRatio:     v.QuotaShareRatio,
		FiveHourLimitUSD:    v.FiveHourLimitUSD,
		FiveHourUsedUSD:     v.FiveHourUsedUSD,
		WeeklyLimitUSD:      v.WeeklyLimitUSD,
		FiveHourWindowStart: v.FiveHourWindowStart,
		CreatedAt:           v.CreatedAt,
		UpdatedAt:           v.UpdatedAt,
	}
}

func CarpoolUsageWindowFromService(v *service.CarpoolUsageWindow) *CarpoolUsageWindow {
	if v == nil {
		return nil
	}
	return &CarpoolUsageWindow{
		Window:          v.Window,
		UsedPoints:      v.UsedPoints,
		LimitPoints:     v.LimitPoints,
		RemainingPoints: v.RemainingPoints,
		Utilization:     v.Utilization,
		ResetAt:         v.ResetAt,
	}
}

func CarpoolJoinRequestFromService(v *service.CarpoolJoinRequest) *CarpoolJoinRequest {
	if v == nil {
		return nil
	}
	return &CarpoolJoinRequest{
		ID:          v.ID,
		PoolID:      v.PoolID,
		UserID:      v.UserID,
		Status:      v.Status,
		Note:        v.Note,
		ReviewNote:  v.ReviewNote,
		ReviewedAt:  v.ReviewedAt,
		ActivatedAt: v.ActivatedAt,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}

func CarpoolPoolDetailFromService(v *service.CarpoolPoolDetail) *CarpoolPoolDetail {
	if v == nil {
		return nil
	}
	out := &CarpoolPoolDetail{
		Pool:             *CarpoolPoolFromService(&v.Pool),
		Summary:          *CarpoolPoolSummaryFromService(&v.Summaries),
		Accounts:         make([]CarpoolPoolAccount, 0, len(v.Accounts)),
		PoolUsageWindows: make([]CarpoolUsageWindow, 0, len(v.PoolUsageWindows)),
		Members:          make([]CarpoolMemberProfile, 0, len(v.Members)),
		JoinRequests:     make([]CarpoolJoinRequestProfile, 0, len(v.JoinRequests)),
	}
	if v.Group != nil {
		out.Group = GroupFromService(v.Group)
	}
	for i := range v.Accounts {
		out.Accounts = append(out.Accounts, *CarpoolPoolAccountFromService(&v.Accounts[i]))
	}
	for i := range v.PoolUsageWindows {
		out.PoolUsageWindows = append(out.PoolUsageWindows, *CarpoolUsageWindowFromService(&v.PoolUsageWindows[i]))
	}
	for i := range v.Members {
		profile := CarpoolMemberProfile{
			Member:         *CarpoolMemberFromService(&v.Members[i].Member),
			MaskedEmail:    v.Members[i].MaskedEmail,
			Username:       v.Members[i].Username,
			WeeklyLimitUSD: v.Members[i].WeeklyLimitUSD,
			WeeklyUsageUSD: v.Members[i].WeeklyUsageUSD,
			WeeklyResetAt:  v.Members[i].WeeklyResetAt,
			UsageWindows:   make([]CarpoolUsageWindow, 0, len(v.Members[i].UsageWindows)),
			TotalTokens:    v.Members[i].TotalTokens,
			TotalCostUSD:   v.Members[i].TotalCostUSD,
		}
		for j := range v.Members[i].UsageWindows {
			profile.UsageWindows = append(profile.UsageWindows, *CarpoolUsageWindowFromService(&v.Members[i].UsageWindows[j]))
		}
		out.Members = append(out.Members, profile)
	}
	for i := range v.JoinRequests {
		out.JoinRequests = append(out.JoinRequests, CarpoolJoinRequestProfile{
			Request:     *CarpoolJoinRequestFromService(&v.JoinRequests[i].Request),
			MaskedEmail: v.JoinRequests[i].MaskedEmail,
			Username:    v.JoinRequests[i].Username,
			Usage: CarpoolApplicantUsageStats{
				TotalRequests:   v.JoinRequests[i].Usage.TotalRequests,
				TotalTokens:     v.JoinRequests[i].Usage.TotalTokens,
				Last7dRequests:  v.JoinRequests[i].Usage.Last7dRequests,
				Last7dTokens:    v.JoinRequests[i].Usage.Last7dTokens,
				Last30dRequests: v.JoinRequests[i].Usage.Last30dRequests,
				Last30dTokens:   v.JoinRequests[i].Usage.Last30dTokens,
			},
		})
	}
	return out
}
