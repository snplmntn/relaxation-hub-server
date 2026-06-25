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
	htmlBody := renderBookingEmailHTML(template, data)

	return EmailMessage{
		Subject:  subject,
		TextBody: text,
		HTMLBody: htmlBody,
	}
}

func bookingEmailSubject(template string) string {
	switch template {
	case "advanced_booking_confirmed":
		return "Your Kalinga Spa booking is confirmed"
	case "advanced_booking_d_day":
		return "Your Kalinga Spa booking is today"
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
		}, append(details, "Thank you for choosing Kalinga Spa.")...)
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
			"Your massage session has been completed successfully. Thank you for choosing Kalinga Spa.",
		}, append(details, "You can rate the session from your booking details.")...)
	default:
		return nil
	}
}

func bookingEmailTitle(template string) string {
	switch template {
	case "advanced_booking_confirmed":
		return "Your booking is confirmed"
	case "advanced_booking_d_day":
		return "Your appointment is today"
	case "therapist_on_the_way":
		return "Your therapist is on the way"
	case "booking_completed_success":
		return "Session completed"
	default:
		return "Booking update"
	}
}

func bookingEmailIntro(template string, data BookingEmailData) string {
	switch template {
	case "advanced_booking_confirmed":
		return "Your advanced booking is confirmed. We have reserved your schedule and will keep you updated as your appointment approaches."
	case "advanced_booking_d_day":
		return "Your booking is scheduled for today. Please keep your phone available for updates from your therapist."
	case "therapist_on_the_way":
		return fallbackName(data.TherapistName, "Your therapist") + " is already on the way to your location."
	case "booking_completed_success":
		return "Your massage session has been completed successfully. Thank you for choosing Kalinga Spa."
	default:
		return "Here is an update about your Kalinga Spa booking."
	}
}

func bookingEmailClosing(template string) string {
	switch template {
	case "advanced_booking_confirmed":
		return "Thank you for choosing Kalinga Spa."
	case "advanced_booking_d_day":
		return "We look forward to serving you today."
	case "therapist_on_the_way":
		return "Please prepare a comfortable space for your massage session."
	case "booking_completed_success":
		return "You can rate the session from your booking details."
	default:
		return "Thank you for choosing Kalinga Spa."
	}
}

func renderBookingEmailHTML(template string, data BookingEmailData) string {
	type detailRow struct {
		Label string
		Value string
	}

	details := []detailRow{
		{Label: "Booking", Value: data.ReferenceCode},
		{Label: "Service", Value: data.ServiceName},
	}
	if data.ScheduledDate != "" || data.ScheduledTime != "" {
		details = append(details, detailRow{Label: "Schedule", Value: strings.TrimSpace(data.ScheduledDate + " at " + data.ScheduledTime)})
	}
	if data.Duration != "" {
		details = append(details, detailRow{Label: "Duration", Value: data.Duration})
	}
	if data.Total != "" {
		details = append(details, detailRow{Label: "Total", Value: data.Total})
	}
	if data.Address != "" {
		details = append(details, detailRow{Label: "Address", Value: data.Address})
	}

	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><meta name="supported-color-schemes" content="light dark"><style>
@media (prefers-color-scheme: dark) {
  .email-body { background: #102624 !important; }
  .email-shell { background: #173734 !important; border-color: #315f59 !important; }
  .email-panel { background: #102624 !important; border-color: #315f59 !important; }
  .email-title, .email-value, .email-brand { color: #fff8f3 !important; }
  .email-copy, .email-label, .email-footer { color: #bcece6 !important; }
  .email-accent { color: #f0c48a !important; }
  .email-rule { border-color: #315f59 !important; }
}
@media screen and (max-width: 620px) {
  .email-container { width: 100% !important; }
  .email-padding { padding-left: 18px !important; padding-right: 18px !important; }
}
</style></head>`)
	b.WriteString(`<body class="email-body" bgcolor="#fff8f3" style="margin:0;padding:0;background:#fff8f3;font-family:Arial,Helvetica,sans-serif;-webkit-font-smoothing:antialiased;color:#1f1b15;">`)
	b.WriteString(`<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;">`)
	b.WriteString(html.EscapeString(bookingEmailIntro(template, data)))
	b.WriteString(`</div>`)
	b.WriteString(`<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" bgcolor="#fff8f3" style="background:#fff8f3;" class="email-body"><tr><td align="center" style="padding:32px 12px;">`)
	b.WriteString(`<table role="presentation" width="600" cellspacing="0" cellpadding="0" border="0" bgcolor="#ffffff" class="email-container email-shell" style="width:600px;max-width:600px;background:#ffffff;border:1px solid #e4d7c8;border-radius:24px;overflow:hidden;box-shadow:0 18px 45px rgba(45,90,86,0.12);">`)
	b.WriteString(`<tr><td class="email-padding" bgcolor="#2d5a56" style="padding:28px 32px 22px;background:#2d5a56;">`)
	b.WriteString(`<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0"><tr><td>`)
	b.WriteString(`<div class="email-brand" style="font-size:24px;line-height:1.1;font-weight:700;color:#fff8f3;letter-spacing:0;">Kalinga Spa</div>`)
	b.WriteString(`</td><td align="right" style="vertical-align:top;">`)
	b.WriteString(`<span style="display:inline-block;border:1px solid rgba(188,236,230,.55);border-radius:999px;padding:7px 12px;font-size:12px;line-height:1;font-weight:700;color:#fff8f3;">Booking update</span>`)
	b.WriteString(`</td></tr></table>`)
	b.WriteString(`</td></tr>`)
	b.WriteString(`<tr><td class="email-padding" style="padding:34px 32px 8px;">`)
	b.WriteString(`<p class="email-accent" style="margin:0 0 12px;font-size:12px;line-height:1.4;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:#755a2c;">Hi `)
	b.WriteString(html.EscapeString(fallbackName(data.ClientName, "there")))
	b.WriteString(`,</p>`)
	b.WriteString(`<h1 class="email-title" style="margin:0;color:#12423f;font-size:28px;line-height:1.22;font-weight:700;letter-spacing:0;">`)
	b.WriteString(html.EscapeString(bookingEmailTitle(template)))
	b.WriteString(`</h1>`)
	b.WriteString(`<p class="email-copy" style="margin:16px 0 0;color:#404847;font-size:16px;line-height:1.65;">`)
	b.WriteString(html.EscapeString(bookingEmailIntro(template, data)))
	b.WriteString(`</p>`)
	b.WriteString(`</td></tr>`)
	b.WriteString(`<tr><td class="email-padding" style="padding:22px 32px 8px;">`)
	b.WriteString(`<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" bgcolor="#fbf2e8" class="email-panel" style="background:#fbf2e8;border:1px solid #e4d7c8;border-radius:18px;">`)
	for i, row := range details {
		if strings.TrimSpace(row.Value) == "" {
			continue
		}
		border := ""
		if i > 0 {
			border = "border-top:1px solid #e4d7c8;"
		}
		b.WriteString(`<tr><td style="padding:14px 18px;`)
		b.WriteString(border)
		b.WriteString(`" class="email-rule"><div class="email-label" style="font-size:12px;line-height:1.4;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:#755a2c;">`)
		b.WriteString(html.EscapeString(row.Label))
		b.WriteString(`</div><div class="email-value" style="margin-top:4px;font-size:15px;line-height:1.55;font-weight:700;color:#12423f;">`)
		b.WriteString(html.EscapeString(row.Value))
		b.WriteString(`</div></td></tr>`)
	}
	b.WriteString(`</table>`)
	b.WriteString(`</td></tr>`)
	b.WriteString(`<tr><td class="email-padding" style="padding:18px 32px 34px;">`)
	b.WriteString(`<p class="email-copy" style="margin:0;color:#404847;font-size:15px;line-height:1.6;">`)
	b.WriteString(html.EscapeString(bookingEmailClosing(template)))
	b.WriteString(`</p>`)
	b.WriteString(`<div class="email-rule" style="margin-top:26px;border-top:1px solid #e4d7c8;padding-top:18px;">`)
	b.WriteString(`<p class="email-footer" style="margin:0;color:#6f766f;font-size:12px;line-height:1.6;">This is an automated message from Kalinga Spa. Please keep this email for your booking reference.</p>`)
	b.WriteString(`</div>`)
	b.WriteString(`</td></tr>`)
	b.WriteString(`</table>`)
	b.WriteString(`</td></tr></table>`)
	b.WriteString(`</body></html>`)
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
