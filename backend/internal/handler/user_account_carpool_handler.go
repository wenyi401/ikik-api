package handler

import (
	"context"
	"strconv"
	"strings"

	"ikik-api/internal/handler/dto"
	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type createCarpoolPoolRequest struct {
	Name                string  `json:"name" binding:"required"`
	Platform            string  `json:"platform" binding:"required"`
	Visibility          string  `json:"visibility"`
	TargetSeats         int     `json:"target_seats" binding:"required"`
	DurationDays        int     `json:"duration_days"`
	SeatPrice           float64 `json:"seat_price"`
	ExtraFee            float64 `json:"extra_fee"`
	ExtraFeeDescription string  `json:"extra_fee_description"`
	SystemProxyEnabled  bool    `json:"system_proxy_enabled"`
	RiskControlEnabled  bool    `json:"risk_control_enabled"`
	Notes               string  `json:"notes"`
}

type bindCarpoolAccountsRequest struct {
	AccountIDs []int64 `json:"account_ids" binding:"required"`
}

type applyCarpoolPoolRequest struct {
	Note string `json:"note"`
}

type reviewCarpoolJoinRequest struct {
	ReviewNote string `json:"review_note"`
}

type updateCarpoolMemberAllocationsRequest struct {
	Allocations []updateCarpoolMemberAllocationItem `json:"allocations" binding:"required"`
}

type updateCarpoolMemberAllocationItem struct {
	MemberID        int64   `json:"member_id" binding:"required"`
	QuotaShareRatio float64 `json:"quota_share_ratio"`
}

func (h *UserAccountHandler) ensureCarpoolService(c *gin.Context) bool {
	if h.settingService != nil && !h.settingService.IsCarpoolEnabled(c.Request.Context()) {
		response.NotFound(c, "Carpool feature is disabled")
		return false
	}
	if h.carpoolService != nil {
		return true
	}
	response.InternalError(c, "Carpool service is not configured")
	return false
}

func (h *UserAccountHandler) ListCarpools(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	data, err := h.carpoolService.ListMine(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.CarpoolMineOverviewFromService(data))
}

func (h *UserAccountHandler) ListCarpoolHall(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.carpoolService.ListHall(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.CarpoolPoolSummary, 0, len(items))
	for i := range items {
		out = append(out, *dto.CarpoolPoolSummaryFromService(&items[i]))
	}
	response.Success(c, out)
}

func (h *UserAccountHandler) GetCarpoolDetail(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("pool_id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool ID")
		return
	}
	data, err := h.carpoolService.GetDetail(c.Request.Context(), subject.UserID, poolID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.CarpoolPoolDetailFromService(data))
}

func (h *UserAccountHandler) GetCarpoolDetailByInviteCode(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	inviteCode := strings.TrimSpace(c.Param("invite_code"))
	if inviteCode == "" {
		response.BadRequest(c, "Invalid invite code")
		return
	}
	data, err := h.carpoolService.GetDetailByInviteCode(c.Request.Context(), subject.UserID, inviteCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.CarpoolPoolDetailFromService(data))
}

func (h *UserAccountHandler) CreateCarpool(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createCarpoolPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	executeUserIdempotentJSON(c, "user.accounts.carpools.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		detail, err := h.carpoolService.CreatePool(ctx, subject.UserID, service.CreateCarpoolPoolRequest{
			Name:                strings.TrimSpace(req.Name),
			Platform:            strings.TrimSpace(req.Platform),
			Visibility:          strings.TrimSpace(req.Visibility),
			TargetSeats:         req.TargetSeats,
			DurationDays:        req.DurationDays,
			SeatPrice:           req.SeatPrice,
			ExtraFee:            req.ExtraFee,
			ExtraFeeDescription: strings.TrimSpace(req.ExtraFeeDescription),
			SystemProxyEnabled:  req.SystemProxyEnabled,
			RiskControlEnabled:  req.RiskControlEnabled,
			Notes:               strings.TrimSpace(req.Notes),
		})
		if err != nil {
			return nil, err
		}
		return dto.CarpoolPoolDetailFromService(detail), nil
	})
}

func (h *UserAccountHandler) DeleteCarpool(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("pool_id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool ID")
		return
	}
	executeUserIdempotentJSON(c, "user.accounts.carpools.delete", map[string]any{
		"pool_id": poolID,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if err := h.carpoolService.DeletePool(ctx, subject.UserID, poolID); err != nil {
			return nil, err
		}
		return gin.H{"deleted": true}, nil
	})
}

func (h *UserAccountHandler) BindCarpoolAccounts(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("pool_id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool ID")
		return
	}
	var req bindCarpoolAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	executeUserIdempotentJSON(c, "user.accounts.carpools.bind-accounts", map[string]any{
		"pool_id":     poolID,
		"account_ids": req.AccountIDs,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		detail, err := h.carpoolService.BindAccounts(ctx, subject.UserID, poolID, service.BindCarpoolAccountsRequest{
			AccountIDs: req.AccountIDs,
		})
		if err != nil {
			return nil, err
		}
		return dto.CarpoolPoolDetailFromService(detail), nil
	})
}

func (h *UserAccountHandler) ResetCarpoolAccountLocalLimit(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("pool_id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool ID")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	executeUserIdempotentJSON(c, "user.accounts.carpools.reset-account-local-limit", map[string]any{
		"pool_id":    poolID,
		"account_id": accountID,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		detail, err := h.carpoolService.ResetPoolAccountLocalLimit(ctx, subject.UserID, poolID, accountID)
		if err != nil {
			return nil, err
		}
		return dto.CarpoolPoolDetailFromService(detail), nil
	})
}

func (h *UserAccountHandler) ApplyCarpool(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("pool_id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool ID")
		return
	}
	var req applyCarpoolPoolRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	executeUserIdempotentJSON(c, "user.accounts.carpools.apply", map[string]any{
		"pool_id": poolID,
		"note":    req.Note,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		item, err := h.carpoolService.Apply(ctx, subject.UserID, poolID, service.ApplyCarpoolPoolRequest{
			Note: strings.TrimSpace(req.Note),
		})
		if err != nil {
			return nil, err
		}
		return dto.CarpoolJoinRequestFromService(item), nil
	})
}

func (h *UserAccountHandler) ApplyCarpoolByInviteCode(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	inviteCode := strings.TrimSpace(c.Param("invite_code"))
	if inviteCode == "" {
		response.BadRequest(c, "Invalid invite code")
		return
	}
	var req applyCarpoolPoolRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	executeUserIdempotentJSON(c, "user.accounts.carpools.apply-invite", map[string]any{
		"invite_code": inviteCode,
		"note":        req.Note,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		item, err := h.carpoolService.ApplyByInviteCode(ctx, subject.UserID, inviteCode, service.ApplyCarpoolPoolRequest{
			Note: strings.TrimSpace(req.Note),
		})
		if err != nil {
			return nil, err
		}
		return dto.CarpoolJoinRequestFromService(item), nil
	})
}

func (h *UserAccountHandler) ApproveCarpoolJoinRequest(c *gin.Context) {
	h.reviewCarpoolJoinRequest(c, true)
}

func (h *UserAccountHandler) RejectCarpoolJoinRequest(c *gin.Context) {
	h.reviewCarpoolJoinRequest(c, false)
}

func (h *UserAccountHandler) reviewCarpoolJoinRequest(c *gin.Context, approve bool) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("pool_id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool ID")
		return
	}
	requestID, err := strconv.ParseInt(c.Param("request_id"), 10, 64)
	if err != nil || requestID <= 0 {
		response.BadRequest(c, "Invalid request ID")
		return
	}
	var req reviewCarpoolJoinRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	scope := "user.accounts.carpools.reject"
	if approve {
		scope = "user.accounts.carpools.approve"
	}
	executeUserIdempotentJSON(c, scope, map[string]any{
		"pool_id":     poolID,
		"request_id":  requestID,
		"review_note": req.ReviewNote,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		var (
			item *service.CarpoolJoinRequest
			err  error
		)
		if approve {
			item, err = h.carpoolService.ApproveJoinRequest(ctx, subject.UserID, poolID, requestID, service.ReviewCarpoolJoinRequest{
				ReviewNote: strings.TrimSpace(req.ReviewNote),
			})
		} else {
			item, err = h.carpoolService.RejectJoinRequest(ctx, subject.UserID, poolID, requestID, service.ReviewCarpoolJoinRequest{
				ReviewNote: strings.TrimSpace(req.ReviewNote),
			})
		}
		if err != nil {
			return nil, err
		}
		return dto.CarpoolJoinRequestFromService(item), nil
	})
}

func (h *UserAccountHandler) ConfirmCarpoolJoinPaid(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("pool_id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool ID")
		return
	}
	requestID, err := strconv.ParseInt(c.Param("request_id"), 10, 64)
	if err != nil || requestID <= 0 {
		response.BadRequest(c, "Invalid request ID")
		return
	}
	executeUserIdempotentJSON(c, "user.accounts.carpools.confirm-paid", map[string]any{
		"pool_id":    poolID,
		"request_id": requestID,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		detail, err := h.carpoolService.ConfirmJoinPaid(ctx, subject.UserID, poolID, requestID)
		if err != nil {
			return nil, err
		}
		return dto.CarpoolPoolDetailFromService(detail), nil
	})
}

func (h *UserAccountHandler) RemoveCarpoolMember(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("pool_id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool ID")
		return
	}
	memberID, err := strconv.ParseInt(c.Param("member_id"), 10, 64)
	if err != nil || memberID <= 0 {
		response.BadRequest(c, "Invalid member ID")
		return
	}
	executeUserIdempotentJSON(c, "user.accounts.carpools.remove-member", map[string]any{
		"pool_id":   poolID,
		"member_id": memberID,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		detail, err := h.carpoolService.RemoveMember(ctx, subject.UserID, poolID, memberID)
		if err != nil {
			return nil, err
		}
		return dto.CarpoolPoolDetailFromService(detail), nil
	})
}

func (h *UserAccountHandler) UpdateCarpoolMemberAllocations(c *gin.Context) {
	if !h.ensureCarpoolService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("pool_id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool ID")
		return
	}
	var req updateCarpoolMemberAllocationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	allocations := make([]service.CarpoolMemberAllocationInput, 0, len(req.Allocations))
	for _, item := range req.Allocations {
		allocations = append(allocations, service.CarpoolMemberAllocationInput{
			MemberID:        item.MemberID,
			QuotaShareRatio: item.QuotaShareRatio,
		})
	}
	executeUserIdempotentJSON(c, "user.accounts.carpools.member-allocations", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		detail, err := h.carpoolService.UpdateMemberAllocations(ctx, subject.UserID, poolID, service.UpdateCarpoolMemberAllocationsRequest{
			Allocations: allocations,
		})
		if err != nil {
			return nil, err
		}
		return dto.CarpoolPoolDetailFromService(detail), nil
	})
}
