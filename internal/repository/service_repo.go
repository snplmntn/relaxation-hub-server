package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ServiceRepository defines persistence methods for catalog services.
type ServiceRepository interface {
	Create(ctx context.Context, svc *model.Service) error
	ListActive(ctx context.Context) ([]model.Service, error)
}

type serviceRepo struct {
	db *pgxpool.Pool
}

func NewServiceRepository(db *pgxpool.Pool) ServiceRepository {
	return &serviceRepo{db: db}
}

func (r *serviceRepo) Create(ctx context.Context, svc *model.Service) error {
	query := `
		INSERT INTO services (name, description, base_price, min_duration_minutes)
		VALUES ($1, $2, $3, $4)
		RETURNING service_id, created_at
	`

	return r.db.QueryRow(ctx, query,
		svc.Name,
		svc.Description,
		svc.BasePrice,
		svc.MinDurationMinutes,
	).Scan(&svc.ServiceID, &svc.CreatedAt)
}

func (r *serviceRepo) ListActive(ctx context.Context) ([]model.Service, error) {
	rows, err := r.db.Query(ctx, `
		SELECT service_id, name, COALESCE(description, ''), base_price, min_duration_minutes, deleted_at, created_at
		FROM services
		WHERE deleted_at IS NULL
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
			&svc.MinDurationMinutes,
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

