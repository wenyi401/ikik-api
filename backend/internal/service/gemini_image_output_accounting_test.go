package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGeminiImageAccountingTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/custom-image-model:generateContent", strings.NewReader("{}"))
	return c
}

func geminiImageAccountingResponse(parts string) string {
	return `{"candidates":[{"content":{"parts":[` + parts + `]}}]}`
}

func TestCountGeminiInlineImageOutputs(t *testing.T) {
	image := `{"inlineData":{"mimeType":"image/png","data":"QUJD"}}`
	secondImage := `{"inline_data":{"mime_type":"image/webp","data":"REVG"}}`

	require.Equal(t, 2, countGeminiInlineImageOutputs([]byte(geminiImageAccountingResponse(image+","+secondImage))))
	require.Equal(t, 0, countGeminiInlineImageOutputs([]byte(geminiImageAccountingResponse(`{"inlineData":{"mimeType":"audio/mpeg","data":"QUJD"}}`))))
	require.Equal(t, 0, countGeminiInlineImageOutputs([]byte(geminiImageAccountingResponse(`{"inlineData":{"mimeType":"image/png","data":""}}`))))
}

func TestGeminiImageOutputObservationUsesLargestChunkAndResets(t *testing.T) {
	c := newGeminiImageAccountingTestContext(t)
	oneImage := []byte(geminiImageAccountingResponse(`{"inlineData":{"mimeType":"image/png","data":"QUJD"}}`))
	twoImages := []byte(geminiImageAccountingResponse(`{"inlineData":{"mimeType":"image/png","data":"QUJD"}},{"inlineData":{"mimeType":"image/png","data":"REVG"}}`))

	beginGeminiImageOutputObservation(c)
	observeGeminiImageOutputs(c, oneImage)
	observeGeminiImageOutputs(c, oneImage)
	observeGeminiImageOutputs(c, twoImages)
	require.Equal(t, 2, observedGeminiImageOutputs(c))
	require.Equal(t, 2, resolveGeminiImageCount(c, "custom-image-model", "custom-image-model"))

	beginGeminiImageOutputObservation(c)
	require.Equal(t, 0, observedGeminiImageOutputs(c))
	require.Equal(t, 1, resolveGeminiImageCount(c, "gemini-3-pro-image", "custom-image-model"))
}

func TestHandleNativeNonStreamingResponseObservesImages(t *testing.T) {
	c := newGeminiImageAccountingTestContext(t)
	beginGeminiImageOutputObservation(c)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(geminiImageAccountingResponse(
			`{"inlineData":{"mimeType":"image/png","data":"QUJD"}}`,
		))),
	}

	usage, err := (&GeminiMessagesCompatService{}).handleNativeNonStreamingResponse(c, resp, false)
	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 1, observedGeminiImageOutputs(c))
}
