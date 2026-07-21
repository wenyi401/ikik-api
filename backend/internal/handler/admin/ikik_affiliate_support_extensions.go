package admin

type BindAffiliateInviterRequest struct {
	InviterUserID int64 `json:"inviter_user_id" binding:"required"`
	ResetValidity bool  `json:"reset_validity"`
}

type ExtendAffiliateInviteRewardsRequest struct {
	Scope          string  `json:"scope" binding:"required"`
	InviterUserID  int64   `json:"inviter_user_id"`
	AllInvitees    bool    `json:"all_invitees"`
	InviteeUserIDs []int64 `json:"invitee_user_ids"`
	ExtendDays     int     `json:"extend_days" binding:"required"`
}
