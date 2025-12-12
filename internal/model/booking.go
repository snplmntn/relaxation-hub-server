package model

import "time"

// Booking represents the bookings table.
type Booking struct {
	BookingID       int64      `db:"booking_id" json:"booking_id"`
	ClientID        int64      `db:"client_id" json:"client_id"`
	TherapistID     int64      `db:"therapist_id" json:"therapist_id"`
	ServiceID       *int64     `db:"service_id" json:"service_id,omitempty"`
	AddressID       *int64     `db:"address_id" json:"address_id,omitempty"`
	PromoID         *int64     `db:"promo_id" json:"promo_id,omitempty"`
	GenderPref      string     `db:"gender_preference" json:"gender_preference"`
	PressurePref    string     `db:"pressure_preference" json:"pressure_preference"`
	Notes           string     `db:"notes" json:"notes"`
	DurationMinutes int        `db:"duration_minutes" json:"duration_minutes"`
	ScheduledStart  *time.Time `db:"scheduled_start" json:"scheduled_start,omitempty"`
	ActualStart     *time.Time `db:"actual_start" json:"actual_start,omitempty"`
	ActualEnd       *time.Time `db:"actual_end" json:"actual_end,omitempty"`
	RawTotal        *float64   `db:"raw_total" json:"raw_total,omitempty"`
	Discount        *float64   `db:"discount" json:"discount,omitempty"`
	FinalTotal      *float64   `db:"final_total" json:"final_total,omitempty"`
	Status          string     `db:"status" json:"status"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
}

// ServiceIDOrZero returns 0 if service is nil.
func (b *Booking) ServiceIDOrZero() int64 {
	if b.ServiceID == nil {
		return 0
	}
	return *b.ServiceID
}

// CreateBookingRequest is the payload for creating a booking.
type CreateBookingRequest struct {
	TherapistID     int64    `json:"therapist_id"`
	ServiceID       *int64   `json:"service_id"`
	AddressID       *int64   `json:"address_id"`
	PromoID         *int64   `json:"promo_id"`
	GenderPref      string   `json:"gender_preference"`
	PressurePref    string   `json:"pressure_preference"`
	Notes           string   `json:"notes"`
	DurationMinutes int      `json:"duration_minutes"`
	ScheduledStart  string   `json:"scheduled_start"` // RFC3339 string
	RawTotal        *float64 `json:"raw_total"`
	Discount        *float64 `json:"discount"`
}

// UpdateBookingRequest allows limited updates (e.g., reschedule or notes).
type UpdateBookingRequest struct {
	ServiceID       *int64  `json:"service_id"`
	AddressID       *int64  `json:"address_id"`
	PromoID         *int64  `json:"promo_id"`
	GenderPref      *string `json:"gender_preference"`
	PressurePref    *string `json:"pressure_preference"`
	Notes           *string `json:"notes"`
	DurationMinutes *int    `json:"duration_minutes"`
	ScheduledStart  *string `json:"scheduled_start"` // RFC3339 string
}

// UpdateBookingStatusRequest captures status transitions.
type UpdateBookingStatusRequest struct {
	Status string `json:"status"`
}

// BookingResponse is returned to clients.
type BookingResponse struct {
	BookingID       int64      `json:"booking_id"`
	ClientID        int64      `json:"client_id"`
	TherapistID     int64      `json:"therapist_id"`
	ServiceID       *int64     `json:"service_id,omitempty"`
	AddressID       *int64     `json:"address_id,omitempty"`
	PromoID         *int64     `json:"promo_id,omitempty"`
	GenderPref      string     `json:"gender_preference"`
	PressurePref    string     `json:"pressure_preference"`
	Notes           string     `json:"notes"`
	DurationMinutes int        `json:"duration_minutes"`
	ScheduledStart  *time.Time `json:"scheduled_start,omitempty"`
	ActualStart     *time.Time `json:"actual_start,omitempty"`
	ActualEnd       *time.Time `json:"actual_end,omitempty"`
	RawTotal        *float64   `json:"raw_total,omitempty"`
	Discount        *float64   `json:"discount,omitempty"`
	FinalTotal      *float64   `json:"final_total,omitempty"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
