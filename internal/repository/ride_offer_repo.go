package repository

import (
	"context"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// RideOfferRepository manages ride_offers lifecycle.
type RideOfferRepository interface {
	Create(ctx context.Context, offer *model.RideOffer) error
	GetActiveByRideID(ctx context.Context, rideID int64) ([]model.RideOffer, error)
	GetActiveForRider(ctx context.Context, riderID int64) ([]model.RideOffer, error)
	GetByRiderAndRide(ctx context.Context, riderID, rideID int64) (*model.RideOffer, error)
	UpdateStatus(ctx context.Context, offerID int64, status string) error
	DeclineOffer(ctx context.Context, offerID int64) error
	ExpireStaleOffers(ctx context.Context) ([]model.RideOffer, error)
	ExpireOffersForRide(ctx context.Context, rideID int64) ([]model.RideOffer, error)
}

type rideOfferRepoImpl struct {
	db db.DBTX
}

func NewRideOfferRepository(database db.DBTX) RideOfferRepository {
	return &rideOfferRepoImpl{db: database}
}

func (r *rideOfferRepoImpl) Create(ctx context.Context, offer *model.RideOffer) error {
	query := `
		INSERT INTO ride_offers (ride_id, rider_id, status, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (ride_id, rider_id) DO NOTHING
		RETURNING offer_id, created_at
	`
	return r.db.QueryRow(ctx, query,
		offer.RideID, offer.RiderID, model.RideOfferStatusPending, offer.ExpiresAt,
	).Scan(&offer.OfferID, &offer.CreatedAt)
}

func (r *rideOfferRepoImpl) GetActiveByRideID(ctx context.Context, rideID int64) ([]model.RideOffer, error) {
	query := `
		SELECT offer_id, ride_id, rider_id, status, created_at, expires_at, responded_at
		FROM ride_offers
		WHERE ride_id = $1 AND status = 'pending' AND expires_at > NOW()
		ORDER BY created_at ASC
	`
	return r.scanOffers(ctx, query, rideID)
}

func (r *rideOfferRepoImpl) GetActiveForRider(ctx context.Context, riderID int64) ([]model.RideOffer, error) {
	query := `
		SELECT offer_id, ride_id, rider_id, status, created_at, expires_at, responded_at
		FROM ride_offers
		WHERE rider_id = $1 AND status = 'pending' AND expires_at > NOW()
		ORDER BY created_at DESC
	`
	return r.scanOffers(ctx, query, riderID)
}

func (r *rideOfferRepoImpl) GetByRiderAndRide(ctx context.Context, riderID, rideID int64) (*model.RideOffer, error) {
	query := `
		SELECT offer_id, ride_id, rider_id, status, created_at, expires_at, responded_at
		FROM ride_offers
		WHERE rider_id = $1 AND ride_id = $2
	`
	var o model.RideOffer
	err := r.db.QueryRow(ctx, query, riderID, rideID).Scan(
		&o.OfferID, &o.RideID, &o.RiderID, &o.Status, &o.CreatedAt, &o.ExpiresAt, &o.RespondedAt,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *rideOfferRepoImpl) UpdateStatus(ctx context.Context, offerID int64, status string) error {
	query := `UPDATE ride_offers SET status = $1, responded_at = NOW() WHERE offer_id = $2`
	_, err := r.db.Exec(ctx, query, status, offerID)
	return err
}

func (r *rideOfferRepoImpl) DeclineOffer(ctx context.Context, offerID int64) error {
	return r.UpdateStatus(ctx, offerID, model.RideOfferStatusDeclined)
}

// ExpireStaleOffers bulk-expires all pending offers past their TTL.
// Returns the expired offers for notification purposes.
func (r *rideOfferRepoImpl) ExpireStaleOffers(ctx context.Context) ([]model.RideOffer, error) {
	query := `
		UPDATE ride_offers SET status = 'expired', responded_at = NOW()
		WHERE status = 'pending' AND expires_at <= NOW()
		RETURNING offer_id, ride_id, rider_id, status, created_at, expires_at, responded_at
	`
	return r.scanOffers(ctx, query)
}

// ExpireOffersForRide expires all pending offers for a specific ride (e.g. when accepted by someone).
func (r *rideOfferRepoImpl) ExpireOffersForRide(ctx context.Context, rideID int64) ([]model.RideOffer, error) {
	query := `
		UPDATE ride_offers SET status = 'expired', responded_at = NOW()
		WHERE ride_id = $1 AND status = 'pending'
		RETURNING offer_id, ride_id, rider_id, status, created_at, expires_at, responded_at
	`
	return r.scanOffers(ctx, query, rideID)
}

// scanOffers is a helper to scan multiple offer rows.
func (r *rideOfferRepoImpl) scanOffers(ctx context.Context, query string, args ...any) ([]model.RideOffer, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offers []model.RideOffer
	for rows.Next() {
		var o model.RideOffer
		if err := rows.Scan(&o.OfferID, &o.RideID, &o.RiderID, &o.Status, &o.CreatedAt, &o.ExpiresAt, &o.RespondedAt); err != nil {
			return nil, err
		}
		offers = append(offers, o)
	}
	return offers, rows.Err()
}

// DefaultRideOfferTTL is the TTL for ride offers (5 minutes for immediate rides).
const DefaultRideOfferTTL = 5 * time.Minute
