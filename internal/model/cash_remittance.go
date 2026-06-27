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
// a cash-on-hand balance. Cash/GCash/Maya/BDO, TotalCollected and TotalRemitted
// reflect only the requested day range; CashOnHand is the running balance as of
// the end of that day (cumulative cash collected minus cumulative remitted,
// cash only). With no date filter every value is the all-time total.
type TherapistCashOnHand struct {
	TherapistID     int64      `json:"therapist_id"`
	TherapistName   string     `json:"therapist_name"`
	BranchName      string     `json:"branch_name,omitempty"`
	Cash            float64    `json:"cash"`
	GCash           float64    `json:"gcash"`
	Maya            float64    `json:"maya"`
	BDO             float64    `json:"bdo"`
	TotalCollected  float64    `json:"total_collected"`
	TotalRemitted   float64    `json:"total_remitted"` // day-scoped: remitted within the requested range
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
