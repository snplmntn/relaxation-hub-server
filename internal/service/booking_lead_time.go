package service

import (
	"context"
	"fmt"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"time"
)

const customerBookingLeadTime = 2 * time.Hour
const hotelBookingLeadTime = time.Hour

func validateBookingLeadTime(ctx context.Context, scheduledStart, now time.Time) error {
	leadTime := customerBookingLeadTime
	role := middleware.UserRoleFromContext(ctx)
	if role == model.RoleHotelAdmin || role == model.RoleHotelStaff {
		leadTime = hotelBookingLeadTime
	}
	return validateBookingLeadTimeDuration(scheduledStart, now, leadTime)
}

func validateCustomerBookingLeadTime(scheduledStart, now time.Time) error {
	return validateBookingLeadTimeDuration(scheduledStart, now, customerBookingLeadTime)
}

func validateBookingLeadTimeDuration(scheduledStart, now time.Time, leadTime time.Duration) error {
	if scheduledStart.Before(now.Add(leadTime)) {
		hours := int(leadTime / time.Hour)
		unit := "hours"
		if hours == 1 {
			unit = "hour"
		}
		return NewValidationError(
			"booking_lead_time",
			fmt.Sprintf("Bookings must be scheduled at least %d %s in advance.", hours, unit),
			map[string]string{"scheduled_start": fmt.Sprintf("must be at least %d %s from now", hours, unit)},
		)
	}
	return nil
}
