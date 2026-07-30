package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"ikik-api/internal/gatewayhook"
	"ikik-api/internal/pkg/ctxkey"
	openaipkg "ikik-api/internal/pkg/openai"
	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

const (
	legacyModerationCheckedContextKey  = "ikik.content_moderation.legacy_checked"
	legacyModerationDecisionContextKey = "ikik.content_moderation.legacy_decision"
)

// ProvideGatewayHookChain 装配网关 pre-flight 钩子链（Wire 注入的核心钩子；
// 裁决 H-C：只建链，不做 gateway.hook.* 命名空间的 Runtime 收集——该机制留待
// Phase-3 与平台 Provider 真实需求合并设计）。当前链 = [content_moderation]。
func ProvideGatewayHookChain(contentModerationService *service.ContentModerationService) *gatewayhook.Chain {
	if contentModerationService == nil {
		return gatewayhook.NewChain(nil)
	}
	return gatewayhook.NewChain(nil, &contentModerationPreFlightHook{svc: contentModerationService})
}

// preFlightLoggerKey 在钩子执行期间通过 ctx 传递请求级 logger（reqLog），
// 保留其已累积的 component/model/stream 等字段，保证审核日志字段等价。
type preFlightLoggerKey struct{}

func withPreFlightLogger(ctx context.Context, log *zap.Logger) context.Context {
	if log == nil {
		return ctx
	}
	return context.WithValue(ctx, preFlightLoggerKey{}, log)
}

func preFlightLoggerFromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return nil
	}
	log, _ := ctx.Value(preFlightLoggerKey{}).(*zap.Logger)
	return log
}

// newGatewayHookRequest 从 gin 上下文构造钩子请求（协议无关只读视图）。
// Path 的计算与原 buildContentModerationInput 的 Endpoint 等价：
// GetInboundEndpoint 优先，空值回退原始 URL path。
func newGatewayHookRequest(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) *gatewayhook.Request {
	caller := gatewayhook.CallerInfo{UserID: subject.UserID}
	if apiKey != nil {
		caller.KeyID = apiKey.ID
		caller.GroupID = apiKey.GroupID
	}
	path := GetInboundEndpoint(c)
	if path == "" && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	return &gatewayhook.Request{
		Protocol: protocol,
		Model:    model,
		Body:     body,
		Caller:   caller,
		APIKey:   apiKey,
		Headers:  gatewayhook.NewRequestHeaders(c.Request.Header),
		Path:     path,
	}
}

// runGatewayPreFlight 构造钩子请求并执行 pre-flight 链。
// 等价性约束（对应原 runContentModeration 的调用面）：
//   - 链为空（审核服务未装配/测试夹具）时直接放行，且不构造任何对象（零分配）；
//   - c / c.Request 为 nil 时直接放行（与原 helper 的 nil 防御一致）。
func runGatewayPreFlight(chain *gatewayhook.Chain, c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) *gatewayhook.Decision {
	if chain.IsEmpty() || c == nil || c.Request == nil {
		return nil
	}
	req := newGatewayHookRequest(c, apiKey, subject, protocol, model, body)
	decision := chain.Run(withPreFlightLogger(c.Request.Context(), reqLog), req)
	// The production chain contains legacy content moderation. Mark completion
	// even when that hook fails open so Prompt Audit does not invoke it again.
	c.Set(legacyModerationCheckedContextKey, true)
	if decision != nil {
		c.Set(legacyModerationDecisionContextKey, decision)
	}
	return decision
}

// runPreFlightHooks 执行网关 pre-flight 钩子链（替代原 checkContentModeration 的 HTTP 调用面）。
func (h *GatewayHandler) runPreFlightHooks(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) *gatewayhook.Decision {
	if h == nil {
		return nil
	}
	return runGatewayPreFlight(h.preFlightHooks, c, reqLog, apiKey, subject, protocol, model, body)
}

// runPreFlightHooks 执行网关 pre-flight 钩子链（HTTP 调用点专用；
// WebSocket 两调用点仍走 checkContentModeration，按裁决排除在链改造之外）。
func (h *OpenAIGatewayHandler) runPreFlightHooks(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) *gatewayhook.Decision {
	if h == nil {
		return nil
	}
	return runGatewayPreFlight(h.preFlightHooks, c, reqLog, apiKey, subject, protocol, model, body)
}

// preFlightStatus 与原 contentModerationStatus 等价：非 4xx/5xx 状态码钳制为 403。
func preFlightStatus(decision *gatewayhook.Decision) int {
	if decision == nil || decision.StatusCode < 400 || decision.StatusCode > 599 {
		return http.StatusForbidden
	}
	return decision.StatusCode
}

// preFlightErrorCode 与原 contentModerationErrorCode 等价：
// 拦截错误码固定回退 content_policy_violation（gemini 调用点不消费该值）。
func preFlightErrorCode(decision *gatewayhook.Decision) string {
	if decision == nil || strings.TrimSpace(decision.ErrorType) == "" {
		return "content_policy_violation"
	}
	return decision.ErrorType
}

// contentModerationPreFlightHook 是内容审核的核心钩子 adapter
// （裁决 H：moderation 本体依赖面超出 Host 能力，不模块化，仅包装现有 Check）。
//
// 等价性硬约束（与原 runContentModeration 逐项对应，WS 路径仍直接使用后者）：
//   - gateway_check_start（11 字段）/ gateway_check_done（8 字段）结构化日志
//     的事件名、字段名、字段来源逐一保留，且经请求级 logger（reqLog）输出；
//   - Check 返回 error 时记 content_moderation.check_failed 后放行（fail-open，
//     由本钩子自行吞错，不依赖链层的通用 error 降级，保持原日志事件名）；
//   - svc 为 nil 时直接放行。
type contentModerationPreFlightHook struct {
	svc *service.ContentModerationService
}

func (h *contentModerationPreFlightHook) HookID() string { return "content_moderation" }

func (h *contentModerationPreFlightHook) CheckPreFlight(ctx context.Context, req *gatewayhook.Request) (*gatewayhook.Decision, error) {
	if h == nil || h.svc == nil || req == nil {
		return nil, nil
	}
	reqLog := preFlightLoggerFromContext(ctx)
	input := moderationInputFromHookRequest(ctx, req)
	if reqLog != nil {
		reqLog.Info("content_moderation.gateway_check_start",
			zap.String("request_id", input.RequestID),
			zap.Int64("user_id", input.UserID),
			zap.Int64("api_key_id", input.APIKeyID),
			zap.String("api_key_name", input.APIKeyName),
			zap.Int64p("group_id", input.GroupID),
			zap.String("group_name", input.GroupName),
			zap.String("endpoint", input.Endpoint),
			zap.String("provider", input.Provider),
			zap.String("protocol", input.Protocol),
			zap.String("model", input.Model),
			zap.Int("body_bytes", len(req.Body)),
		)
	}
	decision, err := h.svc.Check(ctx, input)
	if err != nil {
		if reqLog != nil {
			reqLog.Warn("content_moderation.check_failed", zap.Error(err))
		}
		return nil, nil
	}
	if reqLog != nil && decision != nil {
		reqLog.Info("content_moderation.gateway_check_done",
			zap.String("request_id", input.RequestID),
			zap.Bool("allowed", decision.Allowed),
			zap.Bool("blocked", decision.Blocked),
			zap.Bool("flagged", decision.Flagged),
			zap.String("action", decision.Action),
			zap.Int("status_code", decision.StatusCode),
			zap.String("highest_category", decision.HighestCategory),
			zap.Float64("highest_score", decision.HighestScore),
		)
	}
	if decision == nil || !decision.Blocked {
		return nil, nil
	}
	return &gatewayhook.Decision{
		Blocked:    true,
		StatusCode: decision.StatusCode,
		ErrorType:  "content_policy_violation",
		Message:    decision.Message,
	}, nil
}

// moderationInputFromHookRequest 从钩子请求构造审核输入，与 buildContentModerationInput
// （WS 路径仍在使用）逐字段等价：
//   - RequestID 来自 ctx（调用侧传 c.Request.Context()）；
//   - Endpoint 即 req.Path（调用侧已按 GetInboundEndpoint + URL.Path 兜底预计算）；
//   - 强制平台覆盖改读 ctxkey.ForcePlatform（middleware.ForcePlatform 同步写入
//     gin key 与 request context，两者在审核执行点恒一致）。
func moderationInputFromHookRequest(ctx context.Context, req *gatewayhook.Request) service.ContentModerationCheckInput {
	input := service.ContentModerationCheckInput{
		RequestID: contentModerationRequestID(ctx),
		UserID:    req.Caller.UserID,
		Endpoint:  req.Path,
		Provider:  contentModerationProvider(req.APIKey),
		Model:     strings.TrimSpace(req.Model),
		Protocol:  req.Protocol,
		Body:      req.Body,
		CodexOfficialClient: openaipkg.IsCodexOfficialClientByHeaders(
			req.Headers.Get("User-Agent"),
			req.Headers.Get("originator"),
		),
		InternalSignature: strings.TrimSpace(req.Headers.Get(service.ContentModerationInternalSignatureHeader)),
	}
	if forcedPlatform, ok := ctx.Value(ctxkey.ForcePlatform).(string); ok {
		input.Provider = strings.TrimSpace(forcedPlatform)
	}
	if req.APIKey != nil {
		input.APIKeyID = req.APIKey.ID
		input.APIKeyName = req.APIKey.Name
		if req.APIKey.User != nil {
			input.UserEmail = req.APIKey.User.Email
		}
		if req.APIKey.GroupID != nil {
			groupID := *req.APIKey.GroupID
			input.GroupID = &groupID
		}
		if req.APIKey.Group != nil {
			input.GroupName = req.APIKey.Group.Name
		}
	}
	return input
}
