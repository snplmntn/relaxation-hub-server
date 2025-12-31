package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type mockBookingRepoStart struct{
    booking *model.Booking
    lastEventType string
    lastActorID *int64
}
func (m *mockBookingRepoStart) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoStart) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error { return nil }
func (m *mockBookingRepoStart) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) { return nil, pgx.ErrNoRows }
func (m *mockBookingRepoStart) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoStart) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoStart) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }
func (m *mockBookingRepoStart) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoStart) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoStart) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) { return m.booking, nil }
func (m *mockBookingRepoStart) UpdateStatus(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockBookingRepoStart) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoStart) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) { return map[int64]bool{}, nil }
func (m *mockBookingRepoStart) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) { return nil, nil }
func (m *mockBookingRepoStart) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
    m.lastEventType = eventType
    if actorID != nil {
        v := *actorID
        m.lastActorID = &v
    } else {
        m.lastActorID = nil
    }
    return nil
}
func (m *mockBookingRepoStart) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.booking}, nil
}
func (m *mockBookingRepoStart) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.booking}, nil
}
func (m *mockBookingRepoStart) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoStart) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoStart) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoStart) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return map[int64]int{}, nil
}

func TestStartSession_SucceedsWhenArrived(t *testing.T) {
    now := time.Now()
    b := &model.Booking{BookingID: 1, ClientID: 10, Status: "arrived", TherapistArrivedAt: &now}
    mock := &mockBookingRepoStart{booking: b}
    svc := NewBookingService(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil)

    got, err := svc.StartSession(context.Background(), 1, 10, "client")
    if err != nil { t.Fatalf("unexpected err: %v", err) }
    if got.BookingID != 1 { t.Fatalf("expected booking 1, got %v", got.BookingID) }
    if mock.lastEventType != "client_confirm_start" {
        t.Fatalf("expected client_confirm_start event, got %v", mock.lastEventType)
    }
    if mock.lastActorID == nil || *mock.lastActorID != 10 {
        t.Fatalf("expected actor id 10 recorded, got %v", mock.lastActorID)
    }
}

func TestStartSession_FailsWhenNotArrived(t *testing.T) {
    b := &model.Booking{BookingID: 2, ClientID: 20, Status: "assigned", TherapistArrivedAt: nil}
    mock := &mockBookingRepoStart{booking: b}
    svc := NewBookingService(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil)

    _, err := svc.StartSession(context.Background(), 2, 20, "client")
    if err == nil { t.Fatalf("expected error when therapist not arrived") }
}
