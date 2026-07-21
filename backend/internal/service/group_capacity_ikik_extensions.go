package service

import (
	"context"
	"fmt"
)

type groupCapacityVisibleGroupRepository interface {
	ListActiveVisibleToUser(ctx context.Context, userID int64, subscribedGroupIDs []int64) ([]Group, error)
}

// GetUserVisibleGroupCapacity applies the product visibility rules while
// retaining the upstream batch-capacity implementation.
func (s *GroupCapacityService) GetUserVisibleGroupCapacity(ctx context.Context, userID int64) ([]GroupCapacitySummary, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	visibleGroupRepo, ok := s.groupRepo.(groupCapacityVisibleGroupRepository)
	if !ok {
		return nil, fmt.Errorf("visible group repository is unavailable")
	}
	groups, err := visibleGroupRepo.ListActiveVisibleToUser(ctx, userID, nil)
	if err != nil {
		return nil, err
	}
	groups = filterPublicBalanceGroups(groups)
	groupIDs := make([]int64, 0, len(groups))
	groupByID := make(map[int64]Group, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
		groupByID[group.ID] = group
	}

	var summaries []GroupCapacitySummary
	if lister, ok := s.accountRepo.(groupCapacityAccountLister); ok {
		summaries, err = s.getGroupCapacitiesBatch(ctx, groupIDs, lister)
	} else {
		summaries = s.getGroupCapacitiesSequential(ctx, groupIDs)
	}
	if err != nil {
		return nil, err
	}
	for i := range summaries {
		group := groupByID[summaries[i].GroupID]
		summaries[i].GroupName = group.Name
		summaries[i].GroupPlatform = group.Platform
	}
	return summaries, nil
}

func filterPublicBalanceGroups(groups []Group) []Group {
	out := make([]Group, 0, len(groups))
	for _, group := range groups {
		if group.IsExclusive || group.OwnerUserID != nil || NormalizeGroupScope(group.Scope) != GroupScopePublic {
			continue
		}
		if group.SubscriptionType != "" && group.SubscriptionType != SubscriptionTypeStandard {
			continue
		}
		out = append(out, group)
	}
	return out
}
