package middleware

import (
	"net/http"
	"strings"
)

// DefaultMaxBodySize is 1MB - sufficient for most API requests
const DefaultMaxBodySize = 1 << 20 // 1MB

// LargeUploadMaxBodySize is 10MB - for file uploads
const LargeUploadMaxBodySize = 10 << 20 // 10MB

// BodyLimit returns middleware that limits request body size.
// Requests exceeding the limit will receive a 413 Payload Too Large response.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip for GET, HEAD, OPTIONS which typically don't have bodies
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Multipart uploads have explicit per-handler ParseMultipartForm limits.
			// Skip the global reader cap so upload endpoints can enforce their own limits.
			contentType := r.Header.Get("Content-Type")
			if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
				next.ServeHTTP(w, r)
				return
			}

			// Wrap the body with a size-limited reader
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// DefaultBodyLimit applies the default 1MB limit to all requests.
func DefaultBodyLimit() func(http.Handler) http.Handler {
	return BodyLimit(DefaultMaxBodySize)
}

// LargeUploadBodyLimit applies a 10MB limit for endpoints that handle file uploads.
func LargeUploadBodyLimit() func(http.Handler) http.Handler {
	return BodyLimit(LargeUploadMaxBodySize)
}
