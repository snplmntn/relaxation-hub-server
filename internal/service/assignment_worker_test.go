package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// minimal mocks
type mockQueue struct {
	items []repository.QueueItem
	inc   map[int64]int
}

func (m *mockQueue) Enqueue(ctx context.Context, bookingID int64) error { return nil }
func (m *mockQueue) DequeueBatch(ctx context.Context, limit int) ([]repository.QueueItem, error) {
	if len(m.items) == 0 {
		return nil, nil
	}
	out := m.items
	m.items = nil
	return out, nil
}
func (m *mockQueue) Remove(ctx context.Context, bookingID int64) error { return nil }
func (m *mockQueue) IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error {
	if m.inc == nil { m.inc = map[int64]int{} }
	m.inc[bookingID] = attempts
	return nil
}

type mockBookingRepoAW struct {
	bookings map[int64]*mockBooking
	assignErr error
}

type mockBooking struct {
	ClientID int64
	ServiceID *int64
}

func (m *mockBookingRepoAW) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAW) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAW) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) { return nil, nil }
func (m *mockBookingRepoAW) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoAW) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAW) UpdateStatus(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockBookingRepoAW) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return nil
}
func (m *mockBookingRepoAW) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoAW) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	if m.assignErr != nil { return m.assignErr }
	return nil
}
func (m *mockBookingRepoAW) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoAW) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }

func (m *mockBookingRepoAW) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	b, ok := m.bookings[bookingID]
	if !ok { return nil, errors.New("not found") }
	return &model.Booking{BookingID: bookingID, ClientID: b.ClientID, ServiceID: b.ServiceID, PressurePref: ""}, nil
}

func (m *mockBookingRepoAW) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

func (m *mockBookingRepoAW) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return []model.BookingEvent{}, nil
}

func (m *mockBookingRepoAW) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockBookingRepoAW) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID}}, nil
}
func (m *mockBookingRepoAW) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID}}, nil
}
func (m *mockBookingRepoAW) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1}}, nil
}
func (m *mockBookingRepoAW) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1}}, nil
}
func (m *mockBookingRepoAW) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoAW) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoAW) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return map[int64]int{}, nil
}
func (m *mockBookingRepoAW) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error { return nil }
func (m *mockBookingRepoAW) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error { return nil }
func (m *mockBookingRepoAW) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoAW) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockBookingRepoAW) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}

// mock match service
type mockMatch struct {
	result []model.TherapistProfile
}
func (m *mockMatch) FindAvailableTherapistsForService(ctx context.Context, clientID int64, serviceID int64, genderPref string, pressurePref string) ([]model.TherapistProfile, error) {
	return m.result, nil
}
func (m *mockMatch) FindNearbyAvailableTherapists(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPref string, pressurePref string) ([]model.TherapistProfile, error) { return nil, nil }

// mock notif service
type mockNotif struct { called bool }
func (m *mockNotif) Create(ctx context.Context, req *model.CreateNotificationRequest) (*model.Notification, error) { m.called = true; return nil, nil }

// mock offer repo
type mockOfferRepo struct {
    active []model.BookingOffer
    past []model.BookingOffer
    expired []model.BookingOffer
}
func (m *mockOfferRepo) Create(ctx context.Context, offer *model.BookingOffer) error { return nil }
func (m *mockOfferRepo) GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return m.active, nil }
func (m *mockOfferRepo) GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepo) UpdateStatus(ctx context.Context, offerID int64, status string) error { return nil }
func (m *mockOfferRepo) UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error { return nil }
func (m *mockOfferRepo) ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return m.expired, nil }
func (m *mockOfferRepo) ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error) { return m.expired, nil }
func (m *mockOfferRepo) GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return m.past, nil }
func (m *mockOfferRepo) GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) {
	var out []model.BookingOffer
	for _, o := range m.active {
		if o.TherapistID == therapistID {
			out = append(out, o)
		}
	}
	return out, nil
}

func TestAssignmentWorker_BackoffAndRetry(t *testing.T) {
	// Set up mocks
	q := &mockQueue{items: []repository.QueueItem{{BookingID: 1, Attempts: 0}}}
	br := &mockBookingRepoAW{bookings: map[int64]*mockBooking{1: {ClientID: 10, ServiceID: func() *int64 {v:=int64(5); return &v}()}}}
	mm := &mockMatch{result: nil} // first round: no therapists
	worker := &AssignmentWorker{
		queueRepo: q,
		bookingRepo: br,
		paymentRepo: nil,
        offerRepo: &mockOfferRepo{},
		matchService: mm,
		notificationService: &NotificationService{}, // not used directly here
		pollInterval: time.Millisecond,
		batchSize: 10,
		maxAttempts: 2,
		baseBackoff: time.Millisecond,
	}

	// Run one processOnce; should increment attempt
	worker.processOnce(context.Background())
	if q.inc[1] == 0 {
		t.Fatalf("expected attempt increment recorded")
	}
}
