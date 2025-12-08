package model

import "time"

// EmergencyAlert represents the emergency_alerts table.
type EmergencyAlert struct {
	AlertID        int64      `db:"alert_id" json:"alert_id"`
	BookingID      int64      `db:"booking_id" json:"booking_id"`
	TriggeredBy    int64      `db:"triggered_by" json:"triggered_by"`
	TriggeredAt    time.Time  `db:"triggered_at" json:"triggered_at"`
	LocationLat    *float64   `db:"location_lat" json:"location_lat,omitempty"`
	LocationLng    *float64   `db:"location_lng" json:"location_lng,omitempty"`
	Status         string     `db:"status" json:"status"`
	Resolved       bool       `db:"resolved" json:"resolved"`
	ResolvedAt     *time.Time `db:"resolved_at" json:"resolved_at,omitempty"`
	ResolvedBy     *int64     `db:"resolved_by" json:"resolved_by,omitempty"`
	ResolutionNote string     `db:"resolution_notes" json:"resolution_notes"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

// CreateEmergencyAlertRequest for triggering emergency.
type CreateEmergencyAlertRequest struct {
	BookingID   int64    `json:"booking_id"`
	LocationLat *float64 `json:"location_lat"`
	LocationLng *float64 `json:"location_lng"`
}

// ResolveEmergencyAlertRequest for resolving alert.
type ResolveEmergencyAlertRequest struct {
	Status         string `json:"status"`
	ResolutionNote string `json:"resolution_notes"`
}

// EmergencyAlertResponse to clients.
type EmergencyAlertResponse struct {
	AlertID        int64      `json:"alert_id"`
	BookingID      int64      `json:"booking_id"`
	TriggeredBy    int64      `json:"triggered_by"`
	TriggeredAt    time.Time  `json:"triggered_at"`
	LocationLat    *float64   `json:"location_lat,omitempty"`
	LocationLng    *float64   `json:"location_lng,omitempty"`
	Status         string     `json:"status"`
	Resolved       bool       `json:"resolved"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy     *int64     `json:"resolved_by,omitempty"`
	ResolutionNote string     `json:"resolution_notes"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
