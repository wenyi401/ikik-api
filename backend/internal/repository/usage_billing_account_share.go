package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"ikik-api/internal/service"
)

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

func applyAccountShareSettlement(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	if cmd == nil || cmd.UserID <= 0 || cmd.AccountID <= 0 {
		return nil
	}
	consumerCharge := accountShareConsumerCharge(cmd)
	if !consumerCharge.IsPositive() {
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

	usageLogID, err := ensureAccountShareUsageLog(ctx, tx, cmd)
	if err != nil {
		return err
	}
	if usageLogID > 0 && result != nil {
		result.UsageLogID = &usageLogID
	}

	policy, err := resolveAccountSharePolicy(ctx, tx, cmd)
	if err != nil {
		return err
	}
	usageOccurredAt := resolveAccountShareUsageOccurredAt(cmd)
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
		GroupID:             nullablePositiveInt64Ptr(cmd.GroupID),
		PolicyID:            policy.ID,
		PolicyVersion:       policy.Version,
		ShareModeSnapshot:   shareMode,
		ShareStatusSnapshot: shareStatus,
		ConsumerCharge:      consumerCharge,
		AccountCost:         accountShareAccountCost(cmd),
		OwnerShareRatio:     policy.OwnerShareRatio,
		OwnerCredit:         ownerCredit,
		InviterUserID:       nullablePositiveInt64(invite.InviterUserID),
		InviteBoundAt:       nullableAccountShareTime(invite.BoundAt),
		InviteExpiresAt:     nullableAccountShareTime(invite.ExpiresAt),
		InviteShareRatio:    actualInviteRatio,
		InviteCredit:        inviteCredit,
		PlatformShareRatio:  platformShareRatio,
		PlatformFee:         platformFee,
	})
	if err != nil || !inserted {
		return err
	}

	if ownerCredit.IsPositive() {
		newBalance, err := creditWalletBucket(ctx, tx, account.OwnerUserID, ownerCredit.InexactFloat64(), "share")
		if err != nil {
			return err
		}
		if err := insertAccountShareBalanceLedger(ctx, tx, account.OwnerUserID, ownerCredit, newBalance, "account_share_income", usageLogID, cmd); err != nil {
			return err
		}
		appendAccountShareCreditUser(result, account.OwnerUserID)
	}

	if invite.InviterUserID > 0 && inviteCredit.IsPositive() {
		if err := creditInviteShareBalance(ctx, tx, cmd, usageLogID, invite.InviterUserID, inviteCredit); err != nil {
			return err
		}
		appendAccountShareCreditUser(result, invite.InviterUserID)
	}
	return nil
}

func ensureAccountShareUsageLog(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (int64, error) {
	if cmd == nil || cmd.UsageLog == nil {
		return 0, nil
	}
	log := cmd.UsageLog
	if log.ID > 0 {
		return log.ID, nil
	}
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
	out := accountShareSnapshot{ShareMode: shareMode, ShareStatus: shareStatus, Platform: strings.TrimSpace(platform)}
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
			SharePolicyID: nullablePositiveInt64Ptr(cmd.SharePolicyID),
		}
		if cmd.ShareOwnerUserID != nil && *cmd.ShareOwnerUserID > 0 {
			out.OwnerUserID = *cmd.ShareOwnerUserID
		}
		return out, nil
	}
	return loadAccountShareSnapshot(ctx, tx, cmd.AccountID)
}

func accountShareConsumerCharge(cmd *service.UsageBillingCommand) decimal.Decimal {
	if cmd == nil {
		return decimal.Zero
	}
	if cmd.BalanceCost > 0 {
		return accountShareDecimal(cmd.BalanceCost)
	}
	if cmd.SubscriptionCost > 0 {
		return accountShareDecimal(cmd.SubscriptionCost)
	}
	if cmd.UsageLog != nil && cmd.UsageLog.ActualCost > 0 {
		return accountShareDecimal(cmd.UsageLog.ActualCost)
	}
	return decimal.Zero
}

func resolveAccountSharePolicy(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (accountSharePolicySnapshot, error) {
	if cmd != nil && cmd.ShareSnapshotCaptured && accountShareCommandHasPolicySnapshot(cmd) {
		ownerRatio := clampAccountShareRatio(accountShareDecimal(cmd.OwnerShareRatio))
		inviteRatio := clampAccountShareRatio(accountShareDecimal(cmd.InviteShareRatio))
		if ownerRatio.Add(inviteRatio).GreaterThan(decimal.NewFromInt(1)) {
			inviteRatio = decimal.NewFromInt(1).Sub(ownerRatio)
		}
		return accountSharePolicySnapshot{
			ID:               nullablePositiveInt64Ptr(cmd.SharePolicyID),
			Version:          cmd.SharePolicyVersion,
			OwnerShareRatio:  ownerRatio,
			InviteShareRatio: inviteRatio,
		}, nil
	}
	if policy, found, err := queryAccountSharePolicy(ctx, tx, "scope_type = 'global'", nil); err != nil || found {
		return policy, err
	}
	return accountSharePolicySnapshot{}, nil
}

func accountShareCommandHasPolicySnapshot(cmd *service.UsageBillingCommand) bool {
	return cmd != nil && (cmd.SharePolicyID != nil || cmd.SharePolicyVersion > 0 || cmd.OwnerShareRatio > 0 || cmd.InviteShareRatio > 0)
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
	var ownerRaw, inviteRaw string
	var version int
	var err error
	if arg == nil {
		err = tx.QueryRowContext(ctx, query).Scan(&id, &ownerRaw, &inviteRaw, &version)
	} else {
		err = tx.QueryRowContext(ctx, query, arg).Scan(&id, &ownerRaw, &inviteRaw, &version)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return accountSharePolicySnapshot{}, false, nil
	}
	if err != nil {
		return accountSharePolicySnapshot{}, false, err
	}
	ownerRatio, err := decimal.NewFromString(strings.TrimSpace(ownerRaw))
	if err != nil {
		return accountSharePolicySnapshot{}, false, err
	}
	inviteRatio, err := decimal.NewFromString(strings.TrimSpace(inviteRaw))
	if err != nil {
		return accountSharePolicySnapshot{}, false, err
	}
	ownerRatio = clampAccountShareRatio(ownerRatio)
	inviteRatio = clampAccountShareRatio(inviteRatio)
	if ownerRatio.Add(inviteRatio).GreaterThan(decimal.NewFromInt(1)) {
		inviteRatio = decimal.NewFromInt(1).Sub(ownerRatio)
	}
	return accountSharePolicySnapshot{
		ID:               id,
		Version:          version,
		OwnerShareRatio:  ownerRatio,
		InviteShareRatio: inviteRatio,
	}, true, nil
}

func resolveAccountShareInvite(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, policy accountSharePolicySnapshot, usageOccurredAt time.Time) (accountInviteSnapshot, error) {
	if cmd == nil || cmd.BalanceCost <= 0 || !policy.InviteShareRatio.IsPositive() {
		return accountInviteSnapshot{}, nil
	}
	var out accountInviteSnapshot
	err := tx.QueryRowContext(ctx, `
		SELECT ua.inviter_id,
			COALESCE(ua.inviter_bound_at, ua.created_at),
			ua.invite_reward_expires_at
		FROM user_affiliates ua
		JOIN users inviter ON inviter.id = ua.inviter_id
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
	return out, err
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
			invite_share_ratio, invite_credit, platform_share_ratio, platform_fee, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			$12::numeric, $13::numeric, $14::numeric, $15::numeric,
			$16, $17, $18, $19::numeric, $20::numeric, $21::numeric, $22::numeric, 'applied'
		)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`,
		in.UsageLogID, in.RequestID, in.APIKeyID, in.ConsumerUserID, in.OwnerUserID,
		in.AccountID, in.GroupID, in.PolicyID, in.PolicyVersion, in.ShareModeSnapshot, in.ShareStatusSnapshot,
		in.ConsumerCharge.StringFixed(10), in.AccountCost.StringFixed(10), in.OwnerShareRatio.StringFixed(6), in.OwnerCredit.StringFixed(10),
		in.InviterUserID, in.InviteBoundAt, in.InviteExpiresAt, in.InviteShareRatio.StringFixed(6), in.InviteCredit.StringFixed(10),
		in.PlatformShareRatio.StringFixed(6), in.PlatformFee.StringFixed(10),
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func insertAccountShareBalanceLedger(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal, balanceAfter float64, reason string, usageLogID int64, cmd *service.UsageBillingCommand) error {
	metadata, err := json.Marshal(map[string]any{
		"request_id":       cmd.RequestID,
		"api_key_id":       cmd.APIKeyID,
		"account_id":       cmd.AccountID,
		"consumer_user_id": cmd.UserID,
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_balance_ledger (
			user_id, direction, amount, reason, ref_type, ref_id, balance_after, metadata
		) VALUES ($1, 'credit', $2::numeric, $3, 'usage_log', $4, $5::numeric, $6::jsonb)
		ON CONFLICT DO NOTHING
	`, userID, amount.StringFixed(10), reason, nullablePositiveInt64(usageLogID), accountShareDecimal(balanceAfter).StringFixed(10), string(metadata))
	return err
}

func creditInviteShareBalance(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, usageLogID, inviterUserID int64, amount decimal.Decimal) error {
	newBalance, err := creditWalletBucket(ctx, tx, inviterUserID, amount.InexactFloat64(), "invite")
	if err != nil {
		return err
	}
	if err := insertAccountShareBalanceLedger(ctx, tx, inviterUserID, amount, newBalance, "invite_share_income", usageLogID, cmd); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE user_affiliates
		SET aff_history_quota = aff_history_quota + $1::numeric, updated_at = NOW()
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

func appendAccountShareCreditUser(result *service.UsageBillingApplyResult, userID int64) {
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

func accountShareAccountCost(cmd *service.UsageBillingCommand) decimal.Decimal {
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
		return accountShareDecimal(base).Mul(accountShareDecimal(multiplier)).Round(10)
	}
	return accountShareDecimal(cmd.AccountQuotaCost)
}

func resolveAccountShareUsageOccurredAt(cmd *service.UsageBillingCommand) time.Time {
	if cmd != nil {
		if !cmd.UsageOccurredAt.IsZero() {
			return cmd.UsageOccurredAt
		}
		if cmd.UsageLog != nil && !cmd.UsageLog.CreatedAt.IsZero() {
			return cmd.UsageLog.CreatedAt
		}
	}
	return time.Now()
}

func accountShareDecimal(value float64) decimal.Decimal {
	if value <= 0 {
		return decimal.Zero
	}
	return decimal.NewFromFloat(value).Round(10)
}

func clampAccountShareRatio(value decimal.Decimal) decimal.Decimal {
	if value.IsNegative() {
		return decimal.Zero
	}
	one := decimal.NewFromInt(1)
	if value.GreaterThan(one) {
		return one
	}
	return value
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func nullablePositiveInt64Ptr(value *int64) any {
	if value == nil || *value <= 0 {
		return nil
	}
	return *value
}

func nullableAccountShareTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}
