package repository

import (
	"context"
	"errors"

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
	AssignRider(ctx context.Context, rideID, riderID int64) error
	GetPendingRides(ctx context.Context) ([]model.Ride, error)
	GetRidesForRiderByStatus(ctx context.Context, riderID int64, status string) ([]model.Ride, error)
	GetAvailableRidesNear(ctx context.Context, lat, long, radiusKm float64) ([]model.Ride, error)
	GetRiderProfile(ctx context.Context, userID int64) (*model.RiderProfile, error)
	CreateRiderProfile(ctx context.Context, userID int64, vehicleType, licensePlate string) error
	UpdateRiderLocation(ctx context.Context, riderID int64, lat, long float64) error
	GetActiveRideByRiderID(ctx context.Context, riderID int64) (*model.Ride, error)
	UpdateRiderStatus(ctx context.Context, riderID int64, isOnline bool) error
	GetRideByBookingID(ctx context.Context, bookingID int64) (*model.Ride, error)
	GetProfileByRiderID(ctx context.Context, riderID int64) (*model.RiderProfile, error)
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
			passenger_id, pickup_lat, pickup_long, pickup_address,
			dropoff_lat, dropoff_long, dropoff_address,
			distance_km, status, pricing_snapshot
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
		RETURNING ride_id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		ride.PassengerID, ride.PickupLat, ride.PickupLong, ride.PickupAddress,
		ride.DropoffLat, ride.DropoffLong, ride.DropoffAddress,
		ride.DistanceKm, ride.Status, ride.PricingSnapshot,
	).Scan(&ride.RideID, &ride.CreatedAt, &ride.UpdatedAt)
}

func (r *rideRepoImpl) GetByID(ctx context.Context, rideID int64) (*model.Ride, error) {
	query := `
		SELECT 
			ride_id, rider_id, passenger_id, booking_id,
			pickup_lat, pickup_long, pickup_address,
			dropoff_lat, dropoff_long, dropoff_address,
			distance_km, pricing_snapshot, status,
			created_at, accepted_at, started_at, completed_at, cancelled_at
		FROM rides
		WHERE ride_id = $1
	`
	var ride model.Ride
	err := r.db.QueryRow(ctx, query, rideID).Scan(
		&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID,
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

func (r *rideRepoImpl) UpdateStatus(ctx context.Context, rideID int64, status string) error {
	query := `UPDATE rides SET status = $1, updated_at = NOW() WHERE ride_id = $2`
	_, err := r.db.Exec(ctx, query, status, rideID)
	return err
}

func (r *rideRepoImpl) AssignRider(ctx context.Context, rideID, riderID int64) error {
	query := `UPDATE rides SET rider_id = $1, status = 'offered', offered_at = NOW(), updated_at = NOW() WHERE ride_id = $2`
	_, err := r.db.Exec(ctx, query, riderID, rideID)
	return err
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
			ride_id, rider_id, passenger_id, booking_id,
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
			&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID,
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
			ride_id, rider_id, passenger_id, booking_id,
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
			&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID,
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
		SELECT rider_id, user_id, vehicle_type, license_plate, is_online, rating, total_trips
		FROM rider_profiles WHERE user_id = $1
	`
	var p model.RiderProfile
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&p.RiderID, &p.UserID, &p.VehicleType, &p.LicensePlate, &p.IsOnline, &p.Rating, &p.TotalTrips,
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
	`
	_, err := r.db.Exec(ctx, query, userID, vehicleType, licensePlate)
	return err
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
			ride_id, booking_id, passenger_id, rider_id, status,
			pickup_lat, pickup_long, pickup_address,
			dropoff_lat, dropoff_long, dropoff_address,
			distance_km, pricing_snapshot, created_at, updated_at
		FROM rides
		WHERE rider_id = $1 AND status IN ('accepted', 'arrived_pickup', 'in_progress')
		LIMIT 1 -- Assuming only 1 active ride per rider
	`
	var ride model.Ride
	err := r.db.QueryRow(ctx, query, riderID).Scan(
		&ride.RideID, &ride.BookingID, &ride.PassengerID, &ride.RiderID, &ride.Status,
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
			ride_id, rider_id, passenger_id, booking_id,
			pickup_lat, pickup_long, pickup_address,
			dropoff_lat, dropoff_long, dropoff_address,
			distance_km, pricing_snapshot, status,
			created_at, accepted_at, started_at, completed_at, cancelled_at
		FROM rides
		WHERE booking_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var ride model.Ride
	err := r.db.QueryRow(ctx, query, bookingID).Scan(
		&ride.RideID, &ride.RiderID, &ride.PassengerID, &ride.BookingID,
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
		SELECT rider_id, user_id, vehicle_type, license_plate, is_online, rating, total_trips
		FROM rider_profiles WHERE rider_id = $1
	`
	var p model.RiderProfile
	err := r.db.QueryRow(ctx, query, riderID).Scan(
		&p.RiderID, &p.UserID, &p.VehicleType, &p.LicensePlate, &p.IsOnline, &p.Rating, &p.TotalTrips,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
