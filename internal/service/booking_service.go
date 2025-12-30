package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/socketio"
)

var (
	errInvalidStatus = errors.New("invalid booking status")
)

// AllowedStatus enumerates acceptable booking statuses.
var AllowedStatus = map[string]struct{}{
	"pending":     {},
	"assigned":    {},
	"on_the_way":  {},
	"arrived":     {},
	"in_progress": {},
	"completed":   {},
	"cancelled":   {},
	"no_show":     {},
	"rescheduled": {},
}

type BookingService struct {
	repo      repository.BookingRepository
	promoRepo repository.PromotionRepository
	serviceRepo repository.ServiceRepository
	addressRepo repository.AddressRepository
	db        *pgxpool.Pool
	queueRepo repository.AssignmentQueueRepository
	therapistRepo repository.TherapistRepository
	offerRepo repository.BookingOfferRepository
	messageService *MessageService // for auto-creating conversations on assignment
	notificationService *NotificationService
}

func NewBookingService(repo repository.BookingRepository, promoRepo repository.PromotionRepository, db *pgxpool.Pool, qr repository.AssignmentQueueRepository, tr repository.TherapistRepository, or repository.BookingOfferRepository, sr repository.ServiceRepository, ar repository.AddressRepository, ms *MessageService, ns *NotificationService) *BookingService {
	return &BookingService{repo: repo, promoRepo: promoRepo, db: db, queueRepo: qr, therapistRepo: tr, offerRepo: or, serviceRepo: sr, addressRepo: ar, messageService: ms, notificationService: ns}
}

// ListOffersForTherapist returns current active pending offers targeted to a therapist.
func (s *BookingService) ListOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) {
	if s.offerRepo == nil {
		return nil, nil
	}
	return s.offerRepo.GetActiveOffersForTherapist(ctx, therapistID)
}

func (s *BookingService) Create(ctx context.Context, clientID int64, req *model.CreateBookingRequest, actorID *int64) (*model.Booking, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	// TherapistID is optional for uber-style matching. If omitted, booking
	// will be created without an assigned therapist and enqueued for matching.
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
	var tx pgx.Tx
	if s.db != nil {
		var err error
		tx, err = s.db.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer func() { _ = tx.Rollback(ctx) }()
	} else {
		tx = pgx.Tx(nil)
	}

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

		// Compute discount
		if p.DiscountAmount != nil && *p.DiscountAmount > 0 {
			d := *p.DiscountAmount
			discount = &d
		} else if p.DiscountPct > 0 && req.RawTotal != nil {
			d := (*req.RawTotal) * float64(p.DiscountPct) / 100.0
			discount = &d
		}
		
		// Cap discount at total
		if discount != nil && req.RawTotal != nil && *discount > *req.RawTotal {
			d := *req.RawTotal
			discount = &d
		}
		promoID = &p.PromoID
	}

	if req.Total != nil {
		finalTotal = req.Total
	} else {
		finalTotal = computeFinal(req.RawTotal, discount)
	}

	// Create booking record without persisting therapist_id so that any
	// subsequent assignment goes through the guarded repository methods and
	// records the proper assignment events/actor.
	code := generateReferenceCode(*scheduled)
	booking := &model.Booking{
		ClientID:        clientID,
		TherapistID:     nil,
		ServiceID:       req.ServiceID,
		AddressID:       req.AddressID,
		PromoID:         promoID,
		PaymentMethod:   strings.TrimSpace(pm),
		GenderPref:      genderPref,
		PressurePref:    pressurePref,
		Notes:           strings.TrimSpace(req.Notes),
		DurationMinutes: req.DurationMinutes,
		ScheduledStart:  scheduled,
		RawTotal:        req.RawTotal,
		Discount:        discount,
		FinalTotal:      finalTotal,
		Status:          "pending",
		ReferenceCode:   &code,
	}

	if err := s.repo.CreateTx(ctx, tx, booking); err != nil {
		return nil, err
	}

	if s.db != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}

	// Record booking created event (actor is client unless actorID provided)
	actor := clientID
	if actorID != nil {
		actor = *actorID
	}
	_ = s.repo.InsertEvent(ctx, booking.BookingID, "created", &actor, nil)

	// Broadcast booking created to the client and (if assigned) therapist so
	// connected frontends receive the new booking immediately.
	_ = socketio.BroadcastToUser(booking.ClientID, "booking:created", booking)
	if booking.TherapistID != nil {
		_ = socketio.BroadcastToUser(*booking.TherapistID, "booking:created", booking)
	}

	// If no therapist was assigned, enqueue the booking for background matching.
	if booking.TherapistID == nil {
		// Offer-to-therapists-first: try to create short-lived offers to top candidates
		// Default to up to 3 candidates and 30m TTL (to allow 5m wait before expansion)
		const offerCandidates = 3
		const offerTTL = time.Minute * 30

		var candidates []model.TherapistProfile
		if req.TherapistID != nil {
			// If specific therapist requested, offer to them only
			candidates = []model.TherapistProfile{{TherapistID: *req.TherapistID}}
		} else if booking.ServiceID != nil {
			cands, err := s.therapistRepo.FindAvailableByService(ctx, booking.ClientID, *booking.ServiceID, booking.GenderPref, booking.PressurePref)
			if err == nil {
				candidates = cands
			}
		}

		if len(candidates) > 0 {
			count := offerCandidates
			if len(candidates) < count {
				count = len(candidates)
			}
			now := time.Now()
			for i := 0; i < count; i++ {
				cand := candidates[i]
				o := &model.BookingOffer{
					BookingID:   booking.BookingID,
					TherapistID: cand.TherapistID,
					Status:      model.BookingOfferStatusPending,
					CreatedAt:   now,
					ExpiresAt:   now.Add(offerTTL),
				}
				if err := s.offerRepo.Create(ctx, o); err != nil {
					// best-effort: if offer creation fails continue to next candidate
					continue
				}
				log.Printf("booking service: OFFER MADE: BookingID=%d, TherapistID=%d, OfferID=%d", booking.BookingID, cand.TherapistID, o.OfferID)
				
				// Fetch enriched booking data for socket broadcast (service and address)
				var svc *model.Service
				if booking.ServiceID != nil && s.serviceRepo != nil {
					if service, err := s.serviceRepo.GetByID(ctx, *booking.ServiceID); err == nil {
						svc = service
					}
				}
				var addr *model.Address
				if booking.AddressID != nil && s.addressRepo != nil {
					if address, err := s.addressRepo.GetByIDUnsafe(ctx, *booking.AddressID); err == nil {
						addr = address
					}
				}

				// Fetch client details for socket broadcast
				var clientName, clientPhone, clientPhoto, clientGender string
				if s.db != nil {
					userQuery := `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
					_ = s.db.QueryRow(ctx, userQuery, booking.ClientID).Scan(&clientName, &clientPhone, &clientPhoto, &clientGender)
				}

				// Create enriched payload for socket event (includes full booking details)
				socketPayload := map[string]any{
					"offer_id":           o.OfferID,
					"target_therapist_id": o.TherapistID,
					"expires_at":         o.ExpiresAt.Format(time.RFC3339),
					"booking_id":         booking.BookingID,
					"offer": map[string]any{
						"offer_id":     o.OfferID,
						"booking_id":   o.BookingID,
						"therapist_id": o.TherapistID,
						"status":       string(o.Status),
						"created_at":   o.CreatedAt.Format(time.RFC3339),
						"expires_at":   o.ExpiresAt.Format(time.RFC3339),
					},
					"booking": bookingToMap(booking, svc, addr, clientName, clientPhone, clientPhoto, clientGender),
				}

				// Keep minimal metadata for event log (database storage)
				eventMeta := map[string]any{
					"offer_id":           o.OfferID,
					"target_therapist_id": o.TherapistID,
					"expires_at":         o.ExpiresAt.Format(time.RFC3339),
					"booking_id":         booking.BookingID,
				}
				_ = s.repo.InsertEvent(ctx, booking.BookingID, "offered_to_therapist", nil, eventMeta)
				
				// Notify therapist in real-time via socket.io with enriched data (best-effort)
				go func(tid int64, payload map[string]any) {
					// ignore errors; this is best-effort
					_ = socketio.BroadcastToUser(tid, "offered_to_therapist", payload)
				}(o.TherapistID, socketPayload)
			}
			// Enqueue so the worker monitors the offers and handles expansion/expiration
		}
		_ = s.queueRepo.Enqueue(ctx, booking.BookingID)
	}

	return booking, nil
}

// CreateForAdmin creates a booking on behalf of a client and records an
// admin_created_booking event for audit/timeline.
func (s *BookingService) CreateForAdmin(ctx context.Context, adminID, clientID int64, req *model.CreateBookingRequest) (*model.Booking, error) {
	// If no therapist specified, behave like regular Create and record event.
	if req == nil || req.TherapistID == nil {
		b, err := s.Create(ctx, clientID, req, &adminID)
		if err != nil {
			return nil, err
		}
		// best-effort: record admin-created event
		actor := adminID
		_ = s.repo.InsertEvent(ctx, b.BookingID, "admin_created_booking", &actor, nil)
		return b, nil
	}

	// Admin provided a therapist: perform create+assign atomically and
	// validate assignment. Start a transaction so we can rollback on failure
	// and return a clear validation error to the caller.
	var tx pgx.Tx
	if s.db != nil {
		var err error
		tx, err = s.db.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer func() { _ = tx.Rollback(ctx) }()
	} else {
		// For unit tests s.db may be nil; use a nil pgx.Tx that mocks can accept.
		tx = pgx.Tx(nil)
	}

	// Copy validation from Create for duration and schedules
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 60
	}
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
		now := time.Now()
		scheduled = &now
	}

	code := generateReferenceCode(*scheduled)

	// Payment method validation
	pm := strings.TrimSpace(strings.ToLower(req.PaymentMethod))
	if pm != "" && pm != "cash" && pm != "gcash" {
		return nil, NewValidationError("invalid_payment_method", "invalid payment_method: must be 'cash' or 'gcash'", map[string]string{"payment_method": "allowed values: cash, gcash"})
	}

	booking := &model.Booking{
		ClientID:        clientID,
		TherapistID:     nil,
		ServiceID:       req.ServiceID,
		AddressID:       req.AddressID,
		PromoID:         nil,
		PaymentMethod:   strings.TrimSpace(pm),
		GenderPref:      genderPref,
		PressurePref:    pressurePref,
		Notes:           strings.TrimSpace(req.Notes),
		DurationMinutes: req.DurationMinutes,
		ScheduledStart:  scheduled,
		RawTotal:        req.RawTotal,
		Discount:        req.Discount,
		FinalTotal:      req.Total,
		Status:          "pending",
		ReferenceCode:   &code,
	}

	if err := s.repo.CreateTx(ctx, tx, booking); err != nil {
		return nil, err
	}

	// Validate therapist existence and acceptance of assignments
	tp, terr := s.therapistRepo.GetProfile(ctx, *req.TherapistID)
	if terr != nil {
		if terr == pgx.ErrNoRows {
			return nil, NewValidationError("invalid_therapist", "specified therapist not found", map[string]string{"therapist_id": "not found"})
		}
		return nil, terr
	}
	if !tp.AcceptAssignments {
		return nil, NewValidationError("therapist_not_accepting", "therapist is not accepting assignments", map[string]string{"therapist_id": "accept_assignments = false"})
	}

	// Attempt the guarded assign within tx using repository support
	if err := s.repo.AssignTherapistWithActorTx(ctx, tx, booking.BookingID, *req.TherapistID, adminID); err != nil {
		switch err {
		case repository.ErrTherapistNotFound:
			return nil, NewValidationError("invalid_therapist", "specified therapist not found", map[string]string{"therapist_id": "not found"})
		case repository.ErrTherapistNotAccepting:
			return nil, NewValidationError("therapist_not_accepting", "therapist is not accepting assignments", map[string]string{"therapist_id": "accept_assignments = false"})
		case repository.ErrAlreadyAssigned:
			return nil, NewValidationError("cannot_assign", "therapist already assigned", map[string]string{"therapist_id": "already assigned"})
		case repository.ErrBookingNotAssignable:
			return nil, NewValidationError("cannot_assign", "booking not in assignable state (status/payment)", map[string]string{"booking_id": "not assignable"})
		case repository.ErrAssignConflict:
			return nil, NewValidationError("cannot_assign", "assignment failed due to concurrent change", map[string]string{"therapist_id": "race"})
		case pgx.ErrNoRows:
			return nil, NewValidationError("cannot_assign", "therapist could not be assigned to booking", map[string]string{"therapist_id": "failed gating or already assigned"})
		default:
			return nil, err
		}
	}

	// commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// record admin-created booking event (and assigned event already recorded inside tx)
	actor := adminID
	_ = s.repo.InsertEvent(ctx, booking.BookingID, "admin_created_booking", &actor, nil)

	// reload booking to ensure assigned fields are present
	nb, err := s.repo.GetByBookingID(ctx, booking.BookingID)
	if err != nil {
		return nil, err
	}
	return nb, nil
}

func (s *BookingService) GetByID(ctx context.Context, bookingID, clientID int64) (*model.Booking, error) {
	return s.repo.GetByID(ctx, bookingID, clientID)
}

// GetBookingWithTimeline returns booking and its timeline events for client viewing
// Optimized to use a single query with JOINs for all related data
// Returns: booking, events, service, address, therapistName, therapistPhone, therapistPhoto, therapistGender, therapistRating, clientName, clientPhone, clientPhoto, clientGender, promoCode, error
func (s *BookingService) GetBookingWithTimeline(ctx context.Context, bookingID, clientID int64) (*model.Booking, []model.BookingEvent, *model.Service, *model.Address, string, string, string, string, *float64, string, string, string, string, string, error) {
	// Try optimized query first (works if user is client or therapist)
	details, err := s.repo.GetBookingWithDetails(ctx, bookingID, clientID)
	if err == nil {
		// Successfully fetched with optimized query - fetch events separately
		events, err := s.repo.ListEvents(ctx, bookingID)
		if err != nil {
			// If events fail, we can still return booking
			log.Printf("ListEvents failed for booking %d: %v", bookingID, err)
		}
		return details.Booking, events, details.Service, details.Address, details.TherapistName, details.TherapistPhone, details.TherapistPhoto, details.TherapistGender, details.TherapistRating, details.ClientName, details.ClientPhone, details.ClientPhoto, details.ClientGender, details.PromoCode, nil
	}

	// If optimized query failed (user not client or therapist), check if user has pending offer
	if err == pgx.ErrNoRows && s.offerRepo != nil {
		offer, _ := s.offerRepo.GetByTherapistAndBooking(ctx, clientID, bookingID)
		if offer != nil && offer.Status == model.BookingOfferStatusPending && offer.ExpiresAt.After(time.Now()) {
			// User has active offer, fetch booking without user scoping
			details, err := s.repo.GetBookingWithDetailsUnsafe(ctx, bookingID)
			if err == nil {
				events, err := s.repo.ListEvents(ctx, bookingID)
				if err != nil {
					log.Printf("ListEvents failed for booking %d: %v", bookingID, err)
				}
				return details.Booking, events, details.Service, details.Address, details.TherapistName, details.TherapistPhone, details.TherapistPhoto, details.TherapistGender, details.TherapistRating, details.ClientName, details.ClientPhone, details.ClientPhoto, details.ClientGender, details.PromoCode, nil
			}
		}
	}

	// Fallback to original error
	return nil, nil, nil, nil, "", "", "", "", nil, "", "", "", "", "", err
}

func (s *BookingService) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return s.repo.ListByClient(ctx, clientID)
}

func (s *BookingService) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return s.repo.ListByTherapist(ctx, therapistID)
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
	therapistAllowed := map[string]struct{}{"on_the_way": {}, "arrived": {}, "in_progress": {}, "completed": {}}
	clientAllowed := map[string]struct{}{"cancelled": {}, "pending": {}}

	switch actorRole {
	case "admin":
		// admin may do everything
	case "therapist":
		log.Printf("DEBUG: UpdateStatus: actorID=%d, checking therapistAllowed for status=%s", actorID, status)
		if _, ok := therapistAllowed[status]; !ok {
			return nil, fmt.Errorf("therapist not allowed to set status: %s", status)
		}
	case "client":
		log.Printf("DEBUG: UpdateStatus: actorID=%d, checking clientAllowed for status=%s", actorID, status)
		if _, ok := clientAllowed[status]; !ok {
			return nil, fmt.Errorf("client not allowed to set status: %s", status)
		}
	default:
		return nil, fmt.Errorf("unauthorized role")
	}

	var cancelledBy *string
	var cancellationReason *string
	if status == "cancelled" {
		cancelledBy = &actorRole
		cancellationReason = req.CancellationReason
	}

	if err := s.repo.UpdateStatus(ctx, bookingID, actorID, status, cancelledBy, cancellationReason); err != nil {
		return nil, err
	}


	// Broadcast updated booking to client and therapist so connected clients
	// see status changes in realtime (e.g. accepted, arrived, completed).
	if b, err := s.repo.GetByBookingID(ctx, bookingID); err == nil && b != nil {
		// Fetch related data for enriched payload
		var service *model.Service
		if b.ServiceID != nil && s.serviceRepo != nil {
			if svc, err := s.serviceRepo.GetByID(ctx, *b.ServiceID); err == nil {
				service = svc
			}
		}
		var address *model.Address
		if b.AddressID != nil && s.addressRepo != nil {
			if addr, err := s.addressRepo.GetByIDUnsafe(ctx, *b.AddressID); err == nil {
				address = addr
			}
		}
		var therapist *model.TherapistProfile
		var therapistName, therapistPhone, therapistGender string
		if b.TherapistID != nil {
			if s.therapistRepo != nil {
				if prof, err := s.therapistRepo.GetProfile(ctx, *b.TherapistID); err == nil {
					therapist = prof
				}
			}
			if s.db != nil {
				// Fetch therapist name, phone, and gender from users table
				var userQuery = `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
				_ = s.db.QueryRow(ctx, userQuery, *b.TherapistID).Scan(&therapistName, &therapistPhone, &therapistGender)
			}
		}

		// Fetch client details
		var clientName, clientPhone, clientPhoto, clientGender string
		if s.db != nil {
			clientQuery := `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
			_ = s.db.QueryRow(ctx, clientQuery, b.ClientID).Scan(&clientName, &clientPhone, &clientPhoto, &clientGender)
		}
		
		enrichedPayload := bookingToMapWithTherapist(b, service, address, therapist, therapistName, therapistPhone, clientName, clientPhone, clientPhoto, clientGender, therapistGender)
		
		// Send persistent notification
		s.sendBookingNotification(ctx, b, status, actorRole, therapistName)
		
		if err := socketio.BroadcastToUser(b.ClientID, "booking:updated", enrichedPayload); err != nil {
			log.Printf("UpdateStatus: Failed to broadcast to client %d: %v", b.ClientID, err)
		} else {
			log.Printf("UpdateStatus: Broadcasted booking:updated to client %d", b.ClientID)
		}

		if b.TherapistID != nil {
			if err := socketio.BroadcastToUser(*b.TherapistID, "booking:updated", enrichedPayload); err != nil {
				log.Printf("UpdateStatus: Failed to broadcast to therapist %d: %v", *b.TherapistID, err)
			} else {
				log.Printf("UpdateStatus: Broadcasted booking:updated to therapist %d", *b.TherapistID)
			}
		}
	}
	// Return booking scoped appropriately: clients should only see their own
	// booking via GetByID, whereas therapists and admins may fetch without
	// client scoping using GetByBookingID.
	if actorRole == "client" {
		return s.repo.GetByID(ctx, bookingID, actorID)
	}
	return s.repo.GetByBookingID(ctx, bookingID)
}

// AssignTherapist allows administrative or worker-driven assignment of a
// therapist to a booking. It will attempt a conditional update and return the
// updated booking.
func (s *BookingService) AssignTherapist(ctx context.Context, bookingID, actorID, therapistID int64) (*model.Booking, error) {
	// attempt to assign; repo will return ErrNoRows if already assigned or invalid
	if err := s.repo.AssignTherapist(ctx, bookingID, therapistID); err != nil {
		return nil, err
	}
	// best-effort remove from assignment queue
	_ = s.queueRepo.Remove(ctx, bookingID)
	return s.repo.GetByBookingID(ctx, bookingID)
}

// StartSession attempts to start a session for a booking. It requires that
// the therapist has arrived (status == 'arrived' or therapist_arrived_at set).
// actorRole is used for permission checks (typically 'client').
func (s *BookingService) StartSession(ctx context.Context, bookingID, actorID int64, actorRole string) (*model.Booking, error) {
	// Allow clients, therapists, and admins to start the session timer
	if actorRole != "client" && actorRole != "admin" && actorRole != "therapist" {
		return nil, fmt.Errorf("unauthorized role")
	}

	// Fetch booking without client scoping
	b, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	// Validate therapist arrival
	if b.Status != "arrived" && b.TherapistArrivedAt == nil {
		return nil, fmt.Errorf("therapist not yet arrived")
	}

	// record a role-specific confirmation event for audit/timeline
	var eventType string
	switch actorRole {
	case "therapist":
		eventType = "therapist_confirm_start"
	case "client":
		eventType = "client_confirm_start"
	default:
		eventType = "admin_confirm_start"
	}
	actor := actorID
	_ = s.repo.InsertEvent(ctx, bookingID, eventType, &actor, nil)

	// If admin invoked start, allow immediate transition. Otherwise require
	// both therapist and client confirmations to be present.
	if actorRole == "admin" {
		if err := s.repo.UpdateStatus(ctx, bookingID, actorID, "in_progress", nil, nil); err != nil {
			return nil, err
		}
		return s.repo.GetByBookingID(ctx, bookingID)
	}

	// Check timeline for both confirmations
	events, err := s.repo.ListEvents(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	hasClient := false
	hasTherapist := false
	for _, ev := range events {
		if ev.EventType == "client_confirm_start" {
			hasClient = true
		}
		if ev.EventType == "therapist_confirm_start" {
			hasTherapist = true
		}
	}
	if hasClient && hasTherapist {
		if err := s.repo.UpdateStatus(ctx, bookingID, actorID, "in_progress", nil, nil); err != nil {
			return nil, err
		}
	}

	return s.repo.GetByBookingID(ctx, bookingID)
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

func (s *BookingService) AcceptBookingOffer(ctx context.Context, therapistID, bookingID int64) error {
	// Get the offer
	offer, err := s.offerRepo.GetByTherapistAndBooking(ctx, therapistID, bookingID)
	if err != nil {
		return fmt.Errorf("offer not found: %w", err)
	}

	if offer.Status != model.BookingOfferStatusPending {
		return fmt.Errorf("offer is not pending")
	}

	if offer.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("offer expired")
	}

	// Start transaction (allow nil DB for unit tests)
	var tx pgx.Tx
	if s.db != nil {
		var err error
		tx, err = s.db.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
	} else {
		tx = pgx.Tx(nil)
	}

	// Assign therapist using Tx
	if err := s.repo.AssignTherapistWithActorTx(ctx, tx, bookingID, therapistID, therapistID); err != nil {
		return err
	}

	// Update offer status
	if err := s.offerRepo.UpdateStatusTx(ctx, tx, offer.OfferID, model.BookingOfferStatusAccepted); err != nil {
		return err
	}

	// Expire other offers for this booking
	expired, err := s.offerRepo.ExpireOffersTx(ctx, tx, bookingID)
	if err != nil {
		return err
	}

	// Remove from assignment queue
	if err := s.queueRepo.Remove(ctx, bookingID); err != nil {
		// log error but don't fail
	}

	if s.db != nil {
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	// Automatically create a conversation between client and therapist (best-effort)
	if b, err := s.repo.GetByBookingID(ctx, bookingID); err == nil && b != nil {
		go func() {
			if err := s.EnsureConversation(context.Background(), b.ClientID, therapistID); err != nil {
				log.Printf("AcceptBookingOffer: EnsureConversation failed: %v", err)
			}
		}()
	}

	// Fetch updated booking and broadcast to client and therapist so they see assignment in real-time
	if b, err := s.repo.GetByBookingID(ctx, bookingID); err == nil && b != nil {
		log.Printf("AcceptBookingOffer: Broadcasting booking:updated to client=%d and therapist=%d", b.ClientID, therapistID)
		
		// Fetch related data for enriched payload
		var service *model.Service
		if b.ServiceID != nil && s.serviceRepo != nil {
			if svc, err := s.serviceRepo.GetByID(ctx, *b.ServiceID); err == nil {
				service = svc
			}
		}
		var address *model.Address
		if b.AddressID != nil && s.addressRepo != nil {
			if addr, err := s.addressRepo.GetByIDUnsafe(ctx, *b.AddressID); err == nil {
				address = addr
			}
		}
		var therapist *model.TherapistProfile
		var therapistName, therapistPhone, therapistGender string
		if b.TherapistID != nil {
			if s.therapistRepo != nil {
				if prof, err := s.therapistRepo.GetProfile(ctx, *b.TherapistID); err == nil {
					therapist = prof
				}
			}
			if s.db != nil {
				// Fetch therapist name, phone, and gender from users table
				var userQuery = `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
				_ = s.db.QueryRow(ctx, userQuery, *b.TherapistID).Scan(&therapistName, &therapistPhone, &therapistGender)
				log.Printf("AcceptBookingOffer: Fetched therapistName='%s' for therapistID=%d", therapistName, *b.TherapistID)
			}
		}

		// Fetch client details
		var clientName, clientPhone, clientPhoto, clientGender string
		if s.db != nil {
			clientQuery := `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
			_ = s.db.QueryRow(ctx, clientQuery, b.ClientID).Scan(&clientName, &clientPhone, &clientPhoto, &clientGender)
		}
		
		// Create enriched payload with therapist details
		enrichedPayload := bookingToMapWithTherapist(b, service, address, therapist, therapistName, therapistPhone, clientName, clientPhone, clientPhoto, clientGender, therapistGender)
		
		// Send persistent notification for assignment
		s.sendBookingNotification(ctx, b, "assigned", "therapist", therapistName)

		_ = socketio.BroadcastToUser(b.ClientID, "booking:updated", enrichedPayload)
		_ = socketio.BroadcastToUser(therapistID, "booking:updated", enrichedPayload)
	} else {
		log.Printf("AcceptBookingOffer: Failed to fetch booking %d for broadcast: %v", bookingID, err)
	}

	// Broadcast event
	_ = socketio.BroadcastToUser(therapistID, "offer_accepted", map[string]any{
		"offer_id":   offer.OfferID,
		"booking_id": bookingID,
	})

	// Broadcast expiration to other therapists
	for _, o := range expired {
		// Don't send expired event to the therapist who accepted (though they shouldn't be in the expired list anyway)
		if o.TherapistID != therapistID {
			_ = socketio.BroadcastToUser(o.TherapistID, "offer_expired", map[string]any{
				"offer_id":   o.OfferID,
				"booking_id": o.BookingID,
			})
		}
	}

	return nil
}

func (s *BookingService) DeclineBookingOffer(ctx context.Context, therapistID, bookingID int64) error {
	offer, err := s.offerRepo.GetByTherapistAndBooking(ctx, therapistID, bookingID)
	if err != nil {
		return fmt.Errorf("offer not found: %w", err)
	}
// Broadcast event
	_ = socketio.BroadcastToUser(therapistID, "offer_declined", map[string]any{
		"offer_id":   offer.OfferID,
		"booking_id": bookingID,
	})

	
	if offer.Status != model.BookingOfferStatusPending {
		return fmt.Errorf("offer is not pending")
	}

	if err := s.offerRepo.UpdateStatus(ctx, offer.OfferID, model.BookingOfferStatusDeclined); err != nil {
		return err
	}

	return nil
}

// bookingToMap converts a booking model to a map[string]any for socket emission.
// This matches the structure of BookingResponse used in the REST API.
func bookingToMap(b *model.Booking, service *model.Service, address *model.Address, clientName, clientPhone, clientPhoto, clientGender string) map[string]any {
	result := map[string]any{
		"booking_id":           b.BookingID,
		"client_id":            b.ClientID,
		"therapist_id":         b.TherapistID,
		"service_id":           b.ServiceID,
		"address_id":           b.AddressID,
		"promo_id":             b.PromoID,
		"payment_method":       b.PaymentMethod,
		"gender_preference":    b.GenderPref,
		"pressure_preference":   b.PressurePref,
		"notes":                b.Notes,
		"duration_minutes":     b.DurationMinutes,
		"scheduled_start":      b.ScheduledStart,
		"actual_start":         b.ActualStart,
		"actual_end":           b.ActualEnd,
		"therapist_arrived_at":  b.TherapistArrivedAt,
		"cancelled_by":         b.CancelledBy,
		"cancelled_at":         b.CancelledAt,
		"cancellation_reason":  b.CancellationReason,
		"raw_total":            b.RawTotal,
		"discount":             b.Discount,
		"final_total":          b.FinalTotal,
		"status":               b.Status,
		"created_at":           b.CreatedAt,
		"updated_at":           b.UpdatedAt,
		"assigned_at":          b.AssignedAt,
		"client": map[string]any{
			"client_id": b.ClientID,
			"name":      clientName,
			"phone":     clientPhone,
			"photo":     clientPhoto,
			"gender":    clientGender,
		},
		"reference_code":       b.ReferenceCode,
	}

	// Add service details if available
	if service != nil {
		result["service"] = map[string]any{
			"service_id":       service.ServiceID,
			"name":             service.Name,
			"description":      service.Description,
			"duration_minutes": service.DurationMinutes,
			"base_price":       service.BasePrice,
			"category":         service.Category,
			"is_active":        service.IsActive,
		}
	}

	// Add address details if available
	if address != nil {
		result["address"] = map[string]any{
			"address_id":     address.AddressID,
			"user_id":        address.UserID,
			"label":          address.Label,
			"street_address": address.Street,
			"city":           address.City,
			"province":       address.Province,
			"postal_code":    address.PostalCode,
			"country":        address.Country,
			"latitude":       address.Latitude,
			"longitude":      address.Longitude,
			"is_default":     address.IsDefault,
		}
	}

	return result
}

// bookingToMapWithTherapist extends bookingToMap by adding therapist profile details when available.
// This is used for real-time socket broadcasts to provide clients with complete information.
func bookingToMapWithTherapist(b *model.Booking, service *model.Service, address *model.Address, therapist *model.TherapistProfile, therapistName, therapistPhone, clientName, clientPhone, clientPhoto, clientGender, therapistGender string) map[string]any {
	result := bookingToMap(b, service, address, clientName, clientPhone, clientPhoto, clientGender)
	
	// Add therapist values to a structured map if available
	therapistMap := map[string]any{}
	hasTherapistData := false

	if therapistName != "" {
		therapistMap["name"] = therapistName
		hasTherapistData = true
	}
	if therapistPhone != "" {
		therapistMap["phone"] = therapistPhone
		hasTherapistData = true
	}
	if therapistGender != "" {
		therapistMap["gender"] = therapistGender
		hasTherapistData = true
	}
	if b.TherapistID != nil {
		therapistMap["therapist_id"] = *b.TherapistID
		hasTherapistData = true
	}

	// Add therapist profile details if available
	if therapist != nil {
		therapistMap["rating"] = therapist.AvgRating
		therapistMap["years_experience"] = therapist.YearsExperience
		therapistMap["accept_assignments"] = therapist.AcceptAssignments
		// Ensure core fields are set from profile if not already
		if _, ok := therapistMap["gender"]; !ok {
			therapistMap["gender"] = therapistGender
		}
		hasTherapistData = true
	}

	if hasTherapistData {
		result["therapist"] = therapistMap
	}
	
	return result
}

func (s *BookingService) FetchTherapistInfo(ctx context.Context, therapistID *int64) (string, string, string, string, *float64) {
	if therapistID == nil || s.therapistRepo == nil || s.db == nil {
		return "", "", "", "", nil
	}

	var name string
	var rating *float64

	if s.therapistRepo != nil {
		if prof, err := s.therapistRepo.GetProfile(ctx, *therapistID); err == nil {
			rating = &prof.AvgRating
		}
	}

	// Fetch therapist details from users table
	var userQuery = `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
	var phone, photo, gender string
	_ = s.db.QueryRow(ctx, userQuery, *therapistID).Scan(&name, &phone, &photo, &gender)

	return name, phone, photo, gender, rating
}

func (s *BookingService) FetchClientInfo(ctx context.Context, clientID int64) (string, string, string, string) {
	if s.db == nil {
		return "", "", "", ""
	}

	var name, phone, photo, gender string
	query := `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
	_ = s.db.QueryRow(ctx, query, clientID).Scan(&name, &phone, &photo, &gender)

	return name, phone, photo, gender
}

func generateReferenceCode(t time.Time) string {
	datePart := t.Format("20060102")
	bytes := make([]byte, 3) 
	if _, err := rand.Read(bytes); err != nil {
		// Fallback if random fails
		return fmt.Sprintf("RH-%s-%d", datePart, t.UnixNano()%1000000)
	}
	return fmt.Sprintf("RH-%s-%s", datePart, strings.ToUpper(hex.EncodeToString(bytes)))
}

// EnsureConversation creates a 1-on-1 conversation between client and therapist
// if one does not already exist. This is called automatically when a therapist
// is assigned to a booking. It's idempotent and safe to call multiple times.
func (s *BookingService) EnsureConversation(ctx context.Context, clientID, therapistID int64) error {
	if s.messageService == nil {
		log.Printf("EnsureConversation: messageService is nil, skipping conversation creation")
		return nil
	}

	req := &model.CreateConversationRequest{
		ParticipantIDs: []int64{therapistID},
	}

	conv, err := s.messageService.CreateConversation(ctx, clientID, req)
	if err != nil {
		log.Printf("EnsureConversation: Failed to create conversation for client=%d, therapist=%d: %v", clientID, therapistID, err)
		return err
	}

	log.Printf("EnsureConversation: Conversation created/found id=%d for client=%d, therapist=%d", conv.ConversationID, clientID, therapistID)
	return nil
}

func (s *BookingService) FetchTherapistInfos(ctx context.Context, therapistIDs []int64) map[int64]model.TherapistInfo {
	if len(therapistIDs) == 0 || s.therapistRepo == nil || s.db == nil {
		return nil
	}

	infos := make(map[int64]model.TherapistInfo)

	// Fetch ratings from profiles
	ratings := make(map[int64]*float64)
	if profiles, err := s.therapistRepo.GetProfiles(ctx, therapistIDs); err == nil {
		for _, p := range profiles {
			r := p.AvgRating
			rCopy := r
			ratings[p.TherapistID] = &rCopy
		}
	}

	// Fetch details from users table
	query := "SELECT user_id, COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = ANY($1)"
	if rows, err := s.db.Query(ctx, query, therapistIDs); err == nil {
		defer rows.Close()
		for rows.Next() {
			var info model.TherapistInfo
			if err := rows.Scan(&info.TherapistID, &info.Name, &info.Phone, &info.Photo, &info.Gender); err == nil {
				if r, ok := ratings[info.TherapistID]; ok {
					info.Rating = r
				}
				infos[info.TherapistID] = info
			}
		}
	}

	return infos
}

func (s *BookingService) FetchClientInfos(ctx context.Context, clientIDs []int64) map[int64]model.ClientInfo {
	if len(clientIDs) == 0 || s.db == nil {
		return nil
	}

	infos := make(map[int64]model.ClientInfo)

	query := "SELECT user_id, COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = ANY($1)"
	if rows, err := s.db.Query(ctx, query, clientIDs); err == nil {
		defer rows.Close()
		for rows.Next() {
			var info model.ClientInfo
			if err := rows.Scan(&info.ClientID, &info.Name, &info.Phone, &info.Photo, &info.Gender); err == nil {
				infos[info.ClientID] = info
			}
		}
	}
	return infos
}

func (s *BookingService) sendBookingNotification(ctx context.Context, b *model.Booking, status string, actorRole, therapistName string) {
	if s.notificationService == nil {
		return
	}
    
	var title, message string
    // Default target is client
	targetUserID := b.ClientID

	switch status {
	case "assigned":
		title = "Booking Confirmed"
		if therapistName != "" {
			message = fmt.Sprintf("Your booking has been accepted by %s.", therapistName)
		} else {
			message = "Your booking has been accepted by a therapist."
		}
	case "on_the_way":
		title = "Therapist Incoming"
		if therapistName != "" {
			message = fmt.Sprintf("%s is on the way to your location.", therapistName)
		} else {
			message = "Your therapist is on the way."
		}
	case "arrived":
		title = "Therapist Arrived"
		if therapistName != "" {
			message = fmt.Sprintf("%s has arrived.", therapistName)
		} else {
			message = "Your therapist has arrived."
		}
	case "completed":
		title = "Thank You! 💛"
		message = "Thank you so much for choosing Relaxation Hub! We're truly grateful for your trust. 🙏\nWe hope you feel lighter and completely relaxed! 😄\nWhen you’re ready for your next massage, we’ll be here — just a booking away.\nBook again soon and let us make relaxation the best part of your week! 💆‍♀️✨"
	case "cancelled":
		title = "Booking Cancelled"
		message = "Your booking has been cancelled."
		// If cancelled by client, notify therapist
		if actorRole == "client" && b.TherapistID != nil {
			targetUserID = *b.TherapistID
			message = "The client has cancelled the booking."
		} else if actorRole == "therapist" {
             // If cancelled by therapist, notify client (already default)
             message = "The therapist has cancelled the booking."
        }
	default:
		return
	}

	_, _ = s.notificationService.Create(ctx, &model.CreateNotificationRequest{
		UserID:  targetUserID,
		Type:    "booking_status",
		Title:   title,
		Message: message,
		Data: map[string]any{
			"booking_id": b.BookingID,
			"status":     status,
		},
	})
}
