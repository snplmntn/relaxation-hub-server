package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// --- Mocks ---

type fakePaymentRepo struct {
	repository.PaymentRepository
	payment   *model.Payment
	createErr error
	getErr    error
	updateErr error
}

func (f *fakePaymentRepo) Create(ctx context.Context, p *model.Payment) error {
	if f.createErr != nil {
		return f.createErr
	}
	p.PaymentID = 1
	return nil
}

func (f *fakePaymentRepo) GetByBookingID(ctx context.Context, bookingID int64) (*model.Payment, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.payment, nil
}

func (f *fakePaymentRepo) UpdateStatus(ctx context.Context, bookingID int64, status string, transactionID *string, webhookID *string) error {
	return f.updateErr
}

func (f *fakePaymentRepo) GetOrCreateByBookingID(ctx context.Context, bookingID int64, amount float64, gateway string) (*model.Payment, error) {
	if f.getErr != nil {
		return nil, f.getErr
	} // Reuse getErr or add new one
	if f.payment != nil {
		return f.payment, nil
	}
	return &model.Payment{PaymentID: 1, BookingID: bookingID, Amount: amount, Gateway: gateway, Status: "pending"}, nil
}
func (f *fakePaymentRepo) UpdateProofURL(ctx context.Context, bookingID int64, proofURL string) error {
	return f.updateErr
}
func (f *fakePaymentRepo) Verify(ctx context.Context, bookingID int64, verifiedBy int64, notes *string) error {
	return f.updateErr
}
func (f *fakePaymentRepo) Reject(ctx context.Context, bookingID int64, rejectedBy int64, notes *string) error {
	return f.updateErr
}
func (f *fakePaymentRepo) ClearProof(ctx context.Context, bookingID int64) error { return f.updateErr }

// --- Tests for Create ---

func TestPaymentService_Create_Success(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := NewPaymentService(repo)

	req := &model.CreatePaymentRequest{
		BookingID: 1,
		Amount:    100.50,
		Gateway:   "stripe",
	}

	payment, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payment == nil {
		t.Fatal("expected payment, got nil")
	}
	if payment.BookingID != 1 {
		t.Errorf("expected booking_id=1, got %d", payment.BookingID)
	}
	if payment.Status != "pending" {
		t.Errorf("expected status='pending', got '%s'", payment.Status)
	}
}

func TestPaymentService_Create_NilRequest(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := NewPaymentService(repo)

	_, err := svc.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
}

func TestPaymentService_Create_MissingBookingID(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := NewPaymentService(repo)

	req := &model.CreatePaymentRequest{
		Amount:  100.0,
		Gateway: "stripe",
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing booking_id, got nil")
	}
	if err.Error() != "booking_id is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentService_Create_InvalidAmount(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := NewPaymentService(repo)

	req := &model.CreatePaymentRequest{
		BookingID: 1,
		Amount:    0,
		Gateway:   "stripe",
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid amount, got nil")
	}
	if err.Error() != "amount must be positive" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentService_Create_MissingGateway(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := NewPaymentService(repo)

	req := &model.CreatePaymentRequest{
		BookingID: 1,
		Amount:    100.0,
		Gateway:   "  ",
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing gateway, got nil")
	}
	if err.Error() != "gateway is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentService_Create_RepoError(t *testing.T) {
	repo := &fakePaymentRepo{createErr: errors.New("db error")}
	svc := NewPaymentService(repo)

	req := &model.CreatePaymentRequest{
		BookingID: 1,
		Amount:    100.0,
		Gateway:   "stripe",
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
}

// --- Tests for GetByBookingID ---

func TestPaymentService_GetByBookingID_Success(t *testing.T) {
	expected := &model.Payment{PaymentID: 1, BookingID: 99, Amount: 50.0}
	repo := &fakePaymentRepo{payment: expected}
	svc := NewPaymentService(repo)

	payment, err := svc.GetByBookingID(context.Background(), 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payment == nil || payment.PaymentID != 1 {
		t.Fatal("expected payment with id=1")
	}
}

func TestPaymentService_GetByBookingID_NotFound(t *testing.T) {
	repo := &fakePaymentRepo{getErr: errors.New("no rows")}
	svc := NewPaymentService(repo)

	_, err := svc.GetByBookingID(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for not found, got nil")
	}
}

// --- Tests for UpdateStatus ---

func TestPaymentService_UpdateStatus_Success(t *testing.T) {
	expected := &model.Payment{PaymentID: 1, BookingID: 10, Status: "paid"}
	repo := &fakePaymentRepo{payment: expected}
	svc := NewPaymentService(repo)

	req := &model.UpdatePaymentStatusRequest{Status: "paid"}
	payment, err := svc.UpdateStatus(context.Background(), 10, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payment == nil {
		t.Fatal("expected payment, got nil")
	}
}

func TestPaymentService_UpdateStatus_NilRequest(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := NewPaymentService(repo)

	_, err := svc.UpdateStatus(context.Background(), 10, nil)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
}

func TestPaymentService_UpdateStatus_EmptyStatus(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := NewPaymentService(repo)

	req := &model.UpdatePaymentStatusRequest{Status: "  "}
	_, err := svc.UpdateStatus(context.Background(), 10, req)
	if err == nil {
		t.Fatal("expected error for empty status, got nil")
	}
	if err.Error() != "status is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentService_UpdateStatus_RepoError(t *testing.T) {
	repo := &fakePaymentRepo{updateErr: errors.New("db error")}
	svc := NewPaymentService(repo)

	req := &model.UpdatePaymentStatusRequest{Status: "paid"}
	_, err := svc.UpdateStatus(context.Background(), 10, req)
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
}

// --- Tests for UploadProof ---

func TestPaymentService_UploadProof_Success(t *testing.T) {
	tests := []struct {
		name      string
		bookingID int64
		proofURL  string
		amount    float64
		gateway   string
	}{
		{"Valid proof upload", 1, "https://example.com/proof.jpg", 1000.0, "gcash"},
		{"Bank transfer proof", 2, "https://example.com/bank.png", 2000.0, "bank"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			proofURL := tt.proofURL
			payment := &model.Payment{
				PaymentID: 1,
				BookingID: tt.bookingID,
				Amount:    tt.amount,
				Gateway:   tt.gateway,
				ProofURL:  &proofURL,
			}
			repo := &fakePaymentRepo{payment: payment}
			svc := NewPaymentService(repo)

			result, err := svc.UploadProof(context.Background(), tt.bookingID, tt.proofURL, tt.amount, tt.gateway)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected payment, got nil")
			}
			if result.ProofURL == nil || *result.ProofURL != tt.proofURL {
				t.Errorf("expected proof URL %s, got %v", tt.proofURL, result.ProofURL)
			}
		})
	}
}

func TestPaymentService_UploadProof_GetOrCreateError(t *testing.T) {
	repo := &fakePaymentRepo{getErr: errors.New("database error")}
	svc := NewPaymentService(repo)

	_, err := svc.UploadProof(context.Background(), 1, "https://example.com/proof.jpg", 1000.0, "gcash")
	if err == nil {
		t.Fatal("expected error from GetOrCreateByBookingID, got nil")
	}
	if err.Error() != "failed to get or create payment: database error" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPaymentService_UploadProof_UpdateError(t *testing.T) {
	repo := &fakePaymentRepo{
		payment:   &model.Payment{PaymentID: 1},
		updateErr: errors.New("update failed"),
	}
	svc := NewPaymentService(repo)

	_, err := svc.UploadProof(context.Background(), 1, "https://example.com/proof.jpg", 1000.0, "gcash")
	if err == nil {
		t.Fatal("expected error from UpdateProofURL, got nil")
	}
	if err.Error() != "failed to update proof URL: update failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Tests for Verify ---

func TestPaymentService_Verify_Success(t *testing.T) {
	proofURL := "https://example.com/proof.jpg"
	payment := &model.Payment{
		PaymentID: 1,
		BookingID: 1,
		ProofURL:  &proofURL,
		Status:    "paid",
	}
	repo := &fakePaymentRepo{payment: payment}
	svc := NewPaymentService(repo)

	notes := "Verified payment proof"
	result, err := svc.Verify(context.Background(), 1, 100, &notes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected payment, got nil")
	}
}

func TestPaymentService_Verify_PaymentNotFound(t *testing.T) {
	repo := &fakePaymentRepo{getErr: errors.New("payment not found")}
	svc := NewPaymentService(repo)

	_, err := svc.Verify(context.Background(), 999, 100, nil)
	if err == nil {
		t.Fatal("expected error for payment not found, got nil")
	}
	if err.Error() != "payment not found: payment not found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPaymentService_Verify_NoProofUploaded(t *testing.T) {
	tests := []struct {
		name     string
		proofURL *string
	}{
		{"Nil proof URL", nil},
		{"Empty proof URL", stringPtr("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payment := &model.Payment{
				PaymentID: 1,
				BookingID: 1,
				ProofURL:  tt.proofURL,
			}
			repo := &fakePaymentRepo{payment: payment}
			svc := NewPaymentService(repo)

			_, err := svc.Verify(context.Background(), 1, 100, nil)
			if err == nil {
				t.Fatal("expected error for no proof uploaded, got nil")
			}
			if err.Error() != "no proof uploaded for this payment" {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestPaymentService_Verify_RepoError(t *testing.T) {
	proofURL := "https://example.com/proof.jpg"
	payment := &model.Payment{
		PaymentID: 1,
		BookingID: 1,
		ProofURL:  &proofURL,
	}
	repo := &fakePaymentRepo{
		payment:   payment,
		updateErr: errors.New("verification failed"),
	}
	svc := NewPaymentService(repo)

	_, err := svc.Verify(context.Background(), 1, 100, nil)
	if err == nil {
		t.Fatal("expected error from Verify, got nil")
	}
	if err.Error() != "failed to verify payment: verification failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Tests for Reject ---

func TestPaymentService_Reject_Success(t *testing.T) {
	proofURL := "https://example.com/proof.jpg"
	payment := &model.Payment{
		PaymentID: 1,
		BookingID: 1,
		ProofURL:  &proofURL,
		Status:    "rejected",
	}
	repo := &fakePaymentRepo{payment: payment}
	svc := NewPaymentService(repo)

	notes := "Proof is unclear"
	result, err := svc.Reject(context.Background(), 1, 100, &notes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected payment, got nil")
	}
}

func TestPaymentService_Reject_PaymentNotFound(t *testing.T) {
	repo := &fakePaymentRepo{getErr: errors.New("payment not found")}
	svc := NewPaymentService(repo)

	_, err := svc.Reject(context.Background(), 999, 100, nil)
	if err == nil {
		t.Fatal("expected error for payment not found, got nil")
	}
	if err.Error() != "payment not found: payment not found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPaymentService_Reject_NoProofUploaded(t *testing.T) {
	payment := &model.Payment{
		PaymentID: 1,
		BookingID: 1,
		ProofURL:  nil,
	}
	repo := &fakePaymentRepo{payment: payment}
	svc := NewPaymentService(repo)

	_, err := svc.Reject(context.Background(), 1, 100, nil)
	if err == nil {
		t.Fatal("expected error for no proof uploaded, got nil")
	}
	if err.Error() != "no proof uploaded for this payment" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPaymentService_Reject_RepoError(t *testing.T) {
	proofURL := "https://example.com/proof.jpg"
	payment := &model.Payment{
		PaymentID: 1,
		BookingID: 1,
		ProofURL:  &proofURL,
	}
	repo := &fakePaymentRepo{
		payment:   payment,
		updateErr: errors.New("rejection failed"),
	}
	svc := NewPaymentService(repo)

	_, err := svc.Reject(context.Background(), 1, 100, nil)
	if err == nil {
		t.Fatal("expected error from Reject, got nil")
	}
	if err.Error() != "failed to reject payment: rejection failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Tests for ClearProof ---

func TestPaymentService_ClearProof_Success(t *testing.T) {
	payment := &model.Payment{
		PaymentID: 1,
		BookingID: 1,
	}
	repo := &fakePaymentRepo{payment: payment}
	svc := NewPaymentService(repo)

	err := svc.ClearProof(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentService_ClearProof_PaymentNotFound(t *testing.T) {
	repo := &fakePaymentRepo{getErr: errors.New("payment not found")}
	svc := NewPaymentService(repo)

	err := svc.ClearProof(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for payment not found, got nil")
	}
	if err.Error() != "payment not found: payment not found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPaymentService_ClearProof_RepoError(t *testing.T) {
	payment := &model.Payment{
		PaymentID: 1,
		BookingID: 1,
	}
	repo := &fakePaymentRepo{
		payment:   payment,
		updateErr: errors.New("clear failed"),
	}
	svc := NewPaymentService(repo)

	err := svc.ClearProof(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error from ClearProof, got nil")
	}
	if err.Error() != "failed to clear proof: clear failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
