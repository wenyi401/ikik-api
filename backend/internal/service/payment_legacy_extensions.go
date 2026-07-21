package service

import (
	"context"

	"time"

	dbent "ikik-api/ent"
)

type AdminManualFulfillmentRequest struct {
	PaidAmount *float64 `json:"paid_amount"`
	TradeNo    string   `json:"trade_no"`
	Reason     string   `json:"reason"`
}

type ShopPaymentDeliveryReader interface {
	GetOrderForAdmin(ctx context.Context, orderID int64) (*ShopOrderDTO, error)
}
type ShopPaymentFulfillment interface {
	ConfirmPaidAndDeliver(ctx context.Context, paymentOrderID int64) error
	CancelPendingPayment(ctx context.Context, paymentOrderID int64, shopStatus string) error
	CancelPendingPaymentInTx(ctx context.Context, tx *dbent.Tx, paymentOrderID int64, shopStatus string) error
	ReleaseStalePaymentReservations(ctx context.Context, cutoff time.Time) error
}
