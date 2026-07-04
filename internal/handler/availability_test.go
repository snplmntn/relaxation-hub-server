package handler

import "testing"

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
