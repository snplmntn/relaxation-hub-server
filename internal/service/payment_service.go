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
