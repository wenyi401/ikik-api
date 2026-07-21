package service

import (
	"context"

	"fmt"

	"strings"
)

func (s *GatewayService) SetCarpoolRepository(repo CarpoolRepository) {
	if s != nil {
		s.carpoolRepo = repo
	}
}

func attachAccountShareBillingSnapshot(ctx context.Context, cmd *UsageBillingCommand, p *postUsageBillingParams, deps *billingDeps) error {
	if cmd == nil || p == nil || p.Account == nil || p.User == nil {
		return nil
	}
	account := p.Account
	shareMode := NormalizeAccountShareMode(account.ShareMode)
	shareStatus := NormalizeAccountShareStatus(account.ShareStatus)

	cmd.ShareSnapshotCaptured = true
	cmd.ShareModeSnapshot = shareMode
	cmd.ShareStatusSnapshot = shareStatus
	cmd.SharePlatform = strings.TrimSpace(account.Platform)
	if account.OwnerUserID != nil && *account.OwnerUserID > 0 {
		ownerUserID := *account.OwnerUserID
		cmd.ShareOwnerUserID = &ownerUserID
	}
	if account.SharePolicyID != nil && *account.SharePolicyID > 0 {
		sharePolicyID := *account.SharePolicyID
		cmd.SharePolicyID = &sharePolicyID
	}

	if account.OwnerUserID == nil || *account.OwnerUserID <= 0 || *account.OwnerUserID == p.User.ID {
		return nil
	}
	if shareMode != AccountShareModePublic || shareStatus != AccountShareStatusApproved {
		return nil
	}

	if deps == nil || deps.accountSharePolicyRepo == nil {
		return nil
	}

	var groupID *int64
	if p.APIKey != nil {
		groupID = p.APIKey.GroupID
	}
	policy, err := deps.accountSharePolicyRepo.ResolveEnabledAccountSharePolicy(ctx, account.ID, groupID, account.Platform, account.SharePolicyID)
	if err != nil {
		return fmt.Errorf("resolve account share policy snapshot: %w", err)
	}
	if policy == nil || (policy.OwnerShareRatio <= 0 && policy.InviteShareRatio <= 0) {
		return nil
	}
	sharePolicyID := policy.ID
	cmd.SharePolicyID = &sharePolicyID
	cmd.SharePolicyVersion = policy.Version
	cmd.OwnerShareRatio = policy.OwnerShareRatio
	cmd.InviteShareRatio = policy.InviteShareRatio
	return nil
}
