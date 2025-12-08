package service

import (
	"context"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// ReviewService handles review logic.
type ReviewService struct {
    repo repository.ReviewRepository
}

func NewReviewService(repo repository.ReviewRepository) *ReviewService {
    return &ReviewService{repo: repo}
}

func (s *ReviewService) Create(ctx context.Context, clientID int64, req *model.CreateReviewRequest, booking *model.Booking) (*model.Review, error) {
    if req == nil {
        return nil, fmt.Errorf("request is required")
    }
    if req.BookingID == 0 {
        return nil, fmt.Errorf("booking_id is required")
    }

    validateScore := func(v int, name string) error {
        if v < 1 || v > 5 {
            return fmt.Errorf("%s must be 1-5", name)
        }
        return nil
    }

    if err := validateScore(req.TherapistRating, "therapist_rating"); err != nil {
        return nil, err
    }
    if err := validateScore(req.ServiceRating, "service_rating"); err != nil {
        return nil, err
    }
    if err := validateScore(req.PlatformRating, "platform_rating"); err != nil {
        return nil, err
    }

    rev := &model.Review{
        BookingID:       req.BookingID,
        ClientID:        clientID,
        TherapistID:     booking.TherapistID,
        ServiceID:       booking.ServiceIDOrZero(),
        TherapistRating: req.TherapistRating,
        TherapistReview: req.TherapistReview,
        ServiceRating:   req.ServiceRating,
        ServiceReview:   req.ServiceReview,
        PlatformRating:  req.PlatformRating,
        PlatformReview:  req.PlatformReview,
    }

    if err := s.repo.Create(ctx, rev); err != nil {
        return nil, err
    }
    return rev, nil
}

func (s *ReviewService) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Review, error) {
    return s.repo.ListByTherapist(ctx, therapistID)
}
