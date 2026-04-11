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
	AreaKey            string            `db:"area_key" json:"area_key"`
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
	AreaKey   string    `db:"area_key" json:"area_key"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// AreaInterestedUser is an admin-facing row for users who requested coverage.
type AreaInterestedUser struct {
	UserID       int64     `json:"user_id"`
	FullName     string    `json:"full_name"`
	PrimaryEmail string    `json:"primary_email,omitempty"`
	PrimaryPhone string    `json:"primary_phone,omitempty"`
	RequestedAt  time.Time `json:"requested_at"`
}

// AreaInterestedUsersPage is a paginated response for area interested users.
type AreaInterestedUsersPage struct {
	AreaKey    string               `json:"area_key"`
	AreaName   string               `json:"area_name,omitempty"`
	TotalCount int                  `json:"total_count"`
	Page       int                  `json:"page"`
	Limit      int                  `json:"limit"`
	Users      []AreaInterestedUser `json:"users"`
}

// LocationCheckResult represents the result of a location serviceability check.
type LocationCheckResult struct {
	Status     ServiceAreaStatus `json:"status"`
	Message    string            `json:"message"`
	IsAllowed  bool              `json:"is_allowed"`
	AreaKey    string            `json:"area_key,omitempty"`
	AreaName   string            `json:"area_name,omitempty"`
	MinBooking int               `json:"min_booking_minutes,omitempty"`
}

// CheckLocationRequest is the request payload for validating a location.
// Uses city/barangay names from map geocoding.
type CheckLocationRequest struct {
	CityName     string `json:"city_name,omitempty"`
	BarangayName string `json:"barangay_name,omitempty"`
}

// RecordInterestRequest is the request payload for recording coverage interest.
type RecordInterestRequest struct {
	AreaKey      string           `json:"area_key,omitempty"`
	Name         string           `json:"name,omitempty"`
	CityName     string           `json:"city_name,omitempty"`
	BarangayName string           `json:"barangay_name,omitempty"`
	Level        ServiceAreaLevel `json:"level,omitempty"`
	Lat          *float64         `json:"lat,omitempty"`
	Lng          *float64         `json:"lng,omitempty"`
}
