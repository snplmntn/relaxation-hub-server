package service

import (
	"context"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"time"
)

const customerBookingLeadTime = 2 * time.Hour

func validateBookingLeadTime(ctx context.Context, scheduledStart, now time.Time) error {
	if !model.IsHotelRole(middleware.UserRoleFromContext(ctx)) {
		return validateCustomerBookingLeadTime(scheduledStart, now)
	}
	if scheduledStart.Before(now) {
		return NewValidationError("booking_lead_time", "Choose a booking time that is not in the past.", map[string]string{"scheduled_start": "must be now or later"})
	}
	return nil
}

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
