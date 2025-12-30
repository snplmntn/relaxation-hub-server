package model

import "time"

// Booking represents the bookings table.
type Booking struct {
	BookingID       int64      `db:"booking_id" json:"booking_id"`
	ReferenceCode   *string    `db:"reference_code" json:"reference_code,omitempty"`
	ClientID        int64      `db:"client_id" json:"client_id"`
	TherapistID     *int64     `db:"therapist_id" json:"therapist_id,omitempty"`
	AssignedAt      *time.Time `db:"assigned_at" json:"assigned_at,omitempty"`
	ServiceID       *int64     `db:"service_id" json:"service_id,omitempty"`
	AddressID       *int64     `db:"address_id" json:"address_id,omitempty"`
	PromoID         *int64     `db:"promo_id" json:"promo_id,omitempty"`
	PaymentMethod   string     `db:"payment_method" json:"payment_method,omitempty"`
	GenderPref      string     `db:"gender_preference" json:"gender_preference"`
	PressurePref    string     `db:"pressure_preference" json:"pressure_preference"`
	Notes           string     `db:"notes" json:"notes"`
	DurationMinutes int        `db:"duration_minutes" json:"duration_minutes"`
	ScheduledStart  *time.Time `db:"scheduled_start" json:"scheduled_start,omitempty"`
	ActualStart     *time.Time `db:"actual_start" json:"actual_start,omitempty"`
	ActualEnd       *time.Time `db:"actual_end" json:"actual_end,omitempty"`
	TherapistArrivedAt *time.Time `db:"therapist_arrived_at" json:"therapist_arrived_at,omitempty"`
	NoShowAt        *time.Time `db:"no_show_at" json:"no_show_at,omitempty"`
	CancelledBy     *string    `db:"cancelled_by" json:"cancelled_by,omitempty"`
	CancelledAt     *time.Time `db:"cancelled_at" json:"cancelled_at,omitempty"`
	CancellationReason *string `db:"cancellation_reason" json:"cancellation_reason,omitempty"`
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
	TherapistID     *int64   `json:"therapist_id"`
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
	// Optional / additional fields accepted by the API but not persisted
	PaymentMethod string `json:"payment_method"` // e.g. "cash", "gcash"
	VoucherCode   string `json:"voucher_code"`
	// Total is the final amount the client expects to pay. If provided, it
	// will be used as the booking's final total. Otherwise FinalTotal is
	// computed from RawTotal and Discount.
	Total *float64 `json:"total"`
}

// UpdateBookingRequest allows limited updates (e.g., reschedule or notes).
type UpdateBookingRequest struct {
	ServiceID       *int64   `json:"service_id"`
	AddressID       *int64   `json:"address_id"`
	PromoID         *int64   `json:"promo_id"`
	GenderPref      *string  `json:"gender_preference"`
	PressurePref    *string  `json:"pressure_preference"`
	Notes           *string  `json:"notes"`
	DurationMinutes *int     `json:"duration_minutes"`
	ScheduledStart  *string  `json:"scheduled_start"` // RFC3339 string
	PaymentMethod   *string  `json:"payment_method"`
	VoucherCode     *string  `json:"voucher_code"`
	Total           *float64 `json:"total"`
}

// UpdateBookingStatusRequest captures status transitions.
type UpdateBookingStatusRequest struct {
	Status string `json:"status"`
	CancellationReason *string `json:"cancellation_reason,omitempty"`
}

// TherapistInfo contains therapist details for booking responses.
type TherapistInfo struct {
	TherapistID int64    `json:"therapist_id"`
	Name        string   `json:"name"`
	Phone       string   `json:"phone,omitempty"`
	Photo       string   `json:"photo,omitempty"`
	Gender      string   `json:"gender,omitempty"`
	Rating      *float64 `json:"rating,omitempty"`
}

// ClientInfo contains client details for booking responses.
type ClientInfo struct {
	ClientID int64  `json:"client_id"`
	Name     string `json:"name"`
	Phone    string `json:"phone,omitempty"`
	Photo    string `json:"photo,omitempty"`
}

// BookingResponse is returned to clients.
type BookingResponse struct {
	BookingID       int64      `json:"booking_id"`
	ReferenceCode   *string    `json:"reference_code,omitempty"`
	ClientID        int64      `json:"client_id"`
	TherapistID     *int64     `json:"therapist_id,omitempty"`
	AssignedAt      *time.Time `json:"assigned_at,omitempty"`
	ServiceID       *int64     `json:"service_id,omitempty"`
	Service         *Service   `json:"service,omitempty"`
	AddressID       *int64     `json:"address_id,omitempty"`
	Address         *Address   `json:"address,omitempty"`
	PromoID         *int64     `json:"promo_id,omitempty"`
	PaymentMethod   string     `json:"payment_method,omitempty"`
	GenderPref      string     `json:"gender_preference"`
	PressurePref    string     `json:"pressure_preference"`
	Notes           string     `json:"notes"`
	DurationMinutes int        `json:"duration_minutes"`
	ScheduledStart  *time.Time `json:"scheduled_start,omitempty"`
	ActualStart     *time.Time `json:"actual_start,omitempty"`
	ActualEnd       *time.Time `json:"actual_end,omitempty"`
	TherapistArrivedAt *time.Time `json:"therapist_arrived_at,omitempty"`
	CancelledBy     *string    `json:"cancelled_by,omitempty"`
	CancelledAt     *time.Time `json:"cancelled_at,omitempty"`
	CancellationReason *string `json:"cancellation_reason,omitempty"`
	RawTotal        *float64   `json:"raw_total,omitempty"`
	Discount        *float64   `json:"discount,omitempty"`
	FinalTotal      *float64   `json:"final_total,omitempty"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ServerTime      time.Time  `json:"server_time"`
	Timeline        []BookingEvent `json:"timeline,omitempty"`
	// Therapist and Client are populated similarly to Service and Address
	Therapist       *TherapistInfo `json:"therapist,omitempty"`
	Client          *ClientInfo    `json:"client,omitempty"`
	// Additional flat fields for backward compatibility/ease of access
	TherapistName   *string        `json:"therapist_name,omitempty"`
	TherapistRating *float64       `json:"therapist_rating,omitempty"`
	ClientName      string         `json:"client_name,omitempty"`
	ClientPhone     string         `json:"client_phone,omitempty"`
	ClientPhoto     string         `json:"client_photo,omitempty"`
}

// BookingOffer represents an offer to a therapist for a booking.
type BookingOffer struct {
	OfferID     int64     `db:"offer_id" json:"offer_id"`
	BookingID   int64     `db:"booking_id" json:"booking_id"`
	TherapistID int64     `db:"therapist_id" json:"therapist_id"`
	Status      string    `db:"status" json:"status"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	ExpiresAt   time.Time `db:"expires_at" json:"expires_at"`
}

const (
	BookingOfferStatusPending  = "pending"
	BookingOfferStatusAccepted = "accepted"
	BookingOfferStatusDeclined = "declined"
	BookingOfferStatusExpired  = "expired"
)
