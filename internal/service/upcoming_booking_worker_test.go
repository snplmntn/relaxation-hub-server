package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type mockBookingRepoReminder struct {
	jobs             []repository.BookingReminderJob
	claimCalled      bool
	claimLimit       int
	processedJobIDs  []int64
	insertedEventIDs []int64
	listUpcomingUsed bool
}

func (m *mockBookingRepoReminder) ClaimDueReminderJobs(ctx context.Context, now time.Time, limit int) ([]repository.BookingReminderJob, error) {
	m.claimCalled = true
	m.claimLimit = limit
	return m.jobs, nil
}

func (m *mockBookingRepoReminder) MarkReminderJobProcessed(ctx context.Context, jobID int64) error {
	m.processedJobIDs = append(m.processedJobIDs, jobID)
	return nil
}

func (m *mockBookingRepoReminder) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	m.insertedEventIDs = append(m.insertedEventIDs, bookingID)
	return nil
}

func (m *mockBookingRepoReminder) ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error) {
	m.listUpcomingUsed = true
	return nil, nil
}

func (m *mockBookingRepoReminder) EnqueueReminderJobs(ctx context.Context, bookingID int64, now time.Time) error {
	return nil
}

type mockNotificationRepoReminder struct {
	created []model.Notification
}

func (m *mockNotificationRepoReminder) Create(ctx context.Context, n *model.Notification) error {
	m.created = append(m.created, *n)
	return nil
}

func (m *mockNotificationRepoReminder) CreateMany(ctx context.Context, notifications []*model.Notification) error {
	for _, notification := range notifications {
		m.created = append(m.created, *notification)
	}
	return nil
}

func (m *mockNotificationRepoReminder) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Notification, int, error) {
	return nil, 0, nil
}

func (m *mockNotificationRepoReminder) ListByUserKeyset(ctx context.Context, userID int64, cursor *model.KeysetCursor, limit int) ([]model.Notification, error) {
	return nil, nil
}
func (m *mockNotificationRepoReminder) CountUnreadByUser(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}
func (m *mockNotificationRepoReminder) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	return nil
}
func (m *mockNotificationRepoReminder) MarkAllAsRead(ctx context.Context, userID int64) error {
	return nil
}

func TestUpcomingBookingWorker_ProcessOnce_ClaimsDueReminderJobsWithoutWindowScan(t *testing.T) {
	scheduled := time.Date(2026, time.May, 12, 10, 0, 0, 0, time.UTC)
	therapistID := int64(22)
	bookingRepo := &mockBookingRepoReminder{jobs: []repository.BookingReminderJob{{
		JobID:          1,
		BookingID:      101,
		EventType:      "reminder_24h",
		ScheduledStart: scheduled,
		Booking: model.Booking{
			BookingID:      101,
			ClientID:       11,
			TherapistID:    &therapistID,
			ScheduledStart: &scheduled,
			Status:         model.BookingStatusAssigned,
		},
	}}}
	notificationRepo := &mockNotificationRepoReminder{}
	worker := NewUpcomingBookingWorker(bookingRepo, NewNotificationService(notificationRepo, nil, nil))

	worker.processOnce(context.Background())

	if !bookingRepo.claimCalled {
		t.Fatalf("expected worker to claim due reminder jobs")
	}
	if bookingRepo.claimLimit != upcomingBookingReminderBatchLimit {
		t.Fatalf("expected claim limit %d, got %d", upcomingBookingReminderBatchLimit, bookingRepo.claimLimit)
	}
	if bookingRepo.listUpcomingUsed {
		t.Fatalf("worker must not scan upcoming booking windows")
	}
	if len(notificationRepo.created) != 2 {
		t.Fatalf("expected client and therapist notifications, got %d", len(notificationRepo.created))
	}
	if len(bookingRepo.insertedEventIDs) != 1 || bookingRepo.insertedEventIDs[0] != 101 {
		t.Fatalf("expected one booking event for booking 101, got %v", bookingRepo.insertedEventIDs)
	}
	if len(bookingRepo.processedJobIDs) != 1 || bookingRepo.processedJobIDs[0] != 1 {
		t.Fatalf("expected job 1 marked processed, got %v", bookingRepo.processedJobIDs)
	}
}

func TestUpcomingBookingWorker_ProcessOnce_SkipsCancelledAndRescheduledJobs(t *testing.T) {
	originalStart := time.Date(2026, time.May, 12, 10, 0, 0, 0, time.UTC)
	newStart := originalStart.Add(2 * time.Hour)
	bookingRepo := &mockBookingRepoReminder{jobs: []repository.BookingReminderJob{
		{
			JobID:          1,
			BookingID:      101,
			EventType:      "reminder_24h",
			ScheduledStart: originalStart,
			Booking: model.Booking{
				BookingID:      101,
				ClientID:       11,
				ScheduledStart: &originalStart,
				Status:         model.BookingStatusCancelled,
			},
		},
		{
			JobID:          2,
			BookingID:      102,
			EventType:      "reminder_2h",
			ScheduledStart: originalStart,
			Booking: model.Booking{
				BookingID:      102,
				ClientID:       12,
				ScheduledStart: &newStart,
				Status:         model.BookingStatusAssigned,
			},
		},
	}}
	notificationRepo := &mockNotificationRepoReminder{}
	worker := NewUpcomingBookingWorker(bookingRepo, NewNotificationService(notificationRepo, nil, nil))

	worker.processOnce(context.Background())

	if len(notificationRepo.created) != 0 {
		t.Fatalf("expected no notifications for cancelled/rescheduled jobs, got %d", len(notificationRepo.created))
	}
	if len(bookingRepo.insertedEventIDs) != 0 {
		t.Fatalf("expected no booking events for skipped jobs, got %v", bookingRepo.insertedEventIDs)
	}
	if len(bookingRepo.processedJobIDs) != 2 {
		t.Fatalf("expected skipped jobs marked processed, got %v", bookingRepo.processedJobIDs)
	}
}

func (m *mockBookingRepoReminder) Create(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoReminder) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoReminder) Update(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoReminder) UpdateAdmin(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoReminder) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	return nil
}
func (m *mockBookingRepoReminder) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return nil
}
func (m *mockBookingRepoReminder) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	return nil
}
func (m *mockBookingRepoReminder) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoReminder) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoReminder) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockBookingRepoReminder) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	return nil
}
func (m *mockBookingRepoReminder) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	return nil
}
func (m *mockBookingRepoReminder) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoReminder) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoReminder) RevertOnTheWayToAssigned(ctx context.Context, bookingID, actorID int64) (*repository.RevertOnTheWayToAssignedResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoReminder) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoReminder) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status, dateFrom, dateTo string) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoReminder) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListDueInProgressBookings(ctx context.Context, now time.Time, limit int) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListAllEvents(ctx context.Context, params repository.ListAllEventsParams) ([]model.BookingEvent, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoReminder) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) HasAssignedOutboundRiderCoverage(ctx context.Context, bookingID int64) (bool, error) {
	return false, nil
}
func (m *mockBookingRepoReminder) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockBookingRepoReminder) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) {
	return nil, nil
}
func (m *mockBookingRepoReminder) ListByRecurringID(ctx context.Context, recurringID int64, after time.Time, limit int) ([]model.Booking, error) { return nil, nil }
