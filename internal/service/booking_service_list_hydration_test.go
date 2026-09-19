package service

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type bookingServiceBatchHydrationStub struct {
	repository.BookingServiceRepository
	servicesByBookingID map[int64][]model.BookingService
}

func (s *bookingServiceBatchHydrationStub) ListByBookingIDsWithService(_ context.Context, _ []int64) (map[int64][]model.BookingService, error) {
	return s.servicesByBookingID, nil
}

func TestHydrateBookingServicesForDetailsPreservesMultipleServices(t *testing.T) {
	details := []repository.BookingDetailsResult{
		{Booking: &model.Booking{BookingID: 42}},
		{Booking: &model.Booking{BookingID: 43}},
	}
	stub := &bookingServiceBatchHydrationStub{
		servicesByBookingID: map[int64][]model.BookingService{
			42: {
				{BookingID: 42, ServiceID: 5, Position: 0, Service: &model.Service{Name: "Hilot"}},
				{BookingID: 42, ServiceID: 6, Position: 1, Service: &model.Service{Name: "Deep Tissue"}},
			},
			43: {
				{BookingID: 43, ServiceID: 7, Position: 0, Service: &model.Service{Name: "Swedish"}},
			},
		},
	}
	svc := &BookingService{bookingServiceRepo: stub}

	if err := svc.hydrateBookingServicesForDetails(context.Background(), details); err != nil {
		t.Fatalf("hydrate booking services: %v", err)
	}

	if got := len(details[0].Booking.Services); got != 2 {
		t.Fatalf("booking 42 service count = %d, want 2", got)
	}
	if got := details[0].Booking.Services[1].Service.Name; got != "Deep Tissue" {
		t.Fatalf("booking 42 second service = %q, want Deep Tissue", got)
	}
	if got := len(details[1].Booking.Services); got != 1 {
		t.Fatalf("booking 43 service count = %d, want 1", got)
	}
}
