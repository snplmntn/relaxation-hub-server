package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	repo      repository.BookingRepository
	promoRepo repository.PromotionRepository
	db        *pgxpool.Pool
}

func NewBookingService(repo repository.BookingRepository, promoRepo repository.PromotionRepository, db *pgxpool.Pool) *BookingService {
	return &BookingService{repo: repo, promoRepo: promoRepo, db: db}
}

func (s *BookingService) Create(ctx context.Context, clientID int64, req *model.CreateBookingRequest) (*model.Booking, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.TherapistID == 0 {
		return nil, fmt.Errorf("therapist_id is required")
	}
	// Default duration to 60 minutes (1 hour) when not provided or invalid.
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 60
	}
	// Enforce 30-minute increments.
	if req.DurationMinutes%30 != 0 {
		return nil, NewValidationError("invalid_duration", "duration_minutes must be in 30-minute increments", map[string]string{"duration_minutes": "must be a multiple of 30"})
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
	} else {
		// default to now when not provided
		now := time.Now()
		scheduled = &now
	}

	// Payment method validation (accepted values: cash, gcash, or empty)
	pm := strings.TrimSpace(strings.ToLower(req.PaymentMethod))
	if pm != "" && pm != "cash" && pm != "gcash" {
		return nil, NewValidationError("invalid_payment_method", "invalid payment_method: must be 'cash' or 'gcash'", map[string]string{"payment_method": "allowed values: cash, gcash"})
	}

	// We'll perform promo resolution and booking insertion inside a DB
	// transaction to ensure atomicity of promo redemptions.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var finalTotal *float64
	var discount *float64
	var promoID *int64

	if strings.TrimSpace(req.VoucherCode) != "" {
		// Resolve promo by code
		p, err := s.promoRepo.GetByCode(ctx, strings.TrimSpace(req.VoucherCode))
		if err != nil {
			return nil, NewValidationError("invalid_voucher", "invalid voucher code", map[string]string{"voucher_code": "not found or expired"})
		}
		// Validate time windows
		now := time.Now()
		if p.ValidFrom != nil && p.ValidFrom.After(now) {
			return nil, fmt.Errorf("voucher not yet active")
		}
		if p.ValidUntil != nil && p.ValidUntil.Before(now) {
			return nil, fmt.Errorf("voucher expired")
		}

		// Try increment global usage atomically
		ok, err := s.promoRepo.TryIncrementGlobalUsageTx(ctx, tx, p.PromoID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, NewValidationError("invalid_voucher", "voucher fully redeemed", map[string]string{"voucher_code": "redemption limit reached"})
		}

		// Track per-user usage
		if _, err := s.promoRepo.TryIncrementUserPromoUsageTx(ctx, tx, p.PromoID, clientID); err != nil {
			return nil, err
		}

		// Compute discount (percentage only; promotions currently store percent)
		if p.DiscountPct > 0 && req.RawTotal != nil {
			d := (*req.RawTotal) * float64(p.DiscountPct) / 100.0
			discount = &d
		}
		promoID = &p.PromoID
	}

	if req.Total != nil {
		finalTotal = req.Total
	} else {
		finalTotal = computeFinal(req.RawTotal, discount)
	}

	booking := &model.Booking{
		ClientID:        clientID,
		TherapistID:     req.TherapistID,
		ServiceID:       req.ServiceID,
		AddressID:       req.AddressID,
		PromoID:         promoID,
		GenderPref:      genderPref,
		PressurePref:    pressurePref,
		Notes:           strings.TrimSpace(req.Notes),
		DurationMinutes: req.DurationMinutes,
		ScheduledStart:  scheduled,
		RawTotal:        req.RawTotal,
		Discount:        discount,
		FinalTotal:      finalTotal,
		Status:          "pending",
	}

	if err := s.repo.CreateTx(ctx, tx, booking); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
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
			return nil, NewValidationError("invalid_duration", "duration_minutes must be positive", map[string]string{"duration_minutes": "must be > 0"})
		}
		if *req.DurationMinutes%30 != 0 {
			return nil, NewValidationError("invalid_duration", "duration_minutes must be in 30-minute increments", map[string]string{"duration_minutes": "must be multiple of 30"})
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

	// If a new total was provided, update final total
	if req.Total != nil {
		booking.FinalTotal = req.Total
	}

	if err := s.repo.Update(ctx, booking); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, bookingID, clientID)
}

// UpdateStatus updates a booking's status. The actorID and actorRole determine
// whether the caller is allowed to perform the requested transition.
func (s *BookingService) UpdateStatus(ctx context.Context, bookingID, actorID int64, actorRole string, req *model.UpdateBookingStatusRequest) (*model.Booking, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	status := strings.TrimSpace(req.Status)
	if _, ok := AllowedStatus[status]; !ok {
		return nil, errInvalidStatus
	}

	// Role-based permissions: admins can do any transition; therapists and
	// clients are limited.
	therapistAllowed := map[string]struct{}{"confirmed": {}, "in_progress": {}, "completed": {}}
	clientAllowed := map[string]struct{}{"cancelled": {}, "pending": {}}

	switch actorRole {
	case "admin":
		// admin may do everything
	case "therapist":
		if _, ok := therapistAllowed[status]; !ok {
			return nil, fmt.Errorf("therapist not allowed to set status: %s", status)
		}
	case "client":
		if _, ok := clientAllowed[status]; !ok {
			return nil, fmt.Errorf("client not allowed to set status: %s", status)
		}
	default:
		return nil, fmt.Errorf("unauthorized role")
	}

	if err := s.repo.UpdateStatus(ctx, bookingID, actorID, status); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, bookingID, actorID)
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
