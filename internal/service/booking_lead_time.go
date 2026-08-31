package service

import "time"

const customerBookingLeadTime = 2 * time.Hour

func validateCustomerBookingLeadTime(scheduledStart, now time.Time) error {
	if scheduledStart.Before(now.Add(customerBookingLeadTime)) {
		return NewValidationError(
			"booking_lead_time",
			"Online bookings must be scheduled at least two hours in advance.",
			map[string]string{"scheduled_start": "must be at least two hours from now"},
		)
	}
	return nil
}
