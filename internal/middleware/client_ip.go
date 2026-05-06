package middleware

import (
	"net"
	"net/http"
	"strings"
)

// getClientIP resolves a stable client IP from common proxy headers.
// Preference order: X-Forwarded-For (first valid entry), X-Real-IP, RemoteAddr.
func getClientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range parts {
			ip := strings.TrimSpace(part)
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" && net.ParseIP(xrip) != nil {
		return xrip
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && net.ParseIP(host) != nil {
		return host
	}

	if net.ParseIP(strings.TrimSpace(r.RemoteAddr)) != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}

	return "unknown"
}
