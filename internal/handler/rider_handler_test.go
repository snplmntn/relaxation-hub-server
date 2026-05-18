package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/auth"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type fakeRiderProfileRepo struct {
	updates map[string]interface{}
}

func (f *fakeRiderProfileRepo) Create(ctx context.Context, ride *model.Ride) error { return nil }
func (f *fakeRiderProfileRepo) GetByID(ctx context.Context, rideID int64) (*model.Ride, error) {
	return nil, nil
}
func (f *fakeRiderProfileRepo) UpdateStatus(ctx context.Context, rideID int64, status string) error {
	return nil
}
func (f *fakeRiderProfileRepo) AssignRider(ctx context.Context, rideID, riderID int64) error {
	return nil
}
func (f *fakeRiderProfileRepo) ClaimRide(ctx context.Context, rideID, riderID int64) error {
	return nil
}
func (f *fakeRiderProfileRepo) GetPendingRides(ctx context.Context) ([]model.Ride, error) {
	return nil, nil
}
func (f *fakeRiderProfileRepo) GetRidesForRiderByStatus(ctx context.Context, riderID int64, status string) ([]model.Ride, error) {
	return nil, nil
}
func (f *fakeRiderProfileRepo) GetRidesForRider(ctx context.Context, riderID int64, status string, limit, offset int) ([]model.Ride, error) {
	return nil, nil
}
func (f *fakeRiderProfileRepo) GetAvailableRidesNear(ctx context.Context, lat, long, radiusKm float64) ([]model.Ride, error) {
	return nil, nil
}
func (f *fakeRiderProfileRepo) GetRiderProfile(ctx context.Context, userID int64) (*model.RiderProfile, error) {
	return &model.RiderProfile{RiderID: 33, UserID: userID}, nil
}
func (f *fakeRiderProfileRepo) CreateRiderProfile(ctx context.Context, userID int64, vehicleType, licensePlate string) error {
	return nil
}
func (f *fakeRiderProfileRepo) UpdateRiderProfile(ctx context.Context, riderID int64, updates map[string]interface{}) error {
	f.updates = updates
	return nil
}
func (f *fakeRiderProfileRepo) UpdateRiderLocation(ctx context.Context, riderID int64, lat, long float64) error {
	return nil
}
func (f *fakeRiderProfileRepo) GetActiveRideByRiderID(ctx context.Context, riderID int64) (*model.Ride, error) {
	return nil, nil
}
func (f *fakeRiderProfileRepo) UpdateRiderStatus(ctx context.Context, riderID int64, isOnline bool) error {
	return nil
}
func (f *fakeRiderProfileRepo) GetRideByBookingID(ctx context.Context, bookingID int64) (*model.Ride, error) {
	return nil, nil
}
func (f *fakeRiderProfileRepo) GetRidesByBookingID(ctx context.Context, bookingID int64) ([]model.Ride, error) {
	return nil, nil
}
func (f *fakeRiderProfileRepo) CancelRide(ctx context.Context, rideID int64) error { return nil }
func (f *fakeRiderProfileRepo) GetProfileByRiderID(ctx context.Context, riderID int64) (*model.RiderProfile, error) {
	return nil, nil
}
func (f *fakeRiderProfileRepo) UnassignRider(ctx context.Context, rideID int64) error {
	return nil
}
func (f *fakeRiderProfileRepo) IncrementRetry(ctx context.Context, rideID int64) error {
	return nil
}
func (f *fakeRiderProfileRepo) GetUnmatchedRidesForRetry(ctx context.Context, backoffMinutes int, maxRetries int) ([]model.Ride, error) {
	return nil, nil
}

func TestRiderHandlerUpdateProfilePassesUsualBranchAndLocation(t *testing.T) {
	repo := &fakeRiderProfileRepo{}
	handler := NewRiderHandler(service.NewRideService(repo, nil, nil, nil, nil))
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(12, model.RoleRider, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest("PUT", "/rider/profile", bytes.NewReader([]byte(`{"usual_branch_id":3,"usual_location_label":" Makati "}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.UpdateProfile), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if repo.updates["usual_branch_id"] != int64(3) {
		t.Fatalf("expected usual_branch_id update, got %#v", repo.updates)
	}
	if repo.updates["usual_location_label"] != "Makati" {
		t.Fatalf("expected trimmed usual_location_label, got %#v", repo.updates)
	}
}

func TestRiderHandlerUpdateProfileRejectsInvalidUsualBranch(t *testing.T) {
	repo := &fakeRiderProfileRepo{}
	handler := NewRiderHandler(service.NewRideService(repo, nil, nil, nil, nil))
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(12, model.RoleRider, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := httptest.NewRequest("PUT", "/rider/profile", bytes.NewReader([]byte(`{"usual_branch_id":0}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.UpdateProfile), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if repo.updates != nil {
		t.Fatalf("expected no repo update, got %#v", repo.updates)
	}
}

var _ = time.Time{}
