package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type BookingOfferRepository interface {
	Create(ctx context.Context, offer *model.BookingOffer) error
	CreateTx(ctx context.Context, tx pgx.Tx, offer *model.BookingOffer) error
	GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error)
	// GetActiveOffersBatch fetches active pending offers for multiple bookings in a single query
	GetActiveOffersBatch(ctx context.Context, bookingIDs []int64) (map[int64][]model.BookingOffer, error)
	// Get active pending offers targeted to a therapist
	GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error)
	GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error)
	UpdateStatus(ctx context.Context, offerID int64, status string) error
	UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error
	ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error)
	ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error)
	CancelOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error)
	GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error)
}

type bookingOfferRepoImpl struct {
	db db.DBTX
}

func NewBookingOfferRepository(db db.DBTX) BookingOfferRepository {
	return &bookingOfferRepoImpl{db: db}
}

func (r *bookingOfferRepoImpl) Create(ctx context.Context, offer *model.BookingOffer) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.CreateTx(ctx, tx, offer); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *bookingOfferRepoImpl) CreateTx(ctx context.Context, tx pgx.Tx, offer *model.BookingOffer) error {
	// Insert main offer
	// Note: We still populate booking_id for backward compatibility if provided (usually the "primary" booking)
	err := tx.QueryRow(ctx, `
		INSERT INTO booking_offers (booking_id, therapist_id, status, created_at, expires_at, estimated_earnings, is_bundle)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING offer_id
	`, offer.BookingID, offer.TherapistID, offer.Status, offer.CreatedAt, offer.ExpiresAt, offer.EstimatedEarnings, offer.IsBundle).Scan(&offer.OfferID)
	if err != nil {
		return err
	}

	// Insert items (if any)
	if len(offer.Items) > 0 {
		for _, item := range offer.Items {
			_, err := tx.Exec(ctx, `
				INSERT INTO booking_offer_items (offer_id, booking_id, estimated_earnings)
				VALUES ($1, $2, $3)
				ON CONFLICT DO NOTHING
			`, offer.OfferID, item.BookingID, item.EstimatedEarnings)
			if err != nil {
				return err
			}
		}
	} else if offer.BookingID != 0 {
		// Auto-create item for single booking for consistency
		_, err := tx.Exec(ctx, `
			INSERT INTO booking_offer_items (offer_id, booking_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, offer.OfferID, offer.BookingID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *bookingOfferRepoImpl) GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	// Join with items to find offers relevant to this booking (whether primary or bundled)
	rows, err := r.db.Query(ctx, `
		SELECT o.offer_id, o.booking_id, o.therapist_id, o.status, o.created_at, o.expires_at, o.estimated_earnings, o.is_bundle
		FROM booking_offers o
		LEFT JOIN booking_offer_items i ON o.offer_id = i.offer_id
		WHERE (o.booking_id = $1 OR i.booking_id = $1)
		  AND o.status = 'pending' 
		  AND o.expires_at > NOW()
		GROUP BY o.offer_id
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offers []model.BookingOffer
	for rows.Next() {
		var o model.BookingOffer
		// handle potential NULL booking_id if purely using items (though we fallback to 0)
		var bid *int64
		if err := rows.Scan(&o.OfferID, &bid, &o.TherapistID, &o.Status, &o.CreatedAt, &o.ExpiresAt); err != nil {
			return nil, err
		}
		if bid != nil {
			o.BookingID = *bid
		}
		
		// Fetch items for this offer
		// Note: This matches N+1 but for active offers (rarely > 1-2 concurrent), it's acceptable.
		// Optimizing this requires a more complex query or eager loading.
		items, err := r.getItems(ctx, o.OfferID)
		if err == nil {
			o.Items = items
		}

		offers = append(offers, o)
	}
	return offers, nil
}

// Helper to fetch items
func (r *bookingOfferRepoImpl) getItems(ctx context.Context, offerID int64) ([]model.BookingOfferItem, error) {
	rows, err := r.db.Query(ctx, `SELECT offer_id, booking_id FROM booking_offer_items WHERE offer_id = $1`, offerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.BookingOfferItem
	for rows.Next() {
		var i model.BookingOfferItem
		if err := rows.Scan(&i.OfferID, &i.BookingID); err != nil {
			continue
		}
		items = append(items, i)
	}
	return items, nil
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

func (r *bookingOfferRepoImpl) CancelOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE booking_offers
		SET status = 'cancelled'
		WHERE booking_id = $1 AND status = 'pending'
		RETURNING offer_id, booking_id, therapist_id, status, created_at, expires_at
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cancelled []model.BookingOffer
	for rows.Next() {
		var o model.BookingOffer
		if err := rows.Scan(&o.OfferID, &o.BookingID, &o.TherapistID, &o.Status, &o.CreatedAt, &o.ExpiresAt); err != nil {
			return nil, err
		}
		cancelled = append(cancelled, o)
	}
	return cancelled, nil
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

func (r *bookingOfferRepoImpl) GetActiveOffersBatch(ctx context.Context, bookingIDs []int64) (map[int64][]model.BookingOffer, error) {
	if len(bookingIDs) == 0 {
		return make(map[int64][]model.BookingOffer), nil
	}

	// Find offers linked to any of these bookings (via main ID or items)
	rows, err := r.db.Query(ctx, `
		SELECT o.offer_id, o.booking_id, o.therapist_id, o.status, o.created_at, o.expires_at, i.booking_id as link_id
		FROM booking_offers o
		LEFT JOIN booking_offer_items i ON o.offer_id = i.offer_id
		WHERE (o.booking_id = ANY($1) OR i.booking_id = ANY($1))
		  AND o.status = 'pending' 
		  AND o.expires_at > NOW()
	`, bookingIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Need to map offers back to the requested booking IDs. 
	// An offer might match multiple bookings in the batch (bundled).
	offerMap := make(map[int64]model.BookingOffer)
	bookingToOfferIDs := make(map[int64][]int64)

	for rows.Next() {
		var o model.BookingOffer
		var bid *int64
		var linkID *int64 // The ID that caused the match from items
		if err := rows.Scan(&o.OfferID, &bid, &o.TherapistID, &o.Status, &o.CreatedAt, &o.ExpiresAt, &linkID); err != nil {
			return nil, err
		}
		if bid != nil {
			o.BookingID = *bid
		}
		
		// Store offer uniquely
		if _, exists := offerMap[o.OfferID]; !exists {
			offerMap[o.OfferID] = o
		}

		// Map to the requested booking ID(s)
		// If main booking matches
		if bid != nil {
			bookingToOfferIDs[*bid] = append(bookingToOfferIDs[*bid], o.OfferID)
		}
		// If item link matches
		if linkID != nil {
			bookingToOfferIDs[*linkID] = append(bookingToOfferIDs[*linkID], o.OfferID)
		}
	}

	result := make(map[int64][]model.BookingOffer)
	
	// Populate result
	for bid, offerIDs := range bookingToOfferIDs {
		seen := make(map[int64]bool)
		for _, oid := range offerIDs {
			if seen[oid] { continue }
			seen[oid] = true
			if o, ok := offerMap[oid]; ok {
				result[bid] = append(result[bid], o)
			}
		}
	}
	
	return result, rows.Err()
}

