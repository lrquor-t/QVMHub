package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsProxyHopHeader(t *testing.T) {
	deny := []string{
		"Authorization", "authorization", "Cookie", "cookie",
		"X-API-Key", "x-api-key", "X-API-Key-ID", "x-api-key-id",
		"Connection", "Transfer-Encoding", "Upgrade", "Host",
	}
	for _, h := range deny {
		require.True(t, isProxyHopHeader(h), "%s should be stripped", h)
	}
	allow := []string{"Content-Type", "Accept", "X-High-Risk-Token", "User-Agent"}
	for _, h := range allow {
		require.False(t, isProxyHopHeader(h), "%s should be forwarded", h)
	}
}

// TestCopyProxyHeadersStripsAuth verifies the controller's JWT/cookie never
// reaches the node, while content-type is preserved.
func TestCopyProxyHeadersStripsAuth(t *testing.T) {
	src := http.Header{}
	src.Set("Authorization", "Bearer controller-jwt")
	src.Set("Cookie", "session=abc")
	src.Set("X-API-Key", "should-be-overwritten")
	src.Set("Content-Type", "application/json")
	src.Set("Accept", "application/json")

	dst := http.Header{}
	copyProxyHeaders(dst, src)

	require.NotContains(t, dst, "Authorization")
	require.NotContains(t, dst, "Cookie")
	require.NotContains(t, dst, "X-Api-Key")
	require.NotContains(t, dst, "X-Api-Key-Id")
	require.Equal(t, "application/json", dst.Get("Content-Type"))
	require.Equal(t, "application/json", dst.Get("Accept"))
}

func TestIsStreamRequest(t *testing.T) {
	cases := []struct {
		path   string
		accept string
		want   bool
	}{
		{"/api/host/stats/sse", "", true},
		{"/api/task/events", "", true},
		{"/api/vm/list", "", false},
		{"/api/vm/console", "text/event-stream", true},
		{"/api/vm/list", "application/json", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if tc.accept != "" {
			req.Header.Set("Accept", tc.accept)
		}
		require.Equal(t, tc.want, isStreamRequest(req), "path=%s accept=%s", tc.path, tc.accept)
	}
}
