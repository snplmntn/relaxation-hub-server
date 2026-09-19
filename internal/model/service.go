package model

import "time"

// Service represents a catalog entry from the services table.
type Service struct {
	ServiceID           int64      `json:"service_id"`
	Name                string     `json:"name"`
	Description         string     `json:"description,omitempty"`
	BasePrice           float64    `json:"base_price"`
	DurationMinutes     int        `json:"duration_minutes"`
	Category            string     `json:"category,omitempty"`
	PreviewImageURL     string     `json:"preview_image_url,omitempty"`
	TherapistCommission *float64   `json:"therapist_commission,omitempty"` // Amount therapist earns for base duration
	Subtitle            string     `json:"subtitle,omitempty"`             // Tagline shown under the name on the landing page
	IsFeatured          bool       `json:"is_featured"`                    // Whether the service appears in the landing page Services section
	FeaturedOrder       int        `json:"featured_order"`                 // Sort order within the landing page Services section
	IsActive            bool       `json:"is_active"`
	DeletedAt           *time.Time `json:"-"`
	CreatedAt           time.Time  `json:"created_at"`
}

// CreateServiceRequest captures the payload for creating a service.
type CreateServiceRequest struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	BasePrice           float64  `json:"base_price"`
	DurationMinutes     int      `json:"duration_minutes"`
	Category            string   `json:"category"`
	PreviewImageURL     *string  `json:"preview_image_url,omitempty"`
	TherapistCommission *float64 `json:"therapist_commission,omitempty"`
	Subtitle            *string  `json:"subtitle,omitempty"`
	IsFeatured          *bool    `json:"is_featured,omitempty"`
	FeaturedOrder       *int     `json:"featured_order,omitempty"`
	IsActive            *bool    `json:"is_active"`
}
