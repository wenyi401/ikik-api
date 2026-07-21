package repository

import (
	"context"

	dbent "ikik-api/ent"

	"ikik-api/ent/group"
	"ikik-api/ent/predicate"

	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/service"
)

func (r *groupRepository) ListWithFiltersByScope(
	ctx context.Context,
	params pagination.PaginationParams,
	platform, status, search string,
	isExclusive *bool,
	scope string,
) ([]service.Group, *pagination.PaginationResult, error) {
	return r.listWithFiltersByScope(ctx, params, platform, status, search, isExclusive, scope)
}

func (r *groupRepository) ListActiveVisibleToUser(ctx context.Context, userID int64, subscribedGroupIDs []int64) ([]service.Group, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}

	predicates := []predicate.Group{
		group.And(
			group.ScopeEQ(service.GroupScopePublic),
			group.SubscriptionTypeEQ(service.SubscriptionTypeStandard),
		),
		group.And(
			group.ScopeEQ(service.GroupScopeUserPrivate),
			group.OwnerUserIDEQ(userID),
		),
		group.And(
			group.ScopeEQ(service.GroupScopeUserCarpool),
			group.OwnerUserIDEQ(userID),
		),
	}
	if subscribedGroupIDs := uniquePositiveInt64s(subscribedGroupIDs); len(subscribedGroupIDs) > 0 {
		predicates = append(predicates, group.And(
			group.ScopeEQ(service.GroupScopePublic),
			group.IDIn(subscribedGroupIDs...),
		))
	}

	groups, err := r.client.Group.Query().
		Where(
			group.StatusEQ(service.StatusActive),
			group.Or(predicates...),
		).
		Order(dbent.Asc(group.FieldSortOrder), dbent.Asc(group.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	outGroups := make([]service.Group, 0, len(groups))
	for i := range groups {
		g := groupEntityToService(groups[i])
		outGroups = append(outGroups, *g)
	}

	return outGroups, nil
}
