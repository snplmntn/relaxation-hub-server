package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type DayViewOrderRepository interface {
	GetByViewAndBusinessDate(ctx context.Context, viewKey string, businessDate time.Time) (*model.DayViewTherapistOrder, error)
	Upsert(ctx context.Context, order *model.DayViewTherapistOrder) error
	ListTherapistsByBranch(ctx context.Context, branchID *int64) ([]model.DayViewTherapistCandidate, error)
	GetTherapistHoursBetween(ctx context.Context, therapistIDs []int64, startTime, endTime time.Time) (map[int64]float64, error)
}

type dayViewOrderRepoImpl struct {
	db db.DBTX
}

func NewDayViewOrderRepository(database db.DBTX) DayViewOrderRepository {
	return &dayViewOrderRepoImpl{db: database}
}

func (r *dayViewOrderRepoImpl) GetByViewAndBusinessDate(ctx context.Context, viewKey string, businessDate time.Time) (*model.DayViewTherapistOrder, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	order := &model.DayViewTherapistOrder{}
	err := r.db.QueryRow(ctx, `
		SELECT order_id, view_key, business_date, therapist_ids, source, updated_by_admin_id, created_at, updated_at
		FROM day_view_therapist_orders
		WHERE view_key = $1 AND business_date = $2::date
	`, viewKey, businessDate.Format("2006-01-02")).Scan(
		&order.OrderID,
		&order.ViewKey,
		&order.BusinessDate,
		&order.TherapistIDs,
		&order.Source,
		&order.UpdatedByAdminID,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (r *dayViewOrderRepoImpl) Upsert(ctx context.Context, order *model.DayViewTherapistOrder) error {
	if order == nil {
		return fmt.Errorf("order is required")
	}

	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return r.db.QueryRow(ctx, `
		INSERT INTO day_view_therapist_orders (view_key, business_date, therapist_ids, source, updated_by_admin_id)
		VALUES ($1, $2::date, $3, $4, $5)
		ON CONFLICT (view_key, business_date)
		DO UPDATE SET
			therapist_ids = EXCLUDED.therapist_ids,
			source = EXCLUDED.source,
			updated_by_admin_id = EXCLUDED.updated_by_admin_id,
			updated_at = NOW()
		RETURNING order_id, created_at, updated_at
	`, order.ViewKey, order.BusinessDate.Format("2006-01-02"), order.TherapistIDs, order.Source, order.UpdatedByAdminID).Scan(
		&order.OrderID,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
}

func (r *dayViewOrderRepoImpl) ListTherapistsByBranch(ctx context.Context, branchID *int64) ([]model.DayViewTherapistCandidate, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	baseQuery := `
		SELECT
			tp.therapist_id,
			COALESCE(NULLIF(TRIM(u.full_name), ''), u.primary_email, u.primary_phone, '') AS display_name
		FROM therapist_profiles tp
		LEFT JOIN users u ON u.user_id = tp.therapist_id
		WHERE tp.accept_assignments = TRUE
	`

	var rows pgx.Rows
	var err error
	if branchID == nil {
		rows, err = r.db.Query(ctx, baseQuery+` AND tp.branch_id IS NULL ORDER BY display_name ASC, tp.therapist_id ASC`)
	} else {
		rows, err = r.db.Query(ctx, baseQuery+` AND tp.branch_id = $1 ORDER BY display_name ASC, tp.therapist_id ASC`, *branchID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]model.DayViewTherapistCandidate, 0)
	for rows.Next() {
		var candidate model.DayViewTherapistCandidate
		if err := rows.Scan(&candidate.TherapistID, &candidate.Name); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}

	return candidates, rows.Err()
}

func (r *dayViewOrderRepoImpl) GetTherapistHoursBetween(ctx context.Context, therapistIDs []int64, startTime, endTime time.Time) (map[int64]float64, error) {
	ctx, cancel := db.WithLongQueryTimeout(ctx)
	defer cancel()

	hours := make(map[int64]float64, len(therapistIDs))
	if len(therapistIDs) == 0 {
		return hours, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT therapist_id, COALESCE(SUM(duration_minutes), 0) / 60.0 AS total_hours
		FROM bookings
		WHERE status = 'completed'
		  AND therapist_id = ANY($1)
		  AND actual_end >= $2::timestamp
		  AND actual_end < $3::timestamp
		GROUP BY therapist_id
	`, therapistIDs, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var therapistID int64
		var total float64
		if err := rows.Scan(&therapistID, &total); err != nil {
			return nil, err
		}
		hours[therapistID] = total
	}

	return hours, rows.Err()
}
