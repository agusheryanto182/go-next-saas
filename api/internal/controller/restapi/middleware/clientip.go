package middleware

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

// ClientIP puts the client's address in the request context (read it with ClientIPFrom). Sessions
// and rate limiting use it (ADR 0006, 0013).
//
// X-Forwarded-For is believed only from a trusted proxy, and only as far as the chain of trusted
// proxies goes: the client IP is the rightmost address that is not one of them. Anything to its
// left was written by the client and may be forged.
func ClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip, ok := clientIP(r, trusted); ok {
				r = r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIPFrom returns the address ClientIP found, or false when the peer address is unknown.
func ClientIPFrom(ctx context.Context) (netip.Addr, bool) {
	ip, ok := ctx.Value(clientIPKey{}).(netip.Addr)
	return ip, ok
}

func clientIP(r *http.Request, trusted []netip.Prefix) (netip.Addr, bool) {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, false
	}
	ip := normalize(peer.Addr())

	// Walk the hops from the nearest one outward while the current hop is a trusted proxy.
	hops := forwardedFor(r.Header)
	for i := len(hops) - 1; i >= 0 && isTrusted(ip, trusted); i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break // a malformed entry: keep the last address a trusted proxy vouched for
		}
		ip = normalize(hop)
	}
	return ip, true
}

// forwardedFor returns every X-Forwarded-For entry in order; proxies may append a header line each.
func forwardedFor(h http.Header) []string {
	var hops []string
	for _, line := range h.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(line, ",")...)
	}
	return hops
}

func isTrusted(ip netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// normalize compares IPv4 clients of a dual-stack listener (::ffff:a.b.c.d) as IPv4 and drops zones.
func normalize(ip netip.Addr) netip.Addr {
	return ip.Unmap().WithZone("")
}
