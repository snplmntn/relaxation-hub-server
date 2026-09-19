package handler

import (
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestToBookingResponseIncludesHydratedServices(t *testing.T) {
	primaryID := int64(5)
	booking := &model.Booking{
		BookingID: 42,
		ClientID:  7,
		ServiceID: &primaryID,
		Services: []model.BookingService{
			{ServiceID: 5, PriceSnapshot: 700, DurationSnapshot: 120, Service: &model.Service{ServiceID: 5, Name: "Signature Massage", BasePrice: 999, DurationMinutes: 90}},
			{ServiceID: 6, PriceSnapshot: 500, DurationSnapshot: 60, Service: &model.Service{ServiceID: 6, Name: "Foot Massage", BasePrice: 600, DurationMinutes: 45}},
		},
	}

	response := toBookingResponse(booking, booking.Services[0].Service, nil, nil, "", "", "", "", nil, "", "", "", "", "")

	if len(response.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(response.Services))
	}
	if response.Services[0].BasePrice != 700 || response.Services[0].DurationMinutes != 120 {
		t.Fatalf("expected booking snapshots in response, got price=%v duration=%v", response.Services[0].BasePrice, response.Services[0].DurationMinutes)
	}
}
