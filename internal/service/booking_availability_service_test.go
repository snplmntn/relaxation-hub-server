package service

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type bookingAvailabilityMatcherStub struct {
	candidates        map[int64][]int64
	starts            []time.Time
	genderPreferences []string
}

func (s *bookingAvailabilityMatcherStub) FindAvailableTherapistsForServiceWithTime(
	_ context.Context,
	_ int64,
	serviceID int64,
	genderPreference, _ string,
	start time.Time,
	_ int,
	_, _ *float64,
) ([]model.TherapistProfile, error) {
	s.starts = append(s.starts, start)
	s.genderPreferences = append(s.genderPreferences, genderPreference)
	profiles := make([]model.TherapistProfile, 0, len(s.candidates[serviceID]))
	for _, therapistID := range s.candidates[serviceID] {
		profiles = append(profiles, model.TherapistProfile{TherapistID: therapistID})
	}
	return profiles, nil
}

type bookingAvailabilityAddressStoreStub struct{}

func (bookingAvailabilityAddressStoreStub) GetByID(context.Context, int64, int64) (*model.Address, error) {
	return &model.Address{AddressID: 5}, nil
}

func newBookingAvailabilityService(matcher *bookingAvailabilityMatcherStub) *BookingAvailabilityService {
	service := NewBookingAvailabilityService(matcher, bookingAvailabilityAddressStoreStub{})
	service.now = func() time.Time {
		return time.Date(2026, 7, 25, 12, 0, 0, 0, time.FixedZone("PHT", 8*60*60))
	}
	return service
}

func TestBookingAvailabilityNormalRequiresOneTherapistForEveryService(t *testing.T) {
	matcher := &bookingAvailabilityMatcherStub{
		candidates: map[int64][]int64{1: {10, 11}, 2: {11, 12}},
	}
	result, err := newBookingAvailabilityService(matcher).Check(context.Background(), 7, &BookingAvailabilityRequest{
		Mode:           BookingAvailabilityModeSingle,
		AddressID:      5,
		ScheduledStart: "2026-07-26T15:00:00+08:00",
		Sessions: []BookingAvailabilitySession{{
			ServiceIDs:       []int64{1, 2},
			DurationMinutes:  150,
			GenderPreference: "female",
		}},
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !result.Available {
		t.Fatal("expected shared therapist to make the booking available")
	}
	for _, preference := range matcher.genderPreferences {
		if preference != "any" {
			t.Fatalf("expected availability to ignore gender filtering, got %q", preference)
		}
	}
}

func TestBookingAvailabilityTandemRequiresDistinctTherapists(t *testing.T) {
	matcher := &bookingAvailabilityMatcherStub{
		candidates: map[int64][]int64{1: {10}},
	}
	result, err := newBookingAvailabilityService(matcher).Check(context.Background(), 7, &BookingAvailabilityRequest{
		Mode:           BookingAvailabilityModeTandem,
		AddressID:      5,
		ScheduledStart: "2026-07-26T15:00:00+08:00",
		Sessions: []BookingAvailabilitySession{
			{ServiceIDs: []int64{1}, DurationMinutes: 60},
			{ServiceIDs: []int64{1}, DurationMinutes: 60},
		},
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if result.Available {
		t.Fatal("expected one therapist to be insufficient for tandem sessions")
	}
}

func TestBookingAvailabilityGroupChecksSequentialStarts(t *testing.T) {
	matcher := &bookingAvailabilityMatcherStub{
		candidates: map[int64][]int64{1: {10}, 2: {10}},
	}
	result, err := newBookingAvailabilityService(matcher).Check(context.Background(), 7, &BookingAvailabilityRequest{
		Mode:           BookingAvailabilityModeGroup,
		AddressID:      5,
		ScheduledStart: "2026-07-26T15:00:00+08:00",
		Sessions: []BookingAvailabilitySession{
			{ServiceIDs: []int64{1}, DurationMinutes: 60},
			{ServiceIDs: []int64{2}, DurationMinutes: 90},
		},
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !result.Available {
		t.Fatal("expected shared therapist to cover the sequence")
	}
	if got := matcher.starts[1].Sub(matcher.starts[0]); got != time.Hour {
		t.Fatalf("expected second session one hour later, got %v", got)
	}
}
