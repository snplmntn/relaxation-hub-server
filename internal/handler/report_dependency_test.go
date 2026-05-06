package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type dependencyUnavailableResponse struct {
	Error struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		Dependency string `json:"dependency"`
		Retryable  bool   `json:"retryable"`
	} `json:"error"`
}

func TestGetLedgerSummary_MissingLedgerReturnsStructuredDependencyUnavailable(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/reports/ledger/summary", nil)
	w := httptest.NewRecorder()

	h.GetLedgerSummary(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", w.Code, w.Body.String())
	}

	var resp dependencyUnavailableResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Code != "dependency_unavailable" {
		t.Fatalf("expected code dependency_unavailable, got %q", resp.Error.Code)
	}
	if resp.Error.Dependency != string(reportDependencyLedgerRepo) {
		t.Fatalf("expected dependency %q, got %q", reportDependencyLedgerRepo, resp.Error.Dependency)
	}
	if !resp.Error.Retryable {
		t.Fatal("expected retryable=true")
	}
}

func TestGetReferralSummary_MissingReferralRepoReturnsStructuredDependencyUnavailable(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/reports/referrals/summary", nil)
	w := httptest.NewRecorder()

	h.GetReferralSummary(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", w.Code, w.Body.String())
	}

	var resp dependencyUnavailableResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Dependency != string(reportDependencyBookingReferralRepo) {
		t.Fatalf("expected dependency %q, got %q", reportDependencyBookingReferralRepo, resp.Error.Dependency)
	}
}

func TestListRiderPayoutRequests_MissingRiderWalletReturnsStructuredDependencyUnavailable(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/reports/payouts/requests", nil)
	w := httptest.NewRecorder()

	h.ListRiderPayoutRequests(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", w.Code, w.Body.String())
	}

	var resp dependencyUnavailableResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Dependency != string(reportDependencyRiderWalletService) {
		t.Fatalf("expected dependency %q, got %q", reportDependencyRiderWalletService, resp.Error.Dependency)
	}
}

func TestUploadExpenseReceipt_MissingStorageReturnsStructuredDependencyUnavailable(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/reports/expenses/upload", nil)
	w := httptest.NewRecorder()

	h.UploadExpenseReceipt(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", w.Code, w.Body.String())
	}

	var resp dependencyUnavailableResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Dependency != string(reportDependencyStorageService) {
		t.Fatalf("expected dependency %q, got %q", reportDependencyStorageService, resp.Error.Dependency)
	}
}

func TestReportDependencyMatrix_PreservesPartialAvailability(t *testing.T) {
	ledgerDeps := reportDependenciesFor(reportOperationGetLedgerSummary)
	if len(ledgerDeps) != 1 || ledgerDeps[0] != reportDependencyLedgerRepo {
		t.Fatalf("expected ledger summary to depend only on ledgerRepo, got %v", ledgerDeps)
	}

	referralDeps := reportDependenciesFor(reportOperationGetReferralSummary)
	if len(referralDeps) != 1 || referralDeps[0] != reportDependencyBookingReferralRepo {
		t.Fatalf("expected referral summary to depend only on bookingReferralRepo, got %v", referralDeps)
	}

	resolveDeps := reportDependenciesFor(reportOperationResolveRiderPayoutRequest)
	if len(resolveDeps) != 1 {
		t.Fatalf("expected resolve rider payout request base guard to require 1 dep, got %v", resolveDeps)
	}
	if resolveDeps[0] != reportDependencyRiderWalletService {
		t.Fatalf("unexpected base dependency order for rider payout resolution: %v", resolveDeps)
	}
}

func TestReportDependencyStatusProvider_SnapshotAggregatesDegradedState(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)
	provider := NewReportDependencyStatusProvider(h, nil)

	snapshot := provider.Snapshot(context.Background())

	if snapshot.Status != "degraded" {
		t.Fatalf("expected status degraded, got %q", snapshot.Status)
	}
	if !snapshot.Degraded {
		t.Fatal("expected degraded=true")
	}
	if snapshot.Dependencies[string(reportDependencyLedgerRepo)].Available {
		t.Fatal("expected ledgerRepo unavailable")
	}
	if snapshot.Dependencies[string(reportDependencyStorageService)].Available {
		t.Fatal("expected storageService unavailable")
	}
}

func TestGetLedgerSummary_DatabaseHealthCheckFailureReturnsStructuredDependencyUnavailable(t *testing.T) {
	h := NewReportHandler(nil, &mockLedgerRepoReport{}, nil, nil)
	h.SetDependencyStatusProvider(NewReportDependencyStatusProvider(h, func(context.Context) error {
		return errors.New("db ping failed")
	}))

	req := httptest.NewRequest("GET", "/reports/ledger/summary", nil)
	w := httptest.NewRecorder()

	h.GetLedgerSummary(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", w.Code, w.Body.String())
	}

	var resp dependencyUnavailableResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Dependency != string(reportDependencyLedgerRepo) {
		t.Fatalf("expected dependency %q, got %q", reportDependencyLedgerRepo, resp.Error.Dependency)
	}
	if resp.Error.Message != "database unavailable: db ping failed" {
		t.Fatalf("unexpected message %q", resp.Error.Message)
	}
}

type mockReportStorageService struct {
	configured bool
	healthErr  error
}

func (m *mockReportStorageService) UploadFile(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	return "", nil
}

func (m *mockReportStorageService) GetFileURL(key string) string {
	return ""
}

func (m *mockReportStorageService) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "", nil
}

func (m *mockReportStorageService) DeleteFile(ctx context.Context, key string) error {
	return nil
}

func (m *mockReportStorageService) GenerateKey(prefix, filename string) string {
	return prefix + "/" + filename
}

func (m *mockReportStorageService) IsConfigured() bool {
	return m.configured
}

func (m *mockReportStorageService) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

func TestUploadExpenseReceipt_StorageHealthCheckFailureReturnsStructuredDependencyUnavailable(t *testing.T) {
	storage := &mockReportStorageService{
		configured: true,
		healthErr:  errors.New("bucket unreachable"),
	}
	h := NewReportHandler(nil, nil, storage, nil)
	h.SetDependencyStatusProvider(NewReportDependencyStatusProvider(h, nil))

	req := httptest.NewRequest("POST", "/reports/expenses/upload", nil)
	w := httptest.NewRecorder()

	h.UploadExpenseReceipt(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", w.Code, w.Body.String())
	}

	var resp dependencyUnavailableResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Dependency != string(reportDependencyStorageService) {
		t.Fatalf("expected dependency %q, got %q", reportDependencyStorageService, resp.Error.Dependency)
	}
	if resp.Error.Message != "storage unavailable: bucket unreachable" {
		t.Fatalf("unexpected message %q", resp.Error.Message)
	}
}
