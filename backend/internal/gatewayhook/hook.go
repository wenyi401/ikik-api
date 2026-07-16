// Package gatewayhook 定义网关 pre-flight 钩子链（Phase-2 SEAM-DESIGN 裁决 H）。
//
// 设计要点：
//   - 独立包：不依赖 internal/handler 与 internal/server/middleware（CallerInfo
//     即为与 middleware.AuthSubject 解耦的最小身份视图）；
//   - 链只产 Decision，错误格式化留在各调用点（5 种 HTTP 拦截格式差异由
//     特征化测试锁定，与协议常量无映射关系）；
//   - 核心钩子（如内容审核）由 Wire 装配；gateway.hook.* 命名空间的模块收集
//     机制留待 Phase-3 与平台 Provider 需求合并设计，本包不做 Runtime 收集。
package gatewayhook

import (
	"context"
	"net/http"

	"ikik-api/internal/service"
)

// CallerInfo 是调用方身份的最小视图（与 middleware.AuthSubject 解耦）。
type CallerInfo struct {
	// UserID 为认证后的用户 ID。
	UserID int64
	// GroupID 为 API Key 所属分组 ID（可能为 nil）。
	GroupID *int64
	// KeyID 为平台 API Key ID。
	KeyID int64
}

// RequestHeaders 是入站请求头的只读访问器（不暴露 gin.Context / http.Header 本体）。
type RequestHeaders struct {
	header http.Header
}

// NewRequestHeaders 包装 http.Header 为只读访问器；传 nil 得到空访问器。
func NewRequestHeaders(header http.Header) RequestHeaders {
	return RequestHeaders{header: header}
}

// Get 返回指定 key 的首个 header 值（语义同 http.Header.Get）。
func (r RequestHeaders) Get(key string) string {
	if r.header == nil {
		return ""
	}
	return r.header.Get(key)
}

// Values 返回指定 key 的全部 header 值（语义同 http.Header.Values）。
func (r RequestHeaders) Values(key string) []string {
	if r.header == nil {
		return nil
	}
	return r.header.Values(key)
}

// Request 是 pre-flight 钩子的协议无关只读入参。
type Request struct {
	// Protocol 复用 service.ContentModerationProtocol* 现有常量值
	// （如 "anthropic_messages" / "openai_chat_completions"），禁止新造枚举。
	Protocol string
	// Model 为请求的模型名（未做 TrimSpace，由钩子按需规范化）。
	Model string
	// Body 为送检请求体。各调用点保留自己的入参表达式
	// （如 images 调用点传 parsed.ModerationBody()），不强行统一。
	Body []byte
	// Caller 为调用方最小身份视图。
	Caller CallerInfo
	// APIKey 为完整平台 API Key 上下文（audit 类钩子需要；可能为 nil）。
	APIKey *service.APIKey
	// Headers 为入站请求头的只读访问器。
	Headers RequestHeaders
	// Path 为入站端点路径（调用侧按 inbound endpoint 归一化后传入）。
	Path string
}

// Decision 是钩子的协议无关决策；nil 表示放行。
// ErrorType 对 gemini 调用点无效（googleError 格式化器不消费该字段）。
type Decision struct {
	Blocked    bool
	StatusCode int
	ErrorType  string
	Message    string
}

// PreFlightHook 是网关转发前的拦截钩子。
//
// 契约（链层强制，见 chain.go）：
//   - 返回 (nil, nil) 表示放行；
//   - 返回 Blocked Decision 表示拦截（链短路返回）；
//   - 返回 error 表示钩子自身故障，链层按 fail-open 处理（记日志后继续下一钩子）；
//   - 钩子内 panic 同样被链层隔离并按 fail-open 降级。
type PreFlightHook interface {
	// HookID 返回钩子的稳定标识（用于日志与排障）。
	HookID() string
	// CheckPreFlight 对请求执行转发前检查。
	CheckPreFlight(ctx context.Context, req *Request) (*Decision, error)
}
