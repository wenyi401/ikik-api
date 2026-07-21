package admin

import (
	"context"
	"strconv"

	"ikik-api/internal/handler/dto"
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

type adminPointsService interface {
	UpdateUserPoints(ctx context.Context, userID int64, points float64, operation string, notes string, operatorUserID int64) (*service.User, error)
}

// UpdatePoints handles updating user points.
// POST /api/v1/admin/users/:id/points
func (h *UserHandler) UpdatePoints(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	var req UpdatePointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	operatorUserID, _ := currentAdminUserID(c)
	idempotencyPayload := struct {
		UserID         int64               `json:"user_id"`
		OperatorUserID int64               `json:"operator_user_id"`
		Body           UpdatePointsRequest `json:"body"`
	}{
		UserID:         userID,
		OperatorUserID: operatorUserID,
		Body:           req,
	}
	executeAdminIdempotentJSON(c, "admin.users.points.update", idempotencyPayload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		pointsService, ok := h.adminService.(adminPointsService)
		if !ok {
			return nil, service.ErrServiceUnavailable
		}
		user, execErr := pointsService.UpdateUserPoints(ctx, userID, req.Points, req.Operation, req.Notes, operatorUserID)
		if execErr != nil {
			return nil, execErr
		}
		return dto.UserFromServiceAdmin(user), nil
	})
}
