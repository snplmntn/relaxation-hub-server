package model

import "time"

// Checkout status values.
const (
	CheckoutStatusPending = "pending"
	CheckoutStatusPaid    = "paid"
	CheckoutStatusFailed  = "failed"
	CheckoutStatusExpired = "expired"
)

// Checkout kinds.
const (
	CheckoutKindSingle = "single"
	CheckoutKindGroup  = "group"
)

// BookingCheckout is a booking the customer has started paying for but which
// does not exist yet. The create request is parked here until PayMongo confirms
// payment; the webhook then replays it through the normal creation path.
type BookingCheckout struct {
	CheckoutID int64  `db:"checkout_id" json:"checkout_id"`
	Reference  string `db:"reference" json:"reference"`
	ClientID   int64  `db:"client_id" json:"client_id"`

	Kind           string  `db:"kind" json:"kind"`
	Channel        string  `db:"channel" json:"channel"`
	RequestPayload []byte  `db:"request_payload" json:"-"`
	Amount         float64 `db:"amount" json:"amount"`

	Provider          string  `db:"provider" json:"provider"`
	ProviderSessionID *string `db:"provider_session_id" json:"-"`
	CheckoutURL       *string `db:"checkout_url" json:"checkout_url,omitempty"`

	Status     string  `db:"status" json:"status"`
	EventID    *string `db:"event_id" json:"-"`
	BookingID  *int64  `db:"booking_id" json:"booking_id,omitempty"`
	GroupID    *int64  `db:"group_id" json:"group_id,omitempty"`
	FulfilNote *string `db:"fulfil_note" json:"fulfil_note,omitempty"`

	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	PaidAt    *time.Time `db:"paid_at" json:"paid_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

// StartCheckoutRequest asks for a hosted checkout for a booking that has not
// been created yet. Exactly one of Booking or Group must be supplied.
type StartCheckoutRequest struct {
	Channel string                     `json:"channel"`
	Booking *CreateBookingRequest      `json:"booking,omitempty"`
	Group   *CreateBookingGroupRequest `json:"group,omitempty"`
}

// StartCheckoutResponse is what the browser needs to send the customer onward.
type StartCheckoutResponse struct {
	Reference   string    `json:"reference"`
	CheckoutURL string    `json:"checkout_url"`
	Amount      float64   `json:"amount"`
	Channel     string    `json:"channel"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// CheckoutStatusResponse is polled by the return page while it waits for the
// webhook that actually creates the booking.
type CheckoutStatusResponse struct {
	Reference string  `json:"reference"`
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
	Channel   string  `json:"channel"`
	BookingID *int64  `json:"booking_id,omitempty"`
	GroupID   *int64  `json:"group_id,omitempty"`
	Note      *string `json:"note,omitempty"`
}
