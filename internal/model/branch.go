package model

import "time"

// Branch represents the branches table.
type Branch struct {
	BranchID    int64     `db:"branch_id" json:"branch_id"`
	BranchName  string    `db:"branch_name" json:"branch_name"`
	AddressLine string    `db:"address_line" json:"address_line"`
	Barangay    *string   `db:"barangay" json:"barangay,omitempty"`
	City        string    `db:"city" json:"city"`
	Province    string    `db:"province" json:"province"`
	PostalCode  *string   `db:"postal_code" json:"postal_code,omitempty"`
	Latitude    *float64  `db:"latitude" json:"latitude,omitempty"`
	Longitude   *float64  `db:"longitude" json:"longitude,omitempty"`
	ContactNo   *string   `db:"contact_no" json:"contact_no,omitempty"`
	Email       *string   `db:"email" json:"email,omitempty"`
	IsActive    bool      `db:"is_active" json:"is_active"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// CreateBranchRequest for creating branch.
type CreateBranchRequest struct {
	BranchName  string   `json:"branch_name"`
	AddressLine string   `json:"address_line"`
	Barangay    *string  `json:"barangay"`
	City        string   `json:"city"`
	Province    string   `json:"province"`
	PostalCode  *string  `json:"postal_code"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	ContactNo   *string  `json:"contact_no"`
	Email       *string  `json:"email"`
}

// UpdateBranchRequest for updating branch.
type UpdateBranchRequest struct {
	BranchName  *string  `json:"branch_name"`
	AddressLine *string  `json:"address_line"`
	Barangay    *string  `json:"barangay"`
	City        *string  `json:"city"`
	Province    *string  `json:"province"`
	PostalCode  *string  `json:"postal_code"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	ContactNo   *string  `json:"contact_no"`
	Email       *string  `json:"email"`
	IsActive    *bool    `json:"is_active"`
}

// BranchResponse to clients.
type BranchResponse struct {
	BranchID    int64     `json:"branch_id"`
	BranchName  string    `json:"branch_name"`
	AddressLine string    `json:"address_line"`
	Barangay    *string   `json:"barangay,omitempty"`
	City        string    `json:"city"`
	Province    string    `json:"province"`
	PostalCode  *string   `json:"postal_code,omitempty"`
	Latitude    *float64  `json:"latitude,omitempty"`
	Longitude   *float64  `json:"longitude,omitempty"`
	ContactNo   *string   `json:"contact_no,omitempty"`
	Email       *string   `json:"email,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
