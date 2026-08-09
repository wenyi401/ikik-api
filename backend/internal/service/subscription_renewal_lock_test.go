package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type lockingRenewalRepo struct {
	userSubRepoNoop
	mu        sync.Mutex
	stale     UserSubscription
	current   UserSubscription
	lockReads int
}

func (r *lockingRenewalRepo) ExistsByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	return true, nil
}

func (r *lockingRenewalRepo) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := r.stale
	return &copy, nil
}

func (r *lockingRenewalRepo) GetByID(_ context.Context, _ int64) (*UserSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := r.current
	return &copy, nil
}

func (r *lockingRenewalRepo) GetByIDForUpdate(_ context.Context, _ int64) (*UserSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lockReads++
	copy := r.current
	return &copy, nil
}

func (r *lockingRenewalRepo) ExtendExpiry(_ context.Context, _ int64, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current.ExpiresAt = expiresAt
	return nil
}

func (r *lockingRenewalRepo) UpdateStatus(_ context.Context, _ int64, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current.Status = status
	return nil
}

func (r *lockingRenewalRepo) UpdateNotes(_ context.Context, _ int64, notes string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current.Notes = notes
	return nil
}

func (r *lockingRenewalRepo) Update(_ context.Context, sub *UserSubscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current = *sub
	return nil
}

func TestAssignOrExtendSubscriptionUsesLockedCurrentRow(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	lockedExpiry := now.AddDate(0, 0, 20)
	repo := &lockingRenewalRepo{
		stale: UserSubscription{ID: 7, UserID: 11, GroupID: 13, ExpiresAt: now.Add(-time.Hour), Status: SubscriptionStatusExpired},
		current: UserSubscription{
			ID: 7, UserID: 11, GroupID: 13, ExpiresAt: lockedExpiry, Status: SubscriptionStatusSuspended, Notes: "current",
		},
	}
	svc := NewSubscriptionService(&subscriptionGroupRepoStub{group: &Group{ID: 13, SubscriptionType: SubscriptionTypeSubscription}}, repo, nil, nil, nil)
	svc.now = func() time.Time { return now }

	sub, extended, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{
		UserID: 11, GroupID: 13, ValidityDays: 5, Notes: "renewed",
	})

	require.NoError(t, err)
	require.True(t, extended)
	require.Equal(t, 1, repo.lockReads)
	require.Equal(t, lockedExpiry.AddDate(0, 0, 5), sub.ExpiresAt)
	require.Equal(t, SubscriptionStatusActive, sub.Status)
	require.Equal(t, "current\nrenewed", sub.Notes)
}

func TestAssignSubscriptionDoesNotReviveNewlySuspendedRow(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	current := UserSubscription{
		ID: 27, UserID: 31, GroupID: 33, ExpiresAt: now.Add(-time.Hour), Status: SubscriptionStatusSuspended, Notes: "suspended",
	}
	repo := &lockingRenewalRepo{
		stale:   UserSubscription{ID: 27, UserID: 31, GroupID: 33, ExpiresAt: now.Add(-time.Hour), Status: SubscriptionStatusExpired},
		current: current,
	}
	svc := NewSubscriptionService(&subscriptionGroupRepoStub{group: &Group{ID: 33, SubscriptionType: SubscriptionTypeSubscription}}, repo, nil, nil, nil)
	svc.now = func() time.Time { return now }

	sub, reused, err := svc.assignSubscriptionWithReuse(context.Background(), &AssignSubscriptionInput{
		UserID: 31, GroupID: 33, ValidityDays: 5, Notes: "renewed",
	})

	require.NoError(t, err)
	require.True(t, reused)
	require.Equal(t, 1, repo.lockReads)
	require.Equal(t, current.Status, sub.Status)
	require.Equal(t, current.ExpiresAt, sub.ExpiresAt)
	require.Equal(t, current.Notes, sub.Notes)
}
