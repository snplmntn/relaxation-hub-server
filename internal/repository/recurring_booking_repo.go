package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// RecurringBookingRepository defines data access for recurring booking series.
type RecurringBookingRepository interface {
	Create(ctx context.Context, r *model.RecurringBooking) error
	GetByID(ctx context.Context, id int64) (*model.RecurringBooking, error)
	List(ctx context.Context, status string, clientID *int64, limit, offset int) ([]model.RecurringBooking, int, error)
	Update(ctx context.Context, r *model.RecurringBooking) error
	SetGeneratedUntilTx(ctx context.Context, tx pgx.Tx, id int64, ts time.Time) error
	ListActiveForGeneration(ctx context.Context, now time.Time) ([]model.RecurringBooking, error)
	// CancelFuturePendingTx cancels not-yet-started pending occurrences for the series.
	CancelFuturePendingTx(ctx context.Context, tx pgx.Tx, recurringID int64, after time.Time) error
}

type recurringBookingRepo struct {
	db db.DBTX
}

// NewRecurringBookingRepository creates a new RecurringBookingRepository.
func NewRecurringBookingRepository(db db.DBTX) RecurringBookingRepository {
	return &recurringBookingRepo{db: db}
}

func (r *recurringBookingRepo) Create(ctx context.Context, rec *model.RecurringBooking) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		INSERT INTO recurring_bookings (
			client_id, created_by, service_id, address_id, therapist_id, is_therapist_requested,
			duration_minutes, gender_preference, pressure_preference, notes, payment_method,
			frequency, interval_value, days_of_week, day_of_month, time_of_day,
			start_date, end_date, status
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19
		)
		RETURNING recurring_id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		rec.ClientID, rec.CreatedBy, rec.ServiceID, rec.AddressID, rec.TherapistID, rec.IsTherapistRequested,
		rec.DurationMinutes, rec.GenderPref, rec.PressurePref, rec.Notes, rec.PaymentMethod,
		rec.Frequency, rec.IntervalValue, rec.DaysOfWeek, rec.DayOfMonth, rec.TimeOfDay,
		rec.StartDate, rec.EndDate, rec.Status,
	).Scan(&rec.RecurringID, &rec.CreatedAt, &rec.UpdatedAt)
}

func (r *recurringBookingRepo) GetByID(ctx context.Context, id int64) (*model.RecurringBooking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rec := &model.RecurringBooking{}
	var daysOfWeek []int32
	err := r.db.QueryRow(ctx, `
		SELECT recurring_id, client_id, created_by, service_id, address_id, therapist_id, is_therapist_requested,
		       duration_minutes, gender_preference, pressure_preference, notes, payment_method,
		       frequency, interval_value, days_of_week, day_of_month, time_of_day,
		       start_date, end_date, status, generated_until, created_at, updated_at
		FROM recurring_bookings WHERE recurring_id = $1
	`, id).Scan(
		&rec.RecurringID, &rec.ClientID, &rec.CreatedBy, &rec.ServiceID, &rec.AddressID, &rec.TherapistID, &rec.IsTherapistRequested,
		&rec.DurationMinutes, &rec.GenderPref, &rec.PressurePref, &rec.Notes, &rec.PaymentMethod,
		&rec.Frequency, &rec.IntervalValue, &daysOfWeek, &rec.DayOfMonth, &rec.TimeOfDay,
		&rec.StartDate, &rec.EndDate, &rec.Status, &rec.GeneratedUntil, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	rec.DaysOfWeek = int32SliceToInt(daysOfWeek)
	return rec, nil
}

func (r *recurringBookingRepo) List(ctx context.Context, status string, clientID *int64, limit, offset int) ([]model.RecurringBooking, int, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	args := []any{}
	where := "WHERE 1=1"
	i := 1
	if status != "" && status != "all" {
		where += " AND status = $" + strconv.Itoa(i)
		args = append(args, status)
		i++
	}
	if clientID != nil {
		where += " AND client_id = $" + strconv.Itoa(i)
		args = append(args, *clientID)
		i++
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM recurring_bookings "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, `
		SELECT recurring_id, client_id, created_by, service_id, address_id, therapist_id, is_therapist_requested,
		       duration_minutes, gender_preference, pressure_preference, notes, payment_method,
		       frequency, interval_value, days_of_week, day_of_month, time_of_day,
		       start_date, end_date, status, generated_until, created_at, updated_at
		FROM recurring_bookings `+where+`
		ORDER BY created_at DESC
		LIMIT $`+strconv.Itoa(i)+` OFFSET $`+strconv.Itoa(i+1),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []model.RecurringBooking
	for rows.Next() {
		var rec model.RecurringBooking
		var daysOfWeek []int32
		if err := rows.Scan(
			&rec.RecurringID, &rec.ClientID, &rec.CreatedBy, &rec.ServiceID, &rec.AddressID, &rec.TherapistID, &rec.IsTherapistRequested,
			&rec.DurationMinutes, &rec.GenderPref, &rec.PressurePref, &rec.Notes, &rec.PaymentMethod,
			&rec.Frequency, &rec.IntervalValue, &daysOfWeek, &rec.DayOfMonth, &rec.TimeOfDay,
			&rec.StartDate, &rec.EndDate, &rec.Status, &rec.GeneratedUntil, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		rec.DaysOfWeek = int32SliceToInt(daysOfWeek)
		out = append(out, rec)
	}
	return out, total, rows.Err()
}

func (r *recurringBookingRepo) Update(ctx context.Context, rec *model.RecurringBooking) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := r.db.Exec(ctx, `
		UPDATE recurring_bookings
		SET status = $1, end_date = $2, notes = $3, payment_method = $4, therapist_id = $5, updated_at = NOW()
		WHERE recurring_id = $6
	`, rec.Status, rec.EndDate, rec.Notes, rec.PaymentMethod, rec.TherapistID, rec.RecurringID)
	return err
}

func (r *recurringBookingRepo) SetGeneratedUntilTx(ctx context.Context, tx pgx.Tx, id int64, ts time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE recurring_bookings SET generated_until = $1, updated_at = NOW() WHERE recurring_id = $2
	`, ts, id)
	return err
}

func (r *recurringBookingRepo) ListActiveForGeneration(ctx context.Context, now time.Time) ([]model.RecurringBooking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT recurring_id, client_id, created_by, service_id, address_id, therapist_id, is_therapist_requested,
		       duration_minutes, gender_preference, pressure_preference, notes, payment_method,
		       frequency, interval_value, days_of_week, day_of_month, time_of_day,
		       start_date, end_date, status, generated_until, created_at, updated_at
		FROM recurring_bookings
		WHERE status = 'active'
		  AND start_date <= $1
		  AND (end_date IS NULL OR end_date >= $2)
	`, now, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.RecurringBooking
	for rows.Next() {
		var rec model.RecurringBooking
		var daysOfWeek []int32
		if err := rows.Scan(
			&rec.RecurringID, &rec.ClientID, &rec.CreatedBy, &rec.ServiceID, &rec.AddressID, &rec.TherapistID, &rec.IsTherapistRequested,
			&rec.DurationMinutes, &rec.GenderPref, &rec.PressurePref, &rec.Notes, &rec.PaymentMethod,
			&rec.Frequency, &rec.IntervalValue, &daysOfWeek, &rec.DayOfMonth, &rec.TimeOfDay,
			&rec.StartDate, &rec.EndDate, &rec.Status, &rec.GeneratedUntil, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rec.DaysOfWeek = int32SliceToInt(daysOfWeek)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *recurringBookingRepo) CancelFuturePendingTx(ctx context.Context, tx pgx.Tx, recurringID int64, after time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE bookings
		SET status = 'cancelled', cancelled_at = NOW(), cancellation_reason = 'recurring series cancelled', updated_at = NOW()
		WHERE recurring_id = $1
		  AND status = 'pending'
		  AND (scheduled_start IS NULL OR scheduled_start > $2)
	`, recurringID, after)
	return err
}

func int32SliceToInt(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}
