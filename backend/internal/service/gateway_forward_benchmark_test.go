//go:build unit

// Phase-0 TASK-005 热路径基准：覆盖 anthropic API Key 透传账号的完整
// Forward 路径（非流式 + 流式 SSE）。与 scripts/bench-baseline.sh 配套使用，
// 基线存于 testdata/bench/baseline.txt；对比策略：allocs/op 严格（不允许增加）、
// ns/op 宽松（容忍 15% 抖动）。复用 TASK-002 特征化测试的夹具（passChar* 前缀）。
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
	"ikik-api/internal/config"
)

func benchFwdRun(b *testing.B, header http.Header, upstreamBody string, reqBody string, stream bool) {
	b.Helper()
	gin.SetMode(gin.TestMode)
	// 透传分支每请求有一条 stdlib log，静音以降低基准 I/O 噪声
	// （日志成本在 baseline 与 compare 两侧同等消除，不影响对比有效性）。
	prevLogOut := log.Writer()
	log.SetOutput(io.Discard)
	b.Cleanup(func() { log.SetOutput(prevLogOut) })
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	account := passCharAnthropicAccount(9100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		upstream := &anthropicHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     header.Clone(),
				Body:       io.NopCloser(strings.NewReader(upstreamBody)),
			},
		}
		svc := passCharGatewayService(cfg, upstream)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		parsed := &ParsedRequest{
			Body:   NewRequestBodyRef([]byte(reqBody)),
			Model:  "claude-sonnet-4-20250514",
			Stream: stream,
		}
		if _, err := svc.Forward(context.Background(), c, account, parsed); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGatewayForward_AnthropicNonStreamPassthrough(b *testing.B) {
	upstreamJSON := `{"id":"msg_bench_01","type":"message","content":[{"type":"text","text":"hello"}],` +
		`"usage":{"input_tokens":12,"output_tokens":7,"cache_read_input_tokens":3}}`
	benchFwdRun(b,
		http.Header{"Content-Type": []string{"application/json"}},
		upstreamJSON,
		`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}]}`,
		false,
	)
}

func BenchmarkGatewayForward_AnthropicStreamPassthrough(b *testing.B) {
	events := []string{
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_bench_02\",\"usage\":{\"input_tokens\":9,\"cached_tokens\":3}}}",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hel\"}}",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}",
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}",
	}
	benchFwdRun(b,
		http.Header{"Content-Type": []string{"text/event-stream"}},
		strings.Join(events, "\n\n")+"\n\n",
		`{"model":"claude-sonnet-4-20250514","stream":true,"messages":[{"role":"user","content":"hi"}]}`,
		true,
	)
}
