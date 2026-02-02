package service

import (
	"context"
	"fmt"

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
func (s *RideMatchingService) FindNearbyRiders(ctx context.Context, lat, long float64, radiusKm float64) ([]model.RiderProfile, error) {
	// SOTA 2026: efficient KNN using <-> operator and ST_DWithin filter
	query := `
		SELECT 
			rider_id, user_id, vehicle_type, license_plate, is_online, rating, total_trips,
			ST_Y(current_location::geometry) as current_lat,
			ST_X(current_location::geometry) as current_long
		FROM rider_profiles
		WHERE is_online = true
		  AND ST_DWithin(
				current_location, 
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 
				$3 * 1000 -- Convert km to meters
			  )
		ORDER BY 
		  current_location <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		LIMIT 10;
	`

	rows, err := s.db.Query(ctx, query, long, lat, radiusKm) // Note: PostGIS is (lng, lat) usually for MakePoint
	if err != nil {
		return nil, fmt.Errorf("querying nearby riders: %w", err)
	}
	defer rows.Close()

	var riders []model.RiderProfile
	for rows.Next() {
		var r model.RiderProfile
		var cLat, cLong float64
		
		// Scan fields manually for pgx
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
