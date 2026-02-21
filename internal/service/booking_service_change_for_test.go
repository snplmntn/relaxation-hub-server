package service

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBookingService_Create_WithChangeFor(t *testing.T) {
	mockRepo := new(MockBookingRepository)
	mockServiceRepo := new(MockServiceRepository)
	mockAddressRepo := new(MockAddressRepository)
	mockPromoRepo := new(MockPromoRepository)
	mockQueueRepo := new(MockAssignmentQueueRepository)
	mockPool := new(MockDBTX)
	mockTx := new(MockTx)

	svc := NewBookingService(
		mockRepo,
		mockPromoRepo,
		mockPool,
		mockQueueRepo,
		nil, nil,
		mockServiceRepo,
		mockAddressRepo,
		nil, nil, nil, nil, nil, nil,
	)

	ctx := context.Background()
	clientID := int64(100)
	serviceID := int64(1)
	addressID := int64(5)
	changeFor := 1000.0

	req := &model.CreateBookingRequest{
		ServiceID:      &serviceID,
		AddressID:      &addressID,
		ScheduledStart: time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		PaymentMethod:  "cash",
		ChangeFor:      &changeFor,
	}

	validService := &model.Service{
		ServiceID:       serviceID,
		BasePrice:       500.0,
		DurationMinutes: 60,
	}
	validAddress := &model.Address{
		AddressID: addressID,
		UserID:    clientID,
	}

	mockPool.On("Begin", ctx).Return(mockTx, nil)
	mockTx.On("Rollback", ctx).Return(nil)
	mockTx.On("Commit", ctx).Return(nil)

	mockServiceRepo.On("GetByID", ctx, serviceID).Return(validService, nil)
	mockAddressRepo.On("GetByID", ctx, addressID).Return(validAddress, nil)

	// Create Booking should have ChangeFor mapped
	mockRepo.On("CreateTx", ctx, mockTx, mock.MatchedBy(func(b *model.Booking) bool {
		return b.ChangeFor != nil && *b.ChangeFor == changeFor && b.PaymentMethod == "cash"
	})).Return(nil)

	mockQueueRepo.On("EnqueueTx", ctx, mockTx, mock.AnythingOfType("int64")).Return(nil)
	mockRepo.On("InsertEvent", ctx, mock.Anything, "created", mock.Anything, mock.Anything).Return(nil)

	booking, err := svc.Create(ctx, clientID, req, nil)

	assert.NoError(t, err)
	assert.NotNil(t, booking)
	if booking != nil {
		assert.Equal(t, &changeFor, booking.ChangeFor)
	}

	mockRepo.AssertExpectations(t)
}
