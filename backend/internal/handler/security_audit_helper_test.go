package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"ikik-api/internal/gatewayhook"
	"ikik-api/internal/securityaudit"
	middleware2 "ikik-api/internal/server/middleware"
)

type auditPreFlightHook struct {
	decision *gatewayhook.Decision
	err      error
	calls    atomic.Int64
}

func (h *auditPreFlightHook) HookID() string { return "content_moderation" }
func (h *auditPreFlightHook) CheckPreFlight(context.Context, *gatewayhook.Request) (*gatewayhook.Decision, error) {
	h.calls.Add(1)
	return h.decision, h.err
}

type auditLegacyEngine struct {
	calls atomic.Int64
}

func (e *auditLegacyEngine) Check(context.Context, securityaudit.Request) (*securityaudit.LegacyDecision, error) {
	e.calls.Add(1)
	return &securityaudit.LegacyDecision{Allowed: true}, nil
}

func TestCachesSecurityAuditCompletionSkipsWebSocketStages(t *testing.T) {
	require.True(t, cachesSecurityAuditCompletion("http"))
	require.True(t, cachesSecurityAuditCompletion(""))
	require.False(t, cachesSecurityAuditCompletion("first_turn"))
	require.False(t, cachesSecurityAuditCompletion("subsequent_turn"))
}

func TestRunSecurityAuditDoesNotSkipSubsequentWebSocketTurns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := &turnCountingEngine{mode: securityaudit.ModeAsync}
	coordinator := securityaudit.NewCoordinator(nil, engine)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	subject := middleware2.AuthSubject{UserID: 7, Concurrency: 1}
	first := runSecurityAudit(c, nil, coordinator, nil, nil, subject, "openai_responses", "gpt-test",
		[]byte(`{"type":"response.create","response":{"input":"benign"}}`), "first_turn")
	require.NotNil(t, first)
	require.True(t, first.AllowNextStage)
	require.Equal(t, int64(1), engine.enqueues.Load())
	_, cached := c.Get(securityAuditCompletedContextKey)
	require.False(t, cached, "WebSocket stages must not set the HTTP completion cache")

	// Even if an HTTP path previously cached completion on this Context, WS turns
	// must still audit every response.create payload.
	c.Set(securityAuditCompletedContextKey, true)

	second := runSecurityAudit(c, nil, coordinator, nil, nil, subject, "openai_responses", "gpt-test",
		[]byte(`{"type":"response.create","response":{"input":"malicious follow-up"}}`), "subsequent_turn")
	require.NotNil(t, second)
	require.Equal(t, int64(2), engine.enqueues.Load(), "subsequent WebSocket turns must be audited again")
}

func TestRunSecurityAuditDoesNotRepeatPreFlightLegacyModeration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name    string
		hookErr error
	}{
		{name: "allow"},
		{name: "fail open", hookErr: errors.New("moderation unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			hook := &auditPreFlightHook{err: tt.hookErr}
			chain := gatewayhook.NewChain(nil, hook)
			legacy := &auditLegacyEngine{}
			coordinator := securityaudit.NewCoordinator(legacy, nil)
			subject := middleware2.AuthSubject{UserID: 7, Concurrency: 1}

			require.Nil(t, runGatewayPreFlight(chain, c, nil, nil, subject, "anthropic_messages", "claude-test", []byte(`{"messages":[]}`)))
			decision := runSecurityAudit(c, nil, coordinator, nil, nil, subject, "anthropic_messages", "claude-test", []byte(`{"messages":[]}`), "http")

			require.NotNil(t, decision)
			require.True(t, decision.AllowNextStage)
			require.Equal(t, int64(1), hook.calls.Load())
			require.Zero(t, legacy.calls.Load())
		})
	}
}

func TestRunSecurityAuditStillRunsPromptEngineAfterPreFlightBlock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name            string
		mode            securityaudit.Mode
		wantEnqueues    int64
		wantEvaluations int64
	}{
		{name: "async enqueue", mode: securityaudit.ModeAsync, wantEnqueues: 1},
		{name: "blocking evaluate", mode: securityaudit.ModeBlocking, wantEvaluations: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			hook := &auditPreFlightHook{decision: &gatewayhook.Decision{
				Blocked: true, StatusCode: http.StatusUnavailableForLegalReasons,
				ErrorType: "content_policy_violation", Message: "legacy block wins",
			}}
			chain := gatewayhook.NewChain(nil, hook)
			legacy := &auditLegacyEngine{}
			prompt := &turnCountingEngine{mode: tt.mode}
			coordinator := securityaudit.NewCoordinator(legacy, prompt)
			subject := middleware2.AuthSubject{UserID: 7, Concurrency: 1}
			body := []byte(`{"messages":[{"role":"user","content":"blocked"}]}`)

			preFlight := runGatewayPreFlight(chain, c, nil, nil, subject, "anthropic_messages", "claude-test", body)
			require.NotNil(t, preFlight)
			require.True(t, preFlight.Blocked)
			decision := runSecurityAudit(c, nil, coordinator, nil, nil, subject, "anthropic_messages", "claude-test", body, "http")

			require.NotNil(t, decision)
			require.False(t, decision.AllowNextStage)
			require.Equal(t, securityaudit.DecisionBlock, decision.Kind)
			require.Equal(t, http.StatusUnavailableForLegalReasons, decision.HTTPStatus)
			require.Equal(t, "content_policy_violation", decision.ErrorCode)
			require.Equal(t, "legacy block wins", decision.ClientMessage)
			require.NotNil(t, decision.Legacy)
			require.Zero(t, legacy.calls.Load(), "Coordinator must reuse the pre-flight legacy result")
			require.Equal(t, tt.wantEnqueues, prompt.enqueues.Load())
			require.Equal(t, tt.wantEvaluations, prompt.evaluates.Load())
		})
	}
}

type turnCountingEngine struct {
	mode      securityaudit.Mode
	enqueues  atomic.Int64
	evaluates atomic.Int64
}

func (e *turnCountingEngine) EffectiveMode() securityaudit.Mode { return e.mode }
func (e *turnCountingEngine) Enqueue(context.Context, securityaudit.Request) error {
	e.enqueues.Add(1)
	return nil
}
func (e *turnCountingEngine) Evaluate(context.Context, securityaudit.Request) (*securityaudit.PromptDecision, error) {
	e.evaluates.Add(1)
	return &securityaudit.PromptDecision{Kind: securityaudit.DecisionAllow, AllowNextStage: true}, nil
}
