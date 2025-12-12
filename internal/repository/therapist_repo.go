package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// TherapistRepository manages therapist profiles and related data.
type TherapistRepository interface {
	GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error)
	UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error
	List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error)

	UploadDocument(ctx context.Context, doc *model.TherapistDocument) error
	GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error)
	VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error

	AddService(ctx context.Context, ts *model.TherapistService) error
	RemoveService(ctx context.Context, therapistID, serviceID int64) error
	GetServices(ctx context.Context, therapistID int64) ([]int64, error)

	FindAvailableByService(ctx context.Context, serviceID int64, genderPreference string) ([]model.TherapistProfile, error)
	FindNearbyByService(ctx context.Context, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string) ([]model.TherapistProfile, error)
}

type therapistRepoImpl struct {
	db *pgxpool.Pool
}

func NewTherapistRepository(db *pgxpool.Pool) TherapistRepository {
	return &therapistRepoImpl{db: db}
}

func (r *therapistRepoImpl) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	query := `
		SELECT therapist_id, bio, specialization, years_experience, avg_rating, 
		       total_reviews, total_bookings, is_verified, is_available, created_at, updated_at
		FROM therapist_profiles
		WHERE therapist_id = $1
	`
	var tp model.TherapistProfile
	if err := r.db.QueryRow(ctx, query, therapistID).Scan(
		&tp.TherapistID,
		&tp.Bio,
		&tp.Specialization,
		&tp.YearsExperience,
		&tp.AvgRating,
		&tp.TotalReviews,
		&tp.TotalBookings,
		&tp.IsVerified,
		&tp.IsAvailable,
		&tp.CreatedAt,
		&tp.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &tp, nil
}

func (r *therapistRepoImpl) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []interface{}
	argIdx := 1

	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, therapistID)

	query := fmt.Sprintf("UPDATE therapist_profiles SET %s WHERE therapist_id = $%d", strings.Join(setClauses, ", "), argIdx)

	cmd, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *therapistRepoImpl) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) {
	query := `
		SELECT therapist_id, bio, specialization, years_experience, avg_rating, 
		       total_reviews, total_bookings, is_verified, is_available, created_at, updated_at
		FROM therapist_profiles
	`
	if availableOnly {
		query += " WHERE is_available = TRUE AND is_verified = TRUE"
	}
	query += " ORDER BY avg_rating DESC, total_reviews DESC"

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []model.TherapistProfile
	for rows.Next() {
		var tp model.TherapistProfile
		if err := rows.Scan(&tp.TherapistID, &tp.Bio, &tp.Specialization, &tp.YearsExperience, &tp.AvgRating, &tp.TotalReviews, &tp.TotalBookings, &tp.IsVerified, &tp.IsAvailable, &tp.CreatedAt, &tp.UpdatedAt); err != nil {
			return nil, err
		}
		profiles = append(profiles, tp)
	}
	return profiles, rows.Err()
}

func (r *therapistRepoImpl) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error {
	query := `
		INSERT INTO therapist_documents (therapist_id, document_type, document_url, status)
		VALUES ($1,$2,$3,$4)
		RETURNING document_id, uploaded_at
	`
	return r.db.QueryRow(ctx, query,
		doc.TherapistID,
		doc.DocumentType,
		doc.DocumentURL,
		doc.Status,
	).Scan(&doc.DocumentID, &doc.UploadedAt)
}

func (r *therapistRepoImpl) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) {
	query := `
		SELECT document_id, therapist_id, document_type, document_url, status, 
		       uploaded_at, verified_at, verified_by
		FROM therapist_documents
		WHERE therapist_id = $1
		ORDER BY uploaded_at DESC
	`
	rows, err := r.db.Query(ctx, query, therapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []model.TherapistDocument
	for rows.Next() {
		var doc model.TherapistDocument
		if err := rows.Scan(&doc.DocumentID, &doc.TherapistID, &doc.DocumentType, &doc.DocumentURL, &doc.Status, &doc.UploadedAt, &doc.VerifiedAt, &doc.VerifiedBy); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (r *therapistRepoImpl) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE therapist_documents
		SET status = $1,
		    verified_at = CURRENT_TIMESTAMP,
		    verified_by = $2
		WHERE document_id = $3
	`, status, verifierID, documentID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *therapistRepoImpl) AddService(ctx context.Context, ts *model.TherapistService) error {
	query := `
		INSERT INTO therapist_services (therapist_id, service_id)
		VALUES ($1,$2)
		ON CONFLICT (therapist_id, service_id) DO NOTHING
		RETURNING therapist_service_id, created_at
	`
	return r.db.QueryRow(ctx, query, ts.TherapistID, ts.ServiceID).Scan(&ts.TherapistServiceID, &ts.CreatedAt)
}

func (r *therapistRepoImpl) RemoveService(ctx context.Context, therapistID, serviceID int64) error {
	cmd, err := r.db.Exec(ctx, `
		DELETE FROM therapist_services
		WHERE therapist_id = $1 AND service_id = $2
	`, therapistID, serviceID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *therapistRepoImpl) GetServices(ctx context.Context, therapistID int64) ([]int64, error) {
	query := `
		SELECT service_id
		FROM therapist_services
		WHERE therapist_id = $1
	`
	rows, err := r.db.Query(ctx, query, therapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var serviceIDs []int64
	for rows.Next() {
		var sid int64
		if err := rows.Scan(&sid); err != nil {
			return nil, err
		}
		serviceIDs = append(serviceIDs, sid)
	}
	return serviceIDs, rows.Err()
}

// FindAvailableByService finds available therapists who offer a specific service
// Ordered by rating (highest first) and gender preference (if specified)
func (r *therapistRepoImpl) FindAvailableByService(
	ctx context.Context,
	serviceID int64,
	genderPreference string,
) ([]model.TherapistProfile, error) {
	query := `
		SELECT DISTINCT tp.therapist_id, tp.bio, tp.specialization, tp.years_experience, 
		       tp.avg_rating, tp.total_reviews, tp.total_bookings, tp.is_verified, 
		       tp.is_available, tp.created_at, tp.updated_at,
		       COALESCE(u.gender, '') as gender
		FROM therapist_profiles tp
		JOIN therapist_services ts ON tp.therapist_id = ts.therapist_id
		JOIN users u ON tp.therapist_id = u.user_id
		WHERE ts.service_id = $1 
		  AND tp.is_available = TRUE 
		  AND tp.is_verified = TRUE
		  AND u.deleted_at IS NULL
	`

	args := []interface{}{serviceID}
	argIdx := 2

	if genderPreference != "" && genderPreference != "any" {
		query += fmt.Sprintf(" AND u.gender = $%d", argIdx)
		args = append(args, genderPreference)
		argIdx++
	}

	query += " ORDER BY tp.avg_rating DESC, tp.total_reviews DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []model.TherapistProfile
	for rows.Next() {
		var tp model.TherapistProfile
		var gender string
		if err := rows.Scan(
			&tp.TherapistID, &tp.Bio, &tp.Specialization, &tp.YearsExperience,
			&tp.AvgRating, &tp.TotalReviews, &tp.TotalBookings, &tp.IsVerified,
			&tp.IsAvailable, &tp.CreatedAt, &tp.UpdatedAt, &gender,
		); err != nil {
			return nil, err
		}
		tp.Gender = gender
		profiles = append(profiles, tp)
	}

	return profiles, rows.Err()
}

// FindNearbyByService finds available therapists within a radius offering a service
// Uses geospatial queries with PostgreSQL's earth distance operator
// Ordered by distance (closest first), then rating
func (r *therapistRepoImpl) FindNearbyByService(
	ctx context.Context,
	serviceID int64,
	latitude float64,
	longitude float64,
	radiusKm float64,
	genderPreference string,
) ([]model.TherapistProfile, error) {
	// Using PostgreSQL's point operators for distance calculation
	// Distance is calculated in meters, converted to km
	query := `
		SELECT DISTINCT tp.therapist_id, tp.bio, tp.specialization, tp.years_experience,
		       tp.avg_rating, tp.total_reviews, tp.total_bookings, tp.is_verified,
		       tp.is_available, tp.created_at, tp.updated_at,
		       COALESCE(u.gender, '') as gender,
		       (
		           6371 * acos(
		               cos(radians($2)) * cos(radians(ll.latitude)) *
		               cos(radians(ll.longitude) - radians($3)) +
		               sin(radians($2)) * sin(radians(ll.latitude))
		           )
		       ) AS distance_km
		FROM therapist_profiles tp
		JOIN therapist_services ts ON tp.therapist_id = ts.therapist_id
		JOIN users u ON tp.therapist_id = u.user_id
		LEFT JOIN live_locations ll ON tp.therapist_id = ll.user_id
		WHERE ts.service_id = $1
		  AND tp.is_available = TRUE
		  AND tp.is_verified = TRUE
		  AND u.deleted_at IS NULL
		  AND ll.latitude IS NOT NULL
		  AND ll.longitude IS NOT NULL
	`

	args := []interface{}{serviceID, latitude, longitude, radiusKm}
	argIdx := 5

	if genderPreference != "" && genderPreference != "any" {
		query += fmt.Sprintf(" AND u.gender = $%d", argIdx)
		args = append(args, genderPreference)
		argIdx++
	}

	// Apply radius filter in HAVING clause after distance calculation
	query += ` HAVING (
	           6371 * acos(
	               cos(radians($2)) * cos(radians(ll.latitude)) *
	               cos(radians(ll.longitude) - radians($3)) +
	               sin(radians($2)) * sin(radians(ll.latitude))
	           )
	       ) <= $4
	   ORDER BY distance_km ASC, tp.avg_rating DESC
	`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []model.TherapistProfile
	for rows.Next() {
		var tp model.TherapistProfile
		var gender string
		var distanceKm float64
		if err := rows.Scan(
			&tp.TherapistID, &tp.Bio, &tp.Specialization, &tp.YearsExperience,
			&tp.AvgRating, &tp.TotalReviews, &tp.TotalBookings, &tp.IsVerified,
			&tp.IsAvailable, &tp.CreatedAt, &tp.UpdatedAt, &gender, &distanceKm,
		); err != nil {
			return nil, err
		}
		tp.Gender = gender
		profiles = append(profiles, tp)
	}

	return profiles, rows.Err()
}
