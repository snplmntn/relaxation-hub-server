package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type BookingOfferRepository interface {
	Create(ctx context.Context, offer *model.BookingOffer) error
	GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error)
	// Get active pending offers targeted to a therapist
	GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error)
	GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error)
	UpdateStatus(ctx context.Context, offerID int64, status string) error
	UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error
	ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error)
	ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error)
	GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error)
}

type bookingOfferRepoImpl struct {
	db *pgxpool.Pool
}

func NewBookingOfferRepository(db *pgxpool.Pool) BookingOfferRepository {
	return &bookingOfferRepoImpl{db: db}
}

func (r *bookingOfferRepoImpl) Create(ctx context.Context, offer *model.BookingOffer) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO booking_offers (booking_id, therapist_id, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING offer_id
	`, offer.BookingID, offer.TherapistID, offer.Status, offer.CreatedAt, offer.ExpiresAt).Scan(&offer.OfferID)
	return err
}

func (r *bookingOfferRepoImpl) GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT offer_id, booking_id, therapist_id, status, created_at, expires_at
		FROM booking_offers
		WHERE booking_id = $1 AND status = 'pending' AND expires_at > NOW()
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offers []model.BookingOffer
	for rows.Next() {
		var o model.BookingOffer
		if err := rows.Scan(&o.OfferID, &o.BookingID, &o.TherapistID, &o.Status, &o.CreatedAt, &o.ExpiresAt); err != nil {
			return nil, err
		}
		offers = append(offers, o)
	}
	return offers, nil
}

func (r *bookingOfferRepoImpl) GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error) {
	var o model.BookingOffer
	err := r.db.QueryRow(ctx, `
		SELECT offer_id, booking_id, therapist_id, status, created_at, expires_at
		FROM booking_offers
		WHERE booking_id = $1 AND therapist_id = $2
	`, bookingID, therapistID).Scan(&o.OfferID, &o.BookingID, &o.TherapistID, &o.Status, &o.CreatedAt, &o.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *bookingOfferRepoImpl) UpdateStatus(ctx context.Context, offerID int64, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE booking_offers
		SET status = $1
		WHERE offer_id = $2
	`, status, offerID)
	return err
}

func (r *bookingOfferRepoImpl) UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error {
	_, err := tx.Exec(ctx, `
		UPDATE booking_offers
		SET status = $1
		WHERE offer_id = $2
	`, status, offerID)
	return err
}

func (r *bookingOfferRepoImpl) ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE booking_offers
		SET status = 'expired'
		WHERE booking_id = $1 AND status = 'pending' AND expires_at <= NOW()
		RETURNING offer_id, booking_id, therapist_id, status, created_at, expires_at
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expired []model.BookingOffer
	for rows.Next() {
		var o model.BookingOffer
		if err := rows.Scan(&o.OfferID, &o.BookingID, &o.TherapistID, &o.Status, &o.CreatedAt, &o.ExpiresAt); err != nil {
			return nil, err
		}
		expired = append(expired, o)
	}
	return expired, nil
}

func (r *bookingOfferRepoImpl) ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error) {
	rows, err := tx.Query(ctx, `
		UPDATE booking_offers
		SET status = 'expired'
		WHERE booking_id = $1 AND status = 'pending'
		RETURNING offer_id, booking_id, therapist_id, status, created_at, expires_at
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expired []model.BookingOffer
	for rows.Next() {
		var o model.BookingOffer
		if err := rows.Scan(&o.OfferID, &o.BookingID, &o.TherapistID, &o.Status, &o.CreatedAt, &o.ExpiresAt); err != nil {
			return nil, err
		}
		expired = append(expired, o)
	}
	return expired, nil
}

func (r *bookingOfferRepoImpl) GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT offer_id, booking_id, therapist_id, status, created_at, expires_at
		FROM booking_offers
		WHERE booking_id = $1
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offers []model.BookingOffer
	for rows.Next() {
		var o model.BookingOffer
		if err := rows.Scan(&o.OfferID, &o.BookingID, &o.TherapistID, &o.Status, &o.CreatedAt, &o.ExpiresAt); err != nil {
			return nil, err
		}
		offers = append(offers, o)
	}
	return offers, nil
}

func (r *bookingOfferRepoImpl) GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT offer_id, booking_id, therapist_id, status, created_at, expires_at
		FROM booking_offers
		WHERE therapist_id = $1 AND status = 'pending' AND expires_at > NOW()
	`, therapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offers []model.BookingOffer
	for rows.Next() {
		var o model.BookingOffer
		if err := rows.Scan(&o.OfferID, &o.BookingID, &o.TherapistID, &o.Status, &o.CreatedAt, &o.ExpiresAt); err != nil {
			return nil, err
		}
		offers = append(offers, o)
	}
	return offers, nil
}
