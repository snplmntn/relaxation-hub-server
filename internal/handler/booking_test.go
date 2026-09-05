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

func TestRedactTherapistFromTimeline(t *testing.T) {
	therapistID := int64(22)
	adminID := int64(7)
	events := []model.BookingEvent{
		{
			EventType: "on_the_way",
			ActorID:   &therapistID,
			ActorType: model.RoleTherapist,
			ActorName: "Private Therapist",
		},
		{
			EventType: "admin_reassigned_therapist",
			ActorID:   &adminID,
			ActorType: model.RoleAdmin,
			ActorName: "Dispatcher",
			Metadata: map[string]any{
				"old_therapist_id": int64(11),
				"new_therapist_id": therapistID,
				"reason":           "coverage",
			},
		},
	}

	redactTherapistFromTimeline(events, &therapistID)

	if events[0].ActorID != nil || events[0].ActorName != "" || events[0].ActorType != "" {
		t.Fatalf("therapist actor identity was not redacted: %+v", events[0])
	}
	if events[1].ActorID == nil || *events[1].ActorID != adminID || events[1].ActorName != "Dispatcher" {
		t.Fatalf("non-therapist actor should remain visible: %+v", events[1])
	}
	if _, exists := events[1].Metadata["old_therapist_id"]; exists {
		t.Fatal("old therapist metadata was not redacted")
	}
	if _, exists := events[1].Metadata["new_therapist_id"]; exists {
		t.Fatal("new therapist metadata was not redacted")
	}
	if events[1].Metadata["reason"] != "coverage" {
		t.Fatal("unrelated timeline metadata should remain visible")
	}
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

func TestParseAdminCreateBookingRequest_PreservesAllSelectedServices(t *testing.T) {
	body := bytes.NewBufferString(`{
		"client_id": 91,
		"service_id": 5,
		"service_ids": [5, "6"],
		"service_durations": [{"service_id": 5, "duration_minutes": 75}, {"service_id": 6, "duration_minutes": 45}],
		"duration_minutes": 120,
		"is_therapist_requested": true,
		"referral_source": "Phone"
	}`)

	clientID, req, err := parseAdminCreateBookingRequest(body)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if clientID == nil || *clientID != 91 {
		t.Fatalf("expected client 91, got %v", clientID)
	}
	if len(req.ServiceIDs) != 2 || req.ServiceIDs[0] != 5 || req.ServiceIDs[1] != 6 {
		t.Fatalf("expected service_ids [5 6], got %v", req.ServiceIDs)
	}
	if len(req.ServiceDurations) != 2 || req.ServiceDurations[0].DurationMinutes != 75 || req.ServiceDurations[1].DurationMinutes != 45 {
		t.Fatalf("expected service durations [75 45], got %v", req.ServiceDurations)
	}
	if req.ReferralSource != model.BookingReferralSourcePhone {
		t.Fatalf("expected Phone referral source, got %q", req.ReferralSource)
	}
	if !req.IsTherapistRequested {
		t.Fatal("expected therapist request flag to be preserved")
	}
}

func TestParseCreateBookingRequest_RejectsMalformedServiceIDs(t *testing.T) {
	_, err := parseCreateBookingRequest(bytes.NewBufferString(`{"service_ids":[5,"not-an-id"]}`))
	if err == nil {
		t.Fatal("expected malformed service_ids to be rejected")
	}
}

func TestToBookingResponse_ExposesNoShowAt(t *testing.T) {
	when := time.Date(2026, time.May, 10, 17, 4, 5, 0, time.UTC)
	booking := &model.Booking{
		BookingID:            1,
		ClientID:             2,
		Status:               model.BookingStatusNoShow,
		NoShowAt:             &when,
		IsTherapistRequested: true,
		IsLocked:             true,
	}

	resp := toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", "")

	if resp.NoShowAt == nil {
		t.Fatalf("expected no_show_at to be exposed")
	}
	if !resp.NoShowAt.Equal(when) {
		t.Fatalf("expected no_show_at %v, got %v", when, resp.NoShowAt)
	}
	if !resp.IsTherapistRequested || !resp.IsLocked {
		t.Fatalf("expected booking request and lock flags to be exposed")
	}
}

func TestToBookingResponse_UsesAllocatedServiceDuration(t *testing.T) {
	allocated := 75
	booking := &model.Booking{
		BookingID: 1,
		ClientID:  2,
		Status:    model.BookingStatusAssigned,
		Services: []model.BookingService{{
			ServiceID:                5,
			DurationSnapshot:         60,
			AllocatedDurationMinutes: &allocated,
			Service:                  &model.Service{ServiceID: 5, Name: "Massage", DurationMinutes: 60},
		}},
	}

	resp := toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", "")
	if len(resp.Services) != 1 || resp.Services[0].DurationMinutes != 75 {
		t.Fatalf("expected allocated service duration 75, got %#v", resp.Services)
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

func TestParseOptionalPositiveInt64(t *testing.T) {
	if value, err := parseOptionalPositiveInt64("", "client_id"); err != nil || value != nil {
		t.Fatalf("expected empty value to return nil without error, got value=%v err=%v", value, err)
	}

	value, err := parseOptionalPositiveInt64("42", "client_id")
	if err != nil {
		t.Fatalf("expected positive integer, got error: %v", err)
	}
	if value == nil || *value != 42 {
		t.Fatalf("expected value 42, got %v", value)
	}

	if _, err := parseOptionalPositiveInt64("not-a-number", "client_id"); err == nil {
		t.Fatalf("expected invalid client_id to fail")
	}
	if _, err := parseOptionalPositiveInt64("0", "client_id"); err == nil {
		t.Fatalf("expected zero client_id to fail")
	}
}

func TestBookingReportFilename(t *testing.T) {
	now := time.Date(2026, 5, 23, 7, 8, 9, 0, time.UTC)
	clientID := int64(7871)

	if got := bookingReportFilename(&clientID, now); got != "booking-report-client-7871-20260523-070809.xlsx" {
		t.Fatalf("unexpected client filename: %s", got)
	}
	if got := bookingReportFilename(nil, now); got != "booking-report-all-clients-20260523-070809.xlsx" {
		t.Fatalf("unexpected all-clients filename: %s", got)
	}
}
