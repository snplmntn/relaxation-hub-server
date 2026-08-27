package service

import (
	"context"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// LandingSettingsService exposes business logic for the landing page settings.
type LandingSettingsService struct {
	repo repository.LandingSettingsRepository
}

// NewLandingSettingsService creates a new landing settings service.
func NewLandingSettingsService(repo repository.LandingSettingsRepository) *LandingSettingsService {
	return &LandingSettingsService{repo: repo}
}

// Get returns the current landing settings.
func (s *LandingSettingsService) Get(ctx context.Context) (*model.LandingSettings, error) {
	return s.repo.Get(ctx)
}

// Update applies the provided field updates after filtering to known columns and
// trimming string values. Unknown keys are ignored.
func (s *LandingSettingsService) Update(ctx context.Context, raw map[string]interface{}) (*model.LandingSettings, error) {
	updates := make(map[string]interface{}, len(raw))
	for key, val := range raw {
		col, ok := model.LandingSettingsColumns[key]
		if !ok {
			continue
		}
		if str, ok := val.(string); ok {
			updates[col] = strings.TrimSpace(str)
		} else {
			updates[col] = val
		}
	}
	return s.repo.Update(ctx, updates)
}
