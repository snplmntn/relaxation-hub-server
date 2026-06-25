package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type bookingServiceabilityAreaRepo struct {
	areaByName    *model.ServiceArea
	areasByName   map[string]*model.ServiceArea
	areasByStatus []model.ServiceArea
	recordedKeys  []string
	upsertedAreas []model.ServiceArea
}

func (r *bookingServiceabilityAreaRepo) GetByKey(context.Context, string) (*model.ServiceArea, error) {
	return nil, repository.ErrAreaNotFound
}

func (r *bookingServiceabilityAreaRepo) GetByName(_ context.Context, name string, level model.ServiceAreaLevel) (*model.ServiceArea, error) {
	if len(r.areasByName) > 0 {
		area, ok := r.areasByName[bookingServiceabilityNameKey(string(level), name)]
		if !ok {
			return nil, repository.ErrAreaNotFound
		}
		return area, nil
	}
	if r.areaByName == nil {
		return nil, repository.ErrAreaNotFound
	}
	return r.areaByName, nil
}

func bookingServiceabilityNameKey(level, name string) string {
	return level + ":" + strings.ToLower(strings.TrimSpace(name))
}

func (r *bookingServiceabilityAreaRepo) GetStatusByKey(context.Context, string) (model.ServiceAreaStatus, error) {
	return model.ServiceAreaStatusNotSupported, nil
}

func (r *bookingServiceabilityAreaRepo) ListByStatus(_ context.Context, status model.ServiceAreaStatus) ([]model.ServiceArea, error) {
	var areas []model.ServiceArea
	for _, area := range r.areasByStatus {
		if area.Status == status {
			areas = append(areas, area)
		}
	}
	return areas, nil
}

func (r *bookingServiceabilityAreaRepo) ListAll(context.Context) ([]model.ServiceArea, error) {
	return nil, nil
}

func (r *bookingServiceabilityAreaRepo) ListTopDemand(context.Context, int) ([]model.ServiceArea, error) {
	return nil, nil
}

func (r *bookingServiceabilityAreaRepo) UpdateStatus(context.Context, string, model.ServiceAreaStatus) error {
	return nil
}

func (r *bookingServiceabilityAreaRepo) UpsertArea(_ context.Context, area *model.ServiceArea) error {
	r.upsertedAreas = append(r.upsertedAreas, *area)
	return nil
}

func (r *bookingServiceabilityAreaRepo) RecordInterest(_ context.Context, _ int64, areaKey string) error {
	r.recordedKeys = append(r.recordedKeys, areaKey)
	return nil
}

func (r *bookingServiceabilityAreaRepo) GetInterestCount(context.Context, string) (int, error) {
	return 0, nil
}

func (r *bookingServiceabilityAreaRepo) ListInterestedUsers(context.Context, string) ([]int64, error) {
	return nil, nil
}

func (r *bookingServiceabilityAreaRepo) ListInterestedUsersPage(context.Context, string, int, int) ([]model.AreaInterestedUser, int, error) {
	return nil, 0, nil
}

func TestBookingService_Create(t *testing.T) {
	// Common test data
	clientID := int64(100)
	serviceID := int64(1)
	addressID := int64(5)
	now := time.Now()
	scheduledStart := now.Add(2 * time.Hour)

	validRequest := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		ScheduledStart:  scheduledStart.Format(time.RFC3339),
		GenderPref:      "female",
		PressurePref:    "medium",
		Notes:           "Gate code 1234",
		PaymentMethod:   "card",
		DurationMinutes: 60,
	}

	validService := &model.Service{
		ServiceID:       serviceID,
		Name:            "Swedish Massage",
		DurationMinutes: 60,
		BasePrice:       100.0,
		IsActive:        true,
	}

	validAddress := &model.Address{
		AddressID: addressID,
		UserID:    clientID,
	}

	// Define test cases
	tests := []struct {
		name          string
		request       *model.CreateBookingRequest
		setupMocks    func(m *MockBookingRepository, ms *MockServiceRepository, ma *MockAddressRepository, mp *MockPromoRepository, mq *MockAssignmentQueueRepository)
		expectedError string
	}{
		{
			name:    "Success - Basic Booking",
			request: validRequest,
			setupMocks: func(m *MockBookingRepository, ms *MockServiceRepository, ma *MockAddressRepository, mp *MockPromoRepository, mq *MockAssignmentQueueRepository) {
				// Validate Service
				ms.On("GetByID", mock.Anything, serviceID).Return(validService, nil)

				// Validate Address
				ma.On("GetByID", mock.Anything, addressID, clientID).Return(validAddress, nil)

				// Create Booking
				m.On("CreateTx", mock.Anything, mock.Anything, mock.MatchedBy(func(b *model.Booking) bool {
					return b.ClientID == clientID && b.ServiceID != nil && *b.ServiceID == serviceID &&
						b.Status == "pending" && b.FinalTotal != nil && *b.FinalTotal == 100.0
				})).Return(nil)

				// Enqueue
				mq.On("EnqueueTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				// Log Event
				m.On("InsertEvent", mock.Anything, mock.AnythingOfType("int64"), "created", mock.Anything, mock.Anything).Return(nil)
			},
			expectedError: "",
		},
		{
			name:    "Failure - Service Not Found",
			request: validRequest,
			setupMocks: func(m *MockBookingRepository, ms *MockServiceRepository, ma *MockAddressRepository, mp *MockPromoRepository, mq *MockAssignmentQueueRepository) {
				ms.On("GetByID", mock.Anything, serviceID).Return(nil, errors.New("service not found"))
			},
			expectedError: "service not found",
		},
		{
			name:    "Failure - Address Not Found",
			request: validRequest,
			setupMocks: func(m *MockBookingRepository, ms *MockServiceRepository, ma *MockAddressRepository, mp *MockPromoRepository, mq *MockAssignmentQueueRepository) {
				ms.On("GetByID", mock.Anything, serviceID).Return(validService, nil)
				ma.On("GetByID", mock.Anything, addressID, clientID).Return(nil, errors.New("address not found"))
			},
			expectedError: "address not found",
		},
		{
			name:    "Failure - Booking Creation DB Error",
			request: validRequest,
			setupMocks: func(m *MockBookingRepository, ms *MockServiceRepository, ma *MockAddressRepository, mp *MockPromoRepository, mq *MockAssignmentQueueRepository) {
				ms.On("GetByID", mock.Anything, serviceID).Return(validService, nil)
				ma.On("GetByID", mock.Anything, addressID, clientID).Return(validAddress, nil)

				m.On("CreateTx", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db create failed"))
			},
			expectedError: "db create failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize Mocks
			mockRepo := new(MockBookingRepository)
			mockServiceRepo := new(MockServiceRepository)
			mockAddressRepo := new(MockAddressRepository)
			mockPromoRepo := new(MockPromoRepository)
			mockQueueRepo := new(MockAssignmentQueueRepository)

			// Initialize Service with specific mocks
			svc := NewBookingService(
				mockRepo,
				mockPromoRepo,
				nil, // pool
				mockQueueRepo,
				nil,             // therapistRepo
				nil,             // offerRepo
				mockServiceRepo, // serviceRepo
				mockAddressRepo, // addressRepo
				nil,             // userRepo
				nil,             // messageService
				nil,             // notificationService
				nil,             // extensionRequestRepo
				nil,             // walletService
				nil,             // rideService
			)

			// Setup expectations
			if tt.setupMocks != nil {
				tt.setupMocks(mockRepo, mockServiceRepo, mockAddressRepo, mockPromoRepo, mockQueueRepo)
			}

			// Execute
			ctx := context.Background()
			booking, err := svc.Create(ctx, clientID, tt.request, nil)

			// Verify
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, booking)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, booking)
				assert.Equal(t, model.BookingStatusPending, booking.Status)
			}

			mockRepo.AssertExpectations(t)
			mockServiceRepo.AssertExpectations(t)
			mockAddressRepo.AssertExpectations(t)
			mockQueueRepo.AssertExpectations(t)
		})
	}
}

func TestBookingService_CreateRejectsNonActiveClients(t *testing.T) {
	serviceID := int64(1)
	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		ScheduledStart:  time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		DurationMinutes: 60,
		PaymentMethod:   "cash",
		PressurePref:    "medium",
		GenderPref:      "female",
	}

	for _, status := range []string{"inactive", "suspended", "blocked", "banned"} {
		t.Run(status, func(t *testing.T) {
			userRepo := new(MockUserRepository)
			userRepo.On("FindUserByID", mock.Anything, 100).Return(
				&model.User{UserID: 100, Role: model.RoleClient, AccountStatus: status},
				nil,
			).Once()

			svc := NewBookingService(
				new(MockBookingRepository),
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				userRepo,
				nil,
				nil,
				nil,
				nil,
				nil,
			)

			_, err := svc.Create(context.Background(), 100, req, nil)
			if err == nil || !strings.Contains(err.Error(), "selected client account is not active") {
				t.Fatalf("expected client_not_active error, got %v", err)
			}
			userRepo.AssertExpectations(t)
		})
	}
}

func TestBookingService_CreateAppliesVoucherForNonVIPClient(t *testing.T) {
	clientID := int64(100)
	serviceID := int64(1)
	promoID := int64(40)
	discountAmount := 100.0
	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		ScheduledStart:  time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		DurationMinutes: 60,
		PaymentMethod:   "cash",
		PressurePref:    "medium",
		GenderPref:      "female",
		VoucherCode:     "SAVE10",
	}

	userRepo := new(MockUserRepository)
	userRepo.On("FindUserByID", mock.Anything, int(clientID)).Return(
		&model.User{UserID: int(clientID), Role: model.RoleClient, AccountStatus: "active", IsVIP: false},
		nil,
	).Twice()

	serviceRepo := new(MockServiceRepository)
	serviceRepo.On("GetByID", mock.Anything, serviceID).Return(&model.Service{
		ServiceID:       serviceID,
		BasePrice:       1000,
		DurationMinutes: 60,
	}, nil).Once()

	promoRepo := new(MockPromoRepository)
	promoRepo.On("GetByCode", mock.Anything, "SAVE10").Return(&model.Promotion{
		PromoID:        promoID,
		Code:           "SAVE10",
		DiscountAmount: &discountAmount,
	}, nil).Once()
	promoRepo.On("TryIncrementGlobalUsageTx", mock.Anything, mock.Anything, promoID).Return(true, nil).Once()
	promoRepo.On("TryIncrementUserPromoUsageTx", mock.Anything, mock.Anything, promoID, clientID).Return(true, nil).Once()

	bookingRepo := new(MockBookingRepository)
	bookingRepo.On("CreateTx", mock.Anything, mock.Anything, mock.MatchedBy(func(booking *model.Booking) bool {
		if booking.RawTotal == nil || booking.Discount == nil || booking.FinalTotal == nil || booking.PromoID == nil {
			return false
		}
		return *booking.RawTotal == 1000 &&
			*booking.Discount == 100 &&
			*booking.FinalTotal == 900 &&
			*booking.PromoID == promoID
	})).Run(func(args mock.Arguments) {
		booking := args.Get(2).(*model.Booking)
		booking.BookingID = 55
	}).Return(nil).Once()
	bookingRepo.On("InsertEvent", mock.Anything, int64(55), "created", mock.Anything, mock.Anything).Return(nil).Once()

	queueRepo := new(MockAssignmentQueueRepository)
	queueRepo.On("EnqueueTx", mock.Anything, mock.Anything, int64(55)).Return(nil).Once()

	svc := NewBookingService(
		bookingRepo,
		promoRepo,
		nil,
		queueRepo,
		nil,
		nil,
		serviceRepo,
		nil,
		userRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	booking, err := svc.Create(context.Background(), clientID, req, nil)
	if err != nil {
		t.Fatalf("expected non-VIP voucher booking create to succeed, got %v", err)
	}
	if booking.Discount == nil || *booking.Discount != 100 {
		t.Fatalf("expected voucher discount of 100, got %#v", booking.Discount)
	}
	if booking.FinalTotal == nil || *booking.FinalTotal != 900 {
		t.Fatalf("expected final total of 900, got %#v", booking.FinalTotal)
	}
	bookingRepo.AssertExpectations(t)
	queueRepo.AssertExpectations(t)
	promoRepo.AssertExpectations(t)
	serviceRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestBookingService_CreateAppliesAutomaticVIPDiscount(t *testing.T) {
	clientID := int64(100)
	serviceID := int64(1)
	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		ScheduledStart:  time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		DurationMinutes: 60,
		PaymentMethod:   "cash",
		PressurePref:    "medium",
		GenderPref:      "female",
	}

	userRepo := new(MockUserRepository)
	userRepo.On("FindUserByID", mock.Anything, int(clientID)).Return(
		&model.User{UserID: int(clientID), Role: model.RoleClient, AccountStatus: "active", IsVIP: true},
		nil,
	).Once()

	serviceRepo := new(MockServiceRepository)
	serviceRepo.On("GetByID", mock.Anything, serviceID).Return(&model.Service{
		ServiceID:       serviceID,
		BasePrice:       1000,
		DurationMinutes: 60,
		Name:            "Signature Massage",
	}, nil).Once()

	bookingRepo := new(MockBookingRepository)
	bookingRepo.On("CreateTx", mock.Anything, mock.Anything, mock.MatchedBy(func(booking *model.Booking) bool {
		if booking.RawTotal == nil || booking.Discount == nil || booking.FinalTotal == nil {
			return false
		}
		return *booking.RawTotal == 1000 &&
			*booking.Discount == 100 &&
			*booking.FinalTotal == 900
	})).Run(func(args mock.Arguments) {
		booking := args.Get(2).(*model.Booking)
		booking.BookingID = 55
	}).Return(nil).Once()
	bookingRepo.On("InsertEvent", mock.Anything, int64(55), "created", mock.Anything, mock.Anything).Return(nil).Once()

	queueRepo := new(MockAssignmentQueueRepository)
	queueRepo.On("EnqueueTx", mock.Anything, mock.Anything, int64(55)).Return(nil).Once()

	svc := NewBookingService(
		bookingRepo,
		nil,
		nil,
		queueRepo,
		nil,
		nil,
		serviceRepo,
		nil,
		userRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	booking, err := svc.Create(context.Background(), clientID, req, nil)
	if err != nil {
		t.Fatalf("expected VIP booking create to succeed, got %v", err)
	}
	if booking.Discount == nil || *booking.Discount != 100 {
		t.Fatalf("expected automatic VIP discount of 100, got %#v", booking.Discount)
	}
	if booking.FinalTotal == nil || *booking.FinalTotal != 900 {
		t.Fatalf("expected final total of 900, got %#v", booking.FinalTotal)
	}
	bookingRepo.AssertExpectations(t)
	queueRepo.AssertExpectations(t)
	serviceRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestBookingService_Create_AllowsMissingAddressWhenGeofenceDepsAbsent(t *testing.T) {
	clientID := int64(100)
	serviceID := int64(1)

	mockRepo := new(MockBookingRepository)
	mockServiceRepo := new(MockServiceRepository)
	mockPromoRepo := new(MockPromoRepository)
	mockQueueRepo := new(MockAssignmentQueueRepository)

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		DurationMinutes: 60,
	}

	mockServiceRepo.On("GetByID", mock.Anything, serviceID).Return(&model.Service{
		ServiceID:       serviceID,
		BasePrice:       100.0,
		DurationMinutes: 60,
	}, nil)
	mockRepo.On("CreateTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockQueueRepo.On("EnqueueTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("InsertEvent", mock.Anything, mock.AnythingOfType("int64"), "created", mock.Anything, mock.Anything).Return(nil)

	svc := NewBookingService(
		mockRepo,
		mockPromoRepo,
		nil,
		mockQueueRepo,
		nil,
		nil,
		mockServiceRepo,
		nil, // addressRepo intentionally nil
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		// locationService intentionally omitted
	)

	booking, err := svc.Create(context.Background(), clientID, req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, booking)
	if booking != nil {
		assert.Nil(t, booking.AddressID)
	}

	mockRepo.AssertExpectations(t)
	mockServiceRepo.AssertExpectations(t)
	mockQueueRepo.AssertExpectations(t)
}

func TestBookingService_Create_RequiresAddressWhenGeofenceDepsPresent(t *testing.T) {
	clientID := int64(100)
	serviceID := int64(1)

	mockRepo := new(MockBookingRepository)
	mockServiceRepo := new(MockServiceRepository)
	mockAddressRepo := new(MockAddressRepository)
	mockPromoRepo := new(MockPromoRepository)
	mockQueueRepo := new(MockAssignmentQueueRepository)

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		DurationMinutes: 60,
	}

	mockServiceRepo.On("GetByID", mock.Anything, serviceID).Return(&model.Service{
		ServiceID:       serviceID,
		BasePrice:       100.0,
		DurationMinutes: 60,
	}, nil)

	svc := NewBookingService(
		mockRepo,
		mockPromoRepo,
		nil,
		mockQueueRepo,
		nil,
		nil,
		mockServiceRepo,
		mockAddressRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewLocationService(nil),
	)

	booking, err := svc.Create(context.Background(), clientID, req, nil)
	assert.Nil(t, booking)
	assert.Error(t, err)

	ve, ok := err.(*ValidationError)
	assert.True(t, ok)
	if ok {
		assert.Equal(t, "address_required", ve.Code)
	}

	mockServiceRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "CreateTx", mock.Anything, mock.Anything, mock.Anything)
}

func TestBookingService_Create_UsesAddressCoordinatesForServiceability(t *testing.T) {
	clientID := int64(100)
	serviceID := int64(1)
	addressID := int64(5)
	coveredLat := 14.5547
	coveredLng := 121.0244

	mockRepo := new(MockBookingRepository)
	mockServiceRepo := new(MockServiceRepository)
	mockAddressRepo := new(MockAddressRepository)
	mockPromoRepo := new(MockPromoRepository)
	mockQueueRepo := new(MockAssignmentQueueRepository)
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

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		DurationMinutes: 60,
	}

	mockServiceRepo.On("GetByID", mock.Anything, serviceID).Return(&model.Service{
		ServiceID:       serviceID,
		BasePrice:       100.0,
		DurationMinutes: 60,
	}, nil)
	mockAddressRepo.On("GetByID", mock.Anything, addressID, clientID).Return(&model.Address{
		AddressID: addressID,
		UserID:    clientID,
		City:      "Unmatched map label",
		Barangay:  "Unmatched map place",
		Latitude:  &coveredLat,
		Longitude: &coveredLng,
	}, nil)
	mockRepo.On("CreateTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockQueueRepo.On("EnqueueTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("InsertEvent", mock.Anything, mock.AnythingOfType("int64"), "created", mock.Anything, mock.Anything).Return(nil)

	svc := NewBookingService(
		mockRepo,
		mockPromoRepo,
		nil,
		mockQueueRepo,
		nil,
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

	booking, err := svc.Create(context.Background(), clientID, req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, booking)
	assert.Empty(t, areaRepo.recordedKeys)
	assert.Empty(t, areaRepo.upsertedAreas)

	mockRepo.AssertExpectations(t)
	mockServiceRepo.AssertExpectations(t)
	mockAddressRepo.AssertExpectations(t)
	mockQueueRepo.AssertExpectations(t)
}

func TestBookingService_Create_RejectsCoordinatesWhenBarangayIsBanned(t *testing.T) {
	clientID := int64(100)
	serviceID := int64(1)
	addressID := int64(5)
	coveredLat := 14.5547
	coveredLng := 121.0244
	bannedArea := &model.ServiceArea{
		AreaKey:           "barangay:banned-zone",
		Name:              "Banned Zone",
		Level:             model.ServiceAreaLevelBarangay,
		Status:            model.ServiceAreaStatusBanned,
		MinBookingMinutes: 60,
	}

	mockRepo := new(MockBookingRepository)
	mockServiceRepo := new(MockServiceRepository)
	mockAddressRepo := new(MockAddressRepository)
	mockPromoRepo := new(MockPromoRepository)
	mockQueueRepo := new(MockAssignmentQueueRepository)
	areaRepo := &bookingServiceabilityAreaRepo{
		areasByName: map[string]*model.ServiceArea{
			bookingServiceabilityNameKey(string(model.ServiceAreaLevelBarangay), "Banned Zone"): bannedArea,
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

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		DurationMinutes: 60,
	}

	mockServiceRepo.On("GetByID", mock.Anything, serviceID).Return(&model.Service{
		ServiceID:       serviceID,
		BasePrice:       100.0,
		DurationMinutes: 60,
	}, nil)
	mockAddressRepo.On("GetByID", mock.Anything, addressID, clientID).Return(&model.Address{
		AddressID: addressID,
		UserID:    clientID,
		City:      "Makati",
		Barangay:  "Banned Zone",
		Latitude:  &coveredLat,
		Longitude: &coveredLng,
	}, nil)
	mockRepo.On("CreateTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockQueueRepo.On("EnqueueTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("InsertEvent", mock.Anything, mock.AnythingOfType("int64"), "created", mock.Anything, mock.Anything).Return(nil)

	svc := NewBookingService(
		mockRepo,
		mockPromoRepo,
		nil,
		mockQueueRepo,
		nil,
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

	booking, err := svc.Create(context.Background(), clientID, req, nil)
	assert.Error(t, err)
	assert.Nil(t, booking)

	ve, ok := err.(*ValidationError)
	assert.True(t, ok)
	if ok {
		assert.Equal(t, "location_not_serviceable", ve.Code)
	}
	assert.Equal(t, []string{"barangay:banned-zone"}, areaRepo.recordedKeys)

	mockServiceRepo.AssertExpectations(t)
	mockAddressRepo.AssertExpectations(t)
}

func TestBookingService_Create_FallsBackToNameLookupWhenAddressCoordinatesAbsent(t *testing.T) {
	clientID := int64(100)
	serviceID := int64(1)
	addressID := int64(5)

	mockRepo := new(MockBookingRepository)
	mockServiceRepo := new(MockServiceRepository)
	mockAddressRepo := new(MockAddressRepository)
	mockPromoRepo := new(MockPromoRepository)
	mockQueueRepo := new(MockAssignmentQueueRepository)
	areaRepo := &bookingServiceabilityAreaRepo{
		areaByName: &model.ServiceArea{
			AreaKey:           "city:makati",
			Name:              "Makati",
			Level:             model.ServiceAreaLevelCity,
			Status:            model.ServiceAreaStatusCovered,
			MinBookingMinutes: 60,
		},
	}

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		DurationMinutes: 60,
	}

	mockServiceRepo.On("GetByID", mock.Anything, serviceID).Return(&model.Service{
		ServiceID:       serviceID,
		BasePrice:       100.0,
		DurationMinutes: 60,
	}, nil)
	mockAddressRepo.On("GetByID", mock.Anything, addressID, clientID).Return(&model.Address{
		AddressID: addressID,
		UserID:    clientID,
		City:      "Makati",
		Barangay:  "",
	}, nil)
	mockRepo.On("CreateTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockQueueRepo.On("EnqueueTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("InsertEvent", mock.Anything, mock.AnythingOfType("int64"), "created", mock.Anything, mock.Anything).Return(nil)

	svc := NewBookingService(
		mockRepo,
		mockPromoRepo,
		nil,
		mockQueueRepo,
		nil,
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

	booking, err := svc.Create(context.Background(), clientID, req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, booking)

	mockRepo.AssertExpectations(t)
	mockServiceRepo.AssertExpectations(t)
	mockAddressRepo.AssertExpectations(t)
	mockQueueRepo.AssertExpectations(t)
}

func TestBookingService_Create_RejectsAddressCoordinatesOutsideServiceArea(t *testing.T) {
	clientID := int64(100)
	serviceID := int64(1)
	addressID := int64(5)
	coveredLat := 14.5547
	coveredLng := 121.0244
	outsideLat := 16.4023
	outsideLng := 120.5960

	mockRepo := new(MockBookingRepository)
	mockServiceRepo := new(MockServiceRepository)
	mockAddressRepo := new(MockAddressRepository)
	mockPromoRepo := new(MockPromoRepository)
	mockQueueRepo := new(MockAssignmentQueueRepository)
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

	req := &model.CreateBookingRequest{
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		DurationMinutes: 60,
	}

	mockServiceRepo.On("GetByID", mock.Anything, serviceID).Return(&model.Service{
		ServiceID:       serviceID,
		BasePrice:       100.0,
		DurationMinutes: 60,
	}, nil)
	mockAddressRepo.On("GetByID", mock.Anything, addressID, clientID).Return(&model.Address{
		AddressID: addressID,
		UserID:    clientID,
		City:      "Baguio",
		Barangay:  "",
		Latitude:  &outsideLat,
		Longitude: &outsideLng,
	}, nil)

	svc := NewBookingService(
		mockRepo,
		mockPromoRepo,
		nil,
		mockQueueRepo,
		nil,
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

	booking, err := svc.Create(context.Background(), clientID, req, nil)
	assert.Error(t, err)
	assert.Nil(t, booking)

	ve, ok := err.(*ValidationError)
	assert.True(t, ok)
	if ok {
		assert.Equal(t, "location_not_serviceable", ve.Code)
	}
	assert.Equal(t, []string{"city:baguio"}, areaRepo.recordedKeys)
	if assert.Len(t, areaRepo.upsertedAreas, 1) {
		assert.Equal(t, "city:baguio", areaRepo.upsertedAreas[0].AreaKey)
		assert.Equal(t, model.ServiceAreaStatusNotSupported, areaRepo.upsertedAreas[0].Status)
	}
	mockRepo.AssertNotCalled(t, "CreateTx", mock.Anything, mock.Anything, mock.Anything)

	mockServiceRepo.AssertExpectations(t)
	mockAddressRepo.AssertExpectations(t)
}
