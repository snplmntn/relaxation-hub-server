package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

var (
    errInvalidStatus = errors.New("invalid booking status")
)

// AllowedStatus enumerates acceptable booking statuses.
var AllowedStatus = map[string]struct{}{
    "pending":     {},
    "confirmed":   {},
    "in_progress": {},
    "completed":   {},
    "cancelled":   {},
}

type BookingService struct {
    repo repository.BookingRepository
}

func NewBookingService(repo repository.BookingRepository) *BookingService {
    return &BookingService{repo: repo}
}

func (s *BookingService) Create(ctx context.Context, clientID int64, req *model.CreateBookingRequest) (*model.Booking, error) {
    if req == nil {
        return nil, fmt.Errorf("request is required")
    }
    if req.TherapistID == 0 {
        return nil, fmt.Errorf("therapist_id is required")
    }
    if req.DurationMinutes <= 0 {
        return nil, fmt.Errorf("duration_minutes must be positive")
    }

    genderPref := strings.TrimSpace(req.GenderPref)
    pressurePref := strings.TrimSpace(req.PressurePref)

    var scheduled *time.Time
    if req.ScheduledStart != "" {
        t, err := time.Parse(time.RFC3339, req.ScheduledStart)
        if err != nil {
            return nil, fmt.Errorf("invalid scheduled_start: %w", err)
        }
        scheduled = &t
    }

    finalTotal := computeFinal(req.RawTotal, req.Discount)

    booking := &model.Booking{
        ClientID:        clientID,
        TherapistID:     req.TherapistID,
        ServiceID:       req.ServiceID,
        AddressID:       req.AddressID,
        PromoID:         req.PromoID,
        GenderPref:      genderPref,
        PressurePref:    pressurePref,
        Notes:           strings.TrimSpace(req.Notes),
        DurationMinutes: req.DurationMinutes,
        ScheduledStart:  scheduled,
        RawTotal:        req.RawTotal,
        Discount:        req.Discount,
        FinalTotal:      finalTotal,
        Status:          "pending",
    }

    if err := s.repo.Create(ctx, booking); err != nil {
        return nil, err
    }
    return booking, nil
}

func (s *BookingService) GetByID(ctx context.Context, bookingID, clientID int64) (*model.Booking, error) {
    return s.repo.GetByID(ctx, bookingID, clientID)
}

func (s *BookingService) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
    return s.repo.ListByClient(ctx, clientID)
}

func (s *BookingService) Update(ctx context.Context, bookingID, clientID int64, req *model.UpdateBookingRequest) (*model.Booking, error) {
    if req == nil {
        return nil, fmt.Errorf("request is required")
    }

    booking, err := s.repo.GetByID(ctx, bookingID, clientID)
    if err != nil {
        return nil, err
    }

    if req.ServiceID != nil {
        booking.ServiceID = req.ServiceID
    }
    if req.AddressID != nil {
        booking.AddressID = req.AddressID
    }
    if req.PromoID != nil {
        booking.PromoID = req.PromoID
    }
    if req.GenderPref != nil {
        booking.GenderPref = strings.TrimSpace(*req.GenderPref)
    }
    if req.PressurePref != nil {
        booking.PressurePref = strings.TrimSpace(*req.PressurePref)
    }
    if req.Notes != nil {
        booking.Notes = strings.TrimSpace(*req.Notes)
    }
    if req.DurationMinutes != nil {
        if *req.DurationMinutes <= 0 {
            return nil, fmt.Errorf("duration_minutes must be positive")
        }
        booking.DurationMinutes = *req.DurationMinutes
    }
    if req.ScheduledStart != nil {
        if *req.ScheduledStart == "" {
            booking.ScheduledStart = nil
        } else {
            t, err := time.Parse(time.RFC3339, *req.ScheduledStart)
            if err != nil {
                return nil, fmt.Errorf("invalid scheduled_start: %w", err)
            }
            booking.ScheduledStart = &t
        }
    }

    if err := s.repo.Update(ctx, booking); err != nil {
        return nil, err
    }
    return s.repo.GetByID(ctx, bookingID, clientID)
}

func (s *BookingService) UpdateStatus(ctx context.Context, bookingID, clientID int64, req *model.UpdateBookingStatusRequest) (*model.Booking, error) {
    if req == nil {
        return nil, fmt.Errorf("request is required")
    }
    status := strings.TrimSpace(req.Status)
    if _, ok := AllowedStatus[status]; !ok {
        return nil, errInvalidStatus
    }
    if err := s.repo.UpdateStatus(ctx, bookingID, clientID, status); err != nil {
        return nil, err
    }
    return s.repo.GetByID(ctx, bookingID, clientID)
}

func computeFinal(raw, discount *float64) *float64 {
    if raw == nil {
        return nil
    }
    d := 0.0
    if discount != nil {
        d = *discount
    }
    v := *raw - d
    return &v
}
