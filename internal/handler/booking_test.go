package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func TestCreateBooking_InvalidBody_ReturnsStructuredError(t *testing.T) {
	// booking service is not needed for this test because decode fails first
	h := NewBookingHandler((*service.BookingService)(nil), nil, nil, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/bookings", bytes.NewBufferString("not-json"))

	h.CreateBooking(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err != nil {
		t.Fatalf("failed to decode response: %v; body: %s", err, rr.Body.String())
	}

	if er.Error != "Bad Request" {
		t.Errorf("expected Error 'Bad Request', got %q", er.Error)
	}
	if er.Message != "invalid request body" {
		t.Errorf("expected Message 'invalid request body', got %q", er.Message)
	}
}

func TestAdminCreateBooking_InvalidBody_ReturnsStructuredError(t *testing.T) {
	h := NewBookingHandler((*service.BookingService)(nil), nil, nil, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/bookings", bytes.NewBufferString("not-json"))

	h.AdminCreateBooking(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err != nil {
		t.Fatalf("failed to decode response: %v; body: %s", err, rr.Body.String())
	}

	if er.Error != "Bad Request" {
		t.Errorf("expected Error 'Bad Request', got %q", er.Error)
	}
	if er.Message != "invalid request body" {
		t.Errorf("expected Message 'invalid request body', got %q", er.Message)
	}
}

func TestAdminCreateBooking_NoUser_Unauthorized(t *testing.T) {
	h := NewBookingHandler((*service.BookingService)(nil), nil, nil, nil)

	body := map[string]interface{}{"client_id": 1}
	bb, _ := json.Marshal(body)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/bookings", bytes.NewBuffer(bb))
	req.Header.Set("Content-Type", "application/json")

	h.AdminCreateBooking(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d; body: %s", rr.Code, rr.Body.String())
	}
}
