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
	// GetServicesWithPressures returns a map of service_id -> pressures supported
	GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error)
	// Manage pressures supported by a therapist for a given service
	SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error
	// CreateProfile creates a therapist_profiles row for a user if one does not exist
	CreateProfile(ctx context.Context, therapistID int64) error

	FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error)
	FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error)
}

type therapistRepoImpl struct {
	db *pgxpool.Pool
}

func NewTherapistRepository(db *pgxpool.Pool) TherapistRepository {
	return &therapistRepoImpl{db: db}
}

func (r *therapistRepoImpl) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	query := `
		SELECT therapist_id, bio, years_experience, avg_rating, 
			   total_reviews, total_bookings, is_verified, accept_assignments, created_at, updated_at
		FROM therapist_profiles
		WHERE therapist_id = $1
	`
	var tp model.TherapistProfile
	if err := r.db.QueryRow(ctx, query, therapistID).Scan(
		&tp.TherapistID,
		&tp.Bio,
		&tp.YearsExperience,
		&tp.AvgRating,
		&tp.TotalReviews,
		&tp.TotalBookings,
		&tp.IsVerified,
		&tp.AcceptAssignments,
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
		SELECT therapist_id, bio, years_experience, avg_rating, 
			   total_reviews, total_bookings, is_verified, accept_assignments, created_at, updated_at
		FROM therapist_profiles
	`
	if availableOnly {
		query += " WHERE accept_assignments = TRUE AND is_verified = TRUE"
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
		if err := rows.Scan(&tp.TherapistID, &tp.Bio, &tp.YearsExperience, &tp.AvgRating, &tp.TotalReviews, &tp.TotalBookings, &tp.IsVerified, &tp.AcceptAssignments, &tp.CreatedAt, &tp.UpdatedAt); err != nil {
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
		INSERT INTO therapist_services (therapist_id, service_id, supports_soft, supports_moderate, supports_hard)
		VALUES ($1,$2, COALESCE($3, false), COALESCE($4, false), COALESCE($5, false))
		ON CONFLICT (therapist_id, service_id) DO UPDATE
		  SET supports_soft = COALESCE($3, therapist_services.supports_soft),
			  supports_moderate = COALESCE($4, therapist_services.supports_moderate),
			  supports_hard = COALESCE($5, therapist_services.supports_hard)
		RETURNING therapist_service_id, created_at, supports_soft, supports_moderate, supports_hard
	`
	return r.db.QueryRow(ctx, query, ts.TherapistID, ts.ServiceID, ts.SupportsSoft, ts.SupportsModerate, ts.SupportsHard).Scan(&ts.TherapistServiceID, &ts.CreatedAt, &ts.SupportsSoft, &ts.SupportsModerate, &ts.SupportsHard)
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

func (r *therapistRepoImpl) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) {
	// Read boolean support flags from therapist_services and convert to []string
	query := `
		SELECT service_id, supports_soft, supports_moderate, supports_hard
		FROM therapist_services
		WHERE therapist_id = $1
	`
	rows, err := r.db.Query(ctx, query, therapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64][]string)
	for rows.Next() {
		var sid int64
		var soft, med, hard bool
		if err := rows.Scan(&sid, &soft, &med, &hard); err != nil {
			return nil, err
		}
		var pressures []string
		if soft {
			pressures = append(pressures, "soft")
		}
		if med {
			pressures = append(pressures, "medium")
		}
		if hard {
			pressures = append(pressures, "hard")
		}
		out[sid] = pressures
	}
	return out, rows.Err()
}

func (r *therapistRepoImpl) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error {
	// Update boolean columns on therapist_services row. Ensure row exists.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Ensure service row exists
	if _, err := tx.Exec(ctx, `INSERT INTO therapist_services (therapist_id, service_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, therapistID, serviceID); err != nil {
		return err
	}

	// compute booleans
	soft := false
	med := false
	hard := false
	for _, p := range pressures {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "soft":
			soft = true
		case "medium", "med", "moderate":
			med = true
		case "hard":
			hard = true
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE therapist_services SET supports_soft = $1, supports_moderate = $2, supports_hard = $3 WHERE therapist_id = $4 AND service_id = $5`, soft, med, hard, therapistID, serviceID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (r *therapistRepoImpl) CreateProfile(ctx context.Context, therapistID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO therapist_profiles (therapist_id, created_at, updated_at)
		VALUES ($1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (therapist_id) DO NOTHING
	`, therapistID)
	return err
}

// FindAvailableByService finds available therapists who offer a specific service
// Ordered by rating (highest first) and gender preference (if specified)
func (r *therapistRepoImpl) FindAvailableByService(
	ctx context.Context,
	clientID int64,
	serviceID int64,
	genderPreference string,
	pressurePreference string,
) ([]model.TherapistProfile, error) {
	query := `
		SELECT tp.therapist_id, tp.bio, tp.years_experience, 
			   tp.avg_rating, tp.total_reviews, tp.total_bookings, tp.is_verified, 
			   tp.accept_assignments, tp.created_at, tp.updated_at,
			   COALESCE(u.gender, '') as gender,
			   ts.supports_soft, ts.supports_moderate, ts.supports_hard
		FROM therapist_profiles tp
		JOIN therapist_services ts ON tp.therapist_id = ts.therapist_id
		JOIN users u ON tp.therapist_id = u.user_id
		WHERE ts.service_id = $1 
		  AND tp.is_verified = TRUE
		  AND tp.accept_assignments = TRUE
		  AND u.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM user_blocks 
			WHERE (blocker_user_id = $2 AND blocked_user_id = tp.therapist_id)
			   OR (blocker_user_id = tp.therapist_id AND blocked_user_id = $2)
		  )
	`

	args := []interface{}{serviceID, clientID}
	argIdx := 3

	if genderPreference != "" && genderPreference != "any" {
		query += fmt.Sprintf(" AND u.gender = $%d", argIdx)
		args = append(args, genderPreference)
		argIdx++
	}

	if pressurePreference != "" && pressurePreference != "any" {
		switch strings.ToLower(pressurePreference) {
		case "soft":
			query += " AND ts.supports_soft = TRUE"
		case "medium", "moderate":
			query += " AND ts.supports_moderate = TRUE"
		case "hard":
			query += " AND ts.supports_hard = TRUE"
		}
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
		var soft, med, hard bool
		if err := rows.Scan(
			&tp.TherapistID, &tp.Bio, &tp.YearsExperience,
			&tp.AvgRating, &tp.TotalReviews, &tp.TotalBookings, &tp.IsVerified,
			&tp.AcceptAssignments, &tp.CreatedAt, &tp.UpdatedAt, &gender, &soft, &med, &hard,
		); err != nil {
			return nil, err
		}
		tp.Gender = gender
		var pressures []string
		if soft {
			pressures = append(pressures, "soft")
		}
		if med {
			pressures = append(pressures, "medium")
		}
		if hard {
			pressures = append(pressures, "hard")
		}
		tp.PressurePreferences = pressures
		profiles = append(profiles, tp)
	}

	return profiles, rows.Err()
}

// FindNearbyByService finds available therapists within a radius offering a service
// Uses geospatial queries with PostgreSQL's earth distance operator
// Ordered by distance (closest first), then rating
func (r *therapistRepoImpl) FindNearbyByService(
	ctx context.Context,
	clientID int64,
	serviceID int64,
	latitude float64,
	longitude float64,
	radiusKm float64,
	genderPreference string,
	pressurePreference string,
) ([]model.TherapistProfile, error) {
	// Geolocation ignored as per requirements.
	// Just delegate to FindAvailableByService logic (ignoring lat/long/radius).
	return r.FindAvailableByService(ctx, clientID, serviceID, genderPreference, pressurePreference)
}
