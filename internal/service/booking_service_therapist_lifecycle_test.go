package service

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/mock"
)

func TestUpdateStatus_TherapistLifecycle(t *testing.T) {
	ctx := context.Background()
<<<<<<< HEAD
=======
	mock := &mockBookingRepo{}
	// service.NewBookingService requires dependencies; pass nil for those not used in UpdateStatus logic
	svc := NewBookingService(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
	therapistID := int64(42)
	bookingID := int64(100)
	tid := int64(2)

	// List of statuses a therapist should be able to transition to
	allowedStatuses := []string{
		"on_the_way",
		"arrived",
		"in_progress",
		"completed",
	}

	for _, status := range allowedStatuses {
		t.Run("Therapist sets "+status, func(t *testing.T) {
			mockRepo := new(MockBookingRepository)
			// Pass nil for other repos/services
			svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

			// Setup expectations
			// 1. Broadcast always calls GetByBookingID to fetch updated state
			// For non-completed statuses, expecting strict status return
			if status != "completed" {
				mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
					BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: status,
				}, nil)
			}

			// 2. Log event is only called for completed status
			if status == "completed" {
				// 1a. Commission check calls GetByBookingID (status=in_progress)
				mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
					BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: "in_progress",
				}, nil).Once()

				// 1b. Broadcast calls GetByBookingID (status=completed)
				mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
					BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: "completed",
				}, nil).Once()

				mockRepo.On("CompleteBooking", ctx, bookingID, (*float64)(nil), (*float64)(nil), mock.AnythingOfType("time.Time")).Return(nil)
				
				// Expect InsertEvent for completion
				mockRepo.On("InsertEvent", ctx, bookingID, "completed", mock.Anything, mock.Anything).Return(nil)

				// Expect GetByID called at the end (using actorID)
				mockRepo.On("GetByID", ctx, bookingID, therapistID).Return(&model.Booking{
					BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: status,
				}, nil)
			} else {
				// Standard status update
				mockRepo.On("UpdateStatus", ctx, bookingID, therapistID, "therapist", status, (*string)(nil), (*string)(nil)).Return(nil)
				
				// Expect GetByID called at the end
				mockRepo.On("GetByID", ctx, bookingID, therapistID).Return(&model.Booking{
					BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: status,
				}, nil)
			}

			req := &model.UpdateBookingStatusRequest{Status: status}
			_, err := svc.UpdateStatus(ctx, bookingID, therapistID, "therapist", req)

			if err != nil {
				t.Errorf("Therapist failed to set status '%s': %v", status, err)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}
