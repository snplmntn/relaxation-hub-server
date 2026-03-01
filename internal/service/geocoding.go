package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

// NominatimGeocoder implements Geocoder using Nominatim (OSM) API
type nominatimGeocoder struct {
	baseURL   string
	client    *http.Client
	userAgent string
}

const (
	defaultNominatimBaseURL = "https://nominatim.openstreetmap.org"
	defaultNominatimUA      = "relaxation-hub-server/1.0"
)

func NewMapboxGeocoder(apiKey string) Geocoder {
	return &MapboxGeocoder{
		apiKey: apiKey,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func NewNominatimGeocoder(baseURL, userAgent string) Geocoder {
	if baseURL == "" {
		baseURL = defaultNominatimBaseURL
	}
	if userAgent == "" {
		userAgent = defaultNominatimUA
	}
	return &nominatimGeocoder{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		client:    &http.Client{Timeout: 6 * time.Second},
		userAgent: userAgent,
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

type nominatimSearchResult struct {
	Lat         string  `json:"lat"`
	Lon         string  `json:"lon"`
	DisplayName string  `json:"display_name"`
	Importance  float64 `json:"importance"`
}

func (n *nominatimGeocoder) Geocode(ctx context.Context, fullAddress string) (*GeocodingResult, error) {
	params := url.Values{}
	params.Set("q", fullAddress)
	params.Set("format", "json")
	params.Set("limit", "1")
	params.Set("addressdetails", "0")
	params.Set("countrycodes", "ph")

	endpoint := fmt.Sprintf("%s/search?%s", n.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", n.userAgent)

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocoding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding API returned status %d", resp.StatusCode)
	}

	var results []nominatimSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("no geocoding results found")
	}

	feature := results[0]
	lat, err := strconv.ParseFloat(feature.Lat, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latitude: %w", err)
	}
	lng, err := strconv.ParseFloat(feature.Lon, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse longitude: %w", err)
	}

	return &GeocodingResult{
		Latitude:         lat,
		Longitude:        lng,
		FormattedAddress: feature.DisplayName,
		Confidence:       importanceConfidence(feature.Importance),
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

type nominatimReverseResult struct {
	Lat         string  `json:"lat"`
	Lon         string  `json:"lon"`
	DisplayName string  `json:"display_name"`
	Importance  float64 `json:"importance"`
}

func (n *nominatimGeocoder) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeocodingResult, error) {
	params := url.Values{}
	params.Set("lat", fmt.Sprintf("%f", lat))
	params.Set("lon", fmt.Sprintf("%f", lng))
	params.Set("format", "json")
	params.Set("addressdetails", "1")

	endpoint := fmt.Sprintf("%s/reverse?%s", n.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", n.userAgent)

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reverse geocoding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reverse geocoding API returned status %d", resp.StatusCode)
	}

	var result nominatimReverseResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	parsedLat, err := strconv.ParseFloat(result.Lat, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latitude: %w", err)
	}
	parsedLng, err := strconv.ParseFloat(result.Lon, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse longitude: %w", err)
	}

	return &GeocodingResult{
		Latitude:         parsedLat,
		Longitude:        parsedLng,
		FormattedAddress: result.DisplayName,
		Confidence:       importanceConfidence(result.Importance),
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

func importanceConfidence(score float64) string {
	if score >= 0.9 {
		return "high"
	} else if score >= 0.6 {
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
