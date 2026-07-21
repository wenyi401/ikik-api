package repository

import (
	"context"

	dbaccount "ikik-api/ent/account"
	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/service"
)

var ownedAccountIdentityUniqueIndexSet = map[string]struct{}{
	"idx_accounts_owned_openai_chatgpt_account_id_uniq": {},
	"idx_accounts_owned_openai_chatgpt_user_id_uniq":    {},
	"idx_accounts_owned_anthropic_org_account_uniq":     {},
	"idx_accounts_owned_gemini_project_uniq":            {},
	"idx_accounts_owned_antigravity_project_uniq":       {},
}

func translateAccountPersistenceError(err error, notFound *infraerrors.ApplicationError) error {
	if err == nil {
		return nil
	}
	if isUniqueViolationOnIndex(err, ownedAccountIdentityUniqueIndexSet) {
		return service.ErrOwnedAccountAlreadyExists.WithCause(err)
	}
	return translatePersistenceError(err, notFound, nil)
}

func (r *accountRepository) ListOwnedWithFilters(
	ctx context.Context,
	ownerUserID int64,
	params pagination.PaginationParams,
	platform, accountType, status, search string,
	groupID, proxyID int64,
	privacyMode string,
) ([]service.Account, *pagination.PaginationResult, error) {
	if ownerUserID <= 0 {
		return nil, nil, service.ErrUserNotFound
	}

	q := r.accountListFilteredQuery(platform, accountType, status, search, groupID, privacyMode).
		Where(dbaccount.OwnerUserIDEQ(ownerUserID))
	if proxyID == service.AccountListProxyUnassigned {
		q = q.Where(dbaccount.ProxyIDIsNil())
	} else if proxyID > 0 {
		q = q.Where(dbaccount.ProxyIDEQ(proxyID))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	accountsQuery := q.Offset(params.Offset()).Limit(params.Limit())
	for _, order := range accountListOrder(params) {
		accountsQuery = accountsQuery.Order(order)
	}
	accounts, err := accountsQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out, err := r.accountsToService(ctx, accounts)
	if err != nil {
		return nil, nil, err
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}
