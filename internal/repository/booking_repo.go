package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
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
	ErrServiceNotOffered     = errors.New("therapist does not offer this service")
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
	ActiveRide      *model.Ride
	HatidRide       *model.Ride
	SundoRide       *model.Ride
}

// BookingRepository defines data access methods for bookings.
// BookingWriter handles state-changing operations
type BookingWriter interface {
	Create(ctx context.Context, booking *model.Booking) error
	CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error
	Update(ctx context.Context, booking *model.Booking) error
	UpdateAdmin(ctx context.Context, booking *model.Booking) error
	UpdateStatus(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string) error
	UpdateStatusWithTime(ctx context.Context, bookingID, actorID int64, role, status string, cancelledBy *string, cancellationReason *string, customTime *time.Time) error
	AssignTherapist(ctx context.Context, bookingID, therapistID int64) error
	AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error
	AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error
	UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error
	SetPauseStart(ctx context.Context, bookingID int64, pauseStart *time.Time) error
	ClearPauseAndAddDuration(ctx context.Context, bookingID int64, totalPausedSeconds int) error
	CompleteBooking(ctx context.Context, bookingID int64, earnings, fee *float64, actualEnd time.Time) error
	CompleteBookingWithLedgerTx(ctx context.Context, pool db.DBTX, bookingID int64, therapistID *int64, earnings, fee *float64, revenue float64, actualEnd time.Time) error
	InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error
	RevertOnTheWayToAssigned(ctx context.Context, bookingID, actorID int64) (*RevertOnTheWayToAssignedResult, error)
}

// BookingReader handles read-only operations
type BookingReader interface {
	GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error)
	GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error)
	GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error)
	GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*BookingDetailsResult, error)
	GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*BookingDetailsResult, error)
	GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*BookingDetailsResult, error)
	GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*BookingDetailsResult, error)
	ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error)
	ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error)
	ListByClientWithDetails(ctx context.Context, clientID int64) ([]BookingDetailsResult, error)
	ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]BookingDetailsResult, error)
	ListByClientWithDetailsPaginated(ctx context.Context, clientID int64, limit, offset int) ([]BookingDetailsResult, int, error)
	ListByTherapistWithDetailsPaginated(ctx context.Context, therapistID int64, limit, offset int) ([]BookingDetailsResult, int, error)
	ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]BookingDetailsResult, int, error)
	ListGlobalPending(ctx context.Context) ([]model.Booking, error)
	ListInProgressBookings(ctx context.Context) ([]model.Booking, error)
	FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*BookingDetailsResult, error)
	ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error)
	ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error)
	ListAllEvents(ctx context.Context, params ListAllEventsParams) ([]model.BookingEvent, int, error)
	GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error)
	GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*BookingDetailsResult, error)
	GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error)
	GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error)
	HasAssignedOutboundRiderCoverage(ctx context.Context, bookingID int64) (bool, error)
}

// BookingAnalytics handles stats and reporting
type BookingAnalytics interface {
	GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error)
	GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error)
	GetClientBookingStats(ctx context.Context, clientID int64, lateCancellationSince time.Time) (*ClientBookingStats, error)
	CountEventsByTypeAndActor(ctx context.Context, actorID int64, eventType string, since time.Time) (int, error)
	GetAccountingSummary(ctx context.Context, startDate, endDate time.Time) (*AccountingSummary, error)
	GetDailyAccounting(ctx context.Context, startDate, endDate time.Time) ([]DailyAccountingEntry, error)
}

// BookingRepository defines data access methods for bookings.
type BookingRepository interface {
	BookingWriter
	BookingReader
	BookingAnalytics
}

// ClientBookingStats holds statistics for banning policy evaluation
type ClientBookingStats struct {
	TotalCompleted         int
	TotalLateCancellations int
}

// ListAllEventsParams holds filtering and pagination parameters for listing all events
type ListAllEventsParams struct {
	Limit     int
	Offset    int
	EventType *string
	ActorID   *int64
	StartDate *time.Time
	EndDate   *time.Time
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

type RevertOnTheWayToAssignedResult struct {
	ClearedRideID   int64
	ClearedRiderID  int64
	PassengerID     int64
	ClearedOutbound bool
}

type bookingRepoImpl struct {
	db db.DBTX
}

// selectBookingFields is the shared list of columns for booking queries.
// This reduces duplication and ensures consistency across GetByID, GetByBookingID, ListByClient, etc.
const selectBookingFields = `booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
		   payment_method, change_for,
		   COALESCE(gender_preference, 'any'), COALESCE(pressure_preference, 'medium'), COALESCE(notes, ''), duration_minutes,
		   scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
		   raw_total, discount, final_total, status,
		   created_at, updated_at, total_paused_seconds, current_pause_start, extension_wait_seconds,
		   group_id, COALESCE(guest_name, 'Self'), sequence_number, start_condition`

func NewBookingRepository(db db.DBTX) BookingRepository {
	return &bookingRepoImpl{db: db}
}

func (r *bookingRepoImpl) Create(ctx context.Context, booking *model.Booking) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()
	return r.create(ctx, r.db, booking)
}

func (r *bookingRepoImpl) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	return r.create(ctx, tx, booking)
}

func (r *bookingRepoImpl) create(ctx context.Context, q db.DBTX, booking *model.Booking) error {
	booking.StartCondition = strings.TrimSpace(booking.StartCondition)
	if booking.StartCondition == "" {
		booking.StartCondition = "fixed_time"
	}

	query := `
		INSERT INTO bookings (
			client_id, therapist_id, service_id, address_id, promo_id,
			payment_method, change_for,
			gender_preference, pressure_preference, notes,
			duration_minutes, scheduled_start, raw_total, discount, final_total, status, reference_code,
			group_id, guest_name, sequence_number, start_condition
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
		)
		RETURNING booking_id, created_at, updated_at, assigned_at, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason
    `

	return q.QueryRow(ctx, query,
		booking.ClientID,
		booking.TherapistID,
		booking.ServiceID,
		booking.AddressID,
		booking.PromoID,
		booking.PaymentMethod,
		booking.ChangeFor,
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
		booking.GroupID,
		booking.GuestName,
		booking.SequenceNumber,
		booking.StartCondition,
	).Scan(&booking.BookingID, &booking.CreatedAt, &booking.UpdatedAt, &booking.AssignedAt, &booking.TherapistArrivedAt, &booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason)
}

func (r *bookingRepoImpl) HasActiveNonFinalBookings(ctx context.Context, therapistID int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM bookings
			WHERE therapist_id = $1
			  AND status NOT IN ($2, $3, $4, $5, $6)
		)
	`, therapistID, model.BookingStatusCompleted, model.BookingStatusCancelled, model.BookingStatusNoShow, "paid", "rescheduled").Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *bookingRepoImpl) HasAssignedOutboundRiderCoverage(ctx context.Context, bookingID int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM rides
			WHERE booking_id = $1
			  AND ride_type = $2
			  AND rider_id IS NOT NULL
			  AND status IN ($3, $4, $5)
		)
	`, bookingID, "outbound", "offered", "accepted", "arrived_pickup").Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *bookingRepoImpl) ClearAssignedOutboundRider(ctx context.Context, bookingID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := r.db.Exec(ctx, `
		UPDATE rides
		SET rider_id = NULL,
			status = CASE WHEN status IN ('assigned', 'accepted') THEN 'pending' ELSE status END,
			updated_at = NOW()
		WHERE booking_id = $1
		  AND ride_type = $2
		  AND rider_id IS NOT NULL
	`, bookingID, "outbound")
	return err
}

func (r *bookingRepoImpl) RevertOnTheWayToAssigned(ctx context.Context, bookingID, actorID int64) (*RevertOnTheWayToAssignedResult, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	var updatedBookingID int64
	if err := tx.QueryRow(ctx, `
		UPDATE bookings
		SET status = $1,
			therapist_arrived_at = NULL,
			actual_start = NULL,
			current_pause_start = NULL,
			updated_at = NOW()
		WHERE booking_id = $2
		  AND status = $3
		RETURNING booking_id
	`, model.BookingStatusAssigned, bookingID, model.BookingStatusOnTheWay).Scan(&updatedBookingID); err != nil {
		return nil, err
	}

	result := &RevertOnTheWayToAssignedResult{}
	var rideID, riderID, passengerID int64
	err = tx.QueryRow(ctx, `
		UPDATE rides AS target
		SET rider_id = NULL,
			status = $2,
			accepted_at = NULL,
			offered_at = NULL,
			updated_at = NOW()
		FROM (
			SELECT ride_id, rider_id AS old_rider_id, passenger_id
			FROM rides
			WHERE booking_id = $1
			  AND ride_type = $3
			  AND rider_id IS NOT NULL
			  AND status IN ($4, $5, $6)
			ORDER BY ride_id
			LIMIT 1
			FOR UPDATE
		) existing
		WHERE target.ride_id = existing.ride_id
		RETURNING target.ride_id, existing.old_rider_id, target.passenger_id
	`, bookingID, "pending", "outbound", "offered", "accepted", "arrived_pickup").Scan(&rideID, &riderID, &passengerID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	} else {
		result.ClearedOutbound = true
		result.ClearedRideID = rideID
		result.ClearedRiderID = riderID
		result.PassengerID = passengerID
	}

	if err := r.insertBookingEvent(ctx, tx, bookingID, model.BookingStatusAssigned, &actorID, nil); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	committed = true
	return result, nil
}

func (r *bookingRepoImpl) scanBooking(s pgx.Row, b *model.Booking) error {
	return s.Scan(
		&b.BookingID,
		&b.ReferenceCode,
		&b.ClientID,
		&b.TherapistID,
		&b.AssignedAt,
		&b.ServiceID,
		&b.AddressID,
		&b.PromoID,
		&b.PaymentMethod,
		&b.ChangeFor,
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
		&b.GroupID,
		&b.GuestName,
		&b.SequenceNumber,
		&b.StartCondition,
	)
}

func (r *bookingRepoImpl) scanBookingWithGroups(s pgx.Row, b *model.Booking) error {
	return s.Scan(
		&b.BookingID, &b.ReferenceCode, &b.ClientID, &b.TherapistID, &b.AssignedAt,
		&b.ServiceID, &b.AddressID, &b.PromoID, &b.PaymentMethod,
		&b.GenderPref, &b.PressurePref, &b.Notes, &b.DurationMinutes,
		&b.ScheduledStart, &b.ActualStart, &b.ActualEnd, &b.TherapistArrivedAt, &b.NoShowAt,
		&b.CancelledBy, &b.CancelledAt, &b.CancellationReason,
		&b.RawTotal, &b.Discount, &b.FinalTotal, &b.Status,
		&b.CreatedAt, &b.UpdatedAt, &b.TotalPausedSeconds, &b.CurrentPauseStart, &b.ExtensionWaitSeconds,
		&b.GroupID, &b.GuestName, &b.SequenceNumber, &b.StartCondition,
	)
}

const selectBookingDetailsFields = `
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			COALESCE(b.gender_preference, 'any'), COALESCE(b.pressure_preference, 'medium'), COALESCE(b.notes, ''), b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, 
			b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			b.group_id, COALESCE(b.guest_name, 'Self'), b.sequence_number, b.start_condition,
			(SELECT COUNT(*) > 0 FROM reviews r WHERE r.booking_id = b.booking_id AND r.deleted_at IS NULL) as is_rated,
			-- Service fields (LEFT JOIN)
			COALESCE(s.service_id, 0), COALESCE(s.name, ''), COALESCE(s.description, ''), s.base_price, 
			s.duration_minutes, COALESCE(s.category, ''), s.is_active, 
			COALESCE(s.preview_image_url, ''), s.therapist_commission, s.deleted_at, s.created_at,
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
		LEFT JOIN promotions p ON b.promo_id = p.promo_id AND p.deleted_at IS NULL`

func (r *bookingRepoImpl) scanBookingDetails(s interface{ Scan(dest ...any) error }, res *BookingDetailsResult) error {
	var booking model.Booking
	var serviceID *int64
	var sServiceID *int64
	var sName, sDesc, sCat, sImg string
	var sBasePrice *float64
	var sDuration *int
	var sIsActive *bool
	var sTherapistCommission *float64
	var sDeletedAt, sCreatedAt *time.Time
	var aAddrID, aUserID *int64
	var aLabel, aStreet, aCity, aProv, aZip, aCountry string
	var aLat, aLon *float64
	var aIsDefault *bool
	var aCreatedAt, aUpdatedAt *time.Time

	err := s.Scan(
		&booking.BookingID, &booking.ReferenceCode, &booking.ClientID, &booking.TherapistID, &booking.AssignedAt,
		&serviceID, &booking.AddressID, &booking.PromoID, &booking.PaymentMethod,
		&booking.GenderPref, &booking.PressurePref, &booking.Notes, &booking.DurationMinutes,
		&booking.ScheduledStart, &booking.ActualStart, &booking.ActualEnd, &booking.TherapistArrivedAt,
		&booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason,
		&booking.RawTotal, &booking.Discount, &booking.FinalTotal, &booking.Status,
		&booking.CreatedAt, &booking.UpdatedAt, &booking.TotalPausedSeconds, &booking.CurrentPauseStart, &booking.ExtensionWaitSeconds,
		&booking.GroupID, &booking.GuestName, &booking.SequenceNumber, &booking.StartCondition,
		&booking.IsRated,
		&sServiceID, &sName, &sDesc, &sBasePrice,
		&sDuration, &sCat, &sIsActive,
		&sImg, &sTherapistCommission, &sDeletedAt, &sCreatedAt,
		&aAddrID, &aUserID, &aLabel, &aStreet,
		&aCity, &aProv, &aZip,
		&aCountry, &aLat, &aLon,
		&aIsDefault, &aCreatedAt, &aUpdatedAt,
		&res.ClientName, &res.ClientPhone, &res.ClientPhoto, &res.ClientGender,
		&res.TherapistName, &res.TherapistPhone, &res.TherapistPhoto, &res.TherapistGender, &res.TherapistRating,
		&res.PromoCode,
	)
	if err != nil {
		return err
	}

	booking.ServiceID = serviceID
	res.Booking = &booking

	if sServiceID != nil && *sServiceID != 0 && sBasePrice != nil && sDuration != nil && sIsActive != nil && sCreatedAt != nil {
		res.Service = &model.Service{
			ServiceID:           *sServiceID,
			Name:                sName,
			Description:         sDesc,
			BasePrice:           *sBasePrice,
			DurationMinutes:     *sDuration,
			Category:            sCat,
			IsActive:            *sIsActive,
			PreviewImageURL:     sImg,
			TherapistCommission: sTherapistCommission,
			DeletedAt:           sDeletedAt,
			CreatedAt:           *sCreatedAt,
		}
	}

	if aAddrID != nil && *aAddrID != 0 {
		res.Address = &model.Address{
			AddressID:  *aAddrID,
			UserID:     *aUserID,
			Label:      aLabel,
			Street:     aStreet,
			City:       aCity,
			Province:   aProv,
			PostalCode: aZip,
			Country:    aCountry,
			Latitude:   aLat,
			Longitude:  aLon,
			IsDefault:  *aIsDefault,
			CreatedAt:  *aCreatedAt,
			UpdatedAt:  *aUpdatedAt,
		}
	}

	return nil
}

func (r *bookingRepoImpl) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingFields + ` FROM bookings WHERE booking_id = $1 AND client_id = $2`
	var b model.Booking
	if err := r.scanBooking(r.db.QueryRow(ctx, query, bookingID, userID), &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *bookingRepoImpl) GetByIDs(ctx context.Context, bookingIDs []int64) ([]model.Booking, error) {
	if len(bookingIDs) == 0 {
		return nil, nil
	}
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingFields + ` FROM bookings WHERE booking_id = ANY($1)`
	rows, err := r.db.Query(ctx, query, bookingIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []model.Booking
	for rows.Next() {
		var b model.Booking
		if err := r.scanBooking(rows, &b); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, nil
}

func (r *bookingRepoImpl) GetBookingWithDetailsBatch(ctx context.Context, bookingIDs []int64) (map[int64]*BookingDetailsResult, error) {
	if len(bookingIDs) == 0 {
		return make(map[int64]*BookingDetailsResult), nil
	}
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingDetailsFields + ` WHERE b.booking_id = ANY($1)`
	rows, err := r.db.Query(ctx, query, bookingIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[int64]*BookingDetailsResult)
	for rows.Next() {
		res := &BookingDetailsResult{}
		if err := r.scanBookingDetails(rows, res); err != nil {
			return nil, err
		}
		results[res.Booking.BookingID] = res
	}
	return results, nil
}

func (r *bookingRepoImpl) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingFields + `
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
		if err := r.scanBooking(rows, &b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// GetByBookingIDForUpdateTx fetches a booking with an Advisory Lock within a transaction.
// It uses pg_try_advisory_xact_lock(112233, bookingID) to prevent concurrent modifications.
// This is lighter than FOR UPDATE as it avoids writing to the table page (vacuum safe).
func (r *bookingRepoImpl) GetByBookingIDForUpdateTx(ctx context.Context, tx pgx.Tx, bookingID int64) (*model.Booking, error) {
	// 1. Try to acquire Advisory Lock (Namespace 112233 for Bookings)
	var locked bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(112233, $1)", bookingID).Scan(&locked); err != nil {
		return nil, fmt.Errorf("failed to check lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("booking %d is currently locked by another process", bookingID)
	}

	// 2. Fetch the data (No FOR UPDATE needed, we own the semantic lock)
	query := `SELECT ` + selectBookingFields + `
		FROM bookings
		WHERE booking_id = $1
	`
	var b model.Booking
	err := tx.QueryRow(ctx, query, bookingID).Scan(
		&b.BookingID, &b.ReferenceCode, &b.ClientID, &b.TherapistID, &b.AssignedAt,
		&b.ServiceID, &b.AddressID, &b.PromoID, &b.PaymentMethod, &b.ChangeFor,
		&b.GenderPref, &b.PressurePref, &b.Notes, &b.DurationMinutes,
		&b.ScheduledStart, &b.ActualStart, &b.ActualEnd, &b.TherapistArrivedAt, &b.NoShowAt,
		&b.CancelledBy, &b.CancelledAt, &b.CancellationReason,
		&b.RawTotal, &b.Discount, &b.FinalTotal, &b.Status,
		&b.CreatedAt, &b.UpdatedAt, &b.TotalPausedSeconds, &b.CurrentPauseStart, &b.ExtensionWaitSeconds,
		&b.GroupID, &b.GuestName, &b.SequenceNumber, &b.StartCondition,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("booking not found")
		}
		return nil, err
	}
	return &b, nil
}

func (r *bookingRepoImpl) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingFields + `
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
		if err := r.scanBooking(rows, &b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *bookingRepoImpl) FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*BookingDetailsResult, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, b.service_id, b.address_id, b.promo_id,
			b.payment_method, b.change_for,
			COALESCE(b.gender_preference, 'any'), COALESCE(b.pressure_preference, 'medium'), COALESCE(b.notes, ''), b.duration_minutes,
			b.scheduled_start, b.actual_start, b.actual_end, b.therapist_arrived_at, b.no_show_at, b.cancelled_by, b.cancelled_at, b.cancellation_reason,
			b.raw_total, b.discount, b.final_total, b.status,
			b.created_at, b.updated_at, b.total_paused_seconds, b.current_pause_start, b.extension_wait_seconds,
			b.group_id, COALESCE(b.guest_name, 'Self'), b.sequence_number, b.start_condition,
			a.address_id, a.user_id, COALESCE(a.label, ''), COALESCE(a.street_address, ''), COALESCE(a.city, ''),
			a.latitude, a.longitude, a.is_default, a.created_at, a.updated_at
		FROM bookings b
		JOIN addresses a ON a.address_id = b.address_id AND a.deleted_at IS NULL
		WHERE b.therapist_id = $1
		  AND b.booking_id <> $2
		  AND b.scheduled_start > $3
		  AND b.status NOT IN ('completed', 'cancelled', 'no_show', 'paid', 'rescheduled')
		  AND a.latitude IS NOT NULL
		  AND a.longitude IS NOT NULL
		ORDER BY b.scheduled_start ASC
		LIMIT 1`

	var booking model.Booking
	var address model.Address
	err := r.db.QueryRow(ctx, query, therapistID, excludeBookingID, after).Scan(
		&booking.BookingID, &booking.ReferenceCode, &booking.ClientID, &booking.TherapistID, &booking.AssignedAt,
		&booking.ServiceID, &booking.AddressID, &booking.PromoID, &booking.PaymentMethod, &booking.ChangeFor,
		&booking.GenderPref, &booking.PressurePref, &booking.Notes, &booking.DurationMinutes,
		&booking.ScheduledStart, &booking.ActualStart, &booking.ActualEnd, &booking.TherapistArrivedAt,
		&booking.NoShowAt, &booking.CancelledBy, &booking.CancelledAt, &booking.CancellationReason,
		&booking.RawTotal, &booking.Discount, &booking.FinalTotal, &booking.Status,
		&booking.CreatedAt, &booking.UpdatedAt, &booking.TotalPausedSeconds, &booking.CurrentPauseStart,
		&booking.ExtensionWaitSeconds, &booking.GroupID, &booking.GuestName, &booking.SequenceNumber,
		&booking.StartCondition,
		&address.AddressID, &address.UserID, &address.Label, &address.Street, &address.City,
		&address.Latitude, &address.Longitude, &address.IsDefault, &address.CreatedAt, &address.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &BookingDetailsResult{Booking: &booking, Address: &address}, nil
}

func (r *bookingRepoImpl) Update(ctx context.Context, booking *model.Booking) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
        UPDATE bookings target
        SET service_id = $1,
            address_id = $2,
            promo_id = $3,
            gender_preference = $4,
            pressure_preference = $5,
            notes = $6,
            duration_minutes = $7,
            scheduled_start = $8,
            payment_method = $9,
            change_for = $10,
            raw_total = $11,
            final_total = $12,
            updated_at = NOW()
        WHERE booking_id = $13 AND client_id = $14
    `, booking.ServiceID, booking.AddressID, booking.PromoID, booking.GenderPref, booking.PressurePref,
		booking.Notes, booking.DurationMinutes, booking.ScheduledStart, booking.PaymentMethod, booking.ChangeFor, booking.RawTotal, booking.FinalTotal,
		booking.BookingID, booking.ClientID)
	if err != nil {
		slog.Error("Update booking failed", "booking_id", booking.BookingID, "client_id", booking.ClientID, "error", err)
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *bookingRepoImpl) UpdateAdmin(ctx context.Context, booking *model.Booking) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
        UPDATE bookings target
        SET service_id = $1,
            address_id = $2,
            promo_id = $3,
            gender_preference = $4,
            pressure_preference = $5,
            notes = $6,
            duration_minutes = $7,
            scheduled_start = $8,
            therapist_id = $9,
            payment_method = $10,
            change_for = $11,
            raw_total = $12,
            final_total = $13,
            status = $14,
            assigned_at = $15,
            updated_at = NOW()
		WHERE booking_id = $16
		  AND (
			$9::bigint IS NULL
			OR (
				EXISTS (
					SELECT 1
					FROM therapist_profiles tp
					JOIN users u ON u.user_id = tp.therapist_id
					WHERE tp.therapist_id = $9
					  AND tp.accept_assignments = TRUE
					  AND u.account_status = 'active'
					  AND u.deleted_at IS NULL
				)
				AND EXISTS (
					SELECT 1
					FROM therapist_services ts
					WHERE ts.therapist_id = $9
					  AND ts.service_id = $1
				)
				AND NOT EXISTS (
					SELECT 1
					FROM bookings other
					WHERE other.booking_id <> target.booking_id
					  AND other.therapist_id = $9
					  AND other.status IN ($14, $17, $18)
					  AND other.scheduled_start < ($8 + ($7 * interval '1 minute'))
					  AND $8 < (other.scheduled_start + (other.duration_minutes * interval '1 minute'))
				)
			)
		  )
    `, booking.ServiceID, booking.AddressID, booking.PromoID, booking.GenderPref, booking.PressurePref,
		booking.Notes, booking.DurationMinutes, booking.ScheduledStart, booking.TherapistID, booking.PaymentMethod, booking.ChangeFor, booking.RawTotal, booking.FinalTotal,
		booking.Status, booking.AssignedAt,
		booking.BookingID, model.BookingStatusInProgress, model.BookingStatusArrived)
	if err != nil {
		slog.Error("UpdateAdmin booking failed", "booking_id", booking.BookingID, "error", err)
		return err
	}
	if cmd.RowsAffected() == 0 {
		if booking.TherapistID != nil {
			if err := r.classifyUpdateAdminAssignmentFailure(ctx, booking); err != nil {
				return err
			}
		}
		return pgx.ErrNoRows
	}
	return nil
}

func (r *bookingRepoImpl) classifyUpdateAdminAssignmentFailure(ctx context.Context, booking *model.Booking) error {
	var status string
	var acceptAssignments bool
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(u.account_status, ''), tp.accept_assignments
		FROM therapist_profiles tp
		JOIN users u ON u.user_id = tp.therapist_id
		WHERE tp.therapist_id = $1
		  AND u.deleted_at IS NULL
	`, *booking.TherapistID).Scan(&status, &acceptAssignments); err != nil {
		if err == pgx.ErrNoRows {
			return ErrTherapistNotFound
		}
		return err
	}
	if status != "active" || !acceptAssignments {
		return ErrTherapistNotAccepting
	}

	if booking.ServiceID != nil {
		var offersService bool
		if err := r.db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM therapist_services
				WHERE therapist_id = $1 AND service_id = $2
			)
		`, *booking.TherapistID, *booking.ServiceID).Scan(&offersService); err != nil {
			return err
		}
		if !offersService {
			return ErrServiceNotOffered
		}
	}

	return ErrAssignConflict
}

func (r *bookingRepoImpl) insertBookingEvent(ctx context.Context, q db.DBTX, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
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
	_, err := q.Exec(ctx, `
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
	if err := r.db.QueryRow(ctx, `
		SELECT tp.accept_assignments
		FROM therapist_profiles tp
		JOIN users u ON u.user_id = tp.therapist_id
		WHERE tp.therapist_id = $1
		  AND u.account_status = 'active'
		  AND u.deleted_at IS NULL
	`, therapistID).Scan(&accept); err != nil {
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
		UPDATE bookings target
		SET therapist_id = $1, assigned_at = $2, status = $5, updated_at = $3
		WHERE target.booking_id = $4 AND target.therapist_id IS NULL
		  AND (target.status = $6 OR target.payment_method = $7)
		  AND $1 IN (
			SELECT tp.therapist_id
			FROM therapist_profiles tp
			JOIN users u ON u.user_id = tp.therapist_id
			WHERE tp.accept_assignments = TRUE
			  AND u.account_status = 'active'
			  AND u.deleted_at IS NULL
		  )
		  -- Ensure therapist offers this service
		  AND EXISTS (
			SELECT 1 FROM therapist_services ts
			WHERE ts.therapist_id = $1 AND ts.service_id = target.service_id
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM bookings other
			WHERE other.therapist_id = $1
			AND other.status IN ($5, $8, $9)
			AND other.scheduled_start < (target.scheduled_start + (target.duration_minutes * interval '1 minute'))
			AND target.scheduled_start < (other.scheduled_start + (other.duration_minutes * interval '1 minute'))
		  )
	`, therapistID, now, now, bookingID, model.BookingStatusAssigned, model.BookingStatusPending, model.PaymentMethodCash, model.BookingStatusInProgress, model.BookingStatusArrived)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		// Determine a clearer reason: check booking row
		var currentTherapist *int64
		var status string
		var paymentMethod *string
		var serviceID *int64
		if err := r.db.QueryRow(ctx, `SELECT therapist_id, status, payment_method, service_id FROM bookings WHERE booking_id = $1`, bookingID).Scan(&currentTherapist, &status, &paymentMethod, &serviceID); err != nil {
			if err == pgx.ErrNoRows {
				return pgx.ErrNoRows
			}
			return err
		}
		if currentTherapist != nil {
			return ErrAlreadyAssigned
		}

		// Check service compatibility specifically
		if serviceID != nil {
			var exists bool
			_ = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM therapist_services WHERE therapist_id = $1 AND service_id = $2)`, therapistID, *serviceID).Scan(&exists)
			if !exists {
				return ErrServiceNotOffered
			}
		}

		if !(status == model.BookingStatusPending || (paymentMethod != nil && *paymentMethod == model.PaymentMethodCash)) {
			return ErrBookingNotAssignable
		}
		return ErrAssignConflict
	}
	// Record event (actor is therapist)
	actor := therapistID
	_ = r.insertBookingEvent(ctx, r.db, bookingID, model.EventTypeAssigned, &actor, nil)
	return nil
}

// AssignTherapistWithActor behaves like AssignTherapist but records the provided
// actorID (for example an admin) as the actor for the 'assigned' event.
func (r *bookingRepoImpl) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	// Pre-check therapist
	var accept bool
	if err := r.db.QueryRow(ctx, `
		SELECT tp.accept_assignments
		FROM therapist_profiles tp
		JOIN users u ON u.user_id = tp.therapist_id
		WHERE tp.therapist_id = $1
		  AND u.account_status = 'active'
		  AND u.deleted_at IS NULL
	`, therapistID).Scan(&accept); err != nil {
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
		UPDATE bookings target
		SET therapist_id = $1, assigned_at = $2, status = $5, updated_at = $3
		WHERE target.booking_id = $4 AND target.therapist_id IS NULL
		  AND (target.status = $6 OR target.payment_method = $7)
		  AND $1 IN (
			SELECT tp.therapist_id
			FROM therapist_profiles tp
			JOIN users u ON u.user_id = tp.therapist_id
			WHERE tp.accept_assignments = TRUE
			  AND u.account_status = 'active'
			  AND u.deleted_at IS NULL
		  )
		  -- Ensure therapist offers this service
		  AND EXISTS (
			SELECT 1 FROM therapist_services ts
			WHERE ts.therapist_id = $1 AND ts.service_id = target.service_id
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM bookings other
			WHERE other.therapist_id = $1
			AND other.status IN ($5, $8, $9)
			AND other.scheduled_start < (target.scheduled_start + (target.duration_minutes * interval '1 minute'))
			AND target.scheduled_start < (other.scheduled_start + (other.duration_minutes * interval '1 minute'))
		  )
	`, therapistID, now, now, bookingID, model.BookingStatusAssigned, model.BookingStatusPending, model.PaymentMethodCash, model.BookingStatusInProgress, model.BookingStatusArrived)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		var currentTherapist *int64
		var status string
		var paymentMethod *string
		var serviceID *int64
		if err := r.db.QueryRow(ctx, `SELECT therapist_id, status, payment_method, service_id FROM bookings WHERE booking_id = $1`, bookingID).Scan(&currentTherapist, &status, &paymentMethod, &serviceID); err != nil {
			if err == pgx.ErrNoRows {
				return pgx.ErrNoRows
			}
			return err
		}
		if currentTherapist != nil {
			return ErrAlreadyAssigned
		}

		// Check service compatibility specifically
		if serviceID != nil {
			var exists bool
			_ = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM therapist_services WHERE therapist_id = $1 AND service_id = $2)`, therapistID, *serviceID).Scan(&exists)
			if !exists {
				return ErrServiceNotOffered
			}
		}

		if !(status == model.BookingStatusPending || (paymentMethod != nil && *paymentMethod == model.PaymentMethodCash)) {
			return ErrBookingNotAssignable
		}
		return ErrAssignConflict
	}
	// Record event using provided actor (admin or therapist)
	_ = r.insertBookingEvent(ctx, r.db, bookingID, model.EventTypeAssigned, &actorID, nil)
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
	if err := tx.QueryRow(ctx, `
		SELECT tp.accept_assignments
		FROM therapist_profiles tp
		JOIN users u ON u.user_id = tp.therapist_id
		WHERE tp.therapist_id = $1
		  AND u.account_status = 'active'
		  AND u.deleted_at IS NULL
		FOR UPDATE
	`, therapistID).Scan(&accept); err != nil {
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
		UPDATE bookings target
		SET therapist_id = $1, assigned_at = $2, status = $5, updated_at = $3
		WHERE target.booking_id = $4 AND target.therapist_id IS NULL
		  AND (target.status = $6 OR target.payment_method = $7)
		  AND $1 IN (
			SELECT tp.therapist_id
			FROM therapist_profiles tp
			JOIN users u ON u.user_id = tp.therapist_id
			WHERE tp.accept_assignments = TRUE
			  AND u.account_status = 'active'
			  AND u.deleted_at IS NULL
		  )
		  -- Ensure therapist offers this service
		  AND EXISTS (
			SELECT 1 FROM therapist_services ts
			WHERE ts.therapist_id = $1 AND ts.service_id = target.service_id
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM bookings other
			WHERE other.therapist_id = $1
			AND other.status IN ($5, $8, $9)
			AND other.scheduled_start < (target.scheduled_start + (target.duration_minutes * interval '1 minute'))
			AND target.scheduled_start < (other.scheduled_start + (other.duration_minutes * interval '1 minute'))
		  )
	`, therapistID, now, now, bookingID, model.BookingStatusAssigned, model.BookingStatusPending, model.PaymentMethodCash, model.BookingStatusInProgress, model.BookingStatusArrived)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		var currentTherapist *int64
		var status string
		var paymentMethod *string
		var serviceID *int64
		if err := tx.QueryRow(ctx, `SELECT therapist_id, status, payment_method, service_id FROM bookings WHERE booking_id = $1`, bookingID).Scan(&currentTherapist, &status, &paymentMethod, &serviceID); err != nil {
			if err == pgx.ErrNoRows {
				return pgx.ErrNoRows
			}
			return err
		}
		if currentTherapist != nil {
			return ErrAlreadyAssigned
		}

		// Check service compatibility specifically
		if serviceID != nil {
			var exists bool
			_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM therapist_services WHERE therapist_id = $1 AND service_id = $2)`, therapistID, *serviceID).Scan(&exists)
			if !exists {
				return ErrServiceNotOffered
			}
		}

		if !(status == model.BookingStatusPending || (paymentMethod != nil && *paymentMethod == model.PaymentMethodCash)) {
			return ErrBookingNotAssignable
		}
		return ErrAssignConflict
	}
	// Insert event using provided actor within same transaction
	return r.insertBookingEvent(ctx, tx, bookingID, model.EventTypeAssigned, &actorID, nil)
}

func (r *bookingRepoImpl) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingFields + `
		FROM bookings
		WHERE booking_id = $1
	`

	var b model.Booking
	if err := r.scanBooking(r.db.QueryRow(ctx, query, bookingID), &b); err != nil {
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

func (r *bookingRepoImpl) ListAllEvents(ctx context.Context, params ListAllEventsParams) ([]model.BookingEvent, int, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	whereClauses := []string{"1=1"}
	var args []interface{}
	argID := 1

	if params.EventType != nil && *params.EventType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("e.event_type = $%d", argID))
		args = append(args, *params.EventType)
		argID++
	}

	if params.ActorID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("e.actor_id = $%d", argID))
		args = append(args, *params.ActorID)
		argID++
	}

	if params.StartDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("e.created_at >= $%d", argID))
		args = append(args, *params.StartDate)
		argID++
	}

	if params.EndDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("e.created_at <= $%d", argID))
		args = append(args, *params.EndDate)
		argID++
	}

	whereQuery := strings.Join(whereClauses, " AND ")

	// Count total
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM booking_events e WHERE %s`, whereQuery)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []model.BookingEvent{}, 0, nil
	}

	// Fetch paginated
	query := fmt.Sprintf(`
		SELECT e.event_id, e.booking_id, e.event_type, e.actor_id, e.metadata, e.created_at, 
		       COALESCE(NULLIF(TRIM(u.full_name), ''), u.primary_email, u.role, '')
		FROM booking_events e
		LEFT JOIN users u ON e.actor_id = u.user_id
		WHERE %s
		ORDER BY e.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereQuery, argID, argID+1)

	args = append(args, params.Limit, params.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []model.BookingEvent
	for rows.Next() {
		var ev model.BookingEvent
		var metadata interface{}
		var actorName string
		if err := rows.Scan(&ev.EventID, &ev.BookingID, &ev.EventType, &ev.ActorID, &metadata, &ev.CreatedAt, &actorName); err != nil {
			return nil, 0, err
		}
		if actorName != "" {
			ev.ActorName = actorName
		}
		if metadata != nil {
			switch m := metadata.(type) {
			case map[string]any:
				ev.Metadata = m
			case string:
				var parsed map[string]any
				if err := json.Unmarshal([]byte(m), &parsed); err == nil {
					ev.Metadata = parsed
				}
			case []byte:
				var parsed map[string]any
				if err := json.Unmarshal(m, &parsed); err == nil {
					ev.Metadata = parsed
				}
			}
		}
		out = append(out, ev)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}

func (r *bookingRepoImpl) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()
	return r.insertBookingEvent(ctx, r.db, bookingID, eventType, actorID, metadata)
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
			therapist_arrived_at = CASE
				WHEN $1::text = $8 THEN $2
				WHEN $1::text IN ($14, $15) THEN NULL
				ELSE therapist_arrived_at
			END,
			actual_start = CASE
				WHEN $1::text = $9 AND actual_start IS NULL THEN $2
				WHEN $1::text IN ($14, $15, $8) THEN NULL
				ELSE actual_start
			END,
			actual_end = CASE WHEN $1::text = $10 THEN $2 WHEN $1::text <> $10 THEN NULL ELSE actual_end END,
			current_pause_start = CASE WHEN $1::text IN ($14, $15, $8, $9) THEN NULL ELSE current_pause_start END,
			no_show_at = CASE WHEN $1::text = $11 THEN $2 ELSE no_show_at END,
			cancelled_by = CASE WHEN $1::text = $12 THEN $5::text ELSE cancelled_by END,
			cancelled_at = CASE WHEN $1::text = $12 THEN $2 ELSE cancelled_at END,
			cancellation_reason = CASE WHEN $1::text IN ($11, $12) THEN $6::text ELSE cancellation_reason END,
			updated_at = $2
		WHERE booking_id = $3 AND ($7::text = $13 OR client_id = $4 OR therapist_id = $4)
	`, status, now, bookingID, userID, cancelledBy, cancellationReason, role,
		model.BookingStatusArrived, model.BookingStatusInProgress, model.BookingStatusCompleted, model.BookingStatusNoShow, model.BookingStatusCancelled, model.RoleAdmin,
		model.BookingStatusAssigned, model.BookingStatusOnTheWay)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	// Record event for timeline/audit
	// actor is the acting user
	actor := userID
	_ = r.insertBookingEvent(ctx, r.db, bookingID, status, &actor, bookingStatusEventMetadata(status, cancellationReason))
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
			therapist_arrived_at = CASE WHEN $1::text = $8 THEN $2 ELSE therapist_arrived_at END,
			actual_start = CASE WHEN $1::text = $9 THEN $2 ELSE actual_start END,
			actual_end = CASE WHEN $1::text = $10 THEN $2 ELSE actual_end END,
			no_show_at = CASE WHEN $1::text = $11 THEN $2 ELSE no_show_at END,
			cancelled_by = CASE WHEN $1::text = $12 THEN $5::text ELSE cancelled_by END,
			cancelled_at = CASE WHEN $1::text = $12 THEN $2 ELSE cancelled_at END,
			cancellation_reason = CASE WHEN $1::text IN ($11, $12) THEN $6::text ELSE cancellation_reason END,
			updated_at = $2
		WHERE booking_id = $3 AND ($7::text = $13 OR client_id = $4 OR therapist_id = $4)
	`, status, ts, bookingID, userID, cancelledBy, cancellationReason, role,
		model.BookingStatusArrived, model.BookingStatusInProgress, model.BookingStatusCompleted, model.BookingStatusNoShow, model.BookingStatusCancelled, model.RoleAdmin)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	actor := userID
	_ = r.insertBookingEvent(ctx, r.db, bookingID, status, &actor, bookingStatusEventMetadata(status, cancellationReason))
	return nil
}

func bookingStatusEventMetadata(status string, cancellationReason *string) map[string]any {
	if status == model.BookingStatusNoShow && cancellationReason != nil {
		return map[string]any{"reason": *cancellationReason, "status": status}
	}
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
		slog.Error("GetRecentTherapistStruggleFlags failed", "therapist_ids", therapistIDs, "since", since, "error", err)
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

func (r *bookingRepoImpl) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*BookingDetailsResult, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingDetailsFields + ` WHERE b.booking_id = $1 AND (b.client_id = $2 OR b.therapist_id = $2)`
	row := r.db.QueryRow(ctx, query, bookingID, userID)
	var result BookingDetailsResult
	if err := r.scanBookingDetails(row, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *bookingRepoImpl) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*BookingDetailsResult, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingDetailsFields + ` WHERE b.booking_id = $1`
	row := r.db.QueryRow(ctx, query, bookingID)
	var result BookingDetailsResult
	if err := r.scanBookingDetails(row, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *bookingRepoImpl) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*BookingDetailsResult, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingDetailsFields + ` WHERE b.reference_code = $1 AND (b.client_id = $2 OR b.therapist_id = $2)`
	row := r.db.QueryRow(ctx, query, referenceCode, userID)
	var result BookingDetailsResult
	if err := r.scanBookingDetails(row, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *bookingRepoImpl) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*BookingDetailsResult, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + selectBookingDetailsFields + ` WHERE b.reference_code = $1`
	row := r.db.QueryRow(ctx, query, referenceCode)
	var result BookingDetailsResult
	if err := r.scanBookingDetails(row, &result); err != nil {
		return nil, err
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
			COALESCE(b.gender_preference, 'any'), COALESCE(b.pressure_preference, 'medium'), COALESCE(b.notes, ''), b.duration_minutes,
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
			COALESCE(b.gender_preference, 'any'), COALESCE(b.pressure_preference, 'medium'), COALESCE(b.notes, ''), b.duration_minutes,
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
			COALESCE(b.gender_preference, 'any'), COALESCE(b.pressure_preference, 'medium'), COALESCE(b.notes, ''), b.duration_minutes,
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
			COALESCE(b.gender_preference, 'any'), COALESCE(b.pressure_preference, 'medium'), COALESCE(b.notes, ''), b.duration_minutes,
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

func (r *bookingRepoImpl) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]BookingDetailsResult, int, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var args []interface{}
	argCount := 1

	whereClauses := []string{"1=1"}

	if search != "" {
		searchTerm := "%" + search + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(b.reference_code ILIKE $%d OR client_u.full_name ILIKE $%d OR therapist_u.full_name ILIKE $%d)", argCount, argCount, argCount))
		args = append(args, searchTerm)
		argCount++
	}

	if status != "" && status != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("b.status = $%d", argCount))
		args = append(args, status)
		argCount++
	}

	whereClauseStr := strings.Join(whereClauses, " AND ")

	countQuery := `
		SELECT COUNT(*) 
		FROM bookings b
		LEFT JOIN users client_u ON b.client_id = client_u.user_id AND client_u.deleted_at IS NULL
		LEFT JOIN users therapist_u ON b.therapist_id = therapist_u.user_id AND therapist_u.deleted_at IS NULL
		WHERE ` + whereClauseStr

	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			-- Booking fields
			b.booking_id, b.reference_code, b.client_id, b.therapist_id, b.assigned_at, 
			b.service_id, b.address_id, b.promo_id, b.payment_method,
			COALESCE(b.gender_preference, 'any'), COALESCE(b.pressure_preference, 'medium'), COALESCE(b.notes, ''), b.duration_minutes,
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
		WHERE ` + whereClauseStr + `
		ORDER BY b.created_at DESC
		LIMIT $` + strconv.Itoa(argCount) + ` OFFSET $` + strconv.Itoa(argCount+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
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
		if sName != nil {
			service.Name = *sName
		}
		if sDesc != nil {
			service.Description = *sDesc
		}
		if sPrice != nil {
			service.BasePrice = *sPrice
		}
		if sDur != nil {
			service.DurationMinutes = *sDur
		}
		if sCat != nil {
			service.Category = *sCat
		}
		if sActive != nil {
			service.IsActive = *sActive
		} else {
			service.IsActive = true
		}
		if sImg != nil {
			service.PreviewImageURL = *sImg
		}

		// Populate Address
		if aID != nil {
			address.AddressID = *aID
		}
		if aLabel != nil {
			address.Label = *aLabel
		}
		if aStreet != nil {
			address.Street = *aStreet
		}
		if aCity != nil {
			address.City = *aCity
		}
		if aProv != nil {
			address.Province = *aProv
		}
		if aPostal != nil {
			address.PostalCode = *aPostal
		}
		if aCountry != nil {
			address.Country = *aCountry
		}
		if aLat != nil {
			address.Latitude = aLat
		}
		if aLng != nil {
			address.Longitude = aLng
		}
		if aDef != nil {
			address.IsDefault = *aDef
		}

		// Populate Result Details
		if cName != nil {
			result.ClientName = *cName
		}
		if cPhone != nil {
			result.ClientPhone = *cPhone
		}
		if cPhoto != nil {
			result.ClientPhoto = *cPhoto
		}
		if cGen != nil {
			result.ClientGender = *cGen
		}
		if tName != nil {
			result.TherapistName = *tName
		}
		if tPhone != nil {
			result.TherapistPhone = *tPhone
		}
		if tPhoto != nil {
			result.TherapistPhoto = *tPhoto
		}
		if tGen != nil {
			result.TherapistGender = *tGen
		}
		if pCode != nil {
			result.PromoCode = *pCode
		}

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

	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	// Batch fetch active rides
	if len(results) > 0 {
		bookingIDs := make([]int64, 0, len(results))
		for _, res := range results {
			if res.Booking != nil {
				bookingIDs = append(bookingIDs, res.Booking.BookingID)
			}
		}

		rideQuery := `
			SELECT ride_id, rider_id, passenger_id, booking_id, ride_type, pickup_lat, pickup_long, pickup_address, dropoff_lat, dropoff_long, dropoff_address, distance_km, status, scheduled_for, created_at, offered_at, accepted_at, arrived_at, started_at, completed_at, cancelled_at, cancellation_reason, retry_count, last_retried_at, updated_at
			FROM rides
			WHERE booking_id = ANY($1) 
			  AND status IN ('pending', 'offered', 'accepted', 'assigned', 'on_the_way', 'arrived', 'picked_up', 'in_progress')
		`
		rideRows, err := r.db.Query(ctx, rideQuery, bookingIDs)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to fetch active rides: %w", err)
		}
		defer rideRows.Close()

		activeRideMap := make(map[int64]*model.Ride)
		hatidRideMap := make(map[int64]*model.Ride)
		sundoRideMap := make(map[int64]*model.Ride)
		for rideRows.Next() {
			var ride model.Ride
			err := rideRows.Scan(
				&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID, &ride.RideType,
				&ride.PickupLat, &ride.PickupLong, &ride.PickupAddress, &ride.DropoffLat, &ride.DropoffLong, &ride.DropoffAddress,
				&ride.DistanceKm, &ride.Status, &ride.ScheduledFor, &ride.CreatedAt, &ride.OfferedAt,
				&ride.AcceptedAt, &ride.ArrivedAt, &ride.StartedAt, &ride.CompletedAt, &ride.CancelledAt,
				&ride.CancellationReason, &ride.RetryCount, &ride.LastRetriedAt, &ride.UpdatedAt,
			)
			if err != nil {
				continue
			}
			if ride.BookingID != nil {
				// Categorize ride type
				if ride.RideType == "outbound" {
					hatidRideMap[*ride.BookingID] = &ride
				} else if ride.RideType == "return" {
					sundoRideMap[*ride.BookingID] = &ride
				}

				// Keep ActiveRide mapping for backward compatibility and general status tracking
				// We prioritize rides that are currently ongoing ('on_the_way', 'arrived', 'picked_up', 'in_progress')
				isActiveStates := map[string]bool{"on_the_way": true, "arrived": true, "picked_up": true, "in_progress": true}

				existing, exists := activeRideMap[*ride.BookingID]
				if !exists {
					activeRideMap[*ride.BookingID] = &ride
				} else {
					if isActiveStates[ride.Status] && !isActiveStates[existing.Status] {
						activeRideMap[*ride.BookingID] = &ride
					}
				}
			}
		}

		for i := range results {
			if results[i].Booking != nil {
				results[i].ActiveRide = activeRideMap[results[i].Booking.BookingID]
				results[i].HatidRide = hatidRideMap[results[i].Booking.BookingID]
				results[i].SundoRide = sundoRideMap[results[i].Booking.BookingID]
			}
		}
	}

	return results, total, nil
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
		if sName != nil {
			service.Name = *sName
		}
		if sDesc != nil {
			service.Description = *sDesc
		}
		if sPrice != nil {
			service.BasePrice = *sPrice
		}
		if sDur != nil {
			service.DurationMinutes = *sDur
		}
		if sCat != nil {
			service.Category = *sCat
		}
		if sActive != nil {
			service.IsActive = *sActive
		} else {
			service.IsActive = true
		}
		if sImg != nil {
			service.PreviewImageURL = *sImg
		}

		// Populate Address
		if aID != nil {
			address.AddressID = *aID
		}
		if aLabel != nil {
			address.Label = *aLabel
		}
		if aStreet != nil {
			address.Street = *aStreet
		}
		if aCity != nil {
			address.City = *aCity
		}
		if aProv != nil {
			address.Province = *aProv
		}
		if aPostal != nil {
			address.PostalCode = *aPostal
		}
		if aCountry != nil {
			address.Country = *aCountry
		}
		if aLat != nil {
			address.Latitude = aLat
		}
		if aLng != nil {
			address.Longitude = aLng
		}
		if aDef != nil {
			address.IsDefault = *aDef
		}

		// Populate Result Details
		if cName != nil {
			result.ClientName = *cName
		}
		if cPhone != nil {
			result.ClientPhone = *cPhone
		}
		if cPhoto != nil {
			result.ClientPhoto = *cPhoto
		}
		if cGen != nil {
			result.ClientGender = *cGen
		}
		if tName != nil {
			result.TherapistName = *tName
		}
		if tPhone != nil {
			result.TherapistPhone = *tPhone
		}
		if tPhoto != nil {
			result.TherapistPhoto = *tPhoto
		}
		if tGen != nil {
			result.TherapistGender = *tGen
		}
		if pCode != nil {
			result.PromoCode = *pCode
		}

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
		if sName != nil {
			service.Name = *sName
		}
		if sDesc != nil {
			service.Description = *sDesc
		}
		if sPrice != nil {
			service.BasePrice = *sPrice
		}
		if sDur != nil {
			service.DurationMinutes = *sDur
		}
		if sCat != nil {
			service.Category = *sCat
		}
		if sActive != nil {
			service.IsActive = *sActive
		} else {
			service.IsActive = true
		}
		if sImg != nil {
			service.PreviewImageURL = *sImg
		}

		// Populate Address
		if aID != nil {
			address.AddressID = *aID
		}
		if aLabel != nil {
			address.Label = *aLabel
		}
		if aStreet != nil {
			address.Street = *aStreet
		}
		if aCity != nil {
			address.City = *aCity
		}
		if aProv != nil {
			address.Province = *aProv
		}
		if aPostal != nil {
			address.PostalCode = *aPostal
		}
		if aCountry != nil {
			address.Country = *aCountry
		}
		if aLat != nil {
			address.Latitude = aLat
		}
		if aLng != nil {
			address.Longitude = aLng
		}
		if aDef != nil {
			address.IsDefault = *aDef
		}

		// Populate Result Details
		if cName != nil {
			result.ClientName = *cName
		}
		if cPhone != nil {
			result.ClientPhone = *cPhone
		}
		if cPhoto != nil {
			result.ClientPhoto = *cPhoto
		}
		if cGen != nil {
			result.ClientGender = *cGen
		}
		if tName != nil {
			result.TherapistName = *tName
		}
		if tPhone != nil {
			result.TherapistPhone = *tPhone
		}
		if tPhoto != nil {
			result.TherapistPhoto = *tPhoto
		}
		if tGen != nil {
			result.TherapistGender = *tGen
		}
		if pCode != nil {
			result.PromoCode = *pCode
		}

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
		       COALESCE(b.gender_preference, 'any'), COALESCE(b.pressure_preference, 'medium'), COALESCE(b.notes, ''), b.duration_minutes,
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
func (r *bookingRepoImpl) UnassignTherapist(ctx context.Context, bookingID int64, actorID *int64, metadata map[string]any) error {
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

	// Log event
	_ = r.insertBookingEvent(ctx, r.db, bookingID, model.EventTypeUnassigned, actorID, metadata)

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

// GetByGroupID fetches all bookings belonging to a specific group.
func (r *bookingRepoImpl) GetByGroupID(ctx context.Context, groupID int64) ([]model.Booking, error) {
	query := `
		SELECT booking_id, reference_code, client_id, therapist_id, assigned_at, service_id, address_id, promo_id,
			   payment_method, COALESCE(gender_preference, 'any'), COALESCE(pressure_preference, 'medium'), COALESCE(notes, ''), duration_minutes,
			   scheduled_start, actual_start, actual_end, therapist_arrived_at, no_show_at, cancelled_by, cancelled_at, cancellation_reason,
			   raw_total, discount, final_total, status, created_at, updated_at, total_paused_seconds, current_pause_start, extension_wait_seconds,
			   group_id, COALESCE(guest_name, 'Self'), sequence_number, start_condition
		FROM bookings
		WHERE group_id = $1
		ORDER BY sequence_number ASC, booking_id ASC
	`
	rows, err := r.db.Query(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []model.Booking
	for rows.Next() {
		var b model.Booking
		err := rows.Scan(
			&b.BookingID, &b.ReferenceCode, &b.ClientID, &b.TherapistID, &b.AssignedAt,
			&b.ServiceID, &b.AddressID, &b.PromoID, &b.PaymentMethod,
			&b.GenderPref, &b.PressurePref, &b.Notes, &b.DurationMinutes,
			&b.ScheduledStart, &b.ActualStart, &b.ActualEnd, &b.TherapistArrivedAt, &b.NoShowAt,
			&b.CancelledBy, &b.CancelledAt, &b.CancellationReason,
			&b.RawTotal, &b.Discount, &b.FinalTotal, &b.Status,
			&b.CreatedAt, &b.UpdatedAt, &b.TotalPausedSeconds, &b.CurrentPauseStart, &b.ExtensionWaitSeconds,
			&b.GroupID, &b.GuestName, &b.SequenceNumber, &b.StartCondition,
		)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
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

func (r *bookingRepoImpl) GetByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]model.Booking, error) {
	if len(groupIDs) == 0 {
		return make(map[int64][]model.Booking), nil
	}

	query := `
		SELECT booking_id, client_id, therapist_id, service_id, address_id, promo_id,
		       status, raw_total, discount, final_total, payment_method,
		       scheduled_start, duration_minutes, COALESCE(notes, ''), 
		       COALESCE(gender_preference, 'any'), COALESCE(pressure_preference, 'medium'),
		       reference_code, created_at, updated_at, group_id, COALESCE(guest_name, 'Self'), sequence_number, start_condition
		FROM bookings
		WHERE group_id = ANY($1)
		ORDER BY group_id, sequence_number, booking_id
	`
	rows, err := r.db.Query(ctx, query, groupIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[int64][]model.Booking)
	for rows.Next() {
		var b model.Booking
		err := rows.Scan(
			&b.BookingID, &b.ClientID, &b.TherapistID, &b.ServiceID, &b.AddressID, &b.PromoID,
			&b.Status, &b.RawTotal, &b.Discount, &b.FinalTotal, &b.PaymentMethod,
			&b.ScheduledStart, &b.DurationMinutes, &b.Notes, &b.GenderPref, &b.PressurePref,
			&b.ReferenceCode, &b.CreatedAt, &b.UpdatedAt, &b.GroupID, &b.GuestName, &b.SequenceNumber, &b.StartCondition,
		)
		if err != nil {
			return nil, err
		}
		if b.GroupID != nil {
			res[*b.GroupID] = append(res[*b.GroupID], b)
		}
	}
	return res, rows.Err()
}

func getPooledTime(v *time.Time) time.Time {
	if v == nil {
		return time.Time{}
	}
	return *v
}
