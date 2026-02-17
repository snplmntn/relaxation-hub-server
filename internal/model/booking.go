package model

import "time"

// PaymentBreakdown stores itemized pricing for historical accuracy
type PaymentBreakdown struct {
	BasePrice       float64 `json:"base_price"`
	DurationMarkup  float64 `json:"duration_markup"`   // Cost of initial duration > base
	ExtensionsTotal float64 `json:"extensions_total"`
	ServiceSnapshot string  `json:"service_snapshot_name"` // e.g. "Massage (90min)"
}

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
	ChangeFor       *float64   `db:"change_for" json:"change_for,omitempty"`
	Status          string     `db:"status" json:"status"`
	IsRated         bool       `db:"is_rated" json:"is_rated"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
	TotalPausedSeconds int     `db:"total_paused_seconds" json:"total_paused_seconds"`
	CurrentPauseStart *time.Time `db:"current_pause_start" json:"current_pause_start,omitempty"`
	PausedByRole      *string    `db:"-" json:"paused_by_role,omitempty"`
	ExtensionWaitSeconds int        `db:"extension_wait_seconds" json:"extension_wait_seconds"`
	TherapistEarnings    *float64   `db:"therapist_earnings" json:"therapist_earnings,omitempty"`
	PlatformFee          *float64   `db:"platform_fee" json:"platform_fee,omitempty"`
	PaymentBreakdownJSON []byte  `db:"payment_breakdown" json:"-"` // Raw JSONB from DB
	PaymentBreakdown *PaymentBreakdown `db:"-" json:"payment_breakdown,omitempty"` // Parsed struct

	// Complex Booking Fields (Migration 033)
	GroupID        *int64 `db:"group_id" json:"group_id,omitempty"`
	GuestName      string `db:"guest_name" json:"guest_name,omitempty"`
	SequenceNumber int    `db:"sequence_number" json:"sequence_number"`
	StartCondition string `db:"start_condition" json:"start_condition,omitempty"` // 'fixed_time' or 'after_previous'

	// Hydrated fields
	Addons []BookingAddon `db:"-" json:"addons,omitempty"`
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
	ChangeFor *float64 `json:"change_for"`
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
	ChangeFor       *float64 `json:"change_for"`
	// Consolidated status update fields
	Status             *string `json:"status"`
	CancellationReason *string `json:"cancellation_reason"`
	StartTime          *string `json:"start_time"` // RFC3339 string for offline sync
	TherapistID        *int64  `json:"therapist_id"`
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
	Gender   string `json:"gender,omitempty"`
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
	PromoCode       string     `json:"promo_code,omitempty"`
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
	ChangeFor       *float64   `json:"change_for,omitempty"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ServerTime      time.Time  `json:"server_time"`
	IsRated         bool       `json:"is_rated"`
	Timeline        []BookingEvent `json:"timeline,omitempty"`
	// Therapist and Client are populated similarly to Service and Address
	Therapist       *TherapistInfo `json:"therapist,omitempty"`
	Client          *ClientInfo    `json:"client,omitempty"`
	TotalPausedSeconds int        `json:"total_paused_seconds"`
	CurrentPauseStart *time.Time `json:"current_pause_start,omitempty"`
	PausedByRole      *string    `json:"paused_by_role,omitempty"`
	ExtensionWaitSeconds int     `json:"extension_wait_seconds"`
	TherapistEarnings    *float64 `json:"therapist_earnings,omitempty"`
	PlatformFee          *float64 `json:"platform_fee,omitempty"`
	Payment           *PaymentResponse `json:"payment,omitempty"`
	PaymentBreakdown  *PaymentBreakdown `json:"payment_breakdown,omitempty"`
	ActiveRide        *Ride            `json:"active_ride,omitempty"`
}

// PaginatedBookingsResponse wraps a list of bookings with pagination metadata.
type PaginatedBookingsResponse struct {
	Bookings   []BookingResponse `json:"bookings"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	TotalPages int               `json:"total_pages"`
	HasMore    bool              `json:"has_more"`
}

// BookingOffer represents an offer to a therapist for a booking.
type BookingOffer struct {
	OfferID     int64     `db:"offer_id" json:"offer_id"`
	BookingID   int64     `db:"booking_id" json:"booking_id"`
	TherapistID int64     `db:"therapist_id" json:"therapist_id"`
	Status      string    `db:"status" json:"status"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	ExpiresAt   time.Time          `db:"expires_at" json:"expires_at"`
	EstimatedEarnings *float64     `db:"estimated_earnings" json:"estimated_earnings"`
	IsBundle    bool               `db:"is_bundle" json:"is_bundle"`
	Items       []BookingOfferItem `db:"-" json:"items,omitempty"`
}

type BookingOfferItem struct {
	OfferID   int64 `db:"offer_id" json:"offer_id"`
	BookingID int64 `db:"booking_id" json:"booking_id"`
	EstimatedEarnings float64 `db:"estimated_earnings" json:"estimated_earnings"`
}

const (
	BookingOfferStatusPending  = "pending"
	BookingOfferStatusAccepted = "accepted"
	BookingOfferStatusDeclined = "declined"
	BookingOfferStatusExpired  = "expired"
)
