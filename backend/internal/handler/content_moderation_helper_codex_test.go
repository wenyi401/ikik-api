package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	middleware2 "ikik-api/internal/server/middleware"
	"ikik-api/internal/service"
)

func TestBuildContentModerationInputMarksOfficialCodexClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.144.0 (Windows; x86_64)")
	c.Request.Header.Set("originator", "codex_cli_rs")

	input := buildContentModerationInput(
		c,
		nil,
		middleware2.AuthSubject{},
		service.ContentModerationProtocolOpenAIResponses,
		"gpt-5.5",
		nil,
	)

	require.True(t, input.CodexOfficialClient)
}

func TestBuildContentModerationInputDoesNotTrustOrdinaryClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "curl/8.12.0")

	input := buildContentModerationInput(
		c,
		nil,
		middleware2.AuthSubject{},
		service.ContentModerationProtocolOpenAIResponses,
		"gpt-5.5",
		nil,
	)

	require.False(t, input.CodexOfficialClient)
}
