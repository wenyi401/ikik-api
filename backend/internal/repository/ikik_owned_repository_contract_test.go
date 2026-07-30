package repository

import (
	"context"
	"testing"

	dbent "ikik-api/ent"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/service"
)

type ownedAccountRepositoryContract interface {
	ListOwnedWithFilters(context.Context, int64, pagination.PaginationParams, string, string, string, string, int64, int64, string) ([]service.Account, *pagination.PaginationResult, error)
	ListQuotaPoolAccounts(context.Context, int64) ([]service.Account, error)
}

type privateProxyRepositoryContract interface {
	Create(context.Context, *service.Proxy) error
	GetOwnedByID(context.Context, int64, int64) (*service.Proxy, error)
	ListOwnedByUserID(context.Context, int64) ([]service.ProxyWithAccountCount, error)
	CountByOwnerUserID(context.Context, int64) (int64, error)
	CountOwnedAccountsByProxyID(context.Context, int64, int64) (int64, error)
	Update(context.Context, *service.Proxy) error
	Delete(context.Context, int64) error
}

var _ ownedAccountRepositoryContract = (*accountRepository)(nil)
var _ privateProxyRepositoryContract = (*proxyRepository)(nil)

func TestAccountEntityToServicePreservesOwnershipFields(t *testing.T) {
	ownerUserID := int64(41)
	sharePolicyID := int64(52)
	got := accountEntityToService(&dbent.Account{
		AccountLevel:  service.AccountLevelTeam,
		OwnerUserID:   &ownerUserID,
		ShareMode:     service.AccountShareModePublic,
		ShareStatus:   service.AccountShareStatusApproved,
		SharePolicyID: &sharePolicyID,
	})
	if got == nil {
		t.Fatal("accountEntityToService returned nil")
	}
	if got.AccountLevel != service.AccountLevelTeam || got.OwnerUserID == nil || *got.OwnerUserID != ownerUserID {
		t.Fatalf("ownership fields were not preserved: %#v", got)
	}
	if got.ShareMode != service.AccountShareModePublic || got.ShareStatus != service.AccountShareStatusApproved {
		t.Fatalf("share fields were not preserved: %#v", got)
	}
	if got.SharePolicyID == nil || *got.SharePolicyID != sharePolicyID {
		t.Fatalf("share policy was not preserved: %#v", got.SharePolicyID)
	}
}

func TestProxyEntityToServicePreservesOwner(t *testing.T) {
	ownerUserID := int64(63)
	got := proxyEntityToService(&dbent.Proxy{OwnerUserID: &ownerUserID})
	if got == nil || got.OwnerUserID == nil || *got.OwnerUserID != ownerUserID {
		t.Fatalf("proxy owner was not preserved: %#v", got)
	}
}

func TestGroupEntityToServicePreservesOwnedPoolMetadata(t *testing.T) {
	ownerUserID := int64(42)
	got := groupEntityToService(&dbent.Group{
		OwnerUserID:                 &ownerUserID,
		Scope:                       service.GroupScopeUserPrivate,
		Platform:                    service.PlatformOpenAI,
		RequiredAccountLevel:        service.AccountLevelPlus,
		IsSharedPool:                true,
		KiroCacheEmulationEnabled:   true,
		KiroAutoStickyEnabled:       true,
		KiroStickySessionTTLSeconds: 1800,
		KiroCacheEmulationRatio:     0.75,
		KiroEndpointMode:            "auto",
	})
	if got == nil {
		t.Fatal("groupEntityToService returned nil")
	}
	if got.OwnerUserID == nil || *got.OwnerUserID != ownerUserID {
		t.Fatalf("owner user id was not preserved: %#v", got.OwnerUserID)
	}
	if got.Scope != service.GroupScopeUserPrivate {
		t.Fatalf("scope was not preserved: %q", got.Scope)
	}
	if got.RequiredAccountLevel != service.AccountLevelPlus {
		t.Fatalf("required account level was not preserved: %q", got.RequiredAccountLevel)
	}
	if !got.IsSharedPool {
		t.Fatal("shared pool marker was not preserved")
	}
	if !got.KiroCacheEmulationEnabled || !got.KiroAutoStickyEnabled || got.KiroStickySessionTTLSeconds != 1800 || got.KiroCacheEmulationRatio != 0.75 || got.KiroEndpointMode != "auto" {
		t.Fatalf("kiro group settings were not preserved: %#v", got)
	}
}
