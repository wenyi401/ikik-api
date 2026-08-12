package handler

import (
	"net/http"
	"time"

	"ikik-api/internal/service"

	"github.com/gin-gonic/gin"
)

const browserRefreshCookieName = "ikik_refresh"

func writeBrowserRefreshCookie(c *gin.Context, pair *service.TokenPair) {
	if c == nil || pair == nil || pair.RefreshToken == "" {
		return
	}
	c.Header("Cache-Control", "no-store")
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	if pair.Session != nil && pair.Session.ExpiresAt > time.Now().Unix() {
		expiresAt = time.Unix(pair.Session.ExpiresAt, 0)
	}
	maxAge := int(time.Until(expiresAt) / time.Second)
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name: browserRefreshCookieName, Value: pair.RefreshToken,
		Path: "/api/v1/auth", MaxAge: maxAge, Expires: expiresAt,
		HttpOnly: true, Secure: isRequestHTTPS(c), SameSite: http.SameSiteStrictMode,
	})
}

func clearBrowserRefreshCookie(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	http.SetCookie(c.Writer, &http.Cookie{
		Name: browserRefreshCookieName, Value: "", Path: "/api/v1/auth",
		MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true,
		Secure: isRequestHTTPS(c), SameSite: http.SameSiteStrictMode,
	})
}

func readBrowserRefreshCookie(c *gin.Context) string {
	cookie, err := c.Request.Cookie(browserRefreshCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
