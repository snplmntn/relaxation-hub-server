package model

import "time"

// CashRemittance records a single occasion where an admin collected (remitted)
// cash from a therapist for their cash-payment bookings.
type CashRemittance struct {
	RemittanceID   int64     `json:"remittance_id"`
	TherapistID    int64     `json:"therapist_id"`
	Amount         float64   `json:"amount"`
	Notes          string    `json:"notes,omitempty"`
	RemittedBy     *int64    `json:"remitted_by,omitempty"`
	RemittedByName string    `json:"remitted_by_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// TherapistCashOnHand is the aggregated cash a therapist is currently holding:
// the full client-paid total of their completed cash bookings, minus everything
// already remitted. Only cash bookings contribute — gcash/bdo/bank_transfer do not.
type TherapistCashOnHand struct {
	TherapistID           int64      `json:"therapist_id"`
	TherapistName         string     `json:"therapist_name"`
	BranchName            string     `json:"branch_name,omitempty"`
	TotalCollected        float64    `json:"total_collected"`
	TotalRemitted         float64    `json:"total_remitted"`
	CashOnHand            float64    `json:"cash_on_hand"`
	CompletedCashBookings int        `json:"completed_cash_bookings"`
	LastCollectedAt       *time.Time `json:"last_collected_at,omitempty"`
}

// CreateCashRemittanceRequest is the request body for recording a remittance.
// When Amount is nil the therapist's full outstanding cash on hand is remitted.
type CreateCashRemittanceRequest struct {
	TherapistID int64    `json:"therapist_id"`
	Amount      *float64 `json:"amount,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}
