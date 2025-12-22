package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// mockBookingRepo implements minimal BookingRepository for testing UpdateStatus logic.
type mockBookingRepo struct {
	// record calls
	lastUpdateCalled bool
	lastBookingID    int64
	lastActorID      int64
	lastStatus       string

	// control errors
	updateErr error
}

func (m *mockBookingRepo) Create(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepo) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepo) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	// return a booking reflecting the requested status for assertion
	return &model.Booking{BookingID: bookingID, ClientID: 1, TherapistID: 2, Status: m.lastStatus}, nil
}
func (m *mockBookingRepo) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepo) Update(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepo) UpdateStatus(ctx context.Context, bookingID, actorID int64, status string) error {
	m.lastUpdateCalled = true
	m.lastBookingID = bookingID
	m.lastActorID = actorID
	m.lastStatus = status
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

func TestUpdateStatus_RolePermissions(t *testing.T) {
	ctx := context.Background()

	mock := &mockBookingRepo{}
	// service.NewBookingService requires promoRepo and db; pass nil for both for this unit test
	svc := NewBookingService(mock, nil, nil)

	// Therapist should be allowed to set 'confirmed'
	booking, err := svc.UpdateStatus(ctx, 10, 42, "therapist", &model.UpdateBookingStatusRequest{Status: "confirmed"})
	if err != nil {
		t.Fatalf("unexpected error for therapist allowed status: %v", err)
	}
	if !mock.lastUpdateCalled {
		t.Fatalf("expected repo.UpdateStatus to be called for therapist")
	}
	if mock.lastBookingID != 10 || mock.lastActorID != 42 || mock.lastStatus != "confirmed" {
		t.Fatalf("repo.UpdateStatus called with wrong args: %+v", mock)
	}
	if booking == nil || booking.Status != "confirmed" {
		t.Fatalf("expected returned booking to reflect status 'confirmed', got: %+v", booking)
	}

	// Reset
	mock.lastUpdateCalled = false

	// Client should NOT be allowed to set 'confirmed'
	_, err = svc.UpdateStatus(ctx, 11, 100, "client", &model.UpdateBookingStatusRequest{Status: "confirmed"})
	if err == nil {
		t.Fatalf("expected error when client sets therapist-only status")
	}

	// Admin may set any status (choose 'cancelled')
	mock.lastUpdateCalled = false
	booking, err = svc.UpdateStatus(ctx, 12, 7, "admin", &model.UpdateBookingStatusRequest{Status: "cancelled"})
	if err != nil {
		t.Fatalf("unexpected error for admin: %v", err)
	}
	if !mock.lastUpdateCalled || mock.lastBookingID != 12 || mock.lastActorID != 7 || mock.lastStatus != "cancelled" {
		t.Fatalf("admin update did not call repo correctly: %+v", mock)
	}

	// Unknown role should be rejected
	_, err = svc.UpdateStatus(ctx, 13, 99, "unknown", &model.UpdateBookingStatusRequest{Status: "pending"})
	if err == nil {
		t.Fatalf("expected error for unknown role")
	}

	// Propagate repository error
	mock.updateErr = errors.New("db failure")
	_, err = svc.UpdateStatus(ctx, 14, 1, "admin", &model.UpdateBookingStatusRequest{Status: "confirmed"})
	if err == nil || err.Error() != "db failure" {
		t.Fatalf("expected repo error to propagate, got: %v", err)
	}
}
