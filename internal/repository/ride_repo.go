package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

var (
	ErrRideNotFound = errors.New("ride not found")
)

type RideRepository interface {
	Create(ctx context.Context, ride *model.Ride) error
	GetByID(ctx context.Context, rideID int64) (*model.Ride, error)
	UpdateStatus(ctx context.Context, rideID int64, status string) error
	UpdateStatusForRider(ctx context.Context, rideID, riderID int64, status string) error
	AssignRider(ctx context.Context, rideID, riderID int64) error
	// ClaimRide atomically locks ride row, verifies it's pending, assigns rider, and sets 'accepted'.
	// Returns ErrRideNotFound if ride doesn't exist, or error if ride is no longer available.
	ClaimRide(ctx context.Context, rideID, riderID int64) error
	GetPendingRides(ctx context.Context) ([]model.Ride, error)
	GetRidesForRiderByStatus(ctx context.Context, riderID int64, status string) ([]model.Ride, error)
	GetRidesForRider(ctx context.Context, riderID int64, status string, limit, offset int) ([]model.Ride, error)
	GetAvailableRidesNear(ctx context.Context, lat, long, radiusKm float64) ([]model.Ride, error)
	GetRiderProfile(ctx context.Context, userID int64) (*model.RiderProfile, error)
	CreateRiderProfile(ctx context.Context, userID int64, vehicleType, licensePlate string) error
	UpdateRiderProfile(ctx context.Context, riderID int64, updates map[string]interface{}) error
	UpdateRiderLocation(ctx context.Context, riderID int64, lat, long float64) error
	GetActiveRideByRiderID(ctx context.Context, riderID int64) (*model.Ride, error)
	UpdateRiderStatus(ctx context.Context, riderID int64, isOnline bool) error
	GetRideByBookingID(ctx context.Context, bookingID int64) (*model.Ride, error)
	GetRidesByBookingID(ctx context.Context, bookingID int64) ([]model.Ride, error)
	CancelRide(ctx context.Context, rideID int64) error
	GetProfileByRiderID(ctx context.Context, riderID int64) (*model.RiderProfile, error)
	UnassignRider(ctx context.Context, rideID int64) error
	IncrementRetry(ctx context.Context, rideID int64) error
	GetUnmatchedRidesForRetry(ctx context.Context, backoffMinutes int, maxRetries int) ([]model.Ride, error)
}

type rideRepoImpl struct {
	db db.DBTX // wrapper interface usually
}

func NewRideRepository(db db.DBTX) RideRepository {
	return &rideRepoImpl{db: db}
}

func (r *rideRepoImpl) Create(ctx context.Context, ride *model.Ride) error {
	query := `
		INSERT INTO rides (
			passenger_id, booking_id, ride_type,
			pickup_lat, pickup_long, pickup_address,
			dropoff_lat, dropoff_long, dropoff_address,
			distance_km, status, scheduled_for, pricing_snapshot
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		RETURNING ride_id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		ride.PassengerID, ride.BookingID, ride.RideType,
		ride.PickupLat, ride.PickupLong, ride.PickupAddress,
		ride.DropoffLat, ride.DropoffLong, ride.DropoffAddress,
		ride.DistanceKm, ride.Status, ride.ScheduledFor, ride.PricingSnapshot,
	).Scan(&ride.RideID, &ride.CreatedAt, &ride.UpdatedAt)
}

func (r *rideRepoImpl) GetByID(ctx context.Context, rideID int64) (*model.Ride, error) {
	query := `
		SELECT 
			ride_id, rider_id, passenger_id, booking_id, ride_type,
			pickup_lat, pickup_long, pickup_address,
			dropoff_lat, dropoff_long, dropoff_address,
			distance_km, pricing_snapshot, status,
			created_at, accepted_at, started_at, completed_at, cancelled_at
		FROM rides
		WHERE ride_id = $1
	`
	var ride model.Ride
	err := r.db.QueryRow(ctx, query, rideID).Scan(
		&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID, &ride.RideType,
		&ride.PickupLat, &ride.PickupLong, &ride.PickupAddress,
		&ride.DropoffLat, &ride.DropoffLong, &ride.DropoffAddress,
		&ride.DistanceKm, &ride.PricingSnapshot, &ride.Status,
		&ride.CreatedAt, &ride.AcceptedAt, &ride.StartedAt, &ride.CompletedAt, &ride.CancelledAt,
		&ride.RiderName, &ride.RiderPhone, &ride.VehicleType, &ride.LicensePlate,
	)
	if err != nil {
		return nil, err
	}
	return &ride, nil
}

func (r *rideRepoImpl) UpdateStatus(ctx context.Context, rideID int64, status string) error {
	query := `UPDATE rides SET status = $1, updated_at = NOW() WHERE ride_id = $2`
	_, err := r.db.Exec(ctx, query, status, rideID)
	return err
}

func (r *rideRepoImpl) UpdateStatusForRider(ctx context.Context, rideID, riderID int64, status string) error {
	query := `UPDATE rides SET status = $1, updated_at = NOW() WHERE ride_id = $2 AND rider_id = $3`
	cmd, err := r.db.Exec(ctx, query, status, rideID, riderID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrRideNotFound
	}
	return nil
}

func (r *rideRepoImpl) AssignRider(ctx context.Context, rideID, riderID int64) error {
	query := `UPDATE rides SET rider_id = $1, status = 'offered', offered_at = NOW(), updated_at = NOW() WHERE ride_id = $2`
	_, err := r.db.Exec(ctx, query, riderID, rideID)
	return err
}

// ClaimRide atomically locks the ride row with FOR UPDATE, verifies it's still
// pending (or offered to this rider), then assigns the rider and sets 'accepted'.
// This prevents the race condition where two riders accept simultaneously.
func (r *rideRepoImpl) ClaimRide(ctx context.Context, rideID, riderID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the row and fetch current state
	var status string
	var currentRiderID *int64
	err = tx.QueryRow(ctx, `
		SELECT status, rider_id FROM rides WHERE ride_id = $1 FOR UPDATE
	`, rideID).Scan(&status, &currentRiderID)
	if err != nil {
		return ErrRideNotFound
	}

	// Verify ride is claimable
	switch status {
	case "pending":
		// Open for anyone — proceed
	case "offered":
		// Only the assigned rider can claim
		if currentRiderID == nil || *currentRiderID != riderID {
			return errors.New("ride no longer available")
		}
	default:
		return errors.New("ride no longer available")
	}

	// Assign and accept atomically
	_, err = tx.Exec(ctx, `
		UPDATE rides 
		SET rider_id = $1, status = 'accepted', accepted_at = NOW(), updated_at = NOW() 
		WHERE ride_id = $2
	`, riderID, rideID)
	if err != nil {
		return fmt.Errorf("claim ride: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *rideRepoImpl) GetPendingRides(ctx context.Context) ([]model.Ride, error) {
	query := `SELECT ride_id, passenger_id, pickup_address, status FROM rides WHERE status = 'pending' ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rides []model.Ride
	for rows.Next() {
		var ride model.Ride
		if err := rows.Scan(&ride.RideID, &ride.PassengerID, &ride.PickupAddress, &ride.Status); err != nil {
			return nil, err
		}
		rides = append(rides, ride)
	}
	return rides, nil
}

func (r *rideRepoImpl) GetRidesForRiderByStatus(ctx context.Context, riderID int64, status string) ([]model.Ride, error) {
	query := `
		SELECT 
			ride_id, rider_id, passenger_id, booking_id, ride_type,
			pickup_lat, pickup_long, pickup_address,
			dropoff_lat, dropoff_long, dropoff_address,
			distance_km, pricing_snapshot, status,
			created_at, accepted_at, started_at, completed_at, cancelled_at
		FROM rides
		WHERE rider_id = $1 AND status = $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, riderID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rides []model.Ride
	for rows.Next() {
		var ride model.Ride
		if err := rows.Scan(
			&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID, &ride.RideType,
			&ride.PickupLat, &ride.PickupLong, &ride.PickupAddress,
			&ride.DropoffLat, &ride.DropoffLong, &ride.DropoffAddress,
			&ride.DistanceKm, &ride.PricingSnapshot, &ride.Status,
			&ride.CreatedAt, &ride.AcceptedAt, &ride.StartedAt, &ride.CompletedAt, &ride.CancelledAt,
		); err != nil {
			return nil, err
		}
		rides = append(rides, ride)
	}
	return rides, rows.Err()
}

func (r *rideRepoImpl) GetRidesForRider(ctx context.Context, riderID int64, status string, limit, offset int) ([]model.Ride, error) {
	var query string
	args := []interface{}{riderID, limit, offset}

	if status == "" {
		query = `
			SELECT
				ride_id, rider_id, passenger_id, booking_id, ride_type,
				pickup_lat, pickup_long, pickup_address,
				dropoff_lat, dropoff_long, dropoff_address,
				distance_km, pricing_snapshot, status,
				created_at, accepted_at, started_at, completed_at, cancelled_at
			FROM rides
			WHERE rider_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
	} else {
		query = `
			SELECT
				ride_id, rider_id, passenger_id, booking_id, ride_type,
				pickup_lat, pickup_long, pickup_address,
				dropoff_lat, dropoff_long, dropoff_address,
				distance_km, pricing_snapshot, status,
				created_at, accepted_at, started_at, completed_at, cancelled_at
			FROM rides
			WHERE rider_id = $1 AND status = $2
			ORDER BY created_at DESC
			LIMIT $3 OFFSET $4
		`
		args = []interface{}{riderID, status, limit, offset}
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rides []model.Ride
	for rows.Next() {
		var ride model.Ride
		if err := rows.Scan(
			&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID, &ride.RideType,
			&ride.PickupLat, &ride.PickupLong, &ride.PickupAddress,
			&ride.DropoffLat, &ride.DropoffLong, &ride.DropoffAddress,
			&ride.DistanceKm, &ride.PricingSnapshot, &ride.Status,
			&ride.CreatedAt, &ride.AcceptedAt, &ride.StartedAt, &ride.CompletedAt, &ride.CancelledAt,
		); err != nil {
			return nil, err
		}
		rides = append(rides, ride)
	}
	return rides, rows.Err()
}

func (r *rideRepoImpl) GetAvailableRidesNear(ctx context.Context, lat, long, radiusKm float64) ([]model.Ride, error) {
	// Find pending rides within radius
	query := `
		SELECT 
			ride_id, rider_id, passenger_id, booking_id, ride_type,
			pickup_lat, pickup_long, pickup_address,
			dropoff_lat, dropoff_long, dropoff_address,
			distance_km, pricing_snapshot, status,
			created_at, accepted_at, started_at, completed_at, cancelled_at
		FROM rides
		WHERE status = 'pending'
		  AND ST_DWithin(
				ST_SetSRID(ST_MakePoint(pickup_long, pickup_lat), 4326)::geography, 
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 
				$3 * 1000
			  )
		ORDER BY created_at ASC
	`
	// Note: MakePoint(long, lat)
	rows, err := r.db.Query(ctx, query, long, lat, radiusKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rides []model.Ride
	for rows.Next() {
		var ride model.Ride
		if err := rows.Scan(
			&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID, &ride.RideType,
			&ride.PickupLat, &ride.PickupLong, &ride.PickupAddress,
			&ride.DropoffLat, &ride.DropoffLong, &ride.DropoffAddress,
			&ride.DistanceKm, &ride.PricingSnapshot, &ride.Status,
			&ride.CreatedAt, &ride.AcceptedAt, &ride.StartedAt, &ride.CompletedAt, &ride.CancelledAt,
		); err != nil {
			return nil, err
		}
		rides = append(rides, ride)
	}
	return rides, rows.Err()
}

func (r *rideRepoImpl) GetRiderProfile(ctx context.Context, userID int64) (*model.RiderProfile, error) {
	query := `
		SELECT rider_id, user_id, vehicle_type, license_plate, is_online, rating, total_trips,
			usual_branch_id, usual_location_label
		FROM rider_profiles WHERE user_id = $1
	`
	var p model.RiderProfile
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&p.RiderID, &p.UserID, &p.VehicleType, &p.LicensePlate, &p.IsOnline, &p.Rating, &p.TotalTrips,
		&p.UsualBranchID, &p.UsualLocationLabel,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *rideRepoImpl) CreateRiderProfile(ctx context.Context, userID int64, vehicleType, licensePlate string) error {
	query := `
		INSERT INTO rider_profiles (user_id, vehicle_type, license_plate, is_online, created_at, updated_at)
		VALUES ($1, $2, $3, false, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			vehicle_type = EXCLUDED.vehicle_type,
			license_plate = EXCLUDED.license_plate,
			updated_at = NOW()
	`
	_, err := r.db.Exec(ctx, query, userID, vehicleType, licensePlate)
	return err
}

func (r *rideRepoImpl) UpdateRiderProfile(ctx context.Context, riderID int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Dynamically build query
	query := "UPDATE rider_profiles SET "
	args := []interface{}{}
	i := 1
	for k, v := range updates {
		if !isAllowedRiderProfileUpdateColumn(k) {
			return fmt.Errorf("invalid rider profile update field: %s", k)
		}
		if i > 1 {
			query += ", "
		}
		query += fmt.Sprintf("%s = $%d", k, i)
		args = append(args, v)
		i++
	}
	query += fmt.Sprintf(", updated_at = NOW() WHERE rider_id = $%d", i)
	args = append(args, riderID)

	_, err := r.db.Exec(ctx, query, args...)
	return err
}

func isAllowedRiderProfileUpdateColumn(column string) bool {
	switch column {
	case "vehicle_type", "license_plate", "license_number", "usual_branch_id", "usual_location_label":
		return true
	default:
		return false
	}
}

func (r *rideRepoImpl) UpdateRiderLocation(ctx context.Context, riderID int64, lat, long float64) error {
	// Update geospatial location and timestamp
	query := `
		UPDATE rider_profiles 
		SET current_location = ST_SetSRID(ST_MakePoint($2, $3), 4326), 
		    last_location_update = NOW(),
			updated_at = NOW()
		WHERE rider_id = $1
	`
	_, err := r.db.Exec(ctx, query, riderID, long, lat) // Note: MakePoint(long, lat) order
	return err
}

func (r *rideRepoImpl) GetActiveRideByRiderID(ctx context.Context, riderID int64) (*model.Ride, error) {
	query := `
		SELECT 
			ride_id, booking_id, passenger_id, rider_id, ride_type, status,
			pickup_lat, pickup_long, pickup_address,
			dropoff_lat, dropoff_long, dropoff_address,
			distance_km, pricing_snapshot, created_at, updated_at
		FROM rides
		WHERE rider_id = $1 AND status IN ('accepted', 'arrived_pickup', 'in_progress', 'arrived_dropoff')
		LIMIT 1 -- Assuming only 1 active ride per rider
	`
	var ride model.Ride
	err := r.db.QueryRow(ctx, query, riderID).Scan(
		&ride.RideID, &ride.BookingID, &ride.PassengerID, &ride.RiderID, &ride.RideType, &ride.Status,
		&ride.PickupLat, &ride.PickupLong, &ride.PickupAddress,
		&ride.DropoffLat, &ride.DropoffLong, &ride.DropoffAddress,
		&ride.DistanceKm, &ride.PricingSnapshot, &ride.CreatedAt, &ride.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ride, nil
}

func (r *rideRepoImpl) UpdateRiderStatus(ctx context.Context, riderID int64, isOnline bool) error {
	query := `UPDATE rider_profiles SET is_online = $1, updated_at = NOW() WHERE rider_id = $2`
	_, err := r.db.Exec(ctx, query, isOnline, riderID)
	return err
}

func (r *rideRepoImpl) GetRideByBookingID(ctx context.Context, bookingID int64) (*model.Ride, error) {
	query := `
		SELECT
			r.ride_id, r.rider_id, r.passenger_id, r.booking_id, r.ride_type,
			r.pickup_lat, r.pickup_long, r.pickup_address,
			r.dropoff_lat, r.dropoff_long, r.dropoff_address,
			r.distance_km, r.pricing_snapshot, r.status,
			r.created_at, r.accepted_at, r.started_at, r.completed_at, r.cancelled_at,
			COALESCE(u.full_name, ''), COALESCE(u.primary_phone, ''),
			COALESCE(rp.vehicle_type, ''), COALESCE(rp.license_plate, '')
		FROM rides r
		LEFT JOIN rider_profiles rp ON r.rider_id = rp.rider_id
		LEFT JOIN users u ON rp.user_id = u.user_id AND u.deleted_at IS NULL
		WHERE r.booking_id = $1
		ORDER BY r.created_at DESC
		LIMIT 1
	`
	var ride model.Ride
	err := r.db.QueryRow(ctx, query, bookingID).Scan(
		&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID, &ride.RideType,
		&ride.PickupLat, &ride.PickupLong, &ride.PickupAddress,
		&ride.DropoffLat, &ride.DropoffLong, &ride.DropoffAddress,
		&ride.DistanceKm, &ride.PricingSnapshot, &ride.Status,
		&ride.CreatedAt, &ride.AcceptedAt, &ride.StartedAt, &ride.CompletedAt, &ride.CancelledAt,
	)
	if err != nil {
		return nil, err
	}
	return &ride, nil
}

func (r *rideRepoImpl) GetProfileByRiderID(ctx context.Context, riderID int64) (*model.RiderProfile, error) {
	query := `
		SELECT rider_id, user_id, vehicle_type, license_plate, is_online, rating, total_trips,
			usual_branch_id, usual_location_label
		FROM rider_profiles WHERE rider_id = $1
	`
	var p model.RiderProfile
	err := r.db.QueryRow(ctx, query, riderID).Scan(
		&p.RiderID, &p.UserID, &p.VehicleType, &p.LicensePlate, &p.IsOnline, &p.Rating, &p.TotalTrips,
		&p.UsualBranchID, &p.UsualLocationLabel,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *rideRepoImpl) GetRidesByBookingID(ctx context.Context, bookingID int64) ([]model.Ride, error) {
	query := `
		SELECT
			r.ride_id, r.rider_id, r.passenger_id, r.booking_id, r.ride_type,
			r.pickup_lat, r.pickup_long, r.pickup_address,
			r.dropoff_lat, r.dropoff_long, r.dropoff_address,
			r.distance_km, r.pricing_snapshot, r.status,
			r.created_at, r.accepted_at, r.started_at, r.completed_at, r.cancelled_at,
			COALESCE(u.full_name, ''), COALESCE(u.primary_phone, ''),
			COALESCE(rp.vehicle_type, ''), COALESCE(rp.license_plate, '')
		FROM rides r
		LEFT JOIN rider_profiles rp ON r.rider_id = rp.rider_id
		LEFT JOIN users u ON rp.user_id = u.user_id AND u.deleted_at IS NULL
		WHERE r.booking_id = $1
		ORDER BY r.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rides []model.Ride
	for rows.Next() {
		var ride model.Ride
		if err := rows.Scan(
			&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID, &ride.RideType,
			&ride.PickupLat, &ride.PickupLong, &ride.PickupAddress,
			&ride.DropoffLat, &ride.DropoffLong, &ride.DropoffAddress,
			&ride.DistanceKm, &ride.PricingSnapshot, &ride.Status,
			&ride.CreatedAt, &ride.AcceptedAt, &ride.StartedAt, &ride.CompletedAt, &ride.CancelledAt,
			&ride.RiderName, &ride.RiderPhone, &ride.VehicleType, &ride.LicensePlate,
		); err != nil {
			return nil, err
		}
		rides = append(rides, ride)
	}
	return rides, rows.Err()
}

func (r *rideRepoImpl) CancelRide(ctx context.Context, rideID int64) error {
	query := `UPDATE rides SET status = 'cancelled', cancelled_at = NOW(), updated_at = NOW() WHERE ride_id = $1`
	_, err := r.db.Exec(ctx, query, rideID)
	return err
}

func (r *rideRepoImpl) UnassignRider(ctx context.Context, rideID int64) error {
	query := `UPDATE rides SET rider_id = NULL, status = 'pending', accepted_at = NULL, offered_at = NULL, updated_at = NOW() WHERE ride_id = $1`
	_, err := r.db.Exec(ctx, query, rideID)
	return err
}

func (r *rideRepoImpl) IncrementRetry(ctx context.Context, rideID int64) error {
	query := `UPDATE rides SET retry_count = retry_count + 1, last_retried_at = NOW(), updated_at = NOW() WHERE ride_id = $1`
	_, err := r.db.Exec(ctx, query, rideID)
	return err
}

// GetUnmatchedRidesForRetry returns pending rides with no active offers that are eligible for retry.
// Respects backoff (last_retried_at older than backoffMinutes) and max retries.
func (r *rideRepoImpl) GetUnmatchedRidesForRetry(ctx context.Context, backoffMinutes int, maxRetries int) ([]model.Ride, error) {
	query := `
		SELECT r.ride_id, r.rider_id, r.passenger_id, r.booking_id, r.ride_type,
			r.pickup_lat, r.pickup_long, r.pickup_address,
			r.dropoff_lat, r.dropoff_long, r.dropoff_address,
			r.distance_km, r.status, r.scheduled_for,
			r.retry_count, r.last_retried_at, r.created_at, r.updated_at
		FROM rides r
		WHERE r.status = 'pending'
		  AND r.rider_id IS NULL
		  AND r.retry_count < $1
		  AND (
			r.last_retried_at IS NULL 
			OR r.last_retried_at < NOW() - (interval '1 minute' * $2)
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM ride_offers ro
			WHERE ro.ride_id = r.ride_id AND ro.status = 'pending'
		  )
		ORDER BY r.created_at ASC
		LIMIT 20
	`
	rows, err := r.db.Query(ctx, query, maxRetries, backoffMinutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rides []model.Ride
	for rows.Next() {
		var ride model.Ride
		if err := rows.Scan(
			&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID, &ride.RideType,
			&ride.PickupLat, &ride.PickupLong, &ride.PickupAddress,
			&ride.DropoffLat, &ride.DropoffLong, &ride.DropoffAddress,
			&ride.DistanceKm, &ride.Status, &ride.ScheduledFor,
			&ride.RetryCount, &ride.LastRetriedAt, &ride.CreatedAt, &ride.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rides = append(rides, ride)
	}
	return rides, rows.Err()
}
