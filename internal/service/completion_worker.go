package service

import (
	"context"
	"log"
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
	notificationService *NotificationService
	pollInterval        time.Duration
}

func NewCompletionWorker(pool db.DBTX, br repository.BookingRepository, ns *NotificationService) *CompletionWorker {
	return &CompletionWorker{
		db:                  pool,
		bookingRepo:         br,
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
			log.Printf("completion worker: auto-completing booking %d (actual_start=%v, duration=%dm, paused=%ds, effectiveEnd=%v, now=%v)",
				b.BookingID, b.ActualStart, b.DurationMinutes, b.TotalPausedSeconds, effectiveEndTime, now)

			// Update status to completed
			if err := w.completeBooking(ctx, &b); err != nil {
				log.Printf("completion worker: failed to complete booking %d: %v", b.BookingID, err)
				continue
			}

			log.Printf("completion worker: successfully completed booking %d", b.BookingID)
		}
	}
}

func (w *CompletionWorker) completeBooking(ctx context.Context, b *model.Booking) error {
	now := time.Now()

	// Update booking status to completed with actual_end timestamp
	// Using raw SQL since we need to bypass actor checks (this is system action)
	_, err := w.db.Exec(ctx, `
		UPDATE bookings
		SET status = 'completed',
			actual_end = $1,
			updated_at = $1
		WHERE booking_id = $2 AND status = 'in_progress'
	`, now, b.BookingID)
	if err != nil {
		return err
	}

	// Insert event for timeline
	_ = w.bookingRepo.InsertEvent(ctx, b.BookingID, "auto_completed", nil, map[string]any{
		"reason": "timer_expired",
	})

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
			Message: "Thank you so much for choosing Relaxation Hub! We're truly grateful for your trust. 🙏\nWe hope you feel lighter and completely relaxed! 😄\nWhen you’re ready for your next massage, we’ll be here — just a booking away.\nBook again soon and let us make relaxation the best part of your week! 💆‍♀️✨",
			Data:    map[string]any{"booking_id": b.BookingID},
		})
	}

	return nil
}
