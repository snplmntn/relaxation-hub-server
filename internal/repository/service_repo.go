package repository

import (
	"context"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ServiceRepository defines persistence methods for catalog services.
type ServiceRepository interface {
	Create(ctx context.Context, svc *model.Service) error
	GetByID(ctx context.Context, serviceID int64) (*model.Service, error)
	GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error)
	ListActive(ctx context.Context) ([]model.Service, error)
	// ListRecentByUser returns distinct services from user's recent bookings (limit 3)
	ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error)
	// ListPopular returns the most-booked services across all users (limit 3)
	ListPopular(ctx context.Context) ([]model.Service, error)
	// ListUnavailable returns inactive services (limit 3)
	ListUnavailable(ctx context.Context) ([]model.Service, error)
}

type serviceRepo struct {
	db db.DBTX
}

func NewServiceRepository(db db.DBTX) ServiceRepository {
	return &serviceRepo{db: db}
}

func (r *serviceRepo) Create(ctx context.Context, svc *model.Service) error {
	query := `
		INSERT INTO services (name, description, base_price, duration_minutes, category, is_active, preview_image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING service_id, created_at, is_active, preview_image_url
	`

	return r.db.QueryRow(ctx, query,
		svc.Name,
		svc.Description,
		svc.BasePrice,
		svc.DurationMinutes,
		svc.Category,
		svc.IsActive,
		svc.PreviewImageURL,
	).Scan(&svc.ServiceID, &svc.CreatedAt, &svc.IsActive, &svc.PreviewImageURL)
}

func (r *serviceRepo) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	var svc model.Service
	err := r.db.QueryRow(ctx, `
		SELECT service_id, name, COALESCE(description, ''), base_price, duration_minutes, 
		       COALESCE(category, ''), is_active, COALESCE(preview_image_url, ''), deleted_at, created_at
		FROM services
		WHERE service_id = $1 AND deleted_at IS NULL
	`, serviceID).Scan(
		&svc.ServiceID,
		&svc.Name,
		&svc.Description,
		&svc.BasePrice,
		&svc.DurationMinutes,
		&svc.Category,
		&svc.IsActive,
		&svc.PreviewImageURL,
		&svc.DeletedAt,
		&svc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &svc, nil
}

func (r *serviceRepo) GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error) {
	if len(ids) == 0 {
		return []model.Service{}, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT service_id, name, COALESCE(description, ''), base_price, duration_minutes, 
		       COALESCE(category, ''), is_active, COALESCE(preview_image_url, ''), deleted_at, created_at
		FROM services
		WHERE service_id = ANY($1) AND deleted_at IS NULL
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanServices(rows)
}

func (r *serviceRepo) ListActive(ctx context.Context) ([]model.Service, error) {
	rows, err := r.db.Query(ctx, `
		SELECT service_id, name, COALESCE(description, ''), base_price, duration_minutes, COALESCE(category, ''), is_active, COALESCE(preview_image_url, ''), deleted_at, created_at
		FROM services
		WHERE deleted_at IS NULL AND is_active = TRUE
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []model.Service
	for rows.Next() {
		var svc model.Service
		if err := rows.Scan(
			&svc.ServiceID,
			&svc.Name,
			&svc.Description,
			&svc.BasePrice,
			&svc.DurationMinutes,
			&svc.Category,
			&svc.IsActive,
			&svc.PreviewImageURL,
			&svc.DeletedAt,
			&svc.CreatedAt,
		); err != nil {
			return nil, err
		}
		services = append(services, svc)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return services, nil
}

// ListRecentByUser returns distinct services from user's recent bookings (last 30 days, limit 3)
func (r *serviceRepo) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.service_id, s.name, COALESCE(s.description, ''), s.base_price, 
			s.duration_minutes, COALESCE(s.category, ''), s.is_active, 
			COALESCE(s.preview_image_url, ''), s.deleted_at, s.created_at
		FROM services s
		INNER JOIN (
			SELECT service_id, MAX(created_at) as last_booked
			FROM bookings
			WHERE client_id = $1
			GROUP BY service_id
		) latest_b ON s.service_id = latest_b.service_id
		WHERE s.deleted_at IS NULL
		  AND latest_b.last_booked > NOW() - INTERVAL '30 days'
		ORDER BY latest_b.last_booked DESC
		LIMIT 3
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanServices(rows)
}

// ListPopular returns the most-booked services (completed bookings in last 30 days, limit 3)
func (r *serviceRepo) ListPopular(ctx context.Context) ([]model.Service, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.service_id, s.name, COALESCE(s.description, ''), s.base_price, 
			s.duration_minutes, COALESCE(s.category, ''), s.is_active, 
			COALESCE(s.preview_image_url, ''), s.deleted_at, s.created_at
		FROM services s
		INNER JOIN bookings b ON b.service_id = s.service_id
		WHERE s.deleted_at IS NULL 
		  AND s.is_active = true
		  AND b.status = 'completed'
		  AND b.created_at > NOW() - INTERVAL '30 days'
		GROUP BY s.service_id, s.name, s.description, s.base_price, 
			s.duration_minutes, s.category, s.is_active, s.preview_image_url, 
			s.deleted_at, s.created_at
		ORDER BY COUNT(b.booking_id) DESC
		LIMIT 3
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanServices(rows)
}

// ListUnavailable returns inactive services (limit 3)
func (r *serviceRepo) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	rows, err := r.db.Query(ctx, `
		SELECT service_id, name, COALESCE(description, ''), base_price, 
			duration_minutes, COALESCE(category, ''), is_active, 
			COALESCE(preview_image_url, ''), deleted_at, created_at
		FROM services
		WHERE is_active = false AND deleted_at IS NULL
		ORDER BY name ASC
		LIMIT 3
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanServices(rows)
}

// scanServices is a helper to scan service rows into a slice
func scanServices(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]model.Service, error) {
	var services []model.Service
	for rows.Next() {
		var svc model.Service
		if err := rows.Scan(
			&svc.ServiceID,
			&svc.Name,
			&svc.Description,
			&svc.BasePrice,
			&svc.DurationMinutes,
			&svc.Category,
			&svc.IsActive,
			&svc.PreviewImageURL,
			&svc.DeletedAt,
			&svc.CreatedAt,
		); err != nil {
			return nil, err
		}
		services = append(services, svc)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return services, nil
}
