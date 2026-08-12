package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"ikik-api/internal/service"
)

func TestBrowserRefreshCookieMatchesSessionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	pair := &service.TokenPair{
		RefreshToken: "sid.secret",
		Session:      &service.LoginSessionView{ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix()},
	}

	writeBrowserRefreshCookie(c, pair)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	require.Equal(t, browserRefreshCookieName, cookie.Name)
	require.Equal(t, "/api/v1/auth", cookie.Path)
	require.True(t, cookie.HttpOnly)
	require.True(t, cookie.Secure)
	require.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
	require.Greater(t, cookie.MaxAge, 29*24*60*60)
}

func TestRefreshRequiresHttpOnlyCookieAndRejectsLegacyBodyToken(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":"legacy-body-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	(&AuthHandler{}).RefreshToken(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	cookie := findCookie(recorder.Result().Cookies(), browserRefreshCookieName)
	require.NotNil(t, cookie)
	require.Equal(t, -1, cookie.MaxAge)
}

func TestLogoutRejectsRefreshCookieSessionMismatch(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	c.Request.Header.Set("X-Auth-Session", "11111111-1111-4111-8111-111111111111")
	c.Request.AddCookie(&http.Cookie{
		Name:  browserRefreshCookieName,
		Value: "22222222-2222-4222-8222-222222222222.secret",
	})

	(&AuthHandler{}).Logout(c)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "AUTH_SESSION_MISMATCH")
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}
