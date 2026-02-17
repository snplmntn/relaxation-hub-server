package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type LiveLocationHandler struct {
	liveLocationService *service.LiveLocationService
}

func NewLiveLocationHandler(liveLocationService *service.LiveLocationService) *LiveLocationHandler {
	return &LiveLocationHandler{liveLocationService: liveLocationService}
}

func (h *LiveLocationHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	loc, err := h.liveLocationService.UpdateLocation(r.Context(), userID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toLiveLocationResponse(loc))
}

func (h *LiveLocationHandler) GetLocation(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "user_id")
	targetUserID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Security: IDOR Check
	// TODO: Enhance this to allow tracking if there is an active booking/ride.
	// For now, restrict to Admin or Self to stop global leakage.
	requestingUserID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	if role != "admin" && requestingUserID != targetUserID {
		// Strict privacy: only admins and the user themselves can see raw location.
		// If clients need to see therapist location, they should use a Booking-scoped endpoint
		// or we need to inject BookingService here to verify active booking.
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	loc, err := h.liveLocationService.GetByUserID(r.Context(), targetUserID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "location not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toLiveLocationResponse(loc))
}

func toLiveLocationResponse(loc *model.LiveLocation) model.LiveLocationResponse {
	return model.LiveLocationResponse{
		LocationID:  loc.LocationID,
		UserID:      loc.UserID,
		Latitude:    loc.Latitude,
		Longitude:   loc.Longitude,
		LastUpdated: loc.LastUpdated,
	}
}
