//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/config"
)

func modelProbeTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				AllowInsecureHTTP: true,
			},
		},
	}
}

func TestAccountTestService_ModelProbeOpenAIListModels(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"object":"list","data":[{"id":"gpt-5.4","object":"model","owned_by":"openai"},{"id":"gpt-5.4-openai-compact","object":"model","owned_by":"openai"}]}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModelList(context.Background(), ModelProbeListInput{
		Platform: PlatformOpenAI,
		BaseURL:  "https://one.example.com",
		APIKey:   "sk-test-secret",
	})

	require.NoError(t, err)
	require.Len(t, result.Models, 2)
	require.Equal(t, "gpt-5.4", result.Models[0].ID)
	require.Equal(t, "gpt-5.4-openai-compact", result.Models[1].ID)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.Equal(t, http.MethodGet, req.Method)
	require.Equal(t, "https://one.example.com/v1/models", req.URL.String())
	require.Equal(t, "Bearer sk-test-secret", req.Header.Get("Authorization"))
}

func TestAccountTestService_ModelProbeOpenAIResponsesMinimalRequest(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"id":"resp_123","output":[]}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModels(context.Background(), ModelProbeTestInput{
		Platform: PlatformOpenAI,
		BaseURL:  "https://one.example.com",
		APIKey:   "sk-test-secret",
		Mode:     ModelProbeModeOpenAIResponses,
		Models:   []string{"gpt-5.4"},
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].OK)
	require.Equal(t, http.StatusOK, result.Results[0].Status)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.Equal(t, http.MethodPost, req.Method)
	require.Equal(t, "https://one.example.com/v1/responses", req.URL.String())

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.4","input":"ping","max_output_tokens":1}`, string(body))
}

func TestAccountTestService_ModelProbeAnthropicListUsesBuiltInCandidates(t *testing.T) {
	upstream := &queuedHTTPUpstream{}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModelList(context.Background(), ModelProbeListInput{
		Platform: PlatformAnthropic,
		BaseURL:  "https://invalid.example.com",
		APIKey:   "sk-ant-test",
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Models)
	require.Equal(t, "anthropic", result.Models[0].OwnedBy)
	require.Empty(t, upstream.requests)
}

func TestAccountTestService_ModelProbeOpenAIChatCompletionsMinimalRequest(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"id":"chatcmpl_123","choices":[]}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModels(context.Background(), ModelProbeTestInput{
		Platform: PlatformOpenAI,
		BaseURL:  "https://one.example.com/v1",
		APIKey:   "sk-test-secret",
		Mode:     ModelProbeModeOpenAIChatCompletions,
		Models:   []string{"gpt-5.4"},
	})

	require.NoError(t, err)
	require.True(t, result.Results[0].OK)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.Equal(t, "https://one.example.com/v1/chat/completions", req.URL.String())

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.4","messages":[{"role":"user","content":"ping"}],"max_completion_tokens":1}`, string(body))
}

func TestAccountTestService_ModelProbeOpenAIChatCompletionsFallsBackToMaxTokens(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusBadRequest, `{"error":{"message":"Unknown parameter: max_completion_tokens"}}`),
		newJSONResponse(http.StatusOK, `{"id":"chatcmpl_123","choices":[]}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModels(context.Background(), ModelProbeTestInput{
		Platform: PlatformOpenAI,
		BaseURL:  "https://one.example.com/v1",
		APIKey:   "sk-test-secret",
		Mode:     ModelProbeModeOpenAIChatCompletions,
		Models:   []string{"gpt-5.4"},
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].OK)
	require.Len(t, upstream.requests, 2)

	firstBody, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.4","messages":[{"role":"user","content":"ping"}],"max_completion_tokens":1}`, string(firstBody))

	secondBody, err := io.ReadAll(upstream.requests[1].Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.4","messages":[{"role":"user","content":"ping"}],"max_tokens":1}`, string(secondBody))
}

func TestAccountTestService_ModelProbeKiroRequiresExplicitBaseURL(t *testing.T) {
	upstream := &queuedHTTPUpstream{}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModels(context.Background(), ModelProbeTestInput{
		Platform: PlatformKiro,
		APIKey:   "kiro-api-key",
		Mode:     ModelProbeModeOpenAIChatCompletions,
		Models:   []string{"claude-sonnet-4-6"},
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.False(t, result.Results[0].OK)
	require.Contains(t, strings.ToLower(result.Results[0].Error), "base url")
	require.Empty(t, upstream.requests)
}

func TestAccountTestService_ModelProbeKiroChatCompletionsRequest(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"id":"chatcmpl_kiro","choices":[]}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModels(context.Background(), ModelProbeTestInput{
		Platform: PlatformKiro,
		BaseURL:  "https://kiro-upstream.example.com/v1",
		APIKey:   "kiro-api-key",
		Mode:     ModelProbeModeOpenAIChatCompletions,
		Models:   []string{"claude-sonnet-4.6"},
	})

	require.NoError(t, err)
	require.True(t, result.Results[0].OK)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.Equal(t, "https://kiro-upstream.example.com/v1/chat/completions", req.URL.String())
	require.Equal(t, "Bearer kiro-api-key", req.Header.Get("Authorization"))

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"claude-sonnet-4.6","messages":[{"role":"user","content":"ping"}],"max_completion_tokens":1}`, string(body))
}

func TestAccountTestService_ModelProbeErrorDoesNotExposeAPIKey(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusUnauthorized, `{"error":{"message":"invalid key sk-test-secret"}}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModels(context.Background(), ModelProbeTestInput{
		Platform: PlatformOpenAI,
		BaseURL:  "https://one.example.com",
		APIKey:   "sk-test-secret",
		Mode:     ModelProbeModeOpenAIResponses,
		Models:   []string{"gpt-5.4"},
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.False(t, result.Results[0].OK)
	require.Equal(t, http.StatusUnauthorized, result.Results[0].Status)
	require.NotContains(t, result.Results[0].Error, "sk-test-secret")
	require.Contains(t, strings.ToLower(result.Results[0].Error), "invalid key")
}

func TestAccountTestService_ModelProbeErrorDoesNotExposePartialAPIKey(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusUnauthorized, `{"error":{"message":"invalid key sk-test-sec..."}}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModels(context.Background(), ModelProbeTestInput{
		Platform: PlatformOpenAI,
		BaseURL:  "https://one.example.com",
		APIKey:   "sk-test-secret",
		Mode:     ModelProbeModeOpenAIResponses,
		Models:   []string{"gpt-5.4"},
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.False(t, result.Results[0].OK)
	require.NotContains(t, result.Results[0].Error, "sk-test")
	require.Contains(t, strings.ToLower(result.Results[0].Error), "invalid key")
}

func TestAccountTestService_UserModelProbeRejectsUnsafeBaseURLs(t *testing.T) {
	tests := []string{
		"http://api.example.com",
		"https://127.0.0.1",
		"https://10.0.0.8",
		"https://169.254.169.254/latest/meta-data",
		"https://[::1]",
	}

	for _, baseURL := range tests {
		t.Run(baseURL, func(t *testing.T) {
			upstream := &queuedHTTPUpstream{}
			svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

			_, err := svc.ProbeModelListForUser(context.Background(), ModelProbeListInput{
				Platform: PlatformOpenAI,
				BaseURL:  baseURL,
				APIKey:   "sk-test-secret",
			})

			require.Error(t, err)
			require.Contains(t, strings.ToLower(err.Error()), "unsafe base url")
			require.Empty(t, upstream.requests)
		})
	}
}

func TestAccountTestService_UserModelProbeUsesSafePublicURLAndDisablesRedirects(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"data":[{"id":"gpt-5.4"}]}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModelListForUser(context.Background(), ModelProbeListInput{
		Platform: PlatformOpenAI,
		BaseURL:  "https://1.1.1.1",
		APIKey:   "sk-test-secret",
	})

	require.NoError(t, err)
	require.Equal(t, "gpt-5.4", result.Models[0].ID)
	require.Len(t, upstream.requests, 1)
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.requests[0].Context()))
}

func TestAccountTestService_AdminModelProbeBehaviorRemainsConfigurable(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"data":[{"id":"local-model"}]}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: modelProbeTestConfig()}

	result, err := svc.ProbeModelList(context.Background(), ModelProbeListInput{
		Platform: PlatformOpenAI,
		BaseURL:  "http://127.0.0.1:11434",
		APIKey:   "local-key",
	})

	require.NoError(t, err)
	require.Equal(t, "local-model", result.Models[0].ID)
	require.Len(t, upstream.requests, 1)
}
