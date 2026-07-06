package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func TestParseSlotStart(t *testing.T) {
	// Valid: parsed as Manila local time (UTC+8).
	got, ok := parseSlotStart("2026-07-06", "15:00")
	if !ok {
		t.Fatal("expected valid parse")
	}
	if _, offset := got.Zone(); offset != 8*60*60 {
		t.Fatalf("expected +8h offset, got %d", offset)
	}
	if got.Hour() != 15 || got.Day() != 6 {
		t.Fatalf("unexpected time: %v", got)
	}

	// Invalid / empty inputs must be rejected.
	for _, tc := range []struct{ date, clock string }{
		{"", "15:00"},
		{"2026-07-06", ""},
		{"2026-13-40", "15:00"},
		{"2026-07-06", "25:00"},
		{"july 6", "3pm"},
	} {
		if _, ok := parseSlotStart(tc.date, tc.clock); ok {
			t.Errorf("expected reject for %q %q", tc.date, tc.clock)
		}
	}
}

func TestParseAvailabilityInt(t *testing.T) {
	if got, ok := parseAvailabilityInt("", 60, 30, 240); !ok || got != 60 {
		t.Fatalf("expected default 60, got %d ok=%v", got, ok)
	}
	if got, ok := parseAvailabilityInt("120", 60, 30, 240); !ok || got != 120 {
		t.Fatalf("expected 120, got %d ok=%v", got, ok)
	}
	for _, raw := range []string{"29", "241", "abc"} {
		if _, ok := parseAvailabilityInt(raw, 60, 30, 240); ok {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestCheckAvailabilityReturnsAlternativesWhenUnavailable(t *testing.T) {
	start := time.Date(2026, time.July, 6, 19, 0, 0, 0, manilaLoc)
	matching := &fakeAvailabilityMatchingService{
		available: false,
		alternatives: []service.AvailabilitySlot{
			{Start: start.Add(30 * time.Minute)},
			{Start: start.Add(90 * time.Minute)},
		},
	}
	handler := NewAvailabilityHandler(matching)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/availability?date=2026-07-06&time=19:00&duration_min=120&quantity=2", nil)
	rec := httptest.NewRecorder()

	handler.CheckAvailability(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body availabilityResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Available {
		t.Fatal("expected unavailable response")
	}
	if matching.durationMin != 120 {
		t.Fatalf("expected duration 120, got %d", matching.durationMin)
	}
	if matching.quantity != 2 {
		t.Fatalf("expected quantity 2, got %d", matching.quantity)
	}
	if len(body.Alternatives) != 2 {
		t.Fatalf("expected 2 alternatives, got %d", len(body.Alternatives))
	}
	if body.Alternatives[0].Date != "2026-07-06" || body.Alternatives[0].Time != "19:30" {
		t.Fatalf("unexpected first alternative: %+v", body.Alternatives[0])
	}
	if body.Alternatives[1].Date != "2026-07-06" || body.Alternatives[1].Time != "20:30" {
		t.Fatalf("unexpected second alternative: %+v", body.Alternatives[1])
	}
}

type fakeAvailabilityMatchingService struct {
	available    bool
	alternatives []service.AvailabilitySlot
	durationMin  int
	quantity     int
}

func (f *fakeAvailabilityMatchingService) FindAvailableTherapistsForService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return nil, nil
}

func (f *fakeAvailabilityMatchingService) FindNearbyAvailableTherapists(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return nil, nil
}

func (f *fakeAvailabilityMatchingService) FindAvailableTherapistsForServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error) {
	return nil, nil
}

func (f *fakeAvailabilityMatchingService) IsSlotAvailable(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity int) (bool, error) {
	f.durationMin = durationMinutes
	f.quantity = quantity
	return f.available, nil
}

func (f *fakeAvailabilityMatchingService) FindAlternativeSlots(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity, limit int) ([]service.AvailabilitySlot, error) {
	f.durationMin = durationMinutes
	f.quantity = quantity
	if len(f.alternatives) > limit {
		return f.alternatives[:limit], nil
	}
	return f.alternatives, nil
}
