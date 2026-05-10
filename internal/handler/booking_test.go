package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

const testJWTSecret = "test-secret"

func signedBookingTestToken(t *testing.T, userID int, role string) string {
	t.Helper()

	claims := model.Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(
		[]byte(testJWTSecret),
	)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return token
}

func TestCreateBooking_InvalidBody_ReturnsStructuredError(t *testing.T) {
	// booking service is not needed for this test because decode fails first
	h := NewBookingHandler((*service.BookingService)(nil), nil, nil, nil, nil, nil)

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
	h := NewBookingHandler((*service.BookingService)(nil), nil, nil, nil, nil, nil)

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
	h := NewBookingHandler((*service.BookingService)(nil), nil, nil, nil, nil, nil)

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

func TestToBookingResponse_ExposesNoShowAt(t *testing.T) {
	when := time.Date(2026, time.May, 10, 17, 4, 5, 0, time.UTC)
	booking := &model.Booking{
		BookingID: 1,
		ClientID:  2,
		Status:    model.BookingStatusNoShow,
		NoShowAt:  &when,
	}

	resp := toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", "")

	if resp.NoShowAt == nil {
		t.Fatalf("expected no_show_at to be exposed")
	}
	if !resp.NoShowAt.Equal(when) {
		t.Fatalf("expected no_show_at %v, got %v", when, resp.NoShowAt)
	}
}

func TestListBookings_AdminInvalidClientID_ReturnsStructuredError(t *testing.T) {
	h := NewBookingHandler((*service.BookingService)(nil), nil, nil, nil, nil, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/bookings?client_id=not-a-number", nil)
	req.Header.Set(
		"Authorization",
		"Bearer "+signedBookingTestToken(t, 1, model.RoleAdmin),
	)

	middleware.AuthMiddleware(http.HandlerFunc(h.ListBookings), testJWTSecret).
		ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", rr.Code, rr.Body.String())
	}

	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err != nil {
		t.Fatalf("failed to decode response: %v; body: %s", err, rr.Body.String())
	}

	if er.Message != "client_id must be a positive integer" {
		t.Errorf("expected client_id validation message, got %q", er.Message)
	}
}
