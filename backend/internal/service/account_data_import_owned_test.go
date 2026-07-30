package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type ownedDataImportProxyRepo struct {
	*ownedOAuthProxyRepoStub
	proxies map[int64]*Proxy
	nextID  int64
}

func newOwnedDataImportProxyRepo(proxies ...*Proxy) *ownedDataImportProxyRepo {
	repo := &ownedDataImportProxyRepo{
		ownedOAuthProxyRepoStub: &ownedOAuthProxyRepoStub{},
		proxies:                 make(map[int64]*Proxy, len(proxies)),
		nextID:                  100,
	}
	for _, proxy := range proxies {
		cp := *proxy
		repo.proxies[cp.ID] = &cp
		if cp.ID >= repo.nextID {
			repo.nextID = cp.ID + 1
		}
	}
	return repo
}

func (r *ownedDataImportProxyRepo) Create(_ context.Context, proxy *Proxy) error {
	cp := *proxy
	cp.ID = r.nextID
	r.nextID++
	proxy.ID = cp.ID
	r.proxies[cp.ID] = &cp
	return nil
}

func (r *ownedDataImportProxyRepo) GetOwnedByID(_ context.Context, ownerUserID, id int64) (*Proxy, error) {
	proxy := r.proxies[id]
	if proxy == nil || proxy.OwnerUserID == nil || *proxy.OwnerUserID != ownerUserID {
		return nil, ErrProxyNotFound
	}
	cp := *proxy
	return &cp, nil
}

func (r *ownedDataImportProxyRepo) ListOwnedByUserID(_ context.Context, ownerUserID int64) ([]ProxyWithAccountCount, error) {
	out := make([]ProxyWithAccountCount, 0, len(r.proxies))
	for _, proxy := range r.proxies {
		if proxy.OwnerUserID == nil || *proxy.OwnerUserID != ownerUserID {
			continue
		}
		out = append(out, ProxyWithAccountCount{Proxy: *proxy})
	}
	return out, nil
}

func (r *ownedDataImportProxyRepo) CountByOwnerUserID(_ context.Context, ownerUserID int64) (int64, error) {
	var count int64
	for _, proxy := range r.proxies {
		if proxy.OwnerUserID != nil && *proxy.OwnerUserID == ownerUserID {
			count++
		}
	}
	return count, nil
}

func (r *ownedDataImportProxyRepo) Update(_ context.Context, proxy *Proxy) error {
	cp := *proxy
	r.proxies[cp.ID] = &cp
	return nil
}

func TestImportOwnedDataForcesOwnerPrivateStateAndUsesOnlyOwnedProxy(t *testing.T) {
	ownerID := int64(101)
	foreignOwnerID := int64(202)
	foreignProxy := &Proxy{
		ID:          7,
		Name:        "foreign",
		Protocol:    "http",
		Host:        "127.0.0.1",
		Port:        8080,
		Username:    "user",
		Password:    "pass",
		Status:      StatusActive,
		OwnerUserID: &foreignOwnerID,
	}
	proxyRepo := newOwnedDataImportProxyRepo(foreignProxy)
	accountRepo := &ownedAccountDuplicateRepoStub{}
	svc := NewAccountService(accountRepo, nil, nil, nil)
	svc.SetProxyRepository(proxyRepo)
	svc.SetUserPrivateGroupProvisioner(&ownedPrivateGroupProvisionerStub{
		group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
	})

	proxyKey := "exported-proxy"
	maliciousPolicyID := int64(88)
	maliciousRate := 9.5
	defaultsConcurrency := 4
	defaultsPriority := 2
	result, err := svc.ImportOwnedData(context.Background(), ownerID, AccountDataPayload{
		Proxies: []AccountDataProxy{{
			ProxyKey: proxyKey,
			Name:     "mine-after-import",
			Protocol: foreignProxy.Protocol,
			Host:     foreignProxy.Host,
			Port:     foreignProxy.Port,
			Username: foreignProxy.Username,
			Password: foreignProxy.Password,
			Status:   StatusDisabled,
		}},
		Accounts: []AccountDataAccount{{
			Name:           "owned-import",
			Platform:       PlatformOpenAI,
			Type:           AccountTypeOAuth,
			Credentials:    map[string]any{"access_token": "oauth-token"},
			ProxyKey:       &proxyKey,
			Concurrency:    20,
			Priority:       20,
			RateMultiplier: &maliciousRate,
			OwnerUserID:    &foreignOwnerID,
			ShareMode:      AccountShareModePublic,
			ShareStatus:    AccountShareStatusSuspended,
			SharePolicyID:  &maliciousPolicyID,
		}},
	}, OwnedAccountDataImportOptions{
		AccountDefaults: &OwnedAccountDataImportDefaults{
			Concurrency: &defaultsConcurrency,
			Priority:    &defaultsPriority,
		},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.ProxyCreated)
	require.Zero(t, result.ProxyReused)
	require.Equal(t, 1, result.AccountCreated)
	require.Empty(t, result.Errors)
	require.Len(t, proxyRepo.proxies, 2, "the foreign proxy must not be reused")
	createdProxy := proxyRepo.proxies[100]
	require.NotNil(t, createdProxy)
	require.Equal(t, &ownerID, createdProxy.OwnerUserID)
	require.Equal(t, "inactive", createdProxy.Status, "user-managed inactive status must survive round-trip")

	require.Len(t, accountRepo.createdAccounts, 1)
	createdAccount := accountRepo.createdAccounts[0]
	require.Equal(t, &ownerID, createdAccount.OwnerUserID)
	require.Equal(t, AccountShareModePrivate, createdAccount.ShareMode)
	require.Equal(t, AccountShareStatusApproved, createdAccount.ShareStatus)
	require.Nil(t, createdAccount.SharePolicyID)
	require.Nil(t, createdAccount.RateMultiplier)
	require.Equal(t, &createdProxy.ID, createdAccount.ProxyID)
	require.False(t, createdAccount.Schedulable)
	require.False(t, createdAccount.IsSchedulable())
	require.Equal(t, defaultsConcurrency, createdAccount.Concurrency)
	require.Equal(t, defaultsPriority, createdAccount.Priority)
	require.Equal(t, []int64{99}, accountRepo.boundGroupIDs[createdAccount.ID])
}

func TestImportOwnedDataDoesNotChangeReusedPrivateProxyStatus(t *testing.T) {
	ownerID := int64(101)
	existing := &Proxy{
		ID:          7,
		Name:        "existing",
		Protocol:    "http",
		Host:        "127.0.0.1",
		Port:        8080,
		Status:      StatusActive,
		OwnerUserID: &ownerID,
	}
	proxyRepo := newOwnedDataImportProxyRepo(existing)
	svc := NewAccountService(&ownedAccountDuplicateRepoStub{}, nil, nil, nil)
	svc.SetProxyRepository(proxyRepo)

	result, err := svc.ImportOwnedData(context.Background(), ownerID, AccountDataPayload{
		Proxies: []AccountDataProxy{{
			Name:     "payload-name",
			Protocol: existing.Protocol,
			Host:     existing.Host,
			Port:     existing.Port,
			Status:   "inactive",
		}},
		Accounts: []AccountDataAccount{},
	}, OwnedAccountDataImportOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, result.ProxyReused)
	require.Zero(t, result.ProxyCreated)
	require.Equal(t, StatusActive, proxyRepo.proxies[existing.ID].Status)
}

func TestImportOwnedDataIgnoresAdminOnlyExpiredProxyStatus(t *testing.T) {
	ownerID := int64(101)
	proxyRepo := newOwnedDataImportProxyRepo()
	svc := NewAccountService(&ownedAccountDuplicateRepoStub{}, nil, nil, nil)
	svc.SetProxyRepository(proxyRepo)

	result, err := svc.ImportOwnedData(context.Background(), ownerID, AccountDataPayload{
		Proxies: []AccountDataProxy{{
			Name: "expired-in-export", Protocol: "http", Host: "127.0.0.1", Port: 8080, Status: "expired",
		}},
		Accounts: []AccountDataAccount{},
	}, OwnedAccountDataImportOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, result.ProxyCreated)
	require.Equal(t, StatusActive, proxyRepo.proxies[100].Status)
}

func TestAccountDataPayloadRoundTripReusesOwnedProxy(t *testing.T) {
	ownerID := int64(101)
	proxyID := int64(7)
	proxy := &Proxy{
		ID:          proxyID,
		Name:        "round-trip",
		Protocol:    "socks5",
		Host:        "127.0.0.1",
		Port:        1080,
		Username:    "user",
		Password:    "pass",
		Status:      StatusActive,
		OwnerUserID: &ownerID,
	}
	payload := BuildAccountDataPayload([]Account{{
		ID:          33,
		Name:        "round-trip-account",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "round-trip-token"},
		OwnerUserID: &ownerID,
		ShareMode:   AccountShareModePrivate,
		ShareStatus: AccountShareStatusApproved,
		ProxyID:     &proxyID,
		Concurrency: 3,
		Priority:    1,
		Schedulable: true,
		Status:      StatusActive,
	}}, []Proxy{*proxy}, BuildAccountDataProxyKey)

	proxyRepo := newOwnedDataImportProxyRepo(proxy)
	accountRepo := &ownedAccountDuplicateRepoStub{}
	svc := NewAccountService(accountRepo, nil, nil, nil)
	svc.SetProxyRepository(proxyRepo)
	svc.SetUserPrivateGroupProvisioner(&ownedPrivateGroupProvisionerStub{
		group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
	})

	result, err := svc.ImportOwnedData(context.Background(), ownerID, payload, OwnedAccountDataImportOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, result.ProxyReused)
	require.Zero(t, result.ProxyCreated)
	require.Equal(t, 1, result.AccountCreated)
	require.Len(t, proxyRepo.proxies, 1)
	require.Len(t, accountRepo.createdAccounts, 1)
	require.Equal(t, &proxyID, accountRepo.createdAccounts[0].ProxyID)
	require.Equal(t, &ownerID, accountRepo.createdAccounts[0].OwnerUserID)
	require.True(t, accountRepo.createdAccounts[0].Schedulable)
}

func TestAccountDataPayloadRoundTripReusesInactiveOwnedProxy(t *testing.T) {
	ownerID := int64(101)
	proxyID := int64(7)
	proxy := &Proxy{
		ID:          proxyID,
		Name:        "inactive-round-trip",
		Protocol:    "socks5",
		Host:        "127.0.0.1",
		Port:        1080,
		Username:    "user",
		Password:    "pass",
		Status:      "inactive",
		OwnerUserID: &ownerID,
	}
	payload := BuildAccountDataPayload([]Account{{
		ID:          33,
		Name:        "inactive-round-trip-account",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "inactive-round-trip-token"},
		OwnerUserID: &ownerID,
		ShareMode:   AccountShareModePrivate,
		ShareStatus: AccountShareStatusApproved,
		ProxyID:     &proxyID,
		Concurrency: 3,
		Priority:    1,
		Schedulable: true,
		Status:      StatusActive,
	}}, []Proxy{*proxy}, BuildAccountDataProxyKey)

	proxyRepo := newOwnedDataImportProxyRepo(proxy)
	accountRepo := &ownedAccountDuplicateRepoStub{}
	svc := NewAccountService(accountRepo, nil, nil, nil)
	svc.SetProxyRepository(proxyRepo)
	svc.SetUserPrivateGroupProvisioner(&ownedPrivateGroupProvisionerStub{
		group: &Group{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Scope: GroupScopeUserPrivate},
	})

	_, err := svc.CreateOwned(context.Background(), ownerID, CreateAccountRequest{
		Name:        "normal-create-must-reject-inactive-proxy",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "different-token"},
		ProxyID:     &proxyID,
	})
	require.ErrorIs(t, err, ErrUserPrivateProxyInvalid)
	require.Empty(t, accountRepo.createdAccounts)

	result, err := svc.ImportOwnedData(context.Background(), ownerID, payload, OwnedAccountDataImportOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, result.ProxyReused)
	require.Zero(t, result.ProxyCreated)
	require.Equal(t, 1, result.AccountCreated)
	require.Empty(t, result.Errors)
	require.Len(t, proxyRepo.proxies, 1)
	require.Equal(t, "inactive", proxyRepo.proxies[proxyID].Status)
	require.Len(t, accountRepo.createdAccounts, 1)
	require.Equal(t, &proxyID, accountRepo.createdAccounts[0].ProxyID)
	require.Equal(t, &ownerID, accountRepo.createdAccounts[0].OwnerUserID)
	require.False(t, accountRepo.createdAccounts[0].Schedulable)
	require.False(t, accountRepo.createdAccounts[0].IsSchedulable())
}

func TestImportOwnedDataRejectsTargetGroupsBeforeMutation(t *testing.T) {
	ownerID := int64(101)
	proxyRepo := newOwnedDataImportProxyRepo()
	accountRepo := &ownedAccountDuplicateRepoStub{}
	svc := NewAccountService(accountRepo, nil, nil, nil)
	svc.SetProxyRepository(proxyRepo)

	result, err := svc.ImportOwnedData(context.Background(), ownerID, AccountDataPayload{
		Proxies: []AccountDataProxy{{
			Name: "must-not-create", Protocol: "http", Host: "127.0.0.1", Port: 8001,
		}},
		Accounts: []AccountDataAccount{},
	}, OwnedAccountDataImportOptions{GroupIDs: []int64{999}})

	require.ErrorIs(t, err, ErrOwnedAccountDataImportInvalid)
	require.Zero(t, result.ProxyCreated)
	require.Empty(t, proxyRepo.proxies)
	require.Empty(t, accountRepo.createdAccounts)
}

func TestImportOwnedDataEnforcesPrivateProxyLimit(t *testing.T) {
	ownerID := int64(101)
	proxies := make([]*Proxy, 0, UserPrivateProxyLimit)
	for i := 0; i < UserPrivateProxyLimit; i++ {
		proxies = append(proxies, &Proxy{
			ID:          int64(i + 1),
			Name:        "existing",
			Protocol:    "http",
			Host:        "127.0.0.1",
			Port:        9000 + i,
			Status:      StatusActive,
			OwnerUserID: &ownerID,
		})
	}
	proxyRepo := newOwnedDataImportProxyRepo(proxies...)
	svc := NewAccountService(&ownedAccountDuplicateRepoStub{}, nil, nil, nil)
	svc.SetProxyRepository(proxyRepo)

	proxyKey := "over-limit"
	result, err := svc.ImportOwnedData(context.Background(), ownerID, AccountDataPayload{
		Proxies: []AccountDataProxy{{
			ProxyKey: proxyKey,
			Name:     "fourth",
			Protocol: "http",
			Host:     "127.0.0.1",
			Port:     9999,
		}},
		Accounts: []AccountDataAccount{{
			Name:        "cannot-bind-failed-proxy",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Credentials: map[string]any{"access_token": "oauth-token"},
			ProxyKey:    &proxyKey,
		}},
	}, OwnedAccountDataImportOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, result.ProxyFailed)
	require.Equal(t, 1, result.AccountFailed)
	require.Zero(t, result.ProxyCreated)
	require.Len(t, proxyRepo.proxies, UserPrivateProxyLimit)
}

func TestImportOwnedDataRejectsProxyAliasCollisionBeforeCreatingSecondProxy(t *testing.T) {
	ownerID := int64(101)
	proxyRepo := newOwnedDataImportProxyRepo()
	svc := NewAccountService(&ownedAccountDuplicateRepoStub{}, nil, nil, nil)
	svc.SetProxyRepository(proxyRepo)

	result, err := svc.ImportOwnedData(context.Background(), ownerID, AccountDataPayload{
		Proxies: []AccountDataProxy{
			{ProxyKey: "same-alias", Name: "first", Protocol: "http", Host: "127.0.0.1", Port: 8001},
			{ProxyKey: "same-alias", Name: "second", Protocol: "http", Host: "127.0.0.1", Port: 8002},
		},
		Accounts: []AccountDataAccount{},
	}, OwnedAccountDataImportOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, result.ProxyCreated)
	require.Equal(t, 1, result.ProxyFailed)
	require.Len(t, proxyRepo.proxies, 1)
}
