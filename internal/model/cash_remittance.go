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

// TherapistCashOnHand combines a day-scoped per-method payment breakdown with
// an all-time cash-on-hand balance. Cash/GCash/Maya/BDO and TotalCollected
// reflect only the requested day range; TotalRemitted and CashOnHand are
// all-time (cash only) so the remittance workflow stays accurate.
type TherapistCashOnHand struct {
	TherapistID     int64      `json:"therapist_id"`
	TherapistName   string     `json:"therapist_name"`
	BranchName      string     `json:"branch_name,omitempty"`
	Cash            float64    `json:"cash"`
	GCash           float64    `json:"gcash"`
	Maya            float64    `json:"maya"`
	BDO             float64    `json:"bdo"`
	TotalCollected  float64    `json:"total_collected"`
	TotalRemitted   float64    `json:"total_remitted"`
	CashOnHand      float64    `json:"cash_on_hand"`
	LastCollectedAt *time.Time `json:"last_collected_at,omitempty"`
}

// CreateCashRemittanceRequest is the request body for recording a remittance.
// When Amount is nil the therapist's full outstanding cash on hand is remitted.
type CreateCashRemittanceRequest struct {
	TherapistID int64    `json:"therapist_id"`
	Amount      *float64 `json:"amount,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}
