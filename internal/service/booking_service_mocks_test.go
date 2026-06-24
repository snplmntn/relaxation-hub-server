package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockBookingRepository mocks repository.BookingRepository
type MockBookingRepository struct {
	mock.Mock
}

func (m *MockBookingRepository) Create(ctx context.Context, booking *model.Booking) error {
	args := m.Called(ctx, booking)
	return args.Error(0)
}

func (m *MockBookingRepository) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	args := m.Called(ctx, tx, booking)
	return args.Error(0)
}

func (m *MockBookingRepository) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	args := m.Called(ctx, bookingID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Booking), args.Error(1)
}

func (m *MockBookingRepository) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Booking), args.Error(1)
}

func (m *MockBookingRepository) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	args := m.Called(ctx, bookingID, actorID, role, status, cancelledBy, cancellationReason)
	return args.Error(0)
}

func (m *MockBookingRepository) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	args := m.Called(ctx, bookingID, actorID, role, status, cancelledBy, cancellationReason, customTime)
	return args.Error(0)
}

func (m *MockBookingRepository) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	args := m.Called(ctx, bookingID, therapistID)
	return args.Error(0)
}

func (m *MockBookingRepository) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	args := m.Called(ctx, bookingID, therapistID, actorID)
	return args.Error(0)
}

func (m *MockBookingRepository) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	args := m.Called(ctx, tx, bookingID, therapistID, actorID)
	return args.Error(0)
}

func (m *MockBookingRepository) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	args := m.Called(ctx, clientID)
	return args.Get(0).([]model.Booking), args.Error(1)
}

func (m *MockBookingRepository) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	args := m.Called(ctx, therapistID)
	return args.Get(0).([]model.Booking), args.Error(1)
}

func (m *MockBookingRepository) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) {
	args := m.Called(ctx, tx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Booking), args.Error(1)
}

func (m *MockBookingRepository) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	args := m.Called(ctx, bookingID, eventType, actorID, metadata)
	return args.Error(0)
}

func (m *MockBookingRepository) Update(ctx context.Context, booking *model.Booking) error {
	args := m.Called(ctx, booking)
	return args.Error(0)
}

func (m *MockBookingRepository) UpdateAdmin(ctx context.Context, booking *model.Booking) error {
	args := m.Called(ctx, booking)
	return args.Error(0)
}

func (m *MockBookingRepository) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) {
	args := m.Called(ctx, bookingIDs)
	return args.Get(0).([]model.Booking), args.Error(1)
}

func (m *MockBookingRepository) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	args := m.Called(ctx, groupID)
	return args.Get(0).([]model.Booking), args.Error(1)
}

func (m *MockBookingRepository) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	args := m.Called(ctx, groupIDs)
	return args.Get(0).(map[int64][]model.Booking), args.Error(1)
}

func (m *MockBookingRepository) HasActiveNonFinalBookings(ctx context.Context, therapistID int64) (bool, error) {
	args := m.Called(ctx, therapistID)
	return args.Bool(0), args.Error(1)
}

func (m *MockBookingRepository) HasAssignedOutboundRiderCoverage(ctx context.Context, bookingID int64) (bool, error) {
	args := m.Called(ctx, bookingID)
	return args.Bool(0), args.Error(1)
}

func (m *MockBookingRepository) ClearAssignedOutboundRider(ctx context.Context, bookingID int64) error {
	args := m.Called(ctx, bookingID)
	return args.Error(0)
}

func (m *MockBookingRepository) RevertOnTheWayToAssigned(ctx context.Context, bookingID, actorID int64) (*repository.RevertOnTheWayToAssignedResult, error) {
	args := m.Called(ctx, bookingID, actorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.RevertOnTheWayToAssignedResult), args.Error(1)
}

func (m *MockBookingRepository) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	args := m.Called(ctx, bookingID)
	return args.Get(0).([]model.BookingEvent), args.Error(1)
}

func (m *MockBookingRepository) ListAllEvents(ctx context.Context, params repository.ListAllEventsParams) ([]model.BookingEvent, int, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]model.BookingEvent), args.Int(1), args.Error(2)
}

func (m *MockBookingRepository) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	args := m.Called(ctx, therapistIDs, since)
	return args.Get(0).(map[int64]bool), args.Error(1)
}

func (m *MockBookingRepository) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	args := m.Called(ctx, bookingID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.BookingDetailsResult), args.Error(1)
}

func (m *MockBookingRepository) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.BookingDetailsResult), args.Error(1)
}

func (m *MockBookingRepository) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	args := m.Called(ctx, referenceCode, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.BookingDetailsResult), args.Error(1)
}

func (m *MockBookingRepository) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	args := m.Called(ctx, referenceCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.BookingDetailsResult), args.Error(1)
}

func (m *MockBookingRepository) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) {
	args := m.Called(ctx, bookingIDs)
	return args.Get(0).(map[int64]*repository.BookingDetailsResult), args.Error(1)
}

func (m *MockBookingRepository) FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*repository.BookingDetailsResult, error) {
	args := m.Called(ctx, therapistID, excludeBookingID, after)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.BookingDetailsResult), args.Error(1)
}

func (m *MockBookingRepository) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	args := m.Called(ctx, clientID)
	return args.Get(0).([]repository.BookingDetailsResult), args.Error(1)
}

func (m *MockBookingRepository) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	args := m.Called(ctx, therapistID)
	return args.Get(0).([]repository.BookingDetailsResult), args.Error(1)
}

func (m *MockBookingRepository) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	args := m.Called(ctx, clientID, limit, offset)
	return args.Get(0).([]repository.BookingDetailsResult), args.Int(1), args.Error(2)
}

func (m *MockBookingRepository) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	args := m.Called(ctx, therapistID, limit, offset)
	return args.Get(0).([]repository.BookingDetailsResult), args.Int(1), args.Error(2)
}

func (m *MockBookingRepository) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status, dateFrom, dateTo string) ([]repository.BookingDetailsResult, int, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]repository.BookingDetailsResult), args.Int(1), args.Error(2)
}

func (m *MockBookingRepository) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Booking), args.Error(1)
}

func (m *MockBookingRepository) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	args := m.Called(ctx, therapistIDs, since)
	return args.Get(0).(map[int64]int), args.Error(1)
}

func (m *MockBookingRepository) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	args := m.Called(ctx, bookingID, pauseStart)
	return args.Error(0)
}

func (m *MockBookingRepository) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	args := m.Called(ctx, bookingID, totalPausedSeconds)
	return args.Error(0)
}

func (m *MockBookingRepository) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Booking), args.Error(1)
}

func (m *MockBookingRepository) ListDueInProgressBookings(ctx context.Context, now time.Time, limit int) ([]model.Booking, error) {
	return nil, nil
}

func (m *MockBookingRepository) ListUpcomingBookingsForReminder(ctx context.Context, windowStart, windowEnd time.Time, eventTypeExclude string) ([]model.Booking, error) {
	args := m.Called(ctx, windowStart, windowEnd, eventTypeExclude)
	return args.Get(0).([]model.Booking), args.Error(1)
}

func (m *MockBookingRepository) ClaimDueReminderJobs(ctx context.Context, now time.Time, limit int) ([]repository.BookingReminderJob, error) {
	args := m.Called(ctx, now, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.BookingReminderJob), args.Error(1)
}

func (m *MockBookingRepository) MarkReminderJobProcessed(ctx context.Context, jobID int64) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockBookingRepository) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
	args := m.Called(ctx, bookingID, actorID, metadata)
	return args.Error(0)
}

func (m *MockBookingRepository) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
	args := m.Called(ctx, bookingID, proofURL)
	return args.Error(0)
}

func (m *MockBookingRepository) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
	args := m.Called(ctx, actorID, eventType, since)
	return args.Int(0), args.Error(1)
}

func (m *MockBookingRepository) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
	args := m.Called(ctx, clientID, lateCancellationSince)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ClientBookingStats), args.Error(1)
}

func (m *MockBookingRepository) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	args := m.Called(ctx, bookingID, earnings, fee, actualEnd)
	return args.Error(0)
}

func (m *MockBookingRepository) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	args := m.Called(ctx, pool, bookingID, therapistID, earnings, fee, revenue, actualEnd)
	return args.Error(0)
}

func (m *MockBookingRepository) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) {
	args := m.Called(ctx, startDate, endDate)
	return args.Get(0).(*repository.AccountingSummary), args.Error(1)
}

func (m *MockBookingRepository) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) {
	args := m.Called(ctx, startDate, endDate)
	return args.Get(0).([]repository.DailyAccountingEntry), args.Error(1)
}

func (m *MockBookingRepository) UpdatePayoutReference(ctx context.Context, bookingIDs []int64, payoutID int64) error {
	args := m.Called(ctx, bookingIDs, payoutID)
	return args.Error(0)
}

// MockAssignmentQueueRepository mocks repository.AssignmentQueueRepository
type MockAssignmentQueueRepository struct {
	mock.Mock
}

func (m *MockAssignmentQueueRepository) Enqueue(ctx context.Context, bookingID int64) error {
	args := m.Called(ctx, bookingID)
	return args.Error(0)
}

func (m *MockAssignmentQueueRepository) EnqueueTx(ctx context.Context, tx pgx.Tx, bookingID int64) error {
	args := m.Called(ctx, tx, bookingID)
	return args.Error(0)
}

func (m *MockAssignmentQueueRepository) EnqueueManyTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64) error {
	args := m.Called(ctx, tx, bookingIDs)
	return args.Error(0)
}

func (m *MockAssignmentQueueRepository) DequeueBatch(ctx context.Context, limit int) ([]repository.QueueItem, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]repository.QueueItem), args.Error(1)
}

func (m *MockAssignmentQueueRepository) Remove(ctx context.Context, bookingID int64) error {
	args := m.Called(ctx, bookingID)
	return args.Error(0)
}

func (m *MockAssignmentQueueRepository) IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error {
	args := m.Called(ctx, bookingID, attempts, nextAttempt)
	return args.Error(0)
}

func (m *MockAssignmentQueueRepository) UpdateWorkflowState(ctx context.Context, bookingID int64, state string, data map[string]interface{}) error {
	args := m.Called(ctx, bookingID, state, data)
	return args.Error(0)
}

// MockPromoRepository mocks repository.PromotionRepository
type MockPromoRepository struct {
	mock.Mock
}

func (m *MockPromoRepository) Create(ctx context.Context, p *model.Promotion) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockPromoRepository) ListActive(ctx context.Context, now time.Time) ([]model.Promotion, error) {
	args := m.Called(ctx, now)
	return args.Get(0).([]model.Promotion), args.Error(1)
}

func (m *MockPromoRepository) GetByCode(ctx context.Context, code string) (*model.Promotion, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Promotion), args.Error(1)
}

func (m *MockPromoRepository) TryIncrementGlobalUsageTx(ctx context.Context, tx pgx.Tx, promoID int64) (bool, error) {
	args := m.Called(ctx, tx, promoID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPromoRepository) TryIncrementUserPromoUsageTx(ctx context.Context, tx pgx.Tx, promoID, userID int64) (bool, error) {
	args := m.Called(ctx, tx, promoID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPromoRepository) ListAll(ctx context.Context) ([]model.Promotion, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Promotion), args.Error(1)
}

func (m *MockPromoRepository) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	args := m.Called(ctx, id, updates)
	return args.Error(0)
}

func (m *MockPromoRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockTherapistRepository mocks repository.TherapistRepository
type MockTherapistRepository struct {
	mock.Mock
}

func (m *MockTherapistRepository) CreateProfile(ctx context.Context, therapistID int64) error {
	args := m.Called(ctx, therapistID)
	return args.Error(0)
}

func (m *MockTherapistRepository) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	args := m.Called(ctx, therapistID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.TherapistProfile), args.Error(1)
}

func (m *MockTherapistRepository) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error {
	args := m.Called(ctx, therapistID, updates)
	return args.Error(0)
}

func (m *MockTherapistRepository) SetLifecycleStatus(ctx context.Context, therapistID int64, accountStatus string, acceptAssignments bool) error {
	args := m.Called(ctx, therapistID, accountStatus, acceptAssignments)
	return args.Error(0)
}

func (m *MockTherapistRepository) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) {
	args := m.Called(ctx, availableOnly)
	return args.Get(0).([]model.TherapistProfile), args.Error(1)
}

func (m *MockTherapistRepository) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error {
	args := m.Called(ctx, doc)
	return args.Error(0)
}

func (m *MockTherapistRepository) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) {
	args := m.Called(ctx, therapistID)
	return args.Get(0).([]model.TherapistDocument), args.Error(1)
}

func (m *MockTherapistRepository) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error {
	args := m.Called(ctx, documentID, verifierID, status)
	return args.Error(0)
}

func (m *MockTherapistRepository) AddService(ctx context.Context, ts *model.TherapistService) error {
	args := m.Called(ctx, ts)
	return args.Error(0)
}

func (m *MockTherapistRepository) RemoveService(ctx context.Context, therapistID, serviceID int64) error {
	args := m.Called(ctx, therapistID, serviceID)
	return args.Error(0)
}

func (m *MockTherapistRepository) GetServices(ctx context.Context, therapistID int64) ([]int64, error) {
	args := m.Called(ctx, therapistID)
	return args.Get(0).([]int64), args.Error(1)
}

func (m *MockTherapistRepository) FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	args := m.Called(ctx, clientID, serviceID, genderPreference, pressurePreference)
	return args.Get(0).([]model.TherapistProfile), args.Error(1)
}

func (m *MockTherapistRepository) FindAvailableByServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error) {
	args := m.Called(ctx, clientID, serviceID, genderPreference, pressurePreference, scheduledStart, durationMinutes, lat, lng)
	return args.Get(0).([]model.TherapistProfile), args.Error(1)
}

func (m *MockTherapistRepository) FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	args := m.Called(ctx, clientID, serviceID, latitude, longitude, radiusKm, genderPreference, pressurePreference)
	return args.Get(0).([]model.TherapistProfile), args.Error(1)
}

func (m *MockTherapistRepository) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error {
	args := m.Called(ctx, therapistID, serviceID, pressures)
	return args.Error(0)
}

func (m *MockTherapistRepository) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) {
	args := m.Called(ctx, therapistID)
	return args.Get(0).(map[int64][]string), args.Error(1)
}

func (m *MockTherapistRepository) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) {
	args := m.Called(ctx, therapistIDs)
	return args.Get(0).([]model.TherapistProfile), args.Error(1)
}

func (m *MockTherapistRepository) SetAtBranch(ctx context.Context, therapistID int64, atBranch bool) error {
	args := m.Called(ctx, therapistID, atBranch)
	return args.Error(0)
}

func (m *MockTherapistRepository) TryLockTherapist(ctx context.Context, therapistID int64) (bool, error) {
	args := m.Called(ctx, therapistID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTherapistRepository) TryLockTherapistTx(ctx context.Context, tx pgx.Tx, therapistID int64) (bool, error) {
	args := m.Called(ctx, tx, therapistID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTherapistRepository) SetBatchServices(ctx context.Context, therapistID int64, services []model.AddServiceWithPressuresRequest) error {
	args := m.Called(ctx, therapistID, services)
	return args.Error(0)
}

// MockOfferRepository mocks repository.BookingOfferRepository
type MockOfferRepository struct {
	mock.Mock
}

func (m *MockOfferRepository) Create(ctx context.Context, offer *model.BookingOffer) error {
	args := m.Called(ctx, offer)
	return args.Error(0)
}

func (m *MockOfferRepository) CreateTx(ctx context.Context, tx pgx.Tx, offer *model.BookingOffer) error {
	args := m.Called(ctx, tx, offer)
	return args.Error(0)
}

func (m *MockOfferRepository) CreateOffer(ctx context.Context, offer *model.BookingOffer) error {
	args := m.Called(ctx, offer)
	return args.Error(0)
}

func (m *MockOfferRepository) CreateOfferTx(ctx context.Context, tx pgx.Tx, offer *model.BookingOffer) error {
	args := m.Called(ctx, tx, offer)
	return args.Error(0)
}

func (m *MockOfferRepository) GetOffer(ctx context.Context, offerID int64) (*model.BookingOffer, error) {
	args := m.Called(ctx, offerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) {
	args := m.Called(ctx, therapistID)
	return args.Get(0).([]model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error) {
	args := m.Called(ctx, therapistID, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) UpdateStatus(ctx context.Context, offerID int64, status string) error {
	args := m.Called(ctx, offerID, status)
	return args.Error(0)
}

func (m *MockOfferRepository) UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error {
	args := m.Called(ctx, tx, offerID, status)
	return args.Error(0)
}

func (m *MockOfferRepository) ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error) {
	args := m.Called(ctx, tx, bookingID)
	return args.Get(0).([]model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	args := m.Called(ctx, bookingID)
	return args.Get(0).([]model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) GetActiveOffersBatch(ctx context.Context, bookingIDs []int64) (map[int64][]model.BookingOffer, error) {
	args := m.Called(ctx, bookingIDs)
	return args.Get(0).(map[int64][]model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	args := m.Called(ctx, bookingID)
	return args.Get(0).([]model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) CancelOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	args := m.Called(ctx, bookingID)
	return args.Get(0).([]model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	args := m.Called(ctx, bookingID)
	return args.Get(0).([]model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) ListByBooking(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	args := m.Called(ctx, bookingID)
	return args.Get(0).([]model.BookingOffer), args.Error(1)
}

func (m *MockOfferRepository) MarkAsRejected(ctx context.Context, offerID int64, reason string) error {
	args := m.Called(ctx, offerID, reason)
	return args.Error(0)
}

func (m *MockOfferRepository) ListRecentEvents(ctx context.Context, limit int) ([]model.BookingOffer, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]model.BookingOffer), args.Error(1)
}

// MockExtensionRequestRepository mocks repository.ExtensionRequestRepository
type MockExtensionRequestRepository struct {
	mock.Mock
}

func (m *MockExtensionRequestRepository) Create(ctx context.Context, req *model.ExtensionRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockExtensionRequestRepository) GetByID(ctx context.Context, id int64) (*model.ExtensionRequest, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExtensionRequest), args.Error(1)
}

func (m *MockExtensionRequestRepository) GetPendingByBookingID(ctx context.Context, bookingID int64) (*model.ExtensionRequest, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExtensionRequest), args.Error(1)
}

func (m *MockExtensionRequestRepository) UpdateStatus(ctx context.Context, id int64, status string, actorID int64, note *string) error {
	args := m.Called(ctx, id, status, actorID, note)
	return args.Error(0)
}

// MockServiceRepository mocks repository.ServiceRepository
type MockServiceRepository struct {
	mock.Mock
}

func (m *MockServiceRepository) Create(ctx context.Context, svc *model.Service) error {
	args := m.Called(ctx, svc)
	return args.Error(0)
}

func (m *MockServiceRepository) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	args := m.Called(ctx, serviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Service), args.Error(1)
}

func (m *MockServiceRepository) GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).([]model.Service), args.Error(1)
}

func (m *MockServiceRepository) ListActive(ctx context.Context) ([]model.Service, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Service), args.Error(1)
}

func (m *MockServiceRepository) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.Service), args.Error(1)
}

func (m *MockServiceRepository) ListPopular(ctx context.Context) ([]model.Service, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Service), args.Error(1)
}

func (m *MockServiceRepository) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Service), args.Error(1)
}

func (m *MockServiceRepository) Update(ctx context.Context, serviceID int64, updates map[string]interface{}) error {
	args := m.Called(ctx, serviceID, updates)
	return args.Error(0)
}

func (m *MockServiceRepository) Delete(ctx context.Context, serviceID int64) error {
	args := m.Called(ctx, serviceID)
	return args.Error(0)
}

// MockAddressRepository mocks repository.AddressRepository
type MockAddressRepository struct {
	mock.Mock
}

func (m *MockAddressRepository) Create(ctx context.Context, address *model.Address) error {
	args := m.Called(ctx, address)
	return args.Error(0)
}

func (m *MockAddressRepository) GetByID(ctx context.Context, addressID, userID int64) (*model.Address, error) {
	args := m.Called(ctx, addressID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Address), args.Error(1)
}

func (m *MockAddressRepository) GetByIDUnsafe(ctx context.Context, addressID int64) (*model.Address, error) {
	args := m.Called(ctx, addressID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Address), args.Error(1)
}

func (m *MockAddressRepository) ListForUser(ctx context.Context, userID int64, includeDeleted bool) ([]model.Address, error) {
	args := m.Called(ctx, userID, includeDeleted)
	return args.Get(0).([]model.Address), args.Error(1)
}

func (m *MockAddressRepository) SoftDelete(ctx context.Context, addressID, userID int64) error {
	args := m.Called(ctx, addressID, userID)
	return args.Error(0)
}

func (m *MockAddressRepository) SetDisabled(ctx context.Context, addressID, userID int64, disabled bool) error {
	args := m.Called(ctx, addressID, userID, disabled)
	return args.Error(0)
}

func (m *MockAddressRepository) SetDefault(ctx context.Context, userID, addressID int64) error {
	args := m.Called(ctx, userID, addressID)
	return args.Error(0)
}

func (m *MockAddressRepository) Update(ctx context.Context, address *model.Address) error {
	args := m.Called(ctx, address)
	return args.Error(0)
}

// MockUserRepository mocks repository.UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, userID int64) (*model.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	args := m.Called(ctx, phone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, userID int64) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
	args := m.Called(ctx, user, identity)
	return args.Error(0)
}

func (m *MockUserRepository) CreateUserIdentityAndTherapistProfile(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
	args := m.Called(ctx, user, identity)
	return args.Error(0)
}

func (m *MockUserRepository) FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
	args := m.Called(ctx, provider, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.UserAuthIdentity), args.Error(1)
}

func (m *MockUserRepository) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error {
	args := m.Called(ctx, userID, updates)
	return args.Error(0)
}

func (m *MockUserRepository) ListUsers(ctx context.Context, roleFilter string) ([]model.User, error) {
	args := m.Called(ctx, roleFilter)
	return args.Get(0).([]model.User), args.Error(1)
}

func (m *MockUserRepository) ListUsersPaginated(ctx context.Context, role string, page, limit int, search string) ([]model.User, int, error) {
	args := m.Called(ctx, role, page, limit, search)
	return args.Get(0).([]model.User), args.Int(1), args.Error(2)
}

func (m *MockUserRepository) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
	args := m.Called(ctx, blockerID, blockedID)
	return args.Error(0)
}

func (m *MockUserRepository) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
	args := m.Called(ctx, blockerID, blockedID)
	return args.Error(0)
}

func (m *MockUserRepository) IsBlocked(ctx context.Context, userA, userB int64) (bool, error) {
	args := m.Called(ctx, userA, userB)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) GetBlockList(ctx context.Context, userID int64) ([]repository.BlockedUserEntry, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]repository.BlockedUserEntry), args.Error(1)
}

func (m *MockUserRepository) UpdateFCMToken(ctx context.Context, userID int64, token string) error {
	args := m.Called(ctx, userID, token)
	return args.Error(0)
}

func (m *MockUserRepository) GetFCMToken(ctx context.Context, userID int64) (*string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*string), args.Error(1)
}

func (m *MockUserRepository) GetUserInfoBatch(ctx context.Context, userIDs []int64) (map[int64]*repository.UserInfo, error) {
	args := m.Called(ctx, userIDs)
	return args.Get(0).(map[int64]*repository.UserInfo), args.Error(1)
}

func (m *MockUserRepository) GetTherapistInfoBatch(ctx context.Context, therapistIDs []int64) (map[int64]*repository.TherapistInfo, error) {
	args := m.Called(ctx, therapistIDs)
	return args.Get(0).(map[int64]*repository.TherapistInfo), args.Error(1)
}

func (m *MockUserRepository) AddFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	args := m.Called(ctx, userID, therapistID)
	return args.Error(0)
}

func (m *MockUserRepository) RemoveFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	args := m.Called(ctx, userID, therapistID)
	return args.Error(0)
}

func (m *MockUserRepository) ListFavoriteTherapists(ctx context.Context, userID int64) ([]model.User, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.User), args.Error(1)
}

func (m *MockUserRepository) IsTherapistFavorite(ctx context.Context, userID, therapistID int64) (bool, error) {
	args := m.Called(ctx, userID, therapistID)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) BanUserSystem(ctx context.Context, userID int64, reason string) error {
	args := m.Called(ctx, userID, reason)
	return args.Error(0)
}

func (m *MockUserRepository) SuspendUserSystem(ctx context.Context, userID int64, reason string) error {
	args := m.Called(ctx, userID, reason)
	return args.Error(0)
}

func (m *MockUserRepository) SetOneSignalPlayerID(ctx context.Context, userID int64, playerID string) error {
	args := m.Called(ctx, userID, playerID)
	return args.Error(0)
}

// MockTx mocks pgx.Tx
type MockTx struct {
	mock.Mock
	pgx.Tx
}

func (m *MockTx) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) Rollback(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockTx) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgx.Rows), args.Error(1)
}

func (m *MockTx) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgx.Row)
}

// MockRow mocks pgx.Row
type MockRow struct {
	mock.Mock
}

func (m *MockRow) Scan(dest ...any) error {
	args := m.Called(dest...)
	return args.Error(0)
}

// MockRows mocks pgx.Rows
type MockRows struct {
	mock.Mock
	pgx.Rows
}

func (m *MockRows) Close() {
	m.Called()
}

func (m *MockRows) Next() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockRows) Scan(dest ...any) error {
	args := m.Called(dest...)
	return args.Error(0)
}

func (m *MockRows) Err() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRows) CommandTag() pgconn.CommandTag {
	return m.Called().Get(0).(pgconn.CommandTag)
}

func (m *MockRows) FieldDescriptions() []pgconn.FieldDescription {
	return m.Called().Get(0).([]pgconn.FieldDescription)
}

// MockDBTX mocks db.DBTX
type MockDBTX struct {
	mock.Mock
}

func (m *MockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockDBTX) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	args := m.Called(ctx, sql, arguments)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Rows), args.Error(1)
}

func (m *MockDBTX) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgx.Row)
}

func (m *MockDBTX) Begin(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockDBTX) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	args := m.Called(ctx, tableName, columnNames, rowSrc)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDBTX) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	args := m.Called(ctx, b)
	return args.Get(0).(pgx.BatchResults)
}

func (m *MockDBTX) LargeObjects() pgx.LargeObjects {
	args := m.Called()
	return args.Get(0).(pgx.LargeObjects)
}

func (m *MockDBTX) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	args := m.Called(ctx, name, sql)
	return args.Get(0).(*pgconn.StatementDescription), args.Error(1)
}

func (m *MockDBTX) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockMessageService mocks service.MessageServiceInterface
type MockMessageService struct {
	mock.Mock
}

func (m *MockMessageService) CreateConversation(ctx context.Context, initiatorID int64, req *model.CreateConversationRequest) (*model.ConversationResponse, error) {
	args := m.Called(ctx, initiatorID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ConversationResponse), args.Error(1)
}

func (m *MockMessageService) GetConversationsByUser(ctx context.Context, userID int64) ([]model.ConversationResponse, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.ConversationResponse), args.Error(1)
}

func (m *MockMessageService) SendMessage(ctx context.Context, senderID int64, req *model.SendMessageRequest) (*model.Message, error) {
	args := m.Called(ctx, senderID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Message), args.Error(1)
}

func (m *MockMessageService) GetMessagesByConversation(ctx context.Context, conversationID int64, requestingUserID int64, limit, offset int) (*model.PaginatedMessagesResponse, error) {
	args := m.Called(ctx, conversationID, requestingUserID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaginatedMessagesResponse), args.Error(1)
}

func (m *MockMessageService) MarkMessageAsRead(ctx context.Context, messageID, userID int64) error {
	args := m.Called(ctx, messageID, userID)
	return args.Error(0)
}

// MockNotificationService mocks service.NotificationServiceInterface
type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) Create(ctx context.Context, req *model.CreateNotificationRequest) (*model.Notification, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Notification), args.Error(1)
}

func (m *MockNotificationService) SendPushDirect(ctx context.Context, userID int64, notifType, title, message string, data map[string]string) {
	m.Called(ctx, userID, notifType, title, message, data)
}

func (m *MockNotificationService) ListByUser(ctx context.Context, userID int64, limit, offset int) (*model.PaginatedNotificationsResponse, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaginatedNotificationsResponse), args.Error(1)
}

func (m *MockNotificationService) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	args := m.Called(ctx, notificationID, userID)
	return args.Error(0)
}

// MockLogisticsService mocks service.LogisticsServiceInterface
type MockLogisticsService struct {
	mock.Mock
}

func (m *MockLogisticsService) HandleBookingAssigned(ctx context.Context, bookingID int64) error {
	args := m.Called(ctx, bookingID)
	return args.Error(0)
}

func (m *MockLogisticsService) CancelRideForBooking(ctx context.Context, bookingID int64) error {
	args := m.Called(ctx, bookingID)
	return args.Error(0)
}

func (m *MockLogisticsService) UpdateRideForBooking(ctx context.Context, bookingID int64) error {
	args := m.Called(ctx, bookingID)
	return args.Error(0)
}
func (m *MockBookingRepository) ListByRecurringID(ctx context.Context, recurringID int64, after time.Time, limit int) ([]model.Booking, error) { return nil, nil }
