package gatewayplatform

// 本文件的三个 adapter 仅包装现有 service 的 Forward 入口（实现零移动）。
//
// 错误透传铁律（SEAM-DESIGN 裁决 / T4 特征化锁定）：底层返回的 error 一律
// 原样 return，禁止任何包裹——handler 对 *service.BetaBlockedError /
// *service.PromptTooLongError / *service.UpstreamFailoverError 的 errors.As
// 断言链依赖错误原值。

import (
	"context"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

// geminiForwardAction 是 /v1/messages gemini 分支传给 ForwardGemini 的 action
// 常量（SEAM-DESIGN 裁决：硬编码 "generateContent" + 测试锁定；该参数仅决定
// 客户端响应形态合法性，上游恒为 streamGenerateContent）。
const geminiForwardAction = "generateContent"

// AnthropicProvider 包装 GatewayService.Forward（Claude 协议主路径；
// :794 条件不命中侧，含 antigravity 平台的 APIKey 账号）。
type AnthropicProvider struct {
	gateway *service.GatewayService
}

// NewAnthropicProvider 构造 anthropic 平台 Provider。
func NewAnthropicProvider(gateway *service.GatewayService) *AnthropicProvider {
	return &AnthropicProvider{gateway: gateway}
}

// Platform 返回 service.PlatformAnthropic。
func (p *AnthropicProvider) Platform() string { return service.PlatformAnthropic }

// Forward 转发到 GatewayService.Forward；error 原样透传。
func (p *AnthropicProvider) Forward(ctx context.Context, c *gin.Context, account *service.Account, req *ForwardRequest) (*service.ForwardResult, error) {
	return p.gateway.Forward(ctx, c, account, req.Parsed)
}

// AntigravityProvider 包装 AntigravityGatewayService.Forward（Claude 协议 →
// antigravity 上游；:794 条件命中侧：Platform == Antigravity && Type != APIKey）。
type AntigravityProvider struct {
	antigravity *service.AntigravityGatewayService
}

// NewAntigravityProvider 构造 antigravity 平台 Provider。
func NewAntigravityProvider(antigravity *service.AntigravityGatewayService) *AntigravityProvider {
	return &AntigravityProvider{antigravity: antigravity}
}

// Platform 返回 service.PlatformAntigravity。
func (p *AntigravityProvider) Platform() string { return service.PlatformAntigravity }

// Forward 转发到 AntigravityGatewayService.Forward；error 原样透传。
func (p *AntigravityProvider) Forward(ctx context.Context, c *gin.Context, account *service.Account, req *ForwardRequest) (*service.ForwardResult, error) {
	return p.antigravity.Forward(ctx, c, account, req.Body, req.IsStickySession)
}

// GeminiProvider 包装 AntigravityGatewayService.ForwardGemini（gemini 平台
// 分组下的 antigravity 账号，:444 命中侧；gemini 平台的非 antigravity 账号
// 走 GeminiMessagesCompatService，映射逻辑保留在调用点）。
type GeminiProvider struct {
	antigravity *service.AntigravityGatewayService
}

// NewGeminiProvider 构造 gemini 平台 Provider。
func NewGeminiProvider(antigravity *service.AntigravityGatewayService) *GeminiProvider {
	return &GeminiProvider{antigravity: antigravity}
}

// Platform 返回 service.PlatformGemini。
func (p *GeminiProvider) Platform() string { return service.PlatformGemini }

// Forward 转发到 AntigravityGatewayService.ForwardGemini；error 原样透传。
// model/stream 取自 req.Parsed（与原调用点 reqModel/reqStream 同源）。
func (p *GeminiProvider) Forward(ctx context.Context, c *gin.Context, account *service.Account, req *ForwardRequest) (*service.ForwardResult, error) {
	return p.antigravity.ForwardGemini(
		ctx,
		c,
		account,
		req.Parsed.Model,
		geminiForwardAction,
		req.Parsed.Stream,
		req.Body,
		req.IsStickySession,
		service.WithForwardGeminiSession(req.SessionGroupID, req.SessionKey),
	)
}
