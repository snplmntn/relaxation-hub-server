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

// --- Local Mocks for State Machine Test ---
// Reusing mockDB from mock_db_test.go

// mockQueueState is specific to this test for state inspection
type mockQueueState struct {
	items map[int64]*repository.QueueItem
	updates []string
}
func (m *mockQueueState) Enqueue(ctx context.Context, bookingID int64) error { return nil }
func (m *mockQueueState) EnqueueTx(ctx context.Context, tx pgx.Tx, bookingID int64) error { return nil }
func (m *mockQueueState) EnqueueManyTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64) error { return nil }
func (m *mockQueueState) DequeueBatch(ctx context.Context, limit int) ([]repository.QueueItem, error) {
	var result []repository.QueueItem
	for _, it := range m.items {
		result = append(result, *it)
	}
	return result, nil
}
func (m *mockQueueState) Remove(ctx context.Context, bookingID int64) error {
	delete(m.items, bookingID)
	return nil
}
func (m *mockQueueState) IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error {
	if it, ok := m.items[bookingID]; ok {
		it.Attempts = attempts
	}
	return nil
}
func (m *mockQueueState) UpdateWorkflowState(ctx context.Context, bookingID int64, state string, data map[string]interface{}) error {
	if it, ok := m.items[bookingID]; ok {
		it.WorkflowState = state
		m.updates = append(m.updates, state)
	}
	return nil
}

type mockBookingRepoState struct {
	bookings map[int64]*model.Booking
}

func (m *mockBookingRepoState) UpdatePayoutReference(ctx context.Context, bookingIDs []int64, payoutID int64) error { return nil }
func (m *mockBookingRepoState) UpdatePayoutReferenceTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64, payoutID int64) error { return nil }
func (m *mockBookingRepoState) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoState) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) {
	res := make(map[int64]*repository.BookingDetailsResult)
	for _, id := range bookingIDs {
		if b, ok := m.bookings[id]; ok {
			res[id] = &repository.BookingDetailsResult{Booking: b}
		}
	}
	return res, nil
}
func (m *mockBookingRepoState) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoState) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error { return nil }
func (m *mockBookingRepoState) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	if b, ok := m.bookings[bookingID]; ok {
		return b, nil
	}
	return nil, pgx.ErrNoRows
}
func (m *mockBookingRepoState) GetByBookingIDWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	if b, ok := m.bookings[bookingID]; ok {
		return &repository.BookingDetailsResult{Booking: b}, nil
	}
	return nil, pgx.ErrNoRows
}
func (m *mockBookingRepoState) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoState) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoState) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockBookingRepoState) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error { return nil }
func (m *mockBookingRepoState) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoState) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }
func (m *mockBookingRepoState) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoState) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoState) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) { return m.GetByID(ctx, bookingID, 0) }
func (m *mockBookingRepoState) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) { return m.GetByBookingID(ctx, bookingID) }
func (m *mockBookingRepoState) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	var res []model.Booking
	for _, b := range m.bookings {
		if b.GroupID != nil && *b.GroupID == groupID {
			res = append(res, *b)
		}
	}
	return res, nil
}
func (m *mockBookingRepoState) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	res := make(map[int64][]model.Booking)
	for _, b := range m.bookings {
		if b.GroupID != nil {
			for _, gid := range groupIDs {
				if *b.GroupID == gid {
					res[gid] = append(res[gid], *b)
					break
				}
			}
		}
	}
	return res, nil
}
func (m *mockBookingRepoState) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) { return nil, nil }
func (m *mockBookingRepoState) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) { return nil, nil }
func (m *mockBookingRepoState) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error { return nil }
func (m *mockBookingRepoState) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) { return m.GetByBookingIDWithDetailsUnsafe(ctx, bookingID) }
func (m *mockBookingRepoState) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) { return m.GetByBookingIDWithDetailsUnsafe(ctx, bookingID) }
func (m *mockBookingRepoState) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoState) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoState) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoState) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoState) ListGlobalPending(ctx context.Context) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoState) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) { return nil, nil }
func (m *mockBookingRepoState) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error { return nil }
func (m *mockBookingRepoState) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error { return nil }
func (m *mockBookingRepoState) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoState) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) { return nil, 0, nil }
func (m *mockBookingRepoState) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) { return nil, 0, nil }
func (m *mockBookingRepoState) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error { return nil }
func (m *mockBookingRepoState) ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoState) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error { return nil }
func (m *mockBookingRepoState) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) { return &repository.ClientBookingStats{}, nil }
func (m *mockBookingRepoState) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) { return 0, nil }
func (m *mockBookingRepoState) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) { return &repository.AccountingSummary{}, nil }
func (m *mockBookingRepoState) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) { return nil, nil }
func (m *mockBookingRepoState) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error { return nil }
func (m *mockBookingRepoState) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error { return nil }
func (m *mockBookingRepoState) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int) ([]repository.BookingDetailsResult, int, error) { return nil, 0, nil }


type mockMatchState struct {
	result []model.TherapistProfile
}
func (m *mockMatchState) FindAvailableTherapistsForService(ctx context.Context, clientID int64, serviceID int64, genderPref string, pressurePref string) ([]model.TherapistProfile, error) { return m.result, nil }
func (m *mockMatchState) FindAvailableTherapistsForServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPref string, pressurePref string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error) { return m.result, nil }
func (m *mockMatchState) FindNearbyAvailableTherapists(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPref string, pressurePref string) ([]model.TherapistProfile, error) { return nil, nil }

type mockOfferRepoState struct {
	created []*model.BookingOffer
}
func (m *mockOfferRepoState) Create(ctx context.Context, offer *model.BookingOffer) error {
	offer.OfferID = int64(len(m.created)+1)
	m.created = append(m.created, offer)
	return nil
}
func (m *mockOfferRepoState) CreateTx(ctx context.Context, tx pgx.Tx, offer *model.BookingOffer) error {
	// Simplified transactional create for state test
	return m.Create(ctx, offer)
}
func (m *mockOfferRepoState) GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoState) GetActiveOffersBatch(ctx context.Context, bookingIDs []int64) (map[int64][]model.BookingOffer, error) { return make(map[int64][]model.BookingOffer), nil }
func (m *mockOfferRepoState) GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoState) GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoState) UpdateStatus(ctx context.Context, offerID int64, status string) error { return nil }
func (m *mockOfferRepoState) UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error { return nil }
func (m *mockOfferRepoState) ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoState) ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoState) CancelOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoState) GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }

// mockTherapistRepoState (supports TryLock)
type mockTherapistRepoState struct {}
func (m *mockTherapistRepoState) TryLockTherapist(ctx context.Context, therapistID int64) (bool, error) { return true, nil }
func (m *mockTherapistRepoState) TryLockTherapistTx(ctx context.Context, tx pgx.Tx, therapistID int64) (bool, error) { return true, nil }
func (m *mockTherapistRepoState) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoState) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error { return nil }
func (m *mockTherapistRepoState) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoState) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error { return nil }
func (m *mockTherapistRepoState) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) { return nil, nil }
func (m *mockTherapistRepoState) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error { return nil }
func (m *mockTherapistRepoState) AddService(ctx context.Context, ts *model.TherapistService) error { return nil }
func (m *mockTherapistRepoState) RemoveService(ctx context.Context, therapistID, serviceID int64) error { return nil }
func (m *mockTherapistRepoState) GetServices(ctx context.Context, therapistID int64) ([]int64, error) { return nil, nil }
func (m *mockTherapistRepoState) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error { return nil }
func (m *mockTherapistRepoState) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) { return nil, nil }
func (m *mockTherapistRepoState) CreateProfile(ctx context.Context, therapistID int64) error { return nil }
func (m *mockTherapistRepoState) FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoState) FindAvailableByServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoState) FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoState) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoState) SetAtBranch(ctx context.Context, therapistID int64, atBranch bool) error { return nil }


func TestAssignmentStateMachine_InitToMatching(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(100)
	serviceID := int64(10)

	q := &mockQueueState{items: map[int64]*repository.QueueItem{
		bookingID: {BookingID: bookingID, WorkflowState: ""},
	}}
	
	br := &mockBookingRepoState{bookings: map[int64]*model.Booking{
		bookingID: {BookingID: bookingID, ClientID: 1, ServiceID: &serviceID, DurationMinutes: 60},
	}}
	
	ms := &mockMatchState{result: []model.TherapistProfile{{TherapistID: 99}}}
	
	or := &mockOfferRepoState{}
	
	dbMock := &mockDB{} 

	tr := &mockTherapistRepoState{} // injected

	worker := NewAssignmentWorker(dbMock, q, br, nil, or, nil, nil, tr, ms, nil, nil)
	worker.batchSize = 1 

	worker.processOnce(ctx)

	if len(q.updates) == 0 {
		t.Fatal("expected state transitions")
	}
	
	foundMatching := false
	foundOffering := false
	for _, s := range q.updates {
		if s == "matching" { foundMatching = true }
		if s == "offering" { foundOffering = true }
	}
	if !foundMatching { t.Error("did not transition to matching") }
	if !foundOffering { t.Error("did not transition to offering") }

	if len(or.created) != 1 {
		t.Errorf("expected 1 offer created, got %d", len(or.created))
	}
}
