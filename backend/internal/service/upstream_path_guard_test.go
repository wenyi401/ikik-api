package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSanitizedUpstreamPathSuffix(t *testing.T) {
	for _, suffix := range []string{"", "/compact", "/compact/detail", "/resp_68f0/cancel", "/gemini-2.5-pro_v1.2"} {
		t.Run("accept_"+suffix, func(t *testing.T) {
			got, ok := sanitizedUpstreamPathSuffix(suffix)
			require.True(t, ok)
			require.Equal(t, suffix, got)
		})
	}

	for _, suffix := range []string{"/..", "/./compact", "/compact/..", "//double", "/compact//detail", "/compact/", "/compact?a=b", "/compact%2f..", "/compact\\..", "/compact\x00", "/model name", "/..."} {
		t.Run("reject_"+suffix, func(t *testing.T) {
			got, ok := sanitizedUpstreamPathSuffix(suffix)
			require.False(t, ok)
			require.Empty(t, got)
		})
	}

	longSegment := "/" + strings.Repeat("a", maxUpstreamPathSegmentLen+1)
	_, ok := sanitizedUpstreamPathSuffix(longSegment)
	require.False(t, ok)
}

func TestOpenAIResponsesRequestPathSuffixRejectsUnsafePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{
		"/v1/responses/../../x",
		"/v1/responses/%2e%2e/%2e%2e/x",
		"/responses//double",
		"/backend-api/codex/responses/../../../x",
	} {
		t.Run(path, func(t *testing.T) {
			c := newResponsesSuffixTestContext(t, path)
			require.False(t, IsForwardableOpenAIResponsesRequestPath(c))
			require.Empty(t, openAIResponsesRequestPathSuffix(c))
			require.Equal(t, chatgptCodexURL, appendOpenAIResponsesRequestPathSuffix(chatgptCodexURL, openAIResponsesRequestPathSuffix(c)))
		})
	}

	c := newResponsesSuffixTestContext(t, "/v1/responses/compact")
	require.True(t, IsForwardableOpenAIResponsesRequestPath(c))
	require.Equal(t, "/compact", openAIResponsesRequestPathSuffix(c))
	require.Equal(t, chatgptCodexURL, appendOpenAIResponsesRequestPathSuffix(chatgptCodexURL, "/../../x"))
}

func newResponsesSuffixTestContext(t *testing.T, path string) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c
}
