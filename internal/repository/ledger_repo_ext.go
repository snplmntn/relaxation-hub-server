package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

// TherapistBalance holds the financial status of a therapist
type TherapistBalance struct {
	UserID         int64   `json:"user_id"`
	FullName       string  `json:"full_name"`
	TotalEarned    float64 `json:"total_earned"`
	TotalPaid      float64 `json:"total_paid"`
	CurrentBalance float64 `json:"current_balance"`
}

func (r *ledgerRepoImpl) GetTherapistBalance(ctx context.Context, therapistID int64) (float64, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var balance float64
	// Balance = Total Earnings (Payouts) - Total Paid (Settlements)
	// Both are typically recorded as Debits in this simplified ledger.
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(
			(SUM(CASE WHEN category = 'payout' THEN amount ELSE 0.0 END) -
			 SUM(CASE WHEN category = 'settlement' THEN amount ELSE 0.0 END)), 
			0.0
		)
		FROM ledger_entries
		WHERE target_user_id = $1
	`, therapistID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("failed to get therapist balance: %w", err)
	}
	return balance, nil
}

func (r *ledgerRepoImpl) RecordSettlement(ctx context.Context, therapistID int64, amount float64, reference string, recordedBy int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	// Insert settlement entry
	// Type: debit (money leaving), Category: settlement
	// This entry represents a cash transfer to the therapist.
	_, err := r.db.Exec(ctx, `
		INSERT INTO ledger_entries (entry_type, category, amount, description, entry_date, created_by, target_user_id, status)
		VALUES ('debit', 'settlement', $1, $2, NOW(), $3, $4, 'approved')
	`, amount, reference, recordedBy, therapistID)
	if err != nil {
		return fmt.Errorf("failed to record settlement: %w", err)
	}
	return nil
}

func (r *ledgerRepoImpl) GetTherapistBalances(ctx context.Context) ([]TherapistBalance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT
			u.user_id,
			u.full_name,
			COALESCE(SUM(CASE WHEN le.category = 'payout' THEN le.amount ELSE 0.0 END), 0.0) as total_earned,
			COALESCE(SUM(CASE WHEN le.category = 'settlement' THEN le.amount ELSE 0.0 END), 0.0) as total_paid
		FROM users u
		LEFT JOIN ledger_entries le ON u.user_id = le.target_user_id
		WHERE u.role = 'therapist'
		GROUP BY u.user_id, u.full_name
		ORDER BY u.full_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get therapist balances: %w", err)
	}
	defer rows.Close()

	var balances []TherapistBalance
	for rows.Next() {
		var b TherapistBalance
		if err := rows.Scan(&b.UserID, &b.FullName, &b.TotalEarned, &b.TotalPaid); err != nil {
			return nil, err
		}
		b.CurrentBalance = b.TotalEarned - b.TotalPaid
		balances = append(balances, b)
	}
	return balances, rows.Err()
}

// ListEntries returns all ledger entries within a date range
func (r *ledgerRepoImpl) ListEntries(ctx context.Context, startDate, endDate time.Time) ([]LedgerEntry, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT 
			entry_id, booking_id, entry_type, category, amount, description, 
			entry_date, created_at, created_by, proof_url, status, 
			reviewed_by, reviewed_at, target_user_id
		FROM ledger_entries
		WHERE entry_date >= $1 AND entry_date <= $2
		ORDER BY entry_date DESC, created_at DESC
	`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.EntryID, &e.BookingID, &e.EntryType, &e.Category, &e.Amount, &e.Description,
			&e.EntryDate, &e.CreatedAt, &e.CreatedBy, &e.ProofURL, &e.Status,
			&e.ReviewedBy, &e.ReviewedAt, &e.TargetUserID,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
