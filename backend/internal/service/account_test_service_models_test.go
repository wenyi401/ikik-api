package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"ikik-api/internal/config"
	"ikik-api/internal/pkg/tlsfingerprint"
)

type modelSyncHTTPUpstream struct{}

func (modelSyncHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func (modelSyncHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func TestFetchUpstreamSupportedModelsOpenAICompatible(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.4"},{"id":"gpt-5.4"},{"id":"mimo-v2.5"}]}`))
	}))
	defer upstream.Close()

	svc := &AccountTestService{cfg: insecureURLTestConfig(), httpUpstream: modelSyncHTTPUpstream{}}
	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-key",
			"base_url": upstream.URL + "/v1",
		},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5.4", "mimo-v2.5"}, models)
}

func TestFetchUpstreamSupportedModelsGemini(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/models", r.URL.Path)
		require.Equal(t, "gem-key", r.Header.Get("x-goog-api-key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[
			{"name":"models/gemini-2.5-pro","supportedGenerationMethods":["generateContent"]},
			{"name":"models/text-embedding-004","supportedGenerationMethods":["embedContent"]}
		]}`))
	}))
	defer upstream.Close()

	svc := &AccountTestService{cfg: insecureURLTestConfig(), httpUpstream: modelSyncHTTPUpstream{}}
	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "gem-key",
			"base_url": upstream.URL,
		},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"gemini-2.5-pro", "text-embedding-004"}, models)
}

func TestFetchUpstreamSupportedModelsRequiresAPIKey(t *testing.T) {
	svc := &AccountTestService{cfg: insecureURLTestConfig(), httpUpstream: modelSyncHTTPUpstream{}}
	_, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	})

	require.Error(t, err)
	var syncErr *UpstreamModelSyncError
	require.ErrorAs(t, err, &syncErr)
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
}

func insecureURLTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	return cfg
}
