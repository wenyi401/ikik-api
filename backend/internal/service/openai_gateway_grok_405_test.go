package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldFailoverUpstreamErrorIncludesMethodNotAllowed(t *testing.T) {
	svc := &OpenAIGatewayService{}

	require.True(t, svc.shouldFailoverUpstreamError(http.StatusMethodNotAllowed))
	require.True(t, svc.shouldFailoverUpstreamError(http.StatusTooManyRequests))
	require.False(t, svc.shouldFailoverUpstreamError(http.StatusNotFound))
}
