package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/pkg/ctxkey"
)

func TestIsAccountVisibleToRequestUserAllowsOwnerPublicShareInPrivateGroup(t *testing.T) {
	ownerID := int64(42)
	account := &Account{
		ID:          10,
		OwnerUserID: &ownerID,
		ShareMode:   AccountShareModePublic,
		ShareStatus: AccountShareStatusPending,
	}
	ctx := context.WithValue(context.Background(), ctxkey.AuthenticatedUserID, ownerID)
	ctx = context.WithValue(ctx, ctxkey.Group, &Group{
		ID:          99,
		Scope:       GroupScopeUserPrivate,
		OwnerUserID: &ownerID,
	})

	require.True(t, IsAccountVisibleToRequestUser(ctx, account))

	account.ShareMode = AccountShareModePrivate
	require.True(t, IsAccountVisibleToRequestUser(ctx, account))
}

func TestIsAccountVisibleToRequestUserRejectsPublicShareInAnotherUsersPrivateGroup(t *testing.T) {
	ownerID := int64(42)
	consumerID := int64(100)
	account := &Account{
		ID:          10,
		OwnerUserID: &ownerID,
		ShareMode:   AccountShareModePublic,
		ShareStatus: AccountShareStatusApproved,
	}
	ctx := context.WithValue(context.Background(), ctxkey.AuthenticatedUserID, consumerID)
	ctx = context.WithValue(ctx, ctxkey.Group, &Group{
		ID:          99,
		Scope:       GroupScopeUserPrivate,
		OwnerUserID: &consumerID,
	})

	require.False(t, IsAccountVisibleToRequestUser(ctx, account))
}

func TestIsAccountVisibleToRequestUserRejectsPublicShareInCarpoolGroup(t *testing.T) {
	ownerID := int64(42)
	account := &Account{
		ID:          10,
		OwnerUserID: &ownerID,
		ShareMode:   AccountShareModePublic,
		ShareStatus: AccountShareStatusApproved,
	}
	ctx := context.WithValue(context.Background(), ctxkey.AuthenticatedUserID, ownerID)
	ctx = context.WithValue(ctx, ctxkey.Group, &Group{
		ID:          99,
		Scope:       GroupScopeUserCarpool,
		OwnerUserID: &ownerID,
	})

	require.False(t, IsAccountVisibleToRequestUser(ctx, account))
}

func TestIsAccountVisibleToRequestUserKeepsApprovedPublicShareVisibleInPublicGroup(t *testing.T) {
	ownerID := int64(42)
	consumerID := int64(100)
	account := &Account{
		ID:          10,
		OwnerUserID: &ownerID,
		ShareMode:   AccountShareModePublic,
		ShareStatus: AccountShareStatusApproved,
	}
	ctx := context.WithValue(context.Background(), ctxkey.AuthenticatedUserID, consumerID)
	ctx = context.WithValue(ctx, ctxkey.Group, &Group{
		ID:    6,
		Scope: GroupScopePublic,
	})

	require.True(t, IsAccountVisibleToRequestUser(ctx, account))
}
