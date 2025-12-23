package model

import "time"

// Service represents a catalog entry from the services table.
type Service struct {
	ServiceID       int64      `json:"service_id"`
	Name            string     `json:"name"`
	Description     string     `json:"description,omitempty"`
	BasePrice       float64    `json:"base_price"`
	DurationMinutes int        `json:"duration_minutes"`
	Category        string     `json:"category,omitempty"`
	PreviewImageURL string     `json:"preview_image_url,omitempty"`
	IsActive        bool       `json:"is_active"`
	DeletedAt       *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
}

// CreateServiceRequest captures the payload for creating a service.
type CreateServiceRequest struct {
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	BasePrice       float64 `json:"base_price"`
	DurationMinutes int     `json:"duration_minutes"`
	Category        string  `json:"category"`
	PreviewImageURL *string `json:"preview_image_url,omitempty"`
	IsActive        *bool   `json:"is_active"`
}
