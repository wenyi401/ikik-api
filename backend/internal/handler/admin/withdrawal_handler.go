package admin

import (
	"strconv"

	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type WithdrawalHandler struct {
	withdrawalService *service.WithdrawalService
}

func NewWithdrawalHandler(withdrawalService *service.WithdrawalService) *WithdrawalHandler {
	return &WithdrawalHandler{withdrawalService: withdrawalService}
}

type processWithdrawalRequest struct {
	Note string `json:"note"`
}

func (h *WithdrawalHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var userID int64
	if raw := c.Query("user_id"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			userID = parsed
		}
	}
	items, total, err := h.withdrawalService.AdminList(c.Request.Context(), service.WithdrawalListParams{
		Page:          page,
		PageSize:      pageSize,
		UserID:        userID,
		Status:        c.Query("status"),
		Keyword:       c.Query("keyword"),
		PaymentMethod: c.Query("payment_method"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *WithdrawalHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	out, err := h.withdrawalService.AdminGet(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *WithdrawalHandler) Settle(c *gin.Context) {
	h.process(c, true)
}

func (h *WithdrawalHandler) Reject(c *gin.Context) {
	h.process(c, false)
}

func (h *WithdrawalHandler) process(c *gin.Context, settle bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req processWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var out *service.WithdrawalRequest
	if settle {
		out, err = h.withdrawalService.AdminSettle(c.Request.Context(), id, subject.UserID, req.Note)
	} else {
		out, err = h.withdrawalService.AdminReject(c.Request.Context(), id, subject.UserID, req.Note)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}
