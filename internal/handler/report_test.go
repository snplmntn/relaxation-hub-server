package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type mockBookingRepoReport struct {
	summary *repository.AccountingSummary
	daily   []repository.DailyAccountingEntry
	err     error
}

func (m *mockBookingRepoReport) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*repository.AccountingSummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.summary == nil {
		return &repository.AccountingSummary{}, nil
	}
	return m.summary, nil
}

func (m *mockBookingRepoReport) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]repository.DailyAccountingEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.daily, nil
}

// Stubs for BookingRepository interface
func (m *mockBookingRepoReport) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoReport) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoReport) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoReport) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	return nil
}
func (m *mockBookingRepoReport) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	return nil
}
func (m *mockBookingRepoReport) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	return nil
}
func (m *mockBookingRepoReport) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoReport) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	return nil
}
func (m *mockBookingRepoReport) UpdateAdmin(ctx context.Context, booking *model.Booking) error {
	return nil
}
func (m *mockBookingRepoReport) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockBookingRepoReport) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	return nil
}
func (m *mockBookingRepoReport) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	return nil
}
func (m *mockBookingRepoReport) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoReport) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoReport) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error {
	return nil
}
func (m *mockBookingRepoReport) ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
	return nil
}
func (m *mockBookingRepoReport) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockBookingRepoReport) HasAssignedOutboundRiderCoverage(ctx context.Context, bookingID int64) (bool, error) {
	return true, nil
}
func (m *mockBookingRepoReport) FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) RevertOnTheWayToAssigned(ctx context.Context, bookingID, actorID int64) (*repository.RevertOnTheWayToAssignedResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoReport) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]repository.BookingDetailsResult, int, error) {
	return nil, 0, nil
}
func (m *mockBookingRepoReport) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	return nil
}
func (m *mockBookingRepoReport) UpdatePayoutReference(ctx context.Context, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoReport) UpdatePayoutReferenceTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64, payoutID int64) error {
	return nil
}
func (m *mockBookingRepoReport) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*repository.BookingDetailsResult, error) {
	return nil, nil
}
func (m *mockBookingRepoReport) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	return nil, nil
}

func TestGetAccountingSummary(t *testing.T) {
	mockRepo := &mockBookingRepoReport{
		summary: &repository.AccountingSummary{
			TotalRevenue:          1000,
			TotalTherapistPayouts: 600,
			TotalPlatformProfit:   400,
			BookingCount:          10,
		},
	}
	h := NewReportHandler(mockRepo, nil, nil, nil)

	req := httptest.NewRequest("GET", "/admin/reports/accounting/summary", nil)
	w := httptest.NewRecorder()

	h.GetAccountingSummary(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var result AccountingSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.TotalRevenue != 1000 {
		t.Errorf("expected revenue 1000, got %f", result.TotalRevenue)
	}
	if result.BookingCount != 10 {
		t.Errorf("expected booking count 10, got %d", result.BookingCount)
	}
}

func TestGetDailyAccounting(t *testing.T) {
	mockRepo := &mockBookingRepoReport{
		daily: []repository.DailyAccountingEntry{
			{Date: time.Now(), Revenue: 100, TherapistPayouts: 60, PlatformProfit: 40, BookingCount: 1},
		},
	}
	h := NewReportHandler(mockRepo, nil, nil, nil)

	req := httptest.NewRequest("GET", "/admin/reports/accounting/daily", nil)
	w := httptest.NewRecorder()

	h.GetDailyAccounting(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := result["data"].([]any)
	if !ok {
		t.Fatalf("expected data field to be array")
	}
	if len(data) != 1 {
		t.Errorf("expected 1 entry, got %d", len(data))
	}
}

func (m *mockBookingRepoReport) ListAllEvents(ctx context.Context, params repository.ListAllEventsParams) ([]model.BookingEvent, int, error) {
	return nil, 0, nil
}

type mockBookingReferralRepoReport struct {
	totals    []model.BookingReferralSummaryTotal
	series    []model.BookingReferralSummaryPoint
	totalsErr error
	seriesErr error

	totalsStartDate time.Time
	totalsEndDate   time.Time
	seriesStartDate time.Time
	seriesEndDate   time.Time
	seriesBucket    string
}

func (m *mockBookingReferralRepoReport) CreateTx(ctx context.Context, tx pgx.Tx, referral *model.BookingReferral) error {
	return nil
}

func (m *mockBookingReferralRepoReport) ListSummaryTotals(ctx context.Context, startDate, endDate time.Time) ([]model.BookingReferralSummaryTotal, error) {
	m.totalsStartDate = startDate
	m.totalsEndDate = endDate
	if m.totalsErr != nil {
		return nil, m.totalsErr
	}
	return m.totals, nil
}

func (m *mockBookingReferralRepoReport) ListSummarySeries(ctx context.Context, startDate, endDate time.Time, bucket string) ([]model.BookingReferralSummaryPoint, error) {
	m.seriesStartDate = startDate
	m.seriesEndDate = endDate
	m.seriesBucket = bucket
	if m.seriesErr != nil {
		return nil, m.seriesErr
	}
	return m.series, nil
}

func TestGetReferralSummary_UsesExclusiveEndDate(t *testing.T) {
	startDate, _ := time.Parse("2006-01-02", "2026-02-01")
	endDateInclusive, _ := time.Parse("2006-01-02", "2026-02-10")
	endDateExclusive, _ := time.Parse("2006-01-02", "2026-02-11")

	mockReferralRepo := &mockBookingReferralRepoReport{
		totals: []model.BookingReferralSummaryTotal{
			{Source: "Facebook", Count: 2},
		},
		series: []model.BookingReferralSummaryPoint{
			{PeriodStart: endDateInclusive, Source: "Facebook", Count: 2},
		},
	}

	h := NewReportHandler(nil, nil, nil, nil)
	h.SetBookingReferralRepository(mockReferralRepo)

	req := httptest.NewRequest(
		"GET",
		"/reports/referrals/summary?start_date=2026-02-01&end_date=2026-02-10&bucket=day",
		nil,
	)
	w := httptest.NewRecorder()

	h.GetReferralSummary(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	if !mockReferralRepo.totalsStartDate.Equal(startDate) {
		t.Fatalf("expected totals start %v, got %v", startDate, mockReferralRepo.totalsStartDate)
	}
	if !mockReferralRepo.totalsEndDate.Equal(endDateExclusive) {
		t.Fatalf("expected totals end %v, got %v", endDateExclusive, mockReferralRepo.totalsEndDate)
	}
	if !mockReferralRepo.seriesStartDate.Equal(startDate) {
		t.Fatalf("expected series start %v, got %v", startDate, mockReferralRepo.seriesStartDate)
	}
	if !mockReferralRepo.seriesEndDate.Equal(endDateExclusive) {
		t.Fatalf("expected series end %v, got %v", endDateExclusive, mockReferralRepo.seriesEndDate)
	}
	if mockReferralRepo.seriesBucket != "day" {
		t.Fatalf("expected series bucket day, got %s", mockReferralRepo.seriesBucket)
	}

	var result struct {
		EndDate        string `json:"end_date"`
		Bucket         string `json:"bucket"`
		TotalResponses int64  `json:"total_responses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.EndDate != "2026-02-10" {
		t.Fatalf("expected response end_date 2026-02-10, got %s", result.EndDate)
	}
	if result.Bucket != "day" {
		t.Fatalf("expected response bucket day, got %s", result.Bucket)
	}
	if result.TotalResponses != 2 {
		t.Fatalf("expected total_responses 2, got %d", result.TotalResponses)
	}
}
