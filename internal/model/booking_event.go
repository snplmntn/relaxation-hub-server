package model

import "time"

// BookingEvent represents a row in the booking_events table
type BookingEvent struct {
	EventID   int64          `db:"event_id" json:"event_id"`
	BookingID int64          `db:"booking_id" json:"booking_id"`
	EventType string         `db:"event_type" json:"event_type"`
	ActorID   *int64         `db:"actor_id" json:"actor_id,omitempty"`
	Metadata  map[string]any `db:"metadata" json:"metadata,omitempty"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
}
