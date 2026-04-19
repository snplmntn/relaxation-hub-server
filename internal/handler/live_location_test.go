package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func TestUpdateLocation_InvalidBody_ReturnsStructuredError(t *testing.T) {
	h := NewLiveLocationHandler(nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/locations", bytes.NewBufferString("not-json"))

	h.UpdateLocation(rr, req)

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

type handlerLiveLocationRepoStub struct {
	locationByUserID map[int64]*model.LiveLocation
}

func (s *handlerLiveLocationRepoStub) Upsert(context.Context, *model.LiveLocation) error {
	return nil
}

func (s *handlerLiveLocationRepoStub) GetByUserID(_ context.Context, userID int64) (*model.LiveLocation, error) {
	loc, ok := s.locationByUserID[userID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return loc, nil
}

type handlerBookingRepoStub struct {
	booking *model.Booking
	err     error
}

func (s *handlerBookingRepoStub) GetByBookingID(context.Context, int64) (*model.Booking, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.booking, nil
}

func TestGetLocation_NonSelfNonAdminForbidden(t *testing.T) {
	locationRepo := &handlerLiveLocationRepoStub{
		locationByUserID: map[int64]*model.LiveLocation{
			22: {
				LocationID:  7,
				UserID:      22,
				Latitude:    14.55,
				Longitude:   121.02,
				LastUpdated: time.Now(),
			},
		},
	}
	h := NewLiveLocationHandler(service.NewLiveLocationService(locationRepo, nil, nil))

	r := chi.NewRouter()
	r.With(testAuthMiddleware()).Get("/locations/live/{user_id}", h.GetLocation)

	req := httptest.NewRequest("GET", "/locations/live/22", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(11, model.RoleClient))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}

	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if er.Message != "access denied" {
		t.Fatalf("expected generic access denied message, got %q", er.Message)
	}
}

func TestGetBookingLocation_SuccessReturnsOtherParticipantLocation(t *testing.T) {
	therapistID := int64(22)
	clientID := int64(11)
	now := time.Date(2026, 4, 19, 11, 30, 0, 0, time.UTC)

	locationRepo := &handlerLiveLocationRepoStub{
		locationByUserID: map[int64]*model.LiveLocation{
			therapistID: {
				LocationID:  8,
				UserID:      therapistID,
				Latitude:    14.55,
				Longitude:   121.02,
				LastUpdated: now,
			},
		},
	}
	bookingRepo := &handlerBookingRepoStub{
		booking: &model.Booking{
			BookingID:   91,
			ClientID:    clientID,
			TherapistID: &therapistID,
			Status:      model.BookingStatusOnTheWay,
		},
	}

	h := NewLiveLocationHandler(service.NewLiveLocationService(locationRepo, bookingRepo, nil))
	r := chi.NewRouter()
	r.With(testAuthMiddleware()).Get("/bookings/{id}/live-location", h.GetBookingLocation)

	req := httptest.NewRequest("GET", "/bookings/91/live-location", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(int(clientID), model.RoleClient))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp model.LiveLocationResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.UserID != therapistID {
		t.Fatalf("expected therapist location for user %d, got %d", therapistID, resp.UserID)
	}
}

func TestGetBookingLocation_UnauthorizedReturnsGenericForbidden(t *testing.T) {
	therapistID := int64(22)
	clientID := int64(11)

	locationRepo := &handlerLiveLocationRepoStub{
		locationByUserID: map[int64]*model.LiveLocation{
			therapistID: {
				LocationID:  9,
				UserID:      therapistID,
				Latitude:    14.55,
				Longitude:   121.02,
				LastUpdated: time.Now(),
			},
		},
	}
	bookingRepo := &handlerBookingRepoStub{
		booking: &model.Booking{
			BookingID:   92,
			ClientID:    clientID,
			TherapistID: &therapistID,
			Status:      model.BookingStatusAssigned,
		},
	}

	h := NewLiveLocationHandler(service.NewLiveLocationService(locationRepo, bookingRepo, nil))
	r := chi.NewRouter()
	r.With(testAuthMiddleware()).Get("/bookings/{id}/live-location", h.GetBookingLocation)

	req := httptest.NewRequest("GET", "/bookings/92/live-location", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(int(clientID), model.RoleClient))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}

	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if er.Message != "access denied" {
		t.Fatalf("expected generic access denied message, got %q", er.Message)
	}
}

func testAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return middleware.AuthMiddleware(next, "tests-secret")
	}
}

func testAuthToken(userID int, role string) string {
	claims := &model.Claims{UserID: userID, Role: role}
	tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("tests-secret"))
	return tokenStr
}
