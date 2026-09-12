package service

import (
	"context"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"testing"
	"time"
)

func TestBookingLeadTimeByAuthenticatedRole(t *testing.T) {
	now := time.Date(2026, 9, 9, 17, 0, 0, 0, time.UTC)
	for _, role := range []string{"hotel_admin", "hotel_staff", "client", "admin", ""} {
		t.Run(role, func(t *testing.T) {
			ctx := middleware.SetUserRole(context.Background(), role)
			for _, offset := range []time.Duration{-time.Second, 0, 30 * time.Minute, time.Hour, 2 * time.Hour} {
				err := validateBookingLeadTime(ctx, now.Add(offset), now)
				minimum := 2 * time.Hour
				if role == "hotel_admin" || role == "hotel_staff" {
					minimum = time.Hour
				}
				wantAllowed := offset >= minimum
				if (err == nil) != wantAllowed {
					t.Fatalf("role %q offset %s: got %v", role, offset, err)
				}
			}
		})
	}
}

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
