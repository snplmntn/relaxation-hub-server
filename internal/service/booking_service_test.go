package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
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
	tid := int64(2)
	return &model.Booking{BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}, nil
}
func (m *mockBookingRepo) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepo) Update(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepo) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	m.lastUpdateCalled = true
	m.lastBookingID = bookingID
	m.lastActorID = actorID
	m.lastStatus = status
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

func (m *mockBookingRepo) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return m.UpdateStatus(ctx, bookingID, actorID, role, status, cancelledBy, cancellationReason)
}

func (m *mockBookingRepo) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }

func (m *mockBookingRepo) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepo) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }

func (m *mockBookingRepo) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	tid := int64(2)
	return &model.Booking{BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}, nil
}

func (m *mockBookingRepo) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return []model.BookingEvent{}, nil
}

func (m *mockBookingRepo) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	return nil
}

// Implement GetRecentTherapistStruggleFlags to satisfy the BookingRepository interface in tests
func (m *mockBookingRepo) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

func (m *mockBookingRepo) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepo) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	tid := int64(2)
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}}, nil
}
func (m *mockBookingRepo) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	tid := int64(2)
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}}, nil
}
func (m *mockBookingRepo) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	tid := int64(2)
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1, ReferenceCode: &referenceCode, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}}, nil
}
func (m *mockBookingRepo) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	tid := int64(2)
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1, ReferenceCode: &referenceCode, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}}, nil
}
func (m *mockBookingRepo) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepo) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepo) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepo) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return map[int64]int{}, nil
}
func (m *mockBookingRepo) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error { return nil }
func (m *mockBookingRepo) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error { return nil }
func (m *mockBookingRepo) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepo) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockBookingRepo) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockBookingRepo) ListUpcomingBookingsForReminder(ctx context.Context, windowStart, windowEnd time.Time, eventTypeExclude string) ([]model.Booking, error) {
    return nil, nil
}
func (m *mockBookingRepo) UnassignTherapist(ctx context.Context, bookingID int64) error {
    return nil
}
func (m *mockBookingRepo) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
    return nil
}
func (m *mockBookingRepo) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
    return 0, nil
}
func (m *mockBookingRepo) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
    return &repository.ClientBookingStats{}, nil
}
func (m *mockBookingRepo) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) { return &repository.AccountingSummary{}, nil }
func (m *mockBookingRepo) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) { return nil, nil }
func (m *mockBookingRepo) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	m.lastUpdateCalled = true
	m.lastBookingID = bookingID
	m.lastStatus = "completed"
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}
func (m *mockBookingRepo) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	return m.CompleteBooking(ctx, bookingID, earnings, fee, actualEnd)
}
func (m *mockBookingRepo) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int) ([]repository.BookingDetailsResult, int, error) { return nil, 0, nil }


func TestUpdateStatus_RolePermissions(t *testing.T) {
	ctx := context.Background()

	mock := &mockBookingRepo{}
	// service.NewBookingService requires promoRepo, db and queueRepo
	svc := NewBookingService(mock, nil, nil, &nilQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil)

	// Therapist should be allowed to set 'on_the_way'
	booking, err := svc.UpdateStatus(ctx, 10, 42, "therapist", &model.UpdateBookingStatusRequest{Status: "on_the_way"})
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	if !mock.lastUpdateCalled {
		t.Fatalf("expected repo.UpdateStatus to be called for therapist")
	}
	if mock.lastBookingID != 10 || mock.lastActorID != 42 || mock.lastStatus != "on_the_way" {
		t.Fatalf("mock did not receive expected values: got bookingID=%d actorID=%d status=%s", mock.lastBookingID, mock.lastActorID, mock.lastStatus)
	}
	if booking == nil || booking.Status != "on_the_way" {
		t.Fatalf("expected returned booking to reflect status 'on_the_way', got: %+v", booking)
	}

	// Reset
	mock.lastUpdateCalled = false

	// Client should NOT be allowed to set 'on_the_way'
	_, err = svc.UpdateStatus(ctx, 11, 100, "client", &model.UpdateBookingStatusRequest{Status: "on_the_way"})
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
	_, err = svc.UpdateStatus(ctx, 14, 1, "admin", &model.UpdateBookingStatusRequest{Status: "on_the_way"})
	if err == nil || err.Error() != "db failure" {
		t.Fatalf("expected repo error to propagate, got: %v", err)
	}
}
