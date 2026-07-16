package service

import (
	"context"

	infraerrors "ikik-api/internal/pkg/errors"
)

var ErrSubscriptionRepositoryUnavailable = infraerrors.InternalServer(
	"SUBSCRIPTION_REPOSITORY_UNAVAILABLE",
	"subscription repository is not configured",
)

func resolveRouteSubscription(ctx context.Context, repo UserSubscriptionRepository, apiKey *APIKey, baseSubscription *UserSubscription) (*UserSubscription, error) {
	if apiKey == nil || apiKey.Group == nil || !apiKey.Group.IsSubscriptionType() {
		return nil, nil
	}
	if apiKey.User == nil || apiKey.User.ID <= 0 || apiKey.GroupID == nil || *apiKey.GroupID <= 0 {
		return nil, ErrSubscriptionNotFound
	}
	groupID := *apiKey.GroupID
	userID := apiKey.User.ID
	if baseSubscription != nil && baseSubscription.UserID == userID && baseSubscription.GroupID == groupID && baseSubscription.IsActive() {
		return baseSubscription, nil
	}
	if repo == nil {
		return nil, ErrSubscriptionRepositoryUnavailable
	}
	return repo.GetActiveByUserIDAndGroupID(ctx, userID, groupID)
}

func (s *GatewayService) ResolveRouteSubscription(ctx context.Context, apiKey *APIKey, baseSubscription *UserSubscription) (*UserSubscription, error) {
	if s == nil {
		return nil, ErrSubscriptionRepositoryUnavailable
	}
	return resolveRouteSubscription(ctx, s.userSubRepo, apiKey, baseSubscription)
}

func (s *OpenAIGatewayService) ResolveRouteSubscription(ctx context.Context, apiKey *APIKey, baseSubscription *UserSubscription) (*UserSubscription, error) {
	if s == nil {
		return nil, ErrSubscriptionRepositoryUnavailable
	}
	return resolveRouteSubscription(ctx, s.userSubRepo, apiKey, baseSubscription)
}
