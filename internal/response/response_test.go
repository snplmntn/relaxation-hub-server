package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondError(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		message     string
		wantStatus  int
		wantMessage string
		wantCode    string
	}{
		{
			name:        "preserves client error",
			status:      http.StatusBadRequest,
			message:     "invalid booking status",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid booking status",
		},
		{
			name:        "redacts server error",
			status:      http.StatusInternalServerError,
			message:     "connection refused",
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "We couldn't complete your request right now. Please try again.",
			wantCode:    "internal_error",
		},
		{
			name:        "preserves service unavailable status",
			status:      http.StatusServiceUnavailable,
			message:     "authentication lookup failed",
			wantStatus:  http.StatusServiceUnavailable,
			wantMessage: "We couldn't complete your request right now. Please try again.",
			wantCode:    "internal_error",
		},
		{
			name:        "promotes and redacts database error",
			status:      http.StatusBadRequest,
			message:     "ERROR: invalid input syntax for type json (SQLSTATE 22P02)",
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "We couldn't complete your request right now. Please try again.",
			wantCode:    "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			RespondError(recorder, tt.status, tt.message)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, recorder.Code)
			}

			var response ErrorResponse
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Message != tt.wantMessage {
				t.Fatalf("expected message %q, got %q", tt.wantMessage, response.Message)
			}
			if response.Code != tt.wantCode {
				t.Fatalf("expected code %q, got %q", tt.wantCode, response.Code)
			}
		})
	}
}
