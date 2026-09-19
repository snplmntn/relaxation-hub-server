package service

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/mock"
)

func TestUpcomingBookingWorkerSendsDDayEmailAfterConfiguredHour(t *testing.T) {
	location := time.FixedZone("Asia/Manila", 8*60*60)
	scheduled := time.Date(2026, 5, 12, 10, 0, 0, 0, location)
	booking := model.Booking{BookingID: 10, ClientID: 7, ScheduledStart: &scheduled}
	repo := &MockBookingRepository{}
	repo.On("ListUpcomingBookingsForReminder", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"), BookingEmailEventDDay).
		Return([]model.Booking{booking}, nil).Once()
	repo.On("ListEvents", mock.Anything, booking.BookingID).Return([]model.BookingEvent{}, nil).Once()
	repo.On("GetBookingWithDetailsUnsafe", mock.Anything, booking.BookingID).
		Return(&repository.BookingDetailsResult{Booking: &booking}, nil).Once()
	repo.On("InsertEvent", mock.Anything, booking.BookingID, BookingEmailEventDDay, (*int64)(nil), mock.Anything).Return(nil).Once()
	userStore := &fakeBookingEmailUserStore{user: &model.User{UserID: 7, PrimaryEmail: "maria@example.com"}}
	sender := &fakeEmailSender{}
	emailSvc := NewBookingEmailService(repo, userStore, sender, location)
	emailSvc.now = func() time.Time { return time.Date(2026, 5, 12, 8, 0, 0, 0, location) }
	worker := NewUpcomingBookingWorker(nil, nil)
	worker.SetBookingEmailService(emailSvc, location, 7)
	worker.bookingRepo = repo

	worker.sendDDayEmails(context.Background(), time.Date(2026, 5, 12, 8, 0, 0, 0, location))

	if len(sender.messages) != 1 {
		t.Fatalf("expected 1 d-day email, got %d", len(sender.messages))
	}
	repo.AssertExpectations(t)
}

func TestUpcomingBookingWorkerSkipsDDayEmailBeforeConfiguredHour(t *testing.T) {
	location := time.FixedZone("Asia/Manila", 8*60*60)
	repo := &MockBookingRepository{}
	emailSvc := NewBookingEmailService(repo, &fakeBookingEmailUserStore{}, &fakeEmailSender{}, location)
	worker := NewUpcomingBookingWorker(nil, nil)
	worker.SetBookingEmailService(emailSvc, location, 7)
	worker.bookingRepo = repo

	worker.sendDDayEmails(context.Background(), time.Date(2026, 5, 12, 6, 0, 0, 0, location))

	repo.AssertNotCalled(t, "ListUpcomingBookingsForReminder", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
