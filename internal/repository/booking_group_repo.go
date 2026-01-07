package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// BookingGroupRepository defines the interface for booking group data access.
type BookingGroupRepository interface {
	CreateTx(ctx context.Context, tx pgx.Tx, g *model.BookingGroup) error
	GetByID(ctx context.Context, groupID int64) (*model.BookingGroup, error)
	GetByIDWithBookings(ctx context.Context, groupID int64) (*model.BookingGroup, error)
	UpdateStatus(ctx context.Context, groupID int64, status string) error
	ListByClient(ctx context.Context, clientID int64) ([]model.BookingGroup, error)
}

type bookingGroupRepo struct {
	db db.DBTX
}

// NewBookingGroupRepository creates a new BookingGroupRepository.
func NewBookingGroupRepository(db db.DBTX) BookingGroupRepository {
	return &bookingGroupRepo{db: db}
}

func (r *bookingGroupRepo) CreateTx(ctx context.Context, tx pgx.Tx, g *model.BookingGroup) error {
	query := `
		INSERT INTO booking_groups (client_id, address_id, scheduled_start, raw_total, discount, final_total, payment_method, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING group_id, created_at, updated_at
	`
	return tx.QueryRow(ctx, query,
		g.ClientID, g.AddressID, g.ScheduledStart, g.RawTotal, g.Discount, g.FinalTotal, g.PaymentMethod, g.Status,
	).Scan(&g.GroupID, &g.CreatedAt, &g.UpdatedAt)
}

func (r *bookingGroupRepo) GetByID(ctx context.Context, groupID int64) (*model.BookingGroup, error) {
	query := `
		SELECT group_id, client_id, address_id, scheduled_start, raw_total, discount, final_total, payment_method, status, created_at, updated_at
		FROM booking_groups WHERE group_id = $1
	`
	g := &model.BookingGroup{}
	var scheduledStart *time.Time
	err := r.db.QueryRow(ctx, query, groupID).Scan(
		&g.GroupID, &g.ClientID, &g.AddressID, &scheduledStart, &g.RawTotal, &g.Discount, &g.FinalTotal, &g.PaymentMethod, &g.Status, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	g.ScheduledStart = scheduledStart
	return g, nil
}

func (r *bookingGroupRepo) GetByIDWithBookings(ctx context.Context, groupID int64) (*model.BookingGroup, error) {
	g, err := r.GetByID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	// Fetch bookings for this group
	bookingsQuery := `
		SELECT booking_id, client_id, therapist_id, service_id, address_id, gender_preference, pressure_preference,
		       notes, duration_minutes, scheduled_start, status, group_id, guest_name, sequence_number, start_condition,
		       raw_total, discount, final_total, created_at
		FROM bookings WHERE group_id = $1 ORDER BY sequence_number
	`
	rows, err := r.db.Query(ctx, bookingsQuery, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []model.Booking
	for rows.Next() {
		var b model.Booking
		var scheduledStart *time.Time
		if err := rows.Scan(
			&b.BookingID, &b.ClientID, &b.TherapistID, &b.ServiceID, &b.AddressID, &b.GenderPref, &b.PressurePref,
			&b.Notes, &b.DurationMinutes, &scheduledStart, &b.Status, &b.GroupID, &b.GuestName, &b.SequenceNumber, &b.StartCondition,
			&b.RawTotal, &b.Discount, &b.FinalTotal, &b.CreatedAt,
		); err != nil {
			return nil, err
		}
		b.ScheduledStart = scheduledStart
		bookings = append(bookings, b)
	}
	g.Bookings = bookings
	return g, rows.Err()
}

func (r *bookingGroupRepo) UpdateStatus(ctx context.Context, groupID int64, status string) error {
	query := `UPDATE booking_groups SET status = $1, updated_at = NOW() WHERE group_id = $2`
	_, err := r.db.Exec(ctx, query, status, groupID)
	return err
}

func (r *bookingGroupRepo) ListByClient(ctx context.Context, clientID int64) ([]model.BookingGroup, error) {
	query := `
		SELECT group_id, client_id, address_id, scheduled_start, raw_total, discount, final_total, payment_method, status, created_at, updated_at
		FROM booking_groups WHERE client_id = $1 ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []model.BookingGroup
	for rows.Next() {
		var g model.BookingGroup
		var scheduledStart *time.Time
		if err := rows.Scan(
			&g.GroupID, &g.ClientID, &g.AddressID, &scheduledStart, &g.RawTotal, &g.Discount, &g.FinalTotal, &g.PaymentMethod, &g.Status, &g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, err
		}
		g.ScheduledStart = scheduledStart
		groups = append(groups, g)
	}
	return groups, rows.Err()
}
