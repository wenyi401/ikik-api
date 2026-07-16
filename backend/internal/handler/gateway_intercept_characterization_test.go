//go:build unit

// Phase-0 TASK-002 特征化测试：网关拦截链路（INVARIANTS I-7.1 / I-7.2）。
// 通过完整的 GatewayHandler.Messages / CountTokens 入口驱动，断言客户端最终
// 收到的状态码与错误体：
//   - I-7.1 内容审核拦截：403 + content_policy_violation 错误体；审核服务失败 = 放行（fail-open）；
//   - I-7.2 Claude Code 版本检查：低版本 400 + 升级提示；/count_tokens 路径豁免。
//
// 复用 gateway_handler_warmup_intercept_unit_test.go 的 newTestGatewayHandler 夹具：
// antigravity 账号 intercept_warmup_requests=true 时，Warmup 请求在转发上游前被
// mock 拦截返回 200，使"请求被放行"成为可观测结果（无需真实上游）。
//
// 注意：Claude Code 版本上下限走 service 包内全局 60s TTL 缓存
// （setting_service.go versionBoundsCache）。本文件所有版本测试固定使用同一组
// 上下限（min=1.5.0，无 max），避免同进程内缓存交叉污染。
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ikik-api/internal/config"
	"ikik-api/internal/pkg/ctxkey"
	"ikik-api/internal/pkg/pagination"
	middleware "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// passCharSettingRepo 是 service.SettingRepository 的内存实现。
type passCharSettingRepo struct {
	values map[string]string
}

func (r *passCharSettingRepo) Get(_ context.Context, key string) (*service.Setting, error) {
	if v, ok := r.values[key]; ok {
		return &service.Setting{Key: key, Value: v}, nil
	}
	return nil, service.ErrSettingNotFound
}

func (r *passCharSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}

func (r *passCharSettingRepo) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *passCharSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (r *passCharSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for k, v := range settings {
		_ = r.Set(context.Background(), k, v)
	}
	return nil
}

func (r *passCharSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for k, v := range r.values {
		out[k] = v
	}
	return out, nil
}

func (r *passCharSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

// passCharModerationRepo 是 service.ContentModerationRepository 的空操作实现。
type passCharModerationRepo struct{}

func (r *passCharModerationRepo) CreateLog(context.Context, *service.ContentModerationLog) error {
	return nil
}

func (r *passCharModerationRepo) ListLogs(context.Context, service.ContentModerationLogFilter) ([]service.ContentModerationLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *passCharModerationRepo) CountFlaggedByUserSince(context.Context, int64, time.Time) (int, error) {
	return 0, nil
}

func (r *passCharModerationRepo) CleanupExpiredLogs(context.Context, time.Time, time.Time) (*service.ContentModerationCleanupResult, error) {
	return &service.ContentModerationCleanupResult{}, nil
}

// passCharInterceptFixture 构造完整的 Messages 链路夹具（拦截预热账号 + 上下文注入）。
func passCharInterceptFixture(t *testing.T) (*GatewayHandler, func()) {
	t.Helper()

	groupID := int64(7001)
	accountID := int64(7101)

	group := &service.Group{
		ID:       groupID,
		Hydrated: true,
		Platform: service.PlatformAnthropic,
		Status:   service.StatusActive,
	}
	account := &service.Account{
		ID:       accountID,
		Name:     "pass-char-intercept",
		Platform: service.PlatformAntigravity,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":              "tok_char",
			"intercept_warmup_requests": true,
		},
		Extra: map[string]any{
			"mixed_scheduling": true,
		},
		Concurrency:   1,
		Priority:      1,
		Status:        service.StatusActive,
		Schedulable:   true,
		AccountGroups: []service.AccountGroup{{AccountID: accountID, GroupID: groupID}},
	}
	return newTestGatewayHandler(t, group, []*service.Account{account})
}

// passCharNewMessagesContext 构造带认证上下文的 /v1/messages 请求。
func passCharNewMessagesContext(t *testing.T, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	groupID := int64(7001)
	group := &service.Group{
		ID:       groupID,
		Hydrated: true,
		Platform: service.PlatformAnthropic,
		Status:   service.StatusActive,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.Group, group))
	c.Request = req

	apiKey := &service.APIKey{
		ID:      7301,
		UserID:  7401,
		GroupID: &groupID,
		Status:  service.StatusActive,
		User: &service.User{
			ID:          7401,
			Concurrency: 10,
			Balance:     100,
		},
		Group: group,
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
	return c, rec
}

// passCharWarmupBody 返回触发预热拦截的最小请求体。
func passCharWarmupBody() []byte {
	return []byte(`{
		"model": "claude-sonnet-4-5",
		"max_tokens": 256,
		"messages": [{"role":"user","content":[{"type":"text","text":"Warmup"}]}]
	}`)
}

// passCharClaudeCodeWarmupBody 返回能通过 Claude Code 客户端校验且触发预热拦截的请求体。
func passCharClaudeCodeWarmupBody() []byte {
	deviceID := strings.Repeat("a", 64)
	return []byte(`{
		"model": "claude-sonnet-4-5",
		"max_tokens": 256,
		"system": [{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude."}],
		"metadata": {"user_id": "user_` + deviceID + `_account__session_01234567-89ab-cdef-0123-456789abcdef"},
		"messages": [{"role":"user","content":[{"type":"text","text":"Warmup"}]}]
	}`)
}

// passCharSetClaudeCodeHeaders 设置 Claude Code 客户端校验所需的请求头。
func passCharSetClaudeCodeHeaders(c *gin.Context, ua string) {
	c.Request.Header.Set("User-Agent", ua)
	c.Request.Header.Set("X-App", "cli")
	c.Request.Header.Set("anthropic-beta", "claude-code-20250219")
	c.Request.Header.Set("anthropic-version", "2023-06-01")
}

// passCharModerationService 构造指向 mock 审核 API 的内容审核服务。
func passCharModerationService(t *testing.T, moderationBaseURL string) *service.ContentModerationService {
	t.Helper()
	cfgJSON, err := json.Marshal(map[string]any{
		"enabled":          true,
		"mode":             "pre_block",
		"base_url":         moderationBaseURL,
		"api_keys":         []string{"sk-audit"},
		"sample_rate":      100,
		"all_groups":       true,
		"auto_ban_enabled": false,
		"email_on_hit":     false,
		"retry_count":      0,
		"timeout_ms":       2000,
	})
	require.NoError(t, err)

	settingRepo := &passCharSettingRepo{values: map[string]string{
		service.SettingKeyRiskControlEnabled:      "true",
		service.SettingKeyContentModerationConfig: string(cfgJSON),
	}}
	return service.NewContentModerationService(settingRepo, &passCharModerationRepo{}, nil, nil, nil, nil, nil)
}

// passCharRequireWarmupMock 断言响应为预热拦截 mock（请求被放行并完成）。
func passCharRequireWarmupMock(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "msg_mock_warmup", resp["id"])
}

// TestGatewayCharacterization_ContentModerationBlock 固化 I-7.1 拦截侧：
// 审核 API 判定命中（pre_block 模式）时，客户端收到 403 +
// {"type":"error","error":{"type":"content_policy_violation","message":<配置的拦截文案>}}。
func TestGatewayCharacterization_ContentModerationBlock(t *testing.T) {
	moderationSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"flagged":true,"category_scores":{"sexual":0.99}}]}`))
	}))
	defer moderationSrv.Close()

	h, cleanup := passCharInterceptFixture(t)
	defer cleanup()
	h.preFlightHooks = ProvideGatewayHookChain(passCharModerationService(t, moderationSrv.URL))

	c, rec := passCharNewMessagesContext(t, "/v1/messages", passCharWarmupBody())
	h.Messages(c)

	require.Equal(t, http.StatusForbidden, rec.Code, "内容审核命中应返回 403")
	require.JSONEq(t,
		`{"type":"error","error":{"type":"content_policy_violation","message":"内容审计命中风险规则，请调整输入后重试"}}`,
		rec.Body.String())
}

// TestGatewayCharacterization_ContentModerationFailOpen 固化 I-7.1 fail-open 侧：
// 审核 API 自身失败（HTTP 500）时请求必须放行——客户端收到正常业务响应
// （此处为预热拦截 mock 200），而不是 403。
func TestGatewayCharacterization_ContentModerationFailOpen(t *testing.T) {
	moderationSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "moderation backend exploded", http.StatusInternalServerError)
	}))
	defer moderationSrv.Close()

	h, cleanup := passCharInterceptFixture(t)
	defer cleanup()
	h.preFlightHooks = ProvideGatewayHookChain(passCharModerationService(t, moderationSrv.URL))

	c, rec := passCharNewMessagesContext(t, "/v1/messages", passCharWarmupBody())
	h.Messages(c)

	passCharRequireWarmupMock(t, rec)
}

// passCharVersionBoundSettingService 返回固定 min=1.5.0（无 max）的 SettingService。
// 注意：版本上下限有进程内全局 60s 缓存，本文件所有版本测试统一使用该配置。
func passCharVersionBoundSettingService() *service.SettingService {
	repo := &passCharSettingRepo{values: map[string]string{
		service.SettingKeyMinClaudeCodeVersion: "1.5.0",
	}}
	return service.NewSettingService(repo, &config.Config{})
}

// TestGatewayCharacterization_ClaudeCodeVersionCheck 固化 I-7.2 主路径：
// Claude Code 客户端（UA claude-cli/x.y.z + 完整客户端特征）在 /v1/messages 上：
//   - CLI 版本低于最低要求 → 400 invalid_request_error + 升级提示；
//   - CLI 版本满足要求 → 请求放行（预热拦截 mock 200）。
func TestGatewayCharacterization_ClaudeCodeVersionCheck(t *testing.T) {
	t.Run("低版本_400升级提示", func(t *testing.T) {
		h, cleanup := passCharInterceptFixture(t)
		defer cleanup()
		h.settingService = passCharVersionBoundSettingService()

		c, rec := passCharNewMessagesContext(t, "/v1/messages", passCharClaudeCodeWarmupBody())
		passCharSetClaudeCodeHeaders(c, "claude-cli/1.0.0 (external)")
		h.Messages(c)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		var resp struct {
			Type  string `json:"type"`
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "error", resp.Type)
		require.Equal(t, "invalid_request_error", resp.Error.Type)
		require.Contains(t, resp.Error.Message, "Your Claude Code version (1.0.0) is below the minimum required version (1.5.0)")
		require.Contains(t, resp.Error.Message, "npm update -g @anthropic-ai/claude-code", "错误信息应包含升级指引")
	})

	t.Run("版本达标_请求放行", func(t *testing.T) {
		h, cleanup := passCharInterceptFixture(t)
		defer cleanup()
		h.settingService = passCharVersionBoundSettingService()

		c, rec := passCharNewMessagesContext(t, "/v1/messages", passCharClaudeCodeWarmupBody())
		passCharSetClaudeCodeHeaders(c, "claude-cli/2.0.0 (external)")
		h.Messages(c)

		passCharRequireWarmupMock(t, rec)
	})
}

// TestGatewayCharacterization_ClaudeCodeVersionCheck_CountTokensExempt 固化 I-7.2 豁免侧：
// /v1/messages/count_tokens 不做版本检查——低版本 Claude Code CLI 不会收到版本 400，
// 请求继续走 count_tokens 业务（antigravity 账号当前返回 404 not_found_error）。
func TestGatewayCharacterization_ClaudeCodeVersionCheck_CountTokensExempt(t *testing.T) {
	h, cleanup := passCharInterceptFixture(t)
	defer cleanup()
	h.settingService = passCharVersionBoundSettingService()

	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	c, rec := passCharNewMessagesContext(t, "/v1/messages/count_tokens", body)
	passCharSetClaudeCodeHeaders(c, "claude-cli/1.0.0 (external)")
	h.CountTokens(c)

	var resp struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotContains(t, resp.Error.Message, "below the minimum required version",
		"count_tokens 必须豁免版本检查")

	// 固化当前实际行为：antigravity 账号不支持 count_tokens，返回 404 not_found_error。
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "not_found_error", resp.Error.Type)
}
