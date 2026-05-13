package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// Mocks for testing offer creation
type mockRepoForOffers struct {
	mu               sync.Mutex
	createdBookingID int64
	insertedEvents   []struct {
		bookingID int64
		eventType string
		actorID   *int64
		metadata  map[string]any
	}
}

func (m *mockRepoForOffers) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockRepoForOffers) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	booking.BookingID = 999
	m.createdBookingID = 999
	return nil
}
func (m *mockRepoForOffers) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockRepoForOffers) UpdateAdmin(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockRepoForOffers) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	return nil
}
func (m *mockRepoForOffers) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockRepoForOffers) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockRepoForOffers) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return nil, nil
}
func (m *mockRepoForOffers) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertedEvents = append(m.insertedEvents, struct {
		bookingID int64
		eventType string
		actorID   *int64
		metadata  map[string]any
	}{bookingID, eventType, actorID, metadata})
	return nil
}
func (m *mockRepoForOffers) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	return nil
}
func (m *mockRepoForOffers) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return nil
}
func (m *mockRepoForOffers) UpdatePayoutReference(ctx context.Context, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockRepoForOffers) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockRepoForOffers) FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockRepoForOffers) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}
func (m *mockRepoForOffers) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return map[int64]int{}, nil
}
func (m *mockRepoForOffers) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID}}, nil
}
func (m *mockRepoForOffers) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID}}, nil
}
func (m *mockRepoForOffers) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1}}, nil
}
func (m *mockRepoForOffers) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1}}, nil
}
func (m *mockRepoForOffers) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockRepoForOffers) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockRepoForOffers) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	return nil
}
func (m *mockRepoForOffers) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	return nil
}
func (m *mockRepoForOffers) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) ListDueInProgressBookings(ctx context.Context, now time.Time, limit int) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockRepoForOffers) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockRepoForOffers) ListUpcomingBookingsForReminder(ctx context.Context, windowStart, windowEnd time.Time, eventTypeExclude string) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoForOffers) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockRepoForOffers) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
	return nil
}
func (m *mockRepoForOffers) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockRepoForOffers) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
	return &repository.ClientBookingStats{}, nil
}
func (m *mockRepoForOffers) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) {
	return &repository.AccountingSummary{}, nil
}
func (m *mockRepoForOffers) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) {
	return nil, nil
}
func (m *mockRepoForOffers) HasAssignedOutboundRiderCoverage(ctx context.Context, bookingID int64) (bool, error) {
	return true, nil
}

func (m *mockRepoForOffers) RevertOnTheWayToAssigned(ctx context.Context, bookingID, actorID int64) (*repository.RevertOnTheWayToAssignedResult, error) {
	return nil, nil
}

func (m *mockRepoForOffers) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	return nil
}
func (m *mockRepoForOffers) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	return nil
}
func (m *mockRepoForOffers) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockRepoForOffers) UpdatePayoutReferenceTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64, payoutID int64) error {
	return nil
}

// mockOfferRepo captures created offers
type mockOfferRepoForTest struct {
	mu      sync.Mutex
	created []*model.BookingOffer
}

func (m *mockOfferRepoForTest) Create(ctx context.Context, offer *model.BookingOffer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	offer.OfferID = int64(len(m.created) + 1)
	m.created = append(m.created, offer)
	return nil
}
func (m *mockOfferRepoForTest) CreateTx(ctx context.Context, tx pgx.Tx, offer *model.BookingOffer) error {
	return m.Create(ctx, offer)
}
func (m *mockOfferRepoForTest) GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	return nil, nil
}
func (m *mockOfferRepoForTest) GetActiveOffersBatch(ctx context.Context, bookingIDs []int64) (map[int64][]model.BookingOffer, error) {
	return make(map[int64][]model.BookingOffer), nil
}
func (m *mockOfferRepoForTest) GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) {
	return []model.BookingOffer{}, nil
}
func (m *mockOfferRepoForTest) GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error) {
	return nil, nil
}
func (m *mockOfferRepoForTest) UpdateStatus(ctx context.Context, offerID int64, status string) error {
	return nil
}
func (m *mockOfferRepoForTest) UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error {
	return nil
}
func (m *mockOfferRepoForTest) ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	return nil, nil
}
func (m *mockOfferRepoForTest) ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error) {
	return nil, nil
}
func (m *mockOfferRepoForTest) GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	return nil, nil
}
func (m *mockOfferRepoForTest) CancelOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	return nil, nil
}

// mockTherapistRepo returns candidates
type mockTherapistRepoForTest struct{}

func (m *mockTherapistRepoForTest) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoForTest) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error {
	return nil
}
func (m *mockTherapistRepoForTest) SetLifecycleStatus(ctx context.Context, therapistID int64, accountStatus string, acceptAssignments bool) error {
	return nil
}
func (m *mockTherapistRepoForTest) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoForTest) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error {
	return nil
}
func (m *mockTherapistRepoForTest) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) {
	return nil, nil
}
func (m *mockTherapistRepoForTest) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error {
	return nil
}
func (m *mockTherapistRepoForTest) AddService(ctx context.Context, ts *model.TherapistService) error {
	return nil
}
func (m *mockTherapistRepoForTest) RemoveService(ctx context.Context, therapistID, serviceID int64) error {
	return nil
}
func (m *mockTherapistRepoForTest) GetServices(ctx context.Context, therapistID int64) ([]int64, error) {
	return nil, nil
}
func (m *mockTherapistRepoForTest) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error {
	return nil
}
func (m *mockTherapistRepoForTest) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) {
	return map[int64][]string{}, nil
}
func (m *mockTherapistRepoForTest) CreateProfile(ctx context.Context, therapistID int64) error {
	return nil
}
func (m *mockTherapistRepoForTest) SetBatchServices(ctx context.Context, therapistID int64, serviceIDs []model.AddServiceWithPressuresRequest) error {
	return nil
}

func (m *mockTherapistRepoForTest) FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return []model.TherapistProfile{{TherapistID: 101}, {TherapistID: 102}, {TherapistID: 103}}, nil
}
func (m *mockTherapistRepoForTest) FindAvailableByServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error) {
	return []model.TherapistProfile{{TherapistID: 101}, {TherapistID: 102}, {TherapistID: 103}}, nil
}
func (m *mockTherapistRepoForTest) FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoForTest) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoForTest) SetAtBranch(ctx context.Context, therapistID int64, atBranch bool) error {
	return nil
}
func (m *mockTherapistRepoForTest) TryLockTherapist(ctx context.Context, therapistID int64) (bool, error) {
	return true, nil
}
func (m *mockTherapistRepoForTest) TryLockTherapistTx(ctx context.Context, tx pgx.Tx, therapistID int64) (bool, error) {
	return true, nil
}

// minimal mocks for other dependencies
// nilPromoRepo removed - now using common_test.go
// nilQueueRepo removed - now using common_test.go (renamed to nilAssignmentQueueRepo)
// mockServiceRepo replaced by nilServiceRepo in common_test.go if appropriate,
// or I'll just keep it here if it has specific logic.
// Actually mockServiceRepo here has specific BasePrice logic. I'll keep it but rename it.

// nilAssignmentQueueRepo is in common_test.go

type mockServiceRepo struct{}

func (m *mockServiceRepo) Create(ctx context.Context, svc *model.Service) error { return nil }
func (m *mockServiceRepo) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	return &model.Service{ServiceID: serviceID, BasePrice: 300, DurationMinutes: 60}, nil
}
func (m *mockServiceRepo) GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepo) ListActive(ctx context.Context) ([]model.Service, error) { return nil, nil }
func (m *mockServiceRepo) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepo) ListPopular(ctx context.Context) ([]model.Service, error) { return nil, nil }
func (m *mockServiceRepo) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepo) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	return nil
}
func (m *mockServiceRepo) Delete(ctx context.Context, id int64) error { return nil }

func TestCreate_CreatesOffersAndEvents(t *testing.T) {
	ctx := context.Background()
	mockBooking := &mockRepoForOffers{}
	mockOffer := &mockOfferRepoForTest{}
	mockTher := &mockTherapistRepoForTest{}
	promo := &nilPromoRepo{}
	queue := &nilAssignmentQueueRepo{}

	mockSvcRepo := &mockServiceRepo{}
	svc := NewBookingService(mockBooking, promo, nil, queue, mockTher, mockOffer, mockSvcRepo, nil, nil, nil, nil, nil, nil, nil)

	req := &model.CreateBookingRequest{ServiceID: ptrInt64(10), DurationMinutes: 60}
	b, err := svc.Create(ctx, 11, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatalf("expected booking returned")
	}

	// Retry loop for async offers
	var offers []*model.BookingOffer
	for i := 0; i < 10; i++ {
		mockOffer.mu.Lock()
		if len(mockOffer.created) > 0 {
			offers = make([]*model.BookingOffer, len(mockOffer.created))
			copy(offers, mockOffer.created)
			mockOffer.mu.Unlock()
			break
		}
		mockOffer.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}

	if len(offers) == 0 {
		t.Fatalf("expected offers to be created, got 0")
	}
	mockBooking.mu.Lock()
	createdID := mockBooking.createdBookingID
	mockBooking.mu.Unlock()
	if offers[0].BookingID != createdID {
		t.Fatalf("offer BookingID mismatch: got %d want %d", offers[0].BookingID, createdID)
	}
	// verify events inserted for offers: look for offered_to_therapist
	found := false
	for i := 0; i < 10; i++ {
		mockBooking.mu.Lock()
		for _, ev := range mockBooking.insertedEvents {
			if ev.eventType == "offered_to_therapist" {
				found = true
				break
			}
		}
		mockBooking.mu.Unlock()
		if found {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !found {
		t.Fatalf("offered_to_therapist event not inserted")
	}

	// Verify metadata once found
	mockBooking.mu.Lock()
	defer mockBooking.mu.Unlock()
	for _, ev := range mockBooking.insertedEvents {
		if ev.eventType == "offered_to_therapist" {
			if ev.metadata == nil {
				t.Fatalf("expected metadata on offered_to_therapist event")
			}
			if _, ok := ev.metadata["offer_id"]; !ok {
				t.Fatalf("metadata missing offer_id")
			}
			if _, ok := ev.metadata["target_therapist_id"]; !ok {
				t.Fatalf("metadata missing target_therapist_id")
			}
		}
	}
}

func ptrInt64(v int64) *int64 { return &v }

func (m *mockRepoForOffers) ListAllEvents(ctx context.Context, params repository.ListAllEventsParams) ([]model.BookingEvent, int, error) {
	return nil, 0, nil
}
