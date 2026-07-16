package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type routeSubscriptionRepoStub struct {
	userSubRepoNoop
	sub         *UserSubscription
	err         error
	called      bool
	calledUser  int64
	calledGroup int64
}

func (r *routeSubscriptionRepoStub) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	r.called = true
	r.calledUser = userID
	r.calledGroup = groupID
	if r.err != nil {
		return nil, r.err
	}
	return r.sub, nil
}

func TestResolveRouteSubscriptionLoadsCurrentRouteGroup(t *testing.T) {
	t.Parallel()

	currentGroupID := int64(20)
	currentSub := &UserSubscription{
		ID:        200,
		UserID:    10,
		GroupID:   currentGroupID,
		Status:    SubscriptionStatusActive,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	repo := &routeSubscriptionRepoStub{sub: currentSub}
	apiKey := &APIKey{
		User:    &User{ID: 10},
		GroupID: &currentGroupID,
		Group: &Group{
			ID:               currentGroupID,
			SubscriptionType: SubscriptionTypeSubscription,
		},
	}
	baseSub := &UserSubscription{
		ID:        100,
		UserID:    10,
		GroupID:   10,
		Status:    SubscriptionStatusActive,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	got, err := resolveRouteSubscription(context.Background(), repo, apiKey, baseSub)
	if err != nil {
		t.Fatalf("resolveRouteSubscription error = %v", err)
	}
	if got == nil || got.ID != currentSub.ID {
		t.Fatalf("subscription ID = %v, want %d", got, currentSub.ID)
	}
	if !repo.called || repo.calledUser != 10 || repo.calledGroup != currentGroupID {
		t.Fatalf("repo call = (%v, %d, %d), want (true, 10, %d)", repo.called, repo.calledUser, repo.calledGroup, currentGroupID)
	}
}

func TestResolveRouteSubscriptionRequiresSubscriptionForSubscriptionGroup(t *testing.T) {
	t.Parallel()

	groupID := int64(20)
	repo := &routeSubscriptionRepoStub{err: ErrSubscriptionNotFound}
	apiKey := &APIKey{
		User:    &User{ID: 10},
		GroupID: &groupID,
		Group: &Group{
			ID:               groupID,
			SubscriptionType: SubscriptionTypeSubscription,
		},
	}

	got, err := resolveRouteSubscription(context.Background(), repo, apiKey, nil)
	if got != nil {
		t.Fatalf("subscription = %#v, want nil", got)
	}
	if !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("error = %v, want ErrSubscriptionNotFound", err)
	}
}

func TestBuildUsageBillingCommandSubscriptionWithoutSubscriptionDoesNotChargeBalance(t *testing.T) {
	t.Parallel()

	groupID := int64(20)
	cmd := buildUsageBillingCommand("req-1", nil, &postUsageBillingParams{
		Cost:               &CostBreakdown{TotalCost: 1, ActualCost: 1},
		User:               &User{ID: 10},
		APIKey:             &APIKey{ID: 30, GroupID: &groupID},
		Account:            &Account{ID: 40},
		IsSubscriptionBill: true,
	})
	if cmd == nil {
		t.Fatal("buildUsageBillingCommand returned nil")
	}
	if cmd.BalanceCost != 0 {
		t.Fatalf("BalanceCost = %v, want 0", cmd.BalanceCost)
	}
	if cmd.SubscriptionCost != 0 {
		t.Fatalf("SubscriptionCost = %v, want 0", cmd.SubscriptionCost)
	}
}
