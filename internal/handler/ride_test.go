package handler

import "testing"

func TestNormalizeRideStatusUsesCanonicalRideLifecycle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		valid    bool
	}{
		{name: "canonical arrived pickup", input: "arrived_pickup", expected: "arrived_pickup", valid: true},
		{name: "canonical in progress", input: "in_progress", expected: "in_progress", valid: true},
		{name: "legacy arrived alias", input: "arrived", expected: "arrived_pickup", valid: true},
		{name: "legacy picked up alias", input: "picked_up", expected: "in_progress", valid: true},
		{name: "legacy dropped off alias", input: "dropped_off", expected: "arrived_dropoff", valid: true},
		{name: "booking status is not a ride transition", input: "on_the_way", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := normalizeRideStatus(tt.input)
			if valid != tt.valid {
				t.Fatalf("valid = %v, want %v", valid, tt.valid)
			}
			if got != tt.expected {
				t.Fatalf("status = %q, want %q", got, tt.expected)
			}
		})
	}
}
