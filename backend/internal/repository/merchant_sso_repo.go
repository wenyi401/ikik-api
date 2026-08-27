package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ikik-api/internal/service"
)

type merchantSSORepository struct {
	db *sql.DB
}

func NewMerchantSSORepository(db *sql.DB) service.MerchantSSORepository {
	return &merchantSSORepository{db: db}
}

func (r *merchantSSORepository) ListIntegrations(ctx context.Context, enabledOnly bool) ([]service.MerchantSSOIntegration, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("merchant SSO database is unavailable")
	}
	query := `SELECT id, merchant_code, merchant_name, enabled, register_login_url, login_url, user_sync_url,
        user_sync_auth_type, user_sync_hmac_secret, allowed_redirect_hosts, created_at, updated_at
        FROM merchant_sso_integrations`
	args := []any{}
	if enabledOnly {
		query += " WHERE enabled = TRUE"
	}
	query += " ORDER BY id ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.MerchantSSOIntegration, 0)
	for rows.Next() {
		item, err := scanMerchantSSOIntegration(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *merchantSSORepository) GetIntegrationByID(ctx context.Context, id int64) (*service.MerchantSSOIntegration, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("merchant SSO database is unavailable")
	}
	row := r.db.QueryRowContext(ctx, `SELECT id, merchant_code, merchant_name, enabled, register_login_url, login_url, user_sync_url,
        user_sync_auth_type, user_sync_hmac_secret, allowed_redirect_hosts, created_at, updated_at
        FROM merchant_sso_integrations WHERE id = $1`, id)
	item, err := scanMerchantSSOIntegration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMerchantSSONotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *merchantSSORepository) GetIntegrationByCode(ctx context.Context, merchantCode string) (*service.MerchantSSOIntegration, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("merchant SSO database is unavailable")
	}
	row := r.db.QueryRowContext(ctx, `SELECT id, merchant_code, merchant_name, enabled, register_login_url, login_url, user_sync_url,
        user_sync_auth_type, user_sync_hmac_secret, allowed_redirect_hosts, created_at, updated_at
        FROM merchant_sso_integrations WHERE merchant_code = $1`, strings.TrimSpace(merchantCode))
	item, err := scanMerchantSSOIntegration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMerchantSSONotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *merchantSSORepository) CreateIntegration(ctx context.Context, item *service.MerchantSSOIntegration) error {
	if r == nil || r.db == nil || item == nil {
		return errors.New("merchant SSO database is unavailable")
	}
	hosts, err := json.Marshal(normalizeRedirectHosts(item.AllowedRedirectHosts))
	if err != nil {
		return err
	}
	return r.db.QueryRowContext(ctx, `INSERT INTO merchant_sso_integrations
        (merchant_code, merchant_name, enabled, register_login_url, login_url, user_sync_url,
         user_sync_auth_type, user_sync_hmac_secret, allowed_redirect_hosts)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        RETURNING id, created_at, updated_at`,
		strings.TrimSpace(item.MerchantCode), strings.TrimSpace(item.MerchantName), item.Enabled,
		strings.TrimSpace(item.RegisterLoginURL), strings.TrimSpace(item.LoginURL), strings.TrimSpace(item.UserSyncURL),
		normalizeAuthType(item.UserSyncAuthType), item.HMACSecretEncrypted, hosts,
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *merchantSSORepository) UpdateIntegration(ctx context.Context, item *service.MerchantSSOIntegration) error {
	if r == nil || r.db == nil || item == nil || item.ID <= 0 {
		return errors.New("merchant SSO integration is invalid")
	}
	hosts, err := json.Marshal(normalizeRedirectHosts(item.AllowedRedirectHosts))
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `UPDATE merchant_sso_integrations SET
        merchant_code=$1, merchant_name=$2, enabled=$3, register_login_url=$4, login_url=$5,
        user_sync_url=$6, user_sync_auth_type=$7, user_sync_hmac_secret=$8,
        allowed_redirect_hosts=$9, updated_at=NOW() WHERE id=$10`,
		strings.TrimSpace(item.MerchantCode), strings.TrimSpace(item.MerchantName), item.Enabled,
		strings.TrimSpace(item.RegisterLoginURL), strings.TrimSpace(item.LoginURL), strings.TrimSpace(item.UserSyncURL),
		normalizeAuthType(item.UserSyncAuthType), item.HMACSecretEncrypted, hosts, item.ID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return service.ErrMerchantSSONotFound
	}
	return nil
}

func (r *merchantSSORepository) GetBinding(ctx context.Context, integrationID, userID int64) (*service.MerchantSSOBinding, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("merchant SSO database is unavailable")
	}
	row := r.db.QueryRowContext(ctx, `SELECT id, integration_id, user_id, external_user_id, external_account, email, status, created_at, updated_at
        FROM merchant_sso_bindings WHERE integration_id=$1 AND user_id=$2`, integrationID, userID)
	item, err := scanMerchantSSOBinding(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *merchantSSORepository) UpsertBinding(ctx context.Context, item *service.MerchantSSOBinding) error {
	if r == nil || r.db == nil || item == nil || item.IntegrationID <= 0 || item.UserID <= 0 || strings.TrimSpace(item.ExternalUserID) == "" {
		return errors.New("merchant SSO binding is invalid")
	}
	return r.db.QueryRowContext(ctx, `INSERT INTO merchant_sso_bindings
        (integration_id, user_id, external_user_id, external_account, email, status)
        VALUES ($1,$2,$3,$4,$5,$6)
        ON CONFLICT (integration_id, user_id) DO UPDATE SET
          external_user_id=EXCLUDED.external_user_id,
          external_account=EXCLUDED.external_account,
          email=EXCLUDED.email,
          status=EXCLUDED.status,
          updated_at=NOW()
        RETURNING id, created_at, updated_at`,
		item.IntegrationID, item.UserID, strings.TrimSpace(item.ExternalUserID), strings.TrimSpace(item.ExternalAccount),
		strings.TrimSpace(item.Email), strings.TrimSpace(item.Status),
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *merchantSSORepository) ListBindings(ctx context.Context, integrationID int64) ([]service.MerchantSSOBinding, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("merchant SSO database is unavailable")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, integration_id, user_id, external_user_id, external_account, email, status, created_at, updated_at
        FROM merchant_sso_bindings WHERE integration_id=$1 ORDER BY id ASC`, integrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.MerchantSSOBinding, 0)
	for rows.Next() {
		item, err := scanMerchantSSOBinding(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type merchantSSORowScanner interface{ Scan(dest ...any) error }

func scanMerchantSSOIntegration(row merchantSSORowScanner) (service.MerchantSSOIntegration, error) {
	var item service.MerchantSSOIntegration
	var rawHosts []byte
	if err := row.Scan(&item.ID, &item.MerchantCode, &item.MerchantName, &item.Enabled, &item.RegisterLoginURL, &item.LoginURL,
		&item.UserSyncURL, &item.UserSyncAuthType, &item.HMACSecretEncrypted, &rawHosts, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	if len(rawHosts) > 0 {
		if err := json.Unmarshal(rawHosts, &item.AllowedRedirectHosts); err != nil {
			return item, fmt.Errorf("decode merchant SSO redirect hosts: %w", err)
		}
	}
	item.AllowedRedirectHosts = normalizeRedirectHosts(item.AllowedRedirectHosts)
	return item, nil
}

func scanMerchantSSOBinding(row merchantSSORowScanner) (service.MerchantSSOBinding, error) {
	var item service.MerchantSSOBinding
	if err := row.Scan(&item.ID, &item.IntegrationID, &item.UserID, &item.ExternalUserID, &item.ExternalAccount,
		&item.Email, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	return item, nil
}

func normalizeRedirectHosts(hosts []string) []string {
	seen := make(map[string]struct{}, len(hosts))
	out := make([]string, 0, len(hosts))
	for _, raw := range hosts {
		host := strings.ToLower(strings.TrimSpace(raw))
		host = strings.TrimSuffix(host, ".")
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

func normalizeAuthType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), service.MerchantSSOAuthHMAC) {
		return service.MerchantSSOAuthHMAC
	}
	return service.MerchantSSOAuthNone
}
