package model

import "time"

// Address represents the addresses table
type Address struct {
	AddressID  int64      `db:"address_id" json:"address_id"`
	UserID     int64      `db:"user_id" json:"-"` // Never expose in responses
	Label      string     `db:"label" json:"label"`
	Street     string     `db:"street_address" json:"street_address"`
	Barangay   string     `db:"barangay" json:"barangay"`
	City       string     `db:"city" json:"city"`
	Province   string     `db:"province" json:"province"`
	PostalCode string     `db:"postal_code" json:"postal_code"`
	Landmark   string     `db:"landmark" json:"landmark"`
	Country    string     `db:"country" json:"country"`
	Latitude   *float64   `db:"latitude" json:"latitude"`
	Longitude  *float64   `db:"longitude" json:"longitude"`
	IsDefault  bool       `db:"is_default" json:"is_default"`
	DeletedAt  *time.Time `db:"deleted_at" json:"-"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
}

// CreateAddressRequest is the payload for creating an address
type CreateAddressRequest struct {
	Label      string   `json:"label"`
	Street     string   `json:"street_address" binding:"required"`
	Barangay   string   `json:"barangay"`
	City       string   `json:"city" binding:"required"`
	Province   string   `json:"province"`
	PostalCode string   `json:"postal_code"`
	Landmark   string   `json:"landmark"`
	Country    string   `json:"country"`
	Latitude   *float64 `json:"latitude"`
	Longitude  *float64 `json:"longitude"`
}

// UpdateAddressRequest is the payload for updating an address
type UpdateAddressRequest struct {
	Label      *string `json:"label"`
	Street     *string `json:"street_address"`
	Barangay   *string `json:"barangay"`
	City       *string `json:"city"`
	Province   *string `json:"province"`
	PostalCode *string `json:"postal_code"`
	Landmark   *string `json:"landmark"`
	Country    *string `json:"country"`
}

// AddressResponse is what we send to clients
type AddressResponse struct {
	AddressID  int64     `json:"address_id"`
	Label      string    `json:"label,omitempty"`
	Street     string    `json:"street_address"`
	Barangay   string    `json:"barangay,omitempty"`
	City       string    `json:"city"`
	Province   string    `json:"province,omitempty"`
	PostalCode string    `json:"postal_code,omitempty"`
	Landmark   string    `json:"landmark,omitempty"`
	Country    string    `json:"country"`
	Latitude   *float64  `json:"latitude,omitempty"`
	Longitude  *float64  `json:"longitude,omitempty"`
	IsDefault  bool      `json:"is_default"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
