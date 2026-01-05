package service

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// CompletionWorker periodically checks for in_progress bookings that have
// exceeded their duration and auto-completes them.
type CompletionWorker struct {
	db                  db.DBTX
	bookingRepo         repository.BookingRepository
	paymentRepo         repository.PaymentRepository
	serviceRepo         repository.ServiceRepository
	notificationService *NotificationService
	pollInterval        time.Duration
}

func NewCompletionWorker(pool db.DBTX, br repository.BookingRepository, pr repository.PaymentRepository, sr repository.ServiceRepository, ns *NotificationService) *CompletionWorker {
	return &CompletionWorker{
		db:                  pool,
		bookingRepo:         br,
		paymentRepo:         pr,
		serviceRepo:         sr,
		notificationService: ns,
		pollInterval:        30 * time.Second,
	}
}

func (w *CompletionWorker) Start(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("completion worker: panic recovered: %v", r)
			}
		}()

		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		// Run once immediately on start
		w.processOnce(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.processOnce(ctx)
			}
		}
	}()
}

func (w *CompletionWorker) processOnce(ctx context.Context) {
	bookings, err := w.bookingRepo.ListInProgressBookings(ctx)
	if err != nil {
		log.Printf("completion worker: failed to list in_progress bookings: %v", err)
		return
	}

	if len(bookings) == 0 {
		return
	}

	now := time.Now().UTC()
	for _, b := range bookings {
		// Skip if currently paused (current_pause_start is set)
		if b.CurrentPauseStart != nil {
			continue
		}

		// Skip if actual_start is not set (shouldn't happen for in_progress, but safety check)
		if b.ActualStart == nil {
			continue
		}

		// Calculate effective end time: actual_start + duration_minutes - total_paused_seconds
		durationSecs := b.DurationMinutes * 60
		effectiveEndTime := b.ActualStart.Add(time.Duration(durationSecs) * time.Second)
		effectiveEndTime = effectiveEndTime.Add(time.Duration(b.TotalPausedSeconds) * time.Second)

		if now.After(effectiveEndTime) {
			// Check if payment is verified or paid
			p, err := w.paymentRepo.GetByBookingID(ctx, b.BookingID)
			
			// If no payment record, treat as pending (unless it's a very old system where manual was implied without record, 
			// but we now enforce payment records for proof).
			// If err != nil (e.g. no rows), we assume not paid/verified.
			
			isPaidOrVerified := false
			if err == nil && p != nil {
				// Condition: Status must be explicitly 'paid' or 'verified'.
				// We do not rely solely on VerifiedAt timestamp for security.
				status := strings.ToLower(p.Status)
				if status == "paid" || status == "verified" {
					isPaidOrVerified = true
				}
			}

			if isPaidOrVerified {
				log.Printf("completion worker: booking %d timer expired and payment verified - auto-completing", b.BookingID)
				if err := w.completeBooking(ctx, &b); err != nil {
					log.Printf("completion worker: failed to complete booking %d: %v", b.BookingID, err)
				}
			} else {
				// Not paid/verified yet
				log.Printf("completion worker: booking %d timer expired but payment NOT verified - awaiting confirmation", b.BookingID)
			}
		}
	}
}

func (w *CompletionWorker) completeBooking(ctx context.Context, b *model.Booking) error {
	now := time.Now()

	// Calculate therapist earnings and platform fee
	var therapistEarnings, platformFee *float64
	if b.ServiceID != nil && w.serviceRepo != nil {
		if svc, err := w.serviceRepo.GetByID(ctx, *b.ServiceID); err == nil && svc.TherapistCommission != nil {
			// Base commission
			earnings := *svc.TherapistCommission
			// Pro-rate for extended duration if applicable
			if b.DurationMinutes > svc.DurationMinutes && svc.DurationMinutes > 0 && svc.BasePrice > 0 {
				commissionRatio := *svc.TherapistCommission / svc.BasePrice
				extraMinutes := b.DurationMinutes - svc.DurationMinutes
				ratePerMinute := svc.BasePrice / float64(svc.DurationMinutes)
				extraCost := ratePerMinute * float64(extraMinutes)
				earnings += extraCost * commissionRatio
			}
			therapistEarnings = &earnings
			if b.FinalTotal != nil {
				fee := *b.FinalTotal - earnings
				platformFee = &fee
			}
		}
	}

	// Update booking status to completed with commission data using repository
	if err := w.bookingRepo.CompleteBooking(ctx, b.BookingID, therapistEarnings, platformFee, now); err != nil {
		return err
	}

	// Insert event for timeline
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

