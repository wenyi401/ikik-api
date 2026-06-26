package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	dbent "ikik-api/ent"
	"ikik-api/internal/pkg/logger"
	"ikik-api/internal/service"
)

type usageBillingRepository struct {
	db *sql.DB
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB}
}

func (r *usageBillingRepository) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (_ *service.UsageBillingApplyResult, err error) {
	if cmd == nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}

	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !applied {
		return &service.UsageBillingApplyResult{Applied: false}, nil
	}

	result := &service.UsageBillingApplyResult{Applied: true}
	if err := r.applyUsageBillingEffects(ctx, tx, cmd, result); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) claimUsageBillingKey(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
		VALUES ($1, $2, $3)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, cmd.RequestID, cmd.APIKeyID).Scan(&existingFingerprint); err != nil {
			return false, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var archivedFingerprint string
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, cmd.RequestID, cmd.APIKeyID).Scan(&archivedFingerprint)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return true, nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	usageLogID, err := ensureUsageBillingLog(ctx, tx, cmd)
	if err != nil {
		return err
	}
	if usageLogID > 0 {
		result.UsageLogID = &usageLogID
	}

	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.SubscriptionCost); err != nil {
			return err
		}
	}
	if cmd.GroupID != nil {
		carpoolCost := cmd.SubscriptionCost
		if carpoolCost <= 0 {
			carpoolCost = cmd.BalanceCost
		}
		if carpoolCost > 0 {
			if err := incrementUsageBillingCarpoolFiveHour(ctx, tx, *cmd.GroupID, cmd.UserID, carpoolCost, cmd.UsageOccurredAt); err != nil {
				return err
			}
		}
	}

	if cmd.BalanceCost > 0 {
		newPointsBalance, newBalance, pointsDeducted, balanceDeducted, err := deductUsageBillingWallet(ctx, tx, cmd.UserID, cmd.BalanceCost, cmd.PreferPointsBilling)
		if err != nil {
			return err
		}
		if pointsDeducted > 0 {
			result.NewPointsBalance = &newPointsBalance
			result.PointsDeducted = pointsDeducted
			if err := insertPointsLedger(ctx, tx, pointsLedgerInput{
				UserID:        cmd.UserID,
				Direction:     "debit",
				Amount:        decimalFromFloat(pointsDeducted),
				Reason:        "usage_charge",
				RefType:       "usage_log",
				RefID:         nullablePositiveInt64(usageLogID),
				BalanceBefore: decimalFromFloat(newPointsBalance + pointsDeducted),
				BalanceAfter:  decimalFromFloat(newPointsBalance),
				Metadata: map[string]any{
					"request_id": cmd.RequestID,
					"api_key_id": cmd.APIKeyID,
					"account_id": cmd.AccountID,
					"total_cost": cmd.BalanceCost,
				},
			}); err != nil {
				return err
			}
		}
		if balanceDeducted > 0 {
			result.NewBalance = &newBalance
			result.BalanceDeducted = balanceDeducted
			result.BalanceOverdrafted = newBalance < 0
			if err := insertUserBalanceLedger(ctx, tx, userBalanceLedgerInput{
				UserID:       cmd.UserID,
				Direction:    "debit",
				Amount:       decimalFromFloat(balanceDeducted),
				Reason:       "usage_charge",
				RefType:      "usage_log",
				RefID:        nullablePositiveInt64(usageLogID),
				BalanceAfter: decimalFromSignedFloat(newBalance),
				Metadata: map[string]any{
					"request_id":      cmd.RequestID,
					"api_key_id":      cmd.APIKeyID,
					"account_id":      cmd.AccountID,
					"total_cost":      cmd.BalanceCost,
					"points_deducted": pointsDeducted,
				},
			}); err != nil {
				return err
			}
		}
	}
	if cmd.PrivateGroupCommissionCost > 0 {
		newBalance, err := deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.PrivateGroupCommissionCost)
		if err != nil {
			return err
		}
		result.NewBalance = &newBalance
		result.BalanceOverdrafted = newBalance < 0
		result.CommissionDeducted = cmd.PrivateGroupCommissionCost
		if err := insertUserBalanceLedger(ctx, tx, userBalanceLedgerInput{
			UserID:       cmd.UserID,
			Direction:    "debit",
			Amount:       decimalFromFloat(cmd.PrivateGroupCommissionCost),
			Reason:       "private_group_commission",
			RefType:      "usage_log",
			RefID:        nullablePositiveInt64(usageLogID),
			BalanceAfter: decimalFromFloat(newBalance),
			Metadata: map[string]any{
				"request_id":      cmd.RequestID,
				"api_key_id":      cmd.APIKeyID,
				"account_id":      cmd.AccountID,
				"group_id":        nullablePositiveInt64Value(cmd.GroupID),
				"subscription_id": nullablePositiveInt64Value(cmd.SubscriptionID),
				"base_cost":       cmd.SubscriptionCost,
			},
		}); err != nil {
			return err
		}
	}

	if cmd.APIKeyQuotaCost > 0 {
		exhausted, err := incrementUsageBillingAPIKeyQuota(ctx, tx, cmd.APIKeyID, cmd.APIKeyQuotaCost)
		if err != nil {
			return err
		}
		result.APIKeyQuotaExhausted = exhausted
	}

	if cmd.APIKeyRateLimitCost > 0 {
		if err := incrementUsageBillingAPIKeyRateLimit(ctx, tx, cmd.APIKeyID, cmd.APIKeyRateLimitCost); err != nil {
			return err
		}
	}

	if cmd.AccountQuotaCost > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
		quotaState, err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCost)
		if err != nil {
			return err
		}
		result.QuotaState = quotaState
	}

	if err := applyAccountShareSettlement(ctx, tx, cmd, usageLogID, result); err != nil {
		return err
	}

	return nil
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64) error {
	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = us.daily_usage_usd + $1,
			weekly_usage_usd = us.weekly_usage_usd + $1,
			monthly_usage_usd = us.monthly_usage_usd + $1,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	return service.ErrSubscriptionNotFound
}

func incrementUsageBillingCarpoolFiveHour(ctx context.Context, tx *sql.Tx, groupID, userID int64, costUSD float64, occurredAt time.Time) error {
	if groupID <= 0 || userID <= 0 || costUSD <= 0 {
		return nil
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE carpool_members AS m
		SET
			five_hour_used_usd = CASE
				WHEN m.five_hour_window_start IS NULL OR m.five_hour_window_start + INTERVAL '5 hours' <= $3
					THEN $1
				ELSE COALESCE(m.five_hour_used_usd, 0) + $1
			END,
			five_hour_window_start = CASE
				WHEN m.five_hour_window_start IS NULL OR m.five_hour_window_start + INTERVAL '5 hours' <= $3
					THEN $3
				ELSE m.five_hour_window_start
			END,
			updated_at = NOW()
		FROM carpool_pools AS p
		WHERE p.id = m.pool_id
			AND p.group_id = $2
			AND m.user_id = $4
			AND p.deleted_at IS NULL
			AND m.deleted_at IS NULL
			AND m.status = 'active'
	`, costUSD, groupID, occurredAt, userID)
	return err
}

func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, error) {
	result, err := debitWalletBuckets(ctx, tx, userID, amount)
	if err != nil {
		return 0, err
	}
	return result.NewBalance, nil
}

func deductUsageBillingWallet(ctx context.Context, tx *sql.Tx, userID int64, amount float64, preferPoints bool) (newPointsBalance float64, newBalance float64, pointsDeducted float64, balanceDeducted float64, err error) {
	if amount <= 0 {
		return 0, 0, 0, 0, nil
	}
	if !preferPoints {
		newBalance, err = deductUsageBillingBalance(ctx, tx, userID, amount)
		return 0, newBalance, 0, amount, err
	}

	var currentBalance float64
	var currentPoints float64
	err = tx.QueryRowContext(ctx, `
		SELECT balance, points_balance
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, userID).Scan(&currentBalance, &currentPoints)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, 0, 0, service.ErrUserNotFound
	}
	if err != nil {
		return 0, 0, 0, 0, err
	}

	pointsDeducted = amount
	if currentPoints < pointsDeducted {
		pointsDeducted = currentPoints
	}
	if pointsDeducted < 0 {
		pointsDeducted = 0
	}
	balanceDeducted = amount - pointsDeducted
	if balanceDeducted < 0 {
		balanceDeducted = 0
	}

	newPointsBalance = currentPoints - pointsDeducted
	newBalance = currentBalance
	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET points_balance = $1::numeric,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`, decimalFromFloat(newPointsBalance).StringFixed(10), userID)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if balanceDeducted > 0 {
		var walletResult walletBucketUpdateResult
		walletResult, err = debitWalletBuckets(ctx, tx, userID, balanceDeducted)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		newBalance = walletResult.NewBalance
	}
	return newPointsBalance, newBalance, pointsDeducted, balanceDeducted, nil
}

func ensureUsageBillingLog(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (int64, error) {
	if cmd == nil || cmd.UsageLog == nil {
		return 0, nil
	}
	log := cmd.UsageLog
	if strings.TrimSpace(log.RequestID) == "" {
		log.RequestID = cmd.RequestID
	}
	if log.APIKeyID == 0 {
		log.APIKeyID = cmd.APIKeyID
	}
	if log.UserID == 0 {
		log.UserID = cmd.UserID
	}
	if log.AccountID == 0 {
		log.AccountID = cmd.AccountID
	}

	usageRepo := &usageLogRepository{sql: tx}
	if _, err := usageRepo.createSingle(ctx, tx, log); err != nil {
		return 0, err
	}
	return log.ID, nil
}

type userBalanceLedgerInput struct {
	UserID       int64
	Direction    string
	Amount       decimal.Decimal
	Reason       string
	RefType      string
	RefID        any
	BalanceAfter decimal.Decimal
	Metadata     map[string]any
}

func insertUserBalanceLedger(ctx context.Context, tx *sql.Tx, in userBalanceLedgerInput) error {
	if in.UserID <= 0 || in.Amount.IsNegative() {
		return nil
	}
	metadata := in.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	rawMetadata, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_balance_ledger (
			user_id, direction, amount, reason, ref_type, ref_id, balance_after, metadata
		) VALUES (
			$1, $2, $3::numeric, $4, $5, $6, $7::numeric, $8::jsonb
		)
		ON CONFLICT DO NOTHING
	`, in.UserID, in.Direction, in.Amount.StringFixed(10), in.Reason, in.RefType, in.RefID, in.BalanceAfter.StringFixed(10), string(rawMetadata))
	return err
}

type pointsLedgerInput struct {
	UserID         int64
	Direction      string
	Amount         decimal.Decimal
	Reason         string
	RefType        string
	RefID          any
	BalanceBefore  decimal.Decimal
	BalanceAfter   decimal.Decimal
	OperatorUserID any
	Metadata       map[string]any
}

func insertPointsLedger(ctx context.Context, tx *sql.Tx, in pointsLedgerInput) error {
	if in.UserID <= 0 || in.Amount.IsNegative() {
		return nil
	}
	metadata := in.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	rawMetadata, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO points_ledger (
			user_id, direction, amount, reason, ref_type, ref_id,
			balance_before, balance_after, operator_user_id, metadata
		) VALUES (
			$1, $2, $3::numeric, $4, $5, $6,
			$7::numeric, $8::numeric, $9, $10::jsonb
		)
		ON CONFLICT DO NOTHING
	`,
		in.UserID, in.Direction, in.Amount.StringFixed(10), in.Reason, in.RefType, in.RefID,
		in.BalanceBefore.StringFixed(10), in.BalanceAfter.StringFixed(10), in.OperatorUserID, string(rawMetadata),
	)
	return err
}

type accountShareSnapshot struct {
	OwnerUserID   int64
	ShareMode     string
	ShareStatus   string
	Platform      string
	SharePolicyID any
}

type accountSharePolicySnapshot struct {
	ID               any
	Version          int
	OwnerShareRatio  decimal.Decimal
	InviteShareRatio decimal.Decimal
}

type accountInviteSnapshot struct {
	InviterUserID int64
	BoundAt       sql.NullTime
	ExpiresAt     sql.NullTime
}

func applyAccountShareSettlement(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, usageLogID int64, result *service.UsageBillingApplyResult) error {
	if cmd == nil || cmd.UserID <= 0 || cmd.AccountID <= 0 {
		return nil
	}
	consumerCharge := accountShareConsumerCharge(cmd)
	if consumerCharge.IsZero() {
		return nil
	}

	account, err := accountShareSnapshotForSettlement(ctx, tx, cmd)
	if err != nil {
		return err
	}
	if account.OwnerUserID <= 0 || account.OwnerUserID == cmd.UserID {
		return nil
	}
	shareMode := service.NormalizeAccountShareMode(account.ShareMode)
	shareStatus := service.NormalizeAccountShareStatus(account.ShareStatus)
	if shareMode != service.AccountShareModePublic || shareStatus != service.AccountShareStatusApproved {
		return nil
	}

	policy, err := resolveAccountSharePolicy(ctx, tx, cmd, account)
	if err != nil {
		return err
	}
	accountCost := accountCostForSettlement(cmd)
	usageOccurredAt := resolveUsageOccurredAt(cmd)
	invite, err := resolveAccountShareInvite(ctx, tx, cmd, policy, usageOccurredAt)
	if err != nil {
		return err
	}
	actualInviteRatio := decimal.Zero
	if invite.InviterUserID > 0 {
		actualInviteRatio = policy.InviteShareRatio
	}
	ownerCredit := consumerCharge.Mul(policy.OwnerShareRatio).Round(10)
	if ownerCredit.GreaterThan(consumerCharge) {
		ownerCredit = consumerCharge
	}
	if ownerCredit.IsNegative() {
		ownerCredit = decimal.Zero
	}
	inviteCredit := consumerCharge.Mul(actualInviteRatio).Round(10)
	if inviteCredit.IsNegative() {
		inviteCredit = decimal.Zero
	}
	remainingAfterOwner := consumerCharge.Sub(ownerCredit)
	if inviteCredit.GreaterThan(remainingAfterOwner) {
		inviteCredit = remainingAfterOwner
	}
	platformFee := consumerCharge.Sub(ownerCredit).Sub(inviteCredit).Round(10)
	if platformFee.IsNegative() {
		platformFee = decimal.Zero
	}
	platformShareRatio := decimal.NewFromInt(1).Sub(policy.OwnerShareRatio).Sub(actualInviteRatio)
	if platformShareRatio.IsNegative() {
		platformShareRatio = decimal.Zero
	}

	inserted, err := insertAccountShareSettlement(ctx, tx, accountShareSettlementInput{
		UsageLogID:          nullablePositiveInt64(usageLogID),
		RequestID:           cmd.RequestID,
		APIKeyID:            cmd.APIKeyID,
		ConsumerUserID:      cmd.UserID,
		OwnerUserID:         account.OwnerUserID,
		AccountID:           cmd.AccountID,
		GroupID:             nullablePtrInt64(cmd.GroupID),
		PolicyID:            policy.ID,
		PolicyVersion:       policy.Version,
		ShareModeSnapshot:   shareMode,
		ShareStatusSnapshot: shareStatus,
		ConsumerCharge:      consumerCharge,
		AccountCost:         accountCost,
		OwnerShareRatio:     policy.OwnerShareRatio,
		OwnerCredit:         ownerCredit,
		InviterUserID:       nullablePositiveInt64(invite.InviterUserID),
		InviteBoundAt:       nullableTime(invite.BoundAt),
		InviteExpiresAt:     nullableTime(invite.ExpiresAt),
		InviteShareRatio:    actualInviteRatio,
		InviteCredit:        inviteCredit,
		PlatformShareRatio:  platformShareRatio,
		PlatformFee:         platformFee,
	})
	if err != nil || !inserted {
		return err
	}

	if !ownerCredit.IsZero() {
		newBalance, err := creditUsageBillingBalance(ctx, tx, account.OwnerUserID, ownerCredit, "share")
		if err != nil {
			return err
		}
		if err := insertUserBalanceLedger(ctx, tx, userBalanceLedgerInput{
			UserID:       account.OwnerUserID,
			Direction:    "credit",
			Amount:       ownerCredit,
			Reason:       "account_share_income",
			RefType:      "usage_log",
			RefID:        nullablePositiveInt64(usageLogID),
			BalanceAfter: decimalFromFloat(newBalance),
			Metadata: map[string]any{
				"request_id":       cmd.RequestID,
				"api_key_id":       cmd.APIKeyID,
				"account_id":       cmd.AccountID,
				"consumer_user_id": cmd.UserID,
			},
		}); err != nil {
			return err
		}
		appendUsageBillingCreditUser(result, account.OwnerUserID)
	}

	if invite.InviterUserID > 0 && !inviteCredit.IsZero() {
		if err := creditInviteShareBalance(ctx, tx, cmd, usageLogID, invite.InviterUserID, inviteCredit); err != nil {
			return err
		}
		appendUsageBillingCreditUser(result, invite.InviterUserID)
	}
	return nil
}

func loadAccountShareSnapshot(ctx context.Context, tx *sql.Tx, accountID int64) (accountShareSnapshot, error) {
	var ownerUserID sql.NullInt64
	var shareMode, shareStatus, platform string
	var sharePolicyID sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT owner_user_id,
			COALESCE(NULLIF(share_mode, ''), 'private'),
			COALESCE(NULLIF(share_status, ''), 'approved'),
			platform,
			share_policy_id
		FROM accounts
		WHERE id = $1 AND deleted_at IS NULL
	`, accountID).Scan(&ownerUserID, &shareMode, &shareStatus, &platform, &sharePolicyID)
	if errors.Is(err, sql.ErrNoRows) {
		return accountShareSnapshot{}, service.ErrAccountNotFound
	}
	if err != nil {
		return accountShareSnapshot{}, err
	}
	out := accountShareSnapshot{
		ShareMode:   shareMode,
		ShareStatus: shareStatus,
		Platform:    strings.TrimSpace(platform),
	}
	if ownerUserID.Valid {
		out.OwnerUserID = ownerUserID.Int64
	}
	if sharePolicyID.Valid {
		out.SharePolicyID = sharePolicyID.Int64
	}
	return out, nil
}

func accountShareSnapshotForSettlement(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (accountShareSnapshot, error) {
	if cmd == nil {
		return accountShareSnapshot{}, nil
	}
	if cmd.ShareSnapshotCaptured {
		out := accountShareSnapshot{
			ShareMode:     cmd.ShareModeSnapshot,
			ShareStatus:   cmd.ShareStatusSnapshot,
			Platform:      strings.TrimSpace(cmd.SharePlatform),
			SharePolicyID: nullablePtrInt64(cmd.SharePolicyID),
		}
		if cmd.ShareOwnerUserID != nil && *cmd.ShareOwnerUserID > 0 {
			out.OwnerUserID = *cmd.ShareOwnerUserID
		}
		return out, nil
	}
	if cmd.ShareOwnerUserID == nil || *cmd.ShareOwnerUserID <= 0 {
		return loadAccountShareSnapshot(ctx, tx, cmd.AccountID)
	}
	out := accountShareSnapshot{
		OwnerUserID:   *cmd.ShareOwnerUserID,
		ShareMode:     cmd.ShareModeSnapshot,
		ShareStatus:   cmd.ShareStatusSnapshot,
		Platform:      strings.TrimSpace(cmd.SharePlatform),
		SharePolicyID: nullablePtrInt64(cmd.SharePolicyID),
	}
	if out.ShareMode == "" || out.ShareStatus == "" {
		dbSnapshot, err := loadAccountShareSnapshot(ctx, tx, cmd.AccountID)
		if err != nil {
			return accountShareSnapshot{}, err
		}
		if out.ShareMode == "" {
			out.ShareMode = dbSnapshot.ShareMode
		}
		if out.ShareStatus == "" {
			out.ShareStatus = dbSnapshot.ShareStatus
		}
		if out.Platform == "" {
			out.Platform = dbSnapshot.Platform
		}
		if out.SharePolicyID == nil {
			out.SharePolicyID = dbSnapshot.SharePolicyID
		}
	}
	return out, nil
}

func accountShareConsumerCharge(cmd *service.UsageBillingCommand) decimal.Decimal {
	if cmd == nil {
		return decimal.Zero
	}
	if cmd.BalanceCost > 0 {
		return decimalFromFloat(cmd.BalanceCost)
	}
	if cmd.SubscriptionCost > 0 {
		return decimalFromFloat(cmd.SubscriptionCost)
	}
	if cmd.UsageLog != nil && cmd.UsageLog.ActualCost > 0 {
		return decimalFromFloat(cmd.UsageLog.ActualCost)
	}
	return decimal.Zero
}

func resolveAccountSharePolicy(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, account accountShareSnapshot) (accountSharePolicySnapshot, error) {
	if cmd != nil && cmd.ShareSnapshotCaptured && usageBillingCommandHasPolicySnapshot(cmd) {
		ratio := decimalFromFloat(cmd.OwnerShareRatio)
		if ratio.IsNegative() {
			ratio = decimal.Zero
		}
		if ratio.GreaterThan(decimal.NewFromInt(1)) {
			ratio = decimal.NewFromInt(1)
		}
		inviteRatio := decimalFromFloat(cmd.InviteShareRatio)
		if inviteRatio.IsNegative() {
			inviteRatio = decimal.Zero
		}
		if inviteRatio.GreaterThan(decimal.NewFromInt(1)) {
			inviteRatio = decimal.NewFromInt(1)
		}
		if ratio.Add(inviteRatio).GreaterThan(decimal.NewFromInt(1)) {
			inviteRatio = decimal.NewFromInt(1).Sub(ratio)
			if inviteRatio.IsNegative() {
				inviteRatio = decimal.Zero
			}
		}
		return accountSharePolicySnapshot{
			ID:               nullablePtrInt64(cmd.SharePolicyID),
			Version:          cmd.SharePolicyVersion,
			OwnerShareRatio:  ratio,
			InviteShareRatio: inviteRatio,
		}, nil
	}
	if policy, found, err := queryAccountSharePolicy(ctx, tx, "scope_type = 'global'", nil); err != nil || found {
		return policy, err
	}
	return accountSharePolicySnapshot{OwnerShareRatio: decimal.Zero}, nil
}

func usageBillingCommandHasPolicySnapshot(cmd *service.UsageBillingCommand) bool {
	if cmd == nil {
		return false
	}
	return cmd.SharePolicyID != nil || cmd.SharePolicyVersion > 0 || cmd.OwnerShareRatio > 0 || cmd.InviteShareRatio > 0
}

func queryAccountSharePolicy(ctx context.Context, tx *sql.Tx, predicate string, arg any) (accountSharePolicySnapshot, bool, error) {
	query := `
		SELECT id, owner_share_ratio, invite_share_ratio, version
		FROM account_share_policies
		WHERE deleted_at IS NULL
			AND enabled = TRUE
			AND effective_at <= NOW()
			AND ` + predicate + `
		ORDER BY effective_at DESC, version DESC, id DESC
		LIMIT 1
	`
	var id int64
	var ratio string
	var inviteRatio string
	var version int
	var err error
	if arg == nil {
		err = tx.QueryRowContext(ctx, query).Scan(&id, &ratio, &inviteRatio, &version)
	} else {
		err = tx.QueryRowContext(ctx, query, arg).Scan(&id, &ratio, &inviteRatio, &version)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return accountSharePolicySnapshot{}, false, nil
	}
	if err != nil {
		return accountSharePolicySnapshot{}, false, err
	}
	parsed, err := decimal.NewFromString(strings.TrimSpace(ratio))
	if err != nil {
		return accountSharePolicySnapshot{}, false, err
	}
	if parsed.IsNegative() {
		parsed = decimal.Zero
	}
	if parsed.GreaterThan(decimal.NewFromInt(1)) {
		parsed = decimal.NewFromInt(1)
	}
	parsedInvite, err := decimal.NewFromString(strings.TrimSpace(inviteRatio))
	if err != nil {
		return accountSharePolicySnapshot{}, false, err
	}
	if parsedInvite.IsNegative() {
		parsedInvite = decimal.Zero
	}
	if parsedInvite.GreaterThan(decimal.NewFromInt(1)) {
		parsedInvite = decimal.NewFromInt(1)
	}
	if parsed.Add(parsedInvite).GreaterThan(decimal.NewFromInt(1)) {
		parsedInvite = decimal.NewFromInt(1).Sub(parsed)
		if parsedInvite.IsNegative() {
			parsedInvite = decimal.Zero
		}
	}
	return accountSharePolicySnapshot{
		ID:               id,
		Version:          version,
		OwnerShareRatio:  parsed,
		InviteShareRatio: parsedInvite,
	}, true, nil
}

func resolveAccountShareInvite(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, policy accountSharePolicySnapshot, usageOccurredAt time.Time) (accountInviteSnapshot, error) {
	if cmd == nil || cmd.BalanceCost <= 0 || policy.InviteShareRatio.IsZero() || policy.InviteShareRatio.IsNegative() {
		return accountInviteSnapshot{}, nil
	}
	if enabled, err := isUsageAffiliateEnabled(ctx, tx); err != nil || !enabled {
		return accountInviteSnapshot{}, err
	}

	var out accountInviteSnapshot
	err := tx.QueryRowContext(ctx, `
		SELECT ua.inviter_id,
			COALESCE(ua.inviter_bound_at, ua.created_at) AS inviter_bound_at,
			ua.invite_reward_expires_at
		FROM user_affiliates ua
		JOIN users inviter
			ON inviter.id = ua.inviter_id
			AND inviter.deleted_at IS NULL
			AND inviter.status = $2
		WHERE ua.user_id = $1
			AND ua.inviter_id IS NOT NULL
			AND ua.inviter_id <> ua.user_id
			AND COALESCE(ua.inviter_bound_at, ua.created_at) <= $3
			AND (ua.invite_reward_expires_at IS NULL OR ua.invite_reward_expires_at > $3)
		LIMIT 1
	`, cmd.UserID, service.StatusActive, usageOccurredAt).Scan(&out.InviterUserID, &out.BoundAt, &out.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return accountInviteSnapshot{}, nil
	}
	if err != nil {
		return accountInviteSnapshot{}, err
	}
	return out, nil
}

func resolveUsageOccurredAt(cmd *service.UsageBillingCommand) time.Time {
	if cmd == nil {
		return time.Now()
	}
	if !cmd.UsageOccurredAt.IsZero() {
		return cmd.UsageOccurredAt
	}
	if cmd.UsageLog != nil && !cmd.UsageLog.CreatedAt.IsZero() {
		return cmd.UsageLog.CreatedAt
	}
	return time.Now()
}

func isUsageAffiliateEnabled(ctx context.Context, tx *sql.Tx) (bool, error) {
	var raw string
	err := tx.QueryRowContext(ctx, `
		SELECT value
		FROM settings
		WHERE key = $1
		LIMIT 1
	`, service.SettingKeyAffiliateEnabled).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return service.AffiliateEnabledDefault, nil
	}
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(raw), "true"), nil
}

type accountShareSettlementInput struct {
	UsageLogID          any
	RequestID           string
	APIKeyID            int64
	ConsumerUserID      int64
	OwnerUserID         int64
	AccountID           int64
	GroupID             any
	PolicyID            any
	PolicyVersion       int
	ShareModeSnapshot   string
	ShareStatusSnapshot string
	ConsumerCharge      decimal.Decimal
	AccountCost         decimal.Decimal
	OwnerShareRatio     decimal.Decimal
	OwnerCredit         decimal.Decimal
	InviterUserID       any
	InviteBoundAt       any
	InviteExpiresAt     any
	InviteShareRatio    decimal.Decimal
	InviteCredit        decimal.Decimal
	PlatformShareRatio  decimal.Decimal
	PlatformFee         decimal.Decimal
}

func insertAccountShareSettlement(ctx context.Context, tx *sql.Tx, in accountShareSettlementInput) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO account_share_settlement_entries (
			usage_log_id, request_id, api_key_id, consumer_user_id, owner_user_id,
			account_id, group_id, policy_id, policy_version,
			share_mode_snapshot, share_status_snapshot,
			consumer_charge, account_cost, owner_share_ratio, owner_credit,
			inviter_user_id, invite_bound_at_snapshot, invite_expires_at_snapshot,
			invite_share_ratio, invite_credit, platform_share_ratio, platform_fee,
			status
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11,
			$12::numeric, $13::numeric, $14::numeric, $15::numeric,
			$16, $17, $18,
			$19::numeric, $20::numeric, $21::numeric, $22::numeric,
			'applied'
		)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`,
		in.UsageLogID, in.RequestID, in.APIKeyID, in.ConsumerUserID, in.OwnerUserID,
		in.AccountID, in.GroupID, in.PolicyID, in.PolicyVersion,
		in.ShareModeSnapshot, in.ShareStatusSnapshot,
		in.ConsumerCharge.StringFixed(10), in.AccountCost.StringFixed(10), in.OwnerShareRatio.StringFixed(6), in.OwnerCredit.StringFixed(10),
		in.InviterUserID, in.InviteBoundAt, in.InviteExpiresAt,
		in.InviteShareRatio.StringFixed(6), in.InviteCredit.StringFixed(10), in.PlatformShareRatio.StringFixed(6), in.PlatformFee.StringFixed(10),
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func creditInviteShareBalance(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, usageLogID int64, inviterUserID int64, amount decimal.Decimal) error {
	newBalance, err := creditUsageBillingBalance(ctx, tx, inviterUserID, amount, "invite")
	if err != nil {
		return err
	}
	if err := insertUserBalanceLedger(ctx, tx, userBalanceLedgerInput{
		UserID:       inviterUserID,
		Direction:    "credit",
		Amount:       amount,
		Reason:       "invite_share_income",
		RefType:      "usage_log",
		RefID:        nullablePositiveInt64(usageLogID),
		BalanceAfter: decimalFromFloat(newBalance),
		Metadata: map[string]any{
			"request_id":       cmd.RequestID,
			"api_key_id":       cmd.APIKeyID,
			"account_id":       cmd.AccountID,
			"consumer_user_id": cmd.UserID,
		},
	}); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE user_affiliates
		SET aff_history_quota = aff_history_quota + $1::numeric,
			updated_at = NOW()
		WHERE user_id = $2
	`, amount.StringFixed(10), inviterUserID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, created_at, updated_at)
		VALUES ($1, 'accrue', $2::numeric, $3, NOW(), NOW())
	`, inviterUserID, amount.StringFixed(10), cmd.UserID)
	return err
}

func appendUsageBillingCreditUser(result *service.UsageBillingApplyResult, userID int64) {
	if result == nil || userID <= 0 {
		return
	}
	for _, existing := range result.BalanceCreditUserIDs {
		if existing == userID {
			return
		}
	}
	result.BalanceCreditUserIDs = append(result.BalanceCreditUserIDs, userID)
}

func creditUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal, bucket string) (float64, error) {
	return creditWalletBucket(ctx, tx, userID, amount.InexactFloat64(), bucket)
}

func accountCostForSettlement(cmd *service.UsageBillingCommand) decimal.Decimal {
	if cmd == nil {
		return decimal.Zero
	}
	if cmd.UsageLog != nil {
		base := cmd.UsageLog.TotalCost
		if cmd.UsageLog.AccountStatsCost != nil {
			base = *cmd.UsageLog.AccountStatsCost
		}
		multiplier := 1.0
		if cmd.UsageLog.AccountRateMultiplier != nil {
			multiplier = *cmd.UsageLog.AccountRateMultiplier
		}
		return decimalFromFloat(base).Mul(decimalFromFloat(multiplier)).Round(10)
	}
	return decimalFromFloat(cmd.AccountQuotaCost)
}

func decimalFromFloat(v float64) decimal.Decimal {
	if v <= 0 {
		return decimal.Zero
	}
	return decimal.NewFromFloat(v).Round(10)
}

func decimalFromSignedFloat(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v).Round(10)
}

func nullablePositiveInt64(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func nullablePositiveInt64Value(v *int64) any {
	if v == nil || *v <= 0 {
		return nil
	}
	return *v
}

func nullablePtrInt64(v *int64) any {
	if v == nil || *v <= 0 {
		return nil
	}
	return *v
}

func nullableTime(v sql.NullTime) any {
	if !v.Valid {
		return nil
	}
	return v.Time
}

func incrementUsageBillingAPIKeyQuota(ctx context.Context, tx *sql.Tx, apiKeyID int64, amount float64) (bool, error) {
	var exhausted bool
	err := tx.QueryRowContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + $1,
			status = CASE
				WHEN quota > 0
					AND status = $3
					AND quota_used < quota
					AND quota_used + $1 >= quota
				THEN $4
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING quota > 0 AND quota_used >= quota AND quota_used - $1 < quota
	`, amount, apiKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).Scan(&exhausted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

func incrementUsageBillingAPIKeyRateLimit(ctx context.Context, tx *sql.Tx, apiKeyID int64, cost float64) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE api_keys SET
			usage_5h = CASE WHEN window_5h_start IS NOT NULL AND window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1 ELSE usage_5h + $1 END,
			usage_1d = CASE WHEN window_1d_start IS NOT NULL AND window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1 ELSE usage_1d + $1 END,
			usage_7d = CASE WHEN window_7d_start IS NOT NULL AND window_7d_start + INTERVAL '7 days' <= NOW() THEN $1 ELSE usage_7d + $1 END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN date_trunc('day', NOW()) ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN date_trunc('day', NOW()) ELSE window_7d_start END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`, cost, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func incrementUsageBillingAccountQuota(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) (*service.AccountQuotaState, error) {
	rows, err := tx.QueryContext(ctx,
		`UPDATE accounts SET extra = (
			COALESCE(extra, '{}'::jsonb)
			|| jsonb_build_object('quota_used', COALESCE((extra->>'quota_used')::numeric, 0) + $1)
			|| CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_daily_used',
					CASE WHEN `+dailyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_daily_used')::numeric, 0) + $1 END,
					'quota_daily_start',
					CASE WHEN `+dailyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_daily_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+dailyExpiredExpr+` AND `+nextDailyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_daily_reset_at', `+nextDailyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
			|| CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_weekly_used',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_weekly_used')::numeric, 0) + $1 END,
					'quota_weekly_start',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_weekly_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+weeklyExpiredExpr+` AND `+nextWeeklyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_weekly_reset_at', `+nextWeeklyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
		), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			COALESCE((extra->>'quota_used')::numeric, 0),
			COALESCE((extra->>'quota_limit')::numeric, 0),
			COALESCE((extra->>'quota_daily_used')::numeric, 0),
			COALESCE((extra->>'quota_daily_limit')::numeric, 0),
			COALESCE((extra->>'quota_weekly_used')::numeric, 0),
			COALESCE((extra->>'quota_weekly_limit')::numeric, 0)`,
		amount, accountID)
	if err != nil {
		return nil, err
	}

	var state service.AccountQuotaState
	if rows.Next() {
		if err := rows.Scan(
			&state.TotalUsed, &state.TotalLimit,
			&state.DailyUsed, &state.DailyLimit,
			&state.WeeklyUsed, &state.WeeklyLimit,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else {
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		return nil, service.ErrAccountNotFound
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	// 必须在执行下一条 SQL 前显式关闭 rows：pq 驱动在同一连接上
	// 不允许前一条查询的结果集未耗尽时启动新查询，否则会返回
	// "unexpected Parse response" 错误。
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// 任意维度额度在本次递增中从"未超"跨越到"已超"时，必须刷新调度快照，
	// 否则 Redis 中缓存的 Account 仍显示旧的 used 值，后续请求会继续选中本账号，
	// 最终观察到 daily_used / weekly_used 大幅超过配置的 limit。
	// 对于日/周额度，即使本次触发了周期重置（pre=0、post=amount），
	// 判定式 (post-amount) < limit 同样成立，逻辑与总额度保持一致。
	crossedTotal := state.TotalLimit > 0 && state.TotalUsed >= state.TotalLimit && (state.TotalUsed-amount) < state.TotalLimit
	crossedDaily := state.DailyLimit > 0 && state.DailyUsed >= state.DailyLimit && (state.DailyUsed-amount) < state.DailyLimit
	crossedWeekly := state.WeeklyLimit > 0 && state.WeeklyUsed >= state.WeeklyLimit && (state.WeeklyUsed-amount) < state.WeeklyLimit
	if crossedTotal || crossedDaily || crossedWeekly {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			logger.LegacyPrintf("repository.usage_billing", "[SchedulerOutbox] enqueue quota exceeded failed: account=%d err=%v", accountID, err)
			return nil, err
		}
	}
	return &state, nil
}
