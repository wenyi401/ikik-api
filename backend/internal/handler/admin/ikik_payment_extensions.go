package admin

import (
	"ikik-api/internal/pkg/response"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

// ManualFulfillOrder marks an externally verified order as paid and executes fulfillment.
// POST /api/v1/admin/payment/orders/:id/manual-fulfill
func (h *PaymentHandler) ManualFulfillOrder(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req service.AdminManualFulfillmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.paymentService.AdminManualFulfillOrder(c.Request.Context(), orderID, req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "manual fulfillment completed"})
}
