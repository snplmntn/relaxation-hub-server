package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// LiveLocationRepository manages live location tracking.
type LiveLocationRepository interface {
	Upsert(ctx context.Context, loc *model.LiveLocation) error
	GetByUserID(ctx context.Context, userID int64) (*model.LiveLocation, error)
}

type liveLocationRepoImpl struct {
	db db.DBTX
}

func NewLiveLocationRepository(db db.DBTX) LiveLocationRepository {
	return &liveLocationRepoImpl{db: db}
}

func (r *liveLocationRepoImpl) Upsert(ctx context.Context, loc *model.LiveLocation) error {
	query := `
        INSERT INTO live_locations (user_id, latitude, longitude, last_updated)
        VALUES ($1,$2,$3,CURRENT_TIMESTAMP)
        ON CONFLICT (user_id)
        DO UPDATE SET latitude = EXCLUDED.latitude,
                      longitude = EXCLUDED.longitude,
                      last_updated = CURRENT_TIMESTAMP
        RETURNING location_id, last_updated
    `
	return r.db.QueryRow(ctx, query,
		loc.UserID,
		loc.Latitude,
		loc.Longitude,
	).Scan(&loc.LocationID, &loc.LastUpdated)
}

func (r *liveLocationRepoImpl) GetByUserID(ctx context.Context, userID int64) (*model.LiveLocation, error) {
	query := `
        SELECT location_id, user_id, latitude, longitude, last_updated
        FROM live_locations
        WHERE user_id = $1
    `
	var loc model.LiveLocation
	if err := r.db.QueryRow(ctx, query, userID).Scan(
		&loc.LocationID,
		&loc.UserID,
		&loc.Latitude,
		&loc.Longitude,
		&loc.LastUpdated,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}
	return &loc, nil
}
