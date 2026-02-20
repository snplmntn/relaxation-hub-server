package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

<<<<<<< HEAD
=======
// mockBookingRepo implements minimal BookingRepository for testing UpdateStatus logic.



type mockBookingRepo struct {
	// record calls
	lastUpdateCalled bool
	lastBookingID    int64
	lastActorID      int64
	lastStatus       string

	// control errors
	updateErr error
}

func (m *mockBookingRepo) Create(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepo) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepo) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	// return a booking reflecting the requested status for assertion
	tid := int64(2)
	return &model.Booking{BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}, nil
}
func (m *mockBookingRepo) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepo) Update(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepo) UpdateStatus(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string) error {
	m.lastUpdateCalled = true
	m.lastBookingID = bookingID
	m.lastActorID = actorID
	m.lastStatus = status
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

func (m *mockBookingRepo) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return m.UpdateStatus(ctx, bookingID, actorID, status, cancelledBy, cancellationReason)
}

func (m *mockBookingRepo) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }

func (m *mockBookingRepo) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepo) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }

func (m *mockBookingRepo) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	tid := int64(2)
	return &model.Booking{BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}, nil
}

func (m *mockBookingRepo) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return []model.BookingEvent{}, nil
}

func (m *mockBookingRepo) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	return nil
}

// Implement GetRecentTherapistStruggleFlags to satisfy the BookingRepository interface in tests
func (m *mockBookingRepo) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

func (m *mockBookingRepo) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepo) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	tid := int64(2)
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}}, nil
}
func (m *mockBookingRepo) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	tid := int64(2)
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}}, nil
}
func (m *mockBookingRepo) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	tid := int64(2)
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1, ReferenceCode: &referenceCode, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}}, nil
}
func (m *mockBookingRepo) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	tid := int64(2)
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1, ReferenceCode: &referenceCode, ClientID: 1, TherapistID: &tid, Status: m.lastStatus}}, nil
}
func (m *mockBookingRepo) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepo) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepo) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepo) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return map[int64]int{}, nil
}
func (m *mockBookingRepo) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error { return nil }
func (m *mockBookingRepo) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error { return nil }
func (m *mockBookingRepo) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepo) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockBookingRepo) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return []repository.BookingDetailsResult{}, 0, nil
}
func (m *mockBookingRepo) ListUpcomingBookingsForReminder(ctx context.Context, windowStart, windowEnd time.Time, eventTypeExclude string) ([]model.Booking, error) {
    return nil, nil
}
func (m *mockBookingRepo) UnassignTherapist(ctx context.Context, bookingID int64) error {
    return nil
}
func (m *mockBookingRepo) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
    return nil
}
func (m *mockBookingRepo) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
    return 0, nil
}
func (m *mockBookingRepo) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
    return &repository.ClientBookingStats{}, nil
}
func (m *mockBookingRepo) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) { return &repository.AccountingSummary{}, nil }
func (m *mockBookingRepo) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) { return nil, nil }
func (m *mockBookingRepo) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error { return nil }


>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
func TestUpdateStatus_RolePermissions(t *testing.T) {
	t.Run("Therapist sets on_the_way", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		// Use nil for other services as they are not critical for this permission check or handled gracefully
		svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		ctx := context.Background()
		bookingID, actorID := int64(10), int64(42)
		tid := int64(2)

<<<<<<< HEAD
		// Expect UpdateStatus to be called
		mockRepo.On("UpdateStatus", ctx, bookingID, actorID, model.RoleTherapist, model.BookingStatusOnTheWay, (*string)(nil), (*string)(nil)).Return(nil)
=======
	mock := &mockBookingRepo{}
	// service.NewBookingService requires promoRepo, db and queueRepo
	svc := NewBookingService(mock, nil, nil, &nilQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996

		// Expect broadcast to fetch booking
		mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
			BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: model.BookingStatusOnTheWay,
		}, nil)

		// Expect final fetch
		mockRepo.On("GetByID", ctx, bookingID, actorID).Return(&model.Booking{
			BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: model.BookingStatusOnTheWay,
		}, nil)

		booking, err := svc.UpdateStatus(ctx, bookingID, actorID, model.RoleTherapist, &model.UpdateBookingStatusRequest{Status: model.BookingStatusOnTheWay})

		assert.NoError(t, err)
		assert.Equal(t, model.BookingStatusOnTheWay, booking.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Client tries to set on_the_way (Forbidden)", func(t *testing.T) {
		mockRepo := new(MockBookingRepository)
		svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		ctx := context.Background()

		// Should NOT call UpdateStatus
		_, err := svc.UpdateStatus(ctx, 11, 100, model.RoleClient, &model.UpdateBookingStatusRequest{Status: model.BookingStatusOnTheWay})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
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

		// Expect broadcast
		mockRepo.On("GetByBookingID", ctx, bookingID).Return(&model.Booking{
			BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: model.BookingStatusCancelled,
		}, nil)
		
		// Expect final fetch
		mockRepo.On("GetByID", ctx, bookingID, actorID).Return(&model.Booking{
			BookingID: bookingID, ClientID: 1, TherapistID: &tid, Status: model.BookingStatusCancelled,
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
		mockRepo.On("UpdateStatus", ctx, bookingID, int64(1), role, model.BookingStatusOnTheWay, mock.Anything, mock.Anything).
			Return(errors.New("db error"))

		_, err := svc.UpdateStatus(ctx, bookingID, 1, role, &model.UpdateBookingStatusRequest{Status: model.BookingStatusOnTheWay})

		assert.Error(t, err)
		assert.Equal(t, "db error", err.Error())
	})
}

// Mocks removed - now using common_test.go
