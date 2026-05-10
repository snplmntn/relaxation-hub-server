package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ClientReviewRepository manages therapist-authored reviews of clients.
type ClientReviewRepository interface {
	Create(ctx context.Context, review *model.ClientReview) error
	ListByClient(ctx context.Context, clientID int64) ([]model.ClientReview, error)
	FindByBookingAndTherapist(ctx context.Context, bookingID, therapistID int64) (*model.ClientReview, error)
}

type clientReviewRepoImpl struct {
	db db.DBTX
}

func NewClientReviewRepository(db db.DBTX) ClientReviewRepository {
	return &clientReviewRepoImpl{db: db}
}

func (r *clientReviewRepoImpl) Create(ctx context.Context, review *model.ClientReview) error {
	query := `
        INSERT INTO client_reviews (
            booking_id, therapist_id, client_id,
            client_rating, client_review
        ) VALUES ($1,$2,$3,$4,$5)
        RETURNING client_review_id, created_at, updated_at
    `

	return r.db.QueryRow(ctx, query,
		review.BookingID,
		review.TherapistID,
		review.ClientID,
		review.ClientRating,
		review.ClientReview,
	).Scan(&review.ClientReviewID, &review.CreatedAt, &review.UpdatedAt)
}

func (r *clientReviewRepoImpl) ListByClient(ctx context.Context, clientID int64) ([]model.ClientReview, error) {
	query := `
        SELECT client_review_id, booking_id, therapist_id, client_id,
               client_rating, client_review, created_at, updated_at
        FROM client_reviews
        WHERE client_id = $1 AND deleted_at IS NULL
        ORDER BY created_at DESC
    `

	rows, err := r.db.Query(ctx, query, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.ClientReview
	for rows.Next() {
		var review model.ClientReview
		if err := rows.Scan(
			&review.ClientReviewID,
			&review.BookingID,
			&review.TherapistID,
			&review.ClientID,
			&review.ClientRating,
			&review.ClientReview,
			&review.CreatedAt,
			&review.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, review)
	}

	return out, nil
}

func (r *clientReviewRepoImpl) FindByBookingAndTherapist(ctx context.Context, bookingID, therapistID int64) (*model.ClientReview, error) {
	query := `
        SELECT client_review_id, booking_id, therapist_id, client_id,
               client_rating, client_review, created_at, updated_at
        FROM client_reviews
        WHERE booking_id = $1 AND therapist_id = $2 AND deleted_at IS NULL
        LIMIT 1
    `

	var review model.ClientReview
	err := r.db.QueryRow(ctx, query, bookingID, therapistID).Scan(
		&review.ClientReviewID,
		&review.BookingID,
		&review.TherapistID,
		&review.ClientID,
		&review.ClientRating,
		&review.ClientReview,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &review, nil
}
