package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"ikik-api/internal/plugin"
)

type moduleStatusSourceStub struct {
	statuses []plugin.ModuleStatus
}

func (s *moduleStatusSourceStub) Snapshot() []plugin.ModuleStatus {
	return s.statuses
}

type moduleListEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Modules []struct {
			ID        string `json:"id"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			Enabled   bool   `json:"enabled"`
			State     string `json:"state"`
			Error     string `json:"error"`
		} `json:"modules"`
	} `json:"data"`
}

func performModuleList(t *testing.T, source moduleStatusSource) (*httptest.ResponseRecorder, moduleListEnvelope) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &ModuleHandler{runtime: source}
	router.GET("/admin/modules", handler.List)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/modules", nil)
	router.ServeHTTP(w, req)

	var envelope moduleListEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	return w, envelope
}

func TestModuleHandlerListSerializesSnapshot(t *testing.T) {
	stub := &moduleStatusSourceStub{statuses: []plugin.ModuleStatus{
		{ID: "gateway.platform.anthropic", Enabled: true, State: plugin.StateRunning},
		{ID: "job.hello", Enabled: false, State: plugin.StateRegistered},
	}}

	w, envelope := performModuleList(t, stub)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 0, envelope.Code)
	require.Equal(t, "success", envelope.Message)
	require.Len(t, envelope.Data.Modules, 2)

	first := envelope.Data.Modules[0]
	require.Equal(t, "gateway.platform.anthropic", first.ID)
	require.Equal(t, "gateway.platform", first.Namespace)
	require.Equal(t, "anthropic", first.Name)
	require.True(t, first.Enabled)
	require.Equal(t, "running", first.State)
	require.Empty(t, first.Error)

	// 未启用模块也在列：state=registered、enabled=false
	second := envelope.Data.Modules[1]
	require.Equal(t, "job.hello", second.ID)
	require.Equal(t, "job", second.Namespace)
	require.Equal(t, "hello", second.Name)
	require.False(t, second.Enabled)
	require.Equal(t, "registered", second.State)
	require.Empty(t, second.Error)

	// snake_case 字段恒定输出（含空 error）
	require.Contains(t, w.Body.String(), `"error":""`)
	require.Contains(t, w.Body.String(), `"state":"registered"`)
}

func TestModuleHandlerListSortsByID(t *testing.T) {
	stub := &moduleStatusSourceStub{statuses: []plugin.ModuleStatus{
		{ID: "job.zeta", Enabled: true, State: plugin.StateRunning},
		{ID: "gateway.alpha", Enabled: true, State: plugin.StateRunning},
		{ID: "job.alpha", Enabled: false, State: plugin.StateRegistered},
	}}

	_, envelope := performModuleList(t, stub)

	ids := make([]string, 0, len(envelope.Data.Modules))
	for _, m := range envelope.Data.Modules {
		ids = append(ids, m.ID)
	}
	require.Equal(t, []string{"gateway.alpha", "job.alpha", "job.zeta"}, ids)
}

func TestModuleHandlerListRedactsErrorText(t *testing.T) {
	stub := &moduleStatusSourceStub{statuses: []plugin.ModuleStatus{
		{ID: "job.bad", Enabled: true, State: plugin.StateErrored, Err: "provision failed: access_token=ya29.secret-value"},
	}}

	_, envelope := performModuleList(t, stub)

	require.Len(t, envelope.Data.Modules, 1)
	errText := envelope.Data.Modules[0].Error
	require.Contains(t, errText, "provision failed")
	require.NotContains(t, errText, "ya29.secret-value")
}

func TestModuleHandlerListEmptySnapshot(t *testing.T) {
	w, envelope := performModuleList(t, &moduleStatusSourceStub{})

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, envelope.Data.Modules)
	require.Empty(t, envelope.Data.Modules)
	require.Contains(t, w.Body.String(), `"modules":[]`)
}

type moduleHandlerFakeModule struct {
	id               plugin.ModuleID
	enabledByDefault bool
}

func (m *moduleHandlerFakeModule) ModuleInfo() plugin.ModuleInfo {
	id := m.id
	enabled := m.enabledByDefault
	return plugin.ModuleInfo{
		ID:               id,
		New:              func() plugin.Module { return &moduleHandlerFakeModule{id: id, enabledByDefault: enabled} },
		EnabledByDefault: enabled,
	}
}

// TestModuleHandlerListWithRuntimeSnapshot 用真实 Runtime（隔离注册表）验证
// 接口返回与 Runtime.Snapshot 一致：enabled 模块为 running，
// disabled 模块也在列且 state=registered、enabled=false。
func TestModuleHandlerListWithRuntimeSnapshot(t *testing.T) {
	registry := plugin.NewRegistry()
	registry.RegisterModule(&moduleHandlerFakeModule{id: "job.enabled", enabledByDefault: true})
	registry.RegisterModule(&moduleHandlerFakeModule{id: "job.disabled", enabledByDefault: false})

	runtime := plugin.NewRuntimeWithRegistry(nil, nil, registry)
	require.NoError(t, runtime.Build())
	require.NoError(t, runtime.Start(context.Background()))
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/modules", NewModuleHandler(runtime).List)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/modules", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var envelope moduleListEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Modules, 2)

	disabled := envelope.Data.Modules[0]
	require.Equal(t, "job.disabled", disabled.ID)
	require.False(t, disabled.Enabled)
	require.Equal(t, "registered", disabled.State)

	enabled := envelope.Data.Modules[1]
	require.Equal(t, "job.enabled", enabled.ID)
	require.True(t, enabled.Enabled)
	require.Equal(t, "running", enabled.State)
}
