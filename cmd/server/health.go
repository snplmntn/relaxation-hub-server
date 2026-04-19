package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/snplmntn/relaxation-hub-server/internal/handler"
)

type dependencyHealthProvider interface {
	Snapshot(context.Context) handler.ReportDependencySnapshot
}

func newHealthHandler(provider dependencyHealthProvider) http.HandlerFunc {
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
