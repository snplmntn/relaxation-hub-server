package service

import (
	"context"
	"fmt"
<<<<<<< HEAD
	"log/slog"
=======
	"log"
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// UpcomingBookingWorker sends reminders to clients and therapists for upcoming bookings.
// It runs on a ticker and checks for bookings approaching in 24h and 2h windows.
type UpcomingBookingWorker struct {
	bookingRepo         repository.BookingRepository
	notificationService *NotificationService
	pollInterval        time.Duration
}

// NewUpcomingBookingWorker creates a new UpcomingBookingWorker.
func NewUpcomingBookingWorker(br repository.BookingRepository, ns *NotificationService) *UpcomingBookingWorker {
	return &UpcomingBookingWorker{
		bookingRepo:         br,
		notificationService: ns,
		pollInterval:        5 * time.Minute,
	}
}

// Start begins the background reminder loop.
func (w *UpcomingBookingWorker) Start(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
<<<<<<< HEAD
				slog.Error("upcoming booking worker panic recovered", "error", r)
			}
		}()

		slog.Info("upcoming booking worker started")
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

=======
				log.Printf("upcoming booking worker: panic recovered: %v", r)
			}
		}()

		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		// Run once immediately on startup
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
		w.processOnce(ctx)

		for {
			select {
			case <-ctx.Done():
<<<<<<< HEAD
				slog.Info("upcoming booking worker stopping")
=======
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
				return
			case <-ticker.C:
				w.processOnce(ctx)
			}
		}
	}()
}

<<<<<<< HEAD
func (w *UpcomingBookingWorker) Stop() {
	slog.Info("upcoming booking worker stopped")
}

=======
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
func (w *UpcomingBookingWorker) processOnce(ctx context.Context) {
	now := time.Now()

	// --- 24-Hour Reminders ---
	// Look for bookings with scheduled_start between now+24h and now+24h+15m
	start24h := now.Add(24 * time.Hour)
	end24h := start24h.Add(15 * time.Minute)
	w.sendReminders(ctx, start24h, end24h, "reminder_24h", "24 hours")

	// --- 2-Hour Reminders ---
	// Look for bookings with scheduled_start between now+2h and now+2h+15m
	start2h := now.Add(2 * time.Hour)
	end2h := start2h.Add(15 * time.Minute)
	w.sendReminders(ctx, start2h, end2h, "reminder_2h", "2 hours")
}

func (w *UpcomingBookingWorker) sendReminders(ctx context.Context, start, end time.Time, eventType, timeLabel string) {
	bookings, err := w.bookingRepo.ListUpcomingBookingsForReminder(ctx, start, end, eventType)
	if err != nil {
<<<<<<< HEAD
		slog.Warn("upcoming booking worker: error fetching bookings", "event_type", eventType, "error", err)
=======
		log.Printf("upcoming booking worker: error fetching bookings for %s: %v", eventType, err)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
		return
	}

	if len(bookings) == 0 {
		return
	}

<<<<<<< HEAD
	slog.Debug("upcoming booking worker: found bookings for reminder", "count", len(bookings), "event_type", eventType)
=======
	log.Printf("upcoming booking worker: found %d bookings for %s reminder", len(bookings), eventType)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996

	for _, b := range bookings {
		w.notifyForBooking(ctx, &b, eventType, timeLabel)
	}
}

func (w *UpcomingBookingWorker) notifyForBooking(ctx context.Context, b *model.Booking, eventType, timeLabel string) {
	if w.notificationService == nil {
<<<<<<< HEAD
		slog.Warn("upcoming booking worker: notification service is nil", "booking_id", b.BookingID)
=======
		log.Printf("upcoming booking worker: notification service is nil, skipping booking %d", b.BookingID)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
		return
	}

	// Format time for display
	scheduledTime := "your scheduled time"
	if b.ScheduledStart != nil {
		scheduledTime = b.ScheduledStart.Format("3:04 PM")
	}

	// --- Client Notification ---
	var clientTitle, clientMessage string
	if eventType == "reminder_24h" {
		clientTitle = "Upcoming Massage Tomorrow"
		clientMessage = fmt.Sprintf("You have a booking tomorrow at %s. Use the app to view details or cancel if needed.", scheduledTime)
	} else {
		clientTitle = "Massage Reminder"
		clientMessage = fmt.Sprintf("Your session starts in %s at %s. Your therapist will be preparing to head out soon.", timeLabel, scheduledTime)
	}

	_, err := w.notificationService.Create(ctx, &model.CreateNotificationRequest{
		UserID:  b.ClientID,
		Type:    eventType,
		Title:   clientTitle,
		Message: clientMessage,
		Data:    map[string]any{"booking_id": b.BookingID},
	})
	if err != nil {
<<<<<<< HEAD
		slog.Warn("upcoming booking worker: failed to notify client", "client_id", b.ClientID, "booking_id", b.BookingID, "error", err)
	} else {
		slog.Debug("upcoming booking worker: sent notification to client", "event_type", eventType, "client_id", b.ClientID, "booking_id", b.BookingID)
=======
		log.Printf("upcoming booking worker: failed to notify client %d for booking %d: %v", b.ClientID, b.BookingID, err)
	} else {
		log.Printf("upcoming booking worker: sent %s to client %d for booking %d", eventType, b.ClientID, b.BookingID)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
	}

	// --- Therapist Notification ---
	if b.TherapistID != nil {
		var therapistTitle, therapistMessage string
		if eventType == "reminder_24h" {
			therapistTitle = "Schedule Update"
			therapistMessage = fmt.Sprintf("You have a booking tomorrow at %s. Check your schedule.", scheduledTime)
		} else {
			therapistTitle = "Upcoming Session"
			therapistMessage = fmt.Sprintf("You have a booking in %s at %s. Please prepare to travel soon.", timeLabel, scheduledTime)
		}

		_, err := w.notificationService.Create(ctx, &model.CreateNotificationRequest{
			UserID:  *b.TherapistID,
			Type:    eventType,
			Title:   therapistTitle,
			Message: therapistMessage,
			Data:    map[string]any{"booking_id": b.BookingID},
		})
		if err != nil {
<<<<<<< HEAD
			slog.Warn("upcoming booking worker: failed to notify therapist", "therapist_id", *b.TherapistID, "booking_id", b.BookingID, "error", err)
		} else {
			slog.Debug("upcoming booking worker: sent notification to therapist", "event_type", eventType, "therapist_id", *b.TherapistID, "booking_id", b.BookingID)
=======
			log.Printf("upcoming booking worker: failed to notify therapist %d for booking %d: %v", *b.TherapistID, b.BookingID, err)
		} else {
			log.Printf("upcoming booking worker: sent %s to therapist %d for booking %d", eventType, *b.TherapistID, b.BookingID)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
		}
	}

	// --- Record the event to prevent duplicate notifications ---
	if err := w.bookingRepo.InsertEvent(ctx, b.BookingID, eventType, nil, nil); err != nil {
<<<<<<< HEAD
		slog.Warn("upcoming booking worker: failed to insert event", "event_type", eventType, "booking_id", b.BookingID, "error", err)
=======
		log.Printf("upcoming booking worker: failed to insert %s event for booking %d: %v", eventType, b.BookingID, err)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
	}
}
