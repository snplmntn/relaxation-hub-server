package model

import "time"

// ServiceAreaStatus represents the operational status of a service area.
type ServiceAreaStatus string

const (
	ServiceAreaStatusCovered      ServiceAreaStatus = "covered"
	ServiceAreaStatusBanned       ServiceAreaStatus = "banned"
	ServiceAreaStatusNotSupported ServiceAreaStatus = "not_supported"
)

// ServiceAreaLevel represents the geographic level of a service area.
type ServiceAreaLevel string

const (
	ServiceAreaLevelRegion   ServiceAreaLevel = "region"
	ServiceAreaLevelProvince ServiceAreaLevel = "province"
	ServiceAreaLevelCity     ServiceAreaLevel = "city"
	ServiceAreaLevelBarangay ServiceAreaLevel = "barangay"
)

// ServiceArea represents a geographic area with its operational configuration.
type ServiceArea struct {
	AreaID             int64             `db:"area_id" json:"area_id"`
	PSGCCode           string            `db:"psgc_code" json:"psgc_code"`
	ParentCode         *string           `db:"parent_code" json:"parent_code,omitempty"`
	Name               string            `db:"name" json:"name"`
	Level              ServiceAreaLevel  `db:"level" json:"level"`
	Status             ServiceAreaStatus `db:"status" json:"status"`
	Lat                *float64          `db:"lat" json:"lat,omitempty"`
	Lng                *float64          `db:"lng" json:"lng,omitempty"`
	CachedRequestCount int               `db:"cached_request_count" json:"cached_request_count"`
	MinBookingMinutes  int               `db:"min_booking_minutes" json:"min_booking_minutes"`
	CreatedAt          time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time         `db:"updated_at" json:"updated_at"`
}

// AreaCoverageRequest represents a user's interest in service for an unsupported area.
type AreaCoverageRequest struct {
	RequestID int64     `db:"request_id" json:"request_id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	PSGCCode  string    `db:"psgc_code" json:"psgc_code"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// LocationCheckResult represents the result of a location serviceability check.
type LocationCheckResult struct {
	Status     ServiceAreaStatus `json:"status"`
	Message    string            `json:"message"`
	IsAllowed  bool              `json:"is_allowed"`
	AreaName   string            `json:"area_name,omitempty"`
	MinBooking int               `json:"min_booking_minutes,omitempty"`
}

// CheckLocationRequest is the request payload for validating a location.
// Supports both PSGC codes (for dropdown-based selection) and names (for map-based geocoding).
type CheckLocationRequest struct {
	CityCode     string `json:"city_code,omitempty"`
	BarangayCode string `json:"barangay_code,omitempty"`
	CityName     string `json:"city_name,omitempty"`
	BarangayName string `json:"barangay_name,omitempty"`
}

// RecordInterestRequest is the request payload for recording coverage interest.
type RecordInterestRequest struct {
	PSGCCode string `json:"psgc_code" binding:"required"`
}
