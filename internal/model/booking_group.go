package model

import "time"

// Product represents an add-on item that can be purchased with a booking.
type Product struct {
	ProductID   int64     `json:"product_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Price       float64   `json:"price"`
	ImageURL    *string   `json:"image_url,omitempty"`
	Category    string    `json:"category"` // e.g., 'add_on', 'linen', 'wellness'
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BookingGroup represents a container for multiple related bookings.
type BookingGroup struct {
	GroupID        int64      `json:"group_id"`
	ClientID       int64      `json:"client_id"`
	AddressID      *int64     `json:"address_id,omitempty"`
	ScheduledStart *time.Time `json:"scheduled_start,omitempty"`
	RawTotal       float64    `json:"raw_total"`
	Discount       float64    `json:"discount"`
	FinalTotal     float64    `json:"final_total"`
	PaymentMethod  string     `json:"payment_method,omitempty"`
	Status         string     `json:"status"` // pending, assigned, in_progress, completed, cancelled, paid
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Hydrated fields (not stored directly)
	Bookings []Booking `json:"bookings,omitempty"`
}

// BookingAddon links a product to a booking with quantity and price snapshot.
type BookingAddon struct {
	AddonID        int64     `json:"addon_id"`
	BookingID      int64     `json:"booking_id"`
	ProductID      int64     `json:"product_id"`
	Quantity       int       `json:"quantity"`
	PriceAtBooking float64   `json:"price_at_booking"`
	CreatedAt      time.Time `json:"created_at"`

	// Hydrated field
	Product *Product `json:"product,omitempty"`
}

// CreateProductRequest is the request body for creating a new product.
type CreateProductRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Price       float64 `json:"price"`
	ImageURL    *string `json:"image_url,omitempty"`
	Category    string  `json:"category,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

// UpdateProductRequest is the request body for updating an existing product.
type UpdateProductRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Price       *float64 `json:"price,omitempty"`
	ImageURL    *string  `json:"image_url,omitempty"`
	Category    *string  `json:"category,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
}

// CreateBookingGroupRequest is the request body for creating a booking group.
type CreateBookingGroupRequest struct {
	ClientID       *int64                      `json:"client_id,omitempty"` // Admin-only override target client
	ScheduledStart string                      `json:"scheduled_start"`     // RFC3339
	AddressID      *int64                      `json:"address_id,omitempty"`
	PaymentMethod  string                      `json:"payment_method,omitempty"`
	VoucherCode    string                      `json:"voucher_code,omitempty"`
	Bookings       []CreateGroupBookingRequest `json:"bookings"`
}

type GroupVoucherPreviewResponse struct {
	Valid            bool    `json:"valid"`
	Code             string  `json:"code"`
	PromoID          int64   `json:"promo_id,omitempty"`
	DiscountAmount   float64 `json:"discount_amount"`
	EligibleSubtotal float64 `json:"eligible_subtotal"`
	RawTotal         float64 `json:"raw_total"`
	FinalTotal       float64 `json:"final_total"`
	AppliesTo        string  `json:"applies_to,omitempty"`
	Message          string  `json:"message"`
	Type             string  `json:"type"`
}

// CreateGroupBookingRequest represents a single booking within a group request.
type CreateGroupBookingRequest struct {
	ServiceID       int64                `json:"service_id"`
	GuestName       string               `json:"guest_name,omitempty"` // e.g., "Self", "Wife"
	SequenceNumber  int                  `json:"sequence_number"`      // 0, 1, 2...
	StartCondition  string               `json:"start_condition"`      // 'fixed_time' or 'after_previous'
	DurationMinutes int                  `json:"duration_minutes,omitempty"`
	GenderPref      string               `json:"gender_preference,omitempty"`
	PressurePref    string               `json:"pressure_preference,omitempty"`
	Notes           string               `json:"notes,omitempty"`
	TherapistID     *int64               `json:"therapist_id,omitempty"`
	Addons          []CreateAddonRequest `json:"addons,omitempty"`
}

// CreateAddonRequest represents an add-on selection for a booking.
type CreateAddonRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}
