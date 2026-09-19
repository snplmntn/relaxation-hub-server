package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// RiderHandler handles rider-specific operations
type RiderHandler struct {
	rideService *service.RideService
}

// NewRiderHandler creates a new RiderHandler
func NewRiderHandler(rideService *service.RideService) *RiderHandler {
	return &RiderHandler{rideService: rideService}
}

// GetPendingOffers returns all pending ride offers for the authenticated rider
func (h *RiderHandler) GetPendingOffers(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rides, err := h.rideService.GetRiderOffersByUserID(r.Context(), userID)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, rides)
}

// UpdateStatus handles rider online/offline status updates
func (h *RiderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		IsOnline bool `json:"is_online"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.rideService.ToggleOnlineStatus(r.Context(), userID, req.IsOnline); err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// UpdateLocation handles rider GPS location updates
func (h *RiderHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Lat  float64 `json:"lat"`
		Long float64 `json:"long"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.rideService.UpdateRiderLocationByUserID(r.Context(), userID, req.Lat, req.Long); err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// CreateProfile handles rider profile creation
func (h *RiderHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		VehicleType  string `json:"vehicle_type"`
		LicensePlate string `json:"license_plate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.rideService.CreateRiderProfile(r.Context(), userID, req.VehicleType, req.LicensePlate); err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"status": "profile_created"})
}

// UpdateProfile handles rider profile updates (vehicle info)
func (h *RiderHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.UpdateRiderProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := make(map[string]interface{})
	if req.VehicleType != nil {
		updates["vehicle_type"] = *req.VehicleType
	}
	if req.LicensePlate != nil {
		updates["license_plate"] = *req.LicensePlate
	}
	if req.LicenseNumber != nil {
		updates["license_number"] = *req.LicenseNumber
	}
	if req.UsualBranchID != nil {
		if *req.UsualBranchID <= 0 {
			respondError(w, http.StatusBadRequest, "invalid usual_branch_id")
			return
		}
		updates["usual_branch_id"] = *req.UsualBranchID
	}
	if req.UsualLocationLabel != nil {
		updates["usual_location_label"] = strings.TrimSpace(*req.UsualLocationLabel)
	}

	if err := h.rideService.UpdateRiderProfile(r.Context(), userID, updates); err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// GetActiveRide returns the rider's current active ride (if any)
func (h *RiderHandler) GetActiveRide(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ride, err := h.rideService.GetActiveRideForRider(r.Context(), userID)
	if err != nil {
		// It's valid to have no active ride
		respondJSON(w, http.StatusOK, map[string]interface{}{"ride": nil})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"ride": ride})
}

// GetRideHistory returns paginated rider ride history.
func (h *RiderHandler) GetRideHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	status := r.URL.Query().Get("status")

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	rides, hasMore, err := h.rideService.GetRideHistoryForRider(r.Context(), userID, status, limit, offset)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"rides":     rides,
		"limit":     limit,
		"offset":    offset,
		"has_more":  hasMore,
		"next_page": offset + len(rides),
	})
}
