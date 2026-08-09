package repository

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildUpstreamTransportSetsDialTimeout(t *testing.T) {
	transport, err := buildUpstreamTransport(defaultPoolSettings(nil), nil, upstreamProtocolModeDefault)
	require.NoError(t, err)
	require.NotNil(t, transport.DialContext)
	require.Equal(t, defaultUpstreamTLSHandshakeTimeout, transport.TLSHandshakeTimeout)

	dialer := newUpstreamDialer()
	require.Equal(t, defaultUpstreamDialTimeout, dialer.Timeout)
	require.Equal(t, defaultUpstreamDialKeepAlive, dialer.KeepAlive)
	require.Greater(t, dialer.Timeout, time.Duration(0))
}

func TestBuildUpstreamTransportKeepsDialContextWithProxies(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:1080", "socks5h://127.0.0.1:1080"} {
		t.Run(raw, func(t *testing.T) {
			proxyURL, err := url.Parse(raw)
			require.NoError(t, err)
			transport, err := buildUpstreamTransport(defaultPoolSettings(nil), proxyURL, upstreamProtocolModeDefault)
			require.NoError(t, err)
			require.NotNil(t, transport.DialContext)
		})
	}
}
