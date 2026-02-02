package service

import (
	"context"
	"math"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type RidePricingService struct {
	db db.DBTX
}

func NewRidePricingService(db db.DBTX) *RidePricingService {
	return &RidePricingService{db: db}
}

// GetConfig fetches the active pricing configuration (default key for now).
func (s *RidePricingService) GetConfig(ctx context.Context) (*model.PricingConfig, error) {
	query := `
		SELECT config_id, config_key, base_distance_km, base_rate, per_km_rate, per_100m_rate, 
		       min_fare, max_fare, surge_enabled, surge_multiplier, updated_at
		FROM ride_pricing_config
		WHERE config_key = 'default'
	`
	var c model.PricingConfig
	err := s.db.QueryRow(ctx, query).Scan(
		&c.ConfigID, &c.ConfigKey, &c.BaseDistanceKm, &c.BaseRate, &c.PerKmRate, &c.Per100mRate,
		&c.MinFare, &c.MaxFare, &c.SurgeEnabled, &c.SurgeMultiplier, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateConfig updates the pricing configuration.
func (s *RidePricingService) UpdateConfig(ctx context.Context, config *model.PricingConfig) error {
	query := `
		UPDATE ride_pricing_config
		SET base_distance_km = $1, base_rate = $2, per_km_rate = $3, per_100m_rate = $4,
		    min_fare = $5, max_fare = $6, surge_enabled = $7, surge_multiplier = $8,
		    updated_at = NOW()
		WHERE config_key = 'default'
	`
	_, err := s.db.Exec(ctx, query,
		config.BaseDistanceKm, config.BaseRate, config.PerKmRate, config.Per100mRate,
		config.MinFare, config.MaxFare, config.SurgeEnabled, config.SurgeMultiplier,
	)
	return err
}

// CalculateFare implements SOTA 2026 distance-based pricing with surge support.
func (s *RidePricingService) CalculateFare(distanceKm float64, config *model.PricingConfig) *model.RidePricing {
	if config == nil {
		// Fallback defaults if verification fails or testing
		config = &model.PricingConfig{
			BaseDistanceKm: 3.0, BaseRate: 50.0, PerKmRate: 10.0, SurgeMultiplier: 1.0, MaxFare: 10000,
		}
	}
	var fare float64

	// Base Fare Logic
	if distanceKm <= config.BaseDistanceKm {
		fare = config.BaseRate
	} else {
		extraKm := distanceKm - config.BaseDistanceKm
		fare = config.BaseRate + (extraKm * config.PerKmRate)
	}

	// Apply surge multiplier if enabled
	actualSurge := 1.0
	if config.SurgeEnabled {
		actualSurge = config.SurgeMultiplier
		fare *= actualSurge
	}

	// Enforce min/max bounds
	fare = math.Max(config.MinFare, math.Min(fare, config.MaxFare))
	finalFare := math.Round(fare*100) / 100 // Round to 2 decimals

	return &model.RidePricing{
		BaseRate:        config.BaseRate,
		PerKmRate:       config.PerKmRate,
		SurgeMultiplier: actualSurge,
		FinalFare:       finalFare,
	}
}
