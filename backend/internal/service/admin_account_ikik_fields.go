package service

import (
	"context"
	"errors"
	"fmt"

	infraerrors "ikik-api/internal/pkg/errors"
)

func applyIkikCreateAccountFields(account *Account, input *CreateAccountInput) error {
	if account == nil || input == nil {
		return ErrAccountNilInput
	}
	account.AccountLevel = NormalizeOpenAIAccountLevel(account.Platform, input.AccountLevel, account.Credentials, account.Extra)
	account.OwnerUserID = input.OwnerUserID
	account.ShareMode = NormalizeAccountShareMode(input.ShareMode)
	account.ShareStatus = NormalizeAccountShareStatus(input.ShareStatus)
	account.SharePolicyID = input.SharePolicyID

	concurrency, err := NormalizeOpenAIPlusConcurrency(account.Platform, account.AccountLevel, account.Concurrency)
	if err != nil {
		return err
	}
	account.Concurrency = normalizeAccountConcurrency(account.Platform, account.Type, concurrency)
	return ValidateAccountLoadFactor(account.LoadFactor)
}

func applyIkikUpdateAccountFields(account *Account, input *UpdateAccountInput) {
	if account == nil || input == nil {
		return
	}
	if input.AccountLevel != nil {
		account.AccountLevel = NormalizeAccountLevel(*input.AccountLevel)
	} else {
		account.AccountLevel = NormalizeOpenAIAccountLevel(account.Platform, account.AccountLevel, account.Credentials, account.Extra)
	}
	if input.OwnerUserID != nil {
		account.OwnerUserID = input.OwnerUserID
	}
	if input.ShareMode != "" {
		account.ShareMode = NormalizeAccountShareMode(input.ShareMode)
	}
	if input.ShareStatus != "" {
		account.ShareStatus = NormalizeAccountShareStatus(input.ShareStatus)
	}
	if input.SharePolicyID != nil {
		account.SharePolicyID = input.SharePolicyID
	}
}

func validateIkikUpdatedAccountFields(account *Account) error {
	if account == nil {
		return ErrAccountNotFound
	}
	if err := ValidateOpenAIPlusConcurrency(account.Platform, account.AccountLevel, account.Concurrency); err != nil {
		return err
	}
	return ValidateAccountLoadFactor(account.LoadFactor)
}

func (s *adminServiceImpl) validateIkikCreateAccountGroupBindings(ctx context.Context, input *CreateAccountInput, groupIDs []int64) error {
	if input == nil {
		return ErrAccountNilInput
	}
	level := NormalizeOpenAIAccountLevel(input.Platform, input.AccountLevel, input.Credentials, input.Extra)
	if err := s.validateAccountLevelGroupBinding(ctx, input.Platform, level, groupIDs); err != nil {
		return err
	}
	return s.validateAccountShareGroupBinding(ctx, &Account{
		Platform:    input.Platform,
		OwnerUserID: input.OwnerUserID,
		ShareMode:   NormalizeAccountShareMode(input.ShareMode),
		ShareStatus: NormalizeAccountShareStatus(input.ShareStatus),
	}, groupIDs)
}

func (s *adminServiceImpl) validateIkikUpdateAccountGroupBindings(ctx context.Context, account *Account, input *UpdateAccountInput) error {
	if account == nil || input == nil {
		return ErrAccountNilInput
	}
	groupIDs := account.GroupIDs
	if input.GroupIDs != nil {
		groupIDs = *input.GroupIDs
	}
	if err := s.validateAccountLevelGroupBinding(ctx, account.Platform, account.AccountLevel, groupIDs); err != nil {
		return err
	}
	return s.validateAccountShareGroupBinding(ctx, account, groupIDs)
}

func (s *adminServiceImpl) validateIkikBulkAccountFields(ctx context.Context, input *BulkUpdateAccountsInput, accounts []*Account) error {
	if input == nil {
		return ErrAccountNilInput
	}
	for _, account := range accounts {
		if account == nil {
			continue
		}
		credentials := mergeAccountMap(account.Credentials, input.Credentials)
		extra := mergeAccountMap(account.Extra, input.Extra)
		level := NormalizeOpenAIAccountLevel(account.Platform, account.AccountLevel, credentials, extra)
		if input.AccountLevel != nil {
			level = NormalizeAccountLevel(*input.AccountLevel)
		}
		groupIDs := account.GroupIDs
		if input.GroupIDs != nil {
			groupIDs = *input.GroupIDs
		}
		if err := s.validateAccountLevelGroupBinding(ctx, account.Platform, level, groupIDs); err != nil {
			return err
		}
		if input.GroupIDs != nil {
			candidate := *account
			candidate.AccountLevel = level
			if err := s.validateAccountShareGroupBinding(ctx, &candidate, groupIDs); err != nil {
				return err
			}
		}
		if input.Concurrency != nil {
			if err := ValidateOpenAIPlusConcurrency(account.Platform, level, *input.Concurrency); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *adminServiceImpl) validateAccountLevelGroupBinding(ctx context.Context, accountPlatform, accountLevel string, groupIDs []int64) error {
	if len(groupIDs) == 0 || accountPlatform != PlatformOpenAI {
		return nil
	}
	level := NormalizeAccountLevel(accountLevel)
	for _, groupID := range groupIDs {
		group, err := s.groupRepo.GetByIDLite(ctx, groupID)
		if err != nil {
			return fmt.Errorf("get group: %w", err)
		}
		required := NormalizeRequiredAccountLevel(group.RequiredAccountLevel)
		if group.Platform != PlatformOpenAI || required == "" {
			continue
		}
		if !CanOpenAIAccountJoinSharedPool(level, required) {
			return infraerrors.BadRequest(
				"ACCOUNT_GROUP_BINDING_INVALID",
				fmt.Sprintf("account_level mismatch: OpenAI account level %s cannot bind to group %s requiring %s-compatible account", NormalizeOpenAISharedPoolAccountLevel(level), group.Name, required),
			)
		}
	}
	return nil
}

func (s *adminServiceImpl) validateAccountShareGroupBinding(ctx context.Context, account *Account, groupIDs []int64) error {
	if len(groupIDs) == 0 || account == nil {
		return nil
	}
	if s.groupRepo == nil {
		return errors.New("group repository not configured")
	}
	for _, groupID := range groupIDs {
		group, err := s.groupRepo.GetByIDLite(ctx, groupID)
		if err != nil {
			return fmt.Errorf("get group: %w", err)
		}
		if group == nil || group.ID <= 0 {
			return ErrGroupNotFound
		}

		scope := NormalizeGroupScope(group.Scope)
		if account.OwnerUserID == nil {
			if scope == GroupScopeUserPrivate {
				return infraerrors.BadRequest("ACCOUNT_GROUP_BINDING_INVALID", fmt.Sprintf("platform account cannot bind to user private group %s", group.Name))
			}
			continue
		}
		if scope == GroupScopeUserPrivate {
			if group.OwnerUserID == nil || *group.OwnerUserID != *account.OwnerUserID {
				return infraerrors.BadRequest("ACCOUNT_GROUP_BINDING_INVALID", fmt.Sprintf("owned account cannot bind to another user's private group %s", group.Name))
			}
			continue
		}
		if NormalizeAccountShareMode(account.ShareMode) != AccountShareModePublic ||
			NormalizeAccountShareStatus(account.ShareStatus) != AccountShareStatusApproved {
			return infraerrors.BadRequest("ACCOUNT_GROUP_BINDING_INVALID", fmt.Sprintf("owned account must be approved public share before binding to public group %s", group.Name))
		}
	}
	return nil
}
