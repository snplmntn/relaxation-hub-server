package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

var ErrLiveLocationAccessDenied = errors.New("live location access denied")

type bookingLocationAccessRepository interface {
	GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error)
}

const (
	liveLocationDecisionAllow    = "allow"
	liveLocationDecisionDeny     = "deny"
	liveLocationDecisionNotFound = "not_found"
	liveLocationDecisionError    = "error"
)

type BookingScopedLiveLocationResult struct {
	BookingID    int64
	TargetUserID int64
	Location     *model.LiveLocation
}

type LiveLocationService struct {
	repo        repository.LiveLocationRepository
	bookingRepo bookingLocationAccessRepository
	hub         *ws.Hub
}

func NewLiveLocationService(repo repository.LiveLocationRepository, bookingRepo bookingLocationAccessRepository, hub *ws.Hub) *LiveLocationService {
	return &LiveLocationService{repo: repo, bookingRepo: bookingRepo, hub: hub}
}

func (s *LiveLocationService) UpdateLocation(ctx context.Context, userID int64, req *model.UpdateLocationRequest) (*model.LiveLocation, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.Latitude < 5 || req.Latitude > 20 {
		return nil, fmt.Errorf("latitude out of PH range")
	}
	if req.Longitude < 116 || req.Longitude > 127 {
		return nil, fmt.Errorf("longitude out of PH range")
	}

	loc := &model.LiveLocation{
		UserID:    userID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	}

	if err := s.repo.Upsert(ctx, loc); err != nil {
		return nil, err
	}

	// Broadcast location update to subscribers (can be extended to notify specific users)
	// For now, we could notify all users tracking this therapist
	if s.hub != nil {
		s.hub.SendToUser(userID, "location_update", loc)
	}

	return loc, nil
}

func (s *LiveLocationService) GetByUserID(ctx context.Context, userID int64) (*model.LiveLocation, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *LiveLocationService) GetLocationForBooking(ctx context.Context, bookingID, requesterUserID int64) (*BookingScopedLiveLocationResult, error) {
	if s.bookingRepo == nil {
		return nil, fmt.Errorf("booking repository is required")
	}

	booking, err := s.bookingRepo.GetByBookingID(ctx, bookingID)
	if err != nil {
		s.logBookingScopedAccess(requesterUserID, 0, bookingID, liveLocationDecisionFromError(err))
		return nil, err
	}

	targetUserID, err := validateBookingLiveLocationAccess(booking, requesterUserID)
	if err != nil {
		s.logBookingScopedAccess(requesterUserID, targetUserID, bookingID, liveLocationDecisionDeny)
		return nil, ErrLiveLocationAccessDenied
	}

	loc, err := s.repo.GetByUserID(ctx, targetUserID)
	if err != nil {
		s.logBookingScopedAccess(requesterUserID, targetUserID, bookingID, liveLocationDecisionFromError(err))
		return nil, err
	}

	s.logBookingScopedAccess(requesterUserID, targetUserID, bookingID, liveLocationDecisionAllow)
	return &BookingScopedLiveLocationResult{
		BookingID:    bookingID,
		TargetUserID: targetUserID,
		Location:     loc,
	}, nil
}

func validateBookingLiveLocationAccess(booking *model.Booking, requesterUserID int64) (int64, error) {
	targetUserID, ok := deriveBookingLiveLocationTargetUserID(booking, requesterUserID)
	if !ok || !isBookingLiveLocationShareable(booking.Status) {
		return targetUserID, ErrLiveLocationAccessDenied
	}
	return targetUserID, nil
}

func deriveBookingLiveLocationTargetUserID(booking *model.Booking, requesterUserID int64) (int64, bool) {
	if booking == nil {
		return 0, false
	}

	if requesterUserID == booking.ClientID {
		if booking.TherapistID == nil {
			return 0, false
		}
		return *booking.TherapistID, true
	}

	if booking.TherapistID != nil && requesterUserID == *booking.TherapistID {
		return booking.ClientID, true
	}

	return 0, false
}

func isBookingLiveLocationShareable(status string) bool {
	switch status {
	case model.BookingStatusOnTheWay, model.BookingStatusArrived:
		return true
	default:
		return false
	}
}

func liveLocationDecisionFromError(err error) string {
	if errors.Is(err, pgx.ErrNoRows) {
		return liveLocationDecisionNotFound
	}
	return liveLocationDecisionError
}

func (s *LiveLocationService) logBookingScopedAccess(requesterUserID, targetUserID, bookingID int64, decision string) {
	slog.Info(
		"booking live location access",
		"requester_user_id", requesterUserID,
		"target_user_id", targetUserID,
		"booking_id", bookingID,
		"decision", decision,
	)
}
