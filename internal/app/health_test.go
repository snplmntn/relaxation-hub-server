package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/handler"
)

type stubDependencyHealthProvider struct {
	snapshot handler.ReportDependencySnapshot
}

func (s stubDependencyHealthProvider) Snapshot(context.Context) handler.ReportDependencySnapshot {
	return s.snapshot
}

func TestHealthHandlerReportsDependencySnapshot(t *testing.T) {
	h := NewHealthHandler(stubDependencyHealthProvider{
		snapshot: handler.ReportDependencySnapshot{
			Status:   "degraded",
			Degraded: true,
			Dependencies: map[string]handler.ReportDependencyState{
				"ledgerRepo": {
					Available: false,
					Message:   "ledgerRepo is not configured",
					CheckedAt: time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Status       string                                   `json:"status"`
		Degraded     bool                                     `json:"degraded"`
		Dependencies map[string]handler.ReportDependencyState `json:"dependencies"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health response: %v body=%s", err, w.Body.String())
	}

	if resp.Status != "degraded" {
		t.Fatalf("expected status degraded, got %q", resp.Status)
	}
	if !resp.Degraded {
		t.Fatal("expected degraded=true")
	}
	if resp.Dependencies["ledgerRepo"].Available {
		t.Fatal("expected ledgerRepo to be unavailable")
	}
}
