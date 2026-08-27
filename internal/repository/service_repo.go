package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	dbsqlc "github.com/snplmntn/relaxation-hub-server/internal/db/sqlc"
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
	// Update updates a service
	Update(ctx context.Context, serviceID int64, updates map[string]interface{}) error
	// Delete performs a soft delete on a service
	Delete(ctx context.Context, serviceID int64) error
}

type serviceRepo struct {
	db      db.DBTX
	queries *dbsqlc.Queries
}

func NewServiceRepository(db db.DBTX) ServiceRepository {
	return &serviceRepo{
		db:      db,
		queries: dbsqlc.New(db),
	}
}

func (r *serviceRepo) Create(ctx context.Context, svc *model.Service) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	created, err := r.queries.CreateService(ctx, toCreateServiceParams(svc))
	if err != nil {
		return err
	}

	mapped := toModelService(created)
	*svc = mapped
	return nil
}

func (r *serviceRepo) Update(ctx context.Context, serviceID int64, updates map[string]interface{}) error {
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
	args = append(args, serviceID)

	query := fmt.Sprintf("UPDATE services SET %s WHERE service_id = $%d AND deleted_at IS NULL", strings.Join(setClauses, ", "), argIdx)

	cmd, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows // Or custom ErrNotFound
	}

	return nil
}

func (r *serviceRepo) Delete(ctx context.Context, serviceID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.queries.SoftDeleteService(ctx, serviceID)
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}
	if cmd == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *serviceRepo) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	svc, err := r.queries.GetServiceByID(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	mapped := toModelService(svc)
	return &mapped, nil
}

func (r *serviceRepo) GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	if len(ids) == 0 {
		return []model.Service{}, nil
	}

	svcs, err := r.queries.GetServicesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return toModelServices(svcs), nil
}

func (r *serviceRepo) ListActive(ctx context.Context) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	svcs, err := r.queries.ListActiveServices(ctx)
	if err != nil {
		return nil, err
	}
	return toModelServices(svcs), nil
}

// ListRecentByUser returns distinct services from user's recent bookings (last 30 days, limit 3)
func (r *serviceRepo) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.queries.ListRecentServicesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toModelServicesFromRecentRows(rows), nil
}

// ListPopular returns the most-booked services (completed bookings in last 30 days, limit 3)
func (r *serviceRepo) ListPopular(ctx context.Context) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.queries.ListPopularServices(ctx)
	if err != nil {
		return nil, err
	}
	return toModelServicesFromPopularRows(rows), nil
}

// ListUnavailable returns inactive services (limit 3)
func (r *serviceRepo) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	svcs, err := r.queries.ListUnavailableServices(ctx)
	if err != nil {
		return nil, err
	}
	return toModelServices(svcs), nil
}

func toCreateServiceParams(svc *model.Service) dbsqlc.CreateServiceParams {
	category := strings.TrimSpace(svc.Category)
	description := strings.TrimSpace(svc.Description)
	preview := strings.TrimSpace(svc.PreviewImageURL)
	subtitle := strings.TrimSpace(svc.Subtitle)

	return dbsqlc.CreateServiceParams{
		Name:                svc.Name,
		Description:         &description,
		BasePrice:           svc.BasePrice,
		DurationMinutes:     int32(svc.DurationMinutes),
		Category:            &category,
		IsActive:            &svc.IsActive,
		PreviewImageUrl:     &preview,
		TherapistCommission: svc.TherapistCommission,
		Subtitle:            &subtitle,
		IsFeatured:          svc.IsFeatured,
		FeaturedOrder:       int32(svc.FeaturedOrder),
	}
}

func toModelService(svc dbsqlc.Service) model.Service {
	var deletedAt *time.Time
	if svc.DeletedAt.Valid {
		t := svc.DeletedAt.Time
		deletedAt = &t
	}

	var description string
	if svc.Description != nil {
		description = *svc.Description
	}

	var category string
	if svc.Category != nil {
		category = *svc.Category
	}

	var previewImageURL string
	if svc.PreviewImageUrl != nil {
		previewImageURL = *svc.PreviewImageUrl
	}

	var subtitle string
	if svc.Subtitle != nil {
		subtitle = *svc.Subtitle
	}

	var createdAt time.Time
	if svc.CreatedAt.Valid {
		createdAt = svc.CreatedAt.Time
	}

	isActive := false
	if svc.IsActive != nil {
		isActive = *svc.IsActive
	}

	return model.Service{
		ServiceID:           svc.ServiceID,
		Name:                svc.Name,
		Description:         description,
		BasePrice:           svc.BasePrice,
		DurationMinutes:     int(svc.DurationMinutes),
		Category:            category,
		PreviewImageURL:     previewImageURL,
		TherapistCommission: svc.TherapistCommission,
		Subtitle:            subtitle,
		IsFeatured:          svc.IsFeatured,
		FeaturedOrder:       int(svc.FeaturedOrder),
		IsActive:            isActive,
		DeletedAt:           deletedAt,
		CreatedAt:           createdAt,
	}
}

func toModelServices(svcs []dbsqlc.Service) []model.Service {
	services := make([]model.Service, 0, len(svcs))
	for _, svc := range svcs {
		services = append(services, toModelService(svc))
	}
	return services
}

func toModelServicesFromRecentRows(rows []dbsqlc.ListRecentServicesByUserRow) []model.Service {
	services := make([]model.Service, 0, len(rows))
	for _, row := range rows {
		services = append(services, toModelService(row.Service))
	}
	return services
}

func toModelServicesFromPopularRows(rows []dbsqlc.ListPopularServicesRow) []model.Service {
	services := make([]model.Service, 0, len(rows))
	for _, row := range rows {
		services = append(services, toModelService(row.Service))
	}
	return services
}
