package app

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/snplmntn/relaxation-hub-server/internal/handler"
)

type dependencyHealthProvider interface {
	Snapshot(context.Context) handler.ReportDependencySnapshot
}

// NewHealthHandler returns a lightweight health endpoint backed by the report
// dependency snapshot when the provider is available.
func NewHealthHandler(provider dependencyHealthProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot := handler.ReportDependencySnapshot{
			Status:       "ok",
			Degraded:     false,
			Dependencies: map[string]handler.ReportDependencyState{},
		}
		if provider != nil {
			snapshot = provider.Snapshot(r.Context())
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(snapshot)
	}
}

// headResponseWriter discards body bytes while preserving headers/status.
type headResponseWriter struct {
	http.ResponseWriter
}

func (h *headResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}
