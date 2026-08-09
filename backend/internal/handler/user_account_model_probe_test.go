package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ikik-api/internal/config"
	"ikik-api/internal/pkg/tlsfingerprint"
	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userModelProbeHTTPUpstream struct {
	requests []*http.Request
}

func (u *userModelProbeHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.DoWithTLS(req, "", 0, 0, nil)
}

func (u *userModelProbeHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.requests = append(u.requests, req)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-5.4"}]}`)),
	}, nil
}

func newUserModelProbeHandler(upstream service.HTTPUpstream) *UserAccountHandler {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true},
		},
	}
	accountTestService := service.NewAccountTestService(nil, nil, nil, nil, nil, upstream, cfg, nil)
	return NewUserAccountHandler(nil, nil, accountTestService, nil, nil, nil, nil)
}

func TestUserAccountModelProbeRejectsPrivateTargetWithoutCallingUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &userModelProbeHTTPUpstream{}
	h := newUserModelProbeHandler(upstream)
	router := gin.New()
	router.POST("/api/v1/accounts/model-probe/list", h.ProbeModelList)

	body := `{"platform":"openai","base_url":"https://127.0.0.1","api_key":"sk-private-secret"}`
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/model-probe/list", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, upstream.requests)
	require.NotContains(t, recorder.Body.String(), "sk-private-secret")
}

func TestUserAccountModelProbeReturnsDiscoveredModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &userModelProbeHTTPUpstream{}
	h := newUserModelProbeHandler(upstream)
	router := gin.New()
	router.POST("/api/v1/accounts/model-probe/list", h.ProbeModelList)

	body := `{"platform":"openai","base_url":"https://1.1.1.1","api_key":"sk-test"}`
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/model-probe/list", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "gpt-5.4")
	require.Len(t, upstream.requests, 1)
	require.True(t, service.HTTPUpstreamRedirectsDisabled(upstream.requests[0].Context()))
}
