package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUpdateStatus_RolePermissions(t *testing.T) {
	t.Run("Therapist sets on_the_way", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		// Use nil for other services as they are not critical for this permission check or handled gracefully
		svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		ctx := context.Background()
		bookingID, actorID := int64(10), int64(42)
		tid := int64(2)

		// Expect UpdateStatus to be called
		mockRepo.On("UpdateStatus", ctx, bookingID, actorID, model.RoleTherapist, model.BookingStatusOnTheWay, (*string)(nil), (*string)(nil)).Return(nil)

		// Expect current booking fetch + post-update fetch
		mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
			BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: model.BookingStatusAssigned,
		}, nil)

		booking, err := svc.UpdateStatus(ctx, bookingID, actorID, model.RoleTherapist, &model.UpdateBookingStatusRequest{Status: model.BookingStatusOnTheWay})

		assert.NoError(t, err)
		assert.NotNil(t, booking)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Client tries to set on_the_way (Forbidden)", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		ctx := context.Background()

		// Should NOT call UpdateStatus
		_, err := svc.UpdateStatus(ctx, 11, 100, model.RoleClient, &model.UpdateBookingStatusRequest{Status: model.BookingStatusOnTheWay})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client not allowed")
		mockRepo.AssertNotCalled(t, "UpdateStatus")
	})

	t.Run("Admin sets cancelled", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		ctx := context.Background()
		bookingID, actorID := int64(12), int64(7)
		tid := int64(2)
		role := model.RoleAdmin

		// Expect UpdateStatus with cancellation args
		mockRepo.On("UpdateStatus", ctx, bookingID, actorID, role, model.BookingStatusCancelled, mock.MatchedBy(func(s *string) bool {
			return s != nil && *s == role
		}), (*string)(nil)).Return(nil)

		// Expect current booking fetch + post-update fetch
		mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
			BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: model.BookingStatusAssigned,
		}, nil)

		_, err := svc.UpdateStatus(ctx, bookingID, actorID, role, &model.UpdateBookingStatusRequest{Status: model.BookingStatusCancelled})

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Unknown role rejected", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		ctx := context.Background()

		_, err := svc.UpdateStatus(ctx, 13, 99, "unknown", &model.UpdateBookingStatusRequest{Status: model.BookingStatusPending})

		assert.Error(t, err)
	})

	t.Run("Repo error propagated", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		ctx := context.Background()
		bookingID := int64(14)
		role := model.RoleAdmin

		// Expect UpdateStatus call that fails
		// Using mock.Anything for args to match any valid call structure for this test
		mockRepo.On("UpdateStatus", ctx, bookingID, int64(1), role, model.BookingStatusCompleted, mock.Anything, mock.Anything).
			Return(errors.New("db error"))

		// Note: UpdateStatus logic checks "completed" status commission first.
		// If status is "completed", it calls CompleteBooking, not UpdateStatus.
		// Let's use "cancelled" instead to ensure it hits UpdateStatus, or mock CompleteBooking if we stick to "completed".
		// The original test used "completed" but the manual mock caught "UpdateStatus" or "CompleteBooking".
		// In BookingService.go: if status == "completed" -> CompleteBooking.
		// So we should change status to "arrived" or "cancelled" to hit UpdateStatus, OR mock CompleteBooking.
		// Since I'm testing "UpdateStatus error propagation", I'll use "on_the_way" (admin can do it) to be safe.

		// Wait, admin can set any status?
		// "on_the_way" is allowed for admin.

		mockRepo.ExpectedCalls = nil // Clear previous
		mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
			BookingID: bookingID, ClientID: 1, Status: model.BookingStatusAssigned,
		}, nil)
		mockRepo.On("UpdateStatus", ctx, bookingID, int64(1), role, model.BookingStatusOnTheWay, mock.Anything, mock.Anything).
			Return(errors.New("db error"))

		_, err := svc.UpdateStatus(ctx, bookingID, 1, role, &model.UpdateBookingStatusRequest{Status: model.BookingStatusOnTheWay})

		assert.Error(t, err)
		assert.Equal(t, "db error", err.Error())
	})
}

// Mocks removed - now using common_test.go
