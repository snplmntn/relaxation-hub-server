package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ReviewRepository manages reviews.
type ReviewRepository interface {
	Create(ctx context.Context, r *model.Review) error
	ListByTherapist(ctx context.Context, therapistID int64) ([]model.Review, error)
}

type reviewRepoImpl struct {
	db *pgxpool.Pool
}

func NewReviewRepository(db *pgxpool.Pool) ReviewRepository {
	return &reviewRepoImpl{db: db}
}

func (r *reviewRepoImpl) Create(ctx context.Context, rev *model.Review) error {
	query := `
        INSERT INTO reviews (
            booking_id, client_id, therapist_id, service_id,
            therapist_rating, therapist_review,
            service_rating, service_review,
            platform_rating, platform_review
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        RETURNING review_id, created_at, updated_at
    `
	return r.db.QueryRow(ctx, query,
		rev.BookingID,
		rev.ClientID,
		rev.TherapistID,
		rev.ServiceID,
		rev.TherapistRating,
		rev.TherapistReview,
		rev.ServiceRating,
		rev.ServiceReview,
		rev.PlatformRating,
		rev.PlatformReview,
	).Scan(&rev.ReviewID, &rev.CreatedAt, &rev.UpdatedAt)
}

func (r *reviewRepoImpl) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Review, error) {
	query := `
        SELECT review_id, booking_id, client_id, therapist_id, service_id,
               therapist_rating, therapist_review,
               service_rating, service_review,
               platform_rating, platform_review,
               created_at, updated_at
        FROM reviews
        WHERE therapist_id = $1 AND deleted_at IS NULL
        ORDER BY created_at DESC
    `
	rows, err := r.db.Query(ctx, query, therapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Review
	for rows.Next() {
		var rev model.Review
		if err := rows.Scan(
			&rev.ReviewID,
			&rev.BookingID,
			&rev.ClientID,
			&rev.TherapistID,
			&rev.ServiceID,
			&rev.TherapistRating,
			&rev.TherapistReview,
			&rev.ServiceRating,
			&rev.ServiceReview,
			&rev.PlatformRating,
			&rev.PlatformReview,
			&rev.CreatedAt,
			&rev.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	return out, nil
}
