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
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	loc, err := h.liveLocationService.UpdateLocation(r.Context(), userID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toLiveLocationResponse(loc))
}

func (h *LiveLocationHandler) GetLocation(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "user_id")
	uid, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	loc, err := h.liveLocationService.GetByUserID(r.Context(), uid)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "location not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
