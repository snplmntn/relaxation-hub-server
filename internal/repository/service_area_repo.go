package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

var (
	ErrAreaNotFound      = errors.New("service area not found")
	ErrDuplicateInterest = errors.New("interest already recorded for this area")
)

// ServiceAreaRepository defines data access methods for service areas.
type ServiceAreaRepository interface {
	// GetByCode retrieves a service area by its PSGC code.
	GetByCode(ctx context.Context, psgcCode string) (*model.ServiceArea, error)

	// GetByName retrieves a service area by fuzzy name match (case-insensitive, partial match).
	// Useful for matching geocoded names (e.g., "Manila") to PSGC names (e.g., "City of Manila").
	GetByName(ctx context.Context, name string, level model.ServiceAreaLevel) (*model.ServiceArea, error)

	// GetStatusByCode retrieves just the status of an area (optimized for validation).
	GetStatusByCode(ctx context.Context, psgcCode string) (model.ServiceAreaStatus, error)

	// ListByStatus returns all areas with a given status.
	ListByStatus(ctx context.Context, status model.ServiceAreaStatus) ([]model.ServiceArea, error)

	// ListTopDemand returns areas sorted by cached_request_count (for expansion planning).
	ListTopDemand(ctx context.Context, limit int) ([]model.ServiceArea, error)

	// UpdateStatus changes the status of an area.
	UpdateStatus(ctx context.Context, psgcCode string, status model.ServiceAreaStatus) error

	// UpsertArea creates or updates a service area.
	UpsertArea(ctx context.Context, area *model.ServiceArea) error

	// RecordInterest logs a user's interest in an unsupported area.
	RecordInterest(ctx context.Context, userID int64, psgcCode string) error

	// GetInterestCount returns the number of unique users interested in an area.
	GetInterestCount(ctx context.Context, psgcCode string) (int, error)

	// ListInterestedUsers returns all user IDs who expressed interest in an area.
	ListInterestedUsers(ctx context.Context, psgcCode string) ([]int64, error)
	// ListInterestedUsersPage returns paginated interested users with contact details.
	ListInterestedUsersPage(ctx context.Context, psgcCode string, page, limit int) ([]model.AreaInterestedUser, int, error)
}

type serviceAreaRepo struct {
	db db.DBTX
}

// NewServiceAreaRepository creates a new ServiceAreaRepository.
func NewServiceAreaRepository(db db.DBTX) ServiceAreaRepository {
	return &serviceAreaRepo{db: db}
}

func (r *serviceAreaRepo) GetByCode(ctx context.Context, psgcCode string) (*model.ServiceArea, error) {
	query := `
		SELECT area_id, psgc_code, parent_code, name, level, status, lat, lng, 
		       cached_request_count, min_booking_minutes, created_at, updated_at
		FROM service_areas 
		WHERE psgc_code = $1
	`
	area := &model.ServiceArea{}
	err := r.db.QueryRow(ctx, query, psgcCode).Scan(
		&area.AreaID, &area.PSGCCode, &area.ParentCode, &area.Name, &area.Level, &area.Status,
		&area.Lat, &area.Lng, &area.CachedRequestCount, &area.MinBookingMinutes,
		&area.CreatedAt, &area.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAreaNotFound
		}
		return nil, err
	}
	return area, nil
}

// GetByName retrieves a service area by fuzzy name match.
// Uses ILIKE for case-insensitive partial matching.
// If level is provided (non-empty), restricts search to that level.
func (r *serviceAreaRepo) GetByName(ctx context.Context, name string, level model.ServiceAreaLevel) (*model.ServiceArea, error) {
	var query string
	var args []any

	// Use %name% pattern to match "Manila" inside "City of Manila"
	pattern := "%" + name + "%"

	if level != "" {
		query = `
			SELECT area_id, psgc_code, parent_code, name, level, status, lat, lng, 
			       cached_request_count, min_booking_minutes, created_at, updated_at
			FROM service_areas 
			WHERE name ILIKE $1 AND level = $2
			ORDER BY length(name) ASC
			LIMIT 1
		`
		args = []any{pattern, level}
	} else {
		query = `
			SELECT area_id, psgc_code, parent_code, name, level, status, lat, lng, 
			       cached_request_count, min_booking_minutes, created_at, updated_at
			FROM service_areas 
			WHERE name ILIKE $1
			ORDER BY length(name) ASC
			LIMIT 1
		`
		args = []any{pattern}
	}

	area := &model.ServiceArea{}
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&area.AreaID, &area.PSGCCode, &area.ParentCode, &area.Name, &area.Level, &area.Status,
		&area.Lat, &area.Lng, &area.CachedRequestCount, &area.MinBookingMinutes,
		&area.CreatedAt, &area.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAreaNotFound
		}
		return nil, err
	}
	return area, nil
}

func (r *serviceAreaRepo) GetStatusByCode(ctx context.Context, psgcCode string) (model.ServiceAreaStatus, error) {
	query := `SELECT status FROM service_areas WHERE psgc_code = $1`
	var status model.ServiceAreaStatus
	err := r.db.QueryRow(ctx, query, psgcCode).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Area not in the catalog = not_supported
			return model.ServiceAreaStatusNotSupported, nil
		}
		return "", err
	}
	return status, nil
}

func (r *serviceAreaRepo) ListByStatus(ctx context.Context, status model.ServiceAreaStatus) ([]model.ServiceArea, error) {
	query := `
		SELECT area_id, psgc_code, parent_code, name, level, status, lat, lng, 
		       cached_request_count, min_booking_minutes, created_at, updated_at
		FROM service_areas 
		WHERE status = $1
		ORDER BY name
	`
	rows, err := r.db.Query(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var areas []model.ServiceArea
	for rows.Next() {
		var area model.ServiceArea
		if err := rows.Scan(
			&area.AreaID, &area.PSGCCode, &area.ParentCode, &area.Name, &area.Level, &area.Status,
			&area.Lat, &area.Lng, &area.CachedRequestCount, &area.MinBookingMinutes,
			&area.CreatedAt, &area.UpdatedAt,
		); err != nil {
			return nil, err
		}
		areas = append(areas, area)
	}
	return areas, rows.Err()
}

func (r *serviceAreaRepo) ListTopDemand(ctx context.Context, limit int) ([]model.ServiceArea, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT sa.area_id, sa.psgc_code, sa.parent_code, sa.name, sa.level, sa.status, sa.lat, sa.lng, 
		       COUNT(DISTINCT acr.user_id) AS demand_count, sa.min_booking_minutes, sa.created_at, sa.updated_at
		FROM service_areas sa
		JOIN area_coverage_requests acr ON acr.psgc_code = sa.psgc_code
		WHERE sa.status = 'not_supported'
		GROUP BY sa.area_id, sa.psgc_code, sa.parent_code, sa.name, sa.level, sa.status, sa.lat, sa.lng, sa.min_booking_minutes, sa.created_at, sa.updated_at
		ORDER BY demand_count DESC, sa.updated_at DESC
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var areas []model.ServiceArea
	for rows.Next() {
		var area model.ServiceArea
		if err := rows.Scan(
			&area.AreaID, &area.PSGCCode, &area.ParentCode, &area.Name, &area.Level, &area.Status,
			&area.Lat, &area.Lng, &area.CachedRequestCount, &area.MinBookingMinutes,
			&area.CreatedAt, &area.UpdatedAt,
		); err != nil {
			return nil, err
		}
		areas = append(areas, area)
	}
	return areas, rows.Err()
}

func (r *serviceAreaRepo) UpdateStatus(ctx context.Context, psgcCode string, status model.ServiceAreaStatus) error {
	query := `UPDATE service_areas SET status = $1, updated_at = NOW() WHERE psgc_code = $2`
	result, err := r.db.Exec(ctx, query, status, psgcCode)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrAreaNotFound
	}
	return nil
}

func (r *serviceAreaRepo) UpsertArea(ctx context.Context, area *model.ServiceArea) error {
	query := `
		INSERT INTO service_areas (psgc_code, parent_code, name, level, status, lat, lng, min_booking_minutes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (psgc_code) DO UPDATE SET
			name = EXCLUDED.name,
			level = EXCLUDED.level,
			status = EXCLUDED.status,
			lat = EXCLUDED.lat,
			lng = EXCLUDED.lng,
			min_booking_minutes = EXCLUDED.min_booking_minutes,
			updated_at = NOW()
		RETURNING area_id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		area.PSGCCode, area.ParentCode, area.Name, area.Level, area.Status,
		area.Lat, area.Lng, area.MinBookingMinutes,
	).Scan(&area.AreaID, &area.CreatedAt, &area.UpdatedAt)
}

func (r *serviceAreaRepo) RecordInterest(ctx context.Context, userID int64, psgcCode string) error {
	query := `
		INSERT INTO area_coverage_requests (user_id, psgc_code)
		VALUES ($1, $2)
		ON CONFLICT (user_id, psgc_code) DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, userID, psgcCode)
	return err
}

func (r *serviceAreaRepo) GetInterestCount(ctx context.Context, psgcCode string) (int, error) {
	query := `SELECT COUNT(*) FROM area_coverage_requests WHERE psgc_code = $1`
	var count int
	err := r.db.QueryRow(ctx, query, psgcCode).Scan(&count)
	return count, err
}

func (r *serviceAreaRepo) ListInterestedUsers(ctx context.Context, psgcCode string) ([]int64, error) {
	query := `SELECT user_id FROM area_coverage_requests WHERE psgc_code = $1`
	rows, err := r.db.Query(ctx, query, psgcCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

func (r *serviceAreaRepo) ListInterestedUsersPage(ctx context.Context, psgcCode string, page, limit int) ([]model.AreaInterestedUser, int, error) {
	if psgcCode == "" {
		return nil, 0, fmt.Errorf("psgc_code is required")
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	countQuery := `SELECT COUNT(*) FROM area_coverage_requests WHERE psgc_code = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQuery, psgcCode).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []model.AreaInterestedUser{}, 0, nil
	}

	query := `
		SELECT acr.user_id,
		       COALESCE(NULLIF(TRIM(u.full_name), ''), u.primary_email, u.primary_phone, CONCAT('User #', acr.user_id::text)),
		       COALESCE(u.primary_email, ''),
		       COALESCE(u.primary_phone, ''),
		       acr.created_at
		FROM area_coverage_requests acr
		LEFT JOIN users u ON u.user_id = acr.user_id
		WHERE acr.psgc_code = $1
		ORDER BY acr.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, psgcCode, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]model.AreaInterestedUser, 0, limit)
	for rows.Next() {
		var item model.AreaInterestedUser
		if err := rows.Scan(&item.UserID, &item.FullName, &item.PrimaryEmail, &item.PrimaryPhone, &item.RequestedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}
