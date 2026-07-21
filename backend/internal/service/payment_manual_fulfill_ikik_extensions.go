package service

import (
	"context"

	"fmt"

	"math"

	"strings"
	"time"

	"ikik-api/ent/paymentorder"

	infraerrors "ikik-api/internal/pkg/errors"
)

func (s *PaymentService) AdminManualFulfillOrder(ctx context.Context, oid int64, req AdminManualFulfillmentRequest) error {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return infraerrors.BadRequest("INVALID_REASON", "manual fulfillment reason is required")
	}
	if len([]rune(reason)) > 500 {
		return infraerrors.BadRequest("INVALID_REASON", "manual fulfillment reason is too long")
	}

	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status == OrderStatusCompleted {
		return infraerrors.BadRequest("INVALID_STATUS", "order already completed")
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot fulfill")
	}

	now := time.Now()
	if o.Status == OrderStatusRecharging {
		stale, err := s.isRechargingOrderStale(ctx, o, now)
		if err != nil {
			return err
		}
		if !stale {
			return infraerrors.Conflict("ORDER_PROCESSING", "order is still being fulfilled")
		}
	}

	switch o.Status {
	case OrderStatusPending, OrderStatusExpired, OrderStatusCancelled, OrderStatusFailed, OrderStatusPaid, OrderStatusRecharging:
	default:
		return infraerrors.BadRequest("INVALID_STATUS", "order cannot be manually fulfilled in status "+o.Status)
	}

	paidAmount := o.PayAmount
	if req.PaidAmount != nil {
		if err := validateManualPaidAmount(*req.PaidAmount); err != nil {
			return err
		}
		paidAmount = normalizeManualPaidAmount(*req.PaidAmount)
	}
	tradeNo := strings.TrimSpace(req.TradeNo)
	if len(tradeNo) > 128 {
		return infraerrors.BadRequest("INVALID_TRADE_NO", "trade no is too long")
	}

	if o.Status != OrderStatusPaid {
		up := s.entClient.PaymentOrder.Update().
			Where(paymentorder.IDEQ(oid), paymentorder.StatusEQ(o.Status)).
			SetStatus(OrderStatusPaid).
			SetPayAmount(paidAmount).
			ClearFailedAt().
			ClearFailedReason()
		if o.PaidAt == nil {
			up.SetPaidAt(now)
		}
		if tradeNo != "" {
			up.SetPaymentTradeNo(tradeNo)
		}
		updated, err := up.Save(ctx)
		if err != nil {
			return fmt.Errorf("mark order paid manually: %w", err)
		}
		if updated == 0 {
			return infraerrors.Conflict("CONFLICT", "order status has changed")
		}
	} else if tradeNo != "" || math.Abs(paidAmount-o.PayAmount) > amountToleranceCNY || o.PaidAt == nil {
		up := s.entClient.PaymentOrder.UpdateOneID(oid).
			SetPayAmount(paidAmount)
		if o.PaidAt == nil {
			up.SetPaidAt(now)
		}
		if tradeNo != "" {
			up.SetPaymentTradeNo(tradeNo)
		}
		if _, err := up.Save(ctx); err != nil {
			return fmt.Errorf("update paid order manual details: %w", err)
		}
	}

	s.writeAuditLog(ctx, oid, "ORDER_MANUAL_FULFILL", "admin", map[string]any{
		"reason":            reason,
		"previous_status":   o.Status,
		"expected_pay":      o.PayAmount,
		"manual_paid":       paidAmount,
		"trade_no":          tradeNo,
		"amount_mismatched": math.Abs(paidAmount-o.PayAmount) > amountToleranceCNY,
	})
	return s.executeFulfillment(ctx, oid)
}

func normalizeManualPaidAmount(amount float64) float64 {
	return math.Round(amount*100) / 100
}
func validateManualPaidAmount(amount float64) error {
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return infraerrors.BadRequest("INVALID_PAID_AMOUNT", "paid amount is invalid")
	}
	return nil
}
