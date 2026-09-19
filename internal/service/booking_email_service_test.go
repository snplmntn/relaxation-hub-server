package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/require"
)

type fakeEmailSender struct {
	messages []EmailMessage
	err      error
}

func (f *fakeEmailSender) Send(ctx context.Context, msg EmailMessage) error {
	if f.err != nil {
		return f.err
	}
	f.messages = append(f.messages, msg)
	return nil
}

type fakeBookingEmailStore struct {
	details       *repository.BookingDetailsResult
	events        []model.BookingEvent
	insertedEvent string
}

func (f *fakeBookingEmailStore) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return f.details, nil
}

func (f *fakeBookingEmailStore) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	return f.events, nil
}

func (f *fakeBookingEmailStore) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	f.insertedEvent = eventType
	f.events = append(f.events, model.BookingEvent{BookingID: bookingID, EventType: eventType})
	return nil
}

type fakeBookingEmailUserStore struct {
	user *model.User
}

func (f *fakeBookingEmailUserStore) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	return f.user, nil
}

type fakeBookingAnnouncementResolver struct {
	announcements []model.ResolvedBookingAnnouncement
}

func (f *fakeBookingAnnouncementResolver) ResolveForBooking(context.Context, int64) ([]model.ResolvedBookingAnnouncement, error) {
	return f.announcements, nil
}

func TestRenderBookingEmail(t *testing.T) {
	total := 1200.0
	msg := RenderBookingEmail("advanced_booking_confirmed", BookingEmailData{
		ClientName:    "Maria",
		TherapistName: "Anna",
		ServiceName:   "Swedish Massage",
		ScheduledDate: "Tuesday, May 12, 2026",
		ScheduledTime: "10:00 AM",
		ReferenceCode: "RH-100",
		Duration:      "1 hour",
		Total:         formatAmount(&total),
		Address:       "Makati",
	})

	if msg.Subject != "Your Kalinga Spa booking is confirmed" {
		t.Fatalf("unexpected subject: %s", msg.Subject)
	}
	if !strings.Contains(msg.TextBody, "RH-100") || !strings.Contains(msg.HTMLBody, "Swedish Massage") {
		t.Fatalf("rendered email is missing booking details")
	}
}

func TestBookingEmailServiceRecordsEventAfterSend(t *testing.T) {
	location := time.FixedZone("Asia/Manila", 8*60*60)
	scheduled := time.Date(2026, 5, 13, 10, 0, 0, 0, location)
	store := &fakeBookingEmailStore{
		details: &repository.BookingDetailsResult{
			Booking: &model.Booking{
				BookingID:       10,
				ClientID:        7,
				ScheduledStart:  &scheduled,
				DurationMinutes: 60,
			},
			Service: &model.Service{Name: "Swedish Massage"},
		},
	}
	userStore := &fakeBookingEmailUserStore{user: &model.User{UserID: 7, FullName: "Maria", PrimaryEmail: "maria@example.com"}}
	sender := &fakeEmailSender{}
	svc := NewBookingEmailService(store, userStore, sender, location)
	svc.now = func() time.Time { return time.Date(2026, 5, 12, 8, 0, 0, 0, location) }

	svc.SendAdvancedBookingConfirmed(context.Background(), store.details.Booking)

	if len(sender.messages) != 1 {
		t.Fatalf("expected 1 email, got %d", len(sender.messages))
	}
	if store.insertedEvent != BookingEmailEventAdvancedConfirmed {
		t.Fatalf("expected sent event %q, got %q", BookingEmailEventAdvancedConfirmed, store.insertedEvent)
	}
}

func TestBookingEmailServiceSkipsDuplicateEvent(t *testing.T) {
	location := time.FixedZone("Asia/Manila", 8*60*60)
	scheduled := time.Date(2026, 5, 13, 10, 0, 0, 0, location)
	store := &fakeBookingEmailStore{
		details: &repository.BookingDetailsResult{
			Booking: &model.Booking{BookingID: 10, ClientID: 7, ScheduledStart: &scheduled},
		},
		events: []model.BookingEvent{{BookingID: 10, EventType: BookingEmailEventAdvancedConfirmed}},
	}
	userStore := &fakeBookingEmailUserStore{user: &model.User{UserID: 7, FullName: "Maria", PrimaryEmail: "maria@example.com"}}
	sender := &fakeEmailSender{}
	svc := NewBookingEmailService(store, userStore, sender, location)

	svc.SendAdvancedBookingConfirmed(context.Background(), store.details.Booking)

	if len(sender.messages) != 0 {
		t.Fatalf("expected duplicate email to be skipped")
	}
}

func TestBookingEmailServiceSkipsAdvancedConfirmedForSameDay(t *testing.T) {
	location := time.FixedZone("Asia/Manila", 8*60*60)
	scheduled := time.Date(2026, 5, 12, 10, 0, 0, 0, location)
	store := &fakeBookingEmailStore{
		details: &repository.BookingDetailsResult{
			Booking: &model.Booking{BookingID: 10, ClientID: 7, ScheduledStart: &scheduled},
		},
	}
	userStore := &fakeBookingEmailUserStore{user: &model.User{UserID: 7, PrimaryEmail: "maria@example.com"}}
	sender := &fakeEmailSender{}
	svc := NewBookingEmailService(store, userStore, sender, location)
	svc.now = func() time.Time { return time.Date(2026, 5, 12, 7, 0, 0, 0, location) }

	svc.SendAdvancedBookingConfirmed(context.Background(), store.details.Booking)

	if len(sender.messages) != 0 {
		t.Fatalf("expected same-day advanced confirmation email to be skipped")
	}
}

func TestBookingEmailServiceAddsAnnouncementsToCompletedEmailOnly(t *testing.T) {
	location := time.FixedZone("Asia/Manila", 8*60*60)
	scheduled := time.Date(2026, 9, 19, 21, 0, 0, 0, location)
	store := &fakeBookingEmailStore{
		details: &repository.BookingDetailsResult{
			Booking: &model.Booking{BookingID: 10, ClientID: 7, ScheduledStart: &scheduled},
		},
	}
	userStore := &fakeBookingEmailUserStore{user: &model.User{UserID: 7, FullName: "Maria", PrimaryEmail: "maria@example.com"}}
	resolver := &fakeBookingAnnouncementResolver{announcements: []model.ResolvedBookingAnnouncement{
		{AnnouncementID: 1, VariationID: 2, Message: "Bed cover for PHP 50 <today>."},
	}}
	sender := &fakeEmailSender{}
	svc := NewBookingEmailService(store, userStore, sender, location)
	svc.SetBookingAnnouncementResolver(resolver)

	svc.SendBookingCompleted(context.Background(), store.details.Booking)

	require.Len(t, sender.messages, 1)
	require.Contains(t, sender.messages[0].TextBody, "Announcements\nAnnouncement: Bed cover for PHP 50 <today>.")
	require.Contains(t, sender.messages[0].HTMLBody, "Bed cover for PHP 50 &lt;today&gt;.")

	completed := sender.messages[0]
	sender.messages = nil
	store.events = nil
	svc.SendTherapistOnTheWay(context.Background(), store.details.Booking)
	require.Len(t, sender.messages, 1)
	require.NotContains(t, sender.messages[0].TextBody, "Announcements")
	require.NotContains(t, sender.messages[0].HTMLBody, "Bed cover for PHP 50")
	require.Contains(t, completed.HTMLBody, "Announcements")
}
