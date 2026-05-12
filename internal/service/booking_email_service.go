package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

const (
	BookingEmailEventAdvancedConfirmed = "email_advanced_booking_confirmed"
	BookingEmailEventDDay              = "email_advanced_booking_d_day"
	BookingEmailEventTherapistOnTheWay = "email_therapist_on_the_way"
	BookingEmailEventCompletedSuccess  = "email_booking_completed_success"
)

type bookingEmailBookingStore interface {
	GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error)
	ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error)
	InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error
}

type bookingEmailUserStore interface {
	FindUserByID(ctx context.Context, userID int) (*model.User, error)
}

type BookingEmailService struct {
	bookingStore bookingEmailBookingStore
	userStore    bookingEmailUserStore
	sender       EmailSender
	location     *time.Location
	now          func() time.Time
}

func NewBookingEmailService(bookingStore bookingEmailBookingStore, userStore bookingEmailUserStore, sender EmailSender, location *time.Location) *BookingEmailService {
	if location == nil {
		location = time.FixedZone("Asia/Manila", 8*60*60)
	}
	return &BookingEmailService{
		bookingStore: bookingStore,
		userStore:    userStore,
		sender:       sender,
		location:     location,
		now:          time.Now,
	}
}

func (s *BookingEmailService) SendAdvancedBookingConfirmed(ctx context.Context, b *model.Booking) {
	if b == nil || b.ScheduledStart == nil {
		return
	}
	now := s.now().In(s.location)
	scheduled := b.ScheduledStart.In(s.location)
	if sameLocalDate(now, scheduled) || scheduled.Before(now) {
		return
	}
	s.send(ctx, b.BookingID, BookingEmailEventAdvancedConfirmed, "advanced_booking_confirmed")
}

func (s *BookingEmailService) SendBookingDDay(ctx context.Context, b *model.Booking) {
	if b == nil || b.ScheduledStart == nil {
		return
	}
	now := s.now().In(s.location)
	if !sameLocalDate(now, b.ScheduledStart.In(s.location)) {
		return
	}
	s.send(ctx, b.BookingID, BookingEmailEventDDay, "advanced_booking_d_day")
}

func (s *BookingEmailService) SendTherapistOnTheWay(ctx context.Context, b *model.Booking) {
	if b == nil {
		return
	}
	s.send(ctx, b.BookingID, BookingEmailEventTherapistOnTheWay, "therapist_on_the_way")
}

func (s *BookingEmailService) SendBookingCompleted(ctx context.Context, b *model.Booking) {
	if b == nil {
		return
	}
	s.send(ctx, b.BookingID, BookingEmailEventCompletedSuccess, "booking_completed_success")
}

func (s *BookingEmailService) send(ctx context.Context, bookingID int64, eventType, template string) {
	if s == nil || s.sender == nil || s.bookingStore == nil || s.userStore == nil || bookingID == 0 {
		return
	}
	if s.hasEvent(ctx, bookingID, eventType) {
		return
	}

	data, err := s.buildData(ctx, bookingID)
	if err != nil {
		slog.Warn("booking email: failed to build template data", "booking_id", bookingID, "template", template, "error", err)
		return
	}
	if data.ClientEmail == "" {
		slog.Debug("booking email: client has no email", "booking_id", bookingID, "template", template)
		return
	}

	msg := RenderBookingEmail(template, data)
	if msg.Subject == "" {
		return
	}
	msg.To = data.ClientEmail

	if err := s.sender.Send(ctx, msg); err != nil {
		slog.Warn("booking email: send failed", "booking_id", bookingID, "template", template, "error", err)
		return
	}

	if err := s.bookingStore.InsertEvent(ctx, bookingID, eventType, nil, map[string]any{"recipient": data.ClientEmail}); err != nil {
		slog.Warn("booking email: failed to record sent event", "booking_id", bookingID, "event_type", eventType, "error", err)
	}
}

func (s *BookingEmailService) hasEvent(ctx context.Context, bookingID int64, eventType string) bool {
	events, err := s.bookingStore.ListEvents(ctx, bookingID)
	if err != nil {
		slog.Warn("booking email: failed to list booking events", "booking_id", bookingID, "event_type", eventType, "error", err)
		return false
	}
	for _, event := range events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}

func (s *BookingEmailService) buildData(ctx context.Context, bookingID int64) (BookingEmailData, error) {
	details, err := s.bookingStore.GetBookingWithDetailsUnsafe(ctx, bookingID)
	if err != nil {
		return BookingEmailData{}, err
	}
	if details == nil || details.Booking == nil {
		return BookingEmailData{}, fmt.Errorf("booking details not found")
	}

	client, err := s.userStore.FindUserByID(ctx, int(details.Booking.ClientID))
	if err != nil {
		return BookingEmailData{}, err
	}

	data := BookingEmailData{
		ClientName:    fallbackName(client.FullName, "there"),
		ClientEmail:   strings.TrimSpace(client.PrimaryEmail),
		TherapistName: fallbackName(details.TherapistName, "your therapist"),
		ServiceName:   "your massage session",
		ReferenceCode: fmt.Sprintf("#%d", details.Booking.BookingID),
		Duration:      formatDuration(details.Booking.DurationMinutes),
		Total:         formatAmount(details.Booking.FinalTotal),
	}
	if details.Booking.ReferenceCode != nil && strings.TrimSpace(*details.Booking.ReferenceCode) != "" {
		data.ReferenceCode = strings.TrimSpace(*details.Booking.ReferenceCode)
	}
	if details.Service != nil && strings.TrimSpace(details.Service.Name) != "" {
		data.ServiceName = strings.TrimSpace(details.Service.Name)
	}
	if details.Address != nil {
		data.Address = formatBookingEmailAddress(details.Address)
	}
	if details.Booking.ScheduledStart != nil {
		scheduled := details.Booking.ScheduledStart.In(s.location)
		data.ScheduledDate = scheduled.Format("Monday, January 2, 2006")
		data.ScheduledTime = scheduled.Format("3:04 PM")
	}
	return data, nil
}

type BookingEmailData struct {
	ClientName    string
	ClientEmail   string
	TherapistName string
	ServiceName   string
	ScheduledDate string
	ScheduledTime string
	Address       string
	ReferenceCode string
	Duration      string
	Total         string
}

func RenderBookingEmail(template string, data BookingEmailData) EmailMessage {
	subject := bookingEmailSubject(template)
	if subject == "" {
		return EmailMessage{}
	}

	lines := bookingEmailLines(template, data)
	text := strings.Join(lines, "\n")
	htmlBody := renderBookingEmailHTML(lines)

	return EmailMessage{
		Subject:  subject,
		TextBody: text,
		HTMLBody: htmlBody,
	}
}

func bookingEmailSubject(template string) string {
	switch template {
	case "advanced_booking_confirmed":
		return "Your Relaxation Hub booking is confirmed"
	case "advanced_booking_d_day":
		return "Your Relaxation Hub booking is today"
	case "therapist_on_the_way":
		return "Your therapist is on the way"
	case "booking_completed_success":
		return "Your massage session is complete"
	default:
		return ""
	}
}

func bookingEmailLines(template string, data BookingEmailData) []string {
	header := "Hi " + data.ClientName + ","
	details := []string{
		"Booking: " + data.ReferenceCode,
		"Service: " + data.ServiceName,
	}
	if data.ScheduledDate != "" || data.ScheduledTime != "" {
		details = append(details, "Schedule: "+strings.TrimSpace(data.ScheduledDate+" at "+data.ScheduledTime))
	}
	if data.Duration != "" {
		details = append(details, "Duration: "+data.Duration)
	}
	if data.Total != "" {
		details = append(details, "Total: "+data.Total)
	}
	if data.Address != "" {
		details = append(details, "Address: "+data.Address)
	}

	switch template {
	case "advanced_booking_confirmed":
		return append([]string{
			header,
			"Your advanced booking is confirmed. We have reserved your schedule and will keep you updated as your appointment approaches.",
		}, append(details, "Thank you for choosing Relaxation Hub.")...)
	case "advanced_booking_d_day":
		return append([]string{
			header,
			"Your booking is scheduled for today. Please keep your phone available for updates from your therapist.",
		}, append(details, "We look forward to serving you today.")...)
	case "therapist_on_the_way":
		return append([]string{
			header,
			data.TherapistName + " is already on the way to your location.",
		}, append(details, "Please prepare a comfortable space for your massage session.")...)
	case "booking_completed_success":
		return append([]string{
			header,
			"Your massage session has been completed successfully. Thank you for choosing Relaxation Hub.",
		}, append(details, "You can rate the session from your booking details.")...)
	default:
		return nil
	}
}

func renderBookingEmailHTML(lines []string) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html><body style=\"font-family:Arial,sans-serif;color:#1f2937;line-height:1.5\">")
	b.WriteString("<div style=\"max-width:640px;margin:0 auto;padding:24px\">")
	b.WriteString("<h1 style=\"font-size:20px;margin:0 0 16px\">Relaxation Hub</h1>")
	for i, line := range lines {
		escaped := html.EscapeString(line)
		if i == 0 {
			b.WriteString("<p>")
			b.WriteString(escaped)
			b.WriteString("</p>")
			continue
		}
		if strings.Contains(line, ": ") {
			b.WriteString("<p style=\"margin:4px 0\"><strong>")
			parts := strings.SplitN(escaped, ": ", 2)
			b.WriteString(parts[0])
			b.WriteString(":</strong> ")
			b.WriteString(parts[1])
			b.WriteString("</p>")
			continue
		}
		b.WriteString("<p>")
		b.WriteString(escaped)
		b.WriteString("</p>")
	}
	b.WriteString("</div></body></html>")
	return b.String()
}

func sameLocalDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func fallbackName(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func formatDuration(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	if minutes%60 == 0 {
		hours := minutes / 60
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	return fmt.Sprintf("%d minutes", minutes)
}

func formatAmount(amount *float64) string {
	if amount == nil {
		return ""
	}
	return fmt.Sprintf("PHP %.2f", *amount)
}

func formatBookingEmailAddress(address *model.Address) string {
	parts := []string{address.Street, address.Barangay, address.City, address.Province}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, ", ")
}
