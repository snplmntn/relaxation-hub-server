package model

import "time"

// Payment represents the payments table.
type Payment struct {
	PaymentID      int64      `db:"payment_id" json:"payment_id"`
	BookingID      int64      `db:"booking_id" json:"booking_id"`
	Amount         float64    `db:"amount" json:"amount"`
	Gateway        string     `db:"gateway" json:"gateway"`
	TransactionID  *string    `db:"transaction_id" json:"transaction_id,omitempty"`
	Status         string     `db:"status" json:"status"`
	GatewayPayload []byte     `db:"gateway_response" json:"gateway_response,omitempty"`
	WebhookID      *string    `db:"webhook_id" json:"webhook_id,omitempty"`
	TransactionAt  time.Time  `db:"transaction_date" json:"transaction_date"`
	PaidAt         *time.Time `db:"paid_at" json:"paid_at,omitempty"`
	RefundedAt     *time.Time `db:"refunded_at" json:"refunded_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

// CreatePaymentRequest represents a request to initiate a payment.
type CreatePaymentRequest struct {
	BookingID int64   `json:"booking_id"`
	Amount    float64 `json:"amount"`
	Gateway   string  `json:"gateway"`
}

// UpdatePaymentStatusRequest represents status updates from gateway/webhook.
type UpdatePaymentStatusRequest struct {
	Status        string  `json:"status"`
	TransactionID *string `json:"transaction_id"`
	WebhookID     *string `json:"webhook_id"`
}

// PaymentResponse is sent back to clients.
type PaymentResponse struct {
	PaymentID     int64      `json:"payment_id"`
	BookingID     int64      `json:"booking_id"`
	Amount        float64    `json:"amount"`
	Gateway       string     `json:"gateway"`
	TransactionID *string    `json:"transaction_id,omitempty"`
	Status        string     `json:"status"`
	WebhookID     *string    `json:"webhook_id,omitempty"`
	TransactionAt time.Time  `json:"transaction_date"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	RefundedAt    *time.Time `json:"refunded_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
