//go:build unit

// Phase-0 TASK-002 特征化测试：OpenAI 路径流式/非流式透传（INVARIANTS I-1.5）。
// 固定 mock 上游，断言客户端最终收到的字节/SSE 事件序列/状态码/响应头。
package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"ikik-api/internal/config"
)

// passCharOpenAIAccount 返回 OpenAI API Key 自动透传账号夹具。
func passCharOpenAIAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Name:        "pass-char-openai",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-upstream-openai",
			"base_url": "https://upstream.example.com",
		},
		Extra: map[string]any{
			"use_responses_api":  true,
			"openai_passthrough": true,
		},
		Status:      StatusActive,
		Schedulable: true,
	}
}

func passCharOpenAIService(cfg *config.Config, upstream HTTPUpstream) *OpenAIGatewayService {
	return &OpenAIGatewayService{
		cfg:                  cfg,
		responseHeaderFilter: compileResponseHeaderFilter(cfg),
		httpUpstream:         upstream,
	}
}

func passCharOpenAIGinContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	return c, rec
}

// TestGatewayCharacterization_OpenAIStreamPassthrough 固化 I-1.5 流式侧：
// OpenAI 透传账号流式转发时，客户端收到的 SSE 内容与上游逐事件一致（按 \n\n 分帧），
// 包含 event: 行、preamble 事件（response.created）与终止事件（response.completed/[DONE]）。
func TestGatewayCharacterization_OpenAIStreamPassthrough(t *testing.T) {
	upstreamEvents := []string{
		"event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_char_1\"}}",
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Hel\"}",
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"lo ✓\"}",
		"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_char_1\",\"usage\":{\"input_tokens\":11,\"output_tokens\":4,\"input_tokens_details\":{\"cached_tokens\":2}}}}",
		"data: [DONE]",
	}
	upstreamSSE := strings.Join(upstreamEvents, "\n\n") + "\n\n"

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"x-request-id": []string{"rid-char-openai-stream"},
			},
			Body: io.NopCloser(strings.NewReader(upstreamSSE)),
		},
	}
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	svc := passCharOpenAIService(cfg, upstream)

	c, rec := passCharOpenAIGinContext(t)
	body := []byte(`{"model":"gpt-5","stream":true,"instructions":"be brief","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	result, err := svc.Forward(context.Background(), c, passCharOpenAIAccount(9101), body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)

	gotEvents := passCharSplitSSEEvents(rec.Body.String())
	require.Equal(t, upstreamEvents, gotEvents, "客户端收到的 SSE 事件序列必须与上游逐事件一致")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, "rid-char-openai-stream", rec.Header().Get("x-request-id"))
	require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))

	// 透传模式下请求侧只替换认证：上游应收到原始 body + Bearer 上游 key，不残留入站鉴权。
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, string(body), string(upstream.lastBody), "透传模式上游请求体应与客户端原始 body 一致")
	require.Equal(t, "Bearer sk-upstream-openai", upstream.lastReq.Header.Get("Authorization"))

	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.Equal(t, 2, result.Usage.CacheReadInputTokens)
}

// TestGatewayCharacterization_OpenAINonStreamPassthroughByteExact 固化 I-1.5 非流式侧：
// 上游 2xx JSON 响应体逐字节透传、状态码/Content-Type 透传，x-codex-* 配额头强制放行。
func TestGatewayCharacterization_OpenAINonStreamPassthroughByteExact(t *testing.T) {
	upstreamJSON := "{\"id\":\"resp_char_2\",  \"object\":\"response\",\n" +
		"\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"héllo <&> ✓\"}]}],\n" +
		"\"usage\":{\"input_tokens\":21,\"output_tokens\":8,\"input_tokens_details\":{\"cached_tokens\":5}}}"

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":                 []string{"application/json"},
				"x-request-id":                 []string{"rid-char-openai-nonstream"},
				"Set-Cookie":                   []string{"secret=upstream"},
				"X-Codex-Primary-Used-Percent": []string{"42.5"},
			},
			Body: io.NopCloser(strings.NewReader(upstreamJSON)),
		},
	}
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	svc := passCharOpenAIService(cfg, upstream)

	c, rec := passCharOpenAIGinContext(t)
	body := []byte(`{"model":"gpt-5","stream":false,"instructions":"be brief","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	result, err := svc.Forward(context.Background(), c, passCharOpenAIAccount(9102), body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, upstreamJSON, rec.Body.String(), "上游 2xx body 必须逐字节透传")
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Equal(t, "rid-char-openai-nonstream", rec.Header().Get("x-request-id"))
	require.Equal(t, "42.5", rec.Header().Get("X-Codex-Primary-Used-Percent"), "x-codex-* 配额头透传模式强制放行")
	require.Empty(t, rec.Header().Get("Set-Cookie"), "Set-Cookie 应被响应头过滤移除")

	require.Equal(t, 21, result.Usage.InputTokens)
	require.Equal(t, 8, result.Usage.OutputTokens)
	require.Equal(t, 5, result.Usage.CacheReadInputTokens)
}

// TestGatewayCharacterization_OpenAIPassthroughUpstreamErrorBodyVerbatim 固化 OpenAI 透传
// 模式的错误语义：非容量类 4xx（如 400）保持原样代理——上游状态码 + 原始错误体透传，
// Forward 返回 error（供 handler 记日志，但响应已写完）。
func TestGatewayCharacterization_OpenAIPassthroughUpstreamErrorBodyVerbatim(t *testing.T) {
	upstreamErrJSON := `{"error":{"type":"invalid_request_error","message":"Unsupported parameter: 'foo'","param":"foo"}}`
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"x-request-id": []string{"rid-char-openai-err"},
			},
			Body: io.NopCloser(strings.NewReader(upstreamErrJSON)),
		},
	}
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	svc := passCharOpenAIService(cfg, upstream)

	c, rec := passCharOpenAIGinContext(t)
	body := []byte(`{"model":"gpt-5","stream":false,"instructions":"be brief","input":"hi","foo":1}`)

	result, err := svc.Forward(context.Background(), c, passCharOpenAIAccount(9103), body)
	require.Error(t, err)
	require.Nil(t, result)

	require.Equal(t, http.StatusBadRequest, rec.Code, "透传模式上游错误状态码原样代理")
	require.Equal(t, upstreamErrJSON, rec.Body.String(), "透传模式上游错误体逐字节透传")
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}
