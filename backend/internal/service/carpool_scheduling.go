package service

import (
	"context"
	"errors"
	"sort"
	"time"
)

type carpoolSchedulingAccess struct {
	PoolID     int64
	GroupID    int64
	AccountIDs map[int64]struct{}
}

type carpoolActivePoolByUserPlatformFinder interface {
	FindActivePoolByUserAndPlatform(ctx context.Context, userID int64, platform string) (*CarpoolPool, error)
}

func resolveCarpoolSchedulingAccess(ctx context.Context, repo CarpoolRepository, groupID *int64) (*carpoolSchedulingAccess, bool, error) {
	if repo == nil || groupID == nil || *groupID <= 0 {
		return nil, false, nil
	}
	pool, err := repo.GetPoolByGroupID(ctx, *groupID)
	if errors.Is(err, ErrCarpoolPoolNotFound) {
		if !isUserCarpoolRequestGroup(ctx, *groupID) {
			return nil, false, nil
		}
		pool, err = resolveUserCarpoolSchedulingPool(ctx, repo, *groupID)
		if errors.Is(err, ErrCarpoolPoolNotFound) {
			return nil, true, nil
		}
	}
	if err != nil {
		return nil, true, err
	}
	if pool == nil {
		return nil, true, nil
	}

	access := &carpoolSchedulingAccess{
		PoolID:     pool.ID,
		GroupID:    *groupID,
		AccountIDs: map[int64]struct{}{},
	}
	if pool.Status == CarpoolPoolStatusClosed {
		return access, true, nil
	}

	userID := AuthenticatedUserIDFromContext(ctx)
	if userID <= 0 {
		return access, true, nil
	}
	if pool.OwnerUserID != userID {
		member, memberErr := repo.GetMemberByPoolAndUser(ctx, pool.ID, userID)
		if errors.Is(memberErr, ErrCarpoolMemberNotFound) {
			return access, true, nil
		}
		if memberErr != nil {
			return nil, true, memberErr
		}
		if member == nil || member.Status != CarpoolMemberStatusActive {
			return access, true, nil
		}
	}

	accounts, err := repo.ListPoolAccounts(ctx, pool.ID)
	if err != nil {
		return nil, true, err
	}
	for i := range accounts {
		if accounts[i].AccountID > 0 {
			access.AccountIDs[accounts[i].AccountID] = struct{}{}
		}
	}
	return access, true, nil
}

func isUserCarpoolRequestGroup(ctx context.Context, groupID int64) bool {
	group := GroupFromContext(ctx)
	return group != nil && group.ID == groupID && group.IsUserCarpoolScope()
}

func resolveUserCarpoolSchedulingPool(ctx context.Context, repo CarpoolRepository, groupID int64) (*CarpoolPool, error) {
	group := GroupFromContext(ctx)
	if group == nil || group.ID != groupID || !group.IsUserCarpoolScope() {
		return nil, ErrCarpoolPoolNotFound
	}
	userID := AuthenticatedUserIDFromContext(ctx)
	if userID <= 0 || group.OwnerUserID == nil || *group.OwnerUserID != userID {
		return nil, ErrCarpoolPoolNotFound
	}
	finder, ok := repo.(carpoolActivePoolByUserPlatformFinder)
	if !ok {
		return nil, ErrServiceUnavailable
	}
	pool, err := finder.FindActivePoolByUserAndPlatform(ctx, userID, group.Platform)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, ErrCarpoolPoolNotFound
	}
	return pool, nil
}

func filterCarpoolSchedulingAccounts(ctx context.Context, repo CarpoolRepository, groupID *int64, accounts []Account) ([]Account, bool, error) {
	access, isCarpool, err := resolveCarpoolSchedulingAccess(ctx, repo, groupID)
	if err != nil || !isCarpool {
		return accounts, isCarpool, err
	}
	if access == nil || len(access.AccountIDs) == 0 || len(accounts) == 0 {
		return []Account{}, true, nil
	}
	filtered := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if _, ok := access.AccountIDs[account.ID]; ok {
			filtered = append(filtered, account)
		}
	}
	return filtered, true, nil
}

func listCarpoolSchedulingAccounts(ctx context.Context, carpoolRepo CarpoolRepository, accountRepo AccountRepository, groupID *int64, platforms []string) ([]Account, bool, error) {
	access, isCarpool, err := resolveCarpoolSchedulingAccess(ctx, carpoolRepo, groupID)
	if err != nil || !isCarpool {
		return nil, isCarpool, err
	}
	if access == nil || len(access.AccountIDs) == 0 || accountRepo == nil {
		return []Account{}, true, nil
	}

	ids := make([]int64, 0, len(access.AccountIDs))
	for accountID := range access.AccountIDs {
		ids = append(ids, accountID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	accounts, err := accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, true, err
	}
	platformSet := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		if platform != "" {
			platformSet[platform] = struct{}{}
		}
	}
	filtered := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if !isCarpoolAccountSchedulable(account) {
			continue
		}
		if len(platformSet) > 0 {
			if _, ok := platformSet[account.Platform]; !ok {
				continue
			}
		}
		filtered = append(filtered, *account)
	}
	return filtered, true, nil
}

func isCarpoolAccountSchedulable(account *Account) bool {
	return isCarpoolAccountSchedulableAt(account, time.Now())
}

func isCarpoolAccountSchedulableAt(account *Account, now time.Time) bool {
	if account == nil {
		return false
	}
	if !account.IsActive() || !account.Schedulable {
		return false
	}
	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
		return false
	}
	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		return false
	}
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		return false
	}
	if account.IsAPIKeyOrBedrock() && account.IsQuotaExceededAt(now) {
		return false
	}
	return true
}

func isCarpoolSchedulingAccountAllowed(ctx context.Context, repo CarpoolRepository, groupID *int64, account *Account) bool {
	if account == nil || account.ID <= 0 {
		return false
	}
	access, isCarpool, err := resolveCarpoolSchedulingAccess(ctx, repo, groupID)
	if err != nil || !isCarpool || access == nil {
		return false
	}
	_, ok := access.AccountIDs[account.ID]
	return ok
}

func currentRequestGroupID(ctx context.Context) *int64 {
	group := GroupFromContext(ctx)
	if group == nil || group.ID <= 0 {
		return nil
	}
	groupID := group.ID
	return &groupID
}
