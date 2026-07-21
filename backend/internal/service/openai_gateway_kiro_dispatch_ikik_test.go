package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"ikik-api/internal/pkg/tlsfingerprint"
)

type kiroDispatchHTTPUpstream struct {
	request *http.Request
	body    []byte
}

func (u *kiroDispatchHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	var err error
	u.body, err = io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	return nil, errors.New("kiro dispatch test transport failure")
}

func (u *kiroDispatchHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestForwardAsChatCompletionsDispatchesKiroOAuthToKiroRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &kiroDispatchHTTPUpstream{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          42,
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Concurrency: 2,
		Credentials: map[string]any{"access_token": "kiro-access-token"},
	}
	body := []byte(`{"model":"claude-sonnet-4-6","stream":false,"messages":[{"role":"user","content":"hello"}]}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.Nil(t, result)
	require.ErrorContains(t, err, "kiro upstream request failed")
	require.NotNil(t, upstream.request)
	require.Equal(t, "q.us-east-1.amazonaws.com", upstream.request.URL.Host)
	require.Equal(t, "/generateAssistantResponse", upstream.request.URL.Path)
	require.Contains(t, string(upstream.body), `"modelId":"claude-sonnet-4.6"`)
	require.Equal(t, http.StatusBadGateway, recorder.Code)
}
