package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func TestRespondServiceError(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		err         error
		wantStatus  int
		wantMessage string
		wantCode    string
	}{
		{
			name:        "preserves typed validation error",
			status:      http.StatusBadRequest,
			err:         service.NewValidationError("invalid_booking", "booking is invalid", nil),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "booking is invalid",
			wantCode:    "invalid_booking",
		},
		{
			name:        "preserves wrapped typed validation error",
			status:      http.StatusBadRequest,
			err:         fmt.Errorf("booking 2: %w", service.NewValidationError("invalid_schedule", "Enter a valid scheduled date and time.", nil)),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "Enter a valid scheduled date and time.",
			wantCode:    "invalid_schedule",
		},
		{
			name:        "preserves domain conflict",
			status:      http.StatusConflict,
			err:         errors.New("email already in use"),
			wantStatus:  http.StatusConflict,
			wantMessage: "email already in use",
		},
		{
			name:        "preserves legacy validation error",
			status:      http.StatusBadRequest,
			err:         errors.New("invalid cursor_id"),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid cursor_id",
		},
		{
			name:        "preserves field validation",
			status:      http.StatusBadRequest,
			err:         errors.New("amount must be positive"),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "amount must be positive",
		},
		{
			name:        "preserves signup validation",
			status:      http.StatusBadRequest,
			err:         errors.New("password must have a number"),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "password must have a number",
		},
		{
			name:        "preserves coordinate validation",
			status:      http.StatusBadRequest,
			err:         errors.New("coordinates out of supported range"),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "coordinates out of supported range",
		},
		{
			name:        "preserves payout validation",
			status:      http.StatusBadRequest,
			err:         errors.New("minimum payout is ₱100.00"),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "minimum payout is ₱100.00",
		},
		{
			name:        "preserves booking rule",
			status:      http.StatusBadRequest,
			err:         errors.New("voucher expired"),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "voucher expired",
		},
		{
			name:        "redacts database error",
			status:      http.StatusBadRequest,
			err:         fmt.Errorf("create booking: %w", &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type json"}),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "We couldn't complete your request right now. Please try again.",
			wantCode:    "internal_error",
		},
		{
			name:        "redacts wrapped repository error",
			status:      http.StatusBadRequest,
			err:         fmt.Errorf("load payment: %w", errors.New("boom")),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "We couldn't complete your request right now. Please try again.",
			wantCode:    "internal_error",
		},
		{
			name:        "promotes unknown bad request error",
			status:      http.StatusBadRequest,
			err:         errors.New("dial tcp 10.0.0.1:5432: connection refused"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "We couldn't complete your request right now. Please try again.",
			wantCode:    "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			respondServiceError(recorder, tt.status, tt.err)

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
