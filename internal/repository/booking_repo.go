package repository

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

var (
	ErrTherapistNotFound     = errors.New("therapist not found")
	ErrTherapistNotAccepting = errors.New("therapist not accepting assignments")
	ErrAlreadyAssigned       = errors.New("booking already assigned")
	ErrBookingNotAssignable  = errors.New("booking not in assignable state")
	ErrAssignConflict        = errors.New("assignment conflict")
)

// BookingRepository defines data access methods for bookings.
type BookingRepository interface {
	Create(ctx context.Context, booking *model.Booking) error
	// CreateTx inserts a booking using the provided transaction.
	CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error
	GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error)
	ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error)
	ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error)
	Update(ctx context.Context, booking *model.Booking) error
	// AssignTherapist sets therapist_id for a booking if it's currently NULL and status matches.
	AssignTherapist(ctx context.Context, bookingID, therapistID int64) error
	// AssignTherapistWithActor assigns a therapist and records the provided actor
	// (e.g., an admin) as the actor on the resulting 'assigned' booking_event.
	AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error
	// AssignTherapistWithActorTx behaves like AssignTherapistWithActor but uses
	// the provided transaction for the update and event insert. This allows
	// callers to create a booking and assign within the same DB transaction.
	AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error
	// GetByBookingID fetches a booking without client scoping (admin/worker usage).
	GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error)
	// ListEvents returns booking timeline events ordered by created_at asc
	ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error)
	// InsertEvent writes a booking event row for timeline/audit
	InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error
	// UpdateStatus updates the booking status if the acting user is either the
	// client or the assigned therapist (actorID). This ensures therapists can
	// confirm/accept bookings while clients can cancel or otherwise update.
	UpdateStatus(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string) error
	// GetRecentTherapistStruggleFlags returns a map of therapist_id -> true if the
	// therapist had one or more poor outcomes (cancellations/no-shows) since 'since'
	GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error)
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
			payment_method,
			gender_preference, pressure_preference, notes,
			duration_minutes, scheduled_start, raw_total, discount, final_total, status, reference_code
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16
		)
		RETURNING booking_id, created_at, updated_at, assigned_at, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason
    `

	return r.db.QueryRow(ctx, query,
		booking.ClientID,
		booking.TherapistID,
		booking.ServiceID,
		booking.AddressID,
		booking.PromoID,
		booking.PaymentMethod,
		booking.GenderPref,
		booking.PressurePref,
		booking.Notes,
		booking.DurationMinutes,
		booking.ScheduledStart,
		booking.RawTotal,
		booking.Discount,
		booking.FinalTotal,
		booking.Status,
		booking.ReferenceCode,
	).Scan(&booking.BookingID, &booking.CreatedAt, &booking.UpdatedAt, &booking.AssignedAt, &booking.TherapistArrivedAt, &booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason)
}

func (r *bookingRepoImpl) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	query := `
		INSERT INTO bookings (
			client_id, therapist_id, service_id, address_id, promo_id,
			payment_method,
			gender_preference, pressure_preference, notes,
			duration_minutes, scheduled_start, raw_total, discount, final_total, status, reference_code
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16
		)
		RETURNING booking_id, created_at, updated_at, assigned_at, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason
	`
	return tx.QueryRow(ctx, query,
		booking.ClientID,
		booking.TherapistID,
		booking.ServiceID,
		booking.AddressID,
		booking.PromoID,
		booking.PaymentMethod,
		booking.GenderPref,
		booking.PressurePref,
		booking.Notes,
		booking.DurationMinutes,
		booking.ScheduledStart,
		booking.RawTotal,
		booking.Discount,
		booking.FinalTotal,
		booking.Status,
		booking.ReferenceCode,
	).Scan(&booking.BookingID, &booking.CreatedAt, &booking.UpdatedAt, &booking.AssignedAt, &booking.TherapistArrivedAt, &booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason)
}

func (r *bookingRepoImpl) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	query := `
		 SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			 payment_method,
			 gender_preference, pressure_preference, notes, duration_minutes,
			 scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			 raw_total, discount, final_total, status,
			 created_at, updated_at
        FROM bookings
        WHERE booking_id = $1 AND client_id = $2
    `

	var b model.Booking
	if err := r.db.QueryRow(ctx, query, bookingID, userID).Scan(
		&b.BookingID,
		&b.ReferenceCode,
		&b.ClientID,
		&b.TherapistID,
		&b.AssignedAt,
		&b.ServiceID,
		&b.AddressID,
		&b.PromoID,
		&b.PaymentMethod,
		&b.GenderPref,
		&b.PressurePref,
		&b.Notes,
		&b.DurationMinutes,
		&b.ScheduledStart,
		&b.ActualStart,
		&b.ActualEnd,
		&b.TherapistArrivedAt,
		&b.NoShowAt,
		&b.CancelledBy,
		&b.CancelledAt,
		&b.CancellationReason,
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
		 SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			 payment_method,
			 gender_preference, pressure_preference, notes, duration_minutes,
			 scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
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
			&b.ReferenceCode,
			&b.ClientID,
			&b.TherapistID,
			&b.AssignedAt,
			&b.ServiceID,
			&b.AddressID,
			&b.PromoID,
			&b.PaymentMethod,
			&b.GenderPref,
			&b.PressurePref,
			&b.Notes,
			&b.DurationMinutes,
			&b.ScheduledStart,
			&b.ActualStart,
			&b.ActualEnd,
			&b.TherapistArrivedAt,
			&b.NoShowAt,
			&b.CancelledBy,
			&b.CancelledAt,
			&b.CancellationReason,
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

func (r *bookingRepoImpl) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	query := `
		 SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			 payment_method,
			 gender_preference, pressure_preference, notes, duration_minutes,
			 scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			 raw_total, discount, final_total, status,
			 created_at, updated_at
        FROM bookings
        WHERE therapist_id = $1
        ORDER BY created_at DESC
    `

	rows, err := r.db.Query(ctx, query, therapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Booking
	for rows.Next() {
		var b model.Booking
		if err := rows.Scan(
			&b.BookingID,
			&b.ReferenceCode,
			&b.ClientID,
			&b.TherapistID,
			&b.AssignedAt,
			&b.ServiceID,
			&b.AddressID,
			&b.PromoID,
			&b.PaymentMethod,
			&b.GenderPref,
			&b.PressurePref,
			&b.Notes,
			&b.DurationMinutes,
			&b.ScheduledStart,
			&b.ActualStart,
			&b.ActualEnd,
			&b.TherapistArrivedAt,
			&b.NoShowAt,
			&b.CancelledBy,
			&b.CancelledAt,
			&b.CancellationReason,
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
	if err := rows.Err(); err != nil {
		return nil, err
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
		log.Printf("Update booking failed: booking_id=%d client_id=%d err=%v", booking.BookingID, booking.ClientID, err)
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *bookingRepoImpl) insertBookingEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	// Insert event; metadata may be nil
	var md interface{}
	if metadata != nil {
		md = metadata
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO booking_events (booking_id, event_type, actor_id, metadata)
		VALUES ($1, $2, $3, $4)
	`, bookingID, eventType, actorID, md)
	return err
}

func (r *bookingRepoImpl) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error {
	// Pre-check therapist exists and accepts assignments for clearer errors
	var accept bool
	if err := r.db.QueryRow(ctx, `SELECT accept_assignments FROM therapist_profiles WHERE therapist_id = $1`, therapistID).Scan(&accept); err != nil {
		if err == pgx.ErrNoRows {
			return ErrTherapistNotFound
		}
		return err
	}
	if !accept {
		return ErrTherapistNotAccepting
	}

	now := time.Now()
	cmd, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET therapist_id = $1, assigned_at = $2, status = 'assigned', updated_at = $3
		WHERE booking_id = $4 AND therapist_id IS NULL
		  AND (status = 'pending' OR payment_method = 'cash')
		  AND $1 IN (SELECT therapist_id FROM therapist_profiles WHERE accept_assignments = TRUE)
	`, therapistID, now, now, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		// Determine a clearer reason: check booking row
		var currentTherapist *int64
		var status string
		var paymentMethod *string
		if err := r.db.QueryRow(ctx, `SELECT therapist_id, status, payment_method FROM bookings WHERE booking_id = $1`, bookingID).Scan(&currentTherapist, &status, &paymentMethod); err != nil {
			if err == pgx.ErrNoRows {
				return pgx.ErrNoRows
			}
			return err
		}
		if currentTherapist != nil {
			return ErrAlreadyAssigned
		}
		if !(status == "pending" || (paymentMethod != nil && *paymentMethod == "cash")) {
			return ErrBookingNotAssignable
		}
		return ErrAssignConflict
	}
	// Record event (actor is therapist)
	actor := therapistID
	_ = r.insertBookingEvent(ctx, bookingID, "assigned", &actor, nil)
	return nil
}

// AssignTherapistWithActor behaves like AssignTherapist but records the provided
// actorID (for example an admin) as the actor for the 'assigned' event.
func (r *bookingRepoImpl) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	// Pre-check therapist
	var accept bool
	if err := r.db.QueryRow(ctx, `SELECT accept_assignments FROM therapist_profiles WHERE therapist_id = $1`, therapistID).Scan(&accept); err != nil {
		if err == pgx.ErrNoRows {
			return ErrTherapistNotFound
		}
		return err
	}
	if !accept {
		return ErrTherapistNotAccepting
	}

	now := time.Now()
	cmd, err := r.db.Exec(ctx, `
			UPDATE bookings
			SET therapist_id = $1, assigned_at = $2, status = 'assigned', updated_at = $3
			WHERE booking_id = $4 AND therapist_id IS NULL
				AND (status = 'pending' OR payment_method = 'cash')
				AND $1 IN (SELECT therapist_id FROM therapist_profiles WHERE accept_assignments = TRUE)
	`, therapistID, now, now, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	// Record event using provided actor (admin or therapist)
	_ = r.insertBookingEvent(ctx, bookingID, "assigned", &actorID, nil)
	return nil
}

// AssignTherapistWithActorTx performs the same guarded update as
// AssignTherapistWithActor but uses the provided transaction. It also
// inserts the booking_events row inside the same transaction so callers can
// create+assign atomically.
func (r *bookingRepoImpl) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	// Pre-check therapist exists and accepts assignments using tx
	var accept bool
	if err := tx.QueryRow(ctx, `SELECT accept_assignments FROM therapist_profiles WHERE therapist_id = $1`, therapistID).Scan(&accept); err != nil {
		if err == pgx.ErrNoRows {
			return ErrTherapistNotFound
		}
		return err
	}
	if !accept {
		return ErrTherapistNotAccepting
	}

	now := time.Now()
	cmd, err := tx.Exec(ctx, `
		UPDATE bookings
		SET therapist_id = $1, assigned_at = $2, status = 'assigned', updated_at = $3
		WHERE booking_id = $4 AND therapist_id IS NULL
		  AND (status = 'pending' OR payment_method = 'cash')
		  AND $1 IN (SELECT therapist_id FROM therapist_profiles WHERE accept_assignments = TRUE)
	`, therapistID, now, now, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		var currentTherapist *int64
		var status string
		var paymentMethod *string
		if err := tx.QueryRow(ctx, `SELECT therapist_id, status, payment_method FROM bookings WHERE booking_id = $1`, bookingID).Scan(&currentTherapist, &status, &paymentMethod); err != nil {
			if err == pgx.ErrNoRows {
				return pgx.ErrNoRows
			}
			return err
		}
		if currentTherapist != nil {
			return ErrAlreadyAssigned
		}
		if !(status == "pending" || (paymentMethod != nil && *paymentMethod == "cash")) {
			return ErrBookingNotAssignable
		}
		return ErrAssignConflict
	}
	// Insert event using provided actor within same transaction
	_, err = tx.Exec(ctx, `
		INSERT INTO booking_events (booking_id, event_type, actor_id, metadata)
		VALUES ($1, 'assigned', $2, NULL)
	`, bookingID, actorID)
	if err != nil {
		return err
	}
	return nil
}

func (r *bookingRepoImpl) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	query := `
		SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			   payment_method,
			   gender_preference, pressure_preference, notes, duration_minutes,
			   scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			   raw_total, discount, final_total, status,
			   created_at, updated_at
		FROM bookings
		WHERE booking_id = $1
	`

	var b model.Booking
	if err := r.db.QueryRow(ctx, query, bookingID).Scan(
		&b.BookingID,
		&b.ReferenceCode,
		&b.ClientID,
		&b.TherapistID,
		&b.AssignedAt,
		&b.ServiceID,
		&b.AddressID,
		&b.PromoID,
		&b.PaymentMethod,
		&b.GenderPref,
		&b.PressurePref,
		&b.Notes,
		&b.DurationMinutes,
		&b.ScheduledStart,
		&b.ActualStart,
		&b.ActualEnd,
		&b.TherapistArrivedAt,
		&b.NoShowAt,
		&b.CancelledBy,
		&b.CancelledAt,
		&b.CancellationReason,
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

func (r *bookingRepoImpl) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT event_id, booking_id, event_type, actor_id, metadata, created_at
		FROM booking_events
		WHERE booking_id = $1
		ORDER BY created_at ASC
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.BookingEvent
	for rows.Next() {
		var ev model.BookingEvent
		var metadata interface{}
		if err := rows.Scan(&ev.EventID, &ev.BookingID, &ev.EventType, &ev.ActorID, &metadata, &ev.CreatedAt); err != nil {
			return nil, err
		}
		// Attempt to convert metadata to map[string]any if present
		if metadata != nil {
			if m, ok := metadata.(map[string]any); ok {
				ev.Metadata = m
			}
		}
		out = append(out, ev)
	}
	return out, nil
}

func (r *bookingRepoImpl) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	var md interface{}
	if metadata != nil {
		md = metadata
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO booking_events (booking_id, event_type, actor_id, metadata)
		VALUES ($1, $2, $3, $4)
	`, bookingID, eventType, actorID, md)
	return err
}

func (r *bookingRepoImpl) UpdateStatus(ctx context.Context, bookingID, userID int64, status string, cancelledBy *string, cancellationReason *string) error {
	// Allow the update if the actor is either the client or the assigned therapist
	now := time.Now()
	// Note: pass cancelledBy and cancellationReason as $5 and $6 so types align with SQL placeholders
	cmd, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET status = $1::text,
			therapist_arrived_at = CASE WHEN $1::text = 'arrived' THEN $2 ELSE therapist_arrived_at END,
			actual_start = CASE WHEN $1::text = 'in_progress' THEN $2 ELSE actual_start END,
			actual_end = CASE WHEN $1::text = 'completed' THEN $2 ELSE actual_end END,
			cancelled_by = CASE WHEN $1::text = 'cancelled' THEN $5::text ELSE cancelled_by END,
			cancelled_at = CASE WHEN $1::text = 'cancelled' THEN $2 ELSE cancelled_at END,
			cancellation_reason = CASE WHEN $1::text = 'cancelled' THEN $6::text ELSE cancellation_reason END,
			updated_at = $2
		WHERE booking_id = $3 AND (client_id = $4 OR therapist_id = $4)
	`, status, now, bookingID, userID, cancelledBy, cancellationReason)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	// Record event for timeline/audit
	// actor is the acting user
	actor := userID
	_ = r.insertBookingEvent(ctx, bookingID, status, &actor, nil)
	return nil
}

// GetRecentTherapistStruggleFlags checks bookings in the given time window and
// flags therapists who had cancellations/no-shows OR have low booking volume. Returns a map[therapist_id]bool.
func (r *bookingRepoImpl) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	if len(therapistIDs) == 0 {
		return map[int64]bool{}, nil
	}

	// We want to flag therapists who have cancellations/no-shows OR have low booking volume (e.g. 0 completed/active bookings)
	// We'll fetch counts for bad statuses and good statuses.
	// Note: "less booking" interpreted as 0 successful bookings in the period.
		query := `
				SELECT therapist_id,
							 COUNT(*) FILTER (WHERE status IN ('cancelled_by_therapist', 'no_show')) as bad_cnt,
							 COUNT(*) FILTER (WHERE status IN ('completed', 'assigned', 'on_the_way', 'arrived', 'in_progress')) as good_cnt
				FROM bookings
				WHERE therapist_id = ANY($1)
					AND scheduled_start >= $2
				GROUP BY therapist_id
		`

		// Convert []int64 to []int32 so pgx sends an integer[] that matches the DB column type (INT)
		intIDs := make([]int32, 0, len(therapistIDs))
		for _, id := range therapistIDs {
				intIDs = append(intIDs, int32(id))
		}

		rows, err := r.db.Query(ctx, query, intIDs, since)
		if err != nil {
			log.Printf("GetRecentTherapistStruggleFlags failed: therapistIDs=%v since=%v err=%v", therapistIDs, since, err)
			return nil, err
		}
	defer rows.Close()

	stats := make(map[int64]struct{ bad, good int })
	for rows.Next() {
		var tid int64
		var bad, good int
		if err := rows.Scan(&tid, &bad, &good); err != nil {
			return nil, err
		}
		stats[tid] = struct{ bad, good int }{bad, good}
	}

	out := make(map[int64]bool)
	for _, tid := range therapistIDs {
		s, found := stats[tid]
		if !found {
			// No bookings at all -> struggling (low volume)
			out[tid] = true
		} else {
			// Has bookings. Check if bad > 0 OR good == 0
			if s.bad > 0 || s.good == 0 {
				out[tid] = true
			}
		}
	}

	return out, nil
}
