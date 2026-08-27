package service

import (
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestCalculateBookingServicesTherapistEarningsUsesEachServiceRateAndAllocatedDuration(t *testing.T) {
	lymphaticCommission := 190.0
	footCommission := 240.0
	sixtyMinutes := 60
	items := []model.BookingService{
		{
			PriceSnapshot:            549,
			DurationSnapshot:         60,
			AllocatedDurationMinutes: &sixtyMinutes,
			Service: &model.Service{
				TherapistCommission: &lymphaticCommission,
			},
		},
		{
			PriceSnapshot:            649,
			DurationSnapshot:         60,
			AllocatedDurationMinutes: &sixtyMinutes,
			Service: &model.Service{
				TherapistCommission: &footCommission,
			},
		},
	}

	earnings := CalculateBookingServicesTherapistEarnings(items, 120)
	if earnings == nil || *earnings != 430 {
		t.Fatalf("expected multi-service earnings 430, got %v", earnings)
	}
}

func TestCalculateBookingServicesTherapistEarningsUsesUnevenAllocatedDurations(t *testing.T) {
	firstCommission := 190.0
	secondCommission := 240.0
	thirtyMinutes := 30
	ninetyMinutes := 90
	items := []model.BookingService{
		{PriceSnapshot: 549, DurationSnapshot: 60, AllocatedDurationMinutes: &thirtyMinutes, Service: &model.Service{TherapistCommission: &firstCommission}},
		{PriceSnapshot: 649, DurationSnapshot: 60, AllocatedDurationMinutes: &ninetyMinutes, Service: &model.Service{TherapistCommission: &secondCommission}},
	}

	earnings := CalculateBookingServicesTherapistEarnings(items, 120)
	if earnings == nil || *earnings != 455 {
		t.Fatalf("expected allocated-duration earnings 455, got %v", earnings)
	}
}
