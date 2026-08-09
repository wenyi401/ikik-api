package proxyutil

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSOCKS5ForwardDialerHasBoundedTimeout(t *testing.T) {
	require.Equal(t, socks5DialTimeout, socks5ForwardDialer.Timeout)
	require.Equal(t, socks5DialKeepAlive, socks5ForwardDialer.KeepAlive)
	require.Greater(t, socks5ForwardDialer.Timeout, time.Duration(0))
}

func TestConfigureTransportProxySOCKS5SetsDialContext(t *testing.T) {
	for _, scheme := range []string{"socks5", "socks5h"} {
		t.Run(scheme, func(t *testing.T) {
			proxyURL, err := url.Parse(scheme + "://127.0.0.1:1080")
			require.NoError(t, err)
			transport := &http.Transport{}
			require.NoError(t, ConfigureTransportProxy(transport, proxyURL))
			require.NotNil(t, transport.DialContext)
			require.Nil(t, transport.Proxy)
		})
	}
}

func TestConfigureTransportProxyHTTPPreservesDialContext(t *testing.T) {
	proxyURL, err := url.Parse("http://127.0.0.1:8080")
	require.NoError(t, err)
	called := false
	transport := &http.Transport{DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
		called = true
		return nil, errors.New("stub dial")
	}}
	require.NoError(t, ConfigureTransportProxy(transport, proxyURL))
	_, _ = transport.DialContext(context.Background(), "tcp", "127.0.0.1:1")
	require.True(t, called)
}
