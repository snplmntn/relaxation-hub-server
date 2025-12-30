package service

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestUpdateStatus_TherapistLifecycle(t *testing.T) {
	ctx := context.Background()
	mock := &mockBookingRepo{}
	// service.NewBookingService requires dependencies; pass nil for those not used in UpdateStatus logic
	svc := NewBookingService(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	therapistID := int64(42)
	bookingID := int64(100)

	// List of statuses a therapist should be able to transition to
	allowedStatuses := []string{
		"on_the_way",
		"arrived",
		"in_progress",
		"completed",
	}

	for _, status := range allowedStatuses {
		t.Run("Therapist sets "+status, func(t *testing.T) {
			mock.lastUpdateCalled = false
			mock.lastStatus = "" // reset

			req := &model.UpdateBookingStatusRequest{Status: status}
			_, err := svc.UpdateStatus(ctx, bookingID, therapistID, "therapist", req)

			if err != nil {
				t.Errorf("Therapist failed to set status '%s': %v", status, err)
			}
			if !mock.lastUpdateCalled {
				t.Errorf("Repo update not called for status '%s'", status)
			}
			if mock.lastStatus != status {
				t.Errorf("Expected status '%s', got '%s'", status, mock.lastStatus)
			}
		})
	}
}
