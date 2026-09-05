package repository

import (
	"context"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ReviewDetailsResult contains a review with related data fetched in a single query
type ReviewDetailsResult struct {
	Review         *model.Review
	Service        *model.Service
	BookingDate    *time.Time
	TherapistName  string
	TherapistPhoto string
	ClientName     string
	ClientPhoto    string
}

// ReviewRepository manages reviews.
type ReviewRepository interface {
	Create(ctx context.Context, r *model.Review) error
	ListByTherapist(ctx context.Context, therapistID int64, limit, offset int) ([]model.Review, int, error)
	ListByClient(ctx context.Context, clientID int64, limit, offset int) ([]model.Review, int, error)
	ListByTherapistWithDetails(ctx context.Context, therapistID int64, limit, offset int) ([]ReviewDetailsResult, int, error)
	ListByClientWithDetails(ctx context.Context, clientID int64, limit, offset int) ([]ReviewDetailsResult, int, error)
	ListAllWithDetails(ctx context.Context, therapistID *int64, search string, minAvgRating float64, limit, offset int) ([]ReviewDetailsResult, int, error)
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

func (r *reviewRepoImpl) ListByClientWithDetails(ctx context.Context, clientID int64, limit, offset int) ([]ReviewDetailsResult, int, error) {
	var total int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM reviews WHERE client_id = $1 AND deleted_at IS NULL", clientID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT r.review_id, r.booking_id, r.client_id, r.therapist_id, r.service_id,
               r.therapist_rating, r.therapist_review,
               r.service_rating, r.service_review,
               r.platform_rating, r.platform_review,
               r.created_at, r.updated_at,
               s.service_id, s.name, s.description, s.base_price, s.duration_minutes, s.category,
               b.scheduled_start,
               u.full_name as therapist_name, u.profile_photo as therapist_photo
        FROM reviews r
        LEFT JOIN services s ON r.service_id = s.service_id
        LEFT JOIN bookings b ON r.booking_id = b.booking_id
        LEFT JOIN users u ON r.therapist_id = u.user_id
        WHERE r.client_id = $1 AND r.deleted_at IS NULL
        ORDER BY r.created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := r.db.Query(ctx, query, clientID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []ReviewDetailsResult
	for rows.Next() {
		var res ReviewDetailsResult
		var rev model.Review
		var svc model.Service
		var scheduledStart *time.Time
		var thName, thPhoto *string

		if err := rows.Scan(
			&rev.ReviewID, &rev.BookingID, &rev.ClientID, &rev.TherapistID, &rev.ServiceID,
			&rev.TherapistRating, &rev.TherapistReview,
			&rev.ServiceRating, &rev.ServiceReview,
			&rev.PlatformRating, &rev.PlatformReview,
			&rev.CreatedAt, &rev.UpdatedAt,
			&svc.ServiceID, &svc.Name, &svc.Description, &svc.BasePrice, &svc.DurationMinutes, &svc.Category,
			&scheduledStart,
			&thName, &thPhoto,
		); err != nil {
			return nil, 0, err
		}

		res.Review = &rev
		res.Service = &svc
		res.BookingDate = scheduledStart
		if thName != nil {
			res.TherapistName = *thName
		}
		if thPhoto != nil {
			res.TherapistPhoto = *thPhoto
		}
		out = append(out, res)
	}
	return out, total, nil
}

func (r *reviewRepoImpl) ListByTherapistWithDetails(ctx context.Context, therapistID int64, limit, offset int) ([]ReviewDetailsResult, int, error) {
	var total int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM reviews WHERE therapist_id = $1 AND deleted_at IS NULL", therapistID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT r.review_id, r.booking_id, r.client_id, r.therapist_id, r.service_id,
               r.therapist_rating, r.therapist_review,
               r.service_rating, r.service_review,
               r.platform_rating, r.platform_review,
               r.created_at, r.updated_at,
               s.service_id, s.name, s.description, s.base_price, s.duration_minutes, s.category,
               b.scheduled_start,
               u.full_name as therapist_name, u.profile_photo as therapist_photo,
               c.full_name as client_name, c.profile_photo as client_photo
        FROM reviews r
        LEFT JOIN services s ON r.service_id = s.service_id
        LEFT JOIN bookings b ON r.booking_id = b.booking_id
        LEFT JOIN users u ON r.therapist_id = u.user_id
        LEFT JOIN users c ON r.client_id = c.user_id
        WHERE r.therapist_id = $1 AND r.deleted_at IS NULL
        ORDER BY r.created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := r.db.Query(ctx, query, therapistID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []ReviewDetailsResult
	for rows.Next() {
		var res ReviewDetailsResult
		var rev model.Review
		var svc model.Service
		var scheduledStart *time.Time
		var thName, thPhoto *string
		var cName, cPhoto *string

		if err := rows.Scan(
			&rev.ReviewID, &rev.BookingID, &rev.ClientID, &rev.TherapistID, &rev.ServiceID,
			&rev.TherapistRating, &rev.TherapistReview,
			&rev.ServiceRating, &rev.ServiceReview,
			&rev.PlatformRating, &rev.PlatformReview,
			&rev.CreatedAt, &rev.UpdatedAt,
			&svc.ServiceID, &svc.Name, &svc.Description, &svc.BasePrice, &svc.DurationMinutes, &svc.Category,
			&scheduledStart,
			&thName, &thPhoto,
			&cName, &cPhoto,
		); err != nil {
			return nil, 0, err
		}

		res.Review = &rev
		res.Service = &svc
		res.BookingDate = scheduledStart
		if thName != nil {
			res.TherapistName = *thName
		}
		if thPhoto != nil {
			res.TherapistPhoto = *thPhoto
		}
		if cName != nil {
			res.ClientName = *cName
		}
		if cPhoto != nil {
			res.ClientPhoto = *cPhoto
		}
		out = append(out, res)
	}
	return out, total, nil
}

func (r *reviewRepoImpl) ListAllWithDetails(ctx context.Context, therapistID *int64, search string, minAvgRating float64, limit, offset int) ([]ReviewDetailsResult, int, error) {
	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM reviews r
		LEFT JOIN users therapist_u ON r.therapist_id = therapist_u.user_id
		LEFT JOIN users client_u ON r.client_id = client_u.user_id
		WHERE r.deleted_at IS NULL
		  AND ($1::bigint IS NULL OR r.therapist_id = $1)
		  AND (
		    $2::text = '' OR
		    LOWER(COALESCE(client_u.full_name, '')) LIKE '%' || LOWER($2) || '%' OR
		    LOWER(COALESCE(therapist_u.full_name, '')) LIKE '%' || LOWER($2) || '%' OR
		    LOWER(COALESCE(r.therapist_review, '')) LIKE '%' || LOWER($2) || '%' OR
		    LOWER(COALESCE(r.service_review, '')) LIKE '%' || LOWER($2) || '%' OR
		    LOWER(COALESCE(r.platform_review, '')) LIKE '%' || LOWER($2) || '%' OR
		    CAST(r.booking_id AS TEXT) LIKE '%' || $2 || '%'
		  )
		  AND (
		    $3::double precision <= 0 OR
		    ((COALESCE(r.therapist_rating, 0) + COALESCE(r.service_rating, 0) + COALESCE(r.platform_rating, 0)) /
		      NULLIF((CASE WHEN r.therapist_rating IS NOT NULL THEN 1 ELSE 0 END) +
		             (CASE WHEN r.service_rating IS NOT NULL THEN 1 ELSE 0 END) +
		             (CASE WHEN r.platform_rating IS NOT NULL THEN 1 ELSE 0 END), 0)::double precision) >= $3
		  )
	`
	if err := r.db.QueryRow(ctx, countQuery, therapistID, search, minAvgRating).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT r.review_id, r.booking_id, r.client_id, r.therapist_id, r.service_id,
		       r.therapist_rating, r.therapist_review,
		       r.service_rating, r.service_review,
		       r.platform_rating, r.platform_review,
		       r.created_at, r.updated_at,
		       s.service_id, s.name, s.description, s.base_price, s.duration_minutes, s.category,
		       b.scheduled_start,
		       therapist_u.full_name as therapist_name, therapist_u.profile_photo as therapist_photo,
		       client_u.full_name as client_name, client_u.profile_photo as client_photo
		FROM reviews r
		LEFT JOIN services s ON r.service_id = s.service_id
		LEFT JOIN bookings b ON r.booking_id = b.booking_id
		LEFT JOIN users therapist_u ON r.therapist_id = therapist_u.user_id
		LEFT JOIN users client_u ON r.client_id = client_u.user_id
		WHERE r.deleted_at IS NULL
		  AND ($1::bigint IS NULL OR r.therapist_id = $1)
		  AND (
		    $2::text = '' OR
		    LOWER(COALESCE(client_u.full_name, '')) LIKE '%' || LOWER($2) || '%' OR
		    LOWER(COALESCE(therapist_u.full_name, '')) LIKE '%' || LOWER($2) || '%' OR
		    LOWER(COALESCE(r.therapist_review, '')) LIKE '%' || LOWER($2) || '%' OR
		    LOWER(COALESCE(r.service_review, '')) LIKE '%' || LOWER($2) || '%' OR
		    LOWER(COALESCE(r.platform_review, '')) LIKE '%' || LOWER($2) || '%' OR
		    CAST(r.booking_id AS TEXT) LIKE '%' || $2 || '%'
		  )
		  AND (
		    $3::double precision <= 0 OR
		    ((COALESCE(r.therapist_rating, 0) + COALESCE(r.service_rating, 0) + COALESCE(r.platform_rating, 0)) /
		      NULLIF((CASE WHEN r.therapist_rating IS NOT NULL THEN 1 ELSE 0 END) +
		             (CASE WHEN r.service_rating IS NOT NULL THEN 1 ELSE 0 END) +
		             (CASE WHEN r.platform_rating IS NOT NULL THEN 1 ELSE 0 END), 0)::double precision) >= $3
		  )
		ORDER BY r.created_at DESC
		LIMIT $4 OFFSET $5
	`
	rows, err := r.db.Query(ctx, query, therapistID, search, minAvgRating, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []ReviewDetailsResult
	for rows.Next() {
		var res ReviewDetailsResult
		var rev model.Review
		var svc model.Service
		var scheduledStart *time.Time
		var thName, thPhoto *string
		var cName, cPhoto *string

		if err := rows.Scan(
			&rev.ReviewID, &rev.BookingID, &rev.ClientID, &rev.TherapistID, &rev.ServiceID,
			&rev.TherapistRating, &rev.TherapistReview,
			&rev.ServiceRating, &rev.ServiceReview,
			&rev.PlatformRating, &rev.PlatformReview,
			&rev.CreatedAt, &rev.UpdatedAt,
			&svc.ServiceID, &svc.Name, &svc.Description, &svc.BasePrice, &svc.DurationMinutes, &svc.Category,
			&scheduledStart,
			&thName, &thPhoto,
			&cName, &cPhoto,
		); err != nil {
			return nil, 0, err
		}

		res.Review = &rev
		res.Service = &svc
		res.BookingDate = scheduledStart
		if thName != nil {
			res.TherapistName = *thName
		}
		if thPhoto != nil {
			res.TherapistPhoto = *thPhoto
		}
		if cName != nil {
			res.ClientName = *cName
		}
		if cPhoto != nil {
			res.ClientPhoto = *cPhoto
		}
		out = append(out, res)
	}
	return out, total, nil
}
