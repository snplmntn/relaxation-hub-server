package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

func (r *ledgerRepoImpl) GetPayoutBalance(ctx context.Context, userID int64, role TargetRole) (float64, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var balance float64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(
			SUM(CASE WHEN category = 'payout' THEN amount ELSE 0.0 END) -
			SUM(CASE WHEN category = 'settlement' THEN amount ELSE 0.0 END),
			0.0
		)
		FROM ledger_entries
		WHERE target_user_id = $1 AND target_role = $2 AND voided = FALSE
	`, userID, string(role)).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("failed to get payout balance: %w", err)
	}
	return balance, nil
}

func (r *ledgerRepoImpl) RecordSettlement(ctx context.Context, userID int64, role TargetRole, amount float64, reference string, recordedBy int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := r.db.Exec(ctx, `
		INSERT INTO ledger_entries (entry_type, category, amount, description, entry_date, created_by, target_user_id, target_role, status)
		VALUES ('debit', 'settlement', $1, $2, NOW(), $3, $4, $5, 'approved')
	`, amount, reference, recordedBy, userID, string(role))
	if err != nil {
		return fmt.Errorf("failed to record settlement: %w", err)
	}
	return nil
}

func (r *ledgerRepoImpl) RecordPayrollSettlement(ctx context.Context, payrollRunID, payrollRowID, userID int64, role TargetRole, amount float64, method, reference string, recordedBy int64) (int64, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var entryID int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO ledger_entries (
			entry_type, category, amount, description, entry_date, created_by,
			target_user_id, target_role, payroll_run_id, payroll_row_id, status
		)
		SELECT
			'debit',
			'settlement',
			$5,
			CONCAT(
				'Payroll settlement ',
				TO_CHAR(run.period_start, 'YYYY-MM-DD'),
				' to ',
				TO_CHAR(run.period_end, 'YYYY-MM-DD'),
				' - ',
				pr.full_name_snapshot,
				' via ',
				$6,
				CASE WHEN $7 = '' THEN '' ELSE CONCAT(' ref ', $7) END
			),
			NOW(),
			$8,
			$3,
			$4,
			$1,
			$2,
			'approved'
		FROM payroll_runs run
		JOIN payroll_rows pr ON pr.payroll_run_id = run.payroll_run_id
		WHERE run.payroll_run_id = $1
		  AND pr.payroll_row_id = $2
		  AND pr.user_id = $3
		RETURNING entry_id
	`, payrollRunID, payrollRowID, userID, string(role), amount, method, reference, recordedBy).Scan(&entryID)
	if err != nil {
		return 0, fmt.Errorf("failed to record payroll settlement: %w", err)
	}
	return entryID, nil
}

// GetPayoutBalances returns unified balances for all therapists (from ledger) and riders (from rider_wallets).
func (r *ledgerRepoImpl) GetPayoutBalances(ctx context.Context) ([]PayoutBalance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		-- Therapist balances from ledger entries
		SELECT
			u.user_id,
			'therapist'::TEXT AS role,
			u.full_name,
			COALESCE(SUM(CASE WHEN le.category = 'payout' THEN le.amount ELSE 0.0 END), 0.0)      AS total_earned,
			COALESCE(SUM(CASE WHEN le.category = 'settlement' THEN le.amount ELSE 0.0 END), 0.0)   AS total_settled
		FROM users u
		LEFT JOIN ledger_entries le
			ON u.user_id = le.target_user_id
			AND le.voided = FALSE
			AND (le.target_role = 'therapist' OR le.target_role IS NULL)
		WHERE u.role = 'therapist'
		GROUP BY u.user_id, u.full_name

		UNION ALL

		-- Rider balances from rider_wallets (cents → PHP)
		SELECT
			u.user_id,
			'rider'::TEXT AS role,
			u.full_name,
			COALESCE(rw.total_earned_cents, 0)    / 100.0 AS total_earned,
			COALESCE(rw.total_withdrawn_cents, 0) / 100.0 AS total_settled
		FROM users u
		JOIN rider_wallets rw ON u.user_id = rw.rider_id
		WHERE u.role = 'rider'

		ORDER BY full_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get payout balances: %w", err)
	}
	defer rows.Close()

	var balances []PayoutBalance
	for rows.Next() {
		var b PayoutBalance
		var roleStr string
		if err := rows.Scan(&b.UserID, &roleStr, &b.FullName, &b.TotalEarned, &b.TotalSettled); err != nil {
			return nil, err
		}
		b.Role = TargetRole(roleStr)
		b.BalanceOwed = b.TotalEarned - b.TotalSettled
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
			reviewed_by, reviewed_at, target_user_id, target_role, voided, voided_at, voided_reason
		FROM ledger_entries
		WHERE entry_date >= $1 AND entry_date <= $2 AND voided = FALSE
		ORDER BY entry_date DESC, created_at DESC
	`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		var roleStr *string
		if err := rows.Scan(
			&e.EntryID, &e.BookingID, &e.EntryType, &e.Category, &e.Amount, &e.Description,
			&e.EntryDate, &e.CreatedAt, &e.CreatedBy, &e.ProofURL, &e.Status,
			&e.ReviewedBy, &e.ReviewedAt, &e.TargetUserID, &roleStr, &e.Voided, &e.VoidedAt, &e.VoidedReason,
		); err != nil {
			return nil, err
		}
		if roleStr != nil {
			role := TargetRole(*roleStr)
			e.TargetRole = &role
		}
		entries = append(entries, e)
	}
	return entries, nil
}
