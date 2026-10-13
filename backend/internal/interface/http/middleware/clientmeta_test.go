package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPTrustedHops(t *testing.T) {
	cases := []struct {
		name string
		hops int
		xff  string
		want string
	}{
		{"no proxy ignores forged header", 0, "6.6.6.6", "10.0.0.9"},
		{"one proxy takes rightmost", 1, "6.6.6.6, 203.0.113.7", "203.0.113.7"},
		{"two proxies skip the inner one", 2, "6.6.6.6, 203.0.113.7, 10.0.0.2", "203.0.113.7"},
		{"short header falls back to peer", 2, "203.0.113.7", "10.0.0.9"},
		{"garbage falls back to peer", 1, "not-an-ip", "10.0.0.9"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			r.RemoteAddr = "10.0.0.9:51234"
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}
			if got := clientIP(r, c.hops); got != c.want {
				t.Fatalf("clientIP = %q, want %q", got, c.want)
			}
		})
	}
}
