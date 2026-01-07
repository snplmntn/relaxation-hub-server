package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type mockLedgerRepoReport struct {
	voidedID     int64
	voidedReason string
	voidErr      error
}

func (m *mockLedgerRepoReport) Insert(ctx context.Context, entry *repository.LedgerEntry) error { return nil }
func (m *mockLedgerRepoReport) InsertBookingEntries(ctx context.Context, bookingID int64, therapistID *int64, revenue, payout, commission float64, entryDate time.Time) error { return nil }
func (m *mockLedgerRepoReport) InsertExpense(ctx context.Context, amount float64, description string, category repository.LedgerCategory, entryDate time.Time, createdBy int64, proofURL *string) error { return nil }
func (m *mockLedgerRepoReport) GetSummary(ctx context.Context, startDate, endDate time.Time) (*repository.LedgerSummary, error) { return nil, nil }
func (m *mockLedgerRepoReport) GetSummaryByPeriod(ctx context.Context, startDate, endDate time.Time, granularity string) ([]repository.LedgerPeriodSummary, error) { return nil, nil }
func (m *mockLedgerRepoReport) ListByBookingID(ctx context.Context, bookingID int64) ([]repository.LedgerEntry, error) { return nil, nil }
func (m *mockLedgerRepoReport) ListExpenses(ctx context.Context, startDate, endDate time.Time) ([]repository.LedgerEntry, error) { return nil, nil }
func (m *mockLedgerRepoReport) DeleteExpense(ctx context.Context, entryID int64) error { return nil }
func (m *mockLedgerRepoReport) GetTherapistBalance(ctx context.Context, therapistID int64) (float64, error) { return 0, nil }
func (m *mockLedgerRepoReport) RecordSettlement(ctx context.Context, therapistID int64, amount float64, reference string, recordedBy int64) error { return nil }
func (m *mockLedgerRepoReport) GetTherapistBalances(ctx context.Context) ([]repository.TherapistBalance, error) { return nil, nil }
func (m *mockLedgerRepoReport) ListEntries(ctx context.Context, startDate, endDate time.Time) ([]repository.LedgerEntry, error) { return nil, nil }

// VoidEntry stub implementation
func (m *mockLedgerRepoReport) VoidEntry(ctx context.Context, entryID int64, reason string) error {
	if m.voidErr != nil {
		return m.voidErr
	}
	m.voidedID = entryID
	m.voidedReason = reason
	return nil
}

func TestDeleteExpense_VoidsEntry(t *testing.T) {
	mockRepo := &mockLedgerRepoReport{}
	h := NewReportHandler(nil, mockRepo, nil)

	// Use ServeMux to handle routing and path values correctly
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /admin/reports/expenses/{id}", h.DeleteExpense)

	req := httptest.NewRequest("DELETE", "/admin/reports/expenses/123?reason=dup+entry", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d. Body: %s", resp.StatusCode, w.Body.String())
	}

	if mockRepo.voidedID != 123 {
		t.Errorf("expected voided ID 123, got %d", mockRepo.voidedID)
	}
	if mockRepo.voidedReason != "dup entry" {
		t.Errorf("expected reason 'dup entry', got '%s'", mockRepo.voidedReason)
	}
}

func TestDeleteExpense_DefaultReason(t *testing.T) {
	mockRepo := &mockLedgerRepoReport{}
	h := NewReportHandler(nil, mockRepo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /admin/reports/expenses/{id}", h.DeleteExpense)

	req := httptest.NewRequest("DELETE", "/admin/reports/expenses/456", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", resp.StatusCode)
	}

	if mockRepo.voidedReason != "Manual deletion via admin dashboard" {
		t.Errorf("expected default reason, got '%s'", mockRepo.voidedReason)
	}
}
