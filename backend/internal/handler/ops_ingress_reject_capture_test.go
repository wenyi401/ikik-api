package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	middleware2 "ikik-api/internal/server/middleware"
)

func TestOpsCaptureWriterDoesNotCopyIngressRejectBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	writer := acquireOpsCaptureWriter(context.Writer)
	defer releaseOpsCaptureWriter(writer)
	writer.setContext(context)
	context.Writer = writer
	middleware2.MarkIngressRejected(context, middleware2.IngressRejectInvalidAPIKey)
	context.Status(http.StatusUnauthorized)
	_, err := context.Writer.WriteString(`{"code":"INVALID_API_KEY","message":"Invalid API key"}`)
	require.NoError(t, err)
	require.Empty(t, writer.capturedBytes())
}
