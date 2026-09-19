package app

import (
	"encoding/json"
	"net/http"
)

// NewHealthHandler returns a liveness endpoint that never waits on external
// dependencies. Database and storage readiness is checked at the report routes
// that require those dependencies.
func NewHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// headResponseWriter discards body bytes while preserving headers/status.
type headResponseWriter struct {
	http.ResponseWriter
}

func (h *headResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}
