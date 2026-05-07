package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
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
		tid := actorID

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

func TestUpdateStatus_UnauthorizedClientLateCancellationDoesNotRunSideEffects(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(100)
	clientID := int64(10)
	wrongClientID := int64(99)
	therapistID := int64(20)

	mockRepo := new(MockBookingRepository)
	mockQueue := new(MockAssignmentQueueRepository)
	mockOffer := new(MockOfferRepository)
	mockUser := new(MockUserRepository)
	mockNotification := new(MockNotificationService)
	svc := NewBookingService(mockRepo, nil, nil, mockQueue, nil, mockOffer, nil, nil, mockUser, nil, mockNotification, nil, nil, nil)

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
		BookingID:   bookingID,
		ClientID:    clientID,
		TherapistID: &therapistID,
		Status:      model.BookingStatusOnTheWay,
		ScheduledStart: func() *time.Time {
			v := time.Now().Add(time.Hour)
			return &v
		}(),
	}, nil).Once()

	_, err := svc.UpdateStatus(ctx, bookingID, wrongClientID, model.RoleClient, &model.UpdateBookingStatusRequest{Status: model.BookingStatusCancelled})

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "late_cancellation_by_client", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "GetClientBookingStats", mock.Anything, mock.Anything, mock.Anything)
	mockQueue.AssertNotCalled(t, "Remove", mock.Anything, mock.Anything)
	mockOffer.AssertNotCalled(t, "CancelOffers", mock.Anything, mock.Anything)
	mockUser.AssertNotCalled(t, "BanUserSystem", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateStatus_LateClientCancellationPersistenceFailureDoesNotRunSideEffects(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(108)
	clientID := int64(10)
	therapistID := int64(20)

	mockRepo := new(MockBookingRepository)
	mockQueue := new(MockAssignmentQueueRepository)
	mockOffer := new(MockOfferRepository)
	mockUser := new(MockUserRepository)
	mockNotification := new(MockNotificationService)
	svc := NewBookingService(mockRepo, nil, nil, mockQueue, nil, mockOffer, nil, nil, mockUser, nil, mockNotification, nil, nil, nil)

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
		BookingID:   bookingID,
		ClientID:    clientID,
		TherapistID: &therapistID,
		Status:      model.BookingStatusOnTheWay,
	}, nil).Once()
	mockRepo.On("UpdateStatus", ctx, bookingID, clientID, model.RoleClient, model.BookingStatusCancelled, mock.MatchedBy(func(cancelledBy *string) bool {
		return cancelledBy != nil && *cancelledBy == model.RoleClient
	}), (*string)(nil)).Return(errors.New("status persistence failed")).Once()

	_, err := svc.UpdateStatus(ctx, bookingID, clientID, model.RoleClient, &model.UpdateBookingStatusRequest{Status: model.BookingStatusCancelled})

	assert.Error(t, err)
	assert.Equal(t, "status persistence failed", err.Error())
	mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "late_cancellation_by_client", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "GetClientBookingStats", mock.Anything, mock.Anything, mock.Anything)
	mockQueue.AssertNotCalled(t, "Remove", mock.Anything, mock.Anything)
	mockOffer.AssertNotCalled(t, "CancelOffers", mock.Anything, mock.Anything)
	mockUser.AssertNotCalled(t, "BanUserSystem", mock.Anything, mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
}

func TestStartSession_UnauthorizedTherapistDoesNotConfirmStartBeforeScopedPersistence(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(101)
	assignedTherapistID := int64(20)
	wrongTherapistID := int64(21)
	now := time.Now()

	mockRepo := new(MockBookingRepository)
	svc := NewBookingService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
		BookingID:          bookingID,
		ClientID:           10,
		TherapistID:        &assignedTherapistID,
		Status:             model.BookingStatusArrived,
		TherapistArrivedAt: &now,
	}, nil).Once()

	_, err := svc.StartSession(ctx, bookingID, wrongTherapistID, model.RoleTherapist, nil)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "therapist_confirm_start", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "UpdateStatusWithTime", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestStartSession_PersistenceFailureDoesNotInsertConfirmStartEvent(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(102)
	therapistID := int64(20)
	now := time.Now()

	mockRepo := new(MockBookingRepository)
	svc := NewBookingService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
		BookingID:          bookingID,
		ClientID:           10,
		TherapistID:        &therapistID,
		Status:             model.BookingStatusArrived,
		TherapistArrivedAt: &now,
	}, nil).Once()
	mockRepo.On("UpdateStatusWithTime", ctx, bookingID, therapistID, model.RoleTherapist, model.BookingStatusInProgress, (*string)(nil), (*string)(nil), mock.AnythingOfType("*time.Time")).Return(pgx.ErrNoRows).Once()

	_, err := svc.StartSession(ctx, bookingID, therapistID, model.RoleTherapist, nil)

	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "therapist_confirm_start", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
}

func TestPauseResume_UnauthorizedTherapistDoesNotMutateOrEmitEvents(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(103)
	assignedTherapistID := int64(20)
	wrongTherapistID := int64(21)
	pauseStart := time.Now().Add(-5 * time.Minute).UTC()

	t.Run("pause", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		svc := NewBookingService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
			BookingID:   bookingID,
			ClientID:    10,
			TherapistID: &assignedTherapistID,
			Status:      model.BookingStatusInProgress,
		}, nil).Once()

		_, err := svc.PauseSession(ctx, bookingID, wrongTherapistID, model.RoleTherapist)

		assert.Error(t, err)
		mockRepo.AssertNotCalled(t, "SetPauseStart", mock.Anything, mock.Anything, mock.Anything)
		mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "session_paused", mock.Anything, mock.Anything)
	})

	t.Run("resume", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		svc := NewBookingService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
			BookingID:          bookingID,
			ClientID:           10,
			TherapistID:        &assignedTherapistID,
			Status:             model.BookingStatusInProgress,
			CurrentPauseStart:  &pauseStart,
			TotalPausedSeconds: 30,
		}, nil).Once()

		_, err := svc.ResumeSession(ctx, bookingID, wrongTherapistID, model.RoleTherapist)

		assert.Error(t, err)
		mockRepo.AssertNotCalled(t, "ClearPauseAndAddDuration", mock.Anything, mock.Anything, mock.Anything)
		mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "session_resumed", mock.Anything, mock.Anything)
	})
}

func TestPauseResume_PersistenceFailureDoesNotEmitEvents(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(104)
	therapistID := int64(20)
	pauseStart := time.Now().Add(-5 * time.Minute).UTC()

	t.Run("pause", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		svc := NewBookingService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
			BookingID:   bookingID,
			ClientID:    10,
			TherapistID: &therapistID,
			Status:      model.BookingStatusInProgress,
		}, nil).Once()
		mockRepo.On("SetPauseStart", ctx, bookingID, mock.AnythingOfType("*time.Time")).Return(pgx.ErrNoRows).Once()

		_, err := svc.PauseSession(ctx, bookingID, therapistID, model.RoleTherapist)

		assert.Error(t, err)
		mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "session_paused", mock.Anything, mock.Anything)
		mockRepo.AssertExpectations(t)
	})

	t.Run("resume", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		svc := NewBookingService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
			BookingID:          bookingID,
			ClientID:           10,
			TherapistID:        &therapistID,
			Status:             model.BookingStatusInProgress,
			CurrentPauseStart:  &pauseStart,
			TotalPausedSeconds: 30,
		}, nil).Once()
		mockRepo.On("ClearPauseAndAddDuration", ctx, bookingID, mock.AnythingOfType("int")).Return(pgx.ErrNoRows).Once()

		_, err := svc.ResumeSession(ctx, bookingID, therapistID, model.RoleTherapist)

		assert.Error(t, err)
		mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "session_resumed", mock.Anything, mock.Anything)
		mockRepo.AssertExpectations(t)
	})
}

func TestAdminPatch_AssignedBookingRevalidatesTherapistAssignmentBeforePersistence(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(105)
	adminID := int64(1)
	clientID := int64(10)
	therapistID := int64(20)
	serviceID := int64(30)
	scheduled := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	newSchedule := scheduled.Add(time.Hour).Format(time.RFC3339)

	mockRepo := new(MockBookingRepository)
	mockTherapist := new(MockTherapistRepository)
	svc := NewBookingService(mockRepo, nil, nil, nil, mockTherapist, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
		BookingID:       bookingID,
		ClientID:        clientID,
		TherapistID:     &therapistID,
		ServiceID:       &serviceID,
		PressurePref:    "medium",
		DurationMinutes: 60,
		ScheduledStart:  &scheduled,
		Status:          model.BookingStatusAssigned,
	}, nil).Once()
	mockTherapist.On("GetProfile", ctx, therapistID).Return(&model.TherapistProfile{TherapistID: therapistID, AcceptAssignments: false}, nil).Once()

	_, err := svc.UpdateByAdminWithMeta(ctx, adminID, bookingID, &model.UpdateBookingRequest{ScheduledStart: &newSchedule})

	assert.Error(t, err)
	if assert.IsType(t, &ValidationError{}, err) {
		assert.Equal(t, "therapist_not_accepting", err.(*ValidationError).Code)
	}
	mockRepo.AssertNotCalled(t, "UpdateAdmin", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "admin_reassigned_therapist", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
	mockTherapist.AssertExpectations(t)
}

func TestAdminPatch_AssignedBookingRejectsInactiveTherapistBeforePersistence(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(115)
	adminID := int64(1)
	clientID := int64(10)
	therapistID := int64(20)
	serviceID := int64(30)
	scheduled := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	newSchedule := scheduled.Add(time.Hour).Format(time.RFC3339)

	mockRepo := new(MockBookingRepository)
	mockTherapist := new(MockTherapistRepository)
	svc := NewBookingService(mockRepo, nil, nil, nil, mockTherapist, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
		BookingID:       bookingID,
		ClientID:        clientID,
		TherapistID:     &therapistID,
		ServiceID:       &serviceID,
		PressurePref:    "medium",
		DurationMinutes: 60,
		ScheduledStart:  &scheduled,
		Status:          model.BookingStatusAssigned,
	}, nil).Once()
	mockTherapist.On("GetProfile", ctx, therapistID).Return(&model.TherapistProfile{TherapistID: therapistID, Status: "inactive", AcceptAssignments: true}, nil).Once()
	mockTherapist.On("GetServicesWithPressures", ctx, therapistID).Return(map[int64][]string{serviceID: {"medium"}}, nil).Maybe()
	mockRepo.On("ListByTherapist", ctx, therapistID).Return([]model.Booking{}, nil).Maybe()
	mockRepo.On("UpdateAdmin", ctx, mock.AnythingOfType("*model.Booking")).Return(nil).Maybe()
	mockRepo.On("GetByBookingID", mock.Anything, bookingID).Return(&model.Booking{BookingID: bookingID}, nil).Maybe()

	_, err := svc.UpdateByAdminWithMeta(ctx, adminID, bookingID, &model.UpdateBookingRequest{ScheduledStart: &newSchedule})

	assert.Error(t, err)
	if assert.IsType(t, &ValidationError{}, err) {
		assert.Equal(t, "therapist_not_accepting", err.(*ValidationError).Code)
	}
	mockRepo.AssertNotCalled(t, "UpdateAdmin", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
	mockTherapist.AssertExpectations(t)
}

func TestAdminPatch_AssignedBookingRejectsOverlappingTherapistBeforePersistence(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(106)
	adminID := int64(1)
	clientID := int64(10)
	therapistID := int64(20)
	serviceID := int64(30)
	scheduled := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	overlapID := int64(999)

	mockRepo := new(MockBookingRepository)
	mockTherapist := new(MockTherapistRepository)
	svc := NewBookingService(mockRepo, nil, nil, nil, mockTherapist, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
		BookingID:       bookingID,
		ClientID:        clientID,
		TherapistID:     &therapistID,
		ServiceID:       &serviceID,
		PressurePref:    "medium",
		DurationMinutes: 60,
		ScheduledStart:  &scheduled,
		Status:          model.BookingStatusAssigned,
	}, nil).Once()
	mockTherapist.On("GetProfile", ctx, therapistID).Return(&model.TherapistProfile{TherapistID: therapistID, Status: "active", AcceptAssignments: true}, nil).Once()
	mockTherapist.On("GetServicesWithPressures", ctx, therapistID).Return(map[int64][]string{serviceID: {"medium"}}, nil).Once()
	mockRepo.On("ListByTherapist", ctx, therapistID).Return([]model.Booking{{
		BookingID:       overlapID,
		TherapistID:     &therapistID,
		Status:          model.BookingStatusAssigned,
		ScheduledStart:  func() *time.Time { v := scheduled.Add(30 * time.Minute); return &v }(),
		DurationMinutes: 60,
	}}, nil).Once()

	newDuration := 90
	_, err := svc.UpdateByAdminWithMeta(ctx, adminID, bookingID, &model.UpdateBookingRequest{DurationMinutes: &newDuration})

	assert.Error(t, err)
	if assert.IsType(t, &ValidationError{}, err) {
		assert.Equal(t, "therapist_schedule_conflict", err.(*ValidationError).Code)
	}
	mockRepo.AssertNotCalled(t, "UpdateAdmin", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
	mockTherapist.AssertExpectations(t)
}

func TestClientPatch_AddressChangeRunsServiceabilityBeforePersistence(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(116)
	clientID := int64(10)
	serviceID := int64(30)
	oldAddressID := int64(40)
	newAddressID := int64(41)

	mockRepo := new(MockBookingRepository)
	mockAddress := new(MockAddressRepository)
	areaRepo := &bookingServiceabilityAreaRepo{areasByName: map[string]*model.ServiceArea{
		bookingServiceabilityNameKey(string(model.ServiceAreaLevelCity), "Banned City"): {
			AreaKey: "city:banned-city",
			Name:    "Banned City",
			Level:   model.ServiceAreaLevelCity,
			Status:  model.ServiceAreaStatusBanned,
		},
	}}
	svc := NewBookingService(mockRepo, nil, nil, nil, nil, nil, nil, mockAddress, nil, nil, nil, nil, nil, nil, NewLocationService(areaRepo))

	initial := &model.Booking{
		BookingID:       bookingID,
		ClientID:        clientID,
		ServiceID:       &serviceID,
		AddressID:       &oldAddressID,
		DurationMinutes: 60,
		Status:          model.BookingStatusPending,
	}
	mockRepo.On("GetByID", ctx, bookingID, clientID).Return(initial, nil).Once()
	mockRepo.On("Update", ctx, mock.AnythingOfType("*model.Booking")).Return(nil).Maybe()
	mockRepo.On("GetByID", mock.Anything, bookingID, clientID).Return(initial, nil).Maybe()
	mockRepo.On("GetByBookingID", mock.Anything, bookingID).Return(initial, nil).Maybe()
	mockAddress.On("GetByID", ctx, newAddressID, clientID).Return(&model.Address{
		AddressID: newAddressID,
		UserID:    clientID,
		City:      "Banned City",
		Barangay:  "Allowed Barangay",
	}, nil).Once()
	mockAddress.On("GetByIDUnsafe", mock.Anything, newAddressID).Return(&model.Address{AddressID: newAddressID}, nil).Maybe()

	_, err := svc.UpdateWithMeta(ctx, bookingID, clientID, &model.UpdateBookingRequest{AddressID: &newAddressID})

	assert.Error(t, err)
	if assert.IsType(t, &ValidationError{}, err) {
		assert.Equal(t, "location_not_serviceable", err.(*ValidationError).Code)
	}
	mockRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
	mockAddress.AssertExpectations(t)
}

func TestAdminPatch_TherapistAssignmentCleansQueueAndOffersAfterPersistence(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(109)
	adminID := int64(1)
	clientID := int64(10)
	therapistID := int64(20)
	otherTherapistID := int64(21)
	serviceID := int64(30)

	mockRepo := new(MockBookingRepository)
	mockTherapist := new(MockTherapistRepository)
	mockQueue := new(MockAssignmentQueueRepository)
	mockOffer := new(MockOfferRepository)
	svc := NewBookingService(mockRepo, nil, nil, mockQueue, mockTherapist, mockOffer, nil, nil, nil, nil, nil, nil, nil, nil)

	initial := &model.Booking{
		BookingID:       bookingID,
		ClientID:        clientID,
		ServiceID:       &serviceID,
		PressurePref:    "medium",
		DurationMinutes: 60,
		Status:          model.BookingStatusPending,
	}
	updated := *initial
	updated.TherapistID = &therapistID
	updated.Status = model.BookingStatusAssigned

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(initial, nil).Once()
	mockTherapist.On("GetProfile", ctx, therapistID).Return(&model.TherapistProfile{TherapistID: therapistID, Status: "active", AcceptAssignments: true}, nil).Once()
	mockTherapist.On("GetProfile", mock.Anything, therapistID).Return(&model.TherapistProfile{TherapistID: therapistID, Status: "active", AcceptAssignments: true}, nil).Maybe()
	mockTherapist.On("GetServicesWithPressures", ctx, therapistID).Return(map[int64][]string{serviceID: {"medium"}}, nil).Once()
	mockRepo.On("UpdateAdmin", ctx, mock.MatchedBy(func(booking *model.Booking) bool {
		return booking.BookingID == bookingID && booking.TherapistID != nil && *booking.TherapistID == therapistID && booking.Status == model.BookingStatusAssigned
	})).Return(nil).Once()
	mockRepo.On("InsertEvent", ctx, bookingID, "admin_reassigned_therapist", &adminID, mock.Anything).Return(nil).Once()
	mockQueue.On("Remove", ctx, bookingID).Return(nil).Once()
	mockOffer.On("CancelOffers", ctx, bookingID).Return([]model.BookingOffer{
		{OfferID: 1, BookingID: bookingID, TherapistID: therapistID},
		{OfferID: 2, BookingID: bookingID, TherapistID: otherTherapistID},
	}, nil).Once()
	mockRepo.On("GetByBookingID", mock.Anything, bookingID).Return(&updated, nil).Maybe()

	result, err := svc.UpdateByAdminWithMeta(ctx, adminID, bookingID, &model.UpdateBookingRequest{TherapistID: &therapistID})

	assert.NoError(t, err)
	if assert.NotNil(t, result) && assert.NotNil(t, result.Booking.TherapistID) {
		assert.Equal(t, therapistID, *result.Booking.TherapistID)
	}
	mockRepo.AssertExpectations(t)
	mockTherapist.AssertExpectations(t)
	mockQueue.AssertExpectations(t)
	mockOffer.AssertExpectations(t)
}

func TestAdminPatch_TherapistAssignmentPersistenceFailureDoesNotCleanQueueOrOffers(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(110)
	adminID := int64(1)
	clientID := int64(10)
	therapistID := int64(20)
	serviceID := int64(30)

	mockRepo := new(MockBookingRepository)
	mockTherapist := new(MockTherapistRepository)
	mockQueue := new(MockAssignmentQueueRepository)
	mockOffer := new(MockOfferRepository)
	svc := NewBookingService(mockRepo, nil, nil, mockQueue, mockTherapist, mockOffer, nil, nil, nil, nil, nil, nil, nil, nil)

	initial := &model.Booking{
		BookingID:       bookingID,
		ClientID:        clientID,
		ServiceID:       &serviceID,
		PressurePref:    "medium",
		DurationMinutes: 60,
		Status:          model.BookingStatusPending,
	}

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(initial, nil).Once()
	mockTherapist.On("GetProfile", ctx, therapistID).Return(&model.TherapistProfile{TherapistID: therapistID, Status: "active", AcceptAssignments: true}, nil).Once()
	mockTherapist.On("GetServicesWithPressures", ctx, therapistID).Return(map[int64][]string{serviceID: {"medium"}}, nil).Once()
	mockRepo.On("UpdateAdmin", ctx, mock.AnythingOfType("*model.Booking")).Return(errors.New("admin update failed")).Once()

	_, err := svc.UpdateByAdminWithMeta(ctx, adminID, bookingID, &model.UpdateBookingRequest{TherapistID: &therapistID})

	assert.Error(t, err)
	assert.Equal(t, "admin update failed", err.Error())
	mockQueue.AssertNotCalled(t, "Remove", mock.Anything, mock.Anything)
	mockOffer.AssertNotCalled(t, "CancelOffers", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "admin_reassigned_therapist", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
	mockTherapist.AssertExpectations(t)
}

func TestAdminPatch_TherapistAssignmentMapsRepositoryDeletedTherapistGuard(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(111)
	adminID := int64(1)
	clientID := int64(10)
	therapistID := int64(20)
	serviceID := int64(30)

	mockRepo := new(MockBookingRepository)
	mockTherapist := new(MockTherapistRepository)
	mockQueue := new(MockAssignmentQueueRepository)
	mockOffer := new(MockOfferRepository)
	svc := NewBookingService(mockRepo, nil, nil, mockQueue, mockTherapist, mockOffer, nil, nil, nil, nil, nil, nil, nil, nil)

	initial := &model.Booking{
		BookingID:       bookingID,
		ClientID:        clientID,
		ServiceID:       &serviceID,
		PressurePref:    "medium",
		DurationMinutes: 60,
		Status:          model.BookingStatusPending,
	}
	mockRepo.On("GetByBookingID", ctx, bookingID).Return(initial, nil).Once()
	mockTherapist.On("GetProfile", ctx, therapistID).Return(&model.TherapistProfile{TherapistID: therapistID, Status: "active", AcceptAssignments: true}, nil).Once()
	mockTherapist.On("GetServicesWithPressures", ctx, therapistID).Return(map[int64][]string{serviceID: {"medium"}}, nil).Once()
	mockRepo.On("UpdateAdmin", ctx, mock.AnythingOfType("*model.Booking")).Return(repository.ErrTherapistNotFound).Once()

	_, err := svc.UpdateByAdminWithMeta(ctx, adminID, bookingID, &model.UpdateBookingRequest{TherapistID: &therapistID})

	if assert.IsType(t, &ValidationError{}, err) {
		assert.Equal(t, "invalid_therapist", err.(*ValidationError).Code)
	}
	mockQueue.AssertNotCalled(t, "Remove", mock.Anything, mock.Anything)
	mockOffer.AssertNotCalled(t, "CancelOffers", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "InsertEvent", mock.Anything, mock.Anything, "admin_reassigned_therapist", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
	mockTherapist.AssertExpectations(t)
}

func TestAdminPatch_AddressChangeRunsServiceabilityBeforePersistence(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(107)
	adminID := int64(1)
	clientID := int64(10)
	serviceID := int64(30)
	oldAddressID := int64(40)
	newAddressID := int64(41)

	mockRepo := new(MockBookingRepository)
	mockAddress := new(MockAddressRepository)
	areaRepo := &bookingServiceabilityAreaRepo{areasByName: map[string]*model.ServiceArea{
		bookingServiceabilityNameKey(string(model.ServiceAreaLevelCity), "Banned City"): {
			AreaKey: "city:banned-city",
			Name:    "Banned City",
			Level:   model.ServiceAreaLevelCity,
			Status:  model.ServiceAreaStatusBanned,
		},
	}}
	svc := NewBookingService(mockRepo, nil, nil, nil, nil, nil, nil, mockAddress, nil, nil, nil, nil, nil, nil, NewLocationService(areaRepo))

	mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
		BookingID:       bookingID,
		ClientID:        clientID,
		ServiceID:       &serviceID,
		AddressID:       &oldAddressID,
		DurationMinutes: 60,
		Status:          model.BookingStatusPending,
	}, nil).Once()
	mockAddress.On("GetByID", ctx, newAddressID, clientID).Return(&model.Address{
		AddressID: newAddressID,
		UserID:    clientID,
		City:      "Banned City",
		Barangay:  "Allowed Barangay",
	}, nil).Once()

	_, err := svc.UpdateByAdminWithMeta(ctx, adminID, bookingID, &model.UpdateBookingRequest{AddressID: &newAddressID})

	assert.Error(t, err)
	if assert.IsType(t, &ValidationError{}, err) {
		assert.Equal(t, "location_not_serviceable", err.(*ValidationError).Code)
	}
	mockRepo.AssertNotCalled(t, "UpdateAdmin", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
	mockAddress.AssertExpectations(t)
}

// Mocks removed - now using common_test.go
