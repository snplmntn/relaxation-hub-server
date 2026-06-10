package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// ServiceCatalog exposes business logic for service catalog entries.
type ServiceCatalog struct {
	repo  repository.ServiceRepository
	cache *ServiceCache
}

// NewServiceCatalog creates a new service catalog with the given repository and cache.
func NewServiceCatalog(repo repository.ServiceRepository, cache *ServiceCache) *ServiceCatalog {
	return &ServiceCatalog{repo: repo, cache: cache}
}

func (s *ServiceCatalog) Create(ctx context.Context, req *model.CreateServiceRequest) (*model.Service, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	description := strings.TrimSpace(req.Description)

	if req.BasePrice < 0 {
		return nil, fmt.Errorf("base_price must be non-negative")
	}
	basePrice := req.BasePrice

	duration := req.DurationMinutes
	if duration == 0 {
		duration = 60
	}
	if duration <= 0 {
		return nil, fmt.Errorf("duration_minutes must be positive")
	}

	category := strings.TrimSpace(req.Category)
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	svc := &model.Service{
		Name:            name,
		Description:     description,
		BasePrice:       basePrice,
		DurationMinutes: duration,
		Category:        category,
		IsActive:        isActive,
	}

	if req.PreviewImageURL != nil {
		svc.PreviewImageURL = strings.TrimSpace(*req.PreviewImageURL)
	}

	if req.Subtitle != nil {
		svc.Subtitle = strings.TrimSpace(*req.Subtitle)
	}
	if req.IsFeatured != nil {
		svc.IsFeatured = *req.IsFeatured
	}
	if req.FeaturedOrder != nil {
		svc.FeaturedOrder = *req.FeaturedOrder
	}

	if err := s.repo.Create(ctx, svc); err != nil {
		return nil, err
	}

	// Invalidate cache after creating a new service
	if s.cache != nil {
		s.cache.Invalidate()
	}

	return svc, nil
}

// Update modifies an existing service and invalidates the cache.
func (s *ServiceCatalog) Update(ctx context.Context, serviceID int64, updates map[string]interface{}) (*model.Service, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	// Validate some fields if they are present
	if v, ok := updates["base_price"]; ok {
		if price, ok := v.(float64); ok && price < 0 {
			return nil, fmt.Errorf("base_price must be non-negative")
		}
	}
	if v, ok := updates["duration_minutes"]; ok {
		if duration, ok := v.(float64); ok && duration <= 0 {
			return nil, fmt.Errorf("duration_minutes must be positive")
		}
		// Also handle int if json unmarshaling used int
		if duration, ok := v.(int); ok && duration <= 0 {
			return nil, fmt.Errorf("duration_minutes must be positive")
		}
	}

	if err := s.repo.Update(ctx, serviceID, updates); err != nil {
		return nil, err
	}

	if s.cache != nil {
		s.cache.Invalidate()
	}

	return s.repo.GetByID(ctx, serviceID)
}

// Delete performs a soft delete on a service and invalidates the cache.
func (s *ServiceCatalog) Delete(ctx context.Context, serviceID int64) error {
	if err := s.repo.Delete(ctx, serviceID); err != nil {
		return err
	}

	if s.cache != nil {
		s.cache.Invalidate()
	}

	return nil
}

// ListActive returns all active services (cached).
func (s *ServiceCatalog) ListActive(ctx context.Context) ([]model.Service, error) {
	if s.cache != nil {
		return s.cache.GetActive(func() ([]model.Service, error) {
			return s.repo.ListActive(ctx)
		})
	}
	return s.repo.ListActive(ctx)
}

// ListRecentByUser returns services from user's recent bookings (not cached - user-specific).
func (s *ServiceCatalog) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	return s.repo.ListRecentByUser(ctx, userID)
}

// ListPopular returns the most-booked services (cached).
func (s *ServiceCatalog) ListPopular(ctx context.Context) ([]model.Service, error) {
	if s.cache != nil {
		return s.cache.GetPopular(func() ([]model.Service, error) {
			return s.repo.ListPopular(ctx)
		})
	}
	return s.repo.ListPopular(ctx)
}

// ListUnavailable returns inactive services (cached).
func (s *ServiceCatalog) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	if s.cache != nil {
		return s.cache.GetUnavailable(func() ([]model.Service, error) {
			return s.repo.ListUnavailable(ctx)
		})
	}
	return s.repo.ListUnavailable(ctx)
}
