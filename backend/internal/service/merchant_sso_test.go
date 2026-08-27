package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type merchantSSOTestRepo struct {
	integration *MerchantSSOIntegration
	bindings    map[int64]*MerchantSSOBinding
	requests    []*MerchantSSOBinding
}

func (r *merchantSSOTestRepo) ListIntegrations(_ context.Context, enabledOnly bool) ([]MerchantSSOIntegration, error) {
	if r.integration == nil || (enabledOnly && !r.integration.Enabled) {
		return []MerchantSSOIntegration{}, nil
	}
	return []MerchantSSOIntegration{*r.integration}, nil
}
func (r *merchantSSOTestRepo) GetIntegrationByID(_ context.Context, id int64) (*MerchantSSOIntegration, error) {
	if r.integration == nil || r.integration.ID != id {
		return nil, ErrMerchantSSONotFound
	}
	return r.integration, nil
}
func (r *merchantSSOTestRepo) GetIntegrationByCode(_ context.Context, code string) (*MerchantSSOIntegration, error) {
	if r.integration == nil || r.integration.MerchantCode != code {
		return nil, ErrMerchantSSONotFound
	}
	return r.integration, nil
}
func (r *merchantSSOTestRepo) CreateIntegration(_ context.Context, item *MerchantSSOIntegration) error {
	item.ID = 1
	r.integration = item
	return nil
}
func (r *merchantSSOTestRepo) UpdateIntegration(_ context.Context, item *MerchantSSOIntegration) error {
	r.integration = item
	return nil
}
func (r *merchantSSOTestRepo) GetBinding(_ context.Context, _, userID int64) (*MerchantSSOBinding, error) {
	return r.bindings[userID], nil
}
func (r *merchantSSOTestRepo) UpsertBinding(_ context.Context, item *MerchantSSOBinding) error {
	copy := *item
	r.bindings[item.UserID] = &copy
	r.requests = append(r.requests, &copy)
	return nil
}
func (r *merchantSSOTestRepo) ListBindings(_ context.Context, _ int64) ([]MerchantSSOBinding, error) {
	return nil, nil
}

type merchantSSOTestUserRepo struct {
	UserRepository
	user *User
}

func (r *merchantSSOTestUserRepo) GetByID(_ context.Context, _ int64) (*User, error) {
	return r.user, nil
}
func (r *merchantSSOTestUserRepo) GetByEmail(_ context.Context, email string) (*User, error) {
	if r.user != nil && strings.EqualFold(r.user.Email, strings.TrimSpace(email)) {
		return r.user, nil
	}
	return nil, ErrUserNotFound
}

type merchantSSOPlainEncryptor struct{}

func (merchantSSOPlainEncryptor) Encrypt(value string) (string, error) { return value, nil }
func (merchantSSOPlainEncryptor) Decrypt(value string) (string, error) { return value, nil }

type merchantSSOPrefixEncryptor struct{}

func (merchantSSOPrefixEncryptor) Encrypt(value string) (string, error) { return "enc:" + value, nil }
func (merchantSSOPrefixEncryptor) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "enc:"), nil
}

func testMerchantSSOIntegration(baseURL string) *MerchantSSOIntegration {
	return &MerchantSSOIntegration{
		ID: 1, MerchantCode: "m-1", Enabled: true,
		RegisterLoginURL: baseURL + "/register", LoginURL: baseURL + "/login", UserSyncURL: baseURL + "/sync",
		UserSyncAuthType: MerchantSSOAuthNone, AllowedRedirectHosts: []string{"127.0.0.1"},
	}
}

func TestMerchantSSOStartLoginUsesRegisterThenLogin(t *testing.T) {
	var paths []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		var payload map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/register" {
			require.Equal(t, "m-1", payload["merchantCode"])
			require.Equal(t, "42", payload["userId"])
			require.Equal(t, "alice", payload["username"])
			_, _ = w.Write([]byte(`{"success":true,"data":{"user_id":"ext-42","account":"alice","redirect_url":"https://127.0.0.1/welcome?ticket=one"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"redirect_url":"https://127.0.0.1/consume?ticket=two"}}`))
	}))
	defer server.Close()
	repo := &merchantSSOTestRepo{integration: testMerchantSSOIntegration(server.URL), bindings: map[int64]*MerchantSSOBinding{}}
	svc := NewMerchantSSOService(repo, &merchantSSOTestUserRepo{user: &User{ID: 42, Email: "alice@example.com", Username: "alice"}}, nil)
	svc.client = server.Client()

	first, err := svc.StartLogin(context.Background(), "m-1", 42)
	require.NoError(t, err)
	require.True(t, first.FirstLogin)
	require.Equal(t, "https://127.0.0.1/welcome?ticket=one", first.RedirectURL)
	require.Equal(t, "ext-42", repo.bindings[42].ExternalUserID)

	second, err := svc.StartLogin(context.Background(), "m-1", 42)
	require.NoError(t, err)
	require.False(t, second.FirstLogin)
	require.Equal(t, "https://127.0.0.1/consume?ticket=two", second.RedirectURL)
	require.Equal(t, []string{"/register", "/login"}, paths)
}

func TestMerchantSSOSyncSkipsUnknownUsersAndSignsHMAC(t *testing.T) {
	const secret = "sync-secret"
	var gotSignature, gotCanonical string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := r.Header.Get("X-Nonce")
		timestamp := r.Header.Get("X-Timestamp")
		mac := hmac.New(sha256.New, []byte(secret))
		gotCanonical = strings.Join([]string{"POST", "/sync", timestamp, nonce}, "\n")
		_, _ = mac.Write([]byte(gotCanonical))
		gotSignature = r.Header.Get("X-Signature")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"users":[{"email":"alice@example.com","externalUserId":"ext-1"},{"email":"missing@example.com","externalUserId":"ext-2"},{"email":"bad@example.com"}]}}`))
	}))
	defer server.Close()
	integration := testMerchantSSOIntegration(server.URL)
	integration.UserSyncAuthType = MerchantSSOAuthHMAC
	integration.HMACSecretEncrypted = secret
	repo := &merchantSSOTestRepo{integration: integration, bindings: map[int64]*MerchantSSOBinding{}}
	users := &merchantSSOTestUserRepo{user: &User{ID: 42, Email: "alice@example.com", Username: "alice"}}
	svc := NewMerchantSSOService(repo, users, merchantSSOPlainEncryptor{})
	svc.client = server.Client()

	result, err := svc.SyncUsers(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, &MerchantSSOSyncResult{Matched: 1, Created: 1, Skipped: 2}, result)
	require.Equal(t, "ext-1", repo.bindings[42].ExternalUserID)
	expectedMAC := hmac.New(sha256.New, []byte(secret))
	_, _ = expectedMAC.Write([]byte(gotCanonical))
	require.Equal(t, hex.EncodeToString(expectedMAC.Sum(nil)), gotSignature)
}

func TestMerchantSSORejectsUnsafeRedirect(t *testing.T) {
	err := validateRedirectURL("http://127.0.0.1/ticket", []string{"127.0.0.1"})
	require.ErrorIs(t, err, ErrMerchantSSORedirect)
	err = validateRedirectURL("https://evil.example/ticket", []string{"merchant.example"})
	require.ErrorIs(t, err, ErrMerchantSSORedirect)
	require.NoError(t, validateRedirectURL("https://merchant.example/ticket", []string{"merchant.example:443"}))
}

func TestMerchantSSOGenerateHMACSecretStoresOnlyEncryptedValue(t *testing.T) {
	repo := &merchantSSOTestRepo{integration: &MerchantSSOIntegration{
		ID: 1, MerchantCode: "m-1", Enabled: false,
		RegisterLoginURL: "https://merchant.example/register", LoginURL: "https://merchant.example/login", UserSyncURL: "https://merchant.example/sync",
		UserSyncAuthType: MerchantSSOAuthHMAC, AllowedRedirectHosts: []string{"merchant.example"},
	}, bindings: map[int64]*MerchantSSOBinding{}}
	svc := NewMerchantSSOService(repo, nil, merchantSSOPrefixEncryptor{})
	secret, err := svc.GenerateHMACSecret(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, secret, 64)
	require.Equal(t, "enc:"+secret, repo.integration.HMACSecretEncrypted)
}
