package service

import (
	"context"

	"fmt"

	"time"

	dbent "ikik-api/ent"

	infraerrors "ikik-api/internal/pkg/errors"

	"ikik-api/internal/pkg/logger"
)

func (s *adminServiceImpl) UpdateUserPoints(ctx context.Context, userID int64, points float64, operation string, notes string, operatorUserID int64) (*User, error) {
	if points <= 0 {
		return nil, infraerrors.BadRequest("POINTS_AMOUNT_INVALID", "points amount must be greater than 0")
	}

	var delta float64
	switch operation {
	case "set":
	case "add":
		delta = points
	case "subtract":
		delta = -points
	default:
		return nil, infraerrors.BadRequest("POINTS_OPERATION_INVALID", "invalid points operation")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin points adjustment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if operation == "set" {
		currentPoints, err := currentPointsBalanceInTx(txCtx, tx, userID)
		if err != nil {
			return nil, fmt.Errorf("lock user points: %w", err)
		}
		delta = points - currentPoints
	}
	if delta == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit noop points adjustment transaction: %w", err)
		}
		updated, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		return updated, nil
	}

	code, err := GenerateRedeemCode()
	if err != nil {
		return nil, fmt.Errorf("generate points adjustment code: %w", err)
	}
	now := time.Now()
	adjustmentRecord := &RedeemCode{
		Code:   code,
		Type:   AdjustmentTypeAdminPoints,
		Value:  delta,
		Status: StatusUsed,
		UsedBy: &userID,
		UsedAt: &now,
		Notes:  notes,
	}
	if err := s.redeemCodeRepo.Create(txCtx, adjustmentRecord); err != nil {
		return nil, fmt.Errorf("create points adjustment redeem code: %w", err)
	}
	if err := applyPointsAdjustmentInTx(txCtx, tx, pointsAdjustmentInput{
		UserID:         userID,
		Delta:          delta,
		Reason:         "admin_adjustment",
		RefType:        "redeem_code",
		RefID:          adjustmentRecord.ID,
		OperatorUserID: operatorUserID,
		Metadata: map[string]any{
			"operation": operation,
			"notes":     notes,
		},
	}); err != nil {
		return nil, fmt.Errorf("update user points: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit points adjustment transaction: %w", err)
	}

	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.billingCacheService.InvalidateUserBalance(cacheCtx, userID); err != nil {
				logger.LegacyPrintf("service.admin", "invalidate user balance cache after points update failed: user_id=%d err=%v", userID, err)
			}
		}()
	}

	updated, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}
