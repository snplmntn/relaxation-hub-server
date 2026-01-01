package repository

import (
	"context"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ReviewRepository manages reviews.
type ReviewRepository interface {
	Create(ctx context.Context, r *model.Review) error
	ListByTherapist(ctx context.Context, therapistID int64, limit, offset int) ([]model.Review, int, error)
	ListByClient(ctx context.Context, clientID int64, limit, offset int) ([]model.Review, int, error)
	GetByBookingID(ctx context.Context, bookingID int64) (*model.Review, error)
	Update(ctx context.Context, r *model.Review) error
}

type reviewRepoImpl struct {
	db db.DBTX
}

func NewReviewRepository(db db.DBTX) ReviewRepository {
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

func (r *reviewRepoImpl) Update(ctx context.Context, rev *model.Review) error {
	query := `
		UPDATE reviews
		SET therapist_rating = $1, therapist_review = $2,
			service_rating = $3, service_review = $4,
			platform_rating = $5, platform_review = $6,
			updated_at = NOW()
		WHERE review_id = $7
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query,
		rev.TherapistRating, rev.TherapistReview,
		rev.ServiceRating, rev.ServiceReview,
		rev.PlatformRating, rev.PlatformReview,
		rev.ReviewID,
	).Scan(&rev.UpdatedAt)
}

func (r *reviewRepoImpl) GetByBookingID(ctx context.Context, bookingID int64) (*model.Review, error) {
	query := `
		SELECT review_id, booking_id, client_id, therapist_id, service_id,
			   therapist_rating, therapist_review,
			   service_rating, service_review,
			   platform_rating, platform_review,
			   created_at, updated_at
		FROM reviews
		WHERE booking_id = $1 AND deleted_at IS NULL
	`
	var rev model.Review
	err := r.db.QueryRow(ctx, query, bookingID).Scan(
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
	)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

func (r *reviewRepoImpl) ListByClient(ctx context.Context, clientID int64, limit, offset int) ([]model.Review, int, error) {
	var total int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM reviews WHERE client_id = $1 AND deleted_at IS NULL", clientID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT review_id, booking_id, client_id, therapist_id, service_id,
               therapist_rating, therapist_review,
               service_rating, service_review,
               platform_rating, platform_review,
               created_at, updated_at
        FROM reviews
        WHERE client_id = $1 AND deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := r.db.Query(ctx, query, clientID, limit, offset)
	if err != nil {
		return nil, 0, err
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
			return nil, 0, err
		}
		out = append(out, rev)
	}
	return out, total, nil
}

func (r *reviewRepoImpl) ListByTherapist(ctx context.Context, therapistID int64, limit, offset int) ([]model.Review, int, error) {
	// 1. Get total count
	var total int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM reviews WHERE therapist_id = $1 AND deleted_at IS NULL", therapistID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT review_id, booking_id, client_id, therapist_id, service_id,
               therapist_rating, therapist_review,
               service_rating, service_review,
               platform_rating, platform_review,
               created_at, updated_at
        FROM reviews
        WHERE therapist_id = $1 AND deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := r.db.Query(ctx, query, therapistID, limit, offset)
	if err != nil {
		return nil, 0, err
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
			return nil, 0, err
		}
		out = append(out, rev)
	}
	return out, total, nil
}
