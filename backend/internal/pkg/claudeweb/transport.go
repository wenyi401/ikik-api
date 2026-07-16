package claudeweb

import (
	"context"
	"fmt"
	"io"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type browserTransport interface {
	NewRequest(context.Context, string, string, io.Reader) (*http.Request, error)
	Do(*http.Request) (*http.Response, error)
	CloseIdleConnections()
}

type tlsBrowserTransport struct {
	client  tlsclient.HttpClient
	baseURL string
}

func newTLSBrowserTransport(baseURL, proxyURL string) (*tlsBrowserTransport, error) {
	options := []tlsclient.HttpClientOption{
		tlsclient.WithTimeoutSeconds(300),
		tlsclient.WithClientProfile(profiles.Chrome_146),
		tlsclient.WithCookieJar(tlsclient.NewCookieJar()),
		tlsclient.WithNotFollowRedirects(),
	}
	if proxyURL = strings.TrimSpace(proxyURL); proxyURL != "" {
		options = append(options, tlsclient.WithProxyUrl(proxyURL))
	}
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("create Claude Web TLS client: %w", err)
	}
	return &tlsBrowserTransport{
		client:  client,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
	}, nil
}

func (c *tlsBrowserTransport) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create Claude Web request: %w", err)
	}
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	request.Header.Set("Accept-Encoding", "gzip, deflate, br")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36")
	request.Header.Set("Origin", c.baseURL)
	request.Header.Set("Referer", c.baseURL+"/")
	request.Header.Set("sec-ch-ua", `"Google Chrome";v="147", "Not.A/Brand";v="8", "Chromium";v="147"`)
	request.Header.Set("sec-ch-ua-mobile", "?0")
	request.Header.Set("sec-ch-ua-platform", `"Windows"`)
	request.Header.Set("sec-fetch-dest", "empty")
	request.Header.Set("sec-fetch-mode", "cors")
	request.Header.Set("sec-fetch-site", "same-origin")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func (c *tlsBrowserTransport) Do(request *http.Request) (*http.Response, error) {
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("execute Claude Web request: %w", err)
	}
	return response, nil
}

func (c *tlsBrowserTransport) CloseIdleConnections() {
	if c != nil && c.client != nil {
		c.client.CloseIdleConnections()
	}
}
