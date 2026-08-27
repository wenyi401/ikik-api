package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	MerchantSSOAuthNone         = "none"
	MerchantSSOAuthHMAC         = "hmac"
	merchantSSOHTTPTimeout      = 15 * time.Second
	merchantSSOMaxResponseBytes = 1 << 20
)

var (
	ErrMerchantSSONotFound      = infraerrors.NotFound("MERCHANT_SSO_NOT_FOUND", "merchant SSO integration not found")
	ErrMerchantSSODisabled      = infraerrors.Forbidden("MERCHANT_SSO_DISABLED", "merchant SSO integration is disabled")
	ErrMerchantSSOInvalidConfig = infraerrors.BadRequest("MERCHANT_SSO_INVALID_CONFIG", "merchant SSO integration configuration is invalid")
	ErrMerchantSSOUpstream      = infraerrors.New(http.StatusBadGateway, "MERCHANT_SSO_UPSTREAM_ERROR", "merchant SSO upstream request failed")
	ErrMerchantSSOResponse      = infraerrors.New(http.StatusBadGateway, "MERCHANT_SSO_INVALID_RESPONSE", "merchant SSO returned an invalid response")
	ErrMerchantSSORedirect      = infraerrors.New(http.StatusBadGateway, "MERCHANT_SSO_INVALID_REDIRECT", "merchant SSO returned an unsafe redirect URL")
)

// MerchantSSOIntegration is the persisted configuration for one merchant.
// HMACSecretEncrypted is intentionally never serialized by handlers.
type MerchantSSOIntegration struct {
	ID                   int64     `json:"id"`
	MerchantCode         string    `json:"merchant_code"`
	MerchantName         string    `json:"merchant_name"`
	Enabled              bool      `json:"enabled"`
	RegisterLoginURL     string    `json:"register_login_url"`
	LoginURL             string    `json:"login_url"`
	UserSyncURL          string    `json:"user_sync_url"`
	UserSyncAuthType     string    `json:"user_sync_auth_type"`
	HMACSecretEncrypted  string    `json:"-"`
	AllowedRedirectHosts []string  `json:"allowed_redirect_hosts"`
	HMACConfigured       bool      `json:"hmac_configured"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type MerchantSSOBinding struct {
	ID              int64     `json:"id"`
	IntegrationID   int64     `json:"integration_id"`
	UserID          int64     `json:"user_id"`
	ExternalUserID  string    `json:"external_user_id"`
	ExternalAccount string    `json:"external_account,omitempty"`
	Email           string    `json:"email"`
	Status          string    `json:"status,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type MerchantSSORepository interface {
	ListIntegrations(ctx context.Context, enabledOnly bool) ([]MerchantSSOIntegration, error)
	GetIntegrationByID(ctx context.Context, id int64) (*MerchantSSOIntegration, error)
	GetIntegrationByCode(ctx context.Context, merchantCode string) (*MerchantSSOIntegration, error)
	CreateIntegration(ctx context.Context, integration *MerchantSSOIntegration) error
	UpdateIntegration(ctx context.Context, integration *MerchantSSOIntegration) error
	GetBinding(ctx context.Context, integrationID, userID int64) (*MerchantSSOBinding, error)
	UpsertBinding(ctx context.Context, binding *MerchantSSOBinding) error
	ListBindings(ctx context.Context, integrationID int64) ([]MerchantSSOBinding, error)
}

type MerchantSSOService struct {
	repo      MerchantSSORepository
	userRepo  UserRepository
	encryptor SecretEncryptor
	client    *http.Client
}

func NewMerchantSSOService(repo MerchantSSORepository, userRepo UserRepository, encryptor SecretEncryptor) *MerchantSSOService {
	return &MerchantSSOService{
		repo:      repo,
		userRepo:  userRepo,
		encryptor: encryptor,
		client: &http.Client{
			Timeout: merchantSSOHTTPTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (s *MerchantSSOService) ListIntegrations(ctx context.Context, enabledOnly bool) ([]MerchantSSOIntegration, error) {
	if s == nil || s.repo == nil {
		return []MerchantSSOIntegration{}, nil
	}
	items, err := s.repo.ListIntegrations(ctx, enabledOnly)
	if err != nil {
		return nil, fmt.Errorf("list merchant SSO integrations: %w", err)
	}
	for i := range items {
		items[i].HMACConfigured = items[i].HMACSecretEncrypted != ""
		items[i].HMACSecretEncrypted = ""
	}
	return items, nil
}

func (s *MerchantSSOService) GetIntegration(ctx context.Context, merchantCode string) (*MerchantSSOIntegration, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMerchantSSONotFound
	}
	integration, err := s.repo.GetIntegrationByCode(ctx, strings.TrimSpace(merchantCode))
	if err != nil {
		return nil, fmt.Errorf("get merchant SSO integration: %w", err)
	}
	if integration == nil {
		return nil, ErrMerchantSSONotFound
	}
	integration.HMACConfigured = integration.HMACSecretEncrypted != ""
	integration.HMACSecretEncrypted = ""
	return integration, nil
}

// GetIntegrationConfig is used by the admin update path. The encrypted secret
// stays inside the backend and is never part of JSON because of json:"-".
func (s *MerchantSSOService) GetIntegrationConfig(ctx context.Context, id int64) (*MerchantSSOIntegration, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMerchantSSONotFound
	}
	item, err := s.repo.GetIntegrationByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get merchant SSO integration: %w", err)
	}
	if item == nil {
		return nil, ErrMerchantSSONotFound
	}
	return item, nil
}

type MerchantSSOLoginResult struct {
	RedirectURL string `json:"redirect_url"`
	FirstLogin  bool   `json:"first_login"`
}

func (s *MerchantSSOService) StartLogin(ctx context.Context, merchantCode string, userID int64) (*MerchantSSOLoginResult, error) {
	if s == nil || s.repo == nil || s.userRepo == nil {
		return nil, ErrMerchantSSONotFound
	}
	integration, err := s.repo.GetIntegrationByCode(ctx, strings.TrimSpace(merchantCode))
	if err != nil {
		return nil, fmt.Errorf("get merchant SSO integration: %w", err)
	}
	if integration == nil {
		return nil, ErrMerchantSSONotFound
	}
	if err := validateIntegration(integration); err != nil {
		return nil, err
	}
	if !integration.Enabled {
		return nil, ErrMerchantSSODisabled
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user for merchant SSO: %w", err)
	}
	binding, err := s.repo.GetBinding(ctx, integration.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("load merchant SSO binding: %w", err)
	}
	if binding != nil && strings.TrimSpace(binding.ExternalUserID) != "" {
		redirectURL, err := s.callLogin(ctx, integration, user, binding)
		if err != nil {
			return nil, err
		}
		return &MerchantSSOLoginResult{RedirectURL: redirectURL, FirstLogin: false}, nil
	}
	result, err := s.callRegisterLogin(ctx, integration, user)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpsertBinding(ctx, &MerchantSSOBinding{
		IntegrationID:   integration.ID,
		UserID:          integrationUserID(user),
		ExternalUserID:  result.ExternalUserID,
		ExternalAccount: result.ExternalAccount,
		Email:           user.Email,
	}); err != nil {
		return nil, fmt.Errorf("save merchant SSO binding: %w", err)
	}
	return &MerchantSSOLoginResult{RedirectURL: result.RedirectURL, FirstLogin: true}, nil
}

// integrationUserID keeps the assignment above explicit and makes accidental
// use of a merchant-side ID as the local user ID harder to introduce.
func integrationUserID(user *User) int64 {
	if user == nil {
		return 0
	}
	return user.ID
}

type registerLoginResult struct {
	ExternalUserID  string
	ExternalAccount string
	RedirectURL     string
}

func (s *MerchantSSOService) callRegisterLogin(ctx context.Context, integration *MerchantSSOIntegration, user *User) (*registerLoginResult, error) {
	payload := map[string]string{
		"merchantCode": integration.MerchantCode,
		"userId":       fmt.Sprintf("%d", user.ID),
		"username":     user.Username,
		"nickname":     user.Username,
		"email":        user.Email,
		"phone":        "",
	}
	var data struct {
		UserID      string `json:"user_id"`
		Account     string `json:"account"`
		RedirectURL string `json:"redirect_url"`
	}
	if err := s.postMerchant(ctx, integration.RegisterLoginURL, MerchantSSOAuthNone, "", payload, &data); err != nil {
		return nil, err
	}
	if strings.TrimSpace(data.UserID) == "" || strings.TrimSpace(data.RedirectURL) == "" {
		return nil, ErrMerchantSSOResponse
	}
	if err := validateRedirectURL(data.RedirectURL, integration.AllowedRedirectHosts); err != nil {
		return nil, err
	}
	return &registerLoginResult{ExternalUserID: strings.TrimSpace(data.UserID), ExternalAccount: strings.TrimSpace(data.Account), RedirectURL: data.RedirectURL}, nil
}

func (s *MerchantSSOService) callLogin(ctx context.Context, integration *MerchantSSOIntegration, user *User, binding *MerchantSSOBinding) (string, error) {
	payload := map[string]string{
		"merchantCode":    integration.MerchantCode,
		"userId":          fmt.Sprintf("%d", user.ID),
		"externalUserId":  binding.ExternalUserID,
		"externalAccount": binding.ExternalAccount,
	}
	var data struct {
		RedirectURL string `json:"redirect_url"`
	}
	if err := s.postMerchant(ctx, integration.LoginURL, MerchantSSOAuthNone, "", payload, &data); err != nil {
		return "", err
	}
	if strings.TrimSpace(data.RedirectURL) == "" {
		return "", ErrMerchantSSOResponse
	}
	if err := validateRedirectURL(data.RedirectURL, integration.AllowedRedirectHosts); err != nil {
		return "", err
	}
	return data.RedirectURL, nil
}

type merchantUserSyncItem struct {
	Email           string `json:"email"`
	ExternalUserID  string `json:"externalUserId"`
	ExternalAccount string `json:"externalAccount"`
	Nickname        string `json:"nickname"`
	Phone           string `json:"phone"`
	Status          string `json:"status"`
}

type MerchantSSOSyncResult struct {
	Matched int `json:"matched"`
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}

func (s *MerchantSSOService) SyncUsers(ctx context.Context, integrationID int64) (*MerchantSSOSyncResult, error) {
	if s == nil || s.repo == nil || s.userRepo == nil {
		return nil, ErrMerchantSSONotFound
	}
	integration, err := s.repo.GetIntegrationByID(ctx, integrationID)
	if err != nil {
		return nil, fmt.Errorf("get merchant SSO integration: %w", err)
	}
	if integration == nil {
		return nil, ErrMerchantSSONotFound
	}
	if err := validateIntegration(integration); err != nil {
		return nil, err
	}
	if !integration.Enabled {
		return nil, ErrMerchantSSODisabled
	}
	secret, err := s.decryptSecret(integration.HMACSecretEncrypted)
	if err != nil {
		return nil, err
	}
	var data struct {
		Users []merchantUserSyncItem `json:"users"`
	}
	if err := s.postMerchant(ctx, integration.UserSyncURL, normalizeAuthType(integration.UserSyncAuthType), secret, map[string]string{"merchantCode": integration.MerchantCode}, &data); err != nil {
		return nil, err
	}
	result := &MerchantSSOSyncResult{}
	for _, item := range data.Users {
		email := strings.TrimSpace(item.Email)
		externalID := strings.TrimSpace(item.ExternalUserID)
		if email == "" || externalID == "" {
			result.Skipped++
			continue
		}
		user, lookupErr := s.userRepo.GetByEmail(ctx, email)
		if lookupErr != nil {
			if errors.Is(lookupErr, ErrUserNotFound) {
				result.Skipped++
				continue
			}
			return nil, fmt.Errorf("match merchant SSO user %q: %w", email, lookupErr)
		}
		existing, bindingErr := s.repo.GetBinding(ctx, integration.ID, user.ID)
		if bindingErr != nil {
			return nil, fmt.Errorf("load merchant SSO binding for user %d: %w", user.ID, bindingErr)
		}
		if existing == nil {
			result.Created++
		} else {
			result.Updated++
		}
		if err := s.repo.UpsertBinding(ctx, &MerchantSSOBinding{IntegrationID: integration.ID, UserID: user.ID, ExternalUserID: externalID, ExternalAccount: strings.TrimSpace(item.ExternalAccount), Email: email, Status: strings.TrimSpace(item.Status)}); err != nil {
			return nil, fmt.Errorf("save merchant SSO binding for user %d: %w", user.ID, err)
		}
		result.Matched++
	}
	return result, nil
}

func (s *MerchantSSOService) ListBindings(ctx context.Context, integrationID int64) ([]MerchantSSOBinding, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMerchantSSONotFound
	}
	items, err := s.repo.ListBindings(ctx, integrationID)
	if err != nil {
		return nil, fmt.Errorf("list merchant SSO bindings: %w", err)
	}
	return items, nil
}

func (s *MerchantSSOService) SaveIntegration(ctx context.Context, integration *MerchantSSOIntegration, hmacSecret *string) error {
	if s == nil || s.repo == nil {
		return ErrMerchantSSOInvalidConfig
	}
	if err := validateIntegration(integration); err != nil {
		return err
	}
	if hmacSecret != nil {
		secret := strings.TrimSpace(*hmacSecret)
		if secret == "" {
			integration.HMACSecretEncrypted = ""
		} else {
			if s.encryptor == nil {
				return fmt.Errorf("%w: secret encryption is unavailable", ErrMerchantSSOInvalidConfig)
			}
			encrypted, err := s.encryptor.Encrypt(secret)
			if err != nil {
				return fmt.Errorf("encrypt merchant SSO secret: %w", err)
			}
			integration.HMACSecretEncrypted = encrypted
		}
	}
	if integration.ID == 0 {
		return s.repo.CreateIntegration(ctx, integration)
	}
	return s.repo.UpdateIntegration(ctx, integration)
}

// GenerateHMACSecret creates and stores a new secret. The plaintext is returned
// exactly once to the caller; callers must deliver it to the merchant through a
// server-side secret store and must not persist it in frontend state.
func (s *MerchantSSOService) GenerateHMACSecret(ctx context.Context, integrationID int64) (string, error) {
	if s == nil || s.repo == nil {
		return "", ErrMerchantSSONotFound
	}
	integration, err := s.repo.GetIntegrationByID(ctx, integrationID)
	if err != nil {
		return "", fmt.Errorf("get merchant SSO integration: %w", err)
	}
	if integration == nil {
		return "", ErrMerchantSSONotFound
	}
	if err := validateIntegration(integration); err != nil {
		return "", err
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", fmt.Errorf("generate merchant SSO secret: %w", err)
	}
	secret := hex.EncodeToString(secretBytes)
	if err := s.SaveIntegration(ctx, integration, &secret); err != nil {
		return "", err
	}
	return secret, nil
}

func (s *MerchantSSOService) decryptSecret(encrypted string) (string, error) {
	if strings.TrimSpace(encrypted) == "" {
		return "", nil
	}
	if s.encryptor == nil {
		return "", fmt.Errorf("%w: secret encryption is unavailable", ErrMerchantSSOInvalidConfig)
	}
	secret, err := s.encryptor.Decrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt merchant SSO secret: %w", err)
	}
	return secret, nil
}

func (s *MerchantSSOService) postMerchant(ctx context.Context, endpoint, authType, secret string, payload any, data any) error {
	endpoint = strings.TrimSpace(endpoint)
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrMerchantSSOInvalidConfig
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal merchant SSO request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build merchant SSO request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-Id", newRequestID())
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	if normalizeAuthType(authType) == MerchantSSOAuthHMAC {
		if secret == "" {
			return fmt.Errorf("%w: HMAC secret is not configured", ErrMerchantSSOInvalidConfig)
		}
		nonce := newRequestID()
		timestamp := req.Header.Get("X-Timestamp")
		path := u.EscapedPath()
		if path == "" {
			path = "/"
		}
		canonical := strings.Join([]string{http.MethodPost, path, timestamp, nonce}, "\n")
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(canonical))
		req.Header.Set("X-Nonce", nonce)
		req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
	}
	client := s.client
	if client == nil {
		client = &http.Client{Timeout: merchantSSOHTTPTimeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMerchantSSOUpstream, err)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, merchantSSOMaxResponseBytes+1)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("%w: read response failed", ErrMerchantSSOUpstream)
	}
	if len(respBody) > merchantSSOMaxResponseBytes {
		return fmt.Errorf("%w: response is too large", ErrMerchantSSOResponse)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: HTTP %d", ErrMerchantSSOUpstream, resp.StatusCode)
	}
	var envelope struct {
		Success *bool           `json:"success"`
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("%w: invalid JSON", ErrMerchantSSOResponse)
	}
	if envelope.Success != nil && !*envelope.Success {
		return fmt.Errorf("%w: %s", ErrMerchantSSOResponse, strings.TrimSpace(envelope.Message))
	}
	if envelope.Success == nil && !merchantCodeSuccess(envelope.Code) {
		return fmt.Errorf("%w: %s", ErrMerchantSSOResponse, strings.TrimSpace(envelope.Message))
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return ErrMerchantSSOResponse
	}
	if err := json.Unmarshal(envelope.Data, data); err != nil {
		return fmt.Errorf("%w: invalid data", ErrMerchantSSOResponse)
	}
	return nil
}

func merchantCodeSuccess(raw json.RawMessage) bool {
	value := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	return value == "" || value == "0" || strings.EqualFold(value, "ok") || strings.EqualFold(value, "success")
}

func validateIntegration(integration *MerchantSSOIntegration) error {
	if integration == nil || strings.TrimSpace(integration.MerchantCode) == "" || strings.TrimSpace(integration.RegisterLoginURL) == "" || strings.TrimSpace(integration.LoginURL) == "" || strings.TrimSpace(integration.UserSyncURL) == "" {
		return ErrMerchantSSOInvalidConfig
	}
	authType := strings.ToLower(strings.TrimSpace(integration.UserSyncAuthType))
	if authType != "" && authType != MerchantSSOAuthNone && authType != MerchantSSOAuthHMAC {
		return ErrMerchantSSOInvalidConfig
	}
	for _, endpoint := range []string{integration.RegisterLoginURL, integration.LoginURL, integration.UserSyncURL} {
		u, err := url.Parse(strings.TrimSpace(endpoint))
		if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
			return fmt.Errorf("%w: merchant endpoints must use HTTPS", ErrMerchantSSOInvalidConfig)
		}
	}
	if len(integration.AllowedRedirectHosts) == 0 {
		return fmt.Errorf("%w: at least one redirect host is required", ErrMerchantSSOInvalidConfig)
	}
	return nil
}

func validateRedirectURL(raw string, allowedHosts []string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return ErrMerchantSSORedirect
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	for _, allowed := range allowedHosts {
		allowedHost := strings.ToLower(strings.TrimSpace(allowed))
		if strings.Contains(allowedHost, "://") {
			if parsed, parseErr := url.Parse(allowedHost); parseErr == nil {
				allowedHost = parsed.Hostname()
			}
		} else if parsed, parseErr := url.Parse("//" + allowedHost); parseErr == nil && parsed.Hostname() != "" {
			allowedHost = parsed.Hostname()
		}
		allowedHost = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(allowedHost)), ".")
		if host == allowedHost {
			return nil
		}
	}
	return ErrMerchantSSORedirect
}

func normalizeAuthType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == MerchantSSOAuthHMAC {
		return MerchantSSOAuthHMAC
	}
	return MerchantSSOAuthNone
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
