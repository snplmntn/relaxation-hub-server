package service

import (
	"context"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// ClientReviewService handles therapist-authored ratings of clients.
type ClientReviewService struct {
	repo repository.ClientReviewRepository
}

func NewClientReviewService(repo repository.ClientReviewRepository) *ClientReviewService {
	return &ClientReviewService{repo: repo}
}

func (s *ClientReviewService) Create(ctx context.Context, therapistID int64, req *model.CreateClientReviewRequest, booking *model.Booking) (*model.ClientReview, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.BookingID == 0 {
		return nil, fmt.Errorf("booking_id is required")
	}
	if req.ClientRating < 1 || req.ClientRating > 5 {
		return nil, fmt.Errorf("client_rating must be 1-5")
	}
	if booking == nil {
		return nil, fmt.Errorf("booking is required")
	}
	if booking.ClientID == 0 {
		return nil, fmt.Errorf("booking missing client")
	}
	if booking.TherapistID == nil || *booking.TherapistID != therapistID {
		return nil, fmt.Errorf("not authorized to review this booking")
	}
	if booking.Status != "completed" {
		return nil, fmt.Errorf("booking must be completed before leaving a review")
	}

	existing, err := s.repo.FindByBookingAndTherapist(ctx, req.BookingID, therapistID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("client already reviewed for this booking")
	}

	review := &model.ClientReview{
		BookingID:    req.BookingID,
		TherapistID:  therapistID,
		ClientID:     booking.ClientID,
		ClientRating: req.ClientRating,
		ClientReview: req.ClientReview,
	}

	if err := s.repo.Create(ctx, review); err != nil {
		return nil, err
	}

	return review, nil
}

func (s *ClientReviewService) ListByClient(ctx context.Context, clientID int64) ([]model.ClientReview, error) {
	if clientID == 0 {
		return nil, fmt.Errorf("client_id is required")
	}
	return s.repo.ListByClient(ctx, clientID)
}
