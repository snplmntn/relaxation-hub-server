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
	// GetByKey retrieves a service area by its canonical area key.
	GetByKey(ctx context.Context, areaKey string) (*model.ServiceArea, error)

	// GetByName retrieves a service area by fuzzy name match (case-insensitive, partial match).
	// Useful for matching geocoded names (e.g., "Manila") to canonical area records (e.g., "City of Manila").
	GetByName(ctx context.Context, name string, level model.ServiceAreaLevel) (*model.ServiceArea, error)

	// GetStatusByKey retrieves just the status of an area (optimized for validation).
	GetStatusByKey(ctx context.Context, areaKey string) (model.ServiceAreaStatus, error)

	// ListByStatus returns all areas with a given status.
	ListByStatus(ctx context.Context, status model.ServiceAreaStatus) ([]model.ServiceArea, error)

	// ListAll returns all service areas regardless of status.
	ListAll(ctx context.Context) ([]model.ServiceArea, error)

	// ListTopDemand returns areas sorted by cached_request_count (for expansion planning).
	ListTopDemand(ctx context.Context, limit int) ([]model.ServiceArea, error)

	// UpdateStatus changes the status of an area.
	UpdateStatus(ctx context.Context, areaKey string, status model.ServiceAreaStatus) error

	// UpsertArea creates or updates a service area.
	UpsertArea(ctx context.Context, area *model.ServiceArea) error

	// RecordInterest logs a user's interest in an unsupported area.
	RecordInterest(ctx context.Context, userID int64, areaKey string) error

	// GetInterestCount returns the number of unique users interested in an area.
	GetInterestCount(ctx context.Context, areaKey string) (int, error)

	// ListInterestedUsers returns all user IDs who expressed interest in an area.
	ListInterestedUsers(ctx context.Context, areaKey string) ([]int64, error)
	// ListInterestedUsersPage returns paginated interested users with contact details.
	ListInterestedUsersPage(ctx context.Context, areaKey string, page, limit int) ([]model.AreaInterestedUser, int, error)
}

type serviceAreaRepo struct {
	db db.DBTX
}

// NewServiceAreaRepository creates a new ServiceAreaRepository.
func NewServiceAreaRepository(db db.DBTX) ServiceAreaRepository {
	return &serviceAreaRepo{db: db}
}

func (r *serviceAreaRepo) GetByKey(ctx context.Context, areaKey string) (*model.ServiceArea, error) {
	query := `
		SELECT area_id, area_key, parent_code, name, level, status, lat, lng,
		       cached_request_count, min_booking_minutes, created_at, updated_at
		FROM service_areas
		WHERE area_key = $1
	`
	area := &model.ServiceArea{}
	err := r.db.QueryRow(ctx, query, areaKey).Scan(
		&area.AreaID, &area.AreaKey, &area.ParentCode, &area.Name, &area.Level, &area.Status,
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

	// Use %name% pattern to match "Manila" inside "City of Manila".
	pattern := "%" + name + "%"

	if level != "" {
		query = `
			SELECT area_id, area_key, parent_code, name, level, status, lat, lng,
			       cached_request_count, min_booking_minutes, created_at, updated_at
			FROM service_areas
			WHERE name ILIKE $1 AND level = $2
			ORDER BY length(name) ASC
			LIMIT 1
		`
		args = []any{pattern, level}
	} else {
		query = `
			SELECT area_id, area_key, parent_code, name, level, status, lat, lng,
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
		&area.AreaID, &area.AreaKey, &area.ParentCode, &area.Name, &area.Level, &area.Status,
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

func (r *serviceAreaRepo) GetStatusByKey(ctx context.Context, areaKey string) (model.ServiceAreaStatus, error) {
	query := `SELECT status FROM service_areas WHERE area_key = $1`
	var status model.ServiceAreaStatus
	err := r.db.QueryRow(ctx, query, areaKey).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Area not in the catalog = not_supported.
			return model.ServiceAreaStatusNotSupported, nil
		}
		return "", err
	}
	return status, nil
}

func (r *serviceAreaRepo) ListByStatus(ctx context.Context, status model.ServiceAreaStatus) ([]model.ServiceArea, error) {
	query := `
		SELECT area_id, area_key, parent_code, name, level, status, lat, lng,
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
			&area.AreaID, &area.AreaKey, &area.ParentCode, &area.Name, &area.Level, &area.Status,
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
		SELECT sa.area_id, sa.area_key, sa.parent_code, sa.name, sa.level, sa.status, sa.lat, sa.lng,
		       COUNT(DISTINCT acr.user_id) AS demand_count, sa.min_booking_minutes, sa.created_at, sa.updated_at
		FROM service_areas sa
		JOIN area_coverage_requests acr ON acr.area_key = sa.area_key
		WHERE sa.status = 'not_supported'
		GROUP BY sa.area_id, sa.area_key, sa.parent_code, sa.name, sa.level, sa.status, sa.lat, sa.lng, sa.min_booking_minutes, sa.created_at, sa.updated_at
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
			&area.AreaID, &area.AreaKey, &area.ParentCode, &area.Name, &area.Level, &area.Status,
			&area.Lat, &area.Lng, &area.CachedRequestCount, &area.MinBookingMinutes,
			&area.CreatedAt, &area.UpdatedAt,
		); err != nil {
			return nil, err
		}
		areas = append(areas, area)
	}
	return areas, rows.Err()
}

func (r *serviceAreaRepo) UpdateStatus(ctx context.Context, areaKey string, status model.ServiceAreaStatus) error {
	query := `UPDATE service_areas SET status = $1, updated_at = NOW() WHERE area_key = $2`
	result, err := r.db.Exec(ctx, query, status, areaKey)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrAreaNotFound
	}
	return nil
}

func (r *serviceAreaRepo) ListAll(ctx context.Context) ([]model.ServiceArea, error) {
	query := `
		SELECT area_id, area_key, parent_code, name, level, status, lat, lng,
		       cached_request_count, min_booking_minutes, created_at, updated_at
		FROM service_areas
		ORDER BY status, name
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var areas []model.ServiceArea
	for rows.Next() {
		var area model.ServiceArea
		if err := rows.Scan(
			&area.AreaID, &area.AreaKey, &area.ParentCode, &area.Name, &area.Level, &area.Status,
			&area.Lat, &area.Lng, &area.CachedRequestCount, &area.MinBookingMinutes,
			&area.CreatedAt, &area.UpdatedAt,
		); err != nil {
			return nil, err
		}
		areas = append(areas, area)
	}
	return areas, rows.Err()
}

func (r *serviceAreaRepo) UpsertArea(ctx context.Context, area *model.ServiceArea) error {
	query := `
		INSERT INTO service_areas (area_key, parent_code, name, level, status, lat, lng, min_booking_minutes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (area_key) DO UPDATE SET
			parent_code = COALESCE(EXCLUDED.parent_code, service_areas.parent_code),
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
		area.AreaKey, area.ParentCode, area.Name, area.Level, area.Status,
		area.Lat, area.Lng, area.MinBookingMinutes,
	).Scan(&area.AreaID, &area.CreatedAt, &area.UpdatedAt)
}

func (r *serviceAreaRepo) RecordInterest(ctx context.Context, userID int64, areaKey string) error {
	query := `
		INSERT INTO area_coverage_requests (user_id, area_key)
		VALUES ($1, $2)
		ON CONFLICT (user_id, area_key) DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, userID, areaKey)
	return err
}

func (r *serviceAreaRepo) GetInterestCount(ctx context.Context, areaKey string) (int, error) {
	query := `SELECT COUNT(*) FROM area_coverage_requests WHERE area_key = $1`
	var count int
	err := r.db.QueryRow(ctx, query, areaKey).Scan(&count)
	return count, err
}

func (r *serviceAreaRepo) ListInterestedUsers(ctx context.Context, areaKey string) ([]int64, error) {
	query := `SELECT user_id FROM area_coverage_requests WHERE area_key = $1`
	rows, err := r.db.Query(ctx, query, areaKey)
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

func (r *serviceAreaRepo) ListInterestedUsersPage(ctx context.Context, areaKey string, page, limit int) ([]model.AreaInterestedUser, int, error) {
	if areaKey == "" {
		return nil, 0, fmt.Errorf("area_key is required")
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

	countQuery := `SELECT COUNT(*) FROM area_coverage_requests WHERE area_key = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQuery, areaKey).Scan(&total); err != nil {
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
		WHERE acr.area_key = $1
		ORDER BY acr.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, areaKey, limit, offset)
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
