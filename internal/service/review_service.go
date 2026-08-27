package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

var (
	ErrReviewExists      = errors.New("review already exists for this booking")
	ErrEditPeriodExpired = errors.New("review can only be edited within 24 hours")
)

// ReviewService handles review logic.
type ReviewService struct {
	repo                repository.ReviewRepository
	notificationService *NotificationService
	userRepo            repository.UserRepository
}

func NewReviewService(repo repository.ReviewRepository, notificationService *NotificationService, userRepo repository.UserRepository) *ReviewService {
	return &ReviewService{
		repo:                repo,
		notificationService: notificationService,
		userRepo:            userRepo,
	}
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

	if booking.Status != "completed" {
		return nil, fmt.Errorf("cannot review: booking status must be 'completed', got '%s'", booking.Status)
	}

	if booking.TherapistID == nil {
		return nil, fmt.Errorf("cannot review: booking has no assigned therapist")
	}

	rev := &model.Review{
		BookingID:       req.BookingID,
		ClientID:        clientID,
		TherapistID:     *booking.TherapistID,
		ServiceID:       booking.ServiceIDOrZero(),
		TherapistRating: req.TherapistRating,
		TherapistReview: req.TherapistReview,
		ServiceRating:   req.ServiceRating,
		ServiceReview:   req.ServiceReview,
		PlatformRating:  req.PlatformRating,
		PlatformReview:  req.PlatformReview,
	}

	if err := s.repo.Create(ctx, rev); err != nil {
		// Check for unique_violation (duplicate key)
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "unique constraint") {
			return nil, ErrReviewExists
		}
		return nil, fmt.Errorf("could not create review: %w", err)
	}

	// Notify Therapist
	if s.notificationService != nil {
		go func() {
			clientName := "A client"
			if s.userRepo != nil {
				infos, err := s.userRepo.GetUserInfoBatch(context.Background(), []int64{clientID})
				if err == nil && infos[clientID] != nil {
					clientName = infos[clientID].Name
				}
			}

			if _, err := s.notificationService.Create(context.WithoutCancel(ctx), &model.CreateNotificationRequest{
				UserID:  rev.TherapistID,
				Type:    "new_rating",
				Title:   fmt.Sprintf("New Rating: %d Stars!", req.TherapistRating),
				Message: fmt.Sprintf("%s left you a review.", clientName),
				Data: map[string]interface{}{
					"booking_id": rev.BookingID,
					"rating":     rev.TherapistRating,
				},
			}); err != nil {
				slog.Warn("review service: failed to notify therapist", "booking_id", rev.BookingID, "error", err)
			}
		}()
	}

	return rev, nil
}

func (s *ReviewService) Update(ctx context.Context, clientID int64, reviewID int64, req *model.CreateReviewRequest) (*model.Review, error) {
	// fetching review to check ownership and time
	// For update, we might need a Get method in repo or query by ID.
	// But Update usually implies we know the ID or booking ID.
	// The request has booking ID but we are targeting reviewID.

	// Wait, we need to fetch the existing review first.
	// Since we don't have GetByID (review_id), let's use GetByBookingID if req has bookingID,
	// OR we need to add GetByID to repo.

	// Actually, client will likely pass review_id in URL.
	// But to check 24h window, we need CreatedAt.
	// Let's rely on booking ID for now if we can, OR client passes review object.
	// For simplicity, let's assume we can fetch by BookingID from request.

	existing, err := s.repo.GetByBookingID(ctx, req.BookingID)
	if err != nil {
		return nil, fmt.Errorf("review not found: %w", err)
	}

	if existing.ClientID != clientID {
		return nil, fmt.Errorf("unauthorized")
	}

	if reviewID != 0 && existing.ReviewID != reviewID {
		return nil, fmt.Errorf("review id mismatch")
	}

	if time.Since(existing.CreatedAt) > 24*time.Hour {
		return nil, ErrEditPeriodExpired
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

	existing.TherapistRating = req.TherapistRating
	existing.TherapistReview = req.TherapistReview
	existing.ServiceRating = req.ServiceRating
	existing.ServiceReview = req.ServiceReview
	existing.PlatformRating = req.PlatformRating
	existing.PlatformReview = req.PlatformReview

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *ReviewService) ListByClient(ctx context.Context, clientID int64, limit, offset int) ([]model.Review, int, error) {
	return s.repo.ListByClient(ctx, clientID, limit, offset)
}

func (s *ReviewService) ListByClientWithDetails(ctx context.Context, clientID int64, limit, offset int) ([]repository.ReviewDetailsResult, int, error) {
	return s.repo.ListByClientWithDetails(ctx, clientID, limit, offset)
}

func (s *ReviewService) GetByBookingID(ctx context.Context, bookingID int64) (*model.Review, error) {
	return s.repo.GetByBookingID(ctx, bookingID)
}

func (s *ReviewService) ListByTherapist(ctx context.Context, therapistID int64, limit, offset int) ([]model.Review, int, error) {
	return s.repo.ListByTherapist(ctx, therapistID, limit, offset)
}

func (s *ReviewService) ListByTherapistWithDetails(ctx context.Context, therapistID int64, limit, offset int) ([]repository.ReviewDetailsResult, int, error) {
	return s.repo.ListByTherapistWithDetails(ctx, therapistID, limit, offset)
}

func (s *ReviewService) ListAllWithDetails(ctx context.Context, therapistID *int64, search string, minAvgRating float64, limit, offset int) ([]repository.ReviewDetailsResult, int, error) {
	return s.repo.ListAllWithDetails(ctx, therapistID, search, minAvgRating, limit, offset)
}
