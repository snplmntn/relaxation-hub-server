package model

import "time"

// Service represents a catalog entry from the services table.
type Service struct {
	ServiceID          int64      `json:"service_id"`
	Name               string     `json:"name"`
	Description        string     `json:"description,omitempty"`
	BasePrice          float64    `json:"base_price"`
	MinDurationMinutes int        `json:"min_duration_minutes"`
	DeletedAt          *time.Time `json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
}

// CreateServiceRequest captures the payload for creating a service.
type CreateServiceRequest struct {
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	BasePrice          float64 `json:"base_price"`
	MinDurationMinutes int     `json:"min_duration_minutes"`
}

