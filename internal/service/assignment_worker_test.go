package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// --- Mocks ---

// mockQueue
type mockQueue struct {
	items []repository.QueueItem
	inc   map[int64]int
}

func (m *mockQueue) Enqueue(ctx context.Context, bookingID int64) error              { return nil }
func (m *mockQueue) EnqueueTx(ctx context.Context, tx pgx.Tx, bookingID int64) error { return nil }
func (m *mockQueue) EnqueueManyTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64) error {
	return nil
}
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
	if m.inc == nil {
		m.inc = map[int64]int{}
	}
	m.inc[bookingID] = attempts
	return nil
}
func (m *mockQueue) UpdateWorkflowState(ctx context.Context, bookingID int64, state string, data map[string]interface{}) error {
	return nil
}

// mockBookingRepoAW
type mockBookingRepoAW struct {
	bookings map[int64]*mockBooking
	groups   map[int64][]model.Booking
}

func (m *mockBookingRepoAW) UpdatePayoutReference(ctx context.Context, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoAW) UpdatePayoutReferenceTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoAW) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) {
	res := make(map[int64]*repository.BookingDetailsResult)
	for _, id := range bookingIDs {
		details, err := m.GetBookingWithDetailsUnsafe(ctx, id)
		if err == nil {
			res[id] = details
		}
	}
	return res, nil
}
func (m *mockBookingRepoAW) FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*repository.BookingDetailsResult, error) {
	return nil, nil
}

type mockBooking struct {
	ClientID        int64
	ServiceID       *int64
	DurationMinutes int
	GroupID         *int64
	ScheduledStart  *time.Time
	Service         *model.Service
}

func (m *mockBookingRepoAW) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAW) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoAW) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	if b, ok := m.bookings[bookingID]; ok {
		return &model.Booking{
			BookingID:      bookingID,
			ClientID:       b.ClientID,
			ServiceID:      b.ServiceID,
			Status:         "pending",
			GroupID:        b.GroupID,
			ScheduledStart: b.ScheduledStart,
		}, nil
	}
	return nil, nil // Not found
}
func (m *mockBookingRepoAW) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAW) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	return nil
}
func (m *mockBookingRepoAW) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return nil
}
func (m *mockBookingRepoAW) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	return nil
}
func (m *mockBookingRepoAW) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoAW) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoAW) UpdateAdmin(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoAW) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	if b, ok := m.bookings[bookingID]; ok {
		return &model.Booking{
			BookingID:       bookingID,
			ClientID:        b.ClientID,
			ServiceID:       b.ServiceID,
			DurationMinutes: b.DurationMinutes,
			Status:          "pending",
			GroupID:         b.GroupID,
			ScheduledStart:  b.ScheduledStart,
		}, nil
	}
	return nil, nil
}
func (m *mockBookingRepoAW) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) {
	return m.GetByBookingID(ctx, bookingID)
}
func (m *mockBookingRepoAW) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	if g, ok := m.groups[groupID]; ok {
		return g, nil
	}
	return nil, nil
}
func (m *mockBookingRepoAW) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	res := make(map[int64][]model.Booking)
	for _, gid := range groupIDs {
		if g, ok := m.groups[gid]; ok {
			res[gid] = g
		}
	}
	return res, nil
}
func (m *mockBookingRepoAW) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockBookingRepoAW) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	if b, ok := m.bookings[bookingID]; ok {
		scStart := b.ScheduledStart
		if scStart == nil {
			t := time.Now()
			scStart = &t
		}
		return &repository.BookingDetailsResult{
			Booking: &model.Booking{
				BookingID:       bookingID,
				ClientID:        b.ClientID,
				ServiceID:       b.ServiceID,
				DurationMinutes: b.DurationMinutes,
				Status:          "pending",
				ScheduledStart:  scStart,
				GroupID:         b.GroupID,
			},
			Service: b.Service,
		}, nil
	}
	return nil, pgx.ErrNoRows
}
func (m *mockBookingRepoAW) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	return nil
}
func (m *mockBookingRepoAW) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	return nil
}
func (m *mockBookingRepoAW) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) ListDueInProgressBookings(ctx context.Context, now time.Time, limit int) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoAW) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoAW) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
	return nil
}
func (m *mockBookingRepoAW) ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockBookingRepoAW) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockBookingRepoAW) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) {
	return nil, nil
}
func (m *mockBookingRepoAW) HasAssignedOutboundRiderCoverage(ctx context.Context, bookingID int64) (bool, error) {
	return true, nil
}

func (m *mockBookingRepoAW) RevertOnTheWayToAssigned(ctx context.Context, bookingID, actorID int64) (*repository.RevertOnTheWayToAssignedResult, error) {
	return nil, nil
}

func (m *mockBookingRepoAW) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoAW) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoAW) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}

// mockMatch
type mockMatch struct {
	result []model.TherapistProfile
}

func (m *mockMatch) FindAvailableTherapistsForService(ctx context.Context, clientID int64, serviceID int64, genderPref string, pressurePref string) ([]model.TherapistProfile, error) {
	return m.result, nil
}
func (m *mockMatch) FindNearbyAvailableTherapists(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPref string, pressurePref string) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockMatch) FindAvailableTherapistsForServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPref string, pressurePref string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error) {
	return m.result, nil
}

// mockOfferRepo
type mockOfferRepo struct {
	active  []model.BookingOffer
	expired []model.BookingOffer
}

func (m *mockOfferRepo) Create(ctx context.Context, offer *model.BookingOffer) error {
	m.active = append(m.active, *offer)
	return nil
}
func (m *mockOfferRepo) CreateTx(ctx context.Context, tx pgx.Tx, offer *model.BookingOffer) error {
	return m.Create(ctx, offer)
}
func (m *mockOfferRepo) GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	return m.active, nil
}
func (m *mockOfferRepo) GetActiveOffersBatch(ctx context.Context, bookingIDs []int64) (map[int64][]model.BookingOffer, error) {
	return make(map[int64][]model.BookingOffer), nil
}
func (m *mockOfferRepo) GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error) {
	return nil, nil
}
func (m *mockOfferRepo) UpdateStatus(ctx context.Context, offerID int64, status string) error {
	return nil
}
func (m *mockOfferRepo) UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error {
	return nil
}
func (m *mockOfferRepo) ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	return m.expired, nil
}
func (m *mockOfferRepo) ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error) {
	return m.expired, nil
}
func (m *mockOfferRepo) GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	return nil, nil
}
func (m *mockOfferRepo) CancelOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	return nil, nil
}
func (m *mockOfferRepo) GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) {
	return nil, nil
}

// mockServiceRepoAW
type mockServiceRepoAW struct {
	svc *model.Service
}

func (m *mockServiceRepoAW) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	return m.svc, nil
}
func (m *mockServiceRepoAW) Create(ctx context.Context, svc *model.Service) error { return nil }
func (m *mockServiceRepoAW) GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoAW) ListActive(ctx context.Context) ([]model.Service, error) { return nil, nil }
func (m *mockServiceRepoAW) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoAW) ListPopular(ctx context.Context) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoAW) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoAW) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	return nil
}
func (m *mockServiceRepoAW) Delete(ctx context.Context, id int64) error { return nil }

// mockNotificationRepo
type mockNotificationRepo struct {
	created *model.Notification
}

func (m *mockNotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	m.created = n
	return nil
}

func (m *mockNotificationRepo) CreateMany(ctx context.Context, notifications []*model.Notification) error {
	if len(notifications) > 0 {
		m.created = notifications[len(notifications)-1]
	}
	return nil
}

func (m *mockNotificationRepo) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Notification, int, error) {
	return nil, 0, nil
}

func (m *mockNotificationRepo) ListByUserKeyset(ctx context.Context, userID int64, cursor *model.KeysetCursor, limit int) ([]model.Notification, error) {
	return nil, nil
}
func (m *mockNotificationRepo) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	return nil
}
func (m *mockNotificationRepo) MarkAllAsRead(ctx context.Context, userID int64) error {
	return nil
}
func (m *mockNotificationRepo) CountUnread(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}
func (m *mockNotificationRepo) CountUnreadByUser(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}
func (m *mockNotificationRepo) DeleteOld(ctx context.Context, olderThan time.Duration) error {
	return nil
}

func TestNextAssignmentPollDelay_IdleBackoffSequence(t *testing.T) {
	current := 5 * time.Second
	want := []time.Duration{
		10 * time.Second,
		20 * time.Second,
		40 * time.Second,
		60 * time.Second,
		60 * time.Second,
	}

	for _, expected := range want {
		current = nextAssignmentPollDelay(0, current)
		if current != expected {
			t.Fatalf("expected next delay %s, got %s", expected, current)
		}
	}
}

func TestNextAssignmentPollDelay_ResetOnWork(t *testing.T) {
	if got := nextAssignmentPollDelay(1, 60*time.Second); got != 5*time.Second {
		t.Fatalf("expected delay reset to 5s after work, got %s", got)
	}
	if got := nextAssignmentPollDelay(12, 40*time.Second); got != 5*time.Second {
		t.Fatalf("expected any processed count to reset to 5s, got %s", got)
	}
}

func TestNextAssignmentPollDelay_BelowMinimum(t *testing.T) {
	if got := nextAssignmentPollDelay(0, time.Second); got != 5*time.Second {
		t.Fatalf("expected delay below minimum to normalize to 5s, got %s", got)
	}
}

func TestAssignmentWorker_CancelStopsLongTimerPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if waitForNextAssignmentPoll(ctx, 60*time.Second) {
		t.Fatal("expected canceled context to stop poll wait")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected cancellation to return promptly, took %s", elapsed)
	}
}

func TestAssignmentWorker_DequeueErrorBacksOff(t *testing.T) {
	queue := &errorQueue{err: errors.New("dequeue failed")}
	worker := NewAssignmentWorker(
		&mockDB{},
		queue,
		&mockBookingRepoAW{},
		nil,
		&mockOfferRepo{},
		&mockServiceRepoAW{},
		nil,
		&mockTherapistRepoForTest{},
		&mockMatch{},
		nil,
		nil,
	)
	worker.pollInterval = time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	worker.run(ctx)

	calls := atomic.LoadInt32(&queue.calls)
	if calls < 2 {
		t.Fatalf("expected repeated dequeue attempts, got %d", calls)
	}
	if calls > 20 {
		t.Fatalf("expected dequeue errors to back off instead of spin, got %d calls", calls)
	}
}

type errorQueue struct {
	err   error
	calls int32
}

func (q *errorQueue) Enqueue(ctx context.Context, bookingID int64) error              { return nil }
func (q *errorQueue) EnqueueTx(ctx context.Context, tx pgx.Tx, bookingID int64) error { return nil }
func (q *errorQueue) EnqueueManyTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64) error {
	return nil
}
func (q *errorQueue) DequeueBatch(ctx context.Context, limit int) ([]repository.QueueItem, error) {
	atomic.AddInt32(&q.calls, 1)
	return nil, q.err
}
func (q *errorQueue) Remove(ctx context.Context, bookingID int64) error { return nil }
func (q *errorQueue) IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error {
	return nil
}
func (q *errorQueue) UpdateWorkflowState(ctx context.Context, bookingID int64, state string, data map[string]interface{}) error {
	return nil
}

// Tests
func TestAssignmentWorker_BackoffAndRetry(t *testing.T) {
	// Set up mocks
	q := &mockQueue{items: []repository.QueueItem{{BookingID: 1, Attempts: 0}}}
	br := &mockBookingRepoAW{bookings: map[int64]*mockBooking{1: {ClientID: 10, ServiceID: func() *int64 { v := int64(5); return &v }()}}}
	mm := &mockMatch{result: nil} // first round: no therapists
	worker := NewAssignmentWorker(
		&mockDB{}, // db
		q,
		br,
		nil, // payment
		&mockOfferRepo{},
		&mockServiceRepoAW{},        // service
		nil,                         // area
		&mockTherapistRepoForTest{}, // therapist (injected)
		mm,                          // match
		&NotificationService{},      // notif
		nil,                         // ops
	)

	// Hack: override NotificationService if needed, but here no match found so it won't be called.

	// Run one processOnce; should increment attempt
	worker.processOnce(context.Background())
	if q.inc[1] == 0 {
		t.Fatalf("expected attempt increment recorded")
	}
}

func TestAssignmentWorker_CalculatesEstimatedEarnings(t *testing.T) {
	// Setup
	serviceID := int64(1)
	baseComm := 300.0
	basePrice := 500.0

	svc := &model.Service{
		ServiceID:           serviceID,
		BasePrice:           basePrice,
		TherapistCommission: &baseComm,
		DurationMinutes:     60,
	}
	bID := int64(10)
	mockSvcRepo := &mockServiceRepoAW{svc: svc}

	q := &mockQueue{items: []repository.QueueItem{{BookingID: bID, Attempts: 0}}}

	sIDPtr := &serviceID
	br := &mockBookingRepoAW{bookings: map[int64]*mockBooking{bID: {ClientID: 10, ServiceID: sIDPtr, DurationMinutes: 60, Service: svc}}}

	// Mock match to return 1 therapist
	tProfile := model.TherapistProfile{TherapistID: 99, Status: "active", AcceptAssignments: true}
	mm := &mockMatch{result: []model.TherapistProfile{tProfile}}

	// Mock notification
	mockNotifRepo := &mockNotificationRepo{}
	notifService := NewNotificationService(mockNotifRepo, nil, nil)

	// Mock broadcaster to avoid panic
	originalBroadcast := broadcaster.BroadcastToUser
	defer func() { broadcaster.BroadcastToUser = originalBroadcast }()
	broadcaster.BroadcastToUser = func(userID int64, event string, data interface{}) error { return nil }

	// Construct worker
	worker := NewAssignmentWorker(
		&mockDB{}, q, br, nil, &mockOfferRepo{}, mockSvcRepo, nil, &mockTherapistRepoForTest{}, mm, notifService, nil,
	)

	// Act
	worker.processOnce(context.Background())

	// Assert
	if mockNotifRepo.created == nil {
		t.Fatal("notification not sent")
	}
	// Check Data
	// Data is []byte, explicitly unmarshal
	var dataMap map[string]any
	// using "encoding/json"
	if err := json.Unmarshal(mockNotifRepo.created.Data, &dataMap); err != nil {
		t.Fatalf("failed to unmarshal notification data: %v", err)
	}

	if val, ok := dataMap["estimated_earnings"]; ok {
		valFloat, ok2 := val.(float64)
		if !ok2 {
			t.Errorf("estimated_earnings not float64: %T", val)
		}
		if valFloat != 300.0 {
			t.Errorf("expected 300.0 earnings, got %f", valFloat)
		}
	} else {
		t.Error("estimated_earnings missing from notification data")
	}
}

func TestAssignmentWorker_SequentialBundle(t *testing.T) {
	// Setup Group Bookings (Sequential: different times)
	// B1: 10:00, B2: 11:00
	t1 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	gid := int64(888)

	b1 := model.Booking{
		BookingID: 201, ClientID: 1, ServiceID: ptrInt64(1), DurationMinutes: 60, ScheduledStart: &t1, GroupID: &gid, Status: "pending", FinalTotal: func() *float64 { v := 150.0; return &v }(),
	}
	b2 := model.Booking{
		BookingID: 202, ClientID: 1, ServiceID: ptrInt64(1), DurationMinutes: 60, ScheduledStart: &t2, GroupID: &gid, Status: "pending", FinalTotal: func() *float64 { v := 150.0; return &v }(),
	}

	// Mocks
	q := &mockQueue{
		items: []repository.QueueItem{
			{BookingID: 201, WorkflowState: "init"},
		},
	}
	br := &mockBookingRepoAW{
		bookings: map[int64]*mockBooking{
			201: {ClientID: 1, ServiceID: ptrInt64(1), DurationMinutes: 60, GroupID: &gid, ScheduledStart: &t1},
			202: {ClientID: 1, ServiceID: ptrInt64(1), DurationMinutes: 60, GroupID: &gid, ScheduledStart: &t2},
		},
		groups: map[int64][]model.Booking{
			gid: {b1, b2},
		},
	}

	// Therapist matching returns T-55 for both
	mm := &mockMatch{
		result: []model.TherapistProfile{{TherapistID: 55}},
	}

	tr := &mockTherapistRepoForTest{} // Returns true for locking
	or := &mockOfferRepo{}

	// Broadcaster mock
	originalBroadcast := broadcaster.BroadcastToUser
	defer func() { broadcaster.BroadcastToUser = originalBroadcast }()
	broadcaster.BroadcastToUser = func(userID int64, event string, data interface{}) error { return nil }

	// Construct worker
	worker := NewAssignmentWorker(&mockDB{}, q, br, nil, or, nil, nil, tr, mm, nil, nil)

	// Act: Process B1 (Leader)
	// it should init -> sequence_bundling -> offering in one tick because of state loop
	worker.processOnce(context.Background())

	// Assertions
	if len(or.active) != 1 {
		t.Fatalf("Expected 1 bundle offer, got %d", len(or.active))
	}
	offer := or.active[0]
	if !offer.IsBundle {
		t.Error("Expected IsBundle=true")
	}
	if len(offer.Items) != 2 {
		t.Errorf("Expected 2 items in bundle, got %d", len(offer.Items))
	}
	if offer.TherapistID != 55 {
		t.Errorf("Expected therapist 55, got %d", offer.TherapistID)
	}
}

func (m *mockBookingRepoAW) ListAllEvents(ctx context.Context, params repository.ListAllEventsParams) ([]model.BookingEvent, int, error) {
	return nil, 0, nil
}
