package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// mockBookingRepoUnassign
type mockBookingRepoUnassign struct {
	bookingID      int64
	therapistID    int64
	dailyCount     int
	weeklyCount    int
	countCalls     int
	unassigned     bool
	insertedEvents []string
}

func (m *mockBookingRepoUnassign) UpdatePayoutReference(ctx context.Context, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoUnassign) UpdatePayoutReferenceTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoUnassign) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*repository.BookingDetailsResult, error) {
	return nil, nil
}

func (m *mockBookingRepoUnassign) Create(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoUnassign) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoUnassign) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) Update(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoUnassign) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	return nil
}
func (m *mockBookingRepoUnassign) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoUnassign) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoUnassign) UpdateAdmin(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoUnassign) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	if bookingID == m.bookingID {
		tid := m.therapistID
		return &model.Booking{
			BookingID:   bookingID,
			TherapistID: &tid,
			ClientID:    100,
			Status:      "assigned",
		}, nil
	}
	return nil, pgx.ErrNoRows
}
func (m *mockBookingRepoUnassign) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) {
	return m.GetByBookingID(ctx, bookingID)
}
func (m *mockBookingRepoUnassign) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	m.insertedEvents = append(m.insertedEvents, eventType)
	return nil
}
func (m *mockBookingRepoUnassign) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	return nil
}
func (m *mockBookingRepoUnassign) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return nil
}
func (m *mockBookingRepoUnassign) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	return nil
}
func (m *mockBookingRepoUnassign) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	return nil
}
func (m *mockBookingRepoUnassign) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoUnassign) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoUnassign) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
	return nil
}
func (m *mockBookingRepoUnassign) ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
	m.unassigned = true
	m.insertedEvents = append(m.insertedEvents, model.EventTypeUnassigned)
	return nil
}
func (m *mockBookingRepoUnassign) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
	m.countCalls++
	if m.countCalls%2 == 0 {
		return m.weeklyCount, nil
	}
	return m.dailyCount, nil
}
func (m *mockBookingRepoUnassign) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
	return &repository.ClientBookingStats{}, nil
}
func (m *mockBookingRepoUnassign) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) {
	return &repository.AccountingSummary{}, nil
}
func (m *mockBookingRepoUnassign) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) {
	return nil, nil
}
func (m *mockBookingRepoUnassign) HasAssignedOutboundRiderCoverage(ctx context.Context, bookingID int64) (bool, error) {
	return true, nil
}

func (m *mockBookingRepoUnassign) RevertOnTheWayToAssigned(ctx context.Context, bookingID, actorID int64) (*repository.RevertOnTheWayToAssignedResult, error) {
	return nil, nil
}

func (m *mockBookingRepoUnassign) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoUnassign) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoUnassign) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}

// Mock Therapist Repo
type mockTherapistRepoUnassign struct {
	assignedEnabled bool
	suspendedID     int64
}

func (m *mockTherapistRepoUnassign) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error {
	if val, ok := updates["accept_assignments"]; ok {
		if enabled, isBool := val.(bool); isBool {
			m.assignedEnabled = enabled
			m.suspendedID = therapistID
		}
	}
	return nil
}
func (m *mockTherapistRepoUnassign) SetLifecycleStatus(ctx context.Context, therapistID int64, accountStatus string, acceptAssignments bool) error {
	m.assignedEnabled = acceptAssignments
	m.suspendedID = therapistID
	return nil
}
func (m *mockTherapistRepoUnassign) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoUnassign) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoUnassign) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error {
	return nil
}
func (m *mockTherapistRepoUnassign) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) {
	return nil, nil
}
func (m *mockTherapistRepoUnassign) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error {
	return nil
}
func (m *mockTherapistRepoUnassign) AddService(ctx context.Context, ts *model.TherapistService) error {
	return nil
}
func (m *mockTherapistRepoUnassign) RemoveService(ctx context.Context, therapistID, serviceID int64) error {
	return nil
}
func (m *mockTherapistRepoUnassign) GetServices(ctx context.Context, therapistID int64) ([]int64, error) {
	return nil, nil
}
func (m *mockTherapistRepoUnassign) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error {
	return nil
}
func (m *mockTherapistRepoUnassign) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) {
	return nil, nil
}
func (m *mockTherapistRepoUnassign) CreateProfile(ctx context.Context, therapistID int64) error {
	return nil
}
func (m *mockTherapistRepoUnassign) FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoUnassign) FindAvailableByServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoUnassign) FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoUnassign) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (m *mockTherapistRepoUnassign) SetAtBranch(ctx context.Context, therapistID int64, atBranch bool) error {
	return nil
}
func (m *mockTherapistRepoUnassign) TryLockTherapist(ctx context.Context, therapistID int64) (bool, error) {
	return true, nil
}
func (m *mockTherapistRepoUnassign) TryLockTherapistTx(ctx context.Context, tx pgx.Tx, therapistID int64) (bool, error) {
	return true, nil
}
func (m *mockTherapistRepoUnassign) SetBatchServices(ctx context.Context, therapistID int64, serviceIDs []model.AddServiceWithPressuresRequest) error {
	return nil
}

// Mock User Repo for Admins
type mockUserRepoUnassign struct {
	admins []model.User
}

func (m *mockUserRepoUnassign) CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
	return nil
}
func (m *mockUserRepoUnassign) CreateUserIdentityAndTherapistProfile(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
	return nil
}
func (m *mockUserRepoUnassign) FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
	return nil, nil
}
func (m *mockUserRepoUnassign) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	return nil, nil
}
func (m *mockUserRepoUnassign) UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error {
	return nil
}
func (m *mockUserRepoUnassign) ListUsers(ctx context.Context, roleFilter string) ([]model.User, error) {
	if roleFilter == "admin" {
		return m.admins, nil
	}
	return nil, nil
}
func (m *mockUserRepoUnassign) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
	return nil
}
func (m *mockUserRepoUnassign) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
	return nil
}
func (m *mockUserRepoUnassign) ListUsersPaginated(ctx context.Context, roleFilter string, limit, offset int, search string) ([]model.User, int, error) {
	return nil, 0, nil
}
func (m *mockUserRepoUnassign) SuspendUserSystem(ctx context.Context, userID int64, reason string) error {
	return nil
}
func (m *mockUserRepoUnassign) IsBlocked(ctx context.Context, userA, userB int64) (bool, error) {
	return false, nil
}
func (m *mockUserRepoUnassign) GetBlockList(ctx context.Context, userID int64) ([]repository.BlockedUserEntry, error) {
	return nil, nil
}
func (m *mockUserRepoUnassign) UpdateFCMToken(ctx context.Context, userID int64, token string) error {
	return nil
}
func (m *mockUserRepoUnassign) GetFCMToken(ctx context.Context, userID int64) (*string, error) {
	s := ""
	return &s, nil
}
func (m *mockUserRepoUnassign) GetUserInfoBatch(ctx context.Context, userIDs []int64) (map[int64]*repository.UserInfo, error) {
	return nil, nil
}
func (m *mockUserRepoUnassign) GetTherapistInfoBatch(ctx context.Context, therapistIDs []int64) (map[int64]*repository.TherapistInfo, error) {
	return nil, nil
}
func (m *mockUserRepoUnassign) AddFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	return nil
}
func (m *mockUserRepoUnassign) RemoveFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	return nil
}
func (m *mockUserRepoUnassign) ListFavoriteTherapists(ctx context.Context, userID int64) ([]model.User, error) {
	return nil, nil
}
func (m *mockUserRepoUnassign) IsTherapistFavorite(ctx context.Context, userID, therapistID int64) (bool, error) {
	return false, nil
}
func (m *mockUserRepoUnassign) BanUserSystem(ctx context.Context, userID int64, reason string) error {
	return nil
}
func (m *mockUserRepoUnassign) Create(ctx context.Context, user *model.User) error { return nil }
func (m *mockUserRepoUnassign) GetByID(ctx context.Context, userID int64) (*model.User, error) {
	return nil, nil
}
func (m *mockUserRepoUnassign) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	return nil, nil
}
func (m *mockUserRepoUnassign) Update(ctx context.Context, user *model.User) error { return nil }
func (m *mockUserRepoUnassign) SetOneSignalPlayerID(ctx context.Context, userID int64, playerID string) error {
	return nil
}
func (m *mockUserRepoUnassign) Delete(ctx context.Context, userID int64) error { return nil }

// Mock Notification Repo
type mockNotificationRepoCapture struct {
	captured []*model.Notification
}

func (m *mockNotificationRepoCapture) Create(ctx context.Context, n *model.Notification) error {
	m.captured = append(m.captured, n)
	return nil
}
func (m *mockNotificationRepoCapture) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Notification, int, error) {
	return nil, 0, nil
}
func (m *mockNotificationRepoCapture) MarkAsRead(ctx context.Context, notificationID int64, userID int64) error {
	return nil
}
func (m *mockNotificationRepoCapture) MarkAllAsRead(ctx context.Context, userID int64) error {
	return nil
}
func (m *mockNotificationRepoCapture) CountUnread(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}
func (m *mockNotificationRepoCapture) CountUnreadByUser(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}
func (m *mockNotificationRepoCapture) DeleteOld(ctx context.Context, olderThan time.Duration) error {
	return nil
}

func containsEvent(events []string, target string) bool {
	for _, e := range events {
		if e == target {
			return true
		}
	}
	return false
}

func TestUnassignTherapist_Limits(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(10)
	therapistID := int64(5)

	t.Run("Daily Limit Warning", func(t *testing.T) {
		mockBooking := &mockBookingRepoUnassign{
			bookingID:   bookingID,
			therapistID: therapistID,
			dailyCount:  3,
			weeklyCount: 3,
		}
		mockTher := &mockTherapistRepoUnassign{assignedEnabled: true}
		mockUser := &mockUserRepoUnassign{admins: []model.User{{UserID: 999, Role: "admin"}}}

		mockNotifRepo := &mockNotificationRepoCapture{}
		fcmService := (*FCMService)(nil)
		notifSvc := NewNotificationService(mockNotifRepo, mockUser, fcmService)

		mockQueue := &nilAssignmentQueueRepo{}
		mockOffer := &mockOfferRepoAccept{offers: map[int64]*model.BookingOffer{}}

		svc := NewBookingService(mockBooking, nil, nil, mockQueue, mockTher, mockOffer, nil, nil, mockUser, nil, notifSvc, nil, nil, nil)

		err := svc.UnassignTherapist(ctx, bookingID, therapistID, model.RoleTherapist, nil)
		if err != nil {
			t.Fatalf("UnassignTherapist error: %v", err)
		}

		if !containsEvent(mockBooking.insertedEvents, model.EventTypeUnassigned) {
			t.Error("Expected unassigned event")
		}

		foundWarning := false
		for _, n := range mockNotifRepo.captured {
			if strings.Contains(n.Title, "Therapist Unassignment Warning") && n.UserID == 999 {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Error("Expected Daily Limit Warning notification to admin")
		}

		if !mockTher.assignedEnabled {
			t.Error("Therapist should NOT be suspended for daily limit only")
		}
	})

	t.Run("Weekly Limit Suspension", func(t *testing.T) {
		mockBooking := &mockBookingRepoUnassign{
			bookingID:   bookingID,
			therapistID: therapistID,
			dailyCount:  1,
			weeklyCount: 5,
		}
		mockTher := &mockTherapistRepoUnassign{assignedEnabled: true}
		mockUser := &mockUserRepoUnassign{admins: []model.User{{UserID: 999, Role: "admin"}}}

		mockNotifRepo := &mockNotificationRepoCapture{}
		notifSvc := NewNotificationService(mockNotifRepo, mockUser, nil)
		mockQueue := &nilAssignmentQueueRepo{}
		mockOffer := &mockOfferRepoAccept{offers: map[int64]*model.BookingOffer{}}

		svc := NewBookingService(mockBooking, nil, nil, mockQueue, mockTher, mockOffer, nil, nil, mockUser, nil, notifSvc, nil, nil, nil)

		err := svc.UnassignTherapist(ctx, bookingID, therapistID, model.RoleTherapist, nil)
		if err != nil {
			t.Fatalf("UnassignTherapist error: %v", err)
		}

		if mockTher.assignedEnabled {
			t.Error("Therapist SHOULD be suspended (accept_assignments=false)")
		}
		if mockTher.suspendedID != therapistID {
			t.Errorf("Expected therapist %d to be suspended, got %d", therapistID, mockTher.suspendedID)
		}

		if !containsEvent(mockBooking.insertedEvents, "therapist_suspended_auto") {
			t.Error("Expected therapist_suspended_auto event")
		}

		foundSuspension := false
		for _, n := range mockNotifRepo.captured {
			if strings.Contains(n.Title, "Therapist Auto-Suspended") && n.UserID == 999 {
				foundSuspension = true
				break
			}
		}
		if !foundSuspension {
			t.Error("Expected Suspension notification to admin")
		}
	})
}

func (m *mockBookingRepoUnassign) ListAllEvents(ctx context.Context, params repository.ListAllEventsParams) ([]model.BookingEvent, int, error) {
	return nil, 0, nil
}
