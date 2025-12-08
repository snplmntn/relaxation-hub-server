package service

import (
	"context"
	"errors"
)

// GeocodingResult holds geocoding output
type GeocodingResult struct {
	Latitude           float64
	Longitude          float64
	FormattedAddress   string // Normalized address from provider
	Confidence         string // e.g., "high", "medium", "low"
}

// Geocoder defines the geocoding interface
type Geocoder interface {
	// TODO: Implement Geocode - resolve address to coordinates
	// Should bias to Philippines region
	// Return error if zero results or low confidence
	Geocode(ctx context.Context, fullAddress string) (*GeocodingResult, error)
}

// NoopGeocoder is a safe default that indicates geocoding is not configured.
type NoopGeocoder struct{}

func NewNoopGeocoder() Geocoder {
	return &NoopGeocoder{}
}

func (n *NoopGeocoder) Geocode(ctx context.Context, fullAddress string) (*GeocodingResult, error) {
	return nil, errors.New("geocoding provider not configured")
}

// TODO: Create a provider implementation (e.g., googleGeocoder, mapboxGeocoder)
// type googleGeocoder struct {
// 	apiKey string
// 	client *http.Client
// }

// TODO: Implement NewGeocoder constructor based on config
// func NewGeocoder(provider, apiKey string) Geocoder {
// 	switch provider {
// 	case "google":
// 		return &googleGeocoder{apiKey: apiKey, client: &http.Client{Timeout: 3 * time.Second}}
// 	default:
// 		panic("unsupported geocoding provider")
// 	}
// }

// TODO: Implement caching layer to avoid hitting API for same addresses
// Consider a simple in-memory cache with TTL or Redis
