package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// BookingServiceRepository handles persistence for the booking_services join table.
type BookingServiceRepository interface {
	CreateManyTx(ctx context.Context, tx pgx.Tx, services []model.BookingService) error
	ListByBookingID(ctx context.Context, bookingID int64) ([]model.BookingService, error)
	ListByBookingIDWithService(ctx context.Context, bookingID int64) ([]model.BookingService, error)
	ReplaceByBookingID(ctx context.Context, bookingID int64, services []model.BookingService, paymentBreakdown []byte) error
	DeleteByBookingIDTx(ctx context.Context, tx pgx.Tx, bookingID int64) error
}

type bookingServiceRepo struct {
	db db.DBTX
}

func NewBookingServiceRepository(db db.DBTX) BookingServiceRepository {
	return &bookingServiceRepo{db: db}
}

func (r *bookingServiceRepo) CreateManyTx(ctx context.Context, tx pgx.Tx, services []model.BookingService) error {
	if len(services) == 0 {
		return nil
	}

	bIDs := make([]int64, len(services))
	sIDs := make([]int64, len(services))
	positions := make([]int, len(services))
	prices := make([]float64, len(services))
	durations := make([]int, len(services))
	allocatedDurations := make([]int, len(services))

	for i, s := range services {
		bIDs[i] = s.BookingID
		sIDs[i] = s.ServiceID
		positions[i] = s.Position
		prices[i] = s.PriceSnapshot
		durations[i] = s.DurationSnapshot
		if s.AllocatedDurationMinutes != nil {
			allocatedDurations[i] = *s.AllocatedDurationMinutes
		}
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO booking_services (booking_id, service_id, position, price_snapshot, duration_snapshot, allocated_duration_minutes)
		SELECT UNNEST($1::bigint[]), UNNEST($2::bigint[]), UNNEST($3::int[]), UNNEST($4::numeric[]), UNNEST($5::int[]), NULLIF(UNNEST($6::int[]), 0)
	`, bIDs, sIDs, positions, prices, durations, allocatedDurations)
	return err
}

func (r *bookingServiceRepo) ListByBookingID(ctx context.Context, bookingID int64) ([]model.BookingService, error) {
	rows, err := r.db.Query(ctx, `
		SELECT booking_service_id, booking_id, service_id, position, price_snapshot, duration_snapshot, allocated_duration_minutes, created_at
		FROM booking_services
		WHERE booking_id = $1
		ORDER BY position ASC
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.BookingService
	for rows.Next() {
		var bs model.BookingService
		if err := rows.Scan(&bs.BookingServiceID, &bs.BookingID, &bs.ServiceID, &bs.Position, &bs.PriceSnapshot, &bs.DurationSnapshot, &bs.AllocatedDurationMinutes, &bs.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, bs)
	}
	return result, rows.Err()
}

func (r *bookingServiceRepo) ListByBookingIDWithService(ctx context.Context, bookingID int64) ([]model.BookingService, error) {
	rows, err := r.db.Query(ctx, `
		SELECT bs.booking_service_id, bs.booking_id, bs.service_id, bs.position, bs.price_snapshot, bs.duration_snapshot, bs.allocated_duration_minutes, bs.created_at,
		       s.service_id, s.name, s.description, s.base_price, s.duration_minutes, s.category,
		       s.preview_image_url, s.therapist_commission, s.subtitle, s.is_featured, s.featured_order, s.is_active, s.created_at
		FROM booking_services bs
		JOIN services s ON bs.service_id = s.service_id
		WHERE bs.booking_id = $1
		ORDER BY bs.position ASC
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.BookingService
	for rows.Next() {
		var bs model.BookingService
		var svc model.Service
		if err := rows.Scan(
			&bs.BookingServiceID, &bs.BookingID, &bs.ServiceID, &bs.Position, &bs.PriceSnapshot, &bs.DurationSnapshot, &bs.AllocatedDurationMinutes, &bs.CreatedAt,
			&svc.ServiceID, &svc.Name, &svc.Description, &svc.BasePrice, &svc.DurationMinutes, &svc.Category,
			&svc.PreviewImageURL, &svc.TherapistCommission, &svc.Subtitle, &svc.IsFeatured, &svc.FeaturedOrder, &svc.IsActive, &svc.CreatedAt,
		); err != nil {
			return nil, err
		}
		bs.Service = &svc
		result = append(result, bs)
	}
	return result, rows.Err()
}

func (r *bookingServiceRepo) ReplaceByBookingID(ctx context.Context, bookingID int64, services []model.BookingService, paymentBreakdown []byte) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.DeleteByBookingIDTx(ctx, tx, bookingID); err != nil {
		return err
	}
	if len(services) > 0 {
		for i := range services {
			services[i].BookingID = bookingID
		}
		if err := r.CreateManyTx(ctx, tx, services); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE bookings SET payment_breakdown = $1, updated_at = NOW() WHERE booking_id = $2`, paymentBreakdown, bookingID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *bookingServiceRepo) DeleteByBookingIDTx(ctx context.Context, tx pgx.Tx, bookingID int64) error {
	_, err := tx.Exec(ctx, `DELETE FROM booking_services WHERE booking_id = $1`, bookingID)
	return err
}
