package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// checkoutWindow is how long a parked booking stays claimable. It comfortably
// outlasts PayMongo's own session expiry, including QR Ph's 30-minute code.
const checkoutWindow = 60 * time.Minute

// BookingCheckoutService owns the online-payment path: a booking is priced and
// parked, the customer pays on PayMongo's hosted page, and only then is the
// booking actually created.
//
// This ordering is deliberate. Cash and manual-transfer bookings are created
// immediately and settled later; an online booking is settled first so an
// abandoned checkout leaves nothing behind to chase.
type BookingCheckoutService struct {
	repo         repository.BookingCheckoutRepository
	paymentRepo  repository.PaymentRepository
	bookingRepo  repository.BookingRepository
	userRepo     repository.UserRepository
	bookings     *BookingService
	groups       *BookingGroupService
	paymongo     *PayMongoClient
	successURL   string
	cancelURL    string
	businessName string
}

func NewBookingCheckoutService(
	repo repository.BookingCheckoutRepository,
	paymentRepo repository.PaymentRepository,
	bookingRepo repository.BookingRepository,
	userRepo repository.UserRepository,
	bookings *BookingService,
	groups *BookingGroupService,
	paymongo *PayMongoClient,
	successURL, cancelURL string,
) *BookingCheckoutService {
	return &BookingCheckoutService{
		repo: repo, paymentRepo: paymentRepo, bookingRepo: bookingRepo, userRepo: userRepo,
		bookings: bookings, groups: groups, paymongo: paymongo,
		successURL: successURL, cancelURL: cancelURL, businessName: "Kalinga Home Spa",
	}
}

func newCheckoutReference() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "chk_" + hex.EncodeToString(buf), nil
}

// Start prices the requested booking, creates a single-channel PayMongo
// checkout session and parks the request. Nothing is written to bookings,
// promotions or the assignment queue.
func (s *BookingCheckoutService) Start(ctx context.Context, clientID int64, req *model.StartCheckoutRequest) (*model.StartCheckoutResponse, error) {
	if s.paymongo == nil {
		return nil, NewValidationError("online_payment_unavailable", "online payment is not available", nil)
	}
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	channel := strings.TrimSpace(strings.ToLower(req.Channel))
	if !IsPayMongoChannel(channel) {
		return nil, NewValidationError("invalid_channel", "unsupported online payment channel", map[string]string{"channel": "allowed: gcash, paymaya, card, qrph"})
	}
	if (req.Booking == nil) == (req.Group == nil) {
		return nil, NewValidationError("invalid_checkout", "supply exactly one of booking or group", nil)
	}

	var (
		kind        string
		quote       *BookingQuote
		payload     any
		lineItem    string
		description string
		err         error
	)

	if req.Booking != nil {
		kind = model.CheckoutKindSingle
		// Online means online: the stored draft is normalised before pricing so
		// a client cannot park a draft that later creates a cash booking.
		req.Booking.PaymentMethod = model.PaymentMethodOnline
		req.Booking.VoucherCode = ""
		req.Booking.ChangeFor = nil
		if err = validateCreateRequest(req.Booking); err != nil {
			return nil, err
		}
		if quote, err = s.bookings.QuoteBooking(ctx, clientID, req.Booking); err != nil {
			return nil, err
		}
		payload = req.Booking
		lineItem = "Kalinga home spa booking"
		description = "Booking payment"
	} else {
		kind = model.CheckoutKindGroup
		req.Group.PaymentMethod = model.PaymentMethodOnline
		req.Group.VoucherCode = ""
		if quote, err = s.groups.QuoteGroup(ctx, clientID, req.Group, true); err != nil {
			return nil, err
		}
		payload = req.Group
		lineItem = fmt.Sprintf("Kalinga home spa booking (%d guests)", len(req.Group.Bookings))
		description = "Group booking payment"
	}

	if quote.FinalTotal <= 0 {
		return nil, NewValidationError("invalid_amount", "this booking has no payable total", nil)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("serialize checkout payload: %w", err)
	}
	reference, err := newCheckoutReference()
	if err != nil {
		return nil, err
	}

	checkout := &model.BookingCheckout{
		Reference:      reference,
		ClientID:       clientID,
		Kind:           kind,
		Channel:        channel,
		RequestPayload: raw,
		Amount:         quote.FinalTotal,
		ExpiresAt:      time.Now().Add(checkoutWindow),
	}
	if err := s.repo.Create(ctx, checkout); err != nil {
		return nil, fmt.Errorf("create checkout: %w", err)
	}

	email, name := s.clientContact(ctx, clientID)
	session, err := s.paymongo.CreateCheckoutSession(ctx, CheckoutSessionParams{
		Amount:      quote.FinalTotal,
		Description: description,
		LineItem:    lineItem,
		Channel:     channel,
		Reference:   reference,
		SuccessURL:  withReference(s.successURL, reference),
		CancelURL:   withReference(s.cancelURL, reference),
		Email:       email,
		Name:        name,
	})
	if err != nil {
		_ = s.repo.MarkStatus(ctx, checkout.CheckoutID, model.CheckoutStatusFailed)
		slog.Error("[Checkout] paymongo session failed", "reference", reference, "channel", channel, "error", err)
		return nil, fmt.Errorf("could not start online payment: %w", err)
	}

	if err := s.repo.AttachSession(ctx, checkout.CheckoutID, session.ID, session.CheckoutURL); err != nil {
		return nil, err
	}

	return &model.StartCheckoutResponse{
		Reference:   reference,
		CheckoutURL: session.CheckoutURL,
		Amount:      quote.FinalTotal,
		Channel:     channel,
		ExpiresAt:   checkout.ExpiresAt,
	}, nil
}

func (s *BookingCheckoutService) clientContact(ctx context.Context, clientID int64) (string, string) {
	if s.userRepo == nil {
		return "", ""
	}
	u, err := s.userRepo.FindUserByID(ctx, int(clientID))
	if err != nil || u == nil {
		return "", ""
	}
	return u.PrimaryEmail, strings.TrimSpace(u.FullName)
}

// withReference appends the checkout reference so the return page knows which
// checkout to poll without trusting anything PayMongo echoes back.
func withReference(rawURL, reference string) string {
	if rawURL == "" {
		return ""
	}
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + "checkout=" + reference
}

// Fulfil creates the booking a paid checkout was standing in for. It is driven
// by the webhook rather than the browser redirect, so a customer who closes the
// tab the moment payment completes still gets their booking.
func (s *BookingCheckoutService) Fulfil(ctx context.Context, sessionID, eventID string) error {
	checkout, err := s.repo.GetBySessionID(ctx, sessionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			slog.Warn("[Checkout] paid event for unknown session", "session_id", sessionID)
			return nil
		}
		return err
	}
	if checkout.Status == model.CheckoutStatusPaid {
		return nil // already fulfilled; a retried delivery
	}

	claimed, err := s.repo.ClaimForFulfilment(ctx, checkout.CheckoutID, eventID)
	if err != nil {
		return err
	}
	if !claimed {
		slog.Info("[Checkout] already claimed, skipping", "reference", checkout.Reference)
		return nil
	}

	var (
		bookingID *int64
		groupID   *int64
		note      *string
	)

	switch checkout.Kind {
	case model.CheckoutKindSingle:
		var req model.CreateBookingRequest
		if err := json.Unmarshal(checkout.RequestPayload, &req); err != nil {
			return fmt.Errorf("decode parked booking: %w", err)
		}
		booking, cerr := s.bookings.Create(ctx, checkout.ClientID, &req, &checkout.ClientID)
		if cerr != nil {
			// The one real race: the customer pinned a therapist who was taken
			// while they were on PayMongo's page. The money is already in, so
			// retry unpinned and leave a note rather than failing a paid booking.
			if req.TherapistID == nil {
				return fmt.Errorf("create paid booking: %w", cerr)
			}
			slog.Warn("[Checkout] pinned therapist unavailable, retrying unassigned", "reference", checkout.Reference, "error", cerr)
			req.TherapistID = nil
			req.IsTherapistRequested = false
			retryNote := "Paid online. The requested therapist was no longer available at payment time; assign manually."
			booking, cerr = s.bookings.Create(ctx, checkout.ClientID, &req, &checkout.ClientID)
			if cerr != nil {
				return fmt.Errorf("create paid booking after retry: %w", cerr)
			}
			note = &retryNote
		}
		bookingID = &booking.BookingID
		s.recordPayment(ctx, booking.BookingID, valueOr(booking.FinalTotal, checkout.Amount), checkout)

	case model.CheckoutKindGroup:
		var req model.CreateBookingGroupRequest
		if err := json.Unmarshal(checkout.RequestPayload, &req); err != nil {
			return fmt.Errorf("decode parked group: %w", err)
		}
		group, cerr := s.groups.CreateBookingGroup(ctx, checkout.ClientID, checkout.ClientID, &req, true)
		if cerr != nil {
			return fmt.Errorf("create paid booking group: %w", cerr)
		}
		groupID = &group.GroupID
		// One payment row per child booking, all sharing the session id, so
		// per-booking accounting stays correct while the customer paid once.
		children, lerr := s.bookingRepo.GetByGroupID(ctx, group.GroupID)
		if lerr != nil {
			slog.Error("[Checkout] could not list group children for payment rows", "group_id", group.GroupID, "error", lerr)
		}
		for i := range children {
			s.recordPayment(ctx, children[i].BookingID, valueOr(children[i].FinalTotal, 0), checkout)
		}

	default:
		return fmt.Errorf("unknown checkout kind %q", checkout.Kind)
	}

	if err := s.repo.MarkPaid(ctx, checkout.CheckoutID, bookingID, groupID, note); err != nil {
		return err
	}
	slog.Info("[Checkout] fulfilled", "reference", checkout.Reference, "booking_id", bookingID, "group_id", groupID)
	return nil
}

// recordPayment writes the paid payment row. A failure here must not undo a
// created booking, so it is logged rather than returned: the booking exists and
// the money is in, and a missing payment row is a reconciliation task.
func (s *BookingCheckoutService) recordPayment(ctx context.Context, bookingID int64, amount float64, checkout *model.BookingCheckout) {
	if _, err := s.paymentRepo.GetOrCreateByBookingID(ctx, bookingID, amount, checkout.Provider); err != nil {
		slog.Error("[Checkout] could not create payment row", "booking_id", bookingID, "error", err)
		return
	}
	if err := s.paymentRepo.UpdateStatus(ctx, bookingID, "paid", checkout.ProviderSessionID, checkout.EventID); err != nil {
		slog.Error("[Checkout] could not mark payment paid", "booking_id", bookingID, "error", err)
	}
}

func valueOr(v *float64, fallback float64) float64 {
	if v == nil {
		return fallback
	}
	return *v
}

// MarkFailed records that a checkout will not be paid.
func (s *BookingCheckoutService) MarkFailed(ctx context.Context, sessionID string) error {
	checkout, err := s.repo.GetBySessionID(ctx, sessionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	if checkout.Status != model.CheckoutStatusPending {
		return nil
	}
	return s.repo.MarkStatus(ctx, checkout.CheckoutID, model.CheckoutStatusFailed)
}

// Status is polled by the return page while it waits for the webhook.
func (s *BookingCheckoutService) Status(ctx context.Context, clientID int64, reference string) (*model.CheckoutStatusResponse, error) {
	checkout, err := s.repo.GetByReference(ctx, reference)
	if err != nil {
		return nil, err
	}
	if checkout.ClientID != clientID {
		return nil, NewValidationError("forbidden", "this checkout belongs to another account", nil)
	}

	status := checkout.Status
	// Expiry is computed rather than swept: a pending checkout past its window
	// is simply reported as expired, which saves a background worker whose only
	// job would be to flip a column nobody reads until this call.
	if status == model.CheckoutStatusPending && time.Now().After(checkout.ExpiresAt) {
		status = model.CheckoutStatusExpired
	}

	return &model.CheckoutStatusResponse{
		Reference: checkout.Reference,
		Status:    status,
		Amount:    checkout.Amount,
		Channel:   checkout.Channel,
		BookingID: checkout.BookingID,
		GroupID:   checkout.GroupID,
		Note:      checkout.FulfilNote,
	}, nil
}
