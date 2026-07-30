package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	infraerrors "ikik-api/internal/pkg/errors"
	"ikik-api/internal/pkg/pagination"
	"ikik-api/internal/pkg/response"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

const userAccountDataPageCap = 1000

func (h *UserAccountHandler) ExportData(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	selectedIDs, err := parseUserAccountDataIDs(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	accounts, err := h.resolveOwnedExportAccounts(c, subject.UserID, selectedIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	proxies, err := h.resolveOwnedExportProxies(c.Request.Context(), subject.UserID, accounts)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, service.BuildAccountDataPayload(accounts, proxies, buildUserAccountDataProxyKey))
}

func (h *UserAccountHandler) resolveOwnedExportProxies(ctx context.Context, ownerUserID int64, accounts []service.Account) ([]service.Proxy, error) {
	requested := make(map[int64]struct{})
	for i := range accounts {
		if accounts[i].ProxyID != nil && *accounts[i].ProxyID > 0 {
			requested[*accounts[i].ProxyID] = struct{}{}
		}
	}
	if len(requested) == 0 {
		return []service.Proxy{}, nil
	}

	owned, err := h.accountService.ListOwnedProxies(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	proxies := make([]service.Proxy, 0, len(requested))
	for i := range owned {
		if _, ok := requested[owned[i].ID]; ok {
			proxies = append(proxies, owned[i].Proxy)
		}
	}
	return proxies, nil
}

func (h *UserAccountHandler) resolveOwnedExportAccounts(c *gin.Context, ownerUserID int64, ids []int64) ([]service.Account, error) {
	if len(ids) > 0 {
		out := make([]service.Account, 0, len(ids))
		for _, id := range ids {
			account, err := h.accountService.GetOwnedByID(c.Request.Context(), ownerUserID, id)
			if err != nil {
				return nil, err
			}
			out = append(out, *account)
		}
		return out, nil
	}

	filters, err := parseUserAccountDataFilters(c)
	if err != nil {
		return nil, err
	}
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	page := 1
	out := make([]service.Account, 0)
	for {
		accounts, result, err := h.accountService.ListOwned(c.Request.Context(), ownerUserID, pagination.PaginationParams{
			Page:      page,
			PageSize:  userAccountDataPageCap,
			SortBy:    sortBy,
			SortOrder: sortOrder,
		}, filters)
		if err != nil {
			return nil, err
		}
		out = append(out, accounts...)
		if result == nil || len(out) >= int(result.Total) || len(accounts) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func parseUserAccountDataFilters(c *gin.Context) (service.AccountListFilters, error) {
	filters := service.AccountListFilters{
		Platform:    strings.TrimSpace(c.Query("platform")),
		AccountType: strings.TrimSpace(c.Query("type")),
		Status:      strings.TrimSpace(c.Query("status")),
		Search:      strings.TrimSpace(c.Query("search")),
		PrivacyMode: strings.TrimSpace(c.Query("privacy_mode")),
	}
	if len(filters.Search) > 100 {
		filters.Search = filters.Search[:100]
	}
	if groupIDStr := strings.TrimSpace(c.Query("group_id")); groupIDStr != "" {
		groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			return filters, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter")
		}
		filters.GroupID = groupID
	}
	return filters, nil
}

func parseUserAccountDataIDs(c *gin.Context) ([]int64, error) {
	values := c.QueryArray("ids")
	if len(values) == 0 {
		raw := strings.TrimSpace(c.Query("ids"))
		if raw != "" {
			values = []string{raw}
		}
	}
	if len(values) == 0 {
		return nil, nil
	}

	ids := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, item := range values {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("invalid account id: %s", part)
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func buildUserAccountDataProxyKey(protocol, host string, port int, username, password string) string {
	return service.BuildAccountDataProxyKey(protocol, host, port, username, password)
}
