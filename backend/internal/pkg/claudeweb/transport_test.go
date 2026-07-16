package claudeweb

import (
	"context"
	"testing"

	http "github.com/bogdanfinn/fhttp"
	"github.com/stretchr/testify/require"
)

func TestTLSBrowserTransportBuildsBrowserRequest(t *testing.T) {
	transport := &tlsBrowserTransport{baseURL: "https://claude.ai"}
	request, err := transport.NewRequest(context.Background(), http.MethodGet, "/api/organizations", nil)
	require.NoError(t, err)
	require.Equal(t, "https://claude.ai/api/organizations", request.URL.String())
	require.Contains(t, request.Header.Get("User-Agent"), "Chrome/147")
	require.Equal(t, `"Windows"`, request.Header.Get("sec-ch-ua-platform"))
	require.Equal(t, "same-origin", request.Header.Get("sec-fetch-site"))
}
