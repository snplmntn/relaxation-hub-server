package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type mockBookingRepoStart struct{
    booking *model.Booking
    lastEventType string
    lastActorID *int64
}

func (m *mockBookingRepoStart) UpdatePayoutReference(ctx context.Context, bookingIDs []int64, payoutID int64) error { return nil }
func (m *mockBookingRepoStart) UpdatePayoutReferenceTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64, payoutID int64) error { return nil }
func (m *mockBookingRepoStart) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoStart) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoStart) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoStart) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error { return nil }
func (m *mockBookingRepoStart) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) { return nil, pgx.ErrNoRows }
func (m *mockBookingRepoStart) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoStart) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoStart) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }
func (m *mockBookingRepoStart) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoStart) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoStart) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) { return m.booking, nil }
func (m *mockBookingRepoStart) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) { return m.booking, nil }
func (m *mockBookingRepoStart) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoStart) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoStart) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockBookingRepoStart) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error { return nil }
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
func (m *mockBookingRepoStart) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.booking}, nil
}
func (m *mockBookingRepoStart) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
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
func (m *mockBookingRepoStart) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	return nil
}
func (m *mockBookingRepoStart) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	return nil
}
func (m *mockBookingRepoStart) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoStart) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockBookingRepoStart) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockBookingRepoStart) ListUpcomingBookingsForReminder(ctx context.Context, windowStart, windowEnd time.Time, eventTypeExclude string) ([]model.Booking, error) {
    return nil, nil
}
func (m *mockBookingRepoStart) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
    return nil
}
func (m *mockBookingRepoStart) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
    return nil
}
func (m *mockBookingRepoStart) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
    return 0, nil
}
func (m *mockBookingRepoStart) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
    return &repository.ClientBookingStats{}, nil
}
func (m *mockBookingRepoStart) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) { return &repository.AccountingSummary{}, nil }
func (m *mockBookingRepoStart) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) { return nil, nil }
func (m *mockBookingRepoStart) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error { return nil }
func (m *mockBookingRepoStart) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error { return nil }
func (m *mockBookingRepoStart) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int) ([]repository.BookingDetailsResult, int, error) { return nil, 0, nil }

func TestStartSession_SucceedsWhenArrived(t *testing.T) {
    now := time.Now()
    b := &model.Booking{BookingID: 1, ClientID: 10, Status: "arrived", TherapistArrivedAt: &now}
    mock := &mockBookingRepoStart{booking: b}
    svc := NewBookingService(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

    got, err := svc.StartSession(context.Background(), 1, 10, "client", nil)
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
    svc := NewBookingService(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

    _, err := svc.StartSession(context.Background(), 2, 20, "client", nil)
    if err == nil { t.Fatalf("expected error when therapist not arrived") }
}
