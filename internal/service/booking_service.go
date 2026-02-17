package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"golang.org/x/sync/errgroup"
)

var (
	errInvalidStatus = errors.New("invalid booking status")
)

// AllowedStatus enumerates acceptable booking statuses.
var AllowedStatus = map[string]struct{}{
	model.BookingStatusPending:    {},
	model.BookingStatusAssigned:   {},
	model.BookingStatusOnTheWay:   {},
	model.BookingStatusArrived:    {},
	model.BookingStatusInProgress: {},
	model.BookingStatusCompleted:  {},
	model.BookingStatusCancelled:  {},
	model.BookingStatusNoShow:     {},
	"rescheduled":                 {}, // Not in standard flow yet, leaving permissible
	"paid":                        {}, // Not in standard flow yet
}

type MessageServiceInterface interface {
	CreateConversation(ctx context.Context, initiatorID int64, req *model.CreateConversationRequest) (*model.ConversationResponse, error)
	GetConversationsByUser(ctx context.Context, userID int64) ([]model.ConversationResponse, error)
	SendMessage(ctx context.Context, senderID int64, req *model.SendMessageRequest) (*model.Message, error)
	SendSystemMessage(ctx context.Context, conversationID int64, content string) error
	MarkMessageAsRead(ctx context.Context, messageID, userID int64) error
}

type NotificationServiceInterface interface {
	Create(ctx context.Context, req *model.CreateNotificationRequest) (*model.Notification, error)
	SendPushDirect(ctx context.Context, userID int64, notifType, title, message string, data map[string]string)
	ListByUser(ctx context.Context, userID int64, limit, offset int) (*model.PaginatedNotificationsResponse, error)
	MarkAsRead(ctx context.Context, notificationID, userID int64) error
}

type LogisticsServiceInterface interface {
	HandleBookingAssigned(ctx context.Context, bookingID int64) error
	CancelRideForBooking(ctx context.Context, bookingID int64) error
	UpdateRideForBooking(ctx context.Context, bookingID int64) error
}

type BookingService struct {
	repo                 repository.BookingRepository
	promoRepo            repository.PromotionRepository
	serviceRepo          repository.ServiceRepository
	addressRepo          repository.AddressRepository
	userRepo             repository.UserRepository
	db                   db.DBTX
	queueRepo            repository.AssignmentQueueRepository
	therapistRepo        repository.TherapistRepository
	offerRepo            repository.BookingOfferRepository
	messageService       MessageServiceInterface
	notificationService  NotificationServiceInterface
	logisticsService     LogisticsServiceInterface
	extensionRequestRepo repository.ExtensionRequestRepository
	walletService        *WalletService
	rideService          *RideService
}

// NewBookingService creates a new instance of BookingService.
func NewBookingService(repo repository.BookingRepository, promoRepo repository.PromotionRepository, db db.DBTX, queueRepo repository.AssignmentQueueRepository, therapistRepo repository.TherapistRepository, offerRepo repository.BookingOfferRepository, serviceRepo repository.ServiceRepository, addressRepo repository.AddressRepository, userRepo repository.UserRepository, msgService MessageServiceInterface, notifService NotificationServiceInterface, extRepo repository.ExtensionRequestRepository, walletService *WalletService, rideService *RideService) *BookingService {
	return &BookingService{
		repo:                 repo,
		promoRepo:            promoRepo,
		db:                   db,
		queueRepo:            queueRepo,
		therapistRepo:        therapistRepo,
		offerRepo:            offerRepo,
		serviceRepo:          serviceRepo,
		addressRepo:          addressRepo,
		userRepo:             userRepo,
		messageService:       msgService,
		notificationService:  notifService,
		extensionRequestRepo: extRepo,
		walletService:        walletService,
		rideService:          rideService,
	}
}

// SetLogisticsService allows injecting LogisticsService after BookingService creation
// This is necessary to avoid circular dependencies (LogisticsService needs BookingRepo)
func (s *BookingService) SetLogisticsService(ls *LogisticsService) {
	s.logisticsService = ls
}

// ListOffersForTherapist returns current active pending offers targeted to a therapist.
func (s *BookingService) ListOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) {
	if s.offerRepo == nil {
		return nil, nil
	}
	return s.offerRepo.GetActiveOffersForTherapist(ctx, therapistID)
}

func (s *BookingService) Create(ctx context.Context, clientID int64, req *model.CreateBookingRequest, actorID *int64) (*model.Booking, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}

	scheduledStart := getScheduledStart(req)
	
	// Start Transaction
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

	// 1. Calculate Totals & Resolve Promo
	booking, err := s.prepareBooking(ctx, tx, clientID, req, scheduledStart)
	if err != nil {
		return nil, err
	}

	// 2. Insert Booking
	if err := s.repo.CreateTx(ctx, tx, booking); err != nil {
		return nil, err
	}

	// 3. Enqueue for assignment if no therapist assigned
	// Do this INSIDE the transaction to ensure atomicity
	if booking.TherapistID == nil {
		if err := s.queueRepo.EnqueueTx(ctx, tx, booking.BookingID); err != nil {
			return nil, fmt.Errorf("failed to enqueue booking: %w", err)
		}
	}

	// Commit Transaction
	if s.db != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}

	// --- Post-Commit Actions (Events, Broadcasts, Initial Offers) ---

	// Record creation event
	actor := clientID
	if actorID != nil {
		actor = *actorID
	}
	_ = s.repo.InsertEvent(ctx, booking.BookingID, "created", &actor, nil)

	// Broadcast updates
	_ = broadcaster.BroadcastToUser(booking.ClientID, "booking:created", booking)
	if booking.TherapistID != nil {
		_ = broadcaster.BroadcastToUser(*booking.TherapistID, "booking:created", booking)
	}

	// Optimistic Offering (Fire-and-forget, handled by worker eventually if this fails)
	if booking.TherapistID == nil {
		go s.createInitialOffers(context.WithoutCancel(ctx), booking, req.TherapistID)
	}

	return booking, nil
}

func validateCreateRequest(req *model.CreateBookingRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 60
	}
	if req.DurationMinutes%15 != 0 {
		return NewValidationError("invalid_duration", "duration_minutes must be in 15-minute increments", map[string]string{"duration_minutes": "must be a multiple of 15"})
	}

	pm := strings.TrimSpace(strings.ToLower(req.PaymentMethod))
	if pm != "" && pm != model.PaymentMethodCash && pm != model.PaymentMethodGCash && pm != model.PaymentMethodCard && pm != model.PaymentMethodBDO {
		return NewValidationError("invalid_payment_method", "invalid payment_method: must be 'cash', 'gcash', 'bdo', or 'card'", map[string]string{"payment_method": "allowed values: cash, gcash, bdo, card"})
	}
	return nil
}

func getScheduledStart(req *model.CreateBookingRequest) *time.Time {
	if req.ScheduledStart != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduledStart)
		if err == nil {
			return &t
		}
	}
	now := time.Now()
	return &now
}

func (s *BookingService) prepareBooking(ctx context.Context, tx pgx.Tx, clientID int64, req *model.CreateBookingRequest, scheduled *time.Time) (*model.Booking, error) {
	if req.ServiceID == nil {
		return nil, fmt.Errorf("service_id is required")
	}
	service, err := s.serviceRepo.GetByID(ctx, *req.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("invalid service: %w", err)
	}

	basePrice := service.BasePrice
	extraCost := 0.0
	if req.DurationMinutes > service.DurationMinutes && service.DurationMinutes > 0 {
		diff := req.DurationMinutes - service.DurationMinutes
		ratePerMinute := service.BasePrice / float64(service.DurationMinutes)
		extraCost = ratePerMinute * float64(diff)
	}
	calculatedRawTotal := basePrice + extraCost
	req.RawTotal = &calculatedRawTotal

	var discount *float64
	var promoID *int64

	if strings.TrimSpace(req.VoucherCode) != "" {
		p, err := s.promoRepo.GetByCode(ctx, strings.TrimSpace(req.VoucherCode))
		if err != nil {
			return nil, NewValidationError("invalid_voucher", "invalid voucher code", map[string]string{"voucher_code": "not found or expired"})
		}
		
		now := time.Now()
		if p.ValidFrom != nil && p.ValidFrom.After(now) {
			return nil, fmt.Errorf("voucher not yet active")
		}
		if p.ValidUntil != nil && p.ValidUntil.Before(now) {
			return nil, fmt.Errorf("voucher expired")
		}

		ok, err := s.promoRepo.TryIncrementGlobalUsageTx(ctx, tx, p.PromoID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, NewValidationError("invalid_voucher", "voucher fully redeemed", map[string]string{"voucher_code": "redemption limit reached"})
		}

		if _, err := s.promoRepo.TryIncrementUserPromoUsageTx(ctx, tx, p.PromoID, clientID); err != nil {
			return nil, err
		}

		if p.DiscountAmount != nil && *p.DiscountAmount > 0 {
			d := *p.DiscountAmount
			discount = &d
		} else if p.DiscountPct != nil && *p.DiscountPct > 0 {
			d := calculatedRawTotal * float64(*p.DiscountPct) / 100.0
			discount = &d
		}
		
		if discount != nil && *discount > calculatedRawTotal {
			d := calculatedRawTotal
			discount = &d
		}
		promoID = &p.PromoID
	}

	finalTotal := computeFinal(&calculatedRawTotal, discount)

	serviceSnapshot := service.Name
	if req.DurationMinutes > 0 {
		serviceSnapshot = fmt.Sprintf("%s (%dmin)", service.Name, req.DurationMinutes)
	}
	breakdown := &model.PaymentBreakdown{
		BasePrice:       basePrice,
		DurationMarkup:  extraCost,
		ExtensionsTotal: 0,
		ServiceSnapshot: serviceSnapshot,
	}
	breakdownJSON, _ := json.Marshal(breakdown)
	code := generateReferenceCode(*scheduled)

	return &model.Booking{
		ClientID:        clientID,
		TherapistID:     nil,
		ServiceID:       req.ServiceID,
		AddressID:       req.AddressID,
		PromoID:         promoID,
		PaymentMethod:   strings.TrimSpace(strings.ToLower(req.PaymentMethod)),
		GenderPref:      strings.TrimSpace(req.GenderPref),
		PressurePref:    strings.TrimSpace(req.PressurePref),
		Notes:           strings.TrimSpace(req.Notes),
		DurationMinutes: req.DurationMinutes,
		ChangeFor:       req.ChangeFor,
		ScheduledStart:  scheduled,
		RawTotal:        req.RawTotal,
		Discount:        discount,
		FinalTotal:      finalTotal,
		Status:          model.BookingStatusPending,
		ReferenceCode:   &code,
		PaymentBreakdownJSON: breakdownJSON,
		PaymentBreakdown:     breakdown,
	}, nil
}

func (s *BookingService) createInitialOffers(ctx context.Context, booking *model.Booking, requestedTherapistID *int64) {
	// Offer-to-therapists-first: try to create short-lived offers to top candidates
	// Default to up to 3 candidates
	const offerCandidates = 3

	// Dynamic TTL: If booking is scheduled > 24 hours in the future, give therapist 24h to respond.
	// Otherwise, use aggressive 30-minute TTL for quick fulfillment.
	offerTTL := time.Minute * 30
	isFutureBooking := false
	if booking.ScheduledStart != nil {
		untilStart := time.Until(*booking.ScheduledStart)
		if untilStart > 24*time.Hour {
			offerTTL = 24 * time.Hour
			isFutureBooking = true
		}
	}

	var candidates []model.TherapistProfile
	if requestedTherapistID != nil {
		// If specific therapist requested, offer to them only
		candidates = []model.TherapistProfile{{TherapistID: *requestedTherapistID}}
		// For future bookings with specific therapist, extend TTL to give them more time
		if isFutureBooking {
			offerTTL = 24 * time.Hour
		}
	} else if booking.ServiceID != nil && booking.ScheduledStart != nil {
		// Use time-aware matching to avoid double-booking therapists
		cands, err := s.therapistRepo.FindAvailableByServiceWithTime(ctx, booking.ClientID, *booking.ServiceID, booking.GenderPref, booking.PressurePref, *booking.ScheduledStart, booking.DurationMinutes, nil, nil)
		if err == nil {
			candidates = cands
		}
	} else if booking.ServiceID != nil {
		// Fallback to basic matching if no scheduled start
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

		// Pre-fetch enriched booking data ONCE (service, address, client details)
		// to avoid N+1 queries inside the candidate loop
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
		var clientName, clientPhone, clientPhoto, clientGender string
		if s.db != nil {
			userQuery := `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
			if err := s.db.QueryRow(ctx, userQuery, booking.ClientID).Scan(&clientName, &clientPhone, &clientPhoto, &clientGender); err != nil {
				slog.Warn("failed to fetch client details for offer", "error", err)
			}
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
			slog.Info("offer made", "booking_id", booking.BookingID, "therapist_id", cand.TherapistID, "offer_id", o.OfferID)

			// Create enriched payload for socket event (includes full booking details)
			socketPayload := map[string]any{
				"offer_id":            o.OfferID,
				"target_therapist_id": o.TherapistID,
				"expires_at":          o.ExpiresAt.Format(time.RFC3339),
				"booking_id":          booking.BookingID,
				"is_bundle":           false, // initial offer is single
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
			
			// ENRICHMENT for Therapist App Offer Dialog (expects these at top-level)
			if clientName != "" {
				socketPayload["client_name"] = clientName
			} else {
				socketPayload["client_name"] = "Client"
			}

			if svc != nil {
				socketPayload["service_name"] = svc.Name
			}
			
			if addr != nil {
				txt := addr.Street
				if addr.Label != "" {
					txt = fmt.Sprintf("%s (%s)", addr.Label, txt)
				}
				socketPayload["address"] = txt
			}
			
			if o.EstimatedEarnings != nil {
				socketPayload["price"] = *o.EstimatedEarnings
			} else if booking.FinalTotal != nil {
				// Fallback to booking total if earnings not set (though they should be)
				socketPayload["price"] = *booking.FinalTotal
			}

			// Keep minimal metadata for event log (database storage)
			eventMeta := map[string]any{
				"offer_id":            o.OfferID,
				"target_therapist_id": o.TherapistID,
				"expires_at":          o.ExpiresAt.Format(time.RFC3339),
				"booking_id":          booking.BookingID,
			}
			_ = s.repo.InsertEvent(ctx, booking.BookingID, "offered_to_therapist", nil, eventMeta)

			// Notify therapist in real-time via socket.io with enriched data (best-effort)
			go func(tid int64, payload map[string]any) {
				// ignore errors; this is best-effort
				_ = broadcaster.BroadcastToUser(tid, "offered_to_therapist", payload)
			}(o.TherapistID, socketPayload)

			// Send Push Notification (via NotificationService)
			if s.notificationService != nil {
				// Convert payload to map[string]string for FCM data payload if needed, 
				// but NotificationService.Create takes interface{} for Data and marshals it.
				// We can just pass the socketPayload directly as the Data.
				
				// Create notification record + Push
				offerTitle := "New Booking Offer"
				offerMsg := "You have a new booking offer!"
				
				if svc != nil {
					offerTitle = fmt.Sprintf("New %s Offer", svc.Name)
					offerMsg = fmt.Sprintf("%d min session", svc.DurationMinutes)
				}
				if addr != nil {
					location := addr.City
					if addr.Label != "" {
						location = addr.Label
					}
					offerMsg += fmt.Sprintf(" in %s", location)
				}
				offerMsg += ". Tap to view."

				_, err := s.notificationService.Create(ctx, &model.CreateNotificationRequest{
					UserID:  o.TherapistID,
					Type:    "booking_offer",
					Title:   offerTitle,
					Message: offerMsg,
					Data:    socketPayload,
				})
				if err != nil {
					slog.Warn("failed to create notification for initial offer", "therapist_id", o.TherapistID, "error", err)
				}
			}
		}
	}
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
	if req.DurationMinutes%15 != 0 {
		return nil, NewValidationError("invalid_duration", "duration_minutes must be in 15-minute increments", map[string]string{"duration_minutes": "must be a multiple of 15"})
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
	if pm != "" && pm != model.PaymentMethodCash && pm != model.PaymentMethodGCash && pm != model.PaymentMethodCard && pm != model.PaymentMethodBDO {
		return nil, NewValidationError("invalid_payment_method", "invalid payment_method: must be 'cash', 'gcash', 'bdo', or 'card'", map[string]string{"payment_method": "allowed values: cash, gcash, bdo, card"})
	}

	// Calculate totals if missing (robustness for web client)
	if req.Total == nil || *req.Total == 0 {
		if req.ServiceID == nil {
			return nil, fmt.Errorf("service_id is required")
		}
		service, err := s.serviceRepo.GetByID(ctx, *req.ServiceID)
		if err != nil {
			return nil, fmt.Errorf("invalid service: %w", err)
		}

		basePrice := service.BasePrice
		extraCost := 0.0
		if req.DurationMinutes > service.DurationMinutes && service.DurationMinutes > 0 {
			diff := req.DurationMinutes - service.DurationMinutes
			ratePerMinute := service.BasePrice / float64(service.DurationMinutes)
			extraCost = ratePerMinute * float64(diff)
		}
		calculatedRawTotal := basePrice + extraCost
		req.RawTotal = &calculatedRawTotal
		req.Total = &calculatedRawTotal
		// Note: Admin bookings typically bypass promos and discounts unless explicitly provided,
		// so FinalTotal = RawTotal is a safe default here.
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
		Status:          model.BookingStatusPending,
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
		case repository.ErrServiceNotOffered:
			return nil, NewValidationError("service_not_offered", "therapist does not offer this service", map[string]string{"therapist_id": "does not offer service"})
		case pgx.ErrNoRows:
			return nil, NewValidationError("cannot_assign", "therapist could not be assigned to booking", map[string]string{"therapist_id": "failed gating or already assigned"})
		default:
			return nil, err
		}
	}

	// 3. Enqueue for assignment if not assigned
	// Do this INSIDE the transaction
	if booking.TherapistID == nil {
		if err := s.queueRepo.EnqueueTx(ctx, tx, booking.BookingID); err != nil {
			return nil, fmt.Errorf("failed to enqueue booking: %w", err)
		}
	}

	// commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// record admin-created booking event (and assigned event already recorded inside tx)
	actor := adminID
	_ = s.repo.InsertEvent(ctx, booking.BookingID, "admin_created_booking", &actor, nil)

	// Broadcast updates
	_ = broadcaster.BroadcastToUser(booking.ClientID, "booking:created", booking)
	if booking.TherapistID != nil {
		_ = broadcaster.BroadcastToUser(*booking.TherapistID, "booking:created", booking)
	}

	// Optimistic Offering (Fire-and-forget)
	if booking.TherapistID == nil {
		go s.createInitialOffers(context.WithoutCancel(ctx), booking, req.TherapistID)
	}

	// reload booking to ensure assigned fields are present
	nb, err := s.repo.GetByBookingID(ctx, booking.BookingID)
	if err != nil {
		return nil, err
	}

	// Broadcast updates
	_ = broadcaster.BroadcastToUser(booking.BookingID, "booking:assigned", nil) // triggers reload for anyone watching booking
	_ = broadcaster.BroadcastToUser(nb.ClientID, "booking:updated", nb)
	if nb.TherapistID != nil {
		_ = broadcaster.BroadcastToUser(*nb.TherapistID, "booking:assigned", nb)

		// Notify Therapist (Push) - ONLY if assigned by admin (actorID != therapistID)
		if adminID != *nb.TherapistID && s.notificationService != nil {
			title := "New Booking Assigned"
			msg := "You have been assigned to a new booking."
			
			// Fetch service and address for better message
			var svcName string
			if nb.ServiceID != nil && s.serviceRepo != nil {
				if svc, err := s.serviceRepo.GetByID(ctx, *nb.ServiceID); err == nil {
					svcName = svc.Name
				}
			}
			var location string
			if nb.AddressID != nil && s.addressRepo != nil {
				if addr, err := s.addressRepo.GetByIDUnsafe(ctx, *nb.AddressID); err == nil {
					location = addr.City
				}
			}

			if svcName != "" {
				title = fmt.Sprintf("Assigned: %s", svcName)
			}
			
			timeStr := "now"
			if nb.ScheduledStart != nil {
				timeStr = nb.ScheduledStart.Format("3:04 PM")
			}
			msg = fmt.Sprintf("New booking for %s", timeStr)
			if location != "" {
				msg += fmt.Sprintf(" in %s", location)
			}

			go s.notificationService.Create(context.WithoutCancel(ctx), &model.CreateNotificationRequest{
				UserID:  *nb.TherapistID,
				Type:    "booking_status",
				Title:   title,
				Message: msg,
				Data: map[string]interface{}{
					"booking_id": nb.BookingID,
					"status":     "assigned",
				},
			})
		}
	}

	// Trigger logistics orchestration (ride creation) asynchronously
	if nb.TherapistID != nil && s.logisticsService != nil {
		go func() {
			if err := s.logisticsService.HandleBookingAssigned(context.Background(), nb.BookingID); err != nil {
				slog.Error("CreateForAdmin: failed to handle logistics", "booking_id", nb.BookingID, "error", err)
			}
		}()
	}

	return nb, nil
}

func (s *BookingService) GetByID(ctx context.Context, bookingID, clientID int64) (*model.Booking, error) {
	return s.repo.GetByID(ctx, bookingID, clientID)
}

// GetByBookingID fetches a booking without user scoping (for admin/worker usage).
func (s *BookingService) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	return s.repo.GetByBookingID(ctx, bookingID)
}

func (s *BookingService) GetByCode(ctx context.Context, referenceCode string, clientID int64) (*model.Booking, error) {
	details, err := s.repo.GetBookingByCodeWithDetails(ctx, referenceCode, clientID)
	if err != nil {
		return nil, err
	}
	return details.Booking, nil
}

// GetBookingWithTimeline returns booking and its timeline events for client viewing
// Optimized to use a single query with JOINs for all related data
func (s *BookingService) GetBookingWithTimeline(ctx context.Context, bookingID, clientID int64, actorRole string) (*BookingWithTimelineResult, error) {
	// If admin, use unscoped query
	if actorRole == model.RoleAdmin {
		details, err := s.repo.GetBookingWithDetailsUnsafe(ctx, bookingID)
		if err != nil {
			return nil, err
		}
		events, _ := s.repo.ListEvents(ctx, bookingID)
		return s.toBookingWithTimelineResult(details, events), nil
	}

	// Try optimized query first (works if user is client or therapist)
	details, err := s.repo.GetBookingWithDetails(ctx, bookingID, clientID)
	if err == nil {
		events, _ := s.repo.ListEvents(ctx, bookingID)
		res := s.toBookingWithTimelineResult(details, events)
		if s.rideService != nil {
			ride, _ := s.rideService.GetRideByBookingID(ctx, bookingID)
			if ride != nil && ride.RiderID != nil {
				// Enrich with Rider Profile (Vehicle info)
				if profile, err := s.rideService.GetProfileByRiderID(ctx, *ride.RiderID); err == nil && profile != nil {
					ride.VehicleType = profile.VehicleType
					ride.LicensePlate = profile.LicensePlate

					// Enrich with User Info (Name, Phone)
					if s.userRepo != nil {
						if infos, err := s.userRepo.GetUserInfoBatch(ctx, []int64{profile.UserID}); err == nil {
							if info, ok := infos[profile.UserID]; ok {
								ride.RiderName = info.Name
								ride.RiderPhone = info.Phone
							}
						}
					}
				}
			}
			res.ActiveRide = ride
		}
		return res, nil
	}

	// If optimized query failed, check if user has pending offer
	if err == pgx.ErrNoRows && s.offerRepo != nil {
		offer, _ := s.offerRepo.GetByTherapistAndBooking(ctx, clientID, bookingID)
		if offer != nil && offer.Status == model.BookingOfferStatusPending && offer.ExpiresAt.After(time.Now()) {
			details, err := s.repo.GetBookingWithDetailsUnsafe(ctx, bookingID)
			if err == nil {
				events, _ := s.repo.ListEvents(ctx, bookingID)
				res := s.toBookingWithTimelineResult(details, events)
                
                // Fetch active ride if available
                if s.rideService != nil {
                    ride, _ := s.rideService.GetRideByBookingID(ctx, bookingID)
                    res.ActiveRide = ride
                }
                return res, nil
			}
		}
	}

	return nil, err
}

// GetBookingByCodeWithTimeline returns booking and its timeline events for client viewing by reference code
// GetBookingByCodeWithTimeline returns booking and its timeline events for client viewing by reference code
func (s *BookingService) GetBookingByCodeWithTimeline(ctx context.Context, referenceCode string, clientID int64, actorRole string) (*BookingWithTimelineResult, error) {
	// If admin, use unscoped query
	if actorRole == model.RoleAdmin {
		details, err := s.repo.GetBookingByCodeWithDetailsUnsafe(ctx, referenceCode)
		if err != nil {
			return nil, err
		}
		events, _ := s.repo.ListEvents(ctx, details.Booking.BookingID)
		return s.toBookingWithTimelineResult(details, events), nil
	}

	// Try optimized query first
	details, err := s.repo.GetBookingByCodeWithDetails(ctx, referenceCode, clientID)
	if err == nil {
		events, _ := s.repo.ListEvents(ctx, details.Booking.BookingID)
		return s.toBookingWithTimelineResult(details, events), nil
	}

	// Check if user has pending offer
	if err == pgx.ErrNoRows && s.offerRepo != nil {
		detailsUnsafe, errUnsafe := s.repo.GetBookingByCodeWithDetailsUnsafe(ctx, referenceCode)
		if errUnsafe == nil {
			offer, _ := s.offerRepo.GetByTherapistAndBooking(ctx, clientID, detailsUnsafe.Booking.BookingID)
			if offer != nil && offer.Status == model.BookingOfferStatusPending && offer.ExpiresAt.After(time.Now()) {
				events, _ := s.repo.ListEvents(ctx, detailsUnsafe.Booking.BookingID)
				return s.toBookingWithTimelineResult(detailsUnsafe, events), nil
			}
		}
	}

	return nil, err
}

func (s *BookingService) toBookingWithTimelineResult(details *repository.BookingDetailsResult, events []model.BookingEvent) *BookingWithTimelineResult {
	return &BookingWithTimelineResult{
		Booking:         details.Booking,
		Events:          events,
		Service:         details.Service,
		Address:         details.Address,
		TherapistName:   details.TherapistName,
		TherapistPhone:  details.TherapistPhone,
		TherapistPhoto:  details.TherapistPhoto,
		TherapistGender: details.TherapistGender,
		TherapistRating: details.TherapistRating,
		ClientName:      details.ClientName,
		ClientPhone:     details.ClientPhone,
		ClientPhoto:     details.ClientPhoto,
		ClientGender:    details.ClientGender,
		PromoCode:       details.PromoCode,
	}
}

func (s *BookingService) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	return s.repo.ListByClient(ctx, clientID)
}

func (s *BookingService) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	return s.repo.ListByTherapist(ctx, therapistID)
}

// ListByClientWithDetails returns enriched bookings with all related data using optimized JOINs
func (s *BookingService) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return s.repo.ListByClientWithDetails(ctx, clientID)
}

// ListByTherapistWithDetails returns enriched bookings with all related data using optimized JOINs
func (s *BookingService) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return s.repo.ListByTherapistWithDetails(ctx, therapistID)
}

// ListByClientWithDetailsPaginated returns paginated bookings for a client with total count
func (s *BookingService) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return s.repo.ListByClientWithDetailsPaginated(ctx, clientID, limit, offset)
}

// ListByTherapistWithDetailsPaginated returns paginated bookings for a therapist with total count
func (s *BookingService) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return s.repo.ListByTherapistWithDetailsPaginated(ctx, therapistID, limit, offset)
}

// ListAllWithDetailsPaginated returns paginated bookings for all users (admin usage)
func (s *BookingService) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int) ([]repository.BookingDetailsResult, int, error) {
	return s.repo.ListAllWithDetailsPaginated(ctx, limit, offset)
}

// ListPendingBookings returns all pending bookings without therapist assignment (for admin UI)
func (s *BookingService) ListPendingBookings(ctx context.Context) ([]model.Booking, error) {
	return s.repo.ListGlobalPending(ctx)
}

// GetOffersForBooking returns all offers (active, expired, rejected) for a specific booking
func (s *BookingService) GetOffersForBooking(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	if s.offerRepo == nil {
		return []model.BookingOffer{}, nil
	}
	return s.offerRepo.GetOffersByBookingID(ctx, bookingID)
}

// GetCandidatesForBooking returns potential therapists for a booking (for admin manual intervention)
func (s *BookingService) GetCandidatesForBooking(ctx context.Context, bookingID int64) ([]model.TherapistProfile, error) {
	b, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if b.ServiceID == nil {
		return []model.TherapistProfile{}, nil
	}
	if s.therapistRepo == nil {
		return []model.TherapistProfile{}, nil
	}
	return s.therapistRepo.FindAvailableByService(ctx, b.ClientID, *b.ServiceID, b.GenderPref, b.PressurePref)
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
		if *req.DurationMinutes%15 != 0 {
			return nil, NewValidationError("invalid_duration", "duration_minutes must be in 15-minute increments", map[string]string{"duration_minutes": "must be multiple of 15"})
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

	// Track whether schedule or location changed for ride rescheduling
	scheduleChanged := req.ScheduledStart != nil
	locationChanged := req.AddressID != nil

	if err := s.repo.Update(ctx, booking); err != nil {
		return nil, err
	}

	// Reschedule rides if time or location changed and therapist is assigned
	if (scheduleChanged || locationChanged) && booking.TherapistID != nil && s.logisticsService != nil {
		go func() {
			if err := s.logisticsService.UpdateRideForBooking(context.Background(), bookingID); err != nil {
				slog.Error("Update: failed to reschedule ride", "booking_id", bookingID, "error", err)
			}
		}()
	}

	return s.repo.GetByID(ctx, bookingID, clientID)
}

// UpdateByAdmin allows admins to update any booking, including reassignment.
func (s *BookingService) UpdateByAdmin(ctx context.Context, adminID, bookingID int64, req *model.UpdateBookingRequest) (*model.Booking, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	booking, err := s.repo.GetByBookingID(ctx, bookingID)
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
		if *req.DurationMinutes%15 != 0 {
			return nil, NewValidationError("invalid_duration", "duration_minutes must be in 15-minute increments", map[string]string{"duration_minutes": "must be multiple of 15"})
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

	// Reassignment logic
	therapistChanged := false
	if req.TherapistID != nil {
		newID := *req.TherapistID
		oldID := int64(0)
		if booking.TherapistID != nil {
			oldID = *booking.TherapistID
		}
		if newID != oldID {
			booking.TherapistID = &newID
			// If previously pending, set to assigned?
			if booking.Status == model.BookingStatusPending {
				booking.Status = model.BookingStatusAssigned
				now := time.Now()
				booking.AssignedAt = &now
			}
			therapistChanged = true
			
			// Log reassignment
			md := map[string]any{"old_therapist_id": oldID, "new_therapist_id": newID}
			_ = s.repo.InsertEvent(ctx, bookingID, "admin_reassigned_therapist", &adminID, md)
		}
	}

	if req.Total != nil {
		booking.FinalTotal = req.Total
	}

	// Persist
	if err := s.repo.UpdateAdmin(ctx, booking); err != nil {
		return nil, err
	}

	// Side effects: Ride updates
	scheduleChanged := req.ScheduledStart != nil
	locationChanged := req.AddressID != nil
	
	if (scheduleChanged || locationChanged || therapistChanged) && booking.TherapistID != nil && s.logisticsService != nil {
		go func() {
			if err := s.logisticsService.UpdateRideForBooking(context.Background(), bookingID); err != nil {
				slog.Error("UpdateByAdmin: failed to update ride", "booking_id", bookingID, "error", err)
			}
		}()
	}

	return s.repo.GetByBookingID(ctx, bookingID)
}

// UpdateStatusFromRide updates a booking's status as triggered by a ride event.
// This bypasses role-based checks since it's a system-level transition.
// Implements the BookingStatusUpdater interface for RideService.
func (s *BookingService) UpdateStatusFromRide(ctx context.Context, bookingID int64, status string) error {
	if err := s.repo.UpdateStatus(ctx, bookingID, 0, "system", status, nil, nil); err != nil {
		return err
	}
	s.broadcastBookingUpdate(ctx, bookingID, status, "system")
	slog.Info("UpdateStatusFromRide: booking status updated", "booking_id", bookingID, "status", status)
	return nil
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
	therapistAllowed := map[string]struct{}{"on_the_way": {}, "arrived": {}, "in_progress": {}, "completed": {}, "paid": {}}
	// Clients can cancel from pending, assigned, on_the_way, or arrived.
	// Late cancellations (on_the_way, arrived) will trigger admin notification.
	clientAllowed := map[string]struct{}{"cancelled": {}, "pending": {}}

	switch actorRole {
	case "admin":
		// admin may do everything
	case "therapist":
		slog.Debug("UpdateStatus: checking therapistAllowed", "actor_id", actorID, "status", status)
		if _, ok := therapistAllowed[status]; !ok {
			return nil, fmt.Errorf("therapist not allowed to set status: %s", status)
		}
	case "client":
		slog.Debug("UpdateStatus: checking clientAllowed", "actor_id", actorID, "status", status)
		if _, ok := clientAllowed[status]; !ok {
			return nil, fmt.Errorf("client not allowed to set status: %s", status)
		}
	default:
		return nil, fmt.Errorf("unauthorized role")
	}

	// LATE CANCELLATION CHECK: Fetch current booking to check if it's a late cancel
	var currentBooking *model.Booking
	if status == "cancelled" && actorRole == "client" {
		var err error
		currentBooking, err = s.repo.GetByID(ctx, bookingID, actorID)
		if err != nil {
			return nil, fmt.Errorf("failed to find booking for cancellation check: %w", err)
		}
		currentStatus := strings.ToLower(currentBooking.Status)
		// Allow cancellation from pending, assigned, on_the_way, arrived
		allowedCancelFrom := map[string]bool{"pending": true, "assigned": true, "on_the_way": true, "arrived": true}
		if !allowedCancelFrom[currentStatus] {
			return nil, fmt.Errorf("cannot cancel booking in status: %s", currentStatus)
		}
		// If late cancel (on_the_way or arrived), notify admins
		if currentStatus == "on_the_way" || currentStatus == "arrived" {
			slog.Warn("late cancellation by client", "booking_id", bookingID, "previous_status", currentStatus)
			if s.notificationService != nil {
				// Send persistent notification to admins (user_id=0 can be used for system/admin channel if supported, or log it)
				// For now, logging is the primary notification. Admins can check logs or a dashboard.
				// Optionally, create a record in a dedicated "admin_alerts" table or send to a monitoring channel.
				_ = s.repo.InsertEvent(ctx, bookingID, "late_cancellation_by_client", &actorID, map[string]any{"previous_status": currentStatus})

				// AUTOMATED BANNING POLICY CHECK
				// Fetch client stats after event is recorded (so count includes current cancel)
				// Use all-time stats - no rolling window forgiveness for returning clients
				clientStats, err := s.repo.GetClientBookingStats(ctx, currentBooking.ClientID, time.Time{})
				if err != nil {
					slog.Warn("failed to get client stats for ban check", "error", err)
				} else {
					shouldBan := false
					banReason := ""

					// New client (0 completed bookings): ban immediately on 1st late cancel
					if clientStats.TotalCompleted == 0 {
						shouldBan = true
						banReason = "First-time client late cancellation"
					} else if clientStats.TotalLateCancellations >= 3 {
						// Returning client: ban on 3rd late cancellation
						shouldBan = true
						banReason = fmt.Sprintf("Returning client exceeded 3 late cancellations (%d total)", clientStats.TotalLateCancellations)
					}

					if shouldBan && s.userRepo != nil {
						slog.Warn("SYSTEM BAN: Banning client", "client_id", currentBooking.ClientID, "reason", banReason)
						if err := s.userRepo.BanUserSystem(ctx, currentBooking.ClientID, banReason); err != nil {
							slog.Error("error banning client", "client_id", currentBooking.ClientID, "error", err)
						} else {
							// Notify all admins about the system ban
							s.notifyAdminsOfBan(ctx, currentBooking.ClientID, banReason)
						}
					}
				}
			}
		}
	}

	var cancelledBy *string
	var cancellationReason *string
	if status == "cancelled" {
		cancelledBy = &actorRole
		cancellationReason = req.CancellationReason
	}

	// COMMISSION CALCULATION: For "completed" status, calculate earnings and use CompleteBooking
	if status == "completed" {
		now := time.Now()
		b, err := s.repo.GetByBookingID(ctx, bookingID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch booking for completion: %w", err)
		}

		var therapistEarnings, platformFee *float64
		if b.ServiceID != nil && s.serviceRepo != nil {
			if svc, err := s.serviceRepo.GetByID(ctx, *b.ServiceID); err == nil && svc.TherapistCommission != nil {
				earnings := CalculateCommission(*svc.TherapistCommission, svc.BasePrice, svc.DurationMinutes, b.DurationMinutes)
				therapistEarnings = &earnings
				if b.FinalTotal != nil {
					fee := *b.FinalTotal - earnings
					platformFee = &fee
				}
			}
		}

		if err := s.repo.CompleteBooking(ctx, bookingID, therapistEarnings, platformFee, now); err != nil {
			return nil, err
		}

		// Credit therapist wallet
		if therapistEarnings != nil {
			// We pass nil for ledgerEntryID as the repo handles it or we'll rely on the transaction created within CreditEarning
			if s.walletService != nil {
				if err := s.walletService.CreditEarning(ctx, actorID, bookingID, *therapistEarnings, nil); err != nil {
					slog.Warn("failed to credit wallet on manual completion", "therapist_id", actorID, "booking_id", bookingID, "error", err)
					// We don't fail the request here as the booking is already completed, but we log the error.
					// Ideally this should be robust against failures (e.g. retry queue).
				}
			} else {
				slog.Warn("wallet service not available for manual completion credit", "booking_id", bookingID)
			}
		}

		// Record event
		eventMeta := map[string]any{"completed_by": actorRole}
		if therapistEarnings != nil {
			eventMeta["therapist_earnings"] = *therapistEarnings
		}
		if platformFee != nil {
			eventMeta["platform_fee"] = *platformFee
		}
		_ = s.repo.InsertEvent(ctx, bookingID, "completed", &actorID, eventMeta)
	} else {
		if err := s.repo.UpdateStatus(ctx, bookingID, actorID, actorRole, status, cancelledBy, cancellationReason); err != nil {
			return nil, err
		}
	}

	if status == "cancelled" {
		// Remove from assignment queue immediately
		_ = s.queueRepo.Remove(ctx, bookingID)

		// Cancel associated rides
		if s.logisticsService != nil {
			go func() {
				if err := s.logisticsService.CancelRideForBooking(context.Background(), bookingID); err != nil {
					slog.Error("UpdateStatus: failed to cancel ride for booking", "booking_id", bookingID, "error", err)
				}
			}()
		}

		// Cancel all pending offers and notify therapists
		if s.offerRepo != nil {
			cancelledOffers, err := s.offerRepo.CancelOffers(ctx, bookingID)
			if err != nil {
				slog.Warn("failed to cancel offers for booking", "booking_id", bookingID, "error", err)
			} else {
				for _, o := range cancelledOffers {
					// Broadcast offer cancellation to therapist
					_ = broadcaster.BroadcastToUser(o.TherapistID, "offer_cancelled", map[string]any{
						"offer_id":   o.OfferID,
						"booking_id": o.BookingID,
						"reason":     "booking_cancelled",
					})
				}
			}
		}

		// Notify Assigned Therapist (Push)
		// We need to fetch the booking again to check if a therapist was assigned
		// Note: We use GetByBookingID to bypass any user scoping
		if b, err := s.repo.GetByBookingID(ctx, bookingID); err == nil && b.TherapistID != nil {
			if s.notificationService != nil {
				clientName, _, _, _ := s.FetchClientInfo(ctx, b.ClientID)
				msg := "A booking assigned to you has been cancelled."
				if clientName != "" {
					msg = fmt.Sprintf("Booking with %s has been cancelled.", clientName)
				}
				if b.ScheduledStart != nil {
					msg += fmt.Sprintf(" (%s)", b.ScheduledStart.Format("Jan 02, 3:04 PM"))
				}

				go s.notificationService.Create(context.WithoutCancel(ctx), &model.CreateNotificationRequest{
					UserID:  *b.TherapistID,
					Type:    "booking_status",
					Title:   "Booking Cancelled",
					Message: msg,
					Data: map[string]interface{}{
						"booking_id": b.BookingID,
						"status":     "cancelled",
					},
				})
			}
		}
	}

	// Broadcast updated booking to client and therapist
	s.broadcastBookingUpdate(ctx, bookingID, status, actorRole)

	// Send system message for key status transitions
	if s.messageService != nil && status != "cancelled" {
		if b, err := s.repo.GetByBookingID(ctx, bookingID); err == nil && b.TherapistID != nil {
			var sysMsg string
			switch status {
			case "on_the_way":
				sysMsg = "Therapist is on the way."
			case "arrived":
				sysMsg = "Therapist has arrived."
			case "in_progress":
				sysMsg = "Session started."
			case "completed":
				sysMsg = "Session completed. Thank you!"
			}
			if sysMsg != "" {
				go s.sendBookingSystemMessage(b.ClientID, *b.TherapistID, sysMsg)
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


// UnassignTherapist allows a therapist to cancel their assignment. The booking is
// reset to pending status and re-queued for a new therapist. The cancelling therapist
// is prevented from being re-matched to this booking.
// Policy: 3 unassignments/day → admin notification; 5/week → auto-suspend.
// UnassignTherapist allows a therapist or admin to cancel an assignment. The booking is
// reset to pending status and re-queued for a new therapist. The cancelling therapist
// is prevented from being re-matched to this booking.
// Policy: 3 unassignments/day → admin notification; 5/week → auto-suspend.
func (s *BookingService) UnassignTherapist(ctx context.Context, bookingID, actorID int64, actorRole string, reason *string) error {
	// Fetch booking to verify therapist is assigned
	b, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("booking not found: %w", err)
	}

	var targetTherapistID int64
	if actorRole == model.RoleAdmin {
		// Admin can unassign any therapist
		if b.TherapistID == nil {
			return fmt.Errorf("booking has no assigned therapist")
		}
		targetTherapistID = *b.TherapistID
	} else if actorRole == model.RoleTherapist {
		// Therapist can only unassign themselves
		if b.TherapistID == nil || *b.TherapistID != actorID {
			return fmt.Errorf("unauthorized: therapist is not assigned to this booking")
		}
		targetTherapistID = actorID
	} else {
		return fmt.Errorf("unauthorized role for unassignment")
	}

	// Only allow unassignment from certain statuses
	allowedStatuses := map[string]bool{model.BookingStatusAssigned: true, model.BookingStatusOnTheWay: true}
	if !allowedStatuses[strings.ToLower(b.Status)] {
		return fmt.Errorf("cannot unassign from status: %s", b.Status)
	}

	// Create a "rejected" offer record to prevent this therapist from being re-matched
	if s.offerRepo != nil {
		rejectOffer := &model.BookingOffer{
			BookingID:   bookingID,
			TherapistID: targetTherapistID,
			Status:      model.BookingOfferStatusDeclined,
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now(), // Already expired/rejected
		}
		if err := s.offerRepo.Create(ctx, rejectOffer); err != nil {
			slog.Warn("UnassignTherapist: failed to create rejection record", "error", err)
			// Continue anyway; the main logic should still work
		}
	}

	// Prepare metadata for event log
	metadata := make(map[string]any)
	if reason != nil {
		metadata["reason"] = *reason
	}
	metadata["unassigned_by_role"] = actorRole
	metadata["target_therapist_id"] = targetTherapistID

	// Reset booking: clear therapist and set status to pending
	// Pass actorID (who performed the action) to repo
	if err := s.repo.UnassignTherapist(ctx, bookingID, &actorID, metadata); err != nil {
		return fmt.Errorf("failed to unassign therapist: %w", err)
	}

	// Re-queue for assignment
	_ = s.queueRepo.Enqueue(ctx, bookingID)

	slog.Info("therapist unassigned", "therapist_id", targetTherapistID, "booking_id", bookingID)

	// Cancel associated rides when therapist is unassigned
	if s.logisticsService != nil {
		go func() {
			if err := s.logisticsService.CancelRideForBooking(context.Background(), bookingID); err != nil {
				slog.Error("UnassignTherapist: failed to cancel ride", "booking_id", bookingID, "error", err)
			}
		}()
	}

	// Broadcast update to client and therapist
	s.broadcastBookingUpdate(ctx, bookingID, "pending", "therapist")

	// Notify client that therapist cancelled
	if s.notificationService != nil {
		_, _ = s.notificationService.Create(ctx, &model.CreateNotificationRequest{
			UserID:  b.ClientID,
			Type:    "therapist_cancelled",
			Title:   "Therapist Unavailable",
			Message: "Your assigned therapist is no longer available. We are finding a new one for you.",
		})
	}

	// === UNASSIGNMENT POLICY CHECK ===
	// Check daily and weekly limits for self-unassignments (only if actor is therapist)
	if actorRole == model.RoleTherapist {
		s.checkUnassignmentLimits(ctx, actorID)
	}

	return nil
}

// checkUnassignmentLimits checks if therapist exceeded daily (3) or weekly (5) unassignment limits.
// Daily limit → notify admins. Weekly limit → auto-suspend therapist.
func (s *BookingService) checkUnassignmentLimits(ctx context.Context, therapistID int64) {
	// Use Philippines timezone (UTC+8) for day/week boundaries
	loc := time.FixedZone("Asia/Manila", 8*60*60)
	now := time.Now().In(loc)

	// Calculate start of today (midnight PH time)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).UTC()

	// Calculate start of week (Sunday 12am PH time)
	weekday := int(now.Weekday())
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-weekday, 0, 0, 0, 0, loc).UTC()

	// Count daily unassignments (where actor_id = therapist, meaning self-unassign)
	dailyCount, err := s.repo.CountEventsByTypeAndActor(ctx, therapistID, "therapist_unassigned", startOfDay)
	if err != nil {
		slog.Warn("checkUnassignmentLimits: failed to count daily", "error", err)
		return
	}

	// Count weekly unassignments
	weeklyCount, err := s.repo.CountEventsByTypeAndActor(ctx, therapistID, "therapist_unassigned", startOfWeek)
	if err != nil {
		slog.Warn("checkUnassignmentLimits: failed to count weekly", "error", err)
		return
	}

	slog.Debug("checkUnassignmentLimits", "therapist_id", therapistID, "daily", dailyCount, "weekly", weeklyCount)

	// Weekly limit check (5 unassignments → auto-suspend)
	if weeklyCount >= 5 {
		slog.Warn("therapist suspended for exceeding weekly unassignment limit", "therapist_id", therapistID, "weekly_count", weeklyCount)

		// Suspend therapist (set accept_assignments = false)
		if s.therapistRepo != nil {
			if err := s.therapistRepo.UpdateProfile(ctx, therapistID, map[string]interface{}{
				"accept_assignments": false,
			}); err != nil {
				slog.Error("checkUnassignmentLimits: failed to suspend therapist", "therapist_id", therapistID, "error", err)
			} else {
				// Also set user account_status to suspended with reason
				if s.userRepo != nil {
					_ = s.userRepo.SuspendUserSystem(ctx, therapistID, "Weekly unassignment limit reached (5+)")
				}

				// Record suspension event
				_ = s.repo.InsertEvent(ctx, 0, "therapist_suspended_auto", &therapistID, map[string]any{
					"reason":        "weekly_unassignment_limit",
					"weekly_count":  weeklyCount,
					"limit":         5,
				})

				// Notify admins (critical alert)
				s.notifyAdminsOfTherapistSuspension(ctx, therapistID, weeklyCount)
			}
		}
		return // Weekly limit is more severe, skip daily check notification
	}

	// Daily limit check (3 unassignments → notify admins)
	if dailyCount >= 3 {
		slog.Warn("therapist hit daily unassignment limit", "therapist_id", therapistID, "daily_count", dailyCount)
		s.notifyAdminsOfDailyUnassignmentLimit(ctx, therapistID, dailyCount)
	}
}

// notifyAdminsOfDailyUnassignmentLimit notifies admins when a therapist hits daily limit
func (s *BookingService) notifyAdminsOfDailyUnassignmentLimit(ctx context.Context, therapistID int64, count int) {
	if s.notificationService == nil || s.userRepo == nil {
		return
	}

	// Fetch therapist name
	therapistName := "Unknown"
	if s.db != nil {
		var name string
		_ = s.db.QueryRow(ctx, `SELECT COALESCE(full_name, 'Unknown') FROM users WHERE user_id = $1`, therapistID).Scan(&name)
		if name != "" {
			therapistName = name
		}
	}

	// Notify all admins
	admins, err := s.userRepo.ListUsers(ctx, "admin")
	if err != nil {
		slog.Warn("notifyAdminsOfDailyUnassignmentLimit: failed to list admins", "error", err)
		return
	}

	for _, admin := range admins {
		_, _ = s.notificationService.Create(ctx, &model.CreateNotificationRequest{
			UserID:  int64(admin.UserID),
			Type:    "therapist_unassignment_warning",
			Title:   "Therapist Unassignment Warning",
			Message: fmt.Sprintf("Therapist %s (ID: %d) has unassigned from %d bookings today.", therapistName, therapistID, count),
		})
	}
}

// notifyAdminsOfTherapistSuspension notifies admins when a therapist is auto-suspended
func (s *BookingService) notifyAdminsOfTherapistSuspension(ctx context.Context, therapistID int64, weeklyCount int) {
	if s.notificationService == nil || s.userRepo == nil {
		return
	}

	// Fetch therapist name
	therapistName := "Unknown"
	if s.db != nil {
		var name string
		_ = s.db.QueryRow(ctx, `SELECT COALESCE(full_name, 'Unknown') FROM users WHERE user_id = $1`, therapistID).Scan(&name)
		if name != "" {
			therapistName = name
		}
	}

	// Notify all admins
	admins, err := s.userRepo.ListUsers(ctx, "admin")
	if err != nil {
		slog.Warn("notifyAdminsOfTherapistSuspension: failed to list admins", "error", err)
		return
	}

	for _, admin := range admins {
		_, _ = s.notificationService.Create(ctx, &model.CreateNotificationRequest{
			UserID:  int64(admin.UserID),
			Type:    "therapist_suspended",
			Title:   "CRITICAL: Therapist Auto-Suspended",
			Message: fmt.Sprintf("Therapist %s (ID: %d) has been automatically suspended after %d unassignments this week. Manual re-enablement required.", therapistName, therapistID, weeklyCount),
		})
	}

	// Also notify the therapist
	_, _ = s.notificationService.Create(ctx, &model.CreateNotificationRequest{
		UserID:  therapistID,
		Type:    "account_suspended",
		Title:   "Account Suspended",
		Message: "Your account has been temporarily suspended due to frequent booking unassignments. Please contact support for more information.",
	})
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

	// Cancel all pending offers (since we manually assigned) and notify other therapists
	if s.offerRepo != nil {
		cancelledOffers, err := s.offerRepo.CancelOffers(ctx, bookingID)
		if err != nil {
			slog.Warn("AssignTherapist: failed to cancel offers", "error", err)
		} else {
			for _, o := range cancelledOffers {
				// Don't notify the assigned therapist that their offer was cancelled (redundant/confusing)
				// actually, if they had an offer, it's effectively "accepted" by this assignment, but 
				// since we are forcing assignment, "cancelled" is okay, or we could just skip them.
				// simpler to just tell them "offer_cancelled" if we want to be strict, OR 
				// we trust the booking update will override it.
				// Let's filter out the assigned therapist ID if they had an offer.
				if o.TherapistID != therapistID {
					_ = broadcaster.BroadcastToUser(o.TherapistID, "offer_cancelled", map[string]any{
						"offer_id":   o.OfferID,
						"booking_id": o.BookingID,
						"reason":     "manual_assignment",
					})
				}
			}
		}
	}

	// Fetch updated booking
	b, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	// Broadcast update to Client and Therapist so their UIs update
	// We use the same broadcast logic as UpdateStatus/AcceptOffer
	s.broadcastBookingUpdate(ctx, bookingID, "assigned", "admin")

	// Notify Therapist of Manual Assignment
	if s.notificationService != nil {
		go func() {
			// Fetch details for message
			var svcName = "Service"
			if b.ServiceID != nil && s.serviceRepo != nil {
				if svc, err := s.serviceRepo.GetByID(context.Background(), *b.ServiceID); err == nil {
					svcName = svc.Name
				}
			}
			var location = "Client Location"
			if b.AddressID != nil && s.addressRepo != nil {
				if addr, err := s.addressRepo.GetByIDUnsafe(context.Background(), *b.AddressID); err == nil {
					location = addr.City
				}
			}
			
			title := fmt.Sprintf("Assigned: %s", svcName)
			timeStr := "now"
			if b.ScheduledStart != nil {
				timeStr = b.ScheduledStart.Format("3:04 PM")
			}
			msg := fmt.Sprintf("You have been assigned a booking for %s in %s.", timeStr, location)

			_, _ = s.notificationService.Create(context.Background(), &model.CreateNotificationRequest{
				UserID:  therapistID,
				Type:    "booking_status",
				Title:   title,
				Message: msg,
				Data: map[string]interface{}{
					"booking_id": b.BookingID,
					"status":     "assigned",
				},
			})
		}()
	}

	// Trigger logistics orchestration (ride creation) asynchronously
	if s.logisticsService != nil {
		go func() {
			// Use background context to avoid cancellation
			if err := s.logisticsService.HandleBookingAssigned(context.Background(), bookingID); err != nil {
				slog.Error("Failed to handle logistics for assigned booking", "booking_id", bookingID, "error", err)
			}
		}()
	}

	return b, nil
}

// StartSession attempts to start a session for a booking. It requires that
// the therapist has arrived (status == 'arrived' or therapist_arrived_at set).
// actorRole is used for permission checks (typically 'client').
// startTime is optional - if provided (e.g. for offline sync), it will be used as actual_start.
func (s *BookingService) StartSession(ctx context.Context, bookingID, actorID int64, actorRole string, startTime *time.Time) (*model.Booking, error) {
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

	// Use provided startTime for offline sync, or default to now
	var start time.Time
	if startTime != nil {
		start = *startTime
	} else {
		start = time.Now()
	}
	start = start.UTC() // Ensure UTC storage

	if err := s.repo.UpdateStatusWithTime(ctx, bookingID, actorID, actorRole, "in_progress", nil, nil, &start); err != nil {
		return nil, err
	}

	// Broadcast update
	s.broadcastBookingUpdate(ctx, bookingID, "in_progress", actorRole)

	return s.repo.GetByBookingID(ctx, bookingID)
}

// PauseSession pauses an in-progress booking session. Only therapists can pause.
func (s *BookingService) PauseSession(ctx context.Context, bookingID, actorID int64, actorRole string) (*model.Booking, error) {
	// Only therapists can pause sessions
	if actorRole != "therapist" && actorRole != "admin" {
		return nil, fmt.Errorf("only therapist or admin can pause a session")
	}

	// Fetch booking without scoping
	b, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	// Validate booking is in_progress and not already paused
	if b.Status != "in_progress" {
		return nil, fmt.Errorf("can only pause in_progress sessions")
	}
	if b.CurrentPauseStart != nil {
		return nil, fmt.Errorf("session is already paused")
	}

	// Record pause event and update booking
	now := time.Now().UTC()
	actor := actorID
	_ = s.repo.InsertEvent(ctx, bookingID, "session_paused", &actor, map[string]any{
		"paused_by_role": actorRole,
	})

	// Update current_pause_start on the booking
	if err := s.repo.SetPauseStart(ctx, bookingID, &now); err != nil {
		return nil, err
	}

	// Broadcast update
	s.broadcastBookingUpdate(ctx, bookingID, "in_progress", actorRole)

	return s.repo.GetByBookingID(ctx, bookingID)
}

// ResumeSession resumes a paused booking session. Only therapists can resume.
func (s *BookingService) ResumeSession(ctx context.Context, bookingID, actorID int64, actorRole string) (*model.Booking, error) {
	// Only therapists can resume sessions
	if actorRole != "therapist" && actorRole != "admin" {
		return nil, fmt.Errorf("only therapist or admin can resume a session")
	}

	// Fetch booking
	b, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	// Validate booking is paused
	if b.CurrentPauseStart == nil {
		return nil, fmt.Errorf("session is not paused")
	}

	// Calculate pause duration and add to total
	// Ensure we calculate duration using UTC constant reference
	pauseDuration := int(time.Now().UTC().Sub(*b.CurrentPauseStart).Seconds())
	newTotalPaused := b.TotalPausedSeconds + pauseDuration

	// Record resume event
	actor := actorID
	_ = s.repo.InsertEvent(ctx, bookingID, "session_resumed", &actor, map[string]any{
		"pause_duration_seconds": pauseDuration,
	})

	// Clear pause start and update total paused
	if err := s.repo.ClearPauseAndAddDuration(ctx, bookingID, newTotalPaused); err != nil {
		return nil, err
	}

	// Broadcast update
	s.broadcastBookingUpdate(ctx, bookingID, "in_progress", actorRole)

	return s.repo.GetByBookingID(ctx, bookingID)
}

// ExtendSession extends an in-progress booking session by the given additional minutes.
// Price is calculated at 300 PHP per 30-minute block.
func (s *BookingService) ExtendSession(ctx context.Context, bookingID, actorID int64, actorRole string, additionalMinutes int) (*model.Booking, error) {
	// Validate extension duration (must be positive and in 15-minute increments)
	if additionalMinutes <= 0 {
		return nil, NewValidationError("invalid_extension", "additional_minutes must be positive", map[string]string{"additional_minutes": "must be > 0"})
	}
	if additionalMinutes%15 != 0 {
		return nil, NewValidationError("invalid_extension", "additional_minutes must be in 15-minute increments", map[string]string{"additional_minutes": "must be multiple of 15"})
	}

	// Only clients and therapists can extend (or admins)
	if actorRole != "client" && actorRole != "therapist" && actorRole != "admin" {
		return nil, fmt.Errorf("unauthorized role")
	}

	// Fetch booking
	b, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	// Validate booking is in_progress
	if b.Status != "in_progress" {
		return nil, fmt.Errorf("can only extend in_progress sessions")
	}

	// Calculate additional cost based on service rate
	if b.ServiceID == nil {
		return nil, fmt.Errorf("booking has no service ID")
	}
	svc, err := s.serviceRepo.GetByID(ctx, *b.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch service details: %w", err)
	}
	if svc.DurationMinutes == 0 {
		return nil, fmt.Errorf("service has zero duration, cannot calculate extension rate")
	}

	ratePerMinute := svc.BasePrice / float64(svc.DurationMinutes)
	additionalCost := ratePerMinute * float64(additionalMinutes)

	// Calculate new duration and totals
	newDuration := b.DurationMinutes + additionalMinutes
	var newRawTotal, newFinalTotal *float64
	if b.RawTotal != nil {
		v := *b.RawTotal + additionalCost
		newRawTotal = &v
	} else {
		newRawTotal = &additionalCost
	}
	// Compute final total (raw - discount)
	newFinalTotal = computeFinal(newRawTotal, b.Discount)

	// Update payment breakdown with new extension cost
	var updatedBreakdownJSON []byte
	if len(b.PaymentBreakdownJSON) > 0 {
		var breakdown model.PaymentBreakdown
		if err := json.Unmarshal(b.PaymentBreakdownJSON, &breakdown); err == nil {
			breakdown.ExtensionsTotal += additionalCost
			updatedBreakdownJSON, _ = json.Marshal(breakdown)
		}
	} else {
		// Create new breakdown if missing (for old bookings)
		breakdown := model.PaymentBreakdown{
			BasePrice:       svc.BasePrice,
			DurationMarkup:  0, // Unknown for old bookings
			ExtensionsTotal: additionalCost,
			ServiceSnapshot: fmt.Sprintf("%s (%dmin)", svc.Name, newDuration),
		}
		updatedBreakdownJSON, _ = json.Marshal(breakdown)
	}

	// Update booking in database
	_, err = s.db.Exec(ctx, `
		UPDATE bookings
		SET duration_minutes = $1, raw_total = $2, final_total = $3, payment_breakdown = $4, updated_at = NOW()
		WHERE booking_id = $5
	`, newDuration, newRawTotal, newFinalTotal, updatedBreakdownJSON, bookingID)
	if err != nil {
		return nil, err
	}

	// Record extension event
	actor := actorID
	_ = s.repo.InsertEvent(ctx, bookingID, "session_extended", &actor, map[string]any{
		"additional_minutes": additionalMinutes,
		"additional_cost":    additionalCost,
		"new_duration":       newDuration,
		"extended_by_role":   actorRole,
	})

	// Broadcast update
	s.broadcastBookingUpdate(ctx, bookingID, "in_progress", actorRole)

	// Reschedule return ride to reflect extended duration
	if s.logisticsService != nil {
		go func() {
			if err := s.logisticsService.UpdateRideForBooking(context.Background(), bookingID); err != nil {
				slog.Error("ExtendSession: failed to reschedule rides", "booking_id", bookingID, "error", err)
			}
		}()
	}

	return s.repo.GetByBookingID(ctx, bookingID)
}

// RequestExtension creates a pending extension request (client proposes, therapist confirms)
func (s *BookingService) RequestExtension(ctx context.Context, bookingID, actorID int64, actorRole string, additionalMinutes int) (*model.ExtensionRequest, error) {
	slog.Debug("RequestExtension", "booking_id", bookingID, "actor_id", actorID, "actor_role", actorRole, "additional_minutes", additionalMinutes)
	
	// Validate extension duration (must be positive and in 15-minute increments)
	if additionalMinutes <= 0 {
		return nil, NewValidationError("invalid_extension", "additional_minutes must be positive", map[string]string{"additional_minutes": "must be > 0"})
	}
	if additionalMinutes%15 != 0 {
		return nil, NewValidationError("invalid_extension", "additional_minutes must be in 15-minute increments", map[string]string{"additional_minutes": "must be multiple of 15"})
	}

	// Only clients can request extensions (therapists and admins can directly extend via ExtendSession)
	if actorRole != "client" {
		return nil, fmt.Errorf("only clients can request extensions; use ExtendSession for direct extension")
	}

	// Fetch booking
	b, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil {
		slog.Warn("RequestExtension: GetByBookingID error", "error", err)
		return nil, err
	}
	
	slog.Debug("RequestExtension: booking state", "status", b.Status, "client_id", b.ClientID, "therapist_id", b.TherapistID)

	// Validate booking is in_progress
	if b.Status != "in_progress" {
		return nil, fmt.Errorf("can only request extension for in_progress sessions (current status: %s)", b.Status)
	}

	// Check for existing pending request
	if s.extensionRequestRepo != nil {
		existing, _ := s.extensionRequestRepo.GetPendingByBookingID(ctx, bookingID)
		if existing != nil {
			slog.Debug("RequestExtension: pending request exists", "request_id", existing.RequestID)
			return nil, NewValidationError("pending_exists", "an extension request is already pending", map[string]string{"booking_id": "pending request exists"})
		}
	}

	// Calculate additional cost based on service rate
	if b.ServiceID == nil {
		slog.Warn("RequestExtension: booking has no service ID")
		return nil, fmt.Errorf("booking has no service ID")
	}
	svc, err := s.serviceRepo.GetByID(ctx, *b.ServiceID)
	if err != nil {
		slog.Warn("RequestExtension: failed to fetch service", "error", err)
		return nil, fmt.Errorf("failed to fetch service details: %w", err)
	}
	if svc.DurationMinutes == 0 {
		slog.Warn("RequestExtension: service has zero duration")
		return nil, fmt.Errorf("service has zero duration, cannot calculate extension rate")
	}

	ratePerMinute := svc.BasePrice / float64(svc.DurationMinutes)
	additionalCost := ratePerMinute * float64(additionalMinutes)

	// Create extension request
	req := &model.ExtensionRequest{
		BookingID:        bookingID,
		RequestedMinutes: additionalMinutes,
		AdditionalCost:   additionalCost,
		Status:           model.ExtensionStatusPending,
		RequestedBy:      &actorID,
	}

	if s.extensionRequestRepo != nil {
		if err := s.extensionRequestRepo.Create(ctx, req); err != nil {
			slog.Error("RequestExtension: failed to create request in DB", "error", err)
			return nil, err
		}
		slog.Info("extension request created", "request_id", req.RequestID)
	}

	// Record event
	_ = s.repo.InsertEvent(ctx, bookingID, "extension_requested", &actorID, map[string]any{
		"request_id":         req.RequestID,
		"requested_minutes":  additionalMinutes,
		"additional_cost":    additionalCost,
	})

	// Broadcast to therapist
	if b.TherapistID != nil {
		_ = broadcaster.BroadcastToUser(*b.TherapistID, "extension:requested", map[string]any{
			"booking_id":        bookingID,
			"request_id":        req.RequestID,
			"requested_minutes": additionalMinutes,
			"additional_cost":   additionalCost,
		})
	}

	return req, nil
}

// AcceptExtension accepts a pending extension request and updates the booking
func (s *BookingService) AcceptExtension(ctx context.Context, requestID, actorID int64, actorRole string, note *string) (*model.Booking, error) {
	// Only therapists and admins can accept
	if actorRole != "therapist" && actorRole != "admin" {
		return nil, fmt.Errorf("unauthorized role")
	}

	if s.extensionRequestRepo == nil {
		return nil, fmt.Errorf("extension request repository not configured")
	}

	// Fetch request
	req, err := s.extensionRequestRepo.GetByID(ctx, requestID)
	if err != nil {
		return nil, err
	}

	if req.Status != model.ExtensionStatusPending {
		return nil, NewValidationError("invalid_status", "extension request is not pending", map[string]string{"status": req.Status})
	}

	// Fetch booking
	b, err := s.repo.GetByBookingID(ctx, req.BookingID)
	if err != nil {
		return nil, err
	}

	// Apply extension to booking (similar to ExtendSession)
	newDuration := b.DurationMinutes + req.RequestedMinutes
	var newRawTotal, newFinalTotal *float64
	if b.RawTotal != nil {
		v := *b.RawTotal + req.AdditionalCost
		newRawTotal = &v
	} else {
		newRawTotal = &req.AdditionalCost
	}
	newFinalTotal = computeFinal(newRawTotal, b.Discount)

	// Update payment breakdown with new extension cost
	var updatedBreakdownJSON []byte
	if len(b.PaymentBreakdownJSON) > 0 {
		var breakdown model.PaymentBreakdown
		if err := json.Unmarshal(b.PaymentBreakdownJSON, &breakdown); err == nil {
			breakdown.ExtensionsTotal += req.AdditionalCost
			updatedBreakdownJSON, _ = json.Marshal(breakdown)
		}
	} else {
		// Create new breakdown if missing (for old bookings)
		// Try to get service info for base price
		var basePrice float64
		var serviceName string
		if b.ServiceID != nil {
			if svc, err := s.serviceRepo.GetByID(ctx, *b.ServiceID); err == nil {
				basePrice = svc.BasePrice
				serviceName = svc.Name
			}
		}
		breakdown := model.PaymentBreakdown{
			BasePrice:       basePrice,
			DurationMarkup:  0, // Unknown for old bookings
			ExtensionsTotal: req.AdditionalCost,
			ServiceSnapshot: fmt.Sprintf("%s (%dmin)", serviceName, newDuration),
		}
		updatedBreakdownJSON, _ = json.Marshal(breakdown)
	}

	// Calculate the extension wait time (gap between when session should have ended and now)
	// This ensures the user gets their full extended duration
	newExtensionWait := b.ExtensionWaitSeconds
	if b.ActualStart != nil {
		// Calculate when the session should have ended based on current duration
		expectedEnd := b.ActualStart.Add(time.Duration(b.DurationMinutes) * time.Minute)
		// Account for pauses and existing extension wait
		expectedEnd = expectedEnd.Add(time.Duration(b.TotalPausedSeconds+b.ExtensionWaitSeconds) * time.Second)
		
		now := time.Now().UTC()
		if now.After(expectedEnd) {
			// There's a gap - user waited for approval after session should have ended
			gapSeconds := int(now.Sub(expectedEnd).Seconds())
			newExtensionWait += gapSeconds
			slog.Debug("AcceptExtension: calculated gap time", "gap_seconds", gapSeconds, "booking_id", req.BookingID)
		}
	}

	// Update booking with new duration, extension wait time, and payment breakdown
	_, err = s.db.Exec(ctx, `
		UPDATE bookings
		SET duration_minutes = $1, raw_total = $2, final_total = $3, extension_wait_seconds = $4, payment_breakdown = $5, updated_at = NOW()
		WHERE booking_id = $6
	`, newDuration, newRawTotal, newFinalTotal, newExtensionWait, updatedBreakdownJSON, req.BookingID)
	if err != nil {
		return nil, err
	}

	// Update request status
	if err := s.extensionRequestRepo.UpdateStatus(ctx, requestID, model.ExtensionStatusAccepted, actorID, note); err != nil {
		return nil, err
	}

	// Record event
	_ = s.repo.InsertEvent(ctx, req.BookingID, "extension_accepted", &actorID, map[string]any{
		"request_id":         requestID,
		"additional_minutes": req.RequestedMinutes,
		"additional_cost":    req.AdditionalCost,
		"new_duration":       newDuration,
	})

	// Broadcast to client
	_ = broadcaster.BroadcastToUser(b.ClientID, "extension:accepted", map[string]any{
		"booking_id":        req.BookingID,
		"request_id":        requestID,
		"additional_minutes": req.RequestedMinutes,
		"new_duration":      newDuration,
	})

	// Broadcast booking update
	s.broadcastBookingUpdate(ctx, req.BookingID, "in_progress", actorRole)

	return s.repo.GetByBookingID(ctx, req.BookingID)
}

// RejectExtension rejects a pending extension request
func (s *BookingService) RejectExtension(ctx context.Context, requestID, actorID int64, actorRole string, note *string) error {
	// Only therapists and admins can reject
	if actorRole != "therapist" && actorRole != "admin" {
		return fmt.Errorf("unauthorized role")
	}

	if s.extensionRequestRepo == nil {
		return fmt.Errorf("extension request repository not configured")
	}

	// Fetch request
	req, err := s.extensionRequestRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}

	if req.Status != model.ExtensionStatusPending {
		return NewValidationError("invalid_status", "extension request is not pending", map[string]string{"status": req.Status})
	}

	// Fetch booking for client ID
	b, err := s.repo.GetByBookingID(ctx, req.BookingID)
	if err != nil {
		return err
	}

	// Update request status
	if err := s.extensionRequestRepo.UpdateStatus(ctx, requestID, model.ExtensionStatusRejected, actorID, note); err != nil {
		return err
	}

	// Record event
	_ = s.repo.InsertEvent(ctx, req.BookingID, "extension_rejected", &actorID, map[string]any{
		"request_id": requestID,
		"note":       note,
	})

	// Broadcast to client
	_ = broadcaster.BroadcastToUser(b.ClientID, "extension:rejected", map[string]any{
		"booking_id": req.BookingID,
		"request_id": requestID,
		"note":       note,
	})

	return nil
}

// CancelExtension allows a client to cancel their own pending extension request
func (s *BookingService) CancelExtension(ctx context.Context, requestID, actorID int64, actorRole string) error {
	// Only clients can cancel their own requests
	if actorRole != "client" {
		return fmt.Errorf("only clients can cancel their own extension requests")
	}

	if s.extensionRequestRepo == nil {
		return fmt.Errorf("extension request repository not configured")
	}

	// Fetch request
	req, err := s.extensionRequestRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}

	if req.Status != model.ExtensionStatusPending {
		return NewValidationError("invalid_status", "extension request is not pending", map[string]string{"status": req.Status})
	}

	// Verify the client is the one who made the request
	if req.RequestedBy == nil || *req.RequestedBy != actorID {
		return fmt.Errorf("you can only cancel your own extension requests")
	}

	// Update request status to cancelled (using rejected status for simplicity)
	cancelled := "cancelled"
	if err := s.extensionRequestRepo.UpdateStatus(ctx, requestID, cancelled, actorID, nil); err != nil {
		return err
	}

	// Record event
	_ = s.repo.InsertEvent(ctx, req.BookingID, "extension_cancelled", &actorID, map[string]any{
		"request_id": requestID,
	})

	slog.Info("extension request cancelled", "client_id", actorID, "request_id", requestID)
	return nil
}

// GetPendingExtensionRequest returns the pending extension request for a booking, if any
func (s *BookingService) GetPendingExtensionRequest(ctx context.Context, bookingID int64) (*model.ExtensionRequest, error) {
	if s.extensionRequestRepo == nil {
		return nil, nil
	}
	return s.extensionRequestRepo.GetPendingByBookingID(ctx, bookingID)
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

	// CRITICAL: Lock the booking row to prevent concurrent offer acceptance race condition.
	// This ensures only one therapist can accept the offer at a time.
	var bookingToCheck *model.Booking
	if tx != nil {
		bookingToCheck, err = s.repo.GetByBookingIDForUpdateTx(ctx, tx, bookingID)
		if err != nil {
			return fmt.Errorf("failed to lock booking for validation: %w", err)
		}
	} else {
		// Fallback for unit tests without transaction
		bookingToCheck, err = s.repo.GetByBookingID(ctx, bookingID)
		if err != nil {
			return fmt.Errorf("failed to fetch booking for validation: %w", err)
		}
	}
	
	if bookingToCheck.Status != "pending" {
		return fmt.Errorf("booking is not pending (status=%s)", bookingToCheck.Status)
	}
	// Paranoid check: ensure total is positive
	if bookingToCheck.FinalTotal == nil || *bookingToCheck.FinalTotal <= 0 {
		return fmt.Errorf("booking has invalid total")
	}
	// Double-check therapist not already assigned (race prevention)
	if bookingToCheck.TherapistID != nil {
		return fmt.Errorf("booking already assigned to therapist %d", *bookingToCheck.TherapistID)
	}

	// Assign therapist using Tx
	if err := s.repo.AssignTherapistWithActorTx(ctx, tx, bookingID, therapistID, therapistID); err != nil {
		return err
	}

	// Mark therapist as "in field" (not at branch) since they're now assigned to a booking
	if s.therapistRepo != nil {
		if err := s.therapistRepo.SetAtBranch(ctx, therapistID, false); err != nil {
			slog.Warn("AcceptBookingOffer: failed to set at_branch=false", "therapist_id", therapistID, "error", err)
			// Non-fatal: continue with the assignment
		}
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

	// Automatically create a conversation between client and therapist + system message
	if b, err := s.repo.GetByBookingID(ctx, bookingID); err == nil && b != nil {
		go func() {
			convID, err := s.EnsureConversation(context.Background(), b.ClientID, therapistID)
			if err != nil {
				slog.Warn("AcceptBookingOffer: EnsureConversation failed", "error", err)
				return
			}
			if convID > 0 && s.messageService != nil {
				_ = s.messageService.SendSystemMessage(context.Background(), convID, "Therapist has been assigned to your booking.")
			}
		}()
	}

	// Fetch updated booking and broadcast to client and therapist so they see assignment in real-time
	if b, err := s.repo.GetByBookingID(ctx, bookingID); err == nil && b != nil {
		slog.Debug("AcceptBookingOffer: broadcasting booking:updated", "client_id", b.ClientID, "therapist_id", therapistID)
		
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
		var therapistName, therapistPhone, therapistGender, therapistPhoto string
		if b.TherapistID != nil {
			if s.therapistRepo != nil {
				if prof, err := s.therapistRepo.GetProfile(ctx, *b.TherapistID); err == nil {
					therapist = prof
				}
			}
			if s.db != nil {
				// Fetch therapist name, phone, gender, and photo from users table
				var userQuery = `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(gender, ''), COALESCE(profile_photo, '') FROM users WHERE user_id = $1`
				_ = s.db.QueryRow(ctx, userQuery, *b.TherapistID).Scan(&therapistName, &therapistPhone, &therapistGender, &therapistPhoto)
				slog.Debug("AcceptBookingOffer: fetched therapist details", "therapist_name", therapistName, "therapist_id", *b.TherapistID)
			}
		}

		// Fetch client details
		var clientName, clientPhone, clientPhoto, clientGender string
		if s.db != nil {
			clientQuery := `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
			_ = s.db.QueryRow(ctx, clientQuery, b.ClientID).Scan(&clientName, &clientPhone, &clientPhoto, &clientGender)
		}
		
		// Create enriched payload with therapist details
		enrichedPayload := bookingToMapWithTherapist(b, service, address, therapist, therapistName, therapistPhone, therapistPhoto, clientName, clientPhone, clientPhoto, clientGender, therapistGender)
		
		// Send persistent notification for assignment
		s.sendBookingNotification(ctx, b, "assigned", "therapist", therapistName)

		_ = broadcaster.BroadcastToUser(b.ClientID, "booking:updated", enrichedPayload)
		_ = broadcaster.BroadcastToUser(therapistID, "booking:updated", enrichedPayload)
	} else {
		slog.Warn("AcceptBookingOffer: failed to fetch booking for broadcast", "booking_id", bookingID, "error", err)
	}

	// Broadcast event
	_ = broadcaster.BroadcastToUser(therapistID, "offer_accepted", map[string]any{
		"offer_id":   offer.OfferID,
		"booking_id": bookingID,
	})

	// Broadcast expiration to other therapists
	for _, o := range expired {
		// Don't send expired event to the therapist who accepted (though they shouldn't be in the expired list anyway)
		if o.TherapistID != therapistID {
			_ = broadcaster.BroadcastToUser(o.TherapistID, "offer_expired", map[string]any{
				"offer_id":   o.OfferID,
				"booking_id": o.BookingID,
			})
		}
	}

	// Trigger logistics orchestration (ride creation) asynchronously
	if s.logisticsService != nil {
		go func() {
			if err := s.logisticsService.HandleBookingAssigned(context.Background(), bookingID); err != nil {
				slog.Error("AcceptBookingOffer: failed to handle logistics", "booking_id", bookingID, "error", err)
			}
		}()
	}

	return nil
}

func (s *BookingService) DeclineBookingOffer(ctx context.Context, therapistID, bookingID int64) error {
	offer, err := s.offerRepo.GetByTherapistAndBooking(ctx, therapistID, bookingID)
	if err != nil {
		return fmt.Errorf("offer not found: %w", err)
	}
// Broadcast event
	_ = broadcaster.BroadcastToUser(therapistID, "offer_declined", map[string]any{
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

	// Add payment breakdown if available
	if len(b.PaymentBreakdownJSON) > 0 {
		var breakdown model.PaymentBreakdown
		if err := json.Unmarshal(b.PaymentBreakdownJSON, &breakdown); err == nil {
			result["payment_breakdown"] = breakdown
		}
	}

	return result
}

// bookingToMapWithTherapist extends bookingToMap by adding therapist profile details when available.
// This is used for real-time socket broadcasts to provide clients with complete information.
func bookingToMapWithTherapist(b *model.Booking, service *model.Service, address *model.Address, therapist *model.TherapistProfile, therapistName, therapistPhone, therapistPhoto, clientName, clientPhone, clientPhoto, clientGender, therapistGender string) map[string]any {
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
	if therapistPhoto != "" {
		therapistMap["photo"] = therapistPhoto
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
	if therapistID == nil {
		return "", "", "", "", nil
	}

	// Use repository method instead of inline SQL
	if s.userRepo != nil {
		infos, err := s.userRepo.GetTherapistInfoBatch(ctx, []int64{*therapistID})
		if err == nil {
			if info, ok := infos[*therapistID]; ok {
				return info.Name, info.Phone, info.Photo, info.Gender, info.Rating
			}
		}
	}

	// Fallback to therapist profile for rating only if userRepo not available
	if s.therapistRepo != nil {
		if prof, err := s.therapistRepo.GetProfile(ctx, *therapistID); err == nil {
			return "", "", "", "", &prof.AvgRating
		}
	}

	return "", "", "", "", nil
}

func (s *BookingService) FetchClientInfo(ctx context.Context, clientID int64) (string, string, string, string) {
	// Use repository method instead of inline SQL
	if s.userRepo != nil {
		infos, err := s.userRepo.GetUserInfoBatch(ctx, []int64{clientID})
		if err == nil {
			if info, ok := infos[clientID]; ok {
				return info.Name, info.Phone, info.Photo, info.Gender
			}
		}
	}

	return "", "", "", ""
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
// if one does not already exist. Returns the conversation ID on success.
func (s *BookingService) EnsureConversation(ctx context.Context, clientID, therapistID int64) (int64, error) {
	if s.messageService == nil {
		slog.Debug("EnsureConversation: messageService is nil, skipping conversation creation")
		return 0, nil
	}

	req := &model.CreateConversationRequest{
		ParticipantIDs: []int64{therapistID},
	}

	conv, err := s.messageService.CreateConversation(ctx, clientID, req)
	if err != nil {
		slog.Warn("EnsureConversation: failed to create conversation", "client_id", clientID, "therapist_id", therapistID, "error", err)
		return 0, err
	}

	slog.Debug("EnsureConversation: conversation created/found", "conversation_id", conv.ConversationID, "client_id", clientID, "therapist_id", therapistID)
	return conv.ConversationID, nil
}

// sendBookingSystemMessage finds/creates the conversation between client and therapist and sends a system message.
func (s *BookingService) sendBookingSystemMessage(clientID, therapistID int64, content string) {
	ctx := context.Background()
	convID, err := s.EnsureConversation(ctx, clientID, therapistID)
	if err != nil || convID == 0 {
		return
	}
	_ = s.messageService.SendSystemMessage(ctx, convID, content)
}

func (s *BookingService) FetchTherapistInfos(ctx context.Context, therapistIDs []int64) map[int64]model.TherapistInfo {
	if len(therapistIDs) == 0 {
		return nil
	}

	infos := make(map[int64]model.TherapistInfo)

	// Use repository method instead of inline SQL
	if s.userRepo != nil {
		repoInfos, err := s.userRepo.GetTherapistInfoBatch(ctx, therapistIDs)
		if err == nil {
			for id, info := range repoInfos {
				infos[id] = model.TherapistInfo{
					TherapistID: id,
					Name:        info.Name,
					Phone:       info.Phone,
					Photo:       info.Photo,
					Gender:      info.Gender,
					Rating:      info.Rating,
				}
			}
			return infos
		}
	}

	// Fallback to therapist profile for ratings only if userRepo not available
	if s.therapistRepo != nil {
		if profiles, err := s.therapistRepo.GetProfiles(ctx, therapistIDs); err == nil {
			for _, p := range profiles {
				r := p.AvgRating
				rCopy := r
				infos[p.TherapistID] = model.TherapistInfo{
					TherapistID: p.TherapistID,
					Rating:      &rCopy,
				}
			}
		}
	}

	return infos
}

func (s *BookingService) FetchClientInfos(ctx context.Context, clientIDs []int64) map[int64]model.ClientInfo {
	if len(clientIDs) == 0 {
		return nil
	}

	infos := make(map[int64]model.ClientInfo)

	// Use repository method instead of inline SQL
	if s.userRepo != nil {
		repoInfos, err := s.userRepo.GetUserInfoBatch(ctx, clientIDs)
		if err == nil {
			for id, info := range repoInfos {
				infos[id] = model.ClientInfo{
					ClientID: id,
					Name:     info.Name,
					Phone:    info.Phone,
					Photo:    info.Photo,
					Gender:   info.Gender,
				}
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
			message = fmt.Sprintf("Therapist %s is on the way to your location.", therapistName)
		} else {
			message = "Your therapist is on the way."
		}
	case "arrived":
		title = "Therapist Arrived"
		if therapistName != "" {
			message = fmt.Sprintf("Therapist %s has arrived.", therapistName)
		} else {
			message = "Your therapist has arrived."
		}
	case "completed":
		title = "Thank You! 💛"
		message = "Thank you so much for choosing Relaxation Hub! We're truly grateful for your trust. 🙏\nWe hope you feel lighter and completely relaxed! 😄\nIf you have time, please rate our service in the booking details.\nBook again soon and let us make relaxation the best part of your week! 💆‍♀️✨"
	case "cancelled":
		title = "Booking Cancelled"
		message = "Your booking has been cancelled."
		
		if actorRole == "client" {
			// If cancelled by client, notify therapist if assigned
			if b.TherapistID != nil {
				targetUserID = *b.TherapistID
				message = "The client has cancelled the booking."
			} else {
				// Client cancelled pending booking - no one to notify (except maybe admin, but skipping for now)
				return 
			}
		} else if actorRole == "therapist" {
             // If cancelled by therapist, notify client (already default targetUserID)
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

// broadcastBookingUpdate fetches the latest booking data, enriches it,
// and broadcasts booking:updated to the client and therapist.
func (s *BookingService) broadcastBookingUpdate(ctx context.Context, bookingID int64, status, actorRole string) {
	b, err := s.repo.GetByBookingID(ctx, bookingID)
	if err != nil || b == nil {
		return
	}

	// Use errgroup for concurrent data fetching
	g, gCtx := errgroup.WithContext(ctx)

	var service *model.Service
	var address *model.Address
	var therapist *model.TherapistProfile
	var therapistName, therapistPhone, therapistGender, therapistPhoto string
	var clientName, clientPhone, clientPhoto, clientGender string

	// Concurrent fetch: service
	if b.ServiceID != nil && s.serviceRepo != nil {
		serviceID := *b.ServiceID
		g.Go(func() error {
			if svc, err := s.serviceRepo.GetByID(gCtx, serviceID); err == nil {
				service = svc
			}
			return nil
		})
	}

	// Concurrent fetch: address
	if b.AddressID != nil && s.addressRepo != nil {
		addressID := *b.AddressID
		g.Go(func() error {
			if addr, err := s.addressRepo.GetByIDUnsafe(gCtx, addressID); err == nil {
				address = addr
			}
			return nil
		})
	}

	// Concurrent fetch: therapist profile and user info
	if b.TherapistID != nil {
		therapistID := *b.TherapistID
		if s.therapistRepo != nil {
			g.Go(func() error {
				if prof, err := s.therapistRepo.GetProfile(gCtx, therapistID); err == nil {
					therapist = prof
				}
				return nil
			})
		}
		if s.db != nil {
			g.Go(func() error {
				userQuery := `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(gender, ''), COALESCE(profile_photo, '') FROM users WHERE user_id = $1`
				_ = s.db.QueryRow(gCtx, userQuery, therapistID).Scan(&therapistName, &therapistPhone, &therapistGender, &therapistPhoto)
				return nil
			})
		}
	}

	// Concurrent fetch: client details
	if s.db != nil {
		clientID := b.ClientID
		g.Go(func() error {
			clientQuery := `SELECT COALESCE(full_name, ''), COALESCE(primary_phone, ''), COALESCE(profile_photo, ''), COALESCE(gender, '') FROM users WHERE user_id = $1`
			_ = s.db.QueryRow(gCtx, clientQuery, clientID).Scan(&clientName, &clientPhone, &clientPhoto, &clientGender)
			return nil
		})
	}

	_ = g.Wait()

	enrichedPayload := bookingToMapWithTherapist(b, service, address, therapist, therapistName, therapistPhone, therapistPhoto, clientName, clientPhone, clientPhoto, clientGender, therapistGender)

	// Send persistent notification if status changed (or just passed explicitly)
	// We call this here to ensure it's coupled with broadcast, but note that UpdateStatus called it explicitly before.
	// Since we refactored UpdateStatus to use this, we should include it here OR removing it from UpdateStatus block means we lost it.
	// Wait, I removed the block in UpdateStatus which INCLUDED sendBookingNotification.
	// So I MUST include it here.
	s.sendBookingNotification(ctx, b, status, actorRole, therapistName)

	// Fire-and-forget socket broadcasts
	go func(clientID int64, payload map[string]any) {
		_ = broadcaster.BroadcastToUser(clientID, "booking:updated", payload)
	}(b.ClientID, enrichedPayload)

	if b.TherapistID != nil {
		go func(therapistID int64, payload map[string]any) {
			_ = broadcaster.BroadcastToUser(therapistID, "booking:updated", payload)
		}(*b.TherapistID, enrichedPayload)
	}
}

// notifyAdminsOfBan sends a notification to all admins about a system ban
func (s *BookingService) notifyAdminsOfBan(ctx context.Context, clientID int64, reason string) {
	if s.notificationService == nil || s.userRepo == nil {
		return
	}

	// Fetch client info for the notification message
	clientName := "Unknown"
	if s.db != nil {
		var name string
		_ = s.db.QueryRow(ctx, `SELECT COALESCE(full_name, 'Unknown') FROM users WHERE user_id = $1`, clientID).Scan(&name)
		if name != "" {
			clientName = name
		}
	}

	// Fetch all admin users
	admins, err := s.userRepo.ListUsers(ctx, "admin")
	if err != nil {
		slog.Warn("notifyAdminsOfBan: failed to list admins", "error", err)
		return
	}

	for _, admin := range admins {
		_, _ = s.notificationService.Create(ctx, &model.CreateNotificationRequest{
			UserID:  int64(admin.UserID),
			Type:    "system_ban",
			Title:   "SYSTEM BAN: Client Banned",
			Message: fmt.Sprintf("Client %s (ID: %d) has been automatically banned. Reason: %s", clientName, clientID, reason),
		})
	}
}


