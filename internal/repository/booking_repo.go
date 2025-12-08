package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// BookingRepository defines data access methods for bookings.
type BookingRepository interface {
    Create(ctx context.Context, booking *model.Booking) error
    GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error)
    ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error)
    Update(ctx context.Context, booking *model.Booking) error
    UpdateStatus(ctx context.Context, bookingID, userID int64, status string) error
}

type bookingRepoImpl struct {
    db *pgxpool.Pool
}

func NewBookingRepository(db *pgxpool.Pool) BookingRepository {
    return &bookingRepoImpl{db: db}
}

func (r *bookingRepoImpl) Create(ctx context.Context, booking *model.Booking) error {
    query := `
        INSERT INTO bookings (
            client_id, therapist_id, service_id, address_id, promo_id,
            gender_preference, pressure_preference, notes,
            duration_minutes, scheduled_start, raw_total, discount, final_total, status
        ) VALUES (
            $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14
        )
        RETURNING booking_id, created_at, updated_at
    `

    return r.db.QueryRow(ctx, query,
        booking.ClientID,
        booking.TherapistID,
        booking.ServiceID,
        booking.AddressID,
        booking.PromoID,
        booking.GenderPref,
        booking.PressurePref,
        booking.Notes,
        booking.DurationMinutes,
        booking.ScheduledStart,
        booking.RawTotal,
        booking.Discount,
        booking.FinalTotal,
        booking.Status,
    ).Scan(&booking.BookingID, &booking.CreatedAt, &booking.UpdatedAt)
}

func (r *bookingRepoImpl) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
    query := `
        SELECT booking_id, client_id, therapist_id, service_id, address_id, promo_id,
               gender_preference, pressure_preference, notes, duration_minutes,
               scheduled_start, actual_start, actual_end,
               raw_total, discount, final_total, status,
               created_at, updated_at
        FROM bookings
        WHERE booking_id = $1 AND client_id = $2
    `

    var b model.Booking
    if err := r.db.QueryRow(ctx, query, bookingID, userID).Scan(
        &b.BookingID,
        &b.ClientID,
        &b.TherapistID,
        &b.ServiceID,
        &b.AddressID,
        &b.PromoID,
        &b.GenderPref,
        &b.PressurePref,
        &b.Notes,
        &b.DurationMinutes,
        &b.ScheduledStart,
        &b.ActualStart,
        &b.ActualEnd,
        &b.RawTotal,
        &b.Discount,
        &b.FinalTotal,
        &b.Status,
        &b.CreatedAt,
        &b.UpdatedAt,
    ); err != nil {
        if err == pgx.ErrNoRows {
            return nil, err
        }
        return nil, err
    }

    return &b, nil
}

func (r *bookingRepoImpl) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
    query := `
        SELECT booking_id, client_id, therapist_id, service_id, address_id, promo_id,
               gender_preference, pressure_preference, notes, duration_minutes,
               scheduled_start, actual_start, actual_end,
               raw_total, discount, final_total, status,
               created_at, updated_at
        FROM bookings
        WHERE client_id = $1
        ORDER BY created_at DESC
    `

    rows, err := r.db.Query(ctx, query, clientID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []model.Booking
    for rows.Next() {
        var b model.Booking
        if err := rows.Scan(
            &b.BookingID,
            &b.ClientID,
            &b.TherapistID,
            &b.ServiceID,
            &b.AddressID,
            &b.PromoID,
            &b.GenderPref,
            &b.PressurePref,
            &b.Notes,
            &b.DurationMinutes,
            &b.ScheduledStart,
            &b.ActualStart,
            &b.ActualEnd,
            &b.RawTotal,
            &b.Discount,
            &b.FinalTotal,
            &b.Status,
            &b.CreatedAt,
            &b.UpdatedAt,
        ); err != nil {
            return nil, err
        }
        out = append(out, b)
    }

    return out, nil
}

func (r *bookingRepoImpl) Update(ctx context.Context, booking *model.Booking) error {
    cmd, err := r.db.Exec(ctx, `
        UPDATE bookings
        SET service_id = $1,
            address_id = $2,
            promo_id = $3,
            gender_preference = $4,
            pressure_preference = $5,
            notes = $6,
            duration_minutes = $7,
            scheduled_start = $8
        WHERE booking_id = $9 AND client_id = $10
    `, booking.ServiceID, booking.AddressID, booking.PromoID, booking.GenderPref, booking.PressurePref,
        booking.Notes, booking.DurationMinutes, booking.ScheduledStart, booking.BookingID, booking.ClientID)
    if err != nil {
        return err
    }
    if cmd.RowsAffected() == 0 {
        return pgx.ErrNoRows
    }
    return nil
}

func (r *bookingRepoImpl) UpdateStatus(ctx context.Context, bookingID, userID int64, status string) error {
    cmd, err := r.db.Exec(ctx, `
        UPDATE bookings
        SET status = $1, updated_at = $2
        WHERE booking_id = $3 AND client_id = $4
    `, status, time.Now(), bookingID, userID)
    if err != nil {
        return err
    }
    if cmd.RowsAffected() == 0 {
        return pgx.ErrNoRows
    }
    return nil
}
