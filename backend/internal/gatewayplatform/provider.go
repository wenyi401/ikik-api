// Package gatewayplatform 定义 /v1/messages 平台 Forward 分发的 Provider 接缝
// （Phase-3 SEAM-DESIGN v2 裁决记录）。
//
// 设计要点：
//   - Provider v1 接口仅 Platform/Forward 两方法（DefaultModels / FallbackModel /
//     InvalidateToken 等经评审裁决不进接口）；
//   - adapter 实现零移动：仅包装现有 service 的 Forward 入口；底层 error 一律
//     原样透传（handler 对 BetaBlockedError / PromptTooLongError 等的
//     errors.As 断言链依赖错误原值，禁止任何包裹）；
//   - Registry 构造期注册、运行期并发只读；平台 → Provider 的映射逻辑
//     （如 :794 的 Type != APIKey 条件）保留在调用点，registry 只做查找；
//   - OpenAI 走独立 OpenAIGatewayHandler 与独立计费管线，不进本注册表；
//     禁止统一跨平台返回类型（OpenAIForwardResult 载荷计费字段会丢失）。
package gatewayplatform

import (
	"context"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

// ForwardRequest 是平台 Forward 的统一入参面。字段集为两处分发点现有入参的
// 并集 {Parsed, Body, IsStickySession, SessionGroupID, SessionKey}，不为未接入
// 平台虚构维度；各调用点保留自己的入参表达式，按各 Provider 的消费面填充。
type ForwardRequest struct {
	// Parsed 为已解析的网关请求（anthropic / gemini Provider 消费；
	// gemini Provider 读取 Parsed.Model 与 Parsed.Stream）。
	Parsed *service.ParsedRequest
	// Body 为转发请求体原文（antigravity / gemini Provider 消费）。
	Body []byte
	// IsStickySession 表示请求已命中粘性会话绑定（antigravity / gemini
	// Provider 消费）。
	IsStickySession bool
	// SessionGroupID 与 SessionKey 为粘性会话维度（仅 gemini Provider 消费，
	// 经 WithForwardGeminiSession 透传到模型限流切换时的粘性绑定清除）。
	SessionGroupID int64
	SessionKey     string
}

// Provider 是单一平台的 Forward 入口。
//
// 契约：
//   - Platform 返回 service.Platform* 现有常量，作为 Registry 的查找键；
//   - Forward 将请求转发到对应平台 service，error 必须原样透传（禁止包裹）。
type Provider interface {
	// Platform 返回该 Provider 服务的平台标识（service.Platform* 常量）。
	Platform() string
	// Forward 执行平台转发。
	Forward(ctx context.Context, c *gin.Context, account *service.Account, req *ForwardRequest) (*service.ForwardResult, error)
}
