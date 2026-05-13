package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

const completionWorkerDueBatchLimit = 50

// CompletionWorker periodically checks for in_progress bookings that have
// exceeded their duration and auto-completes them.
type CompletionWorker struct {
	db                  db.DBTX
	bookingRepo         repository.BookingRepository
	paymentRepo         repository.PaymentRepository
	serviceRepo         repository.ServiceRepository
	ledgerRepo          repository.LedgerRepository
	walletService       *WalletService
	notificationService *NotificationService
	pollInterval        time.Duration
}

func NewCompletionWorker(pool db.DBTX, br repository.BookingRepository, pr repository.PaymentRepository, sr repository.ServiceRepository, lr repository.LedgerRepository, ws *WalletService, ns *NotificationService) *CompletionWorker {
	return &CompletionWorker{
		db:                  pool,
		bookingRepo:         br,
		paymentRepo:         pr,
		serviceRepo:         sr,
		ledgerRepo:          lr,
		walletService:       ws,
		notificationService: ns,
		pollInterval:        30 * time.Second,
	}
}

func (w *CompletionWorker) Start(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("completion worker panic recovered", "error", r)
			}
		}()

		slog.Info("completion worker started")
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		// Run once immediately on start
		w.processOnce(ctx)

		for {
			select {
			case <-ctx.Done():
				slog.Info("completion worker stopping")
				return
			case <-ticker.C:
				w.processOnce(ctx)
			}
		}
	}()
}

func (w *CompletionWorker) Stop() {
	slog.Info("completion worker stopped")
}

func (w *CompletionWorker) processOnce(ctx context.Context) {
	now := time.Now().UTC()
	bookings, err := w.bookingRepo.ListDueInProgressBookings(ctx, now, completionWorkerDueBatchLimit)
	if err != nil {
		slog.Error("completion worker: failed to list due in_progress bookings", "error", err)
		return
	}

	if len(bookings) == 0 {
		return
	}

	// --- Batch Pre-Fetch: Payments and Services ---
	bookingIDs := make([]int64, len(bookings))
	serviceIDSet := make(map[int64]struct{})
	for i, b := range bookings {
		bookingIDs[i] = b.BookingID
		if b.ServiceID != nil {
			serviceIDSet[*b.ServiceID] = struct{}{}
		}
	}

	// Batch fetch payments
	paymentsByBookingID, err := w.paymentRepo.GetByBookingIDBatch(ctx, bookingIDs)
	if err != nil {
		slog.Warn("completion worker: failed to batch-fetch payments", "error", err)
		paymentsByBookingID = make(map[int64]*model.Payment)
	}

	// Batch fetch services
	serviceIDs := make([]int64, 0, len(serviceIDSet))
	for id := range serviceIDSet {
		serviceIDs = append(serviceIDs, id)
	}
	servicesByID := make(map[int64]*model.Service)
	if w.serviceRepo != nil && len(serviceIDs) > 0 {
		services, err := w.serviceRepo.GetByIDs(ctx, serviceIDs)
		if err != nil {
			slog.Warn("completion worker: failed to batch-fetch services", "error", err)
		} else {
			for i := range services {
				servicesByID[services[i].ServiceID] = &services[i]
			}
		}
	}

	for _, b := range bookings {
		p := paymentsByBookingID[b.BookingID]

		isPaidOrVerified := false
		if p != nil {
			// Condition: Status must be explicitly 'paid' or 'verified'.
			status := strings.ToLower(p.Status)
			if status == "paid" || status == "verified" {
				isPaidOrVerified = true
			}
		}

		if isPaidOrVerified {
			slog.Info("booking timer expired, auto-completing", "booking_id", b.BookingID, "payment_verified", true)
			// Pass pre-fetched service to avoid per-booking lookup
			var svc *model.Service
			if b.ServiceID != nil {
				svc = servicesByID[*b.ServiceID]
			}
			if err := w.completeBooking(ctx, &b, svc); err != nil {
				slog.Error("failed to complete booking", "booking_id", b.BookingID, "error", err)
			}
		} else {
			// Not paid/verified yet
			slog.Debug("booking timer expired but payment not verified", "booking_id", b.BookingID)
		}
	}
}

func (w *CompletionWorker) completeBooking(ctx context.Context, b *model.Booking, svc *model.Service) error {
	now := time.Now()

	// Calculate therapist earnings and platform fee using pre-fetched service
	var therapistEarnings, platformFee *float64
	if svc != nil && svc.TherapistCommission != nil {
		earnings := CalculateCommission(*svc.TherapistCommission, svc.BasePrice, svc.DurationMinutes, b.DurationMinutes)
		therapistEarnings = &earnings
		if b.FinalTotal != nil {
			fee := *b.FinalTotal - earnings
			platformFee = &fee
		}
	}

	// Atomically update booking status AND insert ledger entries in one transaction.
	// This prevents ledger drift if the process crashes between status and ledger updates.
	if w.db != nil && b.FinalTotal != nil {
		revenue := *b.FinalTotal
		if err := w.bookingRepo.CompleteBookingWithLedgerTx(ctx, w.db, b.BookingID, b.TherapistID, therapistEarnings, platformFee, revenue, now); err != nil {
			return err
		}
	} else {
		// Fallback for unit tests or when db is nil
		if err := w.bookingRepo.CompleteBooking(ctx, b.BookingID, therapistEarnings, platformFee, now); err != nil {
			return err
		}
	}

	// Credit earnings to therapist wallet (async, best-effort)
	if w.walletService != nil && b.TherapistID != nil && therapistEarnings != nil {
		go func(tID, bID int64, amount float64) {
			if err := w.walletService.CreditEarning(context.Background(), tID, bID, amount, nil); err != nil {
				slog.Warn("failed to credit wallet", "therapist_id", tID, "booking_id", bID, "error", err)
			}
		}(*b.TherapistID, b.BookingID, *therapistEarnings)
	}

	// Insert event for timeline (outside transaction, best-effort)
	eventMeta := map[string]any{"reason": "timer_expired"}
	if therapistEarnings != nil {
		eventMeta["therapist_earnings"] = *therapistEarnings
	}
	if platformFee != nil {
		eventMeta["platform_fee"] = *platformFee
	}
	_ = w.bookingRepo.InsertEvent(ctx, b.BookingID, "auto_completed", nil, eventMeta)

	// Broadcast to client and therapist
	updatedBooking := map[string]any{
		"booking_id": b.BookingID,
		"status":     "completed",
		"actual_end": now.Format(time.RFC3339),
	}
	_ = broadcaster.BroadcastToUser(b.ClientID, "booking:completed", updatedBooking)
	if b.TherapistID != nil {
		_ = broadcaster.BroadcastToUser(*b.TherapistID, "booking:completed", updatedBooking)
	}

	// Send notification to client
	if w.notificationService != nil {
		_, _ = w.notificationService.Create(ctx, &model.CreateNotificationRequest{
			UserID:  b.ClientID,
			Type:    "booking_completed",
			Title:   "Session Completed",
			Message: "Thank you so much for choosing Relaxation Hub! We're truly grateful for your trust. 🙏\nWe hope you feel lighter and completely relaxed! 😄\nIf you have time, please rate our service in the booking details.\nBook again soon and let us make relaxation the best part of your week! 💆‍♀️✨",
			Data:    map[string]any{"booking_id": b.BookingID},
		})
	}

	return nil
}
