package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// --- Mocks ---

type mockBookingRepoCW struct {
	inProgress []model.Booking
	completed  map[int64]model.Booking
}

func (m *mockBookingRepoCW) UpdatePayoutReference(ctx context.Context, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoCW) UpdatePayoutReferenceTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoCW) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) {
	return nil, nil
}

func (m *mockBookingRepoCW) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) {
	return m.inProgress, nil
}
func (m *mockBookingRepoCW) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	if m.completed == nil {
		m.completed = make(map[int64]model.Booking)
	}
	m.completed[bookingID] = model.Booking{
		BookingID:         bookingID,
		Status:            "completed",
		TherapistEarnings: earnings,
		PlatformFee:       fee,
		ActualEnd:         &actualEnd,
	}
	return nil
}
func (m *mockBookingRepoCW) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	return nil
}

// Stubs for BookingRepository
func (m *mockBookingRepoCW) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoCW) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoCW) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoCW) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	return nil
}
func (m *mockBookingRepoCW) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return nil
}
func (m *mockBookingRepoCW) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	return nil
}
func (m *mockBookingRepoCW) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoCW) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoCW) UpdateAdmin(ctx context.Context, booking *model.Booking) error {
	return nil
}

func (m *mockBookingRepoCW) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	return nil
}
func (m *mockBookingRepoCW) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	return m.CompleteBooking(ctx, bookingID, earnings, fee, actualEnd)
}
func (m *mockBookingRepoCW) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoCW) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	return nil
}
func (m *mockBookingRepoCW) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoCW) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoCW) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
	return nil
}
func (m *mockBookingRepoCW) ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockBookingRepoCW) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockBookingRepoCW) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) {
	return nil, nil
}
func (m *mockBookingRepoCW) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) {
	return nil, nil
}

// mockServiceRepoCW
type mockServiceRepoCW struct {
	services map[int64]*model.Service
}

func (m *mockServiceRepoCW) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	if m.services != nil {
		if s, ok := m.services[serviceID]; ok {
			return s, nil
		}
	}
	return nil, nil // Not found
}
func (m *mockServiceRepoCW) Create(ctx context.Context, svc *model.Service) error { return nil }
func (m *mockServiceRepoCW) GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error) {
	var result []model.Service
	if m.services != nil {
		for _, id := range ids {
			if s, ok := m.services[id]; ok {
				result = append(result, *s)
			}
		}
	}
	return result, nil
}
func (m *mockServiceRepoCW) ListActive(ctx context.Context) ([]model.Service, error) { return nil, nil }
func (m *mockServiceRepoCW) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoCW) ListPopular(ctx context.Context) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoCW) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoCW) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	return nil
}
func (m *mockServiceRepoCW) Delete(ctx context.Context, id int64) error { return nil }

// mockPaymentRepoCW
type mockPaymentRepoCW struct {
	payments map[int64]*model.Payment
}

func (m *mockPaymentRepoCW) GetByBookingID(ctx context.Context, bookingID int64) (*model.Payment, error) {
	if m.payments != nil {
		if p, ok := m.payments[bookingID]; ok {
			return p, nil
		}
	}
	return nil, nil
}
func (m *mockPaymentRepoCW) GetByBookingIDBatch(ctx context.Context, bookingIDs []int64) (map[int64]*model.Payment, error) {
	result := make(map[int64]*model.Payment)
	if m.payments != nil {
		for _, id := range bookingIDs {
			if p, ok := m.payments[id]; ok {
				result[id] = p
			}
		}
	}
	return result, nil
}
func (m *mockPaymentRepoCW) Create(ctx context.Context, p *model.Payment) error { return nil }
func (m *mockPaymentRepoCW) GetOrCreateByBookingID(ctx context.Context, bookingID int64, amount float64, gateway string) (*model.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepoCW) UpdateStatus(ctx context.Context, bookingID int64, status string, transactionID *string, webhookID *string) error {
	return nil
}
func (m *mockPaymentRepoCW) UpdateProofURL(ctx context.Context, bookingID int64, proofURL string) error {
	return nil
}
func (m *mockPaymentRepoCW) Verify(ctx context.Context, bookingID int64, verifiedBy int64, notes *string) error {
	return nil
}
func (m *mockPaymentRepoCW) Reject(ctx context.Context, bookingID int64, rejectedBy int64, notes *string) error {
	return nil
}
func (m *mockPaymentRepoCW) ClearProof(ctx context.Context, bookingID int64) error { return nil }

// --- Tests ---

func TestCompletionWorker_ProcessOnce_CalculatesCommission(t *testing.T) {
	// Mock broadcaster
	originalBroadcast := broadcaster.BroadcastToUser
	defer func() { broadcaster.BroadcastToUser = originalBroadcast }()
	broadcaster.BroadcastToUser = func(userID int64, event string, data interface{}) error { return nil }

	// Setup
	serviceID := int64(1)
	baseComm := 300.0  // 300 PHP commission
	basePrice := 500.0 // 500 PHP base price
	duration := 60
	svc := &model.Service{
		ServiceID:           serviceID,
		BasePrice:           basePrice,
		DurationMinutes:     duration,
		TherapistCommission: &baseComm,
	}

	finalTotal := 750.0 // 50% increase (e.g. 30 mins extension)
	bookingID := int64(100)
	now := time.Now()
	// Booking extended to 90 mins (60 + 30)
	// Base commission = 300
	// Extra time = 30 mins
	// Rate per min = 500 / 60 = 8.333
	// Extra cost = 8.333 * 30 = 250
	// Comm ratio = 300 / 500 = 0.6
	// Extra comm = 250 * 0.6 = 150
	// Total comm = 300 + 150 = 450

	start := now.Add(-100 * time.Minute)

	b := model.Booking{
		BookingID:       bookingID,
		ServiceID:       &serviceID,
		DurationMinutes: 90, // Extended!
		ScheduledStart:  &start,
		ActualStart:     &start,
		FinalTotal:      &finalTotal,
		Status:          "in_progress",
	}

	// Payment 'paid'
	p := model.Payment{
		PaymentID: 1,
		BookingID: bookingID,
		Status:    "paid",
		Amount:    750,
	}

	repoB := &mockBookingRepoCW{
		inProgress: []model.Booking{b},
	}
	repoS := &mockServiceRepoCW{
		services: map[int64]*model.Service{serviceID: svc},
	}
	repoP := &mockPaymentRepoCW{
		payments: map[int64]*model.Payment{bookingID: &p},
	}

	worker := NewCompletionWorker(nil, repoB, repoP, repoS, nil, nil, nil)

	// Act
	worker.processOnce(context.Background())

	// Assert
	if len(repoB.completed) != 1 {
		t.Fatalf("expected 1 completed booking, got %d", len(repoB.completed))
	}
	completedB := repoB.completed[bookingID]
	if completedB.Status != "completed" {
		t.Errorf("expected status completed, got %s", completedB.Status)
	}

	if completedB.TherapistEarnings == nil {
		t.Fatal("therapist earnings not set")
	}
	if completedB.PlatformFee == nil {
		t.Fatal("platform fee not set")
	}

	expectedEarnings := 450.0
	// 500 base price, 300 commission.
	// 90 mins total duration.
	// Extra 30 mins.
	// Rate = 500/60 = 8.3333...
	// Extra cost = 30 * 8.3333 = 250.
	// Ratio = 300/500 = 0.6.
	// Extra comm = 250 * 0.6 = 150.
	// Total comm = 300 + 150 = 450.
	// Precision might vary (float64).

	if *completedB.TherapistEarnings != expectedEarnings {
		t.Errorf("expected earnings %.2f, got %.2f", expectedEarnings, *completedB.TherapistEarnings)
	}

	expectedFee := finalTotal - expectedEarnings // 750 - 450 = 300
	if *completedB.PlatformFee != expectedFee {
		t.Errorf("expected fee %.2f, got %.2f", expectedFee, *completedB.PlatformFee)
	}
}

func (m *mockBookingRepoCW) ListAllEvents(ctx context.Context, params repository.ListAllEventsParams) ([]model.BookingEvent, int, error) {
	return nil, 0, nil
}
