package admin

import (
	"strconv"

	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

// BindInviter sets or replaces the inviter for a user.
// POST /api/v1/admin/affiliates/users/:user_id/inviter
func (h *AffiliateHandler) BindInviter(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}

	var req BindAffiliateInviterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	summary, err := h.affiliateService.AdminBindInviter(c.Request.Context(), userID, req.InviterUserID, req.ResetValidity)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

// ExtendInviteRewards extends active non-permanent invite reward windows.
// POST /api/v1/admin/affiliates/invite-rewards/extend
func (h *AffiliateHandler) ExtendInviteRewards(c *gin.Context) {
	var req ExtendAffiliateInviteRewardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.affiliateService.AdminExtendInviteRewards(c.Request.Context(), service.AffiliateInviteRewardExtensionRequest{
		Scope:          req.Scope,
		InviterUserID:  req.InviterUserID,
		AllInvitees:    req.AllInvitees,
		InviteeUserIDs: req.InviteeUserIDs,
		ExtendDays:     req.ExtendDays,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
