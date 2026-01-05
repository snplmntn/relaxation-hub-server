package model

import "time"

// ExtensionRequest represents a pending session extension request
type ExtensionRequest struct {
	RequestID        int64      `json:"request_id"`
	BookingID        int64      `json:"booking_id"`
	RequestedMinutes int        `json:"requested_minutes"`
	AdditionalCost   float64    `json:"additional_cost"`
	Status           string     `json:"status"` // pending, accepted, rejected
	RequestedBy      *int64     `json:"requested_by,omitempty"`
	RespondedBy      *int64     `json:"responded_by,omitempty"`
	ResponseNote     *string    `json:"response_note,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ExtensionRequestStatus constants
const (
	ExtensionStatusPending  = "pending"
	ExtensionStatusAccepted = "accepted"
	ExtensionStatusRejected = "rejected"
)

// CreateExtensionRequest is the request body for creating an extension request
type CreateExtensionRequest struct {
	AdditionalMinutes int `json:"additional_minutes"`
}

// RespondExtensionRequest is the request body for accepting/rejecting
type RespondExtensionRequest struct {
	Note string `json:"note,omitempty"`
}
