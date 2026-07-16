package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ikik-api/internal/pkg/pagination"
)

const (
	AccountSharePolicyScopeGlobal   = "global"
	AccountSharePolicyScopePlatform = "platform"
	AccountSharePolicyScopeGroup    = "group"
	AccountSharePolicyScopeAccount  = "account"
)

type AccountSharePolicy struct {
	ID               int64      `json:"id"`
	ScopeType        string     `json:"scope_type"`
	ScopeID          *int64     `json:"scope_id,omitempty"`
	Platform         *string    `json:"platform,omitempty"`
	OwnerShareRatio  float64    `json:"owner_share_ratio"`
	InviteShareRatio float64    `json:"invite_share_ratio"`
	Version          int        `json:"version"`
	Enabled          bool       `json:"enabled"`
	EffectiveAt      time.Time  `json:"effective_at"`
	CreatedByAdminID *int64     `json:"created_by_admin_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

type AccountSharePolicyFilters struct {
	ScopeType string
	Platform  string
	Enabled   *bool
}

type CreateAccountSharePolicyInput struct {
	ScopeType        string
	ScopeID          *int64
	Platform         *string
	OwnerShareRatio  float64
	InviteShareRatio float64
	Enabled          *bool
	EffectiveAt      *time.Time
	CreatedByAdminID *int64
}

type UpdateAccountSharePolicyInput struct {
	ScopeType        *string
	ScopeID          *int64
	Platform         *string
	OwnerShareRatio  *float64
	InviteShareRatio *float64
	Enabled          *bool
	EffectiveAt      *time.Time
}

type AccountSharePolicyRepository interface {
	ListAccountSharePolicies(ctx context.Context, params pagination.PaginationParams, filters AccountSharePolicyFilters) ([]AccountSharePolicy, *pagination.PaginationResult, error)
	GetAccountSharePolicyByID(ctx context.Context, id int64) (*AccountSharePolicy, error)
	ResolveEnabledAccountSharePolicy(ctx context.Context, accountID int64, groupID *int64, platform string, explicitPolicyID *int64) (*AccountSharePolicy, error)
	CreateAccountSharePolicy(ctx context.Context, input CreateAccountSharePolicyInput) (*AccountSharePolicy, error)
	UpdateAccountSharePolicy(ctx context.Context, id int64, input UpdateAccountSharePolicyInput) (*AccountSharePolicy, error)
	DeleteAccountSharePolicy(ctx context.Context, id int64) error
}

type AccountSharePolicyService struct {
	repo AccountSharePolicyRepository
}

func NewAccountSharePolicyService(repo AccountSharePolicyRepository) *AccountSharePolicyService {
	return &AccountSharePolicyService{repo: repo}
}

func (s *AccountSharePolicyService) List(ctx context.Context, params pagination.PaginationParams, filters AccountSharePolicyFilters) ([]AccountSharePolicy, *pagination.PaginationResult, error) {
	return s.repo.ListAccountSharePolicies(ctx, params, filters)
}

func (s *AccountSharePolicyService) GetByID(ctx context.Context, id int64) (*AccountSharePolicy, error) {
	if id <= 0 {
		return nil, ErrAccountNotFound
	}
	return s.repo.GetAccountSharePolicyByID(ctx, id)
}

func (s *AccountSharePolicyService) Create(ctx context.Context, input CreateAccountSharePolicyInput) (*AccountSharePolicy, error) {
	normalized, err := normalizeCreateAccountSharePolicyInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateAccountSharePolicy(ctx, normalized)
}

func (s *AccountSharePolicyService) Update(ctx context.Context, id int64, input UpdateAccountSharePolicyInput) (*AccountSharePolicy, error) {
	if id <= 0 {
		return nil, ErrAccountNotFound
	}
	existing, err := s.repo.GetAccountSharePolicyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	merged := CreateAccountSharePolicyInput{
		ScopeType:        existing.ScopeType,
		ScopeID:          existing.ScopeID,
		Platform:         existing.Platform,
		OwnerShareRatio:  existing.OwnerShareRatio,
		InviteShareRatio: existing.InviteShareRatio,
		Enabled:          &existing.Enabled,
		EffectiveAt:      &existing.EffectiveAt,
	}
	if input.ScopeType != nil {
		merged.ScopeType = *input.ScopeType
	}
	if input.ScopeID != nil {
		if *input.ScopeID <= 0 {
			merged.ScopeID = nil
		} else {
			merged.ScopeID = input.ScopeID
		}
	}
	if input.Platform != nil {
		platform := strings.TrimSpace(*input.Platform)
		if platform == "" {
			merged.Platform = nil
		} else {
			merged.Platform = &platform
		}
	}
	if input.OwnerShareRatio != nil {
		merged.OwnerShareRatio = *input.OwnerShareRatio
	}
	if input.InviteShareRatio != nil {
		merged.InviteShareRatio = *input.InviteShareRatio
	}
	if input.Enabled != nil {
		merged.Enabled = input.Enabled
	}
	if input.EffectiveAt != nil {
		merged.EffectiveAt = input.EffectiveAt
	}
	normalized, err := normalizeCreateAccountSharePolicyInput(merged)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateAccountSharePolicy(ctx, id, UpdateAccountSharePolicyInput{
		ScopeType:        &normalized.ScopeType,
		ScopeID:          normalized.ScopeID,
		Platform:         normalized.Platform,
		OwnerShareRatio:  &normalized.OwnerShareRatio,
		InviteShareRatio: &normalized.InviteShareRatio,
		Enabled:          normalized.Enabled,
		EffectiveAt:      normalized.EffectiveAt,
	})
}

func (s *AccountSharePolicyService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrAccountNotFound
	}
	return s.repo.DeleteAccountSharePolicy(ctx, id)
}

func normalizeCreateAccountSharePolicyInput(input CreateAccountSharePolicyInput) (CreateAccountSharePolicyInput, error) {
	input.ScopeType = normalizeAccountSharePolicyScope(input.ScopeType)
	if input.OwnerShareRatio < 0 || input.OwnerShareRatio > 1 {
		return input, fmt.Errorf("owner_share_ratio must be between 0 and 1")
	}
	if input.InviteShareRatio < 0 || input.InviteShareRatio > 1 {
		return input, fmt.Errorf("invite_share_ratio must be between 0 and 1")
	}
	if input.OwnerShareRatio+input.InviteShareRatio > 1 {
		return input, fmt.Errorf("owner_share_ratio plus invite_share_ratio must be less than or equal to 1")
	}
	if input.Enabled == nil {
		enabled := true
		input.Enabled = &enabled
	}
	if input.EffectiveAt == nil {
		now := time.Now()
		input.EffectiveAt = &now
	}
	if input.Platform != nil {
		platform := strings.TrimSpace(*input.Platform)
		if platform == "" {
			input.Platform = nil
		} else {
			input.Platform = &platform
		}
	}
	if input.ScopeType != AccountSharePolicyScopeGlobal {
		return input, fmt.Errorf("only global account share policy is supported")
	}
	input.ScopeID = nil
	input.Platform = nil
	return input, nil
}

func normalizeAccountSharePolicyScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "", AccountSharePolicyScopeGlobal:
		return AccountSharePolicyScopeGlobal
	default:
		return strings.ToLower(strings.TrimSpace(scope))
	}
}
