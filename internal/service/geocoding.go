package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// GeocodingResult holds geocoding output
type GeocodingResult struct {
	Latitude         float64
	Longitude        float64
	FormattedAddress string // Normalized address from provider
	Confidence       string // e.g., "high", "medium", "low"
}

// Geocoder defines the geocoding interface
type Geocoder interface {
	// Geocode resolves address to coordinates
	Geocode(ctx context.Context, fullAddress string) (*GeocodingResult, error)
	// ReverseGeocode resolves coordinates to address
	ReverseGeocode(ctx context.Context, lat, lng float64) (*GeocodingResult, error)
}

// MapboxGeocoder implements Geocoder using Mapbox Geocoding API
type MapboxGeocoder struct {
	apiKey string
	client *http.Client
}

func NewMapboxGeocoder(apiKey string) Geocoder {
	return &MapboxGeocoder{
		apiKey: apiKey,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

type MapboxGeocodingResponse struct {
	Features []struct {
		Center    []float64 `json:"center"`
		PlaceName string    `json:"place_name"`
		Relevance float64   `json:"relevance"`
	} `json:"features"`
}

func (m *MapboxGeocoder) Geocode(ctx context.Context, fullAddress string) (*GeocodingResult, error) {
	endpoint := fmt.Sprintf(
		"https://api.mapbox.com/geocoding/v5/mapbox.places/%s.json?access_token=%s&country=ph&limit=1",
		url.QueryEscape(fullAddress),
		m.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocoding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding API returned status %d", resp.StatusCode)
	}

	var result MapboxGeocodingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Features) == 0 {
		return nil, errors.New("no geocoding results found")
	}

	feature := result.Features[0]
	return &GeocodingResult{
		Latitude:         feature.Center[1],
		Longitude:        feature.Center[0],
		FormattedAddress: feature.PlaceName,
		Confidence:       determineConfidence(feature.Relevance),
	}, nil
}

func (m *MapboxGeocoder) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeocodingResult, error) {
	endpoint := fmt.Sprintf(
		"https://api.mapbox.com/geocoding/v5/mapbox.places/%f,%f.json?access_token=%s&limit=1&types=address",
		lng, lat, m.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reverse geocoding request failed: %w", err)
	}
	defer resp.Body.Close()

	var result MapboxGeocodingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Features) == 0 {
		// Log warning but return fallback coordinates as address
		// This prevents hard failures when geocoding doesn't find results
		return &GeocodingResult{
			Latitude:         lat,
			Longitude:        lng,
			FormattedAddress: fmt.Sprintf("%.4f, %.4f", lat, lng),
			Confidence:       "low",
		}, nil
	}

	feature := result.Features[0]
	return &GeocodingResult{
		Latitude:         feature.Center[1],
		Longitude:        feature.Center[0],
		FormattedAddress: feature.PlaceName,
		Confidence:       "high",
	}, nil
}

func determineConfidence(relevance float64) string {
	if relevance >= 0.9 {
		return "high"
	} else if relevance >= 0.5 {
		return "medium"
	}
	return "low"
}

// NoopGeocoder is a safe default that indicates geocoding is not configured.
type NoopGeocoder struct{}

func NewNoopGeocoder() Geocoder {
	return &NoopGeocoder{}
}

func (n *NoopGeocoder) Geocode(ctx context.Context, fullAddress string) (*GeocodingResult, error) {
	return nil, errors.New("geocoding provider not configured")
}

func (n *NoopGeocoder) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeocodingResult, error) {
	return nil, errors.New("geocoding provider not configured")
}
