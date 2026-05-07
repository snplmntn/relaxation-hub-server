package service

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUpdateStatus_TherapistLifecycle(t *testing.T) {
	ctx := context.Background()
	therapistID := int64(42)
	bookingID := int64(100)
	tid := therapistID

	transitionCases := []struct {
		targetStatus  string
		currentStatus string
	}{
		{targetStatus: model.BookingStatusOnTheWay, currentStatus: model.BookingStatusAssigned},
		{targetStatus: model.BookingStatusArrived, currentStatus: model.BookingStatusOnTheWay},
		{targetStatus: model.BookingStatusInProgress, currentStatus: model.BookingStatusArrived},
		{targetStatus: model.BookingStatusCompleted, currentStatus: model.BookingStatusInProgress},
	}

	for _, tc := range transitionCases {
		t.Run("Therapist sets "+tc.targetStatus, func(t *testing.T) {
			mockRepo := new(MockBookingRepository)
			svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

			// Initial state lookup for transition validation.
			mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
				BookingID: bookingID,
				ClientID:  1,
				TherapistID: &tid,
				Status:    tc.currentStatus,
			}, nil).Once()

			if tc.targetStatus == model.BookingStatusCompleted {
				// Completion flow re-fetches booking before CompleteBooking.
				mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
					BookingID:   bookingID,
					ClientID:    1,
					TherapistID: &tid,
					Status:      model.BookingStatusInProgress,
				}, nil).Once()

				mockRepo.On("CompleteBooking", ctx, bookingID, (*float64)(nil), (*float64)(nil), mock.AnythingOfType("time.Time")).Return(nil).Once()
				mockRepo.On("InsertEvent", ctx, bookingID, model.BookingStatusCompleted, mock.Anything, mock.Anything).Return(nil).Once()

				// Broadcast + return value fetch.
				mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
					BookingID:   bookingID,
					ClientID:    1,
					TherapistID: &tid,
					Status:      model.BookingStatusCompleted,
				}, nil).Twice()
			} else {
				mockRepo.On("UpdateStatus", ctx, bookingID, therapistID, model.RoleTherapist, tc.targetStatus, (*string)(nil), (*string)(nil)).Return(nil).Once()

				// Broadcast + return value fetch.
				mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
					BookingID:   bookingID,
					ClientID:    1,
					TherapistID: &tid,
					Status:      tc.targetStatus,
				}, nil).Twice()
			}

			req := &model.UpdateBookingStatusRequest{Status: tc.targetStatus}
			booking, err := svc.UpdateStatus(ctx, bookingID, therapistID, model.RoleTherapist, req)

			assert.NoError(t, err)
			assert.NotNil(t, booking)
			assert.Equal(t, tc.targetStatus, booking.Status)
			mockRepo.AssertExpectations(t)
		})
	}
}
