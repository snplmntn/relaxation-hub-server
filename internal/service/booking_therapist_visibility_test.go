package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCanRevealTherapistDetails_ClientPrivacyGate(t *testing.T) {
	ctx := context.Background()
	clientID := int64(10)
	therapistID := int64(20)

	tests := []struct {
		name       string
		status     string
		blocked    bool
		want       bool
		checkBlock bool
	}{
		{name: "assigned remains private", status: model.BookingStatusAssigned},
		{name: "on the way remains private", status: model.BookingStatusOnTheWay},
		{name: "arrived is visible", status: model.BookingStatusArrived, want: true, checkBlock: true},
		{name: "in progress is visible", status: model.BookingStatusInProgress, want: true, checkBlock: true},
		{name: "paused is visible", status: "paused", want: true, checkBlock: true},
		{name: "completed is visible", status: model.BookingStatusCompleted, want: true, checkBlock: true},
		{name: "blocked in progress remains private", status: model.BookingStatusInProgress, blocked: true, checkBlock: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := new(MockUserRepository)
			if tt.checkBlock {
				users.On("IsBlocked", mock.Anything, clientID, therapistID).Return(tt.blocked, nil).Once()
			}
			svc := NewBookingService(nil, nil, nil, nil, nil, nil, nil, nil, users, nil, nil, nil, nil, nil)
			booking := &model.Booking{ClientID: clientID, TherapistID: &therapistID, Status: tt.status}

			got, err := svc.CanRevealTherapistDetails(ctx, booking, clientID, model.RoleClient)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
			users.AssertExpectations(t)
		})
	}
}

func TestCanRevealTherapistDetails_FailsClosedOnBlockLookupError(t *testing.T) {
	ctx := context.Background()
	clientID := int64(10)
	therapistID := int64(20)
	users := new(MockUserRepository)
	users.On("IsBlocked", mock.Anything, clientID, therapistID).Return(false, errors.New("database unavailable")).Once()
	svc := NewBookingService(nil, nil, nil, nil, nil, nil, nil, nil, users, nil, nil, nil, nil, nil)
	booking := &model.Booking{ClientID: clientID, TherapistID: &therapistID, Status: model.BookingStatusCompleted}

	visible, err := svc.CanRevealTherapistDetails(ctx, booking, clientID, model.RoleClient)

	assert.False(t, visible)
	assert.EqualError(t, err, "database unavailable")
	users.AssertExpectations(t)
}

func TestCanRevealTherapistDetails_StaffAndAssignedTherapist(t *testing.T) {
	therapistID := int64(20)
	booking := &model.Booking{ClientID: 10, TherapistID: &therapistID, Status: model.BookingStatusAssigned}
	svc := NewBookingService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	adminVisible, adminErr := svc.CanRevealTherapistDetails(context.Background(), booking, 99, model.RoleAdmin)
	therapistVisible, therapistErr := svc.CanRevealTherapistDetails(context.Background(), booking, therapistID, model.RoleTherapist)
	otherTherapistVisible, otherTherapistErr := svc.CanRevealTherapistDetails(context.Background(), booking, 21, model.RoleTherapist)

	assert.NoError(t, adminErr)
	assert.True(t, adminVisible)
	assert.NoError(t, therapistErr)
	assert.True(t, therapistVisible)
	assert.NoError(t, otherTherapistErr)
	assert.False(t, otherTherapistVisible)
}
