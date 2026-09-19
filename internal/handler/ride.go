package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"

	"github.com/go-chi/chi/v5"
)

type RideHandler struct {
	rideService *service.RideService
}

func NewRideHandler(rideService *service.RideService) *RideHandler {
	return &RideHandler{rideService: rideService}
}

func normalizeRideStatus(status string) (string, bool) {
	switch status {
	case "accepted", "declined", "completed":
		return status, true
	case "arrived_pickup", "in_progress", "arrived_dropoff":
		return status, true
	case "arrived":
		return "arrived_pickup", true
	case "picked_up":
		return "in_progress", true
	case "dropped_off":
		return "arrived_dropoff", true
	default:
		return "", false
	}
}

func (h *RideHandler) RequestRide(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PickupLat      float64  `json:"pickup_lat"`
		PickupLong     float64  `json:"pickup_long"`
		PickupAddress  string   `json:"pickup_address"`
		DropoffLat     float64  `json:"dropoff_lat"`
		DropoffLong    float64  `json:"dropoff_long"`
		DropoffAddress string   `json:"dropoff_address"`
		DistanceKm     *float64 `json:"distance_km"`
		BookingID      *int64   `json:"booking_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	passengerID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	ride := &model.Ride{
		PassengerID:    passengerID,
		PickupLat:      req.PickupLat,
		PickupLong:     req.PickupLong,
		PickupAddress:  req.PickupAddress,
		DropoffLat:     req.DropoffLat,
		DropoffLong:    req.DropoffLong,
		DropoffAddress: req.DropoffAddress,
		DistanceKm:     req.DistanceKm,
		BookingID:      req.BookingID,
	}

	createdRide, err := h.rideService.RequestRide(r.Context(), ride)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusCreated, createdRide)
}

func (h *RideHandler) UpdateRide(w http.ResponseWriter, r *http.Request) {
	rideID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ride id")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	riderID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	status, valid := normalizeRideStatus(req.Status)
	if !valid {
		respondError(w, http.StatusBadRequest, "invalid status transition")
		return
	}

	switch status {
	case "accepted":
		err = h.rideService.AcceptRide(r.Context(), rideID, riderID)
	case "declined":
		err = h.rideService.DeclineRide(r.Context(), rideID, riderID)
	case "completed":
		err = h.rideService.UpdateRideStatus(r.Context(), rideID, riderID, "completed")
	case "arrived_pickup", "in_progress", "arrived_dropoff":
		err = h.rideService.UpdateRideStatus(r.Context(), rideID, riderID, status)
	}

	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (h *RideHandler) UpdateRiderLocation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lat  float64 `json:"lat"`
		Long float64 `json:"long"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	if err := h.rideService.UpdateRiderLocationByUserID(r.Context(), userID, req.Lat, req.Long); err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *RideHandler) ToggleOnline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IsOnline bool `json:"is_online"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	if err := h.rideService.ToggleOnlineStatus(r.Context(), userID, req.IsOnline); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"is_online": req.IsOnline})
}
