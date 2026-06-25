package repository

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// TherapistRepository manages therapist profiles and related data.
type TherapistRepository interface {
	GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error)
	GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error)
	UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error
	SetLifecycleStatus(ctx context.Context, therapistID int64, accountStatus string, acceptAssignments bool) error
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
	FindAvailableByServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error)
	FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error)
	// SetAtBranch updates the at_branch status for a therapist (true = at branch, false = in field)
	SetAtBranch(ctx context.Context, therapistID int64, atBranch bool) error
	// TryLockTherapistTx attempts to acquire a transaction-level advisory lock for the therapist.
	// Must be called within an active transaction.
	TryLockTherapistTx(ctx context.Context, tx pgx.Tx, therapistID int64) (bool, error)
	// SetBatchServices replaces all services for a therapist with the provided ones.
	SetBatchServices(ctx context.Context, therapistID int64, services []model.AddServiceWithPressuresRequest) error
}

type therapistRepoImpl struct {
	db db.DBTX
}

func NewTherapistRepository(db db.DBTX) TherapistRepository {
	return &therapistRepoImpl{db: db}
}

func (r *therapistRepoImpl) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT tp.therapist_id, u.nickname, COALESCE(u.account_status, 'active'), tp.branch_id, tp.home_address_id, tp.bio, tp.years_experience, tp.avg_rating,
			   tp.total_reviews, tp.total_bookings, tp.is_verified, tp.accept_assignments, tp.at_branch, tp.created_at, tp.updated_at
		FROM therapist_profiles tp
		LEFT JOIN users u ON u.user_id = tp.therapist_id
		WHERE tp.therapist_id = $1
	`
	var tp model.TherapistProfile
	if err := r.db.QueryRow(ctx, query, therapistID).Scan(
		&tp.TherapistID,
		&tp.Nickname,
		&tp.Status,
		&tp.BranchID,
		&tp.HomeAddressID,
		&tp.Bio,
		&tp.YearsExperience,
		&tp.AvgRating,
		&tp.TotalReviews,
		&tp.TotalBookings,
		&tp.IsVerified,
		&tp.AcceptAssignments,
		&tp.AtBranch,
		&tp.CreatedAt,
		&tp.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &tp, nil
}

func (r *therapistRepoImpl) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	if len(therapistIDs) == 0 {
		return []model.TherapistProfile{}, nil
	}

	query := `
		SELECT tp.therapist_id, u.nickname, COALESCE(u.account_status, 'active'), tp.branch_id, tp.home_address_id, tp.bio, tp.years_experience, tp.avg_rating,
			   tp.total_reviews, tp.total_bookings, tp.is_verified, tp.accept_assignments, tp.at_branch, tp.created_at, tp.updated_at
		FROM therapist_profiles tp
		LEFT JOIN users u ON u.user_id = tp.therapist_id
		WHERE tp.therapist_id = ANY($1)
	`
	rows, err := r.db.Query(ctx, query, therapistIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []model.TherapistProfile
	for rows.Next() {
		var tp model.TherapistProfile
		if err := rows.Scan(
			&tp.TherapistID,
			&tp.Nickname,
			&tp.Status,
			&tp.BranchID,
			&tp.HomeAddressID,
			&tp.Bio,
			&tp.YearsExperience,
			&tp.AvgRating,
			&tp.TotalReviews,
			&tp.TotalBookings,
			&tp.IsVerified,
			&tp.AcceptAssignments,
			&tp.AtBranch,
			&tp.CreatedAt,
			&tp.UpdatedAt,
		); err != nil {
			return nil, err
		}
		profiles = append(profiles, tp)
	}
	return profiles, rows.Err()
}

func (r *therapistRepoImpl) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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

func (r *therapistRepoImpl) SetLifecycleStatus(ctx context.Context, therapistID int64, accountStatus string, acceptAssignments bool) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	cmd, err := tx.Exec(ctx, `
		UPDATE users
		SET account_status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2 AND role = 'therapist'
	`, accountStatus, therapistID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	cmd, err = tx.Exec(ctx, `
		UPDATE therapist_profiles
		SET accept_assignments = $1, updated_at = CURRENT_TIMESTAMP
		WHERE therapist_id = $2
	`, acceptAssignments, therapistID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return tx.Commit(ctx)
}

func (r *therapistRepoImpl) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT tp.therapist_id,
			   COALESCE(NULLIF(TRIM(u.full_name), ''), u.primary_email, u.primary_phone, ''),
			   u.nickname,
			   COALESCE(u.account_status, 'active'),
			   tp.branch_id, tp.bio, tp.years_experience, tp.avg_rating,
			   tp.total_reviews, tp.total_bookings, tp.is_verified, tp.accept_assignments, tp.at_branch, tp.created_at, tp.updated_at
		FROM therapist_profiles tp
		LEFT JOIN users u ON u.user_id = tp.therapist_id
	`
	if availableOnly {
		query += " WHERE tp.accept_assignments = TRUE AND tp.is_verified = TRUE AND u.account_status = 'active' AND u.deleted_at IS NULL"
	}
	query += " ORDER BY tp.avg_rating DESC, tp.total_reviews DESC"

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []model.TherapistProfile
	for rows.Next() {
		var tp model.TherapistProfile
		if err := rows.Scan(
			&tp.TherapistID, &tp.FullName, &tp.Nickname, &tp.Status, &tp.BranchID, &tp.Bio, &tp.YearsExperience,
			&tp.AvgRating, &tp.TotalReviews, &tp.TotalBookings, &tp.IsVerified, &tp.AcceptAssignments,
			&tp.AtBranch, &tp.CreatedAt, &tp.UpdatedAt,
		); err != nil {
			return nil, err
		}
		profiles = append(profiles, tp)
	}
	return profiles, rows.Err()
}

func (r *therapistRepoImpl) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		INSERT INTO therapist_services (therapist_id, service_id, supports_soft, supports_moderate, supports_hard)
		VALUES ($1,$2, COALESCE($3, false), COALESCE($4, false), COALESCE($5, false))
		ON CONFLICT (therapist_id, service_id) DO UPDATE
		  SET supports_soft = COALESCE($3, therapist_services.supports_soft),
			  supports_moderate = COALESCE($4, therapist_services.supports_moderate),
			  supports_hard = COALESCE($5, therapist_services.supports_hard)
		RETURNING supports_soft, supports_moderate, supports_hard
	`
	// Set CreatedAt manually since DB doesn't track it
	ts.CreatedAt = time.Now()
	return r.db.QueryRow(ctx, query, ts.TherapistID, ts.ServiceID, ts.SupportsSoft, ts.SupportsModerate, ts.SupportsHard).Scan(&ts.SupportsSoft, &ts.SupportsModerate, &ts.SupportsHard)
}

func (r *therapistRepoImpl) RemoveService(ctx context.Context, therapistID, serviceID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
		JOIN branches br ON tp.branch_id = br.branch_id
		WHERE ts.service_id = $1 
		  AND tp.is_verified = TRUE
		  AND tp.accept_assignments = TRUE
		  AND tp.branch_id IS NOT NULL
		  AND u.deleted_at IS NULL
		  AND u.account_status = 'active'
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

// FindAvailableByServiceWithTime finds available therapists who offer a specific service
// AND are not booked during the given time window (prevents double-booking).
// Uses dynamic travel buffer if coordinates are provided.
// Also checks "Home Base" travel buffer from therapist's branch to first booking.
func (r *therapistRepoImpl) FindAvailableByServiceWithTime(
	ctx context.Context,
	clientID int64,
	serviceID int64,
	genderPreference string,
	pressurePreference string,
	scheduledStart time.Time,
	durationMinutes int,
	lat *float64,
	lng *float64,
) ([]model.TherapistProfile, error) {
	scheduledEnd := scheduledStart.Add(time.Duration(durationMinutes) * time.Minute)

	query := `
						SELECT tp.therapist_id, tp.branch_id, br.latitude as branch_lat, br.longitude as branch_lng,
		       tp.bio, tp.years_experience, 
			   tp.avg_rating, tp.total_reviews, tp.total_bookings, tp.is_verified, 
			   tp.accept_assignments, tp.created_at, tp.updated_at,
			   COALESCE(u.gender, '') as gender,
			   ts.supports_soft, ts.supports_moderate, ts.supports_hard,
			   -- Calculate dynamic distance from booking location (if coords provided)
			   CASE 
			     WHEN $5::float8 IS NULL OR $6::float8 IS NULL THEN NULL 
			     ELSE calculate_distance_km(COALESCE(br.latitude, 0)::float8, COALESCE(br.longitude, 0)::float8, $5::float8, $6::float8)
			   END as distance_km
		FROM therapist_profiles tp
		JOIN therapist_services ts ON tp.therapist_id = ts.therapist_id
		JOIN users u ON tp.therapist_id = u.user_id
		LEFT JOIN branches br ON tp.branch_id = br.branch_id
		WHERE ts.service_id = $1 
		  AND tp.is_verified = TRUE
		  AND tp.accept_assignments = TRUE
		  AND u.deleted_at IS NULL
		  AND u.account_status = 'active'
		  AND NOT EXISTS (
			SELECT 1 FROM user_blocks 
			WHERE (blocker_user_id = $2 AND blocked_user_id = tp.therapist_id)
			   OR (blocker_user_id = tp.therapist_id AND blocked_user_id = $2)
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM bookings b
			LEFT JOIN addresses a ON b.address_id = a.address_id
			WHERE b.therapist_id = tp.therapist_id
			  AND b.status NOT IN ('cancelled', 'completed', 'no_show', 'pending')
			  AND b.scheduled_start IS NOT NULL
			  AND (
				  b.scheduled_start::timestamptz < ($4::timestamptz + (COALESCE(calculate_travel_buffer_minutes(calculate_distance_km($5::float8, $6::float8, a.latitude::float8, a.longitude::float8)), 0) * INTERVAL '1 minute'))
				  AND 
				  (b.scheduled_start::timestamptz + (b.duration_minutes * INTERVAL '1 minute') + (COALESCE(calculate_travel_buffer_minutes(calculate_distance_km($5::float8, $6::float8, a.latitude::float8, a.longitude::float8)), 0) * INTERVAL '1 minute')) > $3::timestamptz
			  )
		  )
	`

	args := []interface{}{serviceID, clientID, scheduledStart, scheduledEnd, lat, lng}
	argIdx := 7

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

	// DEBUG LOGGING
	slog.Info("FindAvailableByServiceWithTime: Executing query",
		"serviceID", serviceID,
		"clientID", clientID,
		"scheduledStart", scheduledStart,
		"lat", lat,
		"lng", lng,
		"genderPref", genderPreference,
		"pressurePref", pressurePreference,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		slog.Error("FindAvailableByServiceWithTime: Query failed", "error", err)
		return nil, err
	}
	defer rows.Close()

	var profiles []model.TherapistProfile
	for rows.Next() {
		var tp model.TherapistProfile
		var gender string
		var soft, med, hard bool
		// Handle Nullable Branch fields
		var bId *int64
		var bLat, bLng *float64

		if err := rows.Scan(
			&tp.TherapistID, &bId, &bLat, &bLng,
			&tp.Bio, &tp.YearsExperience,
			&tp.AvgRating, &tp.TotalReviews, &tp.TotalBookings, &tp.IsVerified,
			&tp.AcceptAssignments, &tp.CreatedAt, &tp.UpdatedAt, &gender, &soft, &med, &hard,
			&tp.DistanceKm,
		); err != nil {
			slog.Error("FindAvailableByServiceWithTime: Scan failed", "error", err)
			return nil, err
		}
		tp.BranchID = bId
		tp.BranchLat = bLat
		tp.BranchLng = bLng

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

	slog.Info("FindAvailableByServiceWithTime: Result count", "count", len(profiles))
	return profiles, rows.Err()
}

func (r *therapistRepoImpl) SetAtBranch(ctx context.Context, therapistID int64, atBranch bool) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		UPDATE therapist_profiles 
		SET at_branch = $2, last_location_update = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE therapist_id = $1
	`
	_, err := r.db.Exec(ctx, query, therapistID, atBranch)
	return err
}

func (r *therapistRepoImpl) TryLockTherapistTx(ctx context.Context, tx pgx.Tx, therapistID int64) (bool, error) {
	// Lock ID space: Use a fixed prefix (e.g., 1000) + therapistID to avoid collision with other locks?
	// Postgres advisory locks are 64-bit integers.
	// Let's assume therapistID is unique enough or use a prefix.
	// For safety, let's just use therapistID directly.
	var locked bool
	// Use tx.QueryRow
	err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, therapistID).Scan(&locked)
	return locked, err
}

func (r *therapistRepoImpl) SetBatchServices(ctx context.Context, therapistID int64, services []model.AddServiceWithPressuresRequest) error {
	// Cast DBTX to something that can Begin a transaction if it's a pool.
	// In this app, db.WithTransaction is typically used.
	type beginner interface {
		Begin(context.Context) (pgx.Tx, error)
	}
	b, ok := r.db.(beginner)
	if !ok {
		return fmt.Errorf("repository db does not support transactions")
	}

	tx, err := b.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Delete existing
	_, err = tx.Exec(ctx, `DELETE FROM therapist_services WHERE therapist_id = $1`, therapistID)
	if err != nil {
		return err
	}

	// Insert new
	for _, s := range services {
		soft, med, hard := false, false, false
		for _, p := range s.Pressures {
			switch strings.ToLower(p) {
			case "soft":
				soft = true
			case "medium", "med", "moderate":
				med = true
			case "hard":
				hard = true
			}
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO therapist_services (therapist_id, service_id, supports_soft, supports_moderate, supports_hard)
			VALUES ($1, $2, $3, $4, $5)
		`, therapistID, s.ServiceID, soft, med, hard)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
