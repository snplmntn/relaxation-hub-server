package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

var (
	ErrTherapistNotFound     = errors.New("therapist not found")
	ErrTherapistNotAccepting = errors.New("therapist not accepting assignments")
	ErrAlreadyAssigned       = errors.New("booking already assigned")
	ErrBookingNotAssignable  = errors.New("booking not in assignable state")
	ErrAssignConflict        = errors.New("assignment conflict")
)

// BookingDetailsResult contains a booking with all related data fetched in a single query
type BookingDetailsResult struct {
	Booking         *model.Booking
	Service         *model.Service
	Address         *model.Address
	ClientName      string
	ClientPhone     string
	ClientPhoto     string
	ClientGender    string
	TherapistName   string
	TherapistPhone  string
	TherapistPhoto  string
	TherapistGender string
	TherapistRating *float64
	PromoCode       string
	Timeline        []model.BookingEvent
}

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
	// client or the assigned therapist (actorID), OR if the role is 'admin'.
	UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error
	// UpdateStatusWithTime is like UpdateStatus but allows specifying a custom time
	// for status-related timestamps (e.g., actual_start from offline sync)
	UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error
	// GetRecentTherapistStruggleFlags returns a map of therapist_id -> true if the
	// therapist had one or more poor outcomes (cancellations/no-shows) since 'since'
	GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error)
	// GetBookingWithDetails fetches a booking with all related data (service, address, client, therapist) in a single optimized query
	// userID is used for access control (must be client or therapist)
	GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*BookingDetailsResult, error)
	// GetBookingWithDetailsUnsafe fetches a booking with all related data without userID access control (for admin/offer cases)
	GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*BookingDetailsResult, error)
	// GetBookingByCodeWithDetails fetches a booking with all related data in a single optimized query by reference code
	GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*BookingDetailsResult, error)
	// GetBookingByCodeWithDetailsUnsafe fetches a booking with all related data without userID access control by reference code
	GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*BookingDetailsResult, error)
	// ListByClientWithDetails fetches bookings with all related data using optimized JOINs
	ListByClientWithDetails(ctx context.Context, clientID int64) ([]BookingDetailsResult, error)
	// ListByTherapistWithDetails fetches bookings with all related data using optimized JOINs
	ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]BookingDetailsResult, error)
	// ListByClientWithDetailsPaginated fetches paginated bookings for a client with total count
	ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]BookingDetailsResult, int, error)
	// ListByTherapistWithDetailsPaginated fetches paginated bookings for a therapist with total count
	ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]BookingDetailsResult, int, error)
	// ListAllWithDetailsPaginated fetches paginated bookings for all users with total count (admin usage)
	ListAllWithDetailsPaginated(ctx context.Context, limit, offset int) ([]BookingDetailsResult, int, error)
	// ListGlobalPending returns all bookings with status='pending' ordered by created_at ASC (oldest first)
	ListGlobalPending(ctx context.Context) ([]model.Booking, error)
	// GetTherapistBookingCounts returns a map of therapist_id -> total completed bookings in the given time window
	GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error)
	// SetPauseStart sets the current_pause_start field for a booking (for pause functionality)
	SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error
	// ClearPauseAndAddDuration clears current_pause_start and sets total_paused_seconds
	ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error
	// ListInProgressBookings returns all bookings with status='in_progress' and actual_start set
	ListInProgressBookings(ctx context.Context) ([]model.Booking, error)
	// ListUpcomingBookingsForReminder fetches assigned bookings with scheduled_start in [start, end)
	// that do NOT have a booking_events row with eventTypeExclude for idempotency.
	ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error)
	// UnassignTherapist clears the therapist_id and resets status to pending for reassignment
	UnassignTherapist(ctx context.Context, bookingID int64) error
	// GetClientBookingStats returns completed count and late cancellation count for a client
	GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*ClientBookingStats, error)
	// CountEventsByTypeAndActor counts booking events with a specific event_type where actor_id matches since a given time
	CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error)
	// GetAccountingSummary returns aggregated revenue/payout data for completed bookings in a date range
	GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*AccountingSummary, error)
	// GetDailyAccounting returns daily breakdown of revenue/payout data for completed bookings
	GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]DailyAccountingEntry, error)
	// CompleteBooking updates status to completed and sets commission fields
	CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error
	// CompleteBookingWithLedgerTx atomically updates status, commission fields, and inserts ledger entries in one transaction
	CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error
}

// ClientBookingStats holds statistics for banning policy evaluation
type ClientBookingStats struct {
	TotalCompleted         int
	TotalLateCancellations int
}

// AccountingSummary holds aggregated accounting data
type AccountingSummary struct {
	TotalRevenue          float64
	TotalTherapistPayouts float64
	TotalPlatformProfit   float64
	BookingCount          int
}

// DailyAccountingEntry holds daily accounting data
type DailyAccountingEntry struct {
	Date             time.Time
	Revenue          float64
	TherapistPayouts float64
	PlatformProfit   float64
	BookingCount     int
}

type bookingRepoImpl struct {
	db db.DBTX
}

func NewBookingRepository(db db.DBTX) BookingRepository {
	return &bookingRepoImpl{db: db}
}

func (r *bookingRepoImpl) Create(ctx context.Context, booking *model.Booking) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		 SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			 payment_method,
			 gender_preference, pressure_preference, notes, duration_minutes,
			 scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			 raw_total, discount, final_total, status,
			 created_at, updated_at, total_paused_seconds, current_pause_start, extension_wait_seconds
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
		&b.TotalPausedSeconds,
		&b.CurrentPauseStart,
		&b.ExtensionWaitSeconds,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	return &b, nil
}

func (r *bookingRepoImpl) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		 SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			 payment_method,
			 gender_preference, pressure_preference, notes, duration_minutes,
			 scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			 raw_total, discount, final_total, status,
			 created_at, updated_at, total_paused_seconds, current_pause_start, extension_wait_seconds
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
			&b.TotalPausedSeconds,
			&b.CurrentPauseStart,
			&b.ExtensionWaitSeconds,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}

	return out, nil
}

func (r *bookingRepoImpl) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		 SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			 payment_method,
			 gender_preference, pressure_preference, notes, duration_minutes,
			 scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			 raw_total, discount, final_total, status,
			 created_at, updated_at, total_paused_seconds, current_pause_start, extension_wait_seconds
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
			&b.TotalPausedSeconds,
			&b.CurrentPauseStart,
			&b.ExtensionWaitSeconds,
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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	// Note: insertBookingEvent is typically a helper called within other methods that already manage context/timeout.
	// However, if called independently, ensure context has timeout.
	// Since it's unexported, we'll rely on caller's context manipulation or add it if context is Background.
	// For safety, let's wrap it anyway if we want strict enforcement, but beware double-wrapping.
	// Given it's a small insert, existing context cancellation is likely sufficient if caller set it.
	// Let's Skip explicit timeout here to avoid overriding caller's potentially longer timeout (e.g. transaction).

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	// Use transaction context, so assume timeout is handled by transaction wrapper if applicable, or caller.
	// However, we can ensure this specific operation doesn't hang indefinitely if the tx allows.
	// But usually, the tx context is bound to the connection lifetime.
	// Let's NOT wrap Tx methods to avoid cutting off the transaction prematurely if the caller intended a longer flow.

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			   payment_method,
			   gender_preference, pressure_preference, notes, duration_minutes,
			   scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			   raw_total, discount, final_total, status,
			   created_at, updated_at, total_paused_seconds, current_pause_start, extension_wait_seconds
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
		&b.TotalPausedSeconds,
		&b.CurrentPauseStart,
		&b.ExtensionWaitSeconds,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	// If paused, fetch the role who paused it
	if b.CurrentPauseStart != nil {
		var role string
		// Fetch metadata from last session_paused event
		err := r.db.QueryRow(ctx, `
			SELECT metadata->>'paused_by_role' 
			FROM booking_events 
			WHERE booking_id = $1 AND event_type = 'session_paused' 
			ORDER BY created_at DESC LIMIT 1
		`, bookingID).Scan(&role)
		// Ignore errors (e.g. if metadata is null or event not found)
		if err == nil && role != "" {
			b.PausedByRole = &role
		}
	}

	return &b, nil
}

func (r *bookingRepoImpl) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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

func (r *bookingRepoImpl) UpdateStatus(ctx context.Context, bookingID, userID int64, role, status string, cancelledBy *string, cancellationReason *string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	// Allow the update if the actor is either the client or the assigned therapist, OR if role is admin
	now := time.Now()
	// Note: pass cancelledBy and cancellationReason as $5 and $6
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
		WHERE booking_id = $3 AND ($7::text = 'admin' OR client_id = $4 OR therapist_id = $4)
	`, status, now, bookingID, userID, cancelledBy, cancellationReason, role)
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

// UpdateStatusWithTime updates the booking status with an optional custom time
// for status-related timestamps. If customTime is nil, uses time.Now().
func (r *bookingRepoImpl) UpdateStatusWithTime(ctx context.Context, bookingID, userID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	ts := time.Now()
	if customTime != nil {
		ts = *customTime
	}

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
		WHERE booking_id = $3 AND ($7::text = 'admin' OR client_id = $4 OR therapist_id = $4)
	`, status, ts, bookingID, userID, cancelledBy, cancellationReason, role)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	actor := userID
	_ = r.insertBookingEvent(ctx, bookingID, status, &actor, nil)
	return nil
}

// GetRecentTherapistStruggleFlags checks bookings in the given time window and
// flags therapists who had cancellations/no-shows OR have low booking volume. Returns a map[therapist_id]bool.
func (r *bookingRepoImpl) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) {
	// Use LongQueryTimeout (30s) for analytic/aggregation queries to avoid false positives
	ctx, cancel := db.WithLongQueryTimeout(ctx)
	defer cancel()

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

// GetBookingWithDetails fetches a booking with all related data in a single optimized query using JOINs
// userID is used for access control (must be client or therapist)
func (r *bookingRepoImpl) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*BookingDetailsResult, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), s.base_price, 
			s.duration_minutes, COALESCE(s.category, ''), s.is_active, 
			COALESCE(s.preview_image_url, ''), s.deleted_at, s.created_at,
			-- Address fields (LEFT JOIN)
			a.address_id, a.user_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), 
			COALESCE(a.city, ''), COALESCE(a.province, ''), COALESCE(a.postal_code, ''), 
			COALESCE(a.country, 'Philippines'), a.latitude, a.longitude, 
			a.is_default, a.created_at, a.updated_at,
			-- Client info (LEFT JOIN users)
			COALESCE(client_u.full_name, ''), 
			COALESCE(client_u.primary_phone, ''), 
			COALESCE(client_u.profile_photo, ''),
			COALESCE(client_u.gender, ''),
			-- Therapist info (LEFT JOIN users + therapist_profiles)
			COALESCE(therapist_u.full_name, ''),
			COALESCE(therapist_u.primary_phone, ''),
			COALESCE(therapist_u.profile_photo, ''),
			COALESCE(therapist_u.gender, ''),
			tp.avg_rating,
			-- Promo Code
			COALESCE(p.code, '')
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.service_id AND s.deleted_at IS NULL
		LEFT JOIN addresses a ON b.address_id = a.address_id AND a.deleted_at IS NULL
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id AND tp.deleted_at IS NULL
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL
		WHERE b.booking_id = $1 AND (b.client_id = $2 OR b.therapist_id = $2)
	`

	var result BookingDetailsResult
	var booking model.Booking
	var serviceID *int64

	// Service temp vars
	var sServiceID *int64
	var sName, sDesc, sCat, sImg string
	var sBasePrice *float64
	var sDuration *int
	var sIsActive *bool
	var sDeletedAt, sCreatedAt *time.Time

	// Address temp vars
	var aLabel, aStreet, aCity, aProv, aZip, aCountry string
	var aAddrID, aUserID *int64
	var aLat, aLon *float64
	var aIsDefault *bool
	var aCreatedAt, aUpdatedAt *time.Time

	var therapistRating *float64

	err := r.db.QueryRow(ctx, query, bookingID, userID).Scan(
		// Booking fields
		&booking.BookingID, &booking.ReferenceCode, &booking.ClientID, &booking.TherapistID, &booking.AssignedAt,
		&serviceID, &booking.AddressID, &booking.PromoID, &booking.PaymentMethod,
		&booking.GenderPref, &booking.PressurePref, &booking.Notes, &booking.DurationMinutes,
		&booking.ScheduledStart, &booking.ActualStart, &booking.ActualEnd, &booking.TherapistArrivedAt,
		&booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason,
		&booking.RawTotal, &booking.Discount, &booking.FinalTotal, &booking.Status,
		&booking.CreatedAt, &booking.UpdatedAt, &booking.TotalPausedSeconds, &booking.CurrentPauseStart, &booking.ExtensionWaitSeconds,
		&booking.IsRated,
		// Service fields
		&sServiceID, &sName, &sDesc, &sBasePrice,
		&sDuration, &sCat, &sIsActive,
		&sImg, &sDeletedAt, &sCreatedAt,
		// Address fields
		&aAddrID, &aUserID, &aLabel, &aStreet,
		&aCity, &aProv, &aZip,
		&aCountry, &aLat, &aLon,
		&aIsDefault, &aCreatedAt, &aUpdatedAt,
		// Client info
		&result.ClientName, &result.ClientPhone, &result.ClientPhoto, &result.ClientGender,
		// Therapist info
		&result.TherapistName, &result.TherapistPhone, &result.TherapistPhoto, &result.TherapistGender, &therapistRating,
		// Promo Code
		&result.PromoCode,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	// Set ServiceID on booking
	booking.ServiceID = serviceID
	result.Booking = &booking

	// Only include service if it was found
	if sServiceID != nil && *sServiceID != 0 {
		result.Service = &model.Service{
			ServiceID:       *sServiceID,
			Name:            sName,
			Description:     sDesc,
			BasePrice:       getPooledFloat(sBasePrice),
			DurationMinutes: getPooledInt(sDuration),
			Category:        sCat,
			IsActive:        getPooledBool(sIsActive),
			PreviewImageURL: sImg,
			DeletedAt:       sDeletedAt,
			CreatedAt:       getPooledTime(sCreatedAt),
		}
	}

	// Only include address if it was found
	if aAddrID != nil {
		result.Address = &model.Address{
			AddressID:     *aAddrID,
			UserID:        *aUserID,
			Label:         aLabel,
			Street:        aStreet,
			City:          aCity,
			Province:      aProv,
			PostalCode:    aZip,
			Country:       aCountry,
			Latitude:      aLat,
			Longitude:     aLon,
			IsDefault:     getPooledBool(aIsDefault),
			CreatedAt:     getPooledTime(aCreatedAt),
			UpdatedAt:     getPooledTime(aUpdatedAt),
		}
	}

	// Only include therapist rating if found
	if therapistRating != nil {
		result.TherapistRating = therapistRating
	}

	return &result, nil
}

// GetBookingByCodeWithDetails fetches a booking with all related data in a single optimized query by reference code
func (r *bookingRepoImpl) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*BookingDetailsResult, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), s.base_price, 
			s.duration_minutes, COALESCE(s.category, ''), s.is_active, 
			COALESCE(s.preview_image_url, ''), s.deleted_at, s.created_at,
			-- Address fields (LEFT JOIN)
			a.address_id, a.user_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), 
			COALESCE(a.city, ''), COALESCE(a.province, ''), COALESCE(a.postal_code, ''), 
			COALESCE(a.country, 'Philippines'), a.latitude, a.longitude, 
			a.is_default, a.created_at, a.updated_at,
			-- Client info (LEFT JOIN users)
			COALESCE(client_u.full_name, ''), 
			COALESCE(client_u.primary_phone, ''), 
			COALESCE(client_u.profile_photo, ''),
			COALESCE(client_u.gender, ''),
			-- Therapist info (LEFT JOIN users + therapist_profiles)
			COALESCE(therapist_u.full_name, ''),
			COALESCE(therapist_u.primary_phone, ''),
			COALESCE(therapist_u.profile_photo, ''),
			COALESCE(therapist_u.gender, ''),
			tp.avg_rating,
			-- Promo Code
			COALESCE(p.code, '')
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.service_id AND s.deleted_at IS NULL
		LEFT JOIN addresses a ON b.address_id = a.address_id AND a.deleted_at IS NULL
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id AND tp.deleted_at IS NULL
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL
		WHERE b.reference_code = $1 AND (b.client_id = $2 OR b.therapist_id = $2)
	`

	var result BookingDetailsResult
	var booking model.Booking
	var serviceID *int64

	// Service temp vars
	var sServiceID *int64
	var sName, sDesc, sCat, sImg string
	var sBasePrice *float64
	var sDuration *int
	var sIsActive *bool
	var sDeletedAt, sCreatedAt *time.Time

	// Address temp vars
	var aLabel, aStreet, aCity, aProv, aZip, aCountry string
	var aAddrID, aUserID *int64
	var aLat, aLon *float64
	var aIsDefault *bool
	var aCreatedAt, aUpdatedAt *time.Time

	var therapistRating *float64

	err := r.db.QueryRow(ctx, query, referenceCode, userID).Scan(
		// Booking fields
		&booking.BookingID, &booking.ReferenceCode, &booking.ClientID, &booking.TherapistID, &booking.AssignedAt,
		&serviceID, &booking.AddressID, &booking.PromoID, &booking.PaymentMethod,
		&booking.GenderPref, &booking.PressurePref, &booking.Notes, &booking.DurationMinutes,
		&booking.ScheduledStart, &booking.ActualStart, &booking.ActualEnd, &booking.TherapistArrivedAt,
		&booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason,
		&booking.RawTotal, &booking.Discount, &booking.FinalTotal, &booking.Status,
		&booking.CreatedAt, &booking.UpdatedAt, &booking.TotalPausedSeconds, &booking.CurrentPauseStart, &booking.ExtensionWaitSeconds,
		&booking.IsRated,
		// Service fields
		&sServiceID, &sName, &sDesc, &sBasePrice,
		&sDuration, &sCat, &sIsActive,
		&sImg, &sDeletedAt, &sCreatedAt,
		// Address fields
		&aAddrID, &aUserID, &aLabel, &aStreet,
		&aCity, &aProv, &aZip,
		&aCountry, &aLat, &aLon,
		&aIsDefault, &aCreatedAt, &aUpdatedAt,
		// Client info
		&result.ClientName, &result.ClientPhone, &result.ClientPhoto, &result.ClientGender,
		// Therapist info
		&result.TherapistName, &result.TherapistPhone, &result.TherapistPhoto, &result.TherapistGender, &therapistRating,
		// Promo Code
		&result.PromoCode,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	// Set ServiceID on booking
	booking.ServiceID = serviceID
	result.Booking = &booking

	// Only include service if it was found
	if sServiceID != nil && *sServiceID != 0 {
		result.Service = &model.Service{
			ServiceID:       *sServiceID,
			Name:            sName,
			Description:     sDesc,
			BasePrice:       getPooledFloat(sBasePrice),
			DurationMinutes: getPooledInt(sDuration),
			Category:        sCat,
			IsActive:        getPooledBool(sIsActive),
			PreviewImageURL: sImg,
			DeletedAt:       sDeletedAt,
			CreatedAt:       getPooledTime(sCreatedAt),
		}
	}

	// Only include address if it was found
	if aAddrID != nil {
		result.Address = &model.Address{
			AddressID:     *aAddrID,
			UserID:        *aUserID,
			Label:         aLabel,
			Street:        aStreet,
			City:          aCity,
			Province:      aProv,
			PostalCode:    aZip,
			Country:       aCountry,
			Latitude:      aLat,
			Longitude:     aLon,
			IsDefault:     getPooledBool(aIsDefault),
			CreatedAt:     getPooledTime(aCreatedAt),
			UpdatedAt:     getPooledTime(aUpdatedAt),
		}
	}

	// Only include therapist rating if found
	if therapistRating != nil {
		result.TherapistRating = therapistRating
	}

	return &result, nil
}

// GetBookingWithDetailsUnsafe fetches a booking with all related data without userID access control
func (r *bookingRepoImpl) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*BookingDetailsResult, error) {
	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), s.base_price, 
			s.duration_minutes, COALESCE(s.category, ''), s.is_active, 
			COALESCE(s.preview_image_url, ''), s.deleted_at, s.created_at,
			-- Address fields (LEFT JOIN)
			a.address_id, a.user_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), 
			COALESCE(a.city, ''), COALESCE(a.province, ''), COALESCE(a.postal_code, ''), 
			COALESCE(a.country, 'Philippines'), a.latitude, a.longitude, 
			a.is_default, a.created_at, a.updated_at,
			-- Client info (LEFT JOIN users)
			COALESCE(client_u.full_name, ''), 
			COALESCE(client_u.primary_phone, ''), 
			COALESCE(client_u.profile_photo, ''),
			COALESCE(client_u.gender, ''),
			-- Therapist info (LEFT JOIN users + therapist_profiles)
			COALESCE(therapist_u.full_name, ''),
			COALESCE(therapist_u.primary_phone, ''),
			COALESCE(therapist_u.profile_photo, ''),
			COALESCE(therapist_u.gender, ''),
			tp.avg_rating,
			-- Promo Code
			COALESCE(p.code, '')
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.service_id AND s.deleted_at IS NULL
		LEFT JOIN addresses a ON b.address_id = a.address_id AND a.deleted_at IS NULL
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id AND tp.deleted_at IS NULL
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL
		WHERE b.booking_id = $1
	`

	var result BookingDetailsResult
	var booking model.Booking
	var serviceID *int64

	// Service temp vars
	var sServiceID *int64
	var sName, sDesc, sCat, sImg string
	var sBasePrice *float64
	var sDuration *int
	var sIsActive *bool
	var sDeletedAt, sCreatedAt *time.Time

	// Address temp vars
	var aLabel, aStreet, aCity, aProv, aZip, aCountry string
	var aAddrID, aUserID *int64
	var aLat, aLon *float64
	var aIsDefault *bool
	var aCreatedAt, aUpdatedAt *time.Time

	var therapistRating *float64

	err := r.db.QueryRow(ctx, query, bookingID).Scan(
		// Booking fields
		&booking.BookingID, &booking.ReferenceCode, &booking.ClientID, &booking.TherapistID, &booking.AssignedAt,
		&serviceID, &booking.AddressID, &booking.PromoID, &booking.PaymentMethod,
		&booking.GenderPref, &booking.PressurePref, &booking.Notes, &booking.DurationMinutes,
		&booking.ScheduledStart, &booking.ActualStart, &booking.ActualEnd, &booking.TherapistArrivedAt,
		&booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason,
		&booking.RawTotal, &booking.Discount, &booking.FinalTotal, &booking.Status,
		&booking.CreatedAt, &booking.UpdatedAt, &booking.TotalPausedSeconds, &booking.CurrentPauseStart, &booking.ExtensionWaitSeconds,
		&booking.IsRated,
		// Service fields
		&sServiceID, &sName, &sDesc, &sBasePrice,
		&sDuration, &sCat, &sIsActive,
		&sImg, &sDeletedAt, &sCreatedAt,
		// Address fields
		&aAddrID, &aUserID, &aLabel, &aStreet,
		&aCity, &aProv, &aZip,
		&aCountry, &aLat, &aLon,
		&aIsDefault, &aCreatedAt, &aUpdatedAt,
		// Client info
		&result.ClientName, &result.ClientPhone, &result.ClientPhoto, &result.ClientGender,
		// Therapist info
		&result.TherapistName, &result.TherapistPhone, &result.TherapistPhoto, &result.TherapistGender, &therapistRating,
		// Promo Code
		&result.PromoCode,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	// Set ServiceID on booking
	booking.ServiceID = serviceID
	result.Booking = &booking

	// Only include service if it was found
	if sServiceID != nil && *sServiceID != 0 {
		result.Service = &model.Service{
			ServiceID:       *sServiceID,
			Name:            sName,
			Description:     sDesc,
			BasePrice:       getPooledFloat(sBasePrice),
			DurationMinutes: getPooledInt(sDuration),
			Category:        sCat,
			IsActive:        getPooledBool(sIsActive),
			PreviewImageURL: sImg,
			DeletedAt:       sDeletedAt,
			CreatedAt:       getPooledTime(sCreatedAt),
		}
	}

	// Only include address if it was found
	if aAddrID != nil {
		result.Address = &model.Address{
			AddressID:     *aAddrID,
			UserID:        *aUserID,
			Label:         aLabel,
			Street:        aStreet,
			City:          aCity,
			Province:      aProv,
			PostalCode:    aZip,
			Country:       aCountry,
			Latitude:      aLat,
			Longitude:     aLon,
			IsDefault:     getPooledBool(aIsDefault),
			CreatedAt:     getPooledTime(aCreatedAt),
			UpdatedAt:     getPooledTime(aUpdatedAt),
		}
	}

	// Only include therapist rating if found
	if therapistRating != nil {
		result.TherapistRating = therapistRating
	}

	return &result, nil
}

// GetBookingByCodeWithDetailsUnsafe fetches a booking with all related data without userID access control by reference code
func (r *bookingRepoImpl) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*BookingDetailsResult, error) {
	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), s.base_price, 
			s.duration_minutes, COALESCE(s.category, ''), s.is_active, 
			COALESCE(s.preview_image_url, ''), s.deleted_at, s.created_at,
			-- Address fields (LEFT JOIN)
			a.address_id, a.user_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), 
			COALESCE(a.city, ''), COALESCE(a.province, ''), COALESCE(a.postal_code, ''), 
			COALESCE(a.country, 'Philippines'), a.latitude, a.longitude, 
			a.is_default, a.created_at, a.updated_at,
			-- Client info (LEFT JOIN users)
			COALESCE(client_u.full_name, ''), 
			COALESCE(client_u.primary_phone, ''), 
			COALESCE(client_u.profile_photo, ''),
			COALESCE(client_u.gender, ''),
			-- Therapist info (LEFT JOIN users + therapist_profiles)
			COALESCE(therapist_u.full_name, ''),
			COALESCE(therapist_u.primary_phone, ''),
			COALESCE(therapist_u.profile_photo, ''),
			COALESCE(therapist_u.gender, ''),
			tp.avg_rating,
			-- Promo Code
			COALESCE(p.code, '')
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.service_id AND s.deleted_at IS NULL
		LEFT JOIN addresses a ON b.address_id = a.address_id AND a.deleted_at IS NULL
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id AND tp.deleted_at IS NULL
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL
		WHERE b.reference_code = $1
	`

	var result BookingDetailsResult
	var booking model.Booking
	var serviceID *int64

	// Service temp vars
	var sServiceID *int64
	var sName, sDesc, sCat, sImg string
	var sBasePrice *float64
	var sDuration *int
	var sIsActive *bool
	var sDeletedAt, sCreatedAt *time.Time

	// Address temp vars
	var aLabel, aStreet, aCity, aProv, aZip, aCountry string
	var aAddrID, aUserID *int64
	var aLat, aLon *float64
	var aIsDefault *bool
	var aCreatedAt, aUpdatedAt *time.Time

	var therapistRating *float64

	err := r.db.QueryRow(ctx, query, referenceCode).Scan(
		// Booking fields
		&booking.BookingID, &booking.ReferenceCode, &booking.ClientID, &booking.TherapistID, &booking.AssignedAt,
		&serviceID, &booking.AddressID, &booking.PromoID, &booking.PaymentMethod,
		&booking.GenderPref, &booking.PressurePref, &booking.Notes, &booking.DurationMinutes,
		&booking.ScheduledStart, &booking.ActualStart, &booking.ActualEnd, &booking.TherapistArrivedAt,
		&booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason,
		&booking.RawTotal, &booking.Discount, &booking.FinalTotal, &booking.Status,
		&booking.CreatedAt, &booking.UpdatedAt, &booking.TotalPausedSeconds, &booking.CurrentPauseStart, &booking.ExtensionWaitSeconds,
		&booking.IsRated,
		// Service fields
		&sServiceID, &sName, &sDesc, &sBasePrice,
		&sDuration, &sCat, &sIsActive,
		&sImg, &sDeletedAt, &sCreatedAt,
		// Address fields
		&aAddrID, &aUserID, &aLabel, &aStreet,
		&aCity, &aProv, &aZip,
		&aCountry, &aLat, &aLon,
		&aIsDefault, &aCreatedAt, &aUpdatedAt,
		// Client info
		&result.ClientName, &result.ClientPhone, &result.ClientPhoto, &result.ClientGender,
		// Therapist info
		&result.TherapistName, &result.TherapistPhone, &result.TherapistPhoto, &result.TherapistGender, &therapistRating,
		// Promo Code
		&result.PromoCode,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	// Set ServiceID on booking
	booking.ServiceID = serviceID
	result.Booking = &booking

	// Only include service if it was found
	if sServiceID != nil && *sServiceID != 0 {
		result.Service = &model.Service{
			ServiceID:       *sServiceID,
			Name:            sName,
			Description:     sDesc,
			BasePrice:       getPooledFloat(sBasePrice),
			DurationMinutes: getPooledInt(sDuration),
			Category:        sCat,
			IsActive:        getPooledBool(sIsActive),
			PreviewImageURL: sImg,
			DeletedAt:       sDeletedAt,
			CreatedAt:       getPooledTime(sCreatedAt),
		}
	}

	// Only include address if it was found
	if aAddrID != nil {
		result.Address = &model.Address{
			AddressID:     *aAddrID,
			UserID:        *aUserID,
			Label:         aLabel,
			Street:        aStreet,
			City:          aCity,
			Province:      aProv,
			PostalCode:    aZip,
			Country:       aCountry,
			Latitude:      aLat,
			Longitude:     aLon,
			IsDefault:     getPooledBool(aIsDefault),
			CreatedAt:     getPooledTime(aCreatedAt),
			UpdatedAt:     getPooledTime(aUpdatedAt),
		}
	}

	// Only include therapist rating if found
	if therapistRating != nil {
		result.TherapistRating = therapistRating
	}

	return &result, nil
}

// ListByClientWithDetails fetches bookings with all related data using optimized JOINs
func (r *bookingRepoImpl) ListByClientWithDetails(ctx context.Context, clientID int64) ([]BookingDetailsResult, error) {
	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), COALESCE(s.base_price, 0.0), 
			COALESCE(s.duration_minutes, 0), COALESCE(s.category, ''), COALESCE(s.is_active, true), 
			COALESCE(s.preview_image_url, ''),
			-- Address fields (LEFT JOIN)
			a.address_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), 
			COALESCE(a.city, ''), COALESCE(a.province, ''), COALESCE(a.postal_code, ''), 
			COALESCE(a.country, 'Philippines'), a.latitude, a.longitude, COALESCE(a.is_default, false),
			-- Client info
			COALESCE(client_u.full_name, ''), COALESCE(client_u.primary_phone, ''), 
			COALESCE(client_u.profile_photo, ''), COALESCE(client_u.gender, ''),
			-- Therapist info
			COALESCE(therapist_u.full_name, ''), COALESCE(therapist_u.primary_phone, ''),
			COALESCE(therapist_u.profile_photo, ''), COALESCE(therapist_u.gender, ''),
			tp.avg_rating,
			-- Promo Code
			COALESCE(p.code, '')
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.service_id AND s.deleted_at IS NULL
		LEFT JOIN addresses a ON b.address_id = a.address_id AND a.deleted_at IS NULL
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id AND tp.deleted_at IS NULL
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL
		WHERE b.client_id = $1
		ORDER BY b.created_at DESC
	`
	return r.scanBookingDetailsList(ctx, query, clientID)
}

// ListByTherapistWithDetails fetches bookings with all related data using optimized JOINs
func (r *bookingRepoImpl) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]BookingDetailsResult, error) {
	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), COALESCE(s.base_price, 0.0), 
			COALESCE(s.duration_minutes, 0), COALESCE(s.category, ''), COALESCE(s.is_active, true), 
			COALESCE(s.preview_image_url, ''),
			-- Address fields (LEFT JOIN)
			a.address_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), 
			COALESCE(a.city, ''), COALESCE(a.province, ''), COALESCE(a.postal_code, ''), 
			COALESCE(a.country, 'Philippines'), a.latitude, a.longitude, COALESCE(a.is_default, false),
			-- Client info
			COALESCE(client_u.full_name, ''), COALESCE(client_u.primary_phone, ''), 
			COALESCE(client_u.profile_photo, ''), COALESCE(client_u.gender, ''),
			-- Therapist info
			COALESCE(therapist_u.full_name, ''), COALESCE(therapist_u.primary_phone, ''),
			COALESCE(therapist_u.profile_photo, ''), COALESCE(therapist_u.gender, ''),
			tp.avg_rating,
			-- Promo Code
			COALESCE(p.code, '')
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.service_id AND s.deleted_at IS NULL
		LEFT JOIN addresses a ON b.address_id = a.address_id AND a.deleted_at IS NULL
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id AND tp.deleted_at IS NULL
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL
		WHERE b.therapist_id = $1
		ORDER BY b.created_at DESC
	`
	return r.scanBookingDetailsList(ctx, query, therapistID)
}

// ListByClientWithDetailsPaginated fetches paginated bookings for a client with total count
func (r *bookingRepoImpl) ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]BookingDetailsResult, int, error) {
	// Get total count first
	countQuery := `SELECT COUNT(*) FROM bookings WHERE client_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQuery, clientID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), COALESCE(s.base_price, 0.0), 
			COALESCE(s.duration_minutes, 0), COALESCE(s.category, ''), COALESCE(s.is_active, true), 
			COALESCE(s.preview_image_url, ''),
			-- Address fields (LEFT JOIN)
			a.address_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), 
			COALESCE(a.city, ''), COALESCE(a.province, ''), COALESCE(a.postal_code, ''), 
			COALESCE(a.country, 'Philippines'), a.latitude, a.longitude, COALESCE(a.is_default, false),
			-- Client info
			COALESCE(client_u.full_name, ''), COALESCE(client_u.primary_phone, ''), 
			COALESCE(client_u.profile_photo, ''), COALESCE(client_u.gender, ''),
			-- Therapist info
			COALESCE(therapist_u.full_name, ''), COALESCE(therapist_u.primary_phone, ''),
			COALESCE(therapist_u.profile_photo, ''), COALESCE(therapist_u.gender, ''),
			tp.avg_rating,
			-- Promo Code
			COALESCE(p.code, '')
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.service_id AND s.deleted_at IS NULL
		LEFT JOIN addresses a ON b.address_id = a.address_id AND a.deleted_at IS NULL
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id AND tp.deleted_at IS NULL
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL
		WHERE b.client_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3
	`
	results, err := r.scanBookingDetailsListWithPagination(ctx, query, clientID, limit, offset)
	return results, total, err
}

// ListByTherapistWithDetailsPaginated fetches paginated bookings for a therapist with total count
func (r *bookingRepoImpl) ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]BookingDetailsResult, int, error) {
	// For therapist, we only count their bookings
	countQuery := `SELECT COUNT(*) FROM bookings WHERE therapist_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQuery, therapistID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), COALESCE(s.base_price, 0.0), 
			COALESCE(s.duration_minutes, 0), COALESCE(s.category, ''), COALESCE(s.is_active, true), 
			COALESCE(s.preview_image_url, ''),
			-- Address fields (LEFT JOIN)
			a.address_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), 
			COALESCE(a.city, ''), COALESCE(a.province, ''), COALESCE(a.postal_code, ''), 
			COALESCE(a.country, 'Philippines'), a.latitude, a.longitude, COALESCE(a.is_default, false),
			-- Client info
			COALESCE(client_u.full_name, ''), COALESCE(client_u.primary_phone, ''), 
			COALESCE(client_u.profile_photo, ''), COALESCE(client_u.gender, ''),
			-- Therapist info
			COALESCE(therapist_u.full_name, ''), COALESCE(therapist_u.primary_phone, ''),
			COALESCE(therapist_u.profile_photo, ''), COALESCE(therapist_u.gender, ''),
			tp.avg_rating,
			-- Promo Code
			COALESCE(p.code, '')
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.service_id AND s.deleted_at IS NULL
		LEFT JOIN addresses a ON b.address_id = a.address_id AND a.deleted_at IS NULL
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id AND tp.deleted_at IS NULL
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL
		WHERE b.therapist_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, therapistID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	results, err := r.scanBookingDetailsRows(rows)
	if err != nil {
		return nil, 0, err
	}

	// Batch fetch timeline events
	if len(results) > 0 {
		bookingIDs := make([]int64, 0, len(results))
		for _, res := range results {
			if res.Booking != nil {
				bookingIDs = append(bookingIDs, res.Booking.BookingID)
			}
		}

		eventQuery := `
			SELECT booking_id, event_type, actor_id, metadata, created_at
			FROM booking_events
			WHERE booking_id = ANY($1)
			ORDER BY created_at ASC
		`
		rows, err := r.db.Query(ctx, eventQuery, bookingIDs)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to fetch timeline events: %w", err)
		}
		defer rows.Close()

		eventsMap := make(map[int64][]model.BookingEvent)
		for rows.Next() {
			var evt model.BookingEvent
			var actorID *int64
			err := rows.Scan(&evt.BookingID, &evt.EventType, &actorID, &evt.Metadata, &evt.CreatedAt)
			if err != nil {
				continue 
			}
			evt.ActorID = actorID
			eventsMap[evt.BookingID] = append(eventsMap[evt.BookingID], evt)
		}

		for i := range results {
			if results[i].Booking != nil {
				results[i].Timeline = eventsMap[results[i].Booking.BookingID]
			}
		}
	}

	return results, total, nil
}

func (r *bookingRepoImpl) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int) ([]BookingDetailsResult, int, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	countQuery := `SELECT COUNT(*) FROM bookings`
	var total int
	if err := r.db.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), COALESCE(s.base_price, 0.0), 
			COALESCE(s.duration_minutes, 0), COALESCE(s.category, ''), COALESCE(s.is_active, true), 
			COALESCE(s.preview_image_url, ''),
			-- Address fields (LEFT JOIN)
			a.address_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), 
			COALESCE(a.city, ''), COALESCE(a.province, ''), COALESCE(a.postal_code, ''), 
			COALESCE(a.country, 'Philippines'), a.latitude, a.longitude, COALESCE(a.is_default, false),
			-- Client info
			COALESCE(client_u.full_name, ''), COALESCE(client_u.primary_phone, ''), 
			COALESCE(client_u.profile_photo, ''), COALESCE(client_u.gender, ''),
			-- Therapist info
			COALESCE(therapist_u.full_name, ''), COALESCE(therapist_u.primary_phone, ''),
			COALESCE(therapist_u.profile_photo, ''), COALESCE(therapist_u.gender, ''),
			tp.avg_rating,
			-- Promo Code
			COALESCE(p.code, '')
		FROM bookings b
		LEFT JOIN services s ON b.service_id = s.service_id AND s.deleted_at IS NULL
		LEFT JOIN addresses a ON b.address_id = a.address_id AND a.deleted_at IS NULL
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id AND tp.deleted_at IS NULL
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL
		ORDER BY b.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []BookingDetailsResult
	for rows.Next() {
		var result BookingDetailsResult
		var booking model.Booking
		var service model.Service
		var address model.Address
		var serviceID, addressID *int64
		var therapistRating *float64

		// Service variables
		var sID *int64
		var sName, sDesc, sCat, sImg *string
		var sPrice *float64
		var sDur *int
		var sActive *bool

		// Address variables
		var aID *int64
		var aLabel, aStreet, aCity, aProv, aPostal, aCountry *string
		var aLat, aLng *float64
		var aDef *bool

		// User variables
		var cName, cPhone, cPhoto, cGen, tName, tPhone, tPhoto, tGen *string
		var pCode *string

		err := rows.Scan(
			// Booking fields
			&booking.BookingID, &booking.ReferenceCode, &booking.ClientID, &booking.TherapistID, &booking.AssignedAt,
			&serviceID, &addressID, &booking.PromoID, &booking.PaymentMethod,
			&booking.GenderPref, &booking.PressurePref, &booking.Notes, &booking.DurationMinutes,
			&booking.ScheduledStart, &booking.ActualStart, &booking.ActualEnd, &booking.TherapistArrivedAt,
			&booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason,
			&booking.RawTotal, &booking.Discount, &booking.FinalTotal, &booking.Status,
			&booking.CreatedAt, &booking.UpdatedAt, &booking.TotalPausedSeconds, &booking.CurrentPauseStart, &booking.ExtensionWaitSeconds,
			&booking.IsRated,
			// Service fields
			&sID, &sName, &sDesc, &sPrice, &sDur, &sCat, &sActive, &sImg,
			// Address fields
			&aID, &aLabel, &aStreet, &aCity, &aProv, &aPostal, &aCountry, &aLat, &aLng, &aDef,
			// Client info
			&cName, &cPhone, &cPhoto, &cGen,
			// Therapist info
			&tName, &tPhone, &tPhoto, &tGen,
			&therapistRating,
			// Promo Code
			&pCode,
		)
		if err != nil {
			return nil, 0, err
		}

		booking.ServiceID = serviceID
		booking.AddressID = addressID
		result.Booking = &booking

		// Populate Service
		if sID != nil {
			service.ServiceID = *sID
		}
		if sName != nil { service.Name = *sName }
		if sDesc != nil { service.Description = *sDesc }
		if sPrice != nil { service.BasePrice = *sPrice }
		if sDur != nil { service.DurationMinutes = *sDur }
		if sCat != nil { service.Category = *sCat }
		if sActive != nil { service.IsActive = *sActive } else { service.IsActive = true }
		if sImg != nil { service.PreviewImageURL = *sImg }

		// Populate Address
		if aID != nil {
			address.AddressID = *aID
		}
		if aLabel != nil { address.Label = *aLabel }
		if aStreet != nil { address.Street = *aStreet }
		if aCity != nil { address.City = *aCity }
		if aProv != nil { address.Province = *aProv }
		if aPostal != nil { address.PostalCode = *aPostal }
		if aCountry != nil { address.Country = *aCountry }
		if aLat != nil { address.Latitude = aLat }
		if aLng != nil { address.Longitude = aLng }
		if aDef != nil { address.IsDefault = *aDef }

		// Populate Result Details
		if cName != nil { result.ClientName = *cName }
		if cPhone != nil { result.ClientPhone = *cPhone }
		if cPhoto != nil { result.ClientPhoto = *cPhoto }
		if cGen != nil { result.ClientGender = *cGen }
		if tName != nil { result.TherapistName = *tName }
		if tPhone != nil { result.TherapistPhone = *tPhone }
		if tPhoto != nil { result.TherapistPhoto = *tPhoto }
		if tGen != nil { result.TherapistGender = *tGen }
		if pCode != nil { result.PromoCode = *pCode }

		if serviceID != nil && service.ServiceID != 0 {
			svcCopy := service
			result.Service = &svcCopy
		}
		if addressID != nil && address.AddressID != 0 {
			addrCopy := address
			result.Address = &addrCopy
		}
		if therapistRating != nil {
			result.TherapistRating = therapistRating
		}

		results = append(results, result)
	}

	return results, total, rows.Err()
}

// scanBookingDetailsListWithPagination is a helper for paginated queries with limit/offset
func (r *bookingRepoImpl) scanBookingDetailsListWithPagination(ctx context.Context, query string, id int64, limit, offset int) ([]BookingDetailsResult, error) {
	rows, err := r.db.Query(ctx, query, id, limit, offset)
	if err != nil {
		return nil, err
	}
	return r.scanBookingDetailsRows(rows)
}

func (r *bookingRepoImpl) scanBookingDetailsRows(rows pgx.Rows) ([]BookingDetailsResult, error) {
	defer rows.Close()

	var results []BookingDetailsResult
	for rows.Next() {
		var result BookingDetailsResult
		var booking model.Booking
		var service model.Service
		var address model.Address
		var serviceID, addressID *int64
		var therapistRating *float64


		// Service variables
		var sID *int64
		var sName, sDesc, sCat, sImg *string
		var sPrice *float64
		var sDur *int
		var sActive *bool

		// Address variables
		var aID *int64
		var aLabel, aStreet, aCity, aProv, aPostal, aCountry *string
		var aLat, aLng *float64
		var aDef *bool

		// User variables
		var cName, cPhone, cPhoto, cGen, tName, tPhone, tPhoto, tGen *string
		var pCode *string

		err := rows.Scan(
			// Booking fields
			&booking.BookingID, &booking.ReferenceCode, &booking.ClientID, &booking.TherapistID, &booking.AssignedAt,
			&serviceID, &addressID, &booking.PromoID, &booking.PaymentMethod,
			&booking.GenderPref, &booking.PressurePref, &booking.Notes, &booking.DurationMinutes,
			&booking.ScheduledStart, &booking.ActualStart, &booking.ActualEnd, &booking.TherapistArrivedAt,
			&booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason,
			&booking.RawTotal, &booking.Discount, &booking.FinalTotal, &booking.Status,
			&booking.CreatedAt, &booking.UpdatedAt, &booking.TotalPausedSeconds, &booking.CurrentPauseStart, &booking.ExtensionWaitSeconds,
			&booking.IsRated,
			// Service fields
			&sID, &sName, &sDesc, &sPrice, &sDur, &sCat, &sActive, &sImg,
			// Address fields
			&aID, &aLabel, &aStreet, &aCity, &aProv, &aPostal, &aCountry, &aLat, &aLng, &aDef,
			// Client info
			&cName, &cPhone, &cPhoto, &cGen,
			// Therapist info
			&tName, &tPhone, &tPhoto, &tGen,
			&therapistRating,
			// Promo Code
			&pCode,
		)
		if err != nil {
			return nil, err
		}

		booking.ServiceID = serviceID
		booking.AddressID = addressID
		result.Booking = &booking

		// Populate Service
		if sID != nil {
			service.ServiceID = *sID
		}
		if sName != nil { service.Name = *sName }
		if sDesc != nil { service.Description = *sDesc }
		if sPrice != nil { service.BasePrice = *sPrice }
		if sDur != nil { service.DurationMinutes = *sDur }
		if sCat != nil { service.Category = *sCat }
		if sActive != nil { service.IsActive = *sActive } else { service.IsActive = true }
		if sImg != nil { service.PreviewImageURL = *sImg }

		// Populate Address
		if aID != nil {
			address.AddressID = *aID
		}
		if aLabel != nil { address.Label = *aLabel }
		if aStreet != nil { address.Street = *aStreet }
		if aCity != nil { address.City = *aCity }
		if aProv != nil { address.Province = *aProv }
		if aPostal != nil { address.PostalCode = *aPostal }
		if aCountry != nil { address.Country = *aCountry }
		if aLat != nil { address.Latitude = aLat }
		if aLng != nil { address.Longitude = aLng }
		if aDef != nil { address.IsDefault = *aDef }

		// Populate Result Details
		if cName != nil { result.ClientName = *cName }
		if cPhone != nil { result.ClientPhone = *cPhone }
		if cPhoto != nil { result.ClientPhoto = *cPhoto }
		if cGen != nil { result.ClientGender = *cGen }
		if tName != nil { result.TherapistName = *tName }
		if tPhone != nil { result.TherapistPhone = *tPhone }
		if tPhoto != nil { result.TherapistPhoto = *tPhoto }
		if tGen != nil { result.TherapistGender = *tGen }
		if pCode != nil { result.PromoCode = *pCode }

		if serviceID != nil && service.ServiceID != 0 {
			svcCopy := service
			result.Service = &svcCopy
		}
		if addressID != nil && address.AddressID != 0 {
			addrCopy := address
			result.Address = &addrCopy
		}
		if therapistRating != nil {
			result.TherapistRating = therapistRating
		}

		results = append(results, result)
	}

	return results, rows.Err()
}

// scanBookingDetailsList is a helper that executes a query and scans multiple BookingDetailsResult rows
func (r *bookingRepoImpl) scanBookingDetailsList(ctx context.Context, query string, id int64) ([]BookingDetailsResult, error) {
	rows, err := r.db.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BookingDetailsResult
	for rows.Next() {
		var result BookingDetailsResult
		var booking model.Booking
		var service model.Service
		var address model.Address
		var serviceID, addressID *int64
		var therapistRating *float64


		// Service variables
		var sID *int64
		var sName, sDesc, sCat, sImg *string
		var sPrice *float64
		var sDur *int
		var sActive *bool

		// Address variables
		var aID *int64
		var aLabel, aStreet, aCity, aProv, aPostal, aCountry *string
		var aLat, aLng *float64
		var aDef *bool

		// User variables
		var cName, cPhone, cPhoto, cGen, tName, tPhone, tPhoto, tGen *string
		var pCode *string

		err := rows.Scan(
			// Booking fields
			&booking.BookingID, &booking.ReferenceCode, &booking.ClientID, &booking.TherapistID, &booking.AssignedAt,
			&serviceID, &addressID, &booking.PromoID, &booking.PaymentMethod,
			&booking.GenderPref, &booking.PressurePref, &booking.Notes, &booking.DurationMinutes,
			&booking.ScheduledStart, &booking.ActualStart, &booking.ActualEnd, &booking.TherapistArrivedAt,
			&booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason,
			&booking.RawTotal, &booking.Discount, &booking.FinalTotal, &booking.Status,
			&booking.CreatedAt, &booking.UpdatedAt, &booking.TotalPausedSeconds, &booking.CurrentPauseStart, &booking.ExtensionWaitSeconds,
			&booking.IsRated,
			// Service fields
			&sID, &sName, &sDesc, &sPrice, &sDur, &sCat, &sActive, &sImg,
			// Address fields
			&aID, &aLabel, &aStreet, &aCity, &aProv, &aPostal, &aCountry, &aLat, &aLng, &aDef,
			// Client info
			&cName, &cPhone, &cPhoto, &cGen,
			// Therapist info
			&tName, &tPhone, &tPhoto, &tGen,
			&therapistRating,
			// Promo Code
			&pCode,
		)
		if err != nil {
			return nil, err
		}

		booking.ServiceID = serviceID
		booking.AddressID = addressID
		result.Booking = &booking

		// Populate Service
		if sID != nil {
			service.ServiceID = *sID
		}
		if sName != nil { service.Name = *sName }
		if sDesc != nil { service.Description = *sDesc }
		if sPrice != nil { service.BasePrice = *sPrice }
		if sDur != nil { service.DurationMinutes = *sDur }
		if sCat != nil { service.Category = *sCat }
		if sActive != nil { service.IsActive = *sActive } else { service.IsActive = true }
		if sImg != nil { service.PreviewImageURL = *sImg }

		// Populate Address
		if aID != nil {
			address.AddressID = *aID
		}
		if aLabel != nil { address.Label = *aLabel }
		if aStreet != nil { address.Street = *aStreet }
		if aCity != nil { address.City = *aCity }
		if aProv != nil { address.Province = *aProv }
		if aPostal != nil { address.PostalCode = *aPostal }
		if aCountry != nil { address.Country = *aCountry }
		if aLat != nil { address.Latitude = aLat }
		if aLng != nil { address.Longitude = aLng }
		if aDef != nil { address.IsDefault = *aDef }

		// Populate Result Details
		if cName != nil { result.ClientName = *cName }
		if cPhone != nil { result.ClientPhone = *cPhone }
		if cPhoto != nil { result.ClientPhoto = *cPhoto }
		if cGen != nil { result.ClientGender = *cGen }
		if tName != nil { result.TherapistName = *tName }
		if tPhone != nil { result.TherapistPhone = *tPhone }
		if tPhoto != nil { result.TherapistPhoto = *tPhoto }
		if tGen != nil { result.TherapistGender = *tGen }
		if pCode != nil { result.PromoCode = *pCode }

		if serviceID != nil && service.ServiceID != 0 {
			svcCopy := service
			result.Service = &svcCopy
		}
		if addressID != nil && address.AddressID != 0 {
			addrCopy := address
			result.Address = &addrCopy
		}
		if therapistRating != nil {
			result.TherapistRating = therapistRating
		}

		results = append(results, result)
	}

	return results, rows.Err()
}

// ListGlobalPending returns all bookings with status='pending' ordered by created_at ASC (oldest first)
func (r *bookingRepoImpl) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	query := `
		SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			   payment_method, gender_preference, pressure_preference, notes, duration_minutes,
			   scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			   raw_total, discount, final_total, status,
			   created_at, updated_at, total_paused_seconds, current_pause_start, extension_wait_seconds
		FROM bookings
		WHERE status = 'pending' AND therapist_id IS NULL
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, query)
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
			&b.TotalPausedSeconds,
			&b.CurrentPauseStart,
			&b.ExtensionWaitSeconds,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetTherapistBookingCounts returns a map of therapist_id -> total completed bookings since the given time
func (r *bookingRepoImpl) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	if len(therapistIDs) == 0 {
		return map[int64]int{}, nil
	}

	query := `
		SELECT therapist_id, COUNT(*) as cnt
		FROM bookings
		WHERE therapist_id = ANY($1)
		  AND status = 'completed'
		  AND scheduled_start >= $2
		GROUP BY therapist_id
	`

	rows, err := r.db.Query(ctx, query, therapistIDs, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]int)
	for rows.Next() {
		var tid int64
		var cnt int
		if err := rows.Scan(&tid, &cnt); err != nil {
			return nil, err
		}
		out[tid] = cnt
	}

	return out, rows.Err()
}

// SetPauseStart sets the current_pause_start field for a booking
func (r *bookingRepoImpl) SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET current_pause_start = $1, updated_at = NOW()
		WHERE booking_id = $2
	`, pauseStart, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ClearPauseAndAddDuration clears current_pause_start and sets total_paused_seconds
func (r *bookingRepoImpl) ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET current_pause_start = NULL, 
		    total_paused_seconds = $1,
		    updated_at = NOW()
		WHERE booking_id = $2
	`, totalPausedSeconds, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListInProgressBookings returns all bookings with status='in_progress' and actual_start set
func (r *bookingRepoImpl) ListInProgressBookings(ctx context.Context) ([]model.Booking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			   payment_method,
			   gender_preference, pressure_preference, notes, duration_minutes,
			   scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			   raw_total, discount, final_total, status,
			   created_at, updated_at, total_paused_seconds, current_pause_start, extension_wait_seconds
		FROM bookings
		WHERE status = 'in_progress' AND actual_start IS NOT NULL
		ORDER BY actual_start ASC
	`

	rows, err := r.db.Query(ctx, query)
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
			&b.TotalPausedSeconds,
			&b.CurrentPauseStart,
			&b.ExtensionWaitSeconds,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}

	return out, nil
}

// ListUpcomingBookingsForReminder returns assigned bookings with scheduled_start in [start, end)
// that do NOT already have a booking_events row with the given eventTypeExclude.
func (r *bookingRepoImpl) ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at,
		       b.service_id, b.address_id, b.promo_id, b.payment_method,
		       b.gender_preference, b.pressure_preference, b.notes, b.duration_minutes,
		       b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at,
		       b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
		       b.raw_total, b.discount, b.final_total, b.status,
		       b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds
		FROM bookings b
		LEFT JOIN booking_events be ON be.booking_id = b.booking_id AND be.event_type = $3
		WHERE b.status = 'assigned'
		  AND b.scheduled_start >= $1
		  AND b.scheduled_start < $2
		  AND be.event_id IS NULL
		ORDER BY b.scheduled_start ASC
	`

	rows, err := r.db.Query(ctx, query, start, end, eventTypeExclude)
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
			&b.TotalPausedSeconds,
			&b.CurrentPauseStart,
			&b.ExtensionWaitSeconds,
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

// UnassignTherapist clears the therapist_id and resets status to pending for reassignment.
func (r *bookingRepoImpl) UnassignTherapist(ctx context.Context, bookingID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	now := time.Now()
	cmd, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET therapist_id = NULL, assigned_at = NULL, status = 'pending', updated_at = $1
		WHERE booking_id = $2
	`, now, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetClientBookingStats returns completed bookings count and late cancellation count for banning policy.
// lateCancellationSince allows filtering late cancellations to a rolling window (e.g., 6 months).
func (r *bookingRepoImpl) GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*ClientBookingStats, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	stats := &ClientBookingStats{}

	// Count completed bookings (all-time, used to determine new vs returning client)
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM bookings
		WHERE client_id = $1 AND status = 'completed'
	`, clientID).Scan(&stats.TotalCompleted)
	if err != nil {
		return nil, fmt.Errorf("failed to count completed bookings: %w", err)
	}

	// Count late cancellation events within the rolling window
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM booking_events be
		JOIN bookings b ON be.booking_id = b.booking_id
		WHERE b.client_id = $1 AND be.event_type = 'late_cancellation_by_client'
		  AND be.created_at >= $2
	`, clientID, lateCancellationSince).Scan(&stats.TotalLateCancellations)
	if err != nil {
		return nil, fmt.Errorf("failed to count late cancellations: %w", err)
	}

	return stats, nil
}

// CountEventsByTypeAndActor counts booking events where actor_id matches and event_type is as specified since a given time.
// This is used for therapist unassignment tracking (daily/weekly limits).
func (r *bookingRepoImpl) CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM booking_events
		WHERE actor_id = $1 AND event_type = $2 AND created_at >= $3
	`, actorID, eventType, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count events: %w", err)
	}

	return count, nil
}

// GetAccountingSummary returns aggregated revenue, therapist payouts, and platform profit
// for completed bookings within the specified date range.
func (r *bookingRepoImpl) GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*AccountingSummary, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var totalRevenue, totalPayouts, totalProfit float64
	var bookingCount int

	err := r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(final_total), 0.0) as total_revenue,
			COALESCE(SUM(therapist_earnings), 0.0) as total_payouts,
			COALESCE(SUM(platform_fee), 0.0) as total_profit,
			COUNT(*) as booking_count
		FROM bookings
		WHERE status = 'completed'
		  AND actual_end >= $1 AND actual_end <= $2
	`, startDate, endDate).Scan(&totalRevenue, &totalPayouts, &totalProfit, &bookingCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounting summary: %w", err)
	}

	return &AccountingSummary{
		TotalRevenue:          totalRevenue,
		TotalTherapistPayouts: totalPayouts,
		TotalPlatformProfit:   totalProfit,
		BookingCount:          bookingCount,
	}, nil
}

// GetDailyAccounting returns daily breakdown of revenue, therapist payouts, and platform profit
// for completed bookings within the specified date range.
func (r *bookingRepoImpl) GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]DailyAccountingEntry, error) {
	ctx, cancel := db.WithLongQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT 
			actual_end::date as date,
			COALESCE(SUM(final_total), 0.0) as revenue,
			COALESCE(SUM(therapist_earnings), 0.0) as payouts,
			COALESCE(SUM(platform_fee), 0.0) as profit,
			COUNT(*) as booking_count
		FROM bookings
		WHERE status = 'completed'
		  AND actual_end >= $1 AND actual_end <= $2
		GROUP BY actual_end::date
		ORDER BY actual_end::date ASC
	`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily accounting: %w", err)
	}
	defer rows.Close()

	var entries []DailyAccountingEntry
	for rows.Next() {
		var entry DailyAccountingEntry
		if err := rows.Scan(&entry.Date, &entry.Revenue, &entry.TherapistPayouts, &entry.PlatformProfit, &entry.BookingCount); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func (r *bookingRepoImpl) CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET status = 'completed',
			actual_end = $1,
			therapist_earnings = $3,
			platform_fee = $4,
			updated_at = $1
		WHERE booking_id = $2 AND status = 'in_progress'
	`, actualEnd, bookingID, earnings, fee)
	return err
}

// CompleteBookingWithLedgerTx atomically completes a booking and inserts corresponding ledger entries.
// This prevents ledger drift if the process crashes between status update and ledger write.
func (r *bookingRepoImpl) CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error {
	// Acquire a transaction from the pool
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Update booking status to completed
	cmd, err := tx.Exec(ctx, `
		UPDATE bookings
		SET status = 'completed',
			actual_end = $1,
			therapist_earnings = $3,
			platform_fee = $4,
			updated_at = $1
		WHERE booking_id = $2 AND status = 'in_progress'
	`, actualEnd, bookingID, earnings, fee)
	if err != nil {
		return fmt.Errorf("failed to update booking status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("booking %d not updated (not in_progress or not found)", bookingID)
	}

	// 2. Insert revenue ledger entry (credit)
	if revenue > 0 {
		_, err = tx.Exec(ctx, `
			INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, status)
			VALUES ($1, 'credit', 'revenue', $2, 'Client payment', $3, 'approved')
		`, bookingID, revenue, actualEnd)
		if err != nil {
			return fmt.Errorf("failed to insert revenue ledger entry: %w", err)
		}
	}

	// 3. Insert payout ledger entry (debit) for therapist earnings
	if earnings != nil && *earnings > 0 {
		_, err = tx.Exec(ctx, `
			INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, status, target_user_id)
			VALUES ($1, 'debit', 'payout', $2, 'Therapist payout', $3, 'approved', $4)
		`, bookingID, *earnings, actualEnd, therapistID)
		if err != nil {
			return fmt.Errorf("failed to insert payout ledger entry: %w", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func getPooledFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func getPooledInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func getPooledBool(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func getPooledTime(v *time.Time) time.Time {
	if v == nil {
		return time.Time{}
	}
	return *v
}
