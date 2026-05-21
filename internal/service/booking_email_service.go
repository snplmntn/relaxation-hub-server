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
	details := bookingEmailDetailLines(bookingEmailDetails(data))

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

type bookingEmailDetail struct {
	Label string
	Value string
}

type bookingEmailView struct {
	Preheader   string
	Eyebrow     string
	Badge       string
	Headline    string
	Intro       string
	Closing     string
	Accent      string
	AccentLight string
}

func bookingEmailDetails(data BookingEmailData) []bookingEmailDetail {
	details := []bookingEmailDetail{
		{Label: "Booking", Value: data.ReferenceCode},
		{Label: "Service", Value: data.ServiceName},
	}
	if data.ScheduledDate != "" || data.ScheduledTime != "" {
		details = append(details, bookingEmailDetail{Label: "Schedule", Value: strings.TrimSpace(data.ScheduledDate + " at " + data.ScheduledTime)})
	}
	if data.Duration != "" {
		details = append(details, bookingEmailDetail{Label: "Duration", Value: data.Duration})
	}
	if data.Total != "" {
		details = append(details, bookingEmailDetail{Label: "Total", Value: data.Total})
	}
	if data.Address != "" {
		details = append(details, bookingEmailDetail{Label: "Address", Value: data.Address})
	}
	return details
}

func bookingEmailDetailLines(details []bookingEmailDetail) []string {
	lines := make([]string, 0, len(details))
	for _, detail := range details {
		if strings.TrimSpace(detail.Value) == "" {
			continue
		}
		lines = append(lines, detail.Label+": "+detail.Value)
	}
	return lines
}

func bookingEmailViewFor(template string, data BookingEmailData) bookingEmailView {
	switch template {
	case "advanced_booking_confirmed":
		return bookingEmailView{
			Preheader:   "Your Relaxation Hub booking is confirmed.",
			Eyebrow:     "Booking confirmed",
			Badge:       "Confirmed",
			Headline:    "Your session is reserved",
			Intro:       "Hi " + data.ClientName + ", your advanced booking is confirmed. We have reserved your schedule and will keep you updated as your appointment approaches.",
			Closing:     "Thank you for choosing Relaxation Hub.",
			Accent:      "#0f766e",
			AccentLight: "#ecfdf5",
		}
	case "advanced_booking_d_day":
		return bookingEmailView{
			Preheader:   "Your Relaxation Hub booking is scheduled for today.",
			Eyebrow:     "Today is your session",
			Badge:       "Today",
			Headline:    "Your booking is today",
			Intro:       "Hi " + data.ClientName + ", your booking is scheduled for today. Please keep your phone available for updates from your therapist.",
			Closing:     "We look forward to serving you today.",
			Accent:      "#b45309",
			AccentLight: "#fff7ed",
		}
	case "therapist_on_the_way":
		return bookingEmailView{
			Preheader:   data.TherapistName + " is on the way to your location.",
			Eyebrow:     "Therapist on the way",
			Badge:       "On the way",
			Headline:    "Your therapist is heading to you",
			Intro:       "Hi " + data.ClientName + ", " + data.TherapistName + " is already on the way to your location.",
			Closing:     "Please prepare a comfortable space for your massage session.",
			Accent:      "#2563eb",
			AccentLight: "#eff6ff",
		}
	case "booking_completed_success":
		return bookingEmailView{
			Preheader:   "Your Relaxation Hub massage session has been completed successfully.",
			Eyebrow:     "Session complete",
			Badge:       "Completed",
			Headline:    "Your session is complete",
			Intro:       "Hi " + data.ClientName + ", your massage session has been completed successfully. Thank you for choosing Relaxation Hub.",
			Closing:     "You can rate the session from your booking details.",
			Accent:      "#4d7c0f",
			AccentLight: "#f7fee7",
		}
	default:
		return bookingEmailView{}
	}
}

func renderBookingEmailHTML(template string, data BookingEmailData) string {
	view := bookingEmailViewFor(template, data)
	if view.Headline == "" {
		return ""
	}
	details := bookingEmailDetails(data)

	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>
body{margin:0;padding:0;background:#f6f2ec;font-family:Arial,Helvetica,sans-serif;color:#1f2937;}
table{border-collapse:collapse;}
.wrapper{width:100%;background:#f6f2ec;padding:28px 12px;}
.container{width:100%;max-width:640px;margin:0 auto;background:#ffffff;border-radius:18px;overflow:hidden;border:1px solid #e9ded1;}
.header{padding:28px 32px 18px;}
.brand{font-size:18px;line-height:24px;font-weight:700;color:#111827;}
.badge{display:inline-block;border-radius:999px;padding:7px 12px;font-size:12px;line-height:14px;font-weight:700;}
.hero{padding:10px 32px 26px;}
.eyebrow{font-size:13px;line-height:18px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;margin:0 0 10px;}
.headline{font-size:30px;line-height:36px;font-weight:700;margin:0 0 14px;color:#111827;}
.intro{font-size:16px;line-height:25px;margin:0;color:#374151;}
.details{padding:0 32px 8px;}
.detail-card{border:1px solid #eadfce;border-radius:14px;overflow:hidden;background:#fffdfb;}
.detail-title{font-size:14px;line-height:20px;font-weight:700;color:#111827;padding:18px 20px;border-bottom:1px solid #eadfce;}
.detail-row{border-bottom:1px solid #f1e8dc;}
.detail-row-last{border-bottom:0;}
.detail-label{width:34%;font-size:12px;line-height:18px;font-weight:700;text-transform:uppercase;letter-spacing:.06em;color:#6b7280;padding:14px 20px;vertical-align:top;}
.detail-value{font-size:15px;line-height:22px;color:#111827;padding:14px 20px;vertical-align:top;}
.footer{padding:24px 32px 32px;}
.closing{font-size:15px;line-height:24px;color:#374151;margin:0 0 18px;}
.note{font-size:12px;line-height:18px;color:#6b7280;margin:0;}
@media screen and (max-width:520px){.wrapper{padding:0;background:#ffffff}.container{border-radius:0;border:0}.header,.hero,.details,.footer{padding-left:20px!important;padding-right:20px!important}.headline{font-size:25px;line-height:31px}.detail-label,.detail-value{display:block;width:auto!important;padding:12px 16px}.detail-label{padding-bottom:2px}.detail-value{padding-top:0}}
</style></head>`)
	b.WriteString(`<body><div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;">`)
	b.WriteString(html.EscapeString(view.Preheader))
	b.WriteString(`</div><table role="presentation" class="wrapper" width="100%"><tr><td align="center">`)
	b.WriteString(`<table role="presentation" class="container" width="100%">`)
	b.WriteString(`<tr><td class="header"><table role="presentation" width="100%"><tr><td class="brand">Relaxation Hub</td><td align="right"><span class="badge" style="background:`)
	b.WriteString(view.AccentLight)
	b.WriteString(`;color:`)
	b.WriteString(view.Accent)
	b.WriteString(`;">`)
	b.WriteString(html.EscapeString(view.Badge))
	b.WriteString(`</span></td></tr></table></td></tr>`)
	b.WriteString(`<tr><td class="hero" style="border-top:4px solid `)
	b.WriteString(view.Accent)
	b.WriteString(`;"><p class="eyebrow" style="color:`)
	b.WriteString(view.Accent)
	b.WriteString(`;">`)
	b.WriteString(html.EscapeString(view.Eyebrow))
	b.WriteString(`</p><h1 class="headline">`)
	b.WriteString(html.EscapeString(view.Headline))
	b.WriteString(`</h1><p class="intro">`)
	b.WriteString(html.EscapeString(view.Intro))
	b.WriteString(`</p></td></tr>`)
	b.WriteString(`<tr><td class="details"><table role="presentation" class="detail-card" width="100%"><tr><td class="detail-title" colspan="2">Booking details</td></tr>`)
	for i, detail := range details {
		value := strings.TrimSpace(detail.Value)
		if value == "" {
			continue
		}
		rowClass := "detail-row"
		if i == len(details)-1 {
			rowClass = "detail-row detail-row-last"
		}
		b.WriteString(`<tr class="`)
		b.WriteString(rowClass)
		b.WriteString(`"><td class="detail-label">`)
		b.WriteString(html.EscapeString(detail.Label))
		b.WriteString(`</td><td class="detail-value">`)
		b.WriteString(html.EscapeString(value))
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</table></td></tr>`)
	b.WriteString(`<tr><td class="footer"><p class="closing">`)
	b.WriteString(html.EscapeString(view.Closing))
	b.WriteString(`</p><p class="note">This is an automated booking update from Relaxation Hub. If anything looks wrong, please open your booking details in the app.</p></td></tr>`)
	b.WriteString(`</table></td></tr></table></body></html>`)
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
