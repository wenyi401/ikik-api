package claudeweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ikik-api/internal/pkg/proxyurl"

	fhttp "github.com/bogdanfinn/fhttp"
)

const defaultBaseURL = "https://claude.ai"

type HTTPError struct {
	Operation  string
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "Claude Web request failed"
	}
	message := fmt.Sprintf("Claude Web %s returned %d", e.Operation, e.StatusCode)
	if body := strings.TrimSpace(string(e.Body)); body != "" {
		return message + ": " + body
	}
	return message
}

type CompletionStream struct {
	ConversationID       string
	HumanMessageUUID     string
	AssistantMessageUUID string
	Temporary            bool
	StatusCode           int
	Header               http.Header
	Body                 io.ReadCloser
}

type Client struct {
	baseURL     string
	credentials Credentials
	session     *browserSession
	transport   browserTransport
}

func NewClient(credentials Credentials, proxyURL string) (*Client, error) {
	return newClient(credentials, proxyURL, defaultBaseURL)
}

func newClient(credentials Credentials, proxyURL, baseURL string) (*Client, error) {
	credentials.SessionKey = strings.TrimSpace(credentials.SessionKey)
	credentials.BrowserCookie = strings.TrimSpace(credentials.BrowserCookie)
	if credentials.SessionKey == "" {
		credentials.SessionKey = cookieValue(credentials.BrowserCookie, "sessionKey")
	}
	if credentials.SessionKey == "" {
		return nil, fmt.Errorf("claude Web session key is required")
	}
	if credentials.AuthMode != AuthModeFullCookie {
		credentials.AuthMode = AuthModeSessionKey
	}

	trimmedProxy, _, err := proxyurl.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("parse Claude Web proxy: %w", err)
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	transport, err := newTLSBrowserTransport(baseURL, trimmedProxy)
	if err != nil {
		return nil, err
	}
	session := newBrowserSession(credentials)
	credentials.OrgUUID = firstNonEmpty(credentials.OrgUUID, session.orgUUID)

	return &Client{
		baseURL:     baseURL,
		credentials: credentials,
		session:     session,
		transport:   transport,
	}, nil
}

func (c *Client) Close() {
	if c != nil && c.transport != nil {
		c.transport.CloseIdleConnections()
	}
}

func (c *Client) Validate(ctx context.Context) error {
	_, err := c.ResolveOrganization(ctx)
	return err
}

func (c *Client) ResolveOrganization(ctx context.Context) (string, error) {
	if c == nil {
		return "", fmt.Errorf("claude Web client is not configured")
	}
	if organizationID := strings.TrimSpace(c.credentials.OrgUUID); organizationID != "" {
		return organizationID, nil
	}

	var organizations []struct {
		UUID          string  `json:"uuid"`
		RateLimitTier string  `json:"rate_limit_tier"`
		RavenType     *string `json:"raven_type"`
	}
	response, err := c.doRequest(ctx, fhttp.MethodGet, "/api/organizations", "/new", nil)
	if err != nil {
		return "", fmt.Errorf("request Claude Web organizations: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", newHTTPError("organizations", response)
	}
	if err := json.NewDecoder(response.Body).Decode(&organizations); err != nil {
		return "", fmt.Errorf("decode Claude Web organizations: %w", err)
	}
	if len(organizations) == 0 {
		return "", fmt.Errorf("claude Web returned no organizations")
	}

	for _, organization := range organizations {
		if organization.RavenType != nil && strings.EqualFold(strings.TrimSpace(*organization.RavenType), "team") {
			return c.rememberOrganization(organization.UUID), nil
		}
	}
	for _, organization := range organizations {
		switch strings.TrimSpace(organization.RateLimitTier) {
		case "default_claude_ai", "default_claude_max_20x", "default_raven_enterprise":
			return c.rememberOrganization(organization.UUID), nil
		}
	}
	return c.rememberOrganization(organizations[0].UUID), nil
}

func (c *Client) rememberOrganization(organizationID string) string {
	organizationID = strings.TrimSpace(organizationID)
	c.credentials.OrgUUID = organizationID
	if c.session != nil {
		c.session.orgUUID = organizationID
	}
	return organizationID
}

func (c *Client) StartCompletion(ctx context.Context, options CompletionOptions) (*CompletionStream, error) {
	if c == nil {
		return nil, fmt.Errorf("claude Web client is not configured")
	}
	options = normalizeCompletionOptions(options)
	if err := ValidateModel(options.Model); err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.Prompt) == "" {
		return nil, fmt.Errorf("claude Web prompt is required")
	}
	organizationID, err := c.ResolveOrganization(ctx)
	if err != nil {
		return nil, err
	}

	conversationID := strings.TrimSpace(options.ConversationID)
	createdConversation := false
	if conversationID == "" {
		conversationID, err = c.createConversation(ctx, organizationID)
		if err != nil {
			return nil, err
		}
		createdConversation = true
	}

	payload, humanUUID, assistantUUID := buildCompletionRequest(options)
	path := fmt.Sprintf("/api/organizations/%s/chat_conversations/%s/completion", organizationID, conversationID)
	response, err := c.doRequest(ctx, fhttp.MethodPost, path, "/chat/"+conversationID, payload)
	if err != nil {
		if createdConversation {
			_ = c.DeleteConversation(context.Background(), conversationID)
		}
		return nil, fmt.Errorf("request Claude Web completion: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		failure := newHTTPError("completion", response)
		_ = response.Body.Close()
		if createdConversation {
			_ = c.DeleteConversation(context.Background(), conversationID)
		}
		return nil, failure
	}

	return &CompletionStream{
		ConversationID:       conversationID,
		HumanMessageUUID:     humanUUID,
		AssistantMessageUUID: assistantUUID,
		Temporary:            !options.Persistent,
		StatusCode:           response.StatusCode,
		Header:               standardHeader(response.Header),
		Body:                 response.Body,
	}, nil
}

func (c *Client) DeleteConversation(ctx context.Context, conversationID string) error {
	if c == nil || strings.TrimSpace(conversationID) == "" {
		return nil
	}
	organizationID, err := c.ResolveOrganization(ctx)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/api/organizations/%s/chat_conversations/%s", organizationID, conversationID)
	response, err := c.doRequest(ctx, fhttp.MethodDelete, path, "/chat/"+conversationID, nil)
	if err != nil {
		return fmt.Errorf("delete Claude Web conversation: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent {
		return newHTTPError("delete conversation", response)
	}
	return nil
}

func (c *Client) createConversation(ctx context.Context, organizationID string) (string, error) {
	payload := map[string]any{
		"name":              "chat",
		"organization_uuid": organizationID,
	}
	path := fmt.Sprintf("/api/organizations/%s/chat_conversations", organizationID)
	var result struct {
		UUID string `json:"uuid"`
	}
	response, err := c.doRequest(ctx, fhttp.MethodPost, path, "/new", payload)
	if err != nil {
		return "", fmt.Errorf("create Claude Web conversation: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return "", newHTTPError("create conversation", response)
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Claude Web conversation: %w", err)
	}
	if strings.TrimSpace(result.UUID) == "" {
		return "", fmt.Errorf("claude Web conversation response missing uuid")
	}
	return strings.TrimSpace(result.UUID), nil
}

func (c *Client) doRequest(ctx context.Context, method, path, refererPath string, payload any) (*fhttp.Response, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode Claude Web request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := c.transport.NewRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if refererPath != "" {
		request.Header.Set("Referer", c.baseURL+refererPath)
	}
	request.Header.Set("Priority", "u=1, i")
	c.session.apply(request)
	return c.transport.Do(request)
}

func newHTTPError(operation string, response *fhttp.Response) *HTTPError {
	if response == nil {
		return &HTTPError{Operation: operation}
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	body = bytes.TrimSpace(body)
	return &HTTPError{Operation: operation, StatusCode: response.StatusCode, Body: body}
}

func standardHeader(source fhttp.Header) http.Header {
	result := make(http.Header, len(source))
	for key, values := range source {
		result[key] = append([]string(nil), values...)
	}
	return result
}
