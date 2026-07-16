//go:build unit

// Phase-3 TASK-001 T5：antigravity Forward（Claude→Gemini 转换路径）热路径基准，
// 纳入 scripts/bench-baseline.sh 的回归 gate（基准名匹配 BenchmarkGatewayForward 模式）。
// 覆盖 :794 分发点背后的真实转发成本：请求解析 + Claude→Gemini 转换 +
// v1internal 包裹 + 上游 SSE 解包 +（流式透传转换 / 非流式收集合并）。
// 与 gateway_forward_benchmark_test.go 同策略：mock 上游、静音 stdlib log、
// allocs/op 严格、ns/op 容忍 15% 抖动。
package service

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func benchAntigravityFwdRun(b *testing.B, reqBody string) {
	b.Helper()
	gin.SetMode(gin.TestMode)
	// antigravity 路径每请求有数条 LegacyPrintf（基准模式下全局 logger 未初始化，
	// 回退 stdlib log），静音以降低基准 I/O 噪声（baseline 与 compare 两侧同等消除）。
	prevLogOut := log.Writer()
	log.SetOutput(io.Discard)
	b.Cleanup(func() { log.SetOutput(prevLogOut) })

	upstreamSSE := "data: {\"response\":{\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hel\"}]}}]}}\n\n" +
		"data: {\"response\":{\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"lo\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":12,\"candidatesTokenCount\":5,\"cachedContentTokenCount\":3}}}\n\n"

	account := passCharAntigravityAccount(9800, map[string]any{"claude-sonnet-4-5": "gemini-3-pro-high"})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		upstream := &httpUpstreamStub{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"rid-bench-ag"},
			},
			Body: io.NopCloser(strings.NewReader(upstreamSSE)),
		}}
		svc := passCharAntigravityService(upstream)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(reqBody))
		if _, err := svc.Forward(context.Background(), c, account, []byte(reqBody), false); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGatewayForward_AntigravityClaudeNonStream(b *testing.B) {
	benchAntigravityFwdRun(b,
		`{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`,
	)
}

func BenchmarkGatewayForward_AntigravityClaudeStream(b *testing.B) {
	benchAntigravityFwdRun(b,
		`{"model":"claude-sonnet-4-5","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hi"}]}`,
	)
}
