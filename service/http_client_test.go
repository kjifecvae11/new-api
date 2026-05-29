package service

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewNoReuseHttpClientDisablesPoolingAndHTTP2(t *testing.T) {
	t.Parallel()

	client, err := NewNoReuseHttpClient("")
	require.NoError(t, err)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.True(t, transport.DisableKeepAlives)
	require.False(t, transport.ForceAttemptHTTP2)
	require.NotNil(t, transport.TLSNextProto)
	require.NotNil(t, transport.Proxy)
}

func TestNewNoReuseHttpClientSupportsHTTPProxy(t *testing.T) {
	t.Parallel()

	client, err := NewNoReuseHttpClient("http://127.0.0.1:18080")
	require.NoError(t, err)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.Proxy)

	proxyURL, err := transport.Proxy(&http.Request{URL: mustParseURL(t, "https://example.com")})
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:18080", proxyURL.String())
	require.True(t, transport.DisableKeepAlives)
	require.False(t, transport.ForceAttemptHTTP2)
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	return parsed
}
