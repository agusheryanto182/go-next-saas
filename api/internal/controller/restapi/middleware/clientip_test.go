package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/middleware"
)

func TestClientIP(t *testing.T) {
	t.Parallel()
	// Caddy in front (10.0.0.0/24) and Next.js server rendering (10.0.1.5), IPv6 proxies in fd00::/64.
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("10.0.1.5/32"),
		netip.MustParsePrefix("fd00::/64"),
	}

	tests := []struct {
		name   string
		peer   string
		xff    []string // one header line each
		want   string
		wantOK bool
	}{
		{"no proxy: the peer", "203.0.113.7:51234", nil, "203.0.113.7", true},
		{"untrusted peer: its X-Forwarded-For is ignored", "203.0.113.7:51234", []string{"198.51.100.1"}, "203.0.113.7", true},
		{"trusted peer without X-Forwarded-For", "10.0.0.2:443", nil, "10.0.0.2", true},
		{"one trusted proxy", "10.0.0.2:443", []string{"198.51.100.1"}, "198.51.100.1", true},
		{"a chain of trusted proxies", "10.0.0.2:443", []string{"198.51.100.1, 10.0.1.5"}, "198.51.100.1", true},
		{"a chain over several header lines", "10.0.0.2:443", []string{"198.51.100.1", "10.0.1.5"}, "198.51.100.1", true},
		{
			"a spoofed address to the left of the real client is ignored", "10.0.0.2:443",
			[]string{"6.6.6.6, 198.51.100.1, 10.0.1.5"},
			"198.51.100.1", true,
		},
		{"only trusted proxies: the furthest one", "10.0.0.2:443", []string{"10.0.0.9, 10.0.1.5"}, "10.0.0.9", true},
		{"a malformed entry stops at the last trusted hop", "10.0.0.2:443", []string{"198.51.100.1, garbage"}, "10.0.0.2", true},
		{"IPv6 client and peer", "[2001:db8::1]:51234", nil, "2001:db8::1", true},
		{"IPv6 client behind an IPv6 proxy", "[fd00::2]:443", []string{"2001:db8::7"}, "2001:db8::7", true},
		{"IPv6 client behind an IPv4 proxy", "10.0.0.2:443", []string{"2001:db8::7"}, "2001:db8::7", true},
		{"IPv4 peer on a dual-stack listener counts as IPv4", "[::ffff:10.0.0.2]:443", []string{"198.51.100.1"}, "198.51.100.1", true},
		{"untrusted IPv6 peer spoofing", "[2001:db8::1]:51234", []string{"10.0.0.2"}, "2001:db8::1", true},
		{"unknown peer address", "@", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got netip.Addr
			var gotOK bool
			h := middleware.ClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got, gotOK = middleware.ClientIPFrom(r.Context())
			}))
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			req.RemoteAddr = tt.peer
			for _, line := range tt.xff {
				req.Header.Add("X-Forwarded-For", line)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)

			require.Equal(t, tt.wantOK, gotOK)
			if tt.wantOK {
				assert.Equal(t, tt.want, got.String())
			}
		})
	}
}

func TestClientIPTrustsNobodyByDefault(t *testing.T) {
	t.Parallel()
	var got netip.Addr
	h := middleware.ClientIP(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = middleware.ClientIPFrom(r.Context())
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "127.0.0.1", got.String())
}
