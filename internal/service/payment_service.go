package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type PaymentService struct {
	repo repository.PaymentRepository
}

func NewPaymentService(repo repository.PaymentRepository) *PaymentService {
	return &PaymentService{repo: repo}
}

func (s *PaymentService) Create(ctx context.Context, req *model.CreatePaymentRequest) (*model.Payment, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.BookingID == 0 {
		return nil, fmt.Errorf("booking_id is required")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	gateway := strings.TrimSpace(req.Gateway)
	if gateway == "" {
		return nil, fmt.Errorf("gateway is required")
	}

	p := &model.Payment{
		BookingID: req.BookingID,
		Amount:    req.Amount,
		Gateway:   gateway,
		Status:    "pending",
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *PaymentService) GetByBookingID(ctx context.Context, bookingID int64) (*model.Payment, error) {
	return s.repo.GetByBookingID(ctx, bookingID)
}

func (s *PaymentService) UpdateStatus(ctx context.Context, bookingID int64, req *model.UpdatePaymentStatusRequest) (*model.Payment, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		return nil, fmt.Errorf("status is required")
	}

	if err := s.repo.UpdateStatus(ctx, bookingID, status, req.TransactionID, req.WebhookID); err != nil {
		return nil, err
	}
	return s.repo.GetByBookingID(ctx, bookingID)
}

// UploadProof uploads a payment proof URL to the payment record.
// If no payment record exists, it creates one.
func (s *PaymentService) UploadProof(ctx context.Context, bookingID int64, proofURL string, amount float64, gateway string) (*model.Payment, error) {
	// Ensure payment record exists
	_, err := s.repo.GetOrCreateByBookingID(ctx, bookingID, amount, gateway)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create payment: %w", err)
	}

	// Update proof URL
	if err := s.repo.UpdateProofURL(ctx, bookingID, proofURL); err != nil {
		return nil, fmt.Errorf("failed to update proof URL: %w", err)
	}

	return s.repo.GetByBookingID(ctx, bookingID)
}

// Verify marks the payment as verified by a therapist/admin.
func (s *PaymentService) Verify(ctx context.Context, bookingID int64, verifiedBy int64) (*model.Payment, error) {
	// Check payment exists
	p, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	// Check proof exists
	if p.ProofURL == nil || *p.ProofURL == "" {
		return nil, fmt.Errorf("no proof uploaded for this payment")
	}

	// Mark as verified
	if err := s.repo.Verify(ctx, bookingID, verifiedBy); err != nil {
		return nil, fmt.Errorf("failed to verify payment: %w", err)
	}

	return s.repo.GetByBookingID(ctx, bookingID)
}

