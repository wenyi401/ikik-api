package service

import (
	"context"

	infraerrors "ikik-api/internal/pkg/errors"
)

// AdminBindInviter sets or replaces a user's inviter. resetValidity=true
// starts a fresh invite validity window from the admin binding time.
func (s *AffiliateService) AdminBindInviter(ctx context.Context, userID, inviterID int64, resetValidity bool) (*AffiliateSummary, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if userID <= 0 || inviterID <= 0 || userID == inviterID {
		return nil, ErrAffiliateCodeInvalid
	}
	return s.repo.AdminBindInviter(ctx, userID, inviterID, resetValidity)
}

func (s *AffiliateService) AdminExtendInviteRewards(ctx context.Context, req AffiliateInviteRewardExtensionRequest) (*AffiliateInviteRewardExtensionResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if req.ExtendDays <= 0 || req.ExtendDays > AffiliateRebateDurationDaysMax {
		return nil, infraerrors.BadRequest("INVALID_EXTEND_DAYS", "extend days must be between 1 and 3650")
	}

	switch req.Scope {
	case AffiliateInviteRewardExtensionScopeSite:
		req.InviterUserID = 0
		req.AllInvitees = true
		req.InviteeUserIDs = nil
	case AffiliateInviteRewardExtensionScopeInviter:
		if req.InviterUserID <= 0 {
			return nil, infraerrors.BadRequest("INVALID_INVITER", "invalid inviter")
		}
		if !req.AllInvitees {
			req.InviteeUserIDs = normalizePositiveInt64s(req.InviteeUserIDs)
			if len(req.InviteeUserIDs) == 0 {
				return nil, infraerrors.BadRequest("INVALID_INVITEES", "invitee_user_ids cannot be empty")
			}
			for _, inviteeID := range req.InviteeUserIDs {
				if inviteeID == req.InviterUserID {
					return nil, infraerrors.BadRequest("INVALID_INVITEES", "inviter cannot be included as invitee")
				}
			}
		} else {
			req.InviteeUserIDs = nil
		}
	default:
		return nil, infraerrors.BadRequest("INVALID_SCOPE", "invalid extension scope")
	}

	return s.repo.AdminExtendInviteRewards(ctx, req)
}

type AffiliateInviteRewardExtensionRequest struct {
	Scope          string  `json:"scope"`
	InviterUserID  int64   `json:"inviter_user_id,omitempty"`
	AllInvitees    bool    `json:"all_invitees,omitempty"`
	InviteeUserIDs []int64 `json:"invitee_user_ids,omitempty"`
	ExtendDays     int     `json:"extend_days"`
}

type AffiliateInviteRewardExtensionResult struct {
	Affected int64 `json:"affected"`
}

const (
	AffiliateInviteRewardExtensionScopeSite    = "site"
	AffiliateInviteRewardExtensionScopeInviter = "inviter"
)

func normalizePositiveInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
