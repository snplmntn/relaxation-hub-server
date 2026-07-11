package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/mock"
)

// stubDBTX is a permissive db.DBTX that returns harmless zero values, used when a test needs a
// non-nil pool only to satisfy `if s.db != nil` guards (e.g. the post-completion adjustment tx,
// or the fire-and-forget broadcast) without wiring testify expectations for every query.
type stubRow struct{}

func (stubRow) Scan(dest ...any) error { return errors.New("stub row") }

type stubDBTX struct{}

func (stubDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (stubDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("stub query")
}
func (stubDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return stubRow{}
}
func (stubDBTX) Begin(ctx context.Context) (pgx.Tx, error) { return nil, errors.New("stub begin") }
func (stubDBTX) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (stubDBTX) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }

// mockBookingRepoAdmin is a minimal BookingRepository for admin-create tests
type mockBookingRepoAdmin struct {
	createdBooking *model.Booking
	assignErr      error

	// Captured by AdjustCompletedBookingFinancialsTx for assertions.
	adjustCalled        bool
	adjustRevenueDelta  float64
	adjustEarningsDelta float64
}

func (m *mockBookingRepoAdmin) UpdatePayoutReference(ctx context.Context, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoAdmin) UpdatePayoutReferenceTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoAdmin) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*repository.BookingDetailsResult, error) {
	return nil, nil
}

func (m *mockBookingRepoAdmin) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAdmin) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	// simulate DB assigning an ID
	booking.BookingID = 555
	m.createdBooking = booking
	return nil
}
func (m *mockBookingRepoAdmin) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	return nil, pgx.ErrNoRows
}
func (m *mockBookingRepoAdmin) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAdmin) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	return nil
}
func (m *mockBookingRepoAdmin) UpdateAdmin(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoAdmin) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoAdmin) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	return m.assignErr
}
func (m *mockBookingRepoAdmin) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	if m.createdBooking == nil {
		return nil, pgx.ErrNoRows
	}
	return m.createdBooking, nil
}
func (m *mockBookingRepoAdmin) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) {
	return m.GetByBookingID(ctx, bookingID)
}
func (m *mockBookingRepoAdmin) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return []model.BookingEvent{}, nil
}
func (m *mockBookingRepoAdmin) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockBookingRepoAdmin) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	return nil
}
func (m *mockBookingRepoAdmin) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return nil
}
func (m *mockBookingRepoAdmin) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}
func (m *mockBookingRepoAdmin) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.createdBooking}, nil
}
func (m *mockBookingRepoAdmin) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.createdBooking}, nil
}
func (m *mockBookingRepoAdmin) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.createdBooking}, nil
}
func (m *mockBookingRepoAdmin) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.createdBooking}, nil
}
func (m *mockBookingRepoAdmin) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoAdmin) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoAdmin) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return map[int64]int{}, nil
}
func (m *mockBookingRepoAdmin) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	return nil
}
func (m *mockBookingRepoAdmin) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	return nil
}
func (m *mockBookingRepoAdmin) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) ListDueInProgressBookings(ctx context.Context, now time.Time, limit int) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockBookingRepoAdmin) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockBookingRepoAdmin) ListUpcomingBookingsForReminder(ctx context.Context, windowStart, windowEnd time.Time, eventTypeExclude string) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockBookingRepoAdmin) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
	return nil
}
func (m *mockBookingRepoAdmin) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockBookingRepoAdmin) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
	return &repository.ClientBookingStats{}, nil
}
func (m *mockBookingRepoAdmin) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) {
	return &repository.AccountingSummary{}, nil
}
func (m *mockBookingRepoAdmin) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) HasAssignedOutboundRiderCoverage(ctx context.Context, bookingID int64) (bool, error) {
	return true, nil
}

func (m *mockBookingRepoAdmin) RevertOnTheWayToAssigned(ctx context.Context, bookingID, actorID int64) (*repository.RevertOnTheWayToAssignedResult, error) {
	return nil, nil
}

func (m *mockBookingRepoAdmin) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoAdmin) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoAdmin) AdjustCompletedBookingFinancialsTx(ctx context.Context, pool db.DBTX, booking *model.Booking, revenueDelta, earningsDelta float64, entryDate time.Time) error {
	m.adjustCalled = true
	m.adjustRevenueDelta = revenueDelta
	m.adjustEarningsDelta = earningsDelta
	return nil
}
func (m *mockBookingRepoAdmin) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status, dateFrom, dateTo string) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}

// mockTherapistRepoAdmin controls GetProfile behavior
type mockTherapistRepoAdmin struct {
	profile               *model.TherapistProfile
	err                   error
	servicesWithPressures map[int64][]string
}

func (m *mockTherapistRepoAdmin) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.profile, nil
}
func (m *mockTherapistRepoAdmin) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error {
	return nil
}
func (m *mockTherapistRepoAdmin) SetLifecycleStatus(ctx context.Context, therapistID int64, accountStatus string, acceptAssignments bool) error {
	return nil
}
func (m *mockTherapistRepoAdmin) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoAdmin) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error {
	return nil
}
func (m *mockTherapistRepoAdmin) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) {
	return nil, nil
}
func (m *mockTherapistRepoAdmin) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error {
	return nil
}
func (m *mockTherapistRepoAdmin) AddService(ctx context.Context, ts *model.TherapistService) error {
	return nil
}
func (m *mockTherapistRepoAdmin) RemoveService(ctx context.Context, therapistID, serviceID int64) error {
	return nil
}
func (m *mockTherapistRepoAdmin) GetServices(ctx context.Context, therapistID int64) ([]int64, error) {
	return nil, nil
}
func (m *mockTherapistRepoAdmin) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error {
	return nil
}
func (m *mockTherapistRepoAdmin) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) {
	if m.servicesWithPressures != nil {
		return m.servicesWithPressures, nil
	}
	return map[int64][]string{}, nil
}
func (m *mockTherapistRepoAdmin) SetBatchServices(ctx context.Context, therapistID int64, serviceIDs []model.AddServiceWithPressuresRequest) error {
	return nil
}

func (m *mockTherapistRepoAdmin) CreateProfile(ctx context.Context, therapistID int64) error {
	return nil
}
func (m *mockTherapistRepoAdmin) FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoAdmin) FindAvailableByServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoAdmin) FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoAdmin) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoAdmin) SetAtBranch(ctx context.Context, therapistID int64, atBranch bool) error {
	return nil
}
func (m *mockTherapistRepoAdmin) TryLockTherapist(ctx context.Context, therapistID int64) (bool, error) {
	return true, nil
}
func (m *mockTherapistRepoAdmin) TryLockTherapistTx(ctx context.Context, tx pgx.Tx, therapistID int64) (bool, error) {
	return true, nil
}

// mockServiceRepoAdmin for test
type mockServiceRepoAdmin struct {
	service  *model.Service
	services map[int64]*model.Service
	err      error
}

func (m *mockServiceRepoAdmin) Create(ctx context.Context, svc *model.Service) error { return nil }
func (m *mockServiceRepoAdmin) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.services != nil {
		return m.services[serviceID], nil
	}
	return m.service, nil
}
func (m *mockServiceRepoAdmin) GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoAdmin) ListActive(ctx context.Context) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoAdmin) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoAdmin) ListPopular(ctx context.Context) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoAdmin) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	return nil, nil
}
func (m *mockServiceRepoAdmin) Update(ctx context.Context, serviceID int64, updates map[string]interface{}) error {
	return nil
}
func (m *mockServiceRepoAdmin) Delete(ctx context.Context, serviceID int64) error { return nil }

type mockBookingServiceRepoAdmin struct {
	created []model.BookingService
}

func (m *mockBookingServiceRepoAdmin) CreateManyTx(_ context.Context, _ pgx.Tx, services []model.BookingService) error {
	m.created = append([]model.BookingService(nil), services...)
	return nil
}
func (m *mockBookingServiceRepoAdmin) ListByBookingID(context.Context, int64) ([]model.BookingService, error) {
	return m.created, nil
}
func (m *mockBookingServiceRepoAdmin) ListByBookingIDWithService(context.Context, int64) ([]model.BookingService, error) {
	return m.created, nil
}
func (m *mockBookingServiceRepoAdmin) ReplaceByBookingID(_ context.Context, _ int64, services []model.BookingService, _ []byte) error {
	m.created = append([]model.BookingService(nil), services...)
	return nil
}
func (m *mockBookingServiceRepoAdmin) DeleteByBookingIDTx(context.Context, pgx.Tx, int64) error {
	return nil
}

type mockBookingReferralRepoAdmin struct {
	created *model.BookingReferral
}

func (m *mockBookingReferralRepoAdmin) CreateTx(_ context.Context, _ pgx.Tx, referral *model.BookingReferral) error {
	copy := *referral
	m.created = &copy
	return nil
}
func (m *mockBookingReferralRepoAdmin) ListSummaryTotals(context.Context, time.Time, time.Time) ([]model.BookingReferralSummaryTotal, error) {
	return nil, nil
}
func (m *mockBookingReferralRepoAdmin) ListSummarySeries(context.Context, time.Time, time.Time, string) ([]model.BookingReferralSummaryPoint, error) {
	return nil, nil
}

func TestAdminCreate_Assignment_TherapistNotFound(t *testing.T) {
	ctx := context.Background()
	br := &mockBookingRepoAdmin{}
	tr := &mockTherapistRepoAdmin{err: pgx.ErrNoRows}
	svc := NewBookingService(br, nil, nil, &nilAssignmentQueueRepo{}, tr, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := &model.CreateBookingRequest{DurationMinutes: 60, TherapistID: func() *int64 { v := int64(9); return &v }()}
	_, err := svc.CreateForAdmin(ctx, 1, 2, req)
	if err == nil {
		t.Fatalf("expected error when therapist not found")
	}
	if ve, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	} else if ve.Code != "invalid_therapist" {
		t.Fatalf("expected code invalid_therapist, got %s", ve.Code)
	}
}

func TestAdminCreate_Assignment_TherapistNotAccepting(t *testing.T) {
	ctx := context.Background()
	br := &mockBookingRepoAdmin{}
	tr := &mockTherapistRepoAdmin{profile: &model.TherapistProfile{TherapistID: 9, AcceptAssignments: false}}
	svc := NewBookingService(br, nil, nil, &nilAssignmentQueueRepo{}, tr, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := &model.CreateBookingRequest{DurationMinutes: 60, TherapistID: func() *int64 { v := int64(9); return &v }()}
	_, err := svc.CreateForAdmin(ctx, 1, 2, req)
	if err == nil {
		t.Fatalf("expected error when therapist not accepting")
	}
	if ve, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	} else if ve.Code != "therapist_not_accepting" {
		t.Fatalf("expected code therapist_not_accepting, got %s", ve.Code)
	}
}

func TestAdminCreate_Assignment_RaceConditionAssignFails(t *testing.T) {
	ctx := context.Background()
	br := &mockBookingRepoAdmin{assignErr: pgx.ErrNoRows}
	tr := &mockTherapistRepoAdmin{profile: &model.TherapistProfile{TherapistID: 9, Status: "active", AcceptAssignments: true}}
	svc := NewBookingService(br, nil, nil, &nilAssignmentQueueRepo{}, tr, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := &model.CreateBookingRequest{DurationMinutes: 60, TherapistID: func() *int64 { v := int64(9); return &v }()}
	_, err := svc.CreateForAdmin(ctx, 1, 2, req)
	if err == nil {
		t.Fatalf("expected error when assign fails due to gating race")
	}
	if ve, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	} else if ve.Code != "cannot_assign" {
		t.Fatalf("expected code cannot_assign, got %s", ve.Code)
	}
}

func TestAdminCreate_Assignment_ServiceNotOffered(t *testing.T) {
	ctx := context.Background()
	br := &mockBookingRepoAdmin{assignErr: repository.ErrServiceNotOffered}
	tr := &mockTherapistRepoAdmin{profile: &model.TherapistProfile{TherapistID: 9, Status: "active", AcceptAssignments: true}}
	svc := NewBookingService(br, nil, nil, &nilAssignmentQueueRepo{}, tr, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := &model.CreateBookingRequest{DurationMinutes: 60, TherapistID: func() *int64 { v := int64(9); return &v }()}
	_, err := svc.CreateForAdmin(ctx, 1, 2, req)
	if err == nil {
		t.Fatalf("expected error when therapist does not offer service")
	}
	if ve, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	} else if ve.Code != "service_not_offered" {
		t.Fatalf("expected code service_not_offered, got %s", ve.Code)
	}
}

// Added Test for Missing Total Calculation
func TestBookingService_CreateForAdmin_MissingTotal(t *testing.T) {
	ctx := context.Background()
	mockRepo := &mockBookingRepoAdmin{}
	serviceID := int64(5)
	mockServiceRepo := &mockServiceRepoAdmin{
		service: &model.Service{
			ServiceID:       serviceID,
			Name:            "Test Massage",
			BasePrice:       500.0,
			DurationMinutes: 60,
		},
	}
	therapistID := int64(202)
	mockTherapistRepo := &mockTherapistRepoAdmin{
		profile: &model.TherapistProfile{
			TherapistID:       therapistID,
			Status:            "active",
			AcceptAssignments: true,
		},
	}

	s := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, mockTherapistRepo, nil, mockServiceRepo, nil, nil, nil, nil, nil, nil, nil)

	clientID := int64(101)
	adminID := int64(999)
	addressID := int64(10)

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		TherapistID:     &therapistID,
		DurationMinutes: 60,
		PaymentMethod:   "cash",
		// Total and RawTotal are intentionally missing/nil
	}

	booking, err := s.CreateForAdmin(ctx, adminID, clientID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if booking == nil {
		t.Fatalf("booking should not be nil")
	}
	if booking.FinalTotal == nil {
		t.Fatalf("FinalTotal should not be nil")
	}
	if *booking.FinalTotal != 500.0 {
		t.Errorf("expected FinalTotal 500.0, got %f", *booking.FinalTotal)
	}
}

// Pricing must be charged against the service's OWN minimum duration, not a
// fixed 60-minute baseline — and the server must override any (incorrect)
// client-supplied total. e.g. Baso: 1099 for 120 min => a 120-min booking is 1099.
func TestBookingService_CreateForAdmin_PerServiceDurationBaseline(t *testing.T) {
	ctx := context.Background()
	mockRepo := &mockBookingRepoAdmin{}
	serviceID := int64(5)
	mockServiceRepo := &mockServiceRepoAdmin{
		service: &model.Service{
			ServiceID:       serviceID,
			Name:            "Baso / Ventosa Massage",
			BasePrice:       1099.0,
			DurationMinutes: 120,
		},
	}
	therapistID := int64(202)
	mockTherapistRepo := &mockTherapistRepoAdmin{
		profile: &model.TherapistProfile{
			TherapistID:       therapistID,
			Status:            "active",
			AcceptAssignments: true,
		},
	}

	s := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, mockTherapistRepo, nil, mockServiceRepo, nil, nil, nil, nil, nil, nil, nil)

	clientID := int64(101)
	adminID := int64(999)
	addressID := int64(10)
	buggyClientTotal := 2198.0

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		TherapistID:     &therapistID,
		DurationMinutes: 120,
		PaymentMethod:   "cash",
		// Client sends the wrong, per-60-minute total; the server must override it.
		RawTotal: &buggyClientTotal,
		Total:    &buggyClientTotal,
	}

	booking, err := s.CreateForAdmin(ctx, adminID, clientID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if booking.RawTotal == nil || *booking.RawTotal != 1099.0 {
		t.Errorf("expected RawTotal 1099.0, got %v", booking.RawTotal)
	}
	if booking.FinalTotal == nil || *booking.FinalTotal != 1099.0 {
		t.Errorf("expected FinalTotal 1099.0, got %v", booking.FinalTotal)
	}
}

func TestBookingService_CreateForAdmin_PersistsAllSelectedServices(t *testing.T) {
	ctx := context.Background()
	bookingRepo := &mockBookingRepoAdmin{}
	therapistID := int64(202)
	serviceRepo := &mockServiceRepoAdmin{services: map[int64]*model.Service{
		5: {ServiceID: 5, Name: "Signature Massage", BasePrice: 700, DurationMinutes: 120},
		6: {ServiceID: 6, Name: "Foot Massage", BasePrice: 500, DurationMinutes: 60},
	}}
	therapistRepo := &mockTherapistRepoAdmin{
		profile: &model.TherapistProfile{TherapistID: therapistID, Status: "active", AcceptAssignments: true},
		servicesWithPressures: map[int64][]string{
			5: {"medium"},
			6: {"medium"},
		},
	}
	bookingServices := &mockBookingServiceRepoAdmin{}
	bookingReferrals := &mockBookingReferralRepoAdmin{}
	svc := NewBookingService(bookingRepo, nil, nil, &nilAssignmentQueueRepo{}, therapistRepo, nil, serviceRepo, nil, nil, nil, nil, nil, nil, nil)
	svc.SetBookingServiceRepository(bookingServices)
	svc.SetBookingReferralRepository(bookingReferrals)

	primaryID := int64(5)
	changeFor := 1500.0
	booking, err := svc.CreateForAdmin(ctx, 999, 101, &model.CreateBookingRequest{
		ServiceID:        &primaryID,
		ServiceIDs:       []int64{5, 6},
		ServiceDurations: []model.BookingServiceDurationAllocation{{ServiceID: 5, DurationMinutes: 90}, {ServiceID: 6, DurationMinutes: 90}},
		TherapistID:      &therapistID,
		DurationMinutes:  180,
		PressurePref:     "medium",
		PaymentMethod:    "cash",
		ChangeFor:        &changeFor,
		ReferralSource:   model.BookingReferralSourcePhone,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if booking.ServiceID == nil || *booking.ServiceID != 5 {
		t.Fatalf("expected primary service 5, got %v", booking.ServiceID)
	}
	if len(bookingServices.created) != 2 {
		t.Fatalf("expected 2 persisted services, got %d", len(bookingServices.created))
	}
	if bookingServices.created[0].AllocatedDurationMinutes == nil || *bookingServices.created[0].AllocatedDurationMinutes != 90 ||
		bookingServices.created[1].AllocatedDurationMinutes == nil || *bookingServices.created[1].AllocatedDurationMinutes != 90 {
		t.Fatalf("expected persisted 90/90 service allocation, got %#v", bookingServices.created)
	}
	if booking.RawTotal == nil || *booking.RawTotal != 1200 {
		t.Fatalf("expected summed raw total 1200, got %v", booking.RawTotal)
	}
	if booking.ChangeFor == nil || *booking.ChangeFor != changeFor {
		t.Fatalf("expected change-for %.2f, got %v", changeFor, booking.ChangeFor)
	}
	if bookingReferrals.created == nil || bookingReferrals.created.Source != model.BookingReferralSourcePhone {
		t.Fatalf("expected Phone referral to be persisted, got %v", bookingReferrals.created)
	}
	if bookingReferrals.created.CreatedByUserID != 999 {
		t.Fatalf("expected admin 999 as referral creator, got %d", bookingReferrals.created.CreatedByUserID)
	}
}

func TestBookingService_ApplyBookingEdit_ReplacesServicesAndReprices(t *testing.T) {
	serviceRepo := &mockServiceRepoAdmin{services: map[int64]*model.Service{
		5: {ServiceID: 5, Name: "Signature Massage", BasePrice: 700, DurationMinutes: 120, IsActive: true},
		6: {ServiceID: 6, Name: "Foot Massage", BasePrice: 500, DurationMinutes: 60, IsActive: true},
	}}
	svc := NewBookingService(&mockBookingRepoAdmin{}, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, serviceRepo, nil, nil, nil, nil, nil, nil, nil)
	oldServiceID := int64(5)
	booking := &model.Booking{
		BookingID:       777,
		ClientID:        101,
		ServiceID:       &oldServiceID,
		DurationMinutes: 120,
		Status:          model.BookingStatusAssigned,
	}
	duration := 180

	_, _, matchingChanged, err := svc.applyBookingEditableFields(context.Background(), booking, &model.UpdateBookingRequest{
		ServiceIDs:       []int64{5, 6},
		ServiceDurations: []model.BookingServiceDurationAllocation{{ServiceID: 5, DurationMinutes: 105}, {ServiceID: 6, DurationMinutes: 75}},
		DurationMinutes:  &duration,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matchingChanged {
		t.Fatal("expected service edit to mark matching fields changed")
	}
	if len(booking.Services) != 2 {
		t.Fatalf("expected 2 edited services, got %d", len(booking.Services))
	}
	if booking.Services[0].AllocatedDurationMinutes == nil || *booking.Services[0].AllocatedDurationMinutes != 105 ||
		booking.Services[1].AllocatedDurationMinutes == nil || *booking.Services[1].AllocatedDurationMinutes != 75 {
		t.Fatalf("expected edited 105/75 service allocation, got %#v", booking.Services)
	}
	if booking.RawTotal == nil || *booking.RawTotal != 1200 {
		t.Fatalf("expected summed raw total 1200, got %v", booking.RawTotal)
	}
}

func TestApplyBookingServiceDurationAllocations_RejectsMismatchedTotal(t *testing.T) {
	selection := &resolvedBookingServices{Items: []model.BookingService{
		{ServiceID: 5},
		{ServiceID: 6},
	}}

	err := applyBookingServiceDurationAllocations(selection, []model.BookingServiceDurationAllocation{
		{ServiceID: 5, DurationMinutes: 90},
		{ServiceID: 6, DurationMinutes: 75},
	}, 180)
	if err == nil {
		t.Fatal("expected a total-mismatch validation error")
	}
	validationErr, ok := err.(*ValidationError)
	if !ok || validationErr.Code != "service_duration_total_mismatch" {
		t.Fatalf("expected service_duration_total_mismatch, got %#v", err)
	}
}

// Editing a COMPLETED booking's duration must recompute the price, therapist earnings, and
// platform fee, and reconcile the ledger via AdjustCompletedBookingFinancialsTx with the
// correct deltas — not silently change only the displayed hours.
func TestBookingService_UpdateByAdmin_CompletedDurationAdjustment(t *testing.T) {
	ctx := context.Background()
	serviceID := int64(5)
	therapistID := int64(202)
	rawTotal := 1500.0
	finalTotal := 1500.0
	completed := &model.Booking{
		BookingID:       777,
		ClientID:        101,
		TherapistID:     &therapistID,
		ServiceID:       &serviceID,
		DurationMinutes: 60,
		RawTotal:        &rawTotal,
		FinalTotal:      &finalTotal,
		Status:          model.BookingStatusCompleted,
	}
	mockRepo := &mockBookingRepoAdmin{createdBooking: completed}
	commission := 500.0
	mockServiceRepo := &mockServiceRepoAdmin{
		service: &model.Service{
			ServiceID:           serviceID,
			Name:                "Test Massage",
			BasePrice:           1500.0,
			DurationMinutes:     60,
			TherapistCommission: &commission,
		},
	}

	// db is non-nil so the reconciliation tx path runs; walletService is nil (skipped).
	s := NewBookingService(mockRepo, nil, stubDBTX{}, &nilAssignmentQueueRepo{}, nil, nil, mockServiceRepo, nil, nil, nil, nil, nil, nil, nil)

	newDuration := 120
	req := &model.UpdateBookingRequest{DurationMinutes: &newDuration}

	result, err := s.UpdateByAdminWithMeta(ctx, 999, completed.BookingID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b := result.Booking
	if b.RawTotal == nil || *b.RawTotal != 3000.0 {
		t.Errorf("expected RawTotal 3000, got %v", b.RawTotal)
	}
	if b.FinalTotal == nil || *b.FinalTotal != 3000.0 {
		t.Errorf("expected FinalTotal 3000, got %v", b.FinalTotal)
	}
	if b.TherapistEarnings == nil || *b.TherapistEarnings != 1000.0 {
		t.Errorf("expected therapist earnings 1000, got %v", b.TherapistEarnings)
	}
	if b.PlatformFee == nil || *b.PlatformFee != 2000.0 {
		t.Errorf("expected platform fee 2000, got %v", b.PlatformFee)
	}
	if !mockRepo.adjustCalled {
		t.Fatalf("expected AdjustCompletedBookingFinancialsTx to be called")
	}
	if mockRepo.adjustRevenueDelta != 1500.0 {
		t.Errorf("expected revenue delta 1500, got %f", mockRepo.adjustRevenueDelta)
	}
	if mockRepo.adjustEarningsDelta != 500.0 {
		t.Errorf("expected earnings delta 500, got %f", mockRepo.adjustEarningsDelta)
	}
}

func TestBookingService_CreateForAdmin_UsesAddressCoordinatesForServiceability(t *testing.T) {
	ctx := context.Background()
	adminID := int64(999)
	clientID := int64(101)
	serviceID := int64(5)
	addressID := int64(10)
	therapistID := int64(202)
	coveredLat := 14.5547
	coveredLng := 121.0244

	mockRepo := &mockBookingRepoAdmin{}
	mockServiceRepo := &mockServiceRepoAdmin{
		service: &model.Service{
			ServiceID:       serviceID,
			Name:            "Test Massage",
			BasePrice:       500.0,
			DurationMinutes: 60,
		},
	}
	mockTherapistRepo := &mockTherapistRepoAdmin{
		profile: &model.TherapistProfile{
			TherapistID:       therapistID,
			Status:            "active",
			AcceptAssignments: true,
		},
	}
	mockAddressRepo := new(MockAddressRepository)
	mockAddressRepo.On("GetByID", mock.Anything, addressID, clientID).Return(&model.Address{
		AddressID: addressID,
		UserID:    clientID,
		City:      "Unmatched map label",
		Barangay:  "Unmatched map place",
		Latitude:  &coveredLat,
		Longitude: &coveredLng,
	}, nil)
	areaRepo := &bookingServiceabilityAreaRepo{
		areasByStatus: []model.ServiceArea{
			{
				AreaKey:           "city:makati",
				Name:              "Makati",
				Level:             model.ServiceAreaLevelCity,
				Status:            model.ServiceAreaStatusCovered,
				Lat:               &coveredLat,
				Lng:               &coveredLng,
				MinBookingMinutes: 60,
			},
		},
	}

	svc := NewBookingService(
		mockRepo,
		nil,
		nil,
		&nilAssignmentQueueRepo{},
		mockTherapistRepo,
		nil,
		mockServiceRepo,
		mockAddressRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewLocationService(areaRepo),
	)

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		TherapistID:     &therapistID,
		DurationMinutes: 60,
		PaymentMethod:   "cash",
	}

	booking, err := svc.CreateForAdmin(ctx, adminID, clientID, req)
	if err != nil {
		t.Fatalf("expected coordinates in a covered area to pass serviceability, got %v", err)
	}
	if booking == nil {
		t.Fatalf("booking should not be nil")
	}
	if len(areaRepo.recordedKeys) != 0 {
		t.Fatalf("expected coordinate path not to record unsupported-name interest, got %v", areaRepo.recordedKeys)
	}
	if len(areaRepo.upsertedAreas) != 0 {
		t.Fatalf("expected coordinate path not to upsert unsupported names, got %d", len(areaRepo.upsertedAreas))
	}
	mockAddressRepo.AssertExpectations(t)
}

func TestBookingService_CreateForAdmin_RejectsCoordinatesWhenCityIsBanned(t *testing.T) {
	ctx := context.Background()
	adminID := int64(999)
	clientID := int64(101)
	serviceID := int64(5)
	addressID := int64(10)
	therapistID := int64(202)
	coveredLat := 14.5547
	coveredLng := 121.0244
	bannedArea := &model.ServiceArea{
		AreaKey:           "city:banned-city",
		Name:              "Banned City",
		Level:             model.ServiceAreaLevelCity,
		Status:            model.ServiceAreaStatusBanned,
		MinBookingMinutes: 60,
	}

	mockRepo := &mockBookingRepoAdmin{}
	mockServiceRepo := &mockServiceRepoAdmin{
		service: &model.Service{
			ServiceID:       serviceID,
			Name:            "Test Massage",
			BasePrice:       500.0,
			DurationMinutes: 60,
		},
	}
	mockTherapistRepo := &mockTherapistRepoAdmin{
		profile: &model.TherapistProfile{
			TherapistID:       therapistID,
			Status:            "active",
			AcceptAssignments: true,
		},
	}
	mockAddressRepo := new(MockAddressRepository)
	mockAddressRepo.On("GetByID", mock.Anything, addressID, clientID).Return(&model.Address{
		AddressID: addressID,
		UserID:    clientID,
		City:      "Banned City",
		Barangay:  "Unmatched map place",
		Latitude:  &coveredLat,
		Longitude: &coveredLng,
	}, nil)
	areaRepo := &bookingServiceabilityAreaRepo{
		areasByName: map[string]*model.ServiceArea{
			bookingServiceabilityNameKey(string(model.ServiceAreaLevelCity), "Banned City"): bannedArea,
		},
		areasByStatus: []model.ServiceArea{
			{
				AreaKey:           "city:makati",
				Name:              "Makati",
				Level:             model.ServiceAreaLevelCity,
				Status:            model.ServiceAreaStatusCovered,
				Lat:               &coveredLat,
				Lng:               &coveredLng,
				MinBookingMinutes: 60,
			},
		},
	}

	svc := NewBookingService(
		mockRepo,
		nil,
		nil,
		&nilAssignmentQueueRepo{},
		mockTherapistRepo,
		nil,
		mockServiceRepo,
		mockAddressRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewLocationService(areaRepo),
	)

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		TherapistID:     &therapistID,
		DurationMinutes: 60,
		PaymentMethod:   "cash",
	}

	booking, err := svc.CreateForAdmin(ctx, adminID, clientID, req)
	if err == nil {
		t.Fatalf("expected banned city coordinates to fail serviceability")
	}
	if booking != nil {
		t.Fatalf("expected booking to be nil, got %#v", booking)
	}
	if ve, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	} else if ve.Code != "location_not_serviceable" {
		t.Fatalf("expected code location_not_serviceable, got %s", ve.Code)
	}
	if mockRepo.createdBooking != nil {
		t.Fatalf("expected booking not to be created")
	}
	if len(areaRepo.recordedKeys) != 1 || areaRepo.recordedKeys[0] != "city:banned-city" {
		t.Fatalf("expected banned city interest record, got %v", areaRepo.recordedKeys)
	}
	mockAddressRepo.AssertExpectations(t)
}

func (m *mockBookingRepoAdmin) ListAllEvents(ctx context.Context, params repository.ListAllEventsParams) ([]model.BookingEvent, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoAdmin) ListByRecurringID(ctx context.Context, recurringID int64, after time.Time, limit int) ([]model.Booking, error) {
	return nil, nil
}
