package service

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const (
	openCodeSessionHeader         = "X-OpenCode-Session"
	openCodeInboundBodyContextKey = "opencode_inbound_body"
)

// rememberOpenCodeInboundBody keeps the client request body so protocol
// conversion (Responses↔Anthropic↔Chat Completions) can still recover
// prompt_cache_key / metadata.user_id after those fields are dropped.
func rememberOpenCodeInboundBody(c *gin.Context, body []byte) {
	if c == nil || len(body) == 0 {
		return
	}
	c.Set(openCodeInboundBodyContextKey, body)
}

func openCodeInboundBodies(c *gin.Context) [][]byte {
	if c == nil {
		return nil
	}
	raw, ok := c.Get(openCodeInboundBodyContextKey)
	if !ok {
		return nil
	}
	body, ok := raw.([]byte)
	if !ok || len(body) == 0 {
		return nil
	}
	return [][]byte{body}
}

// applyOpenCodeSessionHeader sets x-opencode-session on outbound inference
// requests. OpenCode Go requires a per-conversation value from 2026-09-05
// (MissingSessionID). Prefer caller headers, then the documented body session
// fields (OpenAI prompt_cache_key / Anthropic metadata.user_id), then any
// already applied account override. A generated UUID is last-resort only for
// probes and clients that omit every stable identifier — a new UUID each turn
// would miss upstream prompt cache.
func applyOpenCodeSessionHeader(c *gin.Context, account *Account, targetURL string, headers http.Header, bodies ...[]byte) {
	if account == nil || account.Type != AccountTypeAPIKey || headers == nil {
		return
	}
	if !shouldSendOpenCodeSessionHeader(account, targetURL) {
		return
	}

	payloads := append(openCodeInboundBodies(c), bodies...)
	sessionID := resolveOpenCodeSessionID(c, headers, shouldGenerateOpenCodeSession(account, targetURL), payloads...)
	if sessionID == "" {
		return
	}
	for key := range headers {
		if strings.EqualFold(key, openCodeSessionHeader) {
			delete(headers, key)
		}
	}
	headers.Set(openCodeSessionHeader, sessionID)
}

func shouldSendOpenCodeSessionHeader(account *Account, targetURL string) bool {
	if account != nil && account.IsOpenCodeGoPlan() {
		return true
	}
	return isOfficialOpenCodeHost(targetURL)
}

func shouldGenerateOpenCodeSession(account *Account, targetURL string) bool {
	if account != nil && account.IsOpenCodeGoPlan() {
		return true
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "https") &&
		strings.EqualFold(parsed.Hostname(), "opencode.ai") &&
		strings.Contains(parsed.Path, "/zen/go")
}

func isOfficialOpenCodeHost(targetURL string) bool {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "https") && strings.EqualFold(parsed.Hostname(), "opencode.ai")
}

func resolveOpenCodeSessionID(c *gin.Context, headers http.Header, generate bool, bodies ...[]byte) string {
	if c != nil && c.Request != nil {
		if sessionID := sanitizeSessionID(c.GetHeader(openCodeSessionHeader)); sessionID != "" {
			return sessionID
		}
		if sessionID := sanitizeSessionID(explicitOpenAIHeaderSessionID(c)); sessionID != "" {
			return sessionID
		}
		if sessionID := sanitizeSessionID(ClaudeCodeSessionIDFromHeader(c)); sessionID != "" {
			return sessionID
		}
	}
	for _, body := range bodies {
		if sessionID := sanitizeSessionID(openCodeSessionIDFromPayload(body)); sessionID != "" {
			return sessionID
		}
	}
	if sessionID := sanitizeSessionID(existingOpenCodeSessionHeader(headers)); sessionID != "" {
		return sessionID
	}
	if generate {
		// 客户端未提供任何会话标识时，优先按「API Key + 对话首条用户消息」派生
		// 稳定会话 ID：编码类 agent 的历史逐轮增长但首条消息不变，同一对话因此
		// 粘住同一会话，上游的会话亲和缓存才能命中；不同对话/不同 Key 天然隔离。
		// 派生失败（空 body、无法定位首条用户消息）才回落到一次性 UUID。
		if derived := openCodeDerivedStableSessionID(c, bodies...); derived != "" {
			return derived
		}
		return uuid.NewString()
	}
	return ""
}

// openCodeDerivedStableSessionID 从请求体提取首条用户消息文本，并与调用方的
// API Key ID 一起哈希成稳定会话 ID。返回空表示无法派生（调用方应回落 UUID）。
func openCodeDerivedStableSessionID(c *gin.Context, bodies ...[]byte) string {
	var apiKeySalt string
	if c != nil {
		if value, exists := c.Get("api_key"); exists {
			if apiKey, ok := value.(*APIKey); ok && apiKey != nil {
				apiKeySalt = strconv.FormatInt(apiKey.ID, 10)
			}
		}
	}
	for _, body := range bodies {
		first := openCodeFirstUserMessageText(body)
		if first == "" {
			continue
		}
		sum := sha256.Sum256([]byte(apiKeySalt + "\x00" + first))
		hex := fmt.Sprintf("%x", sum)
		// 整形成 UUID 形态，与官方会话 ID 的外观一致；熵取前 128 位足够。
		return hex[0:8] + "-" + hex[8:12] + "-4" + hex[13:16] + "-8" + hex[17:20] + "-" + hex[20:32]
	}
	return ""
}

// openCodeFirstUserMessageText 返回请求体里第一条用户消息的文本（Chat
// Completions 的 messages[role=user] 与 Responses 的 input 两种形状都认）。
// 编码 agent 多轮对话的首条消息保持不变，是理想的会话指纹。
func openCodeFirstUserMessageText(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	view := gjson.Get(string(body), "messages")
	if view.IsArray() {
		for _, item := range view.Array() {
			if item.Get("role").String() != "user" {
				continue
			}
			if text := openCodeMessageItemText(item); text != "" {
				return text
			}
			return ""
		}
		return ""
	}
	input := gjson.Get(string(body), "input")
	if input.IsArray() {
		for _, item := range input.Array() {
			if item.Get("type").String() != "" && item.Get("type").String() != "message" {
				continue
			}
			if item.Get("role").String() != "user" {
				continue
			}
			if text := openCodeMessageItemText(item); text != "" {
				return text
			}
			return ""
		}
	}
	return ""
}

func openCodeMessageItemText(item gjson.Result) string {
	if content := item.Get("content"); content.Exists() {
		if content.IsArray() {
			var builder strings.Builder
			for _, part := range content.Array() {
				if part.Get("type").String() == "text" || !part.Get("type").Exists() {
					if text := strings.TrimSpace(part.Get("text").String()); text != "" {
						builder.WriteString(text)
					}
				}
			}
			if builder.Len() > 0 {
				return builder.String()
			}
			return ""
		}
		return strings.TrimSpace(content.String())
	}
	return strings.TrimSpace(item.Get("text").String())
}

// openCodeSessionIDFromPayload reads the stable conversation id from documented
// request-body fields. Kimi Code / OpenAI clients send prompt_cache_key on
// Chat Completions and Responses; Anthropic clients send metadata.user_id on
// Messages. Neither field changes model behavior.
func openCodeSessionIDFromPayload(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	view := openAIRequestPayloadView(body)
	if sessionID := strings.TrimSpace(view.Get("prompt_cache_key").String()); sessionID != "" {
		return sessionID
	}
	return openCodeSessionIDFromMetadataUserID(view.Get("metadata.user_id").String())
}

func openCodeSessionIDFromMetadataUserID(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}
	if strings.HasPrefix(userID, "{") {
		if sessionID := strings.TrimSpace(gjson.Get(userID, "session_id").String()); sessionID != "" {
			return sessionID
		}
	}
	return userID
}

func openCodeSessionHintBody(promptCacheKey string) []byte {
	key := strings.TrimSpace(promptCacheKey)
	if key == "" {
		return nil
	}
	return []byte(`{"prompt_cache_key":` + strconv.Quote(key) + `}`)
}

func existingOpenCodeSessionHeader(headers http.Header) string {
	if headers == nil {
		return ""
	}
	for key, values := range headers {
		if strings.EqualFold(key, openCodeSessionHeader) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
