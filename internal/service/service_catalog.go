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

	if err := s.repo.Create(ctx, svc); err != nil {
		return nil, err
	}

	// Invalidate cache after creating a new service
	if s.cache != nil {
		s.cache.Invalidate()
	}

	return svc, nil
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
