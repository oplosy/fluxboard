package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/mesutokul/fluxboard/backend/internal/pkg/reqmeta"
)

// ClientMeta returns middleware that stashes the client IP and user-agent on the
// request context so deep usecase code (audit, throttles) can read them without
// threading them through every signature (see pkg/reqmeta). trustedHops is the
// number of reverse proxies in front of the API that append to
// X-Forwarded-For; 0 ignores the header entirely. Installed early in the
// infrastructure chain.
func ClientMeta(trustedHops int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := reqmeta.WithMeta(r.Context(), reqmeta.Meta{
				IP:        clientIP(r, trustedHops),
				UserAgent: r.UserAgent(),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// clientIP resolves the caller IP. X-Forwarded-For is client-controlled except
// for the entries our own proxies append on the right, so with N trusted hops
// the client is the N-th entry from the right — never the leftmost, which any
// caller can forge to dodge per-IP throttles. With no trusted hops (or a header
// shorter than expected) the RemoteAddr host is used. Returns "" when nothing
// parses so the audit column stays NULL rather than storing garbage.
func clientIP(r *http.Request, trustedHops int) string {
	if trustedHops > 0 {
		var hops []string
		for _, v := range r.Header.Values("X-Forwarded-For") {
			for _, part := range strings.Split(v, ",") {
				if p := strings.TrimSpace(part); p != "" {
					hops = append(hops, p)
				}
			}
		}
		if len(hops) >= trustedHops {
			if ip := net.ParseIP(hops[len(hops)-trustedHops]); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if net.ParseIP(host) == nil {
		return ""
	}
	return host
}
