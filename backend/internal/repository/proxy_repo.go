package repository

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	dbent "ikik-api/ent"
	dbpredicate "ikik-api/ent/predicate"
	"ikik-api/ent/proxy"
	"ikik-api/internal/pkg/logger"
	"ikik-api/internal/service"

	"ikik-api/internal/pkg/pagination"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/lib/pq"
)

type proxyRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewProxyRepository(client *dbent.Client, sqlDB *sql.DB) service.ProxyRepository {
	return newProxyRepositoryWithSQL(client, sqlDB)
}

func newProxyRepositoryWithSQL(client *dbent.Client, sqlq sqlExecutor) *proxyRepository {
	return &proxyRepository{client: client, sql: sqlq}
}

func (r *proxyRepository) Create(ctx context.Context, proxyIn *service.Proxy) error {
	var username any
	if proxyIn.Username != "" {
		username = proxyIn.Username
	}
	var password any
	if proxyIn.Password != "" {
		password = proxyIn.Password
	}
	var owner any
	if proxyIn.OwnerUserID != nil && *proxyIn.OwnerUserID > 0 {
		owner = *proxyIn.OwnerUserID
	}
	var expiresAt any
	if proxyIn.ExpiresAt != nil {
		expiresAt = *proxyIn.ExpiresAt
	}
	var backupProxyID any
	if proxyIn.BackupProxyID != nil {
		backupProxyID = *proxyIn.BackupProxyID
	}
	fallbackMode := normalizeProxyFallbackModeForStorage(proxyIn.FallbackMode)
	expiryWarnDays := normalizeProxyExpiryWarnDays(proxyIn.ExpiryWarnDays)

	var createdAt, updatedAt sql.NullTime
	err := scanSingleRow(ctx, r.sql, `
		INSERT INTO proxies (
			name, protocol, host, port, username, password, status, owner_user_id,
			expires_at, fallback_mode, backup_proxy_id, expiry_warn_days, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, []any{
		proxyIn.Name,
		proxyIn.Protocol,
		proxyIn.Host,
		proxyIn.Port,
		username,
		password,
		proxyIn.Status,
		owner,
		expiresAt,
		fallbackMode,
		backupProxyID,
		expiryWarnDays,
	}, &proxyIn.ID, &createdAt, &updatedAt)
	if err != nil {
		return err
	}
	if createdAt.Valid {
		proxyIn.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		proxyIn.UpdatedAt = updatedAt.Time
	}
	proxyIn.FallbackMode = fallbackMode
	proxyIn.ExpiryWarnDays = expiryWarnDays
	return nil
}

func (r *proxyRepository) GetByID(ctx context.Context, id int64) (*service.Proxy, error) {
	m, err := r.client.Proxy.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrProxyNotFound
		}
		return nil, err
	}
	out := proxyEntityToService(m)
	if err := r.hydrateProxyExtendedFields(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *proxyRepository) ListByIDs(ctx context.Context, ids []int64) ([]service.Proxy, error) {
	if len(ids) == 0 {
		return []service.Proxy{}, nil
	}

	proxies, err := r.client.Proxy.Query().
		Where(proxy.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]service.Proxy, 0, len(proxies))
	for i := range proxies {
		out = append(out, *proxyEntityToService(proxies[i]))
	}
	if err := r.hydrateProxySliceExtendedFields(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *proxyRepository) Update(ctx context.Context, proxyIn *service.Proxy) error {
	var username any
	if proxyIn.Username != "" {
		username = proxyIn.Username
	}
	var password any
	if proxyIn.Password != "" {
		password = proxyIn.Password
	}
	var expiresAt any
	if proxyIn.ExpiresAt != nil {
		expiresAt = *proxyIn.ExpiresAt
	}
	var backupProxyID any
	if proxyIn.BackupProxyID != nil {
		backupProxyID = *proxyIn.BackupProxyID
	}
	fallbackMode := normalizeProxyFallbackModeForStorage(proxyIn.FallbackMode)
	expiryWarnDays := normalizeProxyExpiryWarnDays(proxyIn.ExpiryWarnDays)

	var createdAt, updatedAt sql.NullTime
	err := scanSingleRow(ctx, r.sql, `
		UPDATE proxies
		SET name = $2,
			protocol = $3,
			host = $4,
			port = $5,
			username = $6,
			password = $7,
			status = $8,
			expires_at = $9,
			fallback_mode = $10,
			backup_proxy_id = $11,
			expiry_warn_days = $12,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING created_at, updated_at
	`, []any{
		proxyIn.ID,
		proxyIn.Name,
		proxyIn.Protocol,
		proxyIn.Host,
		proxyIn.Port,
		username,
		password,
		proxyIn.Status,
		expiresAt,
		fallbackMode,
		backupProxyID,
		expiryWarnDays,
	}, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrProxyNotFound
	}
	if err != nil {
		return err
	}
	if createdAt.Valid {
		proxyIn.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		proxyIn.UpdatedAt = updatedAt.Time
	}
	proxyIn.FallbackMode = fallbackMode
	proxyIn.ExpiryWarnDays = expiryWarnDays
	return nil
}

func (r *proxyRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.client.Proxy.Delete().Where(proxy.IDEQ(id)).Exec(ctx)
	return err
}

func (r *proxyRepository) List(ctx context.Context, params pagination.PaginationParams) ([]service.Proxy, *pagination.PaginationResult, error) {
	return r.ListWithFilters(ctx, params, "", "", "")
}

// ListWithFilters lists proxies with optional filtering by protocol, status, and search query
func (r *proxyRepository) ListWithFilters(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]service.Proxy, *pagination.PaginationResult, error) {
	q := r.client.Proxy.Query().Where(globalProxyPredicate())
	if protocol != "" {
		q = q.Where(proxy.ProtocolEQ(protocol))
	}
	if status != "" {
		q = q.Where(proxy.StatusEQ(status))
	}
	if search != "" {
		q = q.Where(proxy.NameContainsFold(search))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	proxiesQuery := q.
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range proxyListOrder(params) {
		proxiesQuery = proxiesQuery.Order(order)
	}

	proxies, err := proxiesQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	outProxies := make([]service.Proxy, 0, len(proxies))
	for i := range proxies {
		outProxies = append(outProxies, *proxyEntityToService(proxies[i]))
	}
	if err := r.hydrateProxySliceExtendedFields(ctx, outProxies); err != nil {
		return nil, nil, err
	}

	return outProxies, paginationResultFromTotal(int64(total), params), nil
}

// ListWithFiltersAndAccountCount lists proxies with filters and includes account count per proxy
func (r *proxyRepository) ListWithFiltersAndAccountCount(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]service.ProxyWithAccountCount, *pagination.PaginationResult, error) {
	q := r.client.Proxy.Query().Where(globalProxyPredicate())
	if protocol != "" {
		q = q.Where(proxy.ProtocolEQ(protocol))
	}
	if status != "" {
		q = q.Where(proxy.StatusEQ(status))
	}
	if search != "" {
		q = q.Where(proxy.NameContainsFold(search))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	if strings.EqualFold(strings.TrimSpace(params.SortBy), "account_count") {
		return r.listWithAccountCountSort(ctx, q, params, total)
	}

	proxiesQuery := q.
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range proxyListOrder(params) {
		proxiesQuery = proxiesQuery.Order(order)
	}

	proxies, err := proxiesQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return r.buildProxyWithAccountCountResult(ctx, proxies, params, int64(total))
}

func (r *proxyRepository) listWithAccountCountSort(ctx context.Context, q *dbent.ProxyQuery, params pagination.PaginationParams, total int) ([]service.ProxyWithAccountCount, *pagination.PaginationResult, error) {
	proxies, err := q.
		Order(dbent.Desc(proxy.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	result, _, err := r.buildProxyWithAccountCountResult(ctx, proxies, params, int64(total))
	if err != nil {
		return nil, nil, err
	}

	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].AccountCount == result[j].AccountCount {
			return result[i].ID > result[j].ID
		}
		if sortOrder == pagination.SortOrderAsc {
			return result[i].AccountCount < result[j].AccountCount
		}
		return result[i].AccountCount > result[j].AccountCount
	})

	return paginateSlice(result, params), paginationResultFromTotal(int64(total), params), nil
}

func (r *proxyRepository) buildProxyWithAccountCountResult(ctx context.Context, proxies []*dbent.Proxy, params pagination.PaginationParams, total int64) ([]service.ProxyWithAccountCount, *pagination.PaginationResult, error) {
	counts, err := r.GetAccountCountsForProxies(ctx)
	if err != nil {
		return nil, nil, err
	}

	result := make([]service.ProxyWithAccountCount, 0, len(proxies))
	for i := range proxies {
		proxyOut := proxyEntityToService(proxies[i])
		if proxyOut == nil {
			continue
		}
		result = append(result, service.ProxyWithAccountCount{
			Proxy:        *proxyOut,
			AccountCount: counts[proxyOut.ID],
		})
	}
	if err := r.hydrateProxyWithAccountCountExtendedFields(ctx, result); err != nil {
		return nil, nil, err
	}

	return result, paginationResultFromTotal(total, params), nil
}

func proxyListOrder(params pagination.PaginationParams) []func(*entsql.Selector) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	var field string
	switch sortBy {
	case "name":
		field = proxy.FieldName
	case "protocol":
		field = proxy.FieldProtocol
	case "status":
		field = proxy.FieldStatus
	case "created_at":
		field = proxy.FieldCreatedAt
	default:
		field = proxy.FieldID
	}

	if sortOrder == pagination.SortOrderAsc {
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(proxy.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(proxy.FieldID)}
}

func globalProxyPredicate() dbpredicate.Proxy {
	return dbpredicate.Proxy(func(s *entsql.Selector) {
		s.Where(entsql.IsNull(s.C("owner_user_id")))
	})
}

func (r *proxyRepository) ListActive(ctx context.Context) ([]service.Proxy, error) {
	proxies, err := r.client.Proxy.Query().
		Where(proxy.StatusEQ(service.StatusActive), globalProxyPredicate()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	outProxies := make([]service.Proxy, 0, len(proxies))
	for i := range proxies {
		outProxies = append(outProxies, *proxyEntityToService(proxies[i]))
	}
	if err := r.hydrateProxySliceExtendedFields(ctx, outProxies); err != nil {
		return nil, err
	}
	return outProxies, nil
}

// ExistsByHostPortAuth checks if a proxy with the same host, port, username, and password exists
func (r *proxyRepository) ExistsByHostPortAuth(ctx context.Context, host string, port int, username, password string) (bool, error) {
	q := r.client.Proxy.Query().
		Where(proxy.HostEQ(host), proxy.PortEQ(port), globalProxyPredicate())

	if username == "" {
		q = q.Where(proxy.Or(proxy.UsernameIsNil(), proxy.UsernameEQ("")))
	} else {
		q = q.Where(proxy.UsernameEQ(username))
	}
	if password == "" {
		q = q.Where(proxy.Or(proxy.PasswordIsNil(), proxy.PasswordEQ("")))
	} else {
		q = q.Where(proxy.PasswordEQ(password))
	}

	count, err := q.Count(ctx)
	return count > 0, err
}

// CountAccountsByProxyID returns the number of accounts using a specific proxy
func (r *proxyRepository) CountAccountsByProxyID(ctx context.Context, proxyID int64) (int64, error) {
	var count int64
	if err := scanSingleRow(ctx, r.sql, "SELECT COUNT(*) FROM accounts WHERE proxy_id = $1 AND deleted_at IS NULL", []any{proxyID}, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *proxyRepository) CountByOwnerUserID(ctx context.Context, ownerUserID int64) (int64, error) {
	var count int64
	if err := scanSingleRow(ctx, r.sql, `
		SELECT COUNT(*)
		FROM proxies
		WHERE owner_user_id = $1 AND deleted_at IS NULL
	`, []any{ownerUserID}, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *proxyRepository) CountOwnedAccountsByProxyID(ctx context.Context, ownerUserID, proxyID int64) (int64, error) {
	var count int64
	if err := scanSingleRow(ctx, r.sql, `
		SELECT COUNT(*)
		FROM accounts
		WHERE owner_user_id = $1 AND proxy_id = $2 AND deleted_at IS NULL
	`, []any{ownerUserID, proxyID}, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *proxyRepository) GetOwnedByID(ctx context.Context, ownerUserID, id int64) (*service.Proxy, error) {
	out := &service.Proxy{}
	var username, password sql.NullString
	var owner sql.NullInt64
	err := scanSingleRow(ctx, r.sql, `
		SELECT id, name, protocol, host, port, username, password, status, owner_user_id, created_at, updated_at
		FROM proxies
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, []any{id, ownerUserID},
		&out.ID,
		&out.Name,
		&out.Protocol,
		&out.Host,
		&out.Port,
		&username,
		&password,
		&out.Status,
		&owner,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrProxyNotFound
		}
		return nil, err
	}
	if username.Valid {
		out.Username = username.String
	}
	if password.Valid {
		out.Password = password.String
	}
	if owner.Valid {
		out.OwnerUserID = &owner.Int64
	}
	return out, nil
}

func (r *proxyRepository) ListOwnedByUserID(ctx context.Context, ownerUserID int64) ([]service.ProxyWithAccountCount, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			p.id,
			p.name,
			p.protocol,
			p.host,
			p.port,
			p.username,
			p.password,
			p.status,
			p.owner_user_id,
			p.created_at,
			p.updated_at,
			COUNT(a.id) AS account_count
		FROM proxies p
		LEFT JOIN accounts a
			ON a.proxy_id = p.id
			AND a.owner_user_id = $1
			AND a.deleted_at IS NULL
		WHERE p.owner_user_id = $1
			AND p.deleted_at IS NULL
		GROUP BY p.id, p.name, p.protocol, p.host, p.port, p.username, p.password, p.status, p.owner_user_id, p.created_at, p.updated_at
		ORDER BY p.created_at DESC, p.id DESC
	`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.ProxyWithAccountCount, 0)
	for rows.Next() {
		var item service.ProxyWithAccountCount
		var username, password sql.NullString
		var owner sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Protocol,
			&item.Host,
			&item.Port,
			&username,
			&password,
			&item.Status,
			&owner,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.AccountCount,
		); err != nil {
			return nil, err
		}
		if username.Valid {
			item.Username = username.String
		}
		if password.Valid {
			item.Password = password.String
		}
		if owner.Valid {
			item.OwnerUserID = &owner.Int64
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *proxyRepository) ListAccountSummariesByProxyID(ctx context.Context, proxyID int64) ([]service.ProxyAccountSummary, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, name, platform, type, notes
		FROM accounts
		WHERE proxy_id = $1 AND deleted_at IS NULL
		ORDER BY id DESC
	`, proxyID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.ProxyAccountSummary, 0)
	for rows.Next() {
		var (
			id       int64
			name     string
			platform string
			accType  string
			notes    sql.NullString
		)
		if err := rows.Scan(&id, &name, &platform, &accType, &notes); err != nil {
			return nil, err
		}
		var notesPtr *string
		if notes.Valid {
			notesPtr = &notes.String
		}
		out = append(out, service.ProxyAccountSummary{
			ID:       id,
			Name:     name,
			Platform: platform,
			Type:     accType,
			Notes:    notesPtr,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAccountCountsForProxies returns a map of proxy ID to account count for all proxies
func (r *proxyRepository) GetAccountCountsForProxies(ctx context.Context) (counts map[int64]int64, err error) {
	rows, err := r.sql.QueryContext(ctx, "SELECT proxy_id, COUNT(*) AS count FROM accounts WHERE proxy_id IS NOT NULL AND deleted_at IS NULL GROUP BY proxy_id")
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			counts = nil
		}
	}()

	counts = make(map[int64]int64)
	for rows.Next() {
		var proxyID, count int64
		if err = rows.Scan(&proxyID, &count); err != nil {
			return nil, err
		}
		counts[proxyID] = count
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

// ListActiveWithAccountCount returns all active proxies with account count, sorted by creation time descending
func (r *proxyRepository) ListActiveWithAccountCount(ctx context.Context) ([]service.ProxyWithAccountCount, error) {
	proxies, err := r.client.Proxy.Query().
		Where(proxy.StatusEQ(service.StatusActive), globalProxyPredicate()).
		Order(dbent.Desc(proxy.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// Get account counts
	counts, err := r.GetAccountCountsForProxies(ctx)
	if err != nil {
		return nil, err
	}

	// Build result with account counts
	result := make([]service.ProxyWithAccountCount, 0, len(proxies))
	for i := range proxies {
		proxyOut := proxyEntityToService(proxies[i])
		if proxyOut == nil {
			continue
		}
		result = append(result, service.ProxyWithAccountCount{
			Proxy:        *proxyOut,
			AccountCount: counts[proxyOut.ID],
		})
	}
	if err := r.hydrateProxyWithAccountCountExtendedFields(ctx, result); err != nil {
		return nil, err
	}

	return result, nil
}

func proxyEntityToService(m *dbent.Proxy) *service.Proxy {
	if m == nil {
		return nil
	}
	out := &service.Proxy{
		ID:             m.ID,
		Name:           m.Name,
		Protocol:       m.Protocol,
		Host:           m.Host,
		Port:           m.Port,
		Status:         m.Status,
		OwnerUserID:    m.OwnerUserID,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
		FallbackMode:   service.FallbackModeNone,
		ExpiryWarnDays: service.ProxyDefaultExpiryWarnDays,
	}
	if m.Username != nil {
		out.Username = *m.Username
	}
	if m.Password != nil {
		out.Password = *m.Password
	}
	return out
}

func applyProxyEntityToService(dst *service.Proxy, src *dbent.Proxy) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func normalizeProxyFallbackModeForStorage(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case service.FallbackModeProxy:
		return service.FallbackModeProxy
	case service.FallbackModeDirect:
		return service.FallbackModeDirect
	default:
		return service.FallbackModeNone
	}
}

func normalizeProxyExpiryWarnDays(days int) int {
	if days <= 0 {
		return service.ProxyDefaultExpiryWarnDays
	}
	return days
}

func (r *proxyRepository) hydrateProxySliceExtendedFields(ctx context.Context, proxies []service.Proxy) error {
	if len(proxies) == 0 {
		return nil
	}
	ptrs := make([]*service.Proxy, 0, len(proxies))
	for i := range proxies {
		ptrs = append(ptrs, &proxies[i])
	}
	return r.hydrateProxyExtendedFields(ctx, ptrs...)
}

func (r *proxyRepository) hydrateProxyWithAccountCountExtendedFields(ctx context.Context, proxies []service.ProxyWithAccountCount) error {
	if len(proxies) == 0 {
		return nil
	}
	ptrs := make([]*service.Proxy, 0, len(proxies))
	for i := range proxies {
		ptrs = append(ptrs, &proxies[i].Proxy)
	}
	return r.hydrateProxyExtendedFields(ctx, ptrs...)
}

func (r *proxyRepository) hydrateProxyExtendedFields(ctx context.Context, proxies ...*service.Proxy) error {
	ids := make([]int64, 0, len(proxies))
	byID := make(map[int64]*service.Proxy, len(proxies))
	for _, p := range proxies {
		if p == nil || p.ID <= 0 {
			continue
		}
		p.FallbackMode = normalizeProxyFallbackModeForStorage(p.FallbackMode)
		p.ExpiryWarnDays = normalizeProxyExpiryWarnDays(p.ExpiryWarnDays)
		ids = append(ids, p.ID)
		byID[p.ID] = p
	}
	if len(ids) == 0 {
		return nil
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, expires_at, fallback_mode, backup_proxy_id, expiry_warn_days, owner_user_id
		FROM proxies
		WHERE id = ANY($1)
	`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			id             int64
			expiresAt      sql.NullTime
			fallbackMode   sql.NullString
			backupProxyID  sql.NullInt64
			expiryWarnDays sql.NullInt64
			ownerUserID    sql.NullInt64
		)
		if err := rows.Scan(&id, &expiresAt, &fallbackMode, &backupProxyID, &expiryWarnDays, &ownerUserID); err != nil {
			return err
		}
		p := byID[id]
		if p == nil {
			continue
		}
		if expiresAt.Valid {
			p.ExpiresAt = &expiresAt.Time
		} else {
			p.ExpiresAt = nil
		}
		if fallbackMode.Valid {
			p.FallbackMode = normalizeProxyFallbackModeForStorage(fallbackMode.String)
		}
		if backupProxyID.Valid {
			p.BackupProxyID = &backupProxyID.Int64
		} else {
			p.BackupProxyID = nil
		}
		if expiryWarnDays.Valid && expiryWarnDays.Int64 > 0 {
			p.ExpiryWarnDays = int(expiryWarnDays.Int64)
		}
		if ownerUserID.Valid {
			p.OwnerUserID = &ownerUserID.Int64
		} else {
			p.OwnerUserID = nil
		}
	}
	return rows.Err()
}

func (r *proxyRepository) ListAllForFallback(ctx context.Context) ([]service.Proxy, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, name, protocol, host, port, username, password, status, owner_user_id,
		       created_at, updated_at, expires_at, fallback_mode, backup_proxy_id, expiry_warn_days
		FROM proxies
		WHERE deleted_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.Proxy, 0)
	for rows.Next() {
		p, err := scanProxyRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scanProxyRow(rows interface {
	Scan(dest ...any) error
}) (*service.Proxy, error) {
	var (
		p              service.Proxy
		username       sql.NullString
		password       sql.NullString
		ownerUserID    sql.NullInt64
		expiresAt      sql.NullTime
		fallbackMode   sql.NullString
		backupProxyID  sql.NullInt64
		expiryWarnDays sql.NullInt64
	)
	if err := rows.Scan(
		&p.ID,
		&p.Name,
		&p.Protocol,
		&p.Host,
		&p.Port,
		&username,
		&password,
		&p.Status,
		&ownerUserID,
		&p.CreatedAt,
		&p.UpdatedAt,
		&expiresAt,
		&fallbackMode,
		&backupProxyID,
		&expiryWarnDays,
	); err != nil {
		return nil, err
	}
	if username.Valid {
		p.Username = username.String
	}
	if password.Valid {
		p.Password = password.String
	}
	if ownerUserID.Valid {
		p.OwnerUserID = &ownerUserID.Int64
	}
	if expiresAt.Valid {
		p.ExpiresAt = &expiresAt.Time
	}
	if backupProxyID.Valid {
		p.BackupProxyID = &backupProxyID.Int64
	}
	if fallbackMode.Valid {
		p.FallbackMode = normalizeProxyFallbackModeForStorage(fallbackMode.String)
	} else {
		p.FallbackMode = service.FallbackModeNone
	}
	if expiryWarnDays.Valid && expiryWarnDays.Int64 > 0 {
		p.ExpiryWarnDays = int(expiryWarnDays.Int64)
	} else {
		p.ExpiryWarnDays = service.ProxyDefaultExpiryWarnDays
	}
	return &p, nil
}

func (r *proxyRepository) SweepExpiredProxies(ctx context.Context, now time.Time) (int64, error) {
	all, err := r.ListAllForFallback(ctx)
	if err != nil {
		return 0, err
	}
	byID := make(map[int64]service.Proxy, len(all))
	for _, p := range all {
		byID[p.ID] = p
	}

	var totalChanged int64
	for _, p := range all {
		if p.Status != service.StatusActive || !p.IsExpired(now) {
			continue
		}
		target, change := service.ResolveProxyFallbackTarget(p, byID, now)
		if !change && p.FallbackMode == service.FallbackModeProxy {
			logger.LegacyPrintf("repository.proxy", "[ProxyExpiry] proxy %d expired but fallback chain unresolved; accounts kept", p.ID)
		}
		changed, err := r.sweepOneExpiredProxy(ctx, p.ID, target, change)
		if err != nil {
			return totalChanged, err
		}
		totalChanged += changed
	}
	if totalChanged > 0 {
		if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventFullRebuild, nil, nil, nil); err != nil {
			logger.LegacyPrintf("repository.proxy", "[SchedulerOutbox] enqueue proxy expiry rebuild failed: err=%v", err)
		}
	}
	return totalChanged, nil
}

func (r *proxyRepository) sweepOneExpiredProxy(ctx context.Context, proxyID int64, target *int64, change bool) (int64, error) {
	if _, err := r.sql.ExecContext(ctx,
		`UPDATE proxies SET status=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`,
		service.StatusExpired, proxyID); err != nil {
		return 0, err
	}
	if !change {
		return 0, nil
	}
	var (
		res sql.Result
		err error
	)
	if target == nil {
		res, err = r.sql.ExecContext(ctx, `
			UPDATE accounts SET proxy_id=NULL, proxy_fallback_origin_id=$1, updated_at=NOW()
			WHERE proxy_id=$1 AND proxy_fallback_origin_id IS NULL AND deleted_at IS NULL`, proxyID)
	} else {
		res, err = r.sql.ExecContext(ctx, `
			UPDATE accounts SET proxy_id=$2, proxy_fallback_origin_id=$1, updated_at=NOW()
			WHERE proxy_id=$1 AND proxy_fallback_origin_id IS NULL AND deleted_at IS NULL`, proxyID, *target)
	}
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *proxyRepository) CountExpired(ctx context.Context) (int64, error) {
	var c int64
	err := scanSingleRow(ctx, r.sql, `SELECT COUNT(*) FROM proxies WHERE status=$1 AND deleted_at IS NULL`, []any{service.StatusExpired}, &c)
	return c, err
}

func (r *proxyRepository) CountExpiringSoon(ctx context.Context, now time.Time) (int64, error) {
	var c int64
	err := scanSingleRow(ctx, r.sql, `
		SELECT COUNT(*)
		FROM proxies
		WHERE deleted_at IS NULL
			AND status=$1
			AND expires_at IS NOT NULL
			AND expires_at > $2
			AND expires_at <= $2 + (expiry_warn_days || ' days')::interval
	`, []any{service.StatusActive, now}, &c)
	return c, err
}
