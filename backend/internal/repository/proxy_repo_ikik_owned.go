package repository

import (
	"context"

	dbent "ikik-api/ent"
	dbaccount "ikik-api/ent/account"
	dbproxy "ikik-api/ent/proxy"
	"ikik-api/internal/service"
)

func (r *proxyRepository) CountByOwnerUserID(ctx context.Context, ownerUserID int64) (int64, error) {
	count, err := r.client.Proxy.Query().Where(dbproxy.OwnerUserIDEQ(ownerUserID)).Count(ctx)
	return int64(count), err
}

func (r *proxyRepository) CountOwnedAccountsByProxyID(ctx context.Context, ownerUserID, proxyID int64) (int64, error) {
	count, err := r.client.Account.Query().Where(
		dbaccount.OwnerUserIDEQ(ownerUserID),
		dbaccount.ProxyIDEQ(proxyID),
	).Count(ctx)
	return int64(count), err
}

func (r *proxyRepository) GetOwnedByID(ctx context.Context, ownerUserID, id int64) (*service.Proxy, error) {
	m, err := r.client.Proxy.Query().Where(
		dbproxy.IDEQ(id),
		dbproxy.OwnerUserIDEQ(ownerUserID),
	).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrProxyNotFound
		}
		return nil, err
	}
	return proxyEntityToService(m), nil
}

func (r *proxyRepository) ListOwnedByUserID(ctx context.Context, ownerUserID int64) ([]service.ProxyWithAccountCount, error) {
	proxies, err := r.client.Proxy.Query().
		Where(dbproxy.OwnerUserIDEQ(ownerUserID)).
		Order(dbent.Desc(dbproxy.FieldCreatedAt), dbent.Desc(dbproxy.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]service.ProxyWithAccountCount, 0, len(proxies))
	for _, entity := range proxies {
		proxyOut := proxyEntityToService(entity)
		if proxyOut == nil {
			continue
		}
		accountCount, err := r.CountOwnedAccountsByProxyID(ctx, ownerUserID, proxyOut.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, service.ProxyWithAccountCount{
			Proxy:        *proxyOut,
			AccountCount: accountCount,
		})
	}
	return out, nil
}
