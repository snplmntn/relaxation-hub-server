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
	repo repository.ServiceRepository
}

func NewServiceCatalog(repo repository.ServiceRepository) *ServiceCatalog {
	return &ServiceCatalog{repo: repo}
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

	minDuration := req.MinDurationMinutes
	if minDuration == 0 {
		minDuration = 60
	}
	if minDuration <= 0 {
		return nil, fmt.Errorf("min_duration_minutes must be positive")
	}

	svc := &model.Service{
		Name:               name,
		Description:        description,
		BasePrice:          basePrice,
		MinDurationMinutes: minDuration,
	}

	if err := s.repo.Create(ctx, svc); err != nil {
		return nil, err
	}

	return svc, nil
}

func (s *ServiceCatalog) ListActive(ctx context.Context) ([]model.Service, error) {
	return s.repo.ListActive(ctx)
}

