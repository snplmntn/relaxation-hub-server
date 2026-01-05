package service

import (
	"context"
	"fmt"
	"log"
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
				log.Printf("upcoming booking worker: panic recovered: %v", r)
			}
		}()

		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		// Run once immediately on startup
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
		log.Printf("upcoming booking worker: error fetching bookings for %s: %v", eventType, err)
		return
	}

	if len(bookings) == 0 {
		return
	}

	log.Printf("upcoming booking worker: found %d bookings for %s reminder", len(bookings), eventType)

	for _, b := range bookings {
		w.notifyForBooking(ctx, &b, eventType, timeLabel)
	}
}

func (w *UpcomingBookingWorker) notifyForBooking(ctx context.Context, b *model.Booking, eventType, timeLabel string) {
	if w.notificationService == nil {
		log.Printf("upcoming booking worker: notification service is nil, skipping booking %d", b.BookingID)
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
		log.Printf("upcoming booking worker: failed to notify client %d for booking %d: %v", b.ClientID, b.BookingID, err)
	} else {
		log.Printf("upcoming booking worker: sent %s to client %d for booking %d", eventType, b.ClientID, b.BookingID)
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
			log.Printf("upcoming booking worker: failed to notify therapist %d for booking %d: %v", *b.TherapistID, b.BookingID, err)
		} else {
			log.Printf("upcoming booking worker: sent %s to therapist %d for booking %d", eventType, *b.TherapistID, b.BookingID)
		}
	}

	// --- Record the event to prevent duplicate notifications ---
	if err := w.bookingRepo.InsertEvent(ctx, b.BookingID, eventType, nil, nil); err != nil {
		log.Printf("upcoming booking worker: failed to insert %s event for booking %d: %v", eventType, b.BookingID, err)
	}
}
