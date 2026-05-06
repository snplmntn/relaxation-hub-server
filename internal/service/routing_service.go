package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// RoutingService handles route calculations between coordinates
type RoutingService interface {
	GetRoute(ctx context.Context, originLat, originLong, destLat, destLong float64, vehicleType string) (*RouteResult, error)
}

type RouteResult struct {
	DurationSeconds float64 `json:"duration"`
	DistanceMeters  float64 `json:"distance"`
	Polyline        string  `json:"geometry,omitempty"` // Encoded polyline if needed
}

type mapboxRoutingService struct {
	apiKey string
	client *http.Client
}

type osrmRoutingService struct {
	baseURL        string
	client         *http.Client
	defaultProfile string
}

func NewMapboxRoutingService(apiKey string) RoutingService {
	return &mapboxRoutingService{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func NewOSRMRoutingService(baseURL, defaultProfile string) RoutingService {
	if baseURL == "" {
		baseURL = "https://router.project-osrm.org"
	}
	if defaultProfile == "" {
		defaultProfile = "driving"
	}
	return &osrmRoutingService{
		baseURL:        strings.TrimSuffix(baseURL, "/"),
		client:         &http.Client{Timeout: 10 * time.Second},
		defaultProfile: defaultProfile,
	}
}

// Mapbox Response Structures
type mapboxDirectionsResponse struct {
	Routes []mapboxRoute `json:"routes"`
	Code   string        `json:"code"`
}

type mapboxRoute struct {
	Duration float64 `json:"duration"`
	Distance float64 `json:"distance"`
	Geometry string  `json:"geometry"`
}

type osrmDirectionsResponse struct {
	Code   string      `json:"code"`
	Routes []osrmRoute `json:"routes"`
}

type osrmRoute struct {
	Duration float64 `json:"duration"`
	Distance float64 `json:"distance"`
	Geometry string  `json:"geometry"`
}

func (s *mapboxRoutingService) GetRoute(ctx context.Context, originLat, originLong, destLat, destLong float64, vehicleType string) (*RouteResult, error) {
	// Map vehicleType to Mapbox profile
	// Mapbox Profiles: mapbox/driving-traffic, mapbox/driving, mapbox/cycling, mapbox/walking
	profile := "mapbox/driving-traffic" // Default to most accurate traffic data

	switch vehicleType {
	case "motorcycle":
		// Mapbox implementation detail: they recommend driving-traffic for motors too,
		// but we could apply a custom speed factor if post-processing.
		// For now, strict API mapping:
		profile = "mapbox/driving-traffic"
	case "bicycle":
		profile = "mapbox/cycling"
	case "walking":
		profile = "mapbox/walking"
	case "car":
		profile = "mapbox/driving-traffic"
	}

	// URL Construction
	// Format: https://api.mapbox.com/directions/v5/{profile}/{coordinates}
	// Coordinates: {longitude},{latitude};{longitude},{latitude}
	url := fmt.Sprintf("https://api.mapbox.com/directions/v5/%s/%f,%f;%f,%f?access_token=%s&overview=simplified",
		profile, originLong, originLat, destLong, destLat, s.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mapbox api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mapbox api returned status: %d", resp.StatusCode)
	}

	var result mapboxDirectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode mapbox response: %w", err)
	}

	if result.Code != "Ok" || len(result.Routes) == 0 {
		return nil, fmt.Errorf("no route found (code: %s)", result.Code)
	}

	route := result.Routes[0]
	return &RouteResult{
		DurationSeconds: route.Duration,
		DistanceMeters:  route.Distance,
		Polyline:        route.Geometry,
	}, nil
}

func (s *osrmRoutingService) GetRoute(ctx context.Context, originLat, originLong, destLat, destLong float64, vehicleType string) (*RouteResult, error) {
	profile := mapOSRMProfile(vehicleType, s.defaultProfile)
	url := fmt.Sprintf("%s/route/v1/%s/%f,%f;%f,%f?overview=simplified&geometries=polyline",
		s.baseURL, profile, originLong, originLat, destLong, destLat)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("osrm api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("osrm api returned status: %d", resp.StatusCode)
	}

	var result osrmDirectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode osrm response: %w", err)
	}

	if result.Code != "Ok" || len(result.Routes) == 0 {
		return nil, fmt.Errorf("no route found (code: %s)", result.Code)
	}

	route := result.Routes[0]
	return &RouteResult{
		DurationSeconds: route.Duration,
		DistanceMeters:  route.Distance,
		Polyline:        route.Geometry,
	}, nil
}

func mapOSRMProfile(vehicleType, fallback string) string {
	switch strings.ToLower(vehicleType) {
	case "motorcycle", "car", "driving":
		return "driving"
	case "bicycle", "bike", "cycling":
		return "cycling"
	case "walking", "foot", "pedestrian":
		return "foot"
	default:
		return fallback
	}
}
