package service

import (
	"testing"
	"time"
)

func TestValidateCustomerBookingLeadTime(t *testing.T) {
	now := time.Date(2026, time.August, 31, 15, 37, 0, 0, time.FixedZone("Asia/Manila", 8*60*60))

	if err := validateCustomerBookingLeadTime(now.Add(2*time.Hour), now); err != nil {
		t.Fatalf("expected exact two-hour lead time to pass, got %v", err)
	}

	err := validateCustomerBookingLeadTime(now.Add(2*time.Hour-time.Second), now)
	if err == nil {
		t.Fatal("expected a schedule under two hours away to fail")
	}
	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Code != "booking_lead_time" {
		t.Fatalf("expected booking_lead_time code, got %q", validationErr.Code)
	}
}
