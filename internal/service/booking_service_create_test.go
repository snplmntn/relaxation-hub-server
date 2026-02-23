package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

// Additional Mocks needed for this test file (if not in booking_service_mocks_test.go)
// Assumes MockServiceRepository, MockAddressRepository, MockPromoRepository, MockAssignmentQueueRepository
// are defined in booking_service_mocks_test.go or need to be added there.
// I check if they exist. Based on previous turns, I updated booking_service_mocks_test.go partially.
// If they are missing, I must add them.
