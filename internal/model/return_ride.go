package model

import "time"

type ReturnRideDestination string

const (
	ReturnRideDestinationNextBooking ReturnRideDestination = "next_booking"
	ReturnRideDestinationBranch      ReturnRideDestination = "branch"
	ReturnRideDestinationHome        ReturnRideDestination = "home"
)

type ReturnRideOption struct {
	Destination    ReturnRideDestination `json:"destination"`
	Label          string                `json:"label"`
	Address        string                `json:"address,omitempty"`
	Latitude       *float64              `json:"latitude,omitempty"`
	Longitude      *float64              `json:"longitude,omitempty"`
	BookingID      *int64                `json:"booking_id,omitempty"`
	Available      bool                  `json:"available"`
	DisabledReason string                `json:"disabled_reason,omitempty"`
}

type ReturnRideState struct {
	Ride                  *Ride                 `json:"ride,omitempty"`
	Destination           ReturnRideDestination `json:"destination,omitempty"`
	DestinationLabel      string                `json:"destination_label,omitempty"`
	ScheduledFor          *time.Time            `json:"scheduled_for,omitempty"`
	DestinationOverridden bool                  `json:"destination_overridden"`
	ScheduleOverridden    bool                  `json:"schedule_overridden"`
	ActivationError       string                `json:"activation_error,omitempty"`
	Options               []ReturnRideOption    `json:"options,omitempty"`
	Ready                 bool                  `json:"ready"`
}

type UpdateReturnRideRequest struct {
	Destination  *ReturnRideDestination `json:"destination,omitempty"`
	ScheduledFor *string                `json:"scheduled_for,omitempty"`
}
