package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ikik-api/internal/pkg/kiro"
)

func (s *AccountTestService) SetKiroTokenProvider(provider *KiroTokenProvider) {
	if s != nil {
		s.kiroTokenProvider = provider
	}
}

func (s *AccountTestService) testKiroAccountConnection(c *gin.Context, account *Account, modelID string, prompt string) error {
	testModelID := strings.TrimSpace(modelID)
	if testModelID == "" {
		testModelID = kiro.DefaultTestModelID
	}
	testModelID = account.GetMappedModel(testModelID)
	if strings.TrimSpace(testModelID) == "" {
		testModelID = kiro.MapModel(kiro.DefaultTestModelID)
	}

	if account.Type == AccountTypeOAuth {
		return s.testKiroOAuthConnection(c, account, testModelID, prompt)
	}
	if account.Type != AccountTypeAPIKey {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported Kiro account type: %s", account.Type))
	}

	authToken := account.GetOpenAIApiKey()
	if strings.TrimSpace(authToken) == "" {
		return s.sendErrorAndEnd(c, "No API key available")
	}

	baseURL := account.GetOpenAIBaseURL()
	if strings.TrimSpace(baseURL) == "" {
		return s.sendErrorAndEnd(c, "No Kiro base URL available")
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid base URL: %s", err.Error()))
	}

	return s.testOpenAIChatCompletionsConnection(c, account, testModelID, prompt, normalizedBaseURL, authToken)
}

func (s *AccountTestService) testKiroOAuthConnection(c *gin.Context, account *Account, testModelID string, prompt string) error {
	ctx := c.Request.Context()
	authToken := ""
	if s.kiroTokenProvider != nil {
		if token, err := s.kiroTokenProvider.GetAccessToken(ctx, account); err == nil {
			authToken = strings.TrimSpace(token)
		} else {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to get Kiro access token: %s", err.Error()))
		}
	}
	if authToken == "" {
		authToken = strings.TrimSpace(account.GetCredential("access_token"))
	}
	if authToken == "" {
		return s.sendErrorAndEnd(c, "No Kiro access token available")
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})
	s.sendEvent(c, TestEvent{Type: "status", Text: "Testing Kiro OAuth connection"})

	payload, err := createTestPayload(testModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create test payload")
	}
	if strings.TrimSpace(prompt) != "" {
		if messages, ok := payload["messages"].([]map[string]any); ok && len(messages) > 0 {
			if content, ok := messages[0]["content"].([]map[string]any); ok && len(content) > 0 {
				content[0]["text"] = prompt
			}
		}
	}
	anthropicBody, _ := json.Marshal(payload)

	resp, err := s.executeKiroTestUpstream(ctx, account, anthropicBody, testModelID, authToken)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Request failed: %s", err.Error()))
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return s.sendErrorAndEnd(c, formatKiroTestError(resp.StatusCode, body, testModelID, account))
	}

	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = resp.Body.Close() }()
		_, streamErr := kiro.StreamEventStreamAsAnthropicWithContext(ctx, resp.Body, pw, testModelID, estimateKiroInputTokens(anthropicBody), kiro.KiroRequestContext{})
		if streamErr != nil {
			_ = pw.CloseWithError(streamErr)
			return
		}
		_ = pw.Close()
	}()
	return s.processClaudeStream(c, pr)
}

func formatKiroTestError(statusCode int, body []byte, requestedModel string, account *Account) string {
	return fmt.Sprintf("API returned %d: %s", statusCode, string(body))
}

func (s *AccountTestService) executeKiroTestUpstream(ctx context.Context, account *Account, anthropicBody []byte, mappedModel, token string) (*http.Response, error) {
	modelID := kiro.MapModel(mappedModel)
	if strings.TrimSpace(modelID) == "" {
		modelID = mappedModel
	}
	currentToken := token
	profileARN := resolveKiroPayloadProfileArn(account)
	preparedBody := prepareKiroPayloadBodyForRequestModel(anthropicBody, mappedModel)
	buildResult, err := kiro.BuildKiroPayloadWithContext(preparedBody, modelID, profileARN, "AI_EDITOR", nil)
	if err != nil {
		return nil, err
	}
	payload := buildResult.Payload

	endpoints := buildKiroEndpoints(account, KiroEndpointModeQ)
	proxyURL := kiroProxyURL(account)
	tlsProfile := s.tlsFPProfileService.ResolveTLSProfile(account)
	accountKey := buildKiroAccountKey(account)
	const maxRetries = 2
	for endpointIndex, endpoint := range endpoints {
		for attempt := 0; attempt <= maxRetries; attempt++ {
			req, err := newKiroJSONRequest(ctx, endpoint.URL, payload, currentToken, accountKey, buildKiroMachineID(account), endpoint.AmzTarget, account)
			if err != nil {
				return nil, err
			}

			resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile)
			if err != nil {
				return nil, err
			}

			if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
				if endpointIndex+1 < len(endpoints) {
					_ = resp.Body.Close()
					break
				}
				return resp, nil
			}

			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				respBody, readErr := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if readErr != nil {
					return nil, readErr
				}

				if s.kiroTokenProvider != nil && (resp.StatusCode == http.StatusUnauthorized || isKiroTokenErrorBody(respBody)) && attempt < maxRetries {
					refreshedToken, refreshErr := s.kiroTokenProvider.ForceRefreshAccessToken(ctx, account)
					if refreshErr == nil && strings.TrimSpace(refreshedToken) != "" {
						currentToken = refreshedToken
						accountKey = buildKiroAccountKey(account)
						buildResult, err = kiro.BuildKiroPayloadWithContext(preparedBody, modelID, profileARN, "AI_EDITOR", nil)
						if err != nil {
							return nil, err
						}
						payload = buildResult.Payload
						continue
					}
				}

				resetHTTPResponseBody(resp, respBody)
				return resp, nil
			}

			if resp.StatusCode == http.StatusBadRequest {
				respBody, readErr := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if readErr != nil {
					return nil, readErr
				}
				resetHTTPResponseBody(resp, respBody)
				return resp, nil
			}

			return resp, nil
		}
	}

	return nil, fmt.Errorf("kiro upstream endpoints exhausted")
}
