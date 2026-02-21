package model

import "time"

// BookingEvent represents a row in the booking_events table
type BookingEvent struct {
	EventID   int64          `db:"event_id" json:"event_id"`
	BookingID int64          `db:"booking_id" json:"booking_id"`
	EventType string         `db:"event_type" json:"event_type"`
	ActorID   *int64         `db:"actor_id" json:"actor_id,omitempty"`
	ActorType string         `db:"actor_type" json:"actor_type,omitempty"`
	ActorName string         `db:"-" json:"actor_name,omitempty"`
	Metadata  map[string]any `db:"metadata" json:"metadata,omitempty"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
}

const (
	EventTypeRideRequested       = "RIDE_REQUESTED"
	EventTypeRideOffered         = "RIDE_OFFERED"
	EventTypeRideAccepted        = "RIDE_ACCEPTED"
	EventTypeRideArrivedPickup   = "RIDE_ARRIVED_PICKUP"
	EventTypeRideInProgress      = "RIDE_IN_PROGRESS"
	EventTypeRideArrivedDropoff  = "RIDE_ARRIVED_DROPOFF"
	EventTypeRideCompleted       = "RIDE_COMPLETED"
	EventTypeRideCancelled       = "RIDE_CANCELLED"
	EventTypeRideDispatchRetried = "RIDE_DISPATCH_RETRIED"
)
