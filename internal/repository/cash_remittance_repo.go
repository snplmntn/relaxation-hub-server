package repository

import (
	"context"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// CashRemittanceRepository handles cash-on-hand aggregation and remittance records.
type CashRemittanceRepository interface {
	ListTherapistCashOnHand(ctx context.Context) ([]model.TherapistCashOnHand, error)
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

// ListTherapistCashOnHand aggregates every therapist with completed cash bookings,
// summing the full client-paid total and subtracting amounts already remitted.
func (r *cashRemittanceRepo) ListTherapistCashOnHand(ctx context.Context) ([]model.TherapistCashOnHand, error) {
	query := `
		SELECT
			b.therapist_id,
			u.full_name,
			COALESCE(br.branch_name, '') AS branch_name,
			COALESCE(SUM(b.final_total), 0) AS total_collected,
			COUNT(*) AS completed_cash_bookings,
			MAX(b.actual_end) AS last_collected_at,
			COALESCE(rem.total_remitted, 0) AS total_remitted
		FROM bookings b
		JOIN users u ON u.user_id = b.therapist_id
		LEFT JOIN therapist_profiles tp ON tp.therapist_id = b.therapist_id
		LEFT JOIN branches br ON br.branch_id = tp.branch_id
		LEFT JOIN (
			SELECT therapist_id, SUM(amount) AS total_remitted
			FROM cash_remittances
			GROUP BY therapist_id
		) rem ON rem.therapist_id = b.therapist_id
		WHERE b.status = 'completed'
		  AND b.payment_method = 'cash'
		  AND b.therapist_id IS NOT NULL
		GROUP BY b.therapist_id, u.full_name, br.branch_name, rem.total_remitted
		ORDER BY (COALESCE(SUM(b.final_total), 0) - COALESCE(rem.total_remitted, 0)) DESC, u.full_name ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.TherapistCashOnHand
	for rows.Next() {
		var item model.TherapistCashOnHand
		if err := rows.Scan(
			&item.TherapistID,
			&item.TherapistName,
			&item.BranchName,
			&item.TotalCollected,
			&item.CompletedCashBookings,
			&item.LastCollectedAt,
			&item.TotalRemitted,
		); err != nil {
			return nil, err
		}
		item.CashOnHand = item.TotalCollected - item.TotalRemitted
		result = append(result, item)
	}
	return result, rows.Err()
}

// GetTherapistCashOnHand returns the collected/remitted totals for one therapist.
// It always returns a row (zeros when there are no completed cash bookings).
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
	if err := r.db.QueryRow(ctx, query, therapistID).Scan(
		&item.TotalCollected, &item.TotalRemitted, &item.TherapistName,
	); err != nil {
		return nil, err
	}
	item.CashOnHand = item.TotalCollected - item.TotalRemitted
	return item, nil
}

func (r *cashRemittanceRepo) CreateRemittance(ctx context.Context, remittance *model.CashRemittance) error {
	query := `
		INSERT INTO cash_remittances (therapist_id, amount, notes, remitted_by)
		VALUES ($1, $2, $3, $4)
		RETURNING remittance_id, created_at
	`
	return r.db.QueryRow(ctx, query,
		remittance.TherapistID, remittance.Amount, remittance.Notes, remittance.RemittedBy,
	).Scan(&remittance.RemittanceID, &remittance.CreatedAt)
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
		return nil, err
	}
	defer rows.Close()

	var result []model.CashRemittance
	for rows.Next() {
		var item model.CashRemittance
		if err := rows.Scan(
			&item.RemittanceID, &item.TherapistID, &item.Amount, &item.Notes,
			&item.RemittedBy, &item.RemittedByName, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
