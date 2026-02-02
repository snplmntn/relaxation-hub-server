package handler

import (
	"encoding/json"
	"net/http"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// LocationHandler handles location validation and service area operations.
type LocationHandler struct {
	locationService *service.LocationService
}

// NewLocationHandler creates a new LocationHandler.
func NewLocationHandler(locationService *service.LocationService) *LocationHandler {
	return &LocationHandler{locationService: locationService}
}

// CheckLocation handles POST /api/v1/location/check
// Validates if a location is serviceable and records interest if not.
// Supports both PSGC codes and city/barangay names (for map-based lookups).
func (h *LocationHandler) CheckLocation(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r) // Optional - guest users allowed

	var req model.CheckLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var result *model.LocationCheckResult
	var err error

	// Prefer name-based lookup (from map geocoding), fall back to code-based
	if req.CityName != "" || req.BarangayName != "" {
		result, err = h.locationService.CheckLocationByName(r.Context(), userID, req.CityName, req.BarangayName)
	} else if req.CityCode != "" {
		result, err = h.locationService.CheckLocationStatus(r.Context(), userID, req.CityCode, req.BarangayCode)
	} else {
		respondError(w, http.StatusBadRequest, "city_code or city_name is required")
		return
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check location")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// RequestCoverage handles POST /api/v1/location/request-coverage
// Records a user's interest in an unsupported area.
func (h *LocationHandler) RequestCoverage(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req model.RecordInterestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PSGCCode == "" {
		respondError(w, http.StatusBadRequest, "psgc_code is required")
		return
	}

	// The CheckLocationStatus already records interest, but this endpoint
	// allows explicit "Request Coverage" button clicks
	result, err := h.locationService.CheckLocationStatus(r.Context(), userID, req.PSGCCode, "")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to record interest")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Thanks! We've noted your interest in this area.",
		"status":  result.Status,
	})
}

// ListCoveredAreas handles GET /api/v1/location/covered
// Returns all areas that are currently serviceable.
func (h *LocationHandler) ListCoveredAreas(w http.ResponseWriter, r *http.Request) {
	areas, err := h.locationService.GetCoveredAreas(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list covered areas")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"areas": areas,
	})
}

// ListTopDemand handles GET /api/v1/location/demand (Admin only)
// Returns areas with the most user interest for expansion planning.
func (h *LocationHandler) ListTopDemand(w http.ResponseWriter, r *http.Request) {
	areas, err := h.locationService.GetTopDemandAreas(r.Context(), 20)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list demand areas")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"areas": areas,
	})
}

// UpdateAreaStatus handles PATCH /api/v1/location/areas/{psgc_code} (Admin only)
// Updates the operational status of a service area.
func (h *LocationHandler) UpdateAreaStatus(w http.ResponseWriter, r *http.Request) {
	// Extract psgc_code from URL path
	// Expecting: /api/v1/location/areas/{psgc_code}
	path := r.URL.Path
	// Simple extraction - find last segment
	var psgcCode string
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			psgcCode = path[i+1:]
			break
		}
	}

	if psgcCode == "" {
		respondError(w, http.StatusBadRequest, "psgc_code is required in URL path")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate status
	status := model.ServiceAreaStatus(req.Status)
	if status != model.ServiceAreaStatusCovered &&
		status != model.ServiceAreaStatusBanned &&
		status != model.ServiceAreaStatusNotSupported {
		respondError(w, http.StatusBadRequest, "status must be 'covered', 'banned', or 'not_supported'")
		return
	}

	if err := h.locationService.SetAreaStatus(r.Context(), psgcCode, status); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update area status")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"psgc_code": psgcCode,
		"status":    status,
	})
}

// CreateServiceArea handles POST /api/v1/location/areas
// Creates or updates a service area.
func (h *LocationHandler) CreateServiceArea(w http.ResponseWriter, r *http.Request) {
	var req model.ServiceArea
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.locationService.CreateServiceArea(r.Context(), &req); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create service area")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"area":    req,
	})
}

