package model

import (
	"encoding/json"
	"time"
)

// Cart represents a user's shopping cart.
type Cart struct {
	CartID    int64      `json:"cart_id"`
	UserID    int64      `json:"user_id"`
	Items     []CartItem `json:"items"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// CartItemAddon represents a product add-on in a cart item.
type CartItemAddon struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

// CartItem represents an item in a shopping cart.
type CartItem struct {
	CartItemID         int64           `json:"cart_item_id"`
	CartID             int64           `json:"cart_id"`
	ServiceID          int64           `json:"service_id"`
	Service            *Service        `json:"service,omitempty"` // Populated on read
	GuestName          string          `json:"guest_name"`
	DurationMinutes    int             `json:"duration_minutes"`
	GenderPreference   string          `json:"gender_preference"`
	PressurePreference string          `json:"pressure_preference"`
	Notes              string          `json:"notes,omitempty"`
	SequenceNumber     int             `json:"sequence_number"`
	StartCondition     string          `json:"start_condition"`
	Addons             []CartItemAddon `json:"addons"`
	DateAdded          time.Time       `json:"date_added"`
}

// AddonsJSON returns the addons as a JSON byte slice for database storage.
func (ci *CartItem) AddonsJSON() ([]byte, error) {
	if ci.Addons == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(ci.Addons)
}

// ParseAddonsJSON parses a JSON byte slice into the Addons field.
func (ci *CartItem) ParseAddonsJSON(data []byte) error {
	if len(data) == 0 {
		ci.Addons = []CartItemAddon{}
		return nil
	}
	return json.Unmarshal(data, &ci.Addons)
}

// AddToCartRequest is the request body for adding an item to the cart.
type AddToCartRequest struct {
	ServiceID          int64           `json:"service_id"`
	GuestName          string          `json:"guest_name"`
	DurationMinutes    int             `json:"duration_minutes"`
	GenderPreference   string          `json:"gender_preference"`
	PressurePreference string          `json:"pressure_preference"`
	Notes              string          `json:"notes,omitempty"`
	StartCondition     string          `json:"start_condition"`
	Addons             []CartItemAddon `json:"addons,omitempty"`
}

// UpdateCartItemRequest is the request body for updating a cart item.
type UpdateCartItemRequest struct {
	GuestName          *string          `json:"guest_name,omitempty"`
	DurationMinutes    *int             `json:"duration_minutes,omitempty"`
	GenderPreference   *string          `json:"gender_preference,omitempty"`
	PressurePreference *string          `json:"pressure_preference,omitempty"`
	Notes              *string          `json:"notes,omitempty"`
	StartCondition     *string          `json:"start_condition,omitempty"`
	Addons             *[]CartItemAddon `json:"addons,omitempty"`
}
