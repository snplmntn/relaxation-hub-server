package handler

import (
	"encoding/json"
	"errors"
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

const liveLocationAccessDeniedMessage = "access denied"

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

	respondLiveLocation(w, loc)
}

func (h *LiveLocationHandler) GetBookingLocation(w http.ResponseWriter, r *http.Request) {
	bookingIDStr := chi.URLParam(r, "id")
	bookingID, err := strconv.ParseInt(bookingIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	requestingUserID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	result, err := h.liveLocationService.GetLocationForBooking(r.Context(), bookingID, requestingUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLiveLocationAccessDenied):
			respondError(w, http.StatusForbidden, liveLocationAccessDeniedMessage)
		case errors.Is(err, pgx.ErrNoRows):
			respondError(w, http.StatusNotFound, "location not found")
		default:
			respondError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	respondLiveLocation(w, result.Location)
}

func respondLiveLocation(w http.ResponseWriter, loc *model.LiveLocation) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toLiveLocationResponse(loc))
}

func respondLiveLocationLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(w, http.StatusNotFound, "location not found")
		return
	}
	respondError(w, http.StatusInternalServerError, err.Error())
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
