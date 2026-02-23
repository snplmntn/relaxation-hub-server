package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// mockLedgerRepoReport implements repository.LedgerRepository for handler tests.
type mockLedgerRepoReport struct {
	voidedID     int64
	voidedReason string
	voidErr      error

	// Configurable stubs for new tests
	payoutBalances    []repository.PayoutBalance
	payoutBalancesErr error
	settlementErr     error
	settlementCalled  bool
}

func (m *mockLedgerRepoReport) Insert(ctx context.Context, entry *repository.LedgerEntry) error {
	return nil
}
func (m *mockLedgerRepoReport) InsertBookingEntries(ctx context.Context, bookingID int64, therapistID *int64, revenue, payout, commission float64, entryDate time.Time) error {
	return nil
}
func (m *mockLedgerRepoReport) InsertExpense(ctx context.Context, amount float64, description string, category repository.LedgerCategory, entryDate time.Time, createdBy int64, proofURL *string) error {
	return nil
}
func (m *mockLedgerRepoReport) GetSummary(ctx context.Context, startDate, endDate time.Time) (*repository.LedgerSummary, error) {
	return nil, nil
}
func (m *mockLedgerRepoReport) GetSummaryByPeriod(ctx context.Context, startDate, endDate time.Time, granularity string) ([]repository.LedgerPeriodSummary, error) {
	return nil, nil
}
func (m *mockLedgerRepoReport) ListByBookingID(ctx context.Context, bookingID int64) ([]repository.LedgerEntry, error) {
	return nil, nil
}
func (m *mockLedgerRepoReport) ListExpenses(ctx context.Context, startDate, endDate time.Time) ([]repository.LedgerEntry, error) {
	return nil, nil
}
func (m *mockLedgerRepoReport) DeleteExpense(ctx context.Context, entryID int64) error { return nil }
func (m *mockLedgerRepoReport) GetPayoutBalance(ctx context.Context, userID int64, role repository.TargetRole) (float64, error) {
	return 0, nil
}
func (m *mockLedgerRepoReport) RecordSettlement(ctx context.Context, userID int64, role repository.TargetRole, amount float64, reference string, recordedBy int64) error {
	m.settlementCalled = true
	return m.settlementErr
}
func (m *mockLedgerRepoReport) GetPayoutBalances(ctx context.Context) ([]repository.PayoutBalance, error) {
	return m.payoutBalances, m.payoutBalancesErr
}
func (m *mockLedgerRepoReport) ListEntries(ctx context.Context, startDate, endDate time.Time) ([]repository.LedgerEntry, error) {
	return nil, nil
}
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
	h := NewReportHandler(nil, mockRepo, nil, nil)

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
	h := NewReportHandler(nil, mockRepo, nil, nil)

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

func TestListPayoutBalances_Unified(t *testing.T) {
	therapistRole := repository.TargetRoleTherapist
	riderRole := repository.TargetRoleRider

	mockRepo := &mockLedgerRepoReport{
		payoutBalances: []repository.PayoutBalance{
			{UserID: 1, Role: therapistRole, FullName: "Anna", TotalEarned: 5000, TotalSettled: 2000, BalanceOwed: 3000},
			{UserID: 2, Role: riderRole, FullName: "Bob", TotalEarned: 1000, TotalSettled: 0, BalanceOwed: 1000},
		},
	}
	h := NewReportHandler(nil, mockRepo, nil, nil)

	req := httptest.NewRequest("GET", "/reports/payouts/balances", nil)
	w := httptest.NewRecorder()
	h.ListPayoutBalances(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string][]repository.PayoutBalance
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := body["data"]
	if len(data) != 2 {
		t.Errorf("expected 2 balances, got %d", len(data))
	}
	if data[0].Role != therapistRole || data[1].Role != riderRole {
		t.Errorf("unexpected roles: %v, %v", data[0].Role, data[1].Role)
	}
}

func TestRecordSettlement_RejectsRider(t *testing.T) {
	mockRepo := &mockLedgerRepoReport{}
	h := NewReportHandler(nil, mockRepo, nil, nil)

	body, _ := json.Marshal(map[string]any{
		"user_id": 5,
		"role":    "rider",
		"amount":  500.0,
	})
	req := httptest.NewRequest("POST", "/reports/payouts/settle", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.RecordSettlement(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for rider role, got %d", w.Code)
	}
	if mockRepo.settlementCalled {
		t.Error("settlement should not be recorded for rider role")
	}
}
