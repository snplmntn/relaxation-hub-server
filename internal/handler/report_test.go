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

// Stubs
func (m *mockBookingRepoReport) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoReport) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error { return nil }
func (m *mockBookingRepoReport) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) { return nil, nil }
func (m *mockBookingRepoReport) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoReport) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoReport) UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockBookingRepoReport) UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error { return nil }
func (m *mockBookingRepoReport) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoReport) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }
func (m *mockBookingRepoReport) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoReport) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoReport) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) { return nil, nil }
func (m *mockBookingRepoReport) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) { return nil, nil }
func (m *mockBookingRepoReport) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) { return nil, nil }
func (m *mockBookingRepoReport) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error { return nil }
func (m *mockBookingRepoReport) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoReport) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoReport) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoReport) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoReport) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoReport) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) { return nil, nil }
func (m *mockBookingRepoReport) ListGlobalPending(ctx context.Context) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoReport) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) { return nil, nil }
func (m *mockBookingRepoReport) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error { return nil }
func (m *mockBookingRepoReport) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error { return nil }
func (m *mockBookingRepoReport) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoReport) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) { return nil, 0, nil }
func (m *mockBookingRepoReport) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) { return nil, 0, nil }
func (m *mockBookingRepoReport) UpdatePaymentProof(ctx context.Context, bookingID int64, proofURL string) error { return nil }
func (m *mockBookingRepoReport) ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoReport) UnassignTherapist(ctx context.Context, bookingID int64) error { return nil }
func (m *mockBookingRepoReport) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*repository.ClientBookingStats, error) { return nil, nil }
func (m *mockBookingRepoReport) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) { return 0, nil }
func (m *mockBookingRepoReport) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error { return nil }
func (m *mockBookingRepoReport) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int) ([]repository.BookingDetailsResult, int, error) { return nil, 0, nil }

func TestGetAccountingSummary(t *testing.T) {
	mockRepo := &mockBookingRepoReport{
		summary: &repository.AccountingSummary{
			TotalRevenue: 1000,
			TotalTherapistPayouts: 600,
			TotalPlatformProfit: 400,
			BookingCount: 10,
		},
	}
	h := NewReportHandler(mockRepo, nil, nil)

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
	h := NewReportHandler(mockRepo, nil, nil)

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

func (m *mockBookingRepoReport) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error { return nil }
