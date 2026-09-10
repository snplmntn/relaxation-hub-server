package service

import (
	"context"
	"time"
)

const customerBookingLeadTime = 2 * time.Hour

func validateBookingLeadTime(_ context.Context, scheduledStart, now time.Time) error {
	return validateCustomerBookingLeadTime(scheduledStart, now)
}

func validateCustomerBookingLeadTime(scheduledStart, now time.Time) error {
	if scheduledStart.Before(now.Add(customerBookingLeadTime)) {
		return NewValidationError(
			"booking_lead_time",
			"Bookings must be scheduled at least two hours in advance.",
			map[string]string{"scheduled_start": "must be at least two hours from now"},
		)
	}
	return nil
}
