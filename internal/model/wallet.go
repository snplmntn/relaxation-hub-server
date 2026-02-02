package model

import (
	"encoding/json"
	"time"
)

// Wallet represents a therapist's balance account.
type Wallet struct {
	WalletID         int64      `json:"wallet_id"`
	TherapistID      int64      `json:"therapist_id"`
	AvailableBalance float64    `json:"available_balance"`
	PendingBalance   float64    `json:"pending_balance"`
	TotalEarned      float64    `json:"total_earned"`
	TotalWithdrawn   float64    `json:"total_withdrawn"`
	TotalAdvances    float64    `json:"total_advances"`
	MinimumPayout    float64    `json:"minimum_payout"`
	LastPayoutAt     *time.Time `json:"last_payout_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// WalletTransaction represents a single balance change event.
type WalletTransaction struct {
	TransactionID int64      `json:"transaction_id"`
	WalletID      int64      `json:"wallet_id"`
	BookingID     *int64     `json:"booking_id,omitempty"`
	LedgerEntryID *int64     `json:"ledger_entry_id,omitempty"`
	Type          string     `json:"type"` // earning, earning_released, payout, cash_advance, advance_repayment, adjustment, refund_clawback
	Amount        float64    `json:"amount"`
	BalanceAfter  float64    `json:"balance_after"`
	PendingAfter  float64    `json:"pending_after"`
	Description   *string    `json:"description,omitempty"`
	ProcessedBy   *int64     `json:"processed_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// PayoutRequest represents a therapist's request to withdraw funds.
type PayoutRequest struct {
	RequestID            int64           `json:"request_id"`
	WalletID             int64           `json:"wallet_id"`
	TherapistID          int64           `json:"therapist_id"`
	Amount               float64         `json:"amount"`
	PayoutMethod         string          `json:"payout_method"` // gcash, bank_transfer, cash
	AccountDetails       json.RawMessage `json:"account_details,omitempty"`
	Status               string          `json:"status"` // pending, approved, completed, rejected, cancelled
	ProcessedBy          *int64          `json:"processed_by,omitempty"`
	ProcessedAt          *time.Time      `json:"processed_at,omitempty"`
	RejectionReason      *string         `json:"rejection_reason,omitempty"`
	TransactionReference *string         `json:"transaction_reference,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

// CashAdvance represents a pre-payment given to a therapist.
type CashAdvance struct {
	AdvanceID        int64      `json:"advance_id"`
	WalletID         int64      `json:"wallet_id"`
	TherapistID      int64      `json:"therapist_id"`
	OriginalAmount   float64    `json:"original_amount"`
	RemainingBalance float64    `json:"remaining_balance"`
	RepaymentRate    float64    `json:"repayment_rate"` // Percentage (e.g., 50.00 = 50%)
	Status           string     `json:"status"`         // active, paid_off, written_off
	ApprovedBy       *int64     `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time `json:"approved_at,omitempty"`
	Reason           *string    `json:"reason,omitempty"`
	PaidOffAt        *time.Time `json:"paid_off_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// WalletSummary provides a view for therapist dashboard.
type WalletSummary struct {
	Wallet           *Wallet        `json:"wallet"`
	ActiveAdvance    *CashAdvance   `json:"active_advance,omitempty"`
	PendingPayouts   int            `json:"pending_payouts"`
	RecentTransactions []WalletTransaction `json:"recent_transactions,omitempty"`
}
