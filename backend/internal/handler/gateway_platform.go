package handler

import (
	"ikik-api/internal/gatewayplatform"
	"ikik-api/internal/service"
)

// ProvideGatewayPlatformRegistry 装配 /v1/messages 平台 Forward 分发注册表
// （Phase-3 SEAM-DESIGN 裁决：anthropic / antigravity / gemini 三 adapter；
// OpenAI 走独立 OpenAIGatewayHandler 与独立计费管线，不进本注册表）。
func ProvideGatewayPlatformRegistry(
	gatewayService *service.GatewayService,
	antigravityGatewayService *service.AntigravityGatewayService,
) *gatewayplatform.Registry {
	return gatewayplatform.NewRegistry(
		gatewayplatform.NewAnthropicProvider(gatewayService),
		gatewayplatform.NewAntigravityProvider(antigravityGatewayService),
		gatewayplatform.NewGeminiProvider(antigravityGatewayService),
	)
}
