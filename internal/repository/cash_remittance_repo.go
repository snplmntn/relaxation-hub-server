package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// CashRemittanceRepository handles cash-on-hand aggregation and remittance records.
type CashRemittanceRepository interface {
	ListTherapistCashOnHand(ctx context.Context, dateFrom, dateTo *time.Time) ([]model.TherapistCashOnHand, error)
	GetTherapistCashOnHand(ctx context.Context, therapistID int64) (*model.TherapistCashOnHand, error)
	CreateRemittance(ctx context.Context, r *model.CashRemittance) error
	ListRemittancesByTherapist(ctx context.Context, therapistID int64, limit int) ([]model.CashRemittance, error)
}

type cashRemittanceRepo struct {
	db db.DBTX
}

// NewCashRemittanceRepository creates a new CashRemittanceRepository.
func NewCashRemittanceRepository(database db.DBTX) CashRemittanceRepository {
	return &cashRemittanceRepo{db: database}
}

// ListTherapistCashOnHand returns a per-therapist breakdown of payments for the
// given day range (dateFrom inclusive, dateTo exclusive) combined with all-time
// cash-on-hand balances. Passing nil for both dates returns all-time breakdowns.
// Rows are included when the therapist had a booking in the range OR has
// positive all-time cash on hand (preserving the remittance workflow).
func (r *cashRemittanceRepo) ListTherapistCashOnHand(ctx context.Context, dateFrom, dateTo *time.Time) ([]model.TherapistCashOnHand, error) {
	query := `
		WITH day_breakdown AS (
			SELECT
				b.therapist_id,
				SUM(CASE WHEN b.payment_method = 'cash'  THEN b.final_total ELSE 0 END) AS cash,
				SUM(CASE WHEN b.payment_method = 'gcash' THEN b.final_total ELSE 0 END) AS gcash,
				SUM(CASE WHEN b.payment_method = 'maya'  THEN b.final_total ELSE 0 END) AS maya,
				SUM(CASE WHEN b.payment_method = 'bdo'   THEN b.final_total ELSE 0 END) AS bdo,
				MAX(COALESCE(b.actual_end, b.scheduled_start)) AS last_collected_at
			FROM bookings b
			WHERE b.status = 'completed'
			  AND b.therapist_id IS NOT NULL
			  AND ($1::timestamptz IS NULL OR COALESCE(b.actual_end, b.scheduled_start) >= $1)
			  AND ($2::timestamptz IS NULL OR COALESCE(b.actual_end, b.scheduled_start) < $2)
			GROUP BY b.therapist_id
		),
		asof_cash AS (
			-- Cumulative cash collected through the end of the requested day
			-- ($2 upper bound). With no date filter this is the all-time total.
			SELECT
				b.therapist_id,
				SUM(b.final_total) AS cash_collected
			FROM bookings b
			WHERE b.status = 'completed'
			  AND b.payment_method = 'cash'
			  AND b.therapist_id IS NOT NULL
			  AND ($2::timestamptz IS NULL OR COALESCE(b.actual_end, b.scheduled_start) < $2)
			GROUP BY b.therapist_id
		),
		asof_remitted AS (
			-- Cumulative remitted through the end of the requested day.
			SELECT therapist_id, SUM(amount) AS total_remitted
			FROM cash_remittances
			WHERE ($2::timestamptz IS NULL OR created_at < $2)
			GROUP BY therapist_id
		),
		day_remitted AS (
			SELECT therapist_id, SUM(amount) AS remitted
			FROM cash_remittances
			WHERE ($1::timestamptz IS NULL OR created_at >= $1)
			  AND ($2::timestamptz IS NULL OR created_at < $2)
			GROUP BY therapist_id
		)
		SELECT
			u.user_id AS therapist_id,
			u.full_name,
			COALESCE(br.branch_name, '') AS branch_name,
			COALESCE(db.cash,  0) AS cash,
			COALESCE(db.gcash, 0) AS gcash,
			COALESCE(db.maya,  0) AS maya,
			COALESCE(db.bdo,   0) AS bdo,
			COALESCE(db.cash, 0) + COALESCE(db.gcash, 0) + COALESCE(db.maya, 0) + COALESCE(db.bdo, 0) AS total_collected,
			COALESCE(ac.cash_collected, 0) AS asof_cash,
			COALESCE(dr.remitted, 0) AS day_remitted,
			COALESCE(ar.total_remitted, 0) AS asof_remitted,
			db.last_collected_at
		FROM users u
		JOIN therapist_profiles tp ON tp.therapist_id = u.user_id
		LEFT JOIN branches br ON br.branch_id = tp.branch_id
		LEFT JOIN day_breakdown db ON db.therapist_id = u.user_id
		LEFT JOIN asof_cash ac ON ac.therapist_id = u.user_id
		LEFT JOIN asof_remitted ar ON ar.therapist_id = u.user_id
		LEFT JOIN day_remitted dr ON dr.therapist_id = u.user_id
		WHERE (
			db.therapist_id IS NOT NULL
			OR COALESCE(dr.remitted, 0) > 0
			OR (COALESCE(ac.cash_collected, 0) - COALESCE(ar.total_remitted, 0)) > 0
		)
		ORDER BY (COALESCE(ac.cash_collected, 0) - COALESCE(ar.total_remitted, 0)) DESC, u.full_name ASC
	`
	rows, err := r.db.Query(ctx, query, dateFrom, dateTo)
	if err != nil {
		return nil, fmt.Errorf("query therapist cash on hand: %w", err)
	}
	defer rows.Close()

	var result []model.TherapistCashOnHand
	for rows.Next() {
		var item model.TherapistCashOnHand
		var asOfCash, asOfRemitted float64
		if err := rows.Scan(
			&item.TherapistID,
			&item.TherapistName,
			&item.BranchName,
			&item.Cash,
			&item.GCash,
			&item.Maya,
			&item.BDO,
			&item.TotalCollected,
			&asOfCash,
			&item.TotalRemitted, // day-scoped: amount remitted within the requested range
			&asOfRemitted,
			&item.LastCollectedAt,
		); err != nil {
			return nil, fmt.Errorf("scan therapist cash on hand: %w", err)
		}
		// Cash on hand is the running balance as of the end of the requested day
		// (cumulative cash collected minus cumulative remitted through that day).
		item.CashOnHand = asOfCash - asOfRemitted
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate therapist cash on hand: %w", err)
	}
	return result, nil
}

// GetTherapistCashOnHand returns the all-time collected/remitted totals for one
// therapist (cash bookings only). Used by the remittance validation flow.
func (r *cashRemittanceRepo) GetTherapistCashOnHand(ctx context.Context, therapistID int64) (*model.TherapistCashOnHand, error) {
	query := `
		SELECT
			COALESCE((
				SELECT SUM(final_total) FROM bookings
				WHERE therapist_id = $1 AND status = 'completed' AND payment_method = 'cash'
			), 0) AS total_collected,
			COALESCE((
				SELECT SUM(amount) FROM cash_remittances WHERE therapist_id = $1
			), 0) AS total_remitted,
			COALESCE((SELECT full_name FROM users WHERE user_id = $1), '') AS full_name
	`
	item := &model.TherapistCashOnHand{TherapistID: therapistID}
	var allTimeCash float64
	if err := r.db.QueryRow(ctx, query, therapistID).Scan(
		&allTimeCash, &item.TotalRemitted, &item.TherapistName,
	); err != nil {
		return nil, fmt.Errorf("get therapist cash on hand: %w", err)
	}
	item.Cash = allTimeCash
	item.TotalCollected = allTimeCash
	item.CashOnHand = allTimeCash - item.TotalRemitted
	return item, nil
}

func (r *cashRemittanceRepo) CreateRemittance(ctx context.Context, remittance *model.CashRemittance) error {
	query := `
		INSERT INTO cash_remittances (therapist_id, amount, notes, remitted_by)
		VALUES ($1, $2, $3, $4)
		RETURNING remittance_id, created_at
	`
	if err := r.db.QueryRow(ctx, query,
		remittance.TherapistID, remittance.Amount, remittance.Notes, remittance.RemittedBy,
	).Scan(&remittance.RemittanceID, &remittance.CreatedAt); err != nil {
		return fmt.Errorf("create cash remittance: %w", err)
	}
	return nil
}

func (r *cashRemittanceRepo) ListRemittancesByTherapist(ctx context.Context, therapistID int64, limit int) ([]model.CashRemittance, error) {
	query := `
		SELECT cr.remittance_id, cr.therapist_id, cr.amount, COALESCE(cr.notes, ''),
		       cr.remitted_by, COALESCE(u.full_name, ''), cr.created_at
		FROM cash_remittances cr
		LEFT JOIN users u ON u.user_id = cr.remitted_by
		WHERE cr.therapist_id = $1
		ORDER BY cr.created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, therapistID, limit)
	if err != nil {
		return nil, fmt.Errorf("query remittances by therapist: %w", err)
	}
	defer rows.Close()

	var result []model.CashRemittance
	for rows.Next() {
		var item model.CashRemittance
		if err := rows.Scan(
			&item.RemittanceID, &item.TherapistID, &item.Amount, &item.Notes,
			&item.RemittedBy, &item.RemittedByName, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan remittance: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate remittances: %w", err)
	}
	return result, nil
}
