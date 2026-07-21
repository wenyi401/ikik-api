package service

import (
	"context"
	"fmt"

	"time"

	dbent "ikik-api/ent"
)

func (s *PaymentService) isRechargingOrderStale(ctx context.Context, order *dbent.PaymentOrder, now time.Time) (bool, error) {
	if order == nil || order.Status != OrderStatusRecharging {
		return false, nil
	}
	timeout, err := s.rechargingTimeout(ctx)
	if err != nil {
		return false, err
	}
	return !order.UpdatedAt.After(now.Add(-timeout)), nil
}
func (s *PaymentService) rechargingTimeout(ctx context.Context) (time.Duration, error) {
	timeoutMin := defaultOrderTimeoutMin
	if s.configService != nil {
		cfg, err := s.configService.GetPaymentConfig(ctx)
		if err != nil {
			return 0, fmt.Errorf("load payment config for fulfillment timeout: %w", err)
		}
		timeoutMin = cfg.OrderTimeoutMin
	}
	if timeoutMin < paymentGraceMinutes {
		timeoutMin = paymentGraceMinutes
	}
	return time.Duration(timeoutMin) * time.Minute, nil
}
