//go:build unit

// Phase-2 TASK-003 钩子链等价性单测：
//   - moderationInputFromHookRequest 与 buildContentModerationInput（WS 路径仍用）逐字段等价；
//   - moderation 核心钩子的 gateway_check_start（11 字段）/ gateway_check_done（8 字段）
//     结构化日志契约；
//   - 空链 / nil 防御路径零分配（moderation 未装配时热路径无新增开销）。
package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ikik-api/internal/gatewayhook"
	"ikik-api/internal/pkg/ctxkey"
	middleware "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// preFlightTestGinContext 构造带请求上下文的 gin.Context。
// forcePlatform 非空时按 middleware.ForcePlatform 的真实行为同时写入
// request context（ctxkey.ForcePlatform）与 gin key。
func preFlightTestGinContext(t *testing.T, path string, forcePlatform string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{}`)))
	ctx := context.WithValue(req.Context(), ctxkey.RequestID, "req-pf-001")
	if forcePlatform != "" {
		ctx = context.WithValue(ctx, ctxkey.ForcePlatform, forcePlatform)
	}
	c.Request = req.WithContext(ctx)
	if forcePlatform != "" {
		c.Set(string(middleware.ContextKeyForcePlatform), forcePlatform)
	}
	return c
}

func preFlightTestAPIKey() *service.APIKey {
	groupID := int64(9001)
	return &service.APIKey{
		ID:      9101,
		UserID:  9201,
		Name:    "pf-key",
		GroupID: &groupID,
		User:    &service.User{ID: 9201, Email: "pf@example.com"},
		Group:   &service.Group{ID: groupID, Name: "pf-group", Platform: service.PlatformAnthropic},
	}
}

// TestPreFlightModerationInput_EquivalentToLegacyBuilder 锁定钩子侧审核输入
// 与 WS 路径仍在使用的 buildContentModerationInput 逐字段等价。
func TestPreFlightModerationInput_EquivalentToLegacyBuilder(t *testing.T) {
	subject := middleware.AuthSubject{UserID: 9201, Concurrency: 5}
	body := []byte(`{"model":"claude-sonnet-4-5"}`)

	cases := []struct {
		name          string
		forcePlatform string
		apiKey        *service.APIKey
		model         string
	}{
		{name: "常规请求", apiKey: preFlightTestAPIKey(), model: " claude-sonnet-4-5 "},
		{name: "强制平台覆盖provider", forcePlatform: "antigravity", apiKey: preFlightTestAPIKey(), model: "claude-sonnet-4-5"},
		{name: "nil apiKey防御", apiKey: nil, model: "gpt-5.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := preFlightTestGinContext(t, "/v1/messages", tc.forcePlatform)

			legacy := buildContentModerationInput(c, tc.apiKey, subject, service.ContentModerationProtocolAnthropicMessages, tc.model, body)
			req := newGatewayHookRequest(c, tc.apiKey, subject, service.ContentModerationProtocolAnthropicMessages, tc.model, body)
			hookInput := moderationInputFromHookRequest(c.Request.Context(), req)

			require.Equal(t, legacy, hookInput, "钩子侧审核输入必须与原 builder 逐字段等价")
		})
	}
}

// TestContentModerationPreFlightHook_CheckLogContract 锁定核心钩子的
// gateway_check_start / gateway_check_done 日志字段契约与拦截决策映射。
func TestContentModerationPreFlightHook_CheckLogContract(t *testing.T) {
	moderationSrv := p2CharFlagAllModerationServer(t)
	defer moderationSrv.Close()

	core, logs := observer.New(zap.InfoLevel)
	reqLog := zap.New(core)

	hook := &contentModerationPreFlightHook{svc: passCharModerationService(t, moderationSrv.URL)}
	c := preFlightTestGinContext(t, "/v1/messages", "")
	apiKey := preFlightTestAPIKey()
	subject := middleware.AuthSubject{UserID: 9201, Concurrency: 5}
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"bad words"}]}`)

	req := newGatewayHookRequest(c, apiKey, subject, service.ContentModerationProtocolAnthropicMessages, "claude-sonnet-4-5", body)
	ctx := withPreFlightLogger(c.Request.Context(), reqLog)

	decision, err := hook.CheckPreFlight(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, decision)
	require.True(t, decision.Blocked)
	require.Equal(t, "content_policy_violation", decision.ErrorType)
	require.NotEmpty(t, decision.Message)

	start := logs.FilterMessage("content_moderation.gateway_check_start").All()
	require.Len(t, start, 1, "必须记一条 gateway_check_start")
	startFields := start[0].ContextMap()
	require.Equal(t, []string{
		"request_id", "user_id", "api_key_id", "api_key_name", "group_id",
		"group_name", "endpoint", "provider", "protocol", "model", "body_bytes",
	}, preFlightLogFieldKeys(start[0]), "gateway_check_start 11 字段的名称与顺序必须保留")
	require.Equal(t, "req-pf-001", startFields["request_id"])
	require.Equal(t, int64(9201), startFields["user_id"])
	require.Equal(t, int64(9101), startFields["api_key_id"])
	require.Equal(t, "pf-key", startFields["api_key_name"])
	require.Equal(t, int64(9001), startFields["group_id"])
	require.Equal(t, "pf-group", startFields["group_name"])
	require.Equal(t, "/v1/messages", startFields["endpoint"])
	require.Equal(t, service.PlatformAnthropic, startFields["provider"])
	require.Equal(t, service.ContentModerationProtocolAnthropicMessages, startFields["protocol"])
	require.Equal(t, "claude-sonnet-4-5", startFields["model"])
	require.Equal(t, int64(len(body)), startFields["body_bytes"])

	done := logs.FilterMessage("content_moderation.gateway_check_done").All()
	require.Len(t, done, 1, "必须记一条 gateway_check_done")
	require.Equal(t, []string{
		"request_id", "allowed", "blocked", "flagged", "action",
		"status_code", "highest_category", "highest_score",
	}, preFlightLogFieldKeys(done[0]), "gateway_check_done 8 字段的名称与顺序必须保留")
	doneFields := done[0].ContextMap()
	require.Equal(t, true, doneFields["blocked"])
	require.Equal(t, "req-pf-001", doneFields["request_id"])
}

func preFlightLogFieldKeys(entry observer.LoggedEntry) []string {
	keys := make([]string, 0, len(entry.Context))
	for _, f := range entry.Context {
		keys = append(keys, f.Key)
	}
	return keys
}

// TestRunGatewayPreFlight_EmptyChainZeroAlloc 锁定审核未装配（空链/nil 链）时
// pre-flight 调用面零分配直接放行——热路径无新增开销。
func TestRunGatewayPreFlight_EmptyChainZeroAlloc(t *testing.T) {
	c := preFlightTestGinContext(t, "/v1/messages", "")
	apiKey := preFlightTestAPIKey()
	subject := middleware.AuthSubject{UserID: 9201}
	body := []byte(`{}`)
	reqLog := zap.NewNop()

	for name, chain := range map[string]*gatewayhook.Chain{
		"nil链": nil,
		"空链":   ProvideGatewayHookChain(nil),
	} {
		t.Run(name, func(t *testing.T) {
			require.Nil(t, runGatewayPreFlight(chain, c, reqLog, apiKey, subject, service.ContentModerationProtocolAnthropicMessages, "m", body))
			allocs := testing.AllocsPerRun(100, func() {
				_ = runGatewayPreFlight(chain, c, reqLog, apiKey, subject, service.ContentModerationProtocolAnthropicMessages, "m", body)
			})
			require.Zero(t, allocs, "空链路径必须零分配")
		})
	}
}

// TestContentModerationPreFlightHook_NilGuards 锁定 nil 防御：
// svc/req 为 nil 或 ctx 无请求级 logger 时不 panic 且放行。
func TestContentModerationPreFlightHook_NilGuards(t *testing.T) {
	var nilHook *contentModerationPreFlightHook
	d, err := nilHook.CheckPreFlight(context.Background(), &gatewayhook.Request{})
	require.NoError(t, err)
	require.Nil(t, d)

	hookNoSvc := &contentModerationPreFlightHook{}
	d, err = hookNoSvc.CheckPreFlight(context.Background(), &gatewayhook.Request{})
	require.NoError(t, err)
	require.Nil(t, d)

	moderationSrv := p2CharFlagAllModerationServer(t)
	defer moderationSrv.Close()
	hook := &contentModerationPreFlightHook{svc: passCharModerationService(t, moderationSrv.URL)}
	d, err = hook.CheckPreFlight(context.Background(), nil)
	require.NoError(t, err)
	require.Nil(t, d)
}
