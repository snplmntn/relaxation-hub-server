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
	if f.getErr != nil { return nil, f.getErr } // Reuse getErr or add new one
	if f.payment != nil { return f.payment, nil }
	return &model.Payment{PaymentID: 1, BookingID: bookingID, Amount: amount, Gateway: gateway, Status: "pending"}, nil
}
func (f *fakePaymentRepo) UpdateProofURL(ctx context.Context, bookingID int64, proofURL string) error { return f.updateErr }
func (f *fakePaymentRepo) Verify(ctx context.Context, bookingID int64, verifiedBy int64) error { return f.updateErr }

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
