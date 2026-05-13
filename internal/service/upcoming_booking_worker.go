package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

const upcomingBookingReminderBatchLimit = 50

type bookingReminderJobRepository interface {
	ClaimDueReminderJobs(ctx context.Context, now time.Time, limit int) ([]repository.BookingReminderJob, error)
	MarkReminderJobProcessed(ctx context.Context, jobID int64) error
	InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error
}

// UpcomingBookingWorker sends reminders to clients and therapists for upcoming bookings.
// It runs on a ticker and claims due reminder jobs.
type UpcomingBookingWorker struct {
	bookingRepo         bookingReminderJobRepository
	notificationService *NotificationService
	pollInterval        time.Duration
}

// NewUpcomingBookingWorker creates a new UpcomingBookingWorker.
func NewUpcomingBookingWorker(br bookingReminderJobRepository, ns *NotificationService) *UpcomingBookingWorker {
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
				slog.Error("upcoming booking worker panic recovered", "error", r)
			}
		}()

		slog.Info("upcoming booking worker started")
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		w.processOnce(ctx)

		for {
			select {
			case <-ctx.Done():
				slog.Info("upcoming booking worker stopping")
				return
			case <-ticker.C:
				w.processOnce(ctx)
			}
		}
	}()
}

func (w *UpcomingBookingWorker) Stop() {
	slog.Info("upcoming booking worker stopped")
}

func (w *UpcomingBookingWorker) processOnce(ctx context.Context) {
	now := time.Now().UTC()
	jobs, err := w.bookingRepo.ClaimDueReminderJobs(ctx, now, upcomingBookingReminderBatchLimit)
	if err != nil {
		slog.Warn("upcoming booking worker: error claiming reminder jobs", "error", err)
		return
	}

	if len(jobs) == 0 {
		return
	}

	slog.Debug("upcoming booking worker: found due reminder jobs", "count", len(jobs))

	for _, job := range jobs {
		w.processReminderJob(ctx, job)
	}
}

func (w *UpcomingBookingWorker) processReminderJob(ctx context.Context, job repository.BookingReminderJob) {
	if !shouldProcessReminderJob(job) {
		if err := w.bookingRepo.MarkReminderJobProcessed(ctx, job.JobID); err != nil {
			slog.Warn("upcoming booking worker: failed to mark skipped reminder job processed", "job_id", job.JobID, "booking_id", job.BookingID, "error", err)
		}
		return
	}

	w.notifyForBooking(ctx, &job.Booking, job.EventType, reminderTimeLabel(job.EventType))
	if err := w.bookingRepo.MarkReminderJobProcessed(ctx, job.JobID); err != nil {
		slog.Warn("upcoming booking worker: failed to mark reminder job processed", "job_id", job.JobID, "booking_id", job.BookingID, "error", err)
	}
}

func shouldProcessReminderJob(job repository.BookingReminderJob) bool {
	if job.ProcessedAt != nil || job.Booking.Status != model.BookingStatusAssigned || job.Booking.ScheduledStart == nil {
		return false
	}
	return job.Booking.ScheduledStart.Equal(job.ScheduledStart)
}

func reminderTimeLabel(eventType string) string {
	if eventType == "reminder_24h" {
		return "24 hours"
	}
	return "2 hours"
}

func (w *UpcomingBookingWorker) notifyForBooking(ctx context.Context, b *model.Booking, eventType, timeLabel string) {
	if w.notificationService == nil {
		slog.Warn("upcoming booking worker: notification service is nil", "booking_id", b.BookingID)
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
		slog.Warn("upcoming booking worker: failed to notify client", "client_id", b.ClientID, "booking_id", b.BookingID, "error", err)
	} else {
		slog.Debug("upcoming booking worker: sent notification to client", "event_type", eventType, "client_id", b.ClientID, "booking_id", b.BookingID)
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
			slog.Warn("upcoming booking worker: failed to notify therapist", "therapist_id", *b.TherapistID, "booking_id", b.BookingID, "error", err)
		} else {
			slog.Debug("upcoming booking worker: sent notification to therapist", "event_type", eventType, "therapist_id", *b.TherapistID, "booking_id", b.BookingID)
		}
	}

	// --- Record the event to prevent duplicate notifications ---
	if err := w.bookingRepo.InsertEvent(ctx, b.BookingID, eventType, nil, nil); err != nil {
		slog.Warn("upcoming booking worker: failed to insert event", "event_type", eventType, "booking_id", b.BookingID, "error", err)
	}
}
