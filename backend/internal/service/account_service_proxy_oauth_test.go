package service

import (
	"context"
	"testing"
	"time"

	"ikik-api/internal/pkg/pagination"

	"github.com/stretchr/testify/require"
)

type ownedOAuthProxyRepoStub struct {
	proxy *Proxy
	err   error
}

func (s *ownedOAuthProxyRepoStub) Create(ctx context.Context, proxy *Proxy) error { return nil }

func (s *ownedOAuthProxyRepoStub) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	return nil, ErrProxyNotFound
}

func (s *ownedOAuthProxyRepoStub) ListByIDs(ctx context.Context, ids []int64) ([]Proxy, error) {
	return nil, nil
}

func (s *ownedOAuthProxyRepoStub) GetOwnedByID(ctx context.Context, ownerUserID, id int64) (*Proxy, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.proxy, nil
}

func (s *ownedOAuthProxyRepoStub) ListOwnedByUserID(ctx context.Context, ownerUserID int64) ([]ProxyWithAccountCount, error) {
	return nil, nil
}

func (s *ownedOAuthProxyRepoStub) CountByOwnerUserID(ctx context.Context, ownerUserID int64) (int64, error) {
	return 0, nil
}

func (s *ownedOAuthProxyRepoStub) CountOwnedAccountsByProxyID(ctx context.Context, ownerUserID, proxyID int64) (int64, error) {
	return 0, nil
}

func (s *ownedOAuthProxyRepoStub) Update(ctx context.Context, proxy *Proxy) error { return nil }

func (s *ownedOAuthProxyRepoStub) Delete(ctx context.Context, id int64) error { return nil }

func (s *ownedOAuthProxyRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *ownedOAuthProxyRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]Proxy, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *ownedOAuthProxyRepoStub) ListWithFiltersAndAccountCount(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *ownedOAuthProxyRepoStub) ListActive(ctx context.Context) ([]Proxy, error) {
	return nil, nil
}

func (s *ownedOAuthProxyRepoStub) ListActiveWithAccountCount(ctx context.Context) ([]ProxyWithAccountCount, error) {
	return nil, nil
}

func (s *ownedOAuthProxyRepoStub) ListAllForFallback(ctx context.Context) ([]Proxy, error) {
	return nil, nil
}

func (s *ownedOAuthProxyRepoStub) SweepExpiredProxies(ctx context.Context, now time.Time) (int64, error) {
	return 0, nil
}

func (s *ownedOAuthProxyRepoStub) CountExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

func (s *ownedOAuthProxyRepoStub) CountExpiringSoon(ctx context.Context, now time.Time) (int64, error) {
	return 0, nil
}

func (s *ownedOAuthProxyRepoStub) ExistsByHostPortAuth(ctx context.Context, host string, port int, username, password string) (bool, error) {
	return false, nil
}

func (s *ownedOAuthProxyRepoStub) CountAccountsByProxyID(ctx context.Context, proxyID int64) (int64, error) {
	return 0, nil
}

func (s *ownedOAuthProxyRepoStub) ListAccountSummariesByProxyID(ctx context.Context, proxyID int64) ([]ProxyAccountSummary, error) {
	return nil, nil
}

func TestValidateOwnedOAuthProxyIDIgnoresMissingProxy(t *testing.T) {
	proxyID := int64(42)
	svc := &AccountService{proxyRepo: &ownedOAuthProxyRepoStub{err: ErrProxyNotFound}}

	got, err := svc.ValidateOwnedOAuthProxyID(context.Background(), 1001, &proxyID)

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestValidateOwnedOAuthProxyIDKeepsActiveProxy(t *testing.T) {
	proxyID := int64(42)
	svc := &AccountService{proxyRepo: &ownedOAuthProxyRepoStub{
		proxy: &Proxy{ID: proxyID, Status: StatusActive},
	}}

	got, err := svc.ValidateOwnedOAuthProxyID(context.Background(), 1001, &proxyID)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, proxyID, *got)
}

func TestValidateOwnedOAuthProxyIDRejectsInactiveProxy(t *testing.T) {
	proxyID := int64(42)
	svc := &AccountService{proxyRepo: &ownedOAuthProxyRepoStub{
		proxy: &Proxy{ID: proxyID, Status: StatusError},
	}}

	got, err := svc.ValidateOwnedOAuthProxyID(context.Background(), 1001, &proxyID)

	require.ErrorIs(t, err, ErrUserPrivateProxyInvalid)
	require.Nil(t, got)
}
