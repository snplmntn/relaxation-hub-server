package model

import "time"

// LiveLocation represents the live_locations table.
type LiveLocation struct {
	LocationID  int64     `db:"location_id" json:"location_id"`
	UserID      int64     `db:"user_id" json:"-"`
	Latitude    float64   `db:"latitude" json:"latitude"`
	Longitude   float64   `db:"longitude" json:"longitude"`
	LastUpdated time.Time `db:"last_updated" json:"last_updated"`
}

// UpdateLocationRequest for updating live location.
type UpdateLocationRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// LiveLocationResponse to clients.
type LiveLocationResponse struct {
	LocationID  int64     `json:"location_id"`
	UserID      int64     `json:"user_id"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	LastUpdated time.Time `json:"last_updated"`
}
