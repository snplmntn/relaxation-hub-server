package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type BookingAnnouncementService struct {
	repo repository.BookingAnnouncementRepository
}

func NewBookingAnnouncementService(repo repository.BookingAnnouncementRepository) *BookingAnnouncementService {
	return &BookingAnnouncementService{repo: repo}
}

func (s *BookingAnnouncementService) List(ctx context.Context) ([]model.BookingAnnouncement, error) {
	return s.repo.List(ctx)
}

func (s *BookingAnnouncementService) Create(ctx context.Context, req model.BookingAnnouncementRequest) (*model.BookingAnnouncement, error) {
	announcement, err := validateBookingAnnouncement(req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, announcement); err != nil {
		return nil, err
	}
	return announcement, nil
}

func (s *BookingAnnouncementService) Update(ctx context.Context, announcementID int64, req model.BookingAnnouncementRequest) (*model.BookingAnnouncement, error) {
	announcement, err := validateBookingAnnouncement(req)
	if err != nil {
		return nil, err
	}
	announcement.AnnouncementID = announcementID
	if err := s.repo.Update(ctx, announcement); err != nil {
		if errors.Is(err, repository.ErrWinningVariationRequired) {
			return nil, NewValidationError("invalid_announcement", "Choose another winner before removing the current winner.", map[string]string{"variations": "winner required"})
		}
		if errors.Is(err, repository.ErrVariationNotInCampaign) {
			return nil, NewValidationError("invalid_announcement", "One or more announcement versions are invalid.", map[string]string{"variations": "invalid"})
		}
		return nil, err
	}
	return announcement, nil
}

func (s *BookingAnnouncementService) Delete(ctx context.Context, announcementID int64) error {
	return s.repo.Delete(ctx, announcementID)
}

func (s *BookingAnnouncementService) ChooseWinner(ctx context.Context, announcementID, variationID int64) (*model.BookingAnnouncement, error) {
	if announcementID <= 0 || variationID <= 0 {
		return nil, NewValidationError("invalid_announcement_winner", "Choose a valid announcement version.", map[string]string{"variation_id": "invalid"})
	}
	announcement, err := s.repo.ChooseWinner(ctx, announcementID, variationID)
	if errors.Is(err, repository.ErrVariationNotInCampaign) {
		return nil, NewValidationError("invalid_announcement_winner", "That version does not belong to this announcement.", map[string]string{"variation_id": "invalid"})
	}
	return announcement, err
}

func (s *BookingAnnouncementService) ResolveForBooking(ctx context.Context, bookingID int64) ([]model.ResolvedBookingAnnouncement, error) {
	if bookingID <= 0 {
		return nil, NewValidationError("invalid_booking", "Choose a valid booking.", map[string]string{"booking_id": "invalid"})
	}
	return s.repo.ResolveForBooking(ctx, bookingID)
}

func validateBookingAnnouncement(req model.BookingAnnouncementRequest) (*model.BookingAnnouncement, error) {
	if len(req.Variations) == 0 {
		return nil, NewValidationError("invalid_announcement", "Add at least one announcement version.", map[string]string{"variations": "required"})
	}

	variations := make([]model.BookingAnnouncementVariation, len(req.Variations))
	for i, input := range req.Variations {
		message := strings.TrimSpace(input.Message)
		if message == "" {
			return nil, NewValidationError("invalid_announcement", "Announcement versions cannot be blank.", map[string]string{"variations": "blank"})
		}
		variations[i] = model.BookingAnnouncementVariation{Message: message, Position: i + 1}
		if input.VariationID != nil {
			variations[i].VariationID = *input.VariationID
		}
	}

	startDate, err := time.Parse(time.DateOnly, req.StartDate)
	if err != nil {
		return nil, NewValidationError("invalid_announcement", "Enter a valid start date.", map[string]string{"start_date": "invalid"})
	}
	endDate, err := time.Parse(time.DateOnly, req.EndDate)
	if err != nil {
		return nil, NewValidationError("invalid_announcement", "Enter a valid end date.", map[string]string{"end_date": "invalid"})
	}
	if endDate.Before(startDate) {
		return nil, NewValidationError("invalid_announcement", "End date must be on or after start date.", map[string]string{"end_date": "before start date"})
	}

	return &model.BookingAnnouncement{
		Message:    variations[0].Message,
		StartDate:  startDate,
		EndDate:    endDate,
		Variations: variations,
	}, nil
}

func chooseBookingAnnouncementVariation(announcement model.BookingAnnouncement, subjectID int64) model.BookingAnnouncementVariation {
	return model.ChooseBookingAnnouncementVariation(announcement, subjectID)
}
