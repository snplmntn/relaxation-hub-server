package service

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type RideMatchingService struct {
	db db.DBTX
}

func NewRideMatchingService(db db.DBTX) *RideMatchingService {
	return &RideMatchingService{db: db}
}

// FindNearbyRiders uses PostGIS ST_DWithin for efficient K-nearest-neighbor search.
// lat/long should be decimal degrees. radiusKm is the search radius.
// If scheduledFor is non-nil, riders with an active ride overlapping ±1 hour are excluded.
func (s *RideMatchingService) FindNearbyRiders(ctx context.Context, lat, long float64, radiusKm float64, scheduledFor *time.Time) ([]model.RiderProfile, error) {
	// Base query: online riders within radius
	baseQuery := `
		SELECT 
			rp.rider_id, rp.user_id, rp.vehicle_type, rp.license_plate, rp.is_online, rp.rating, rp.total_trips,
			ST_Y(rp.current_location::geometry) as current_lat,
			ST_X(rp.current_location::geometry) as current_long
		FROM rider_profiles rp
		WHERE rp.is_online = true
		  AND ST_DWithin(
				rp.current_location, 
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 
				$3 * 1000
			  )
	`

	var args []any
	args = append(args, long, lat, radiusKm) // PostGIS: (lng, lat)

	// Schedule-aware: exclude riders with overlapping active rides
	if scheduledFor != nil {
		baseQuery += `
		  AND NOT EXISTS (
			SELECT 1 FROM rides r
			WHERE r.rider_id = rp.rider_id
			  AND r.status IN ('accepted', 'in_progress', 'arrived')
			  AND r.scheduled_for IS NOT NULL
			  AND r.scheduled_for BETWEEN $4 AND $5
		  )
		`
		windowStart := scheduledFor.Add(-1 * time.Hour)
		windowEnd := scheduledFor.Add(1 * time.Hour)
		args = append(args, windowStart, windowEnd)
	}

	baseQuery += `
		ORDER BY 
		  rp.current_location <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		LIMIT 10;
	`

	rows, err := s.db.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("querying nearby riders: %w", err)
	}
	defer rows.Close()

	var riders []model.RiderProfile
	for rows.Next() {
		var r model.RiderProfile
		var cLat, cLong float64

		if err := rows.Scan(
			&r.RiderID, &r.UserID, &r.VehicleType, &r.LicensePlate, &r.IsOnline, &r.Rating, &r.TotalTrips,
			&cLat, &cLong,
		); err != nil {
			return nil, fmt.Errorf("scanning rider row: %w", err)
		}
		r.CurrentLat = &cLat
		r.CurrentLong = &cLong
		riders = append(riders, r)
	}

	return riders, nil
}
