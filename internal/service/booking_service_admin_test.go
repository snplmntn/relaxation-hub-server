package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/mock"
)

// mockBookingRepoAdmin is a minimal BookingRepository for admin-create tests
type mockBookingRepoAdmin struct {
	createdBooking *model.Booking
	assignErr      error
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
func (m *mockBookingRepoAdmin) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}

// mockTherapistRepoAdmin controls GetProfile behavior
type mockTherapistRepoAdmin struct {
	profile *model.TherapistProfile
	err     error
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
	service *model.Service
	err     error
}

func (m *mockServiceRepoAdmin) Create(ctx context.Context, svc *model.Service) error { return nil }
func (m *mockServiceRepoAdmin) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	if m.err != nil {
		return nil, m.err
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
