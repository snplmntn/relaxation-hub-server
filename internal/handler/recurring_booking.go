package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// RecurringBookingHandler handles recurring booking series operations.
type RecurringBookingHandler struct {
	recurringService *service.RecurringBookingService
}

// NewRecurringBookingHandler creates a new RecurringBookingHandler.
func NewRecurringBookingHandler(s *service.RecurringBookingService) *RecurringBookingHandler {
	return &RecurringBookingHandler{recurringService: s}
}

// Create handles POST /api/v1/recurring-bookings
func (h *RecurringBookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.CreateRecurringBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	series, err := h.recurringService.CreateSeries(r.Context(), actorID, &req)
	if err != nil {
		var blockErr *service.BlockedAssignmentError
		if errors.As(err, &blockErr) {
			respondValidation(w, http.StatusConflict, "therapist_blocked", blockErr.Error(), map[string]string{"therapist_id": "blocked"})
			return
		}
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(series)
}

// List handles GET /api/v1/recurring-bookings
func (h *RecurringBookingHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	clientIDStr := r.URL.Query().Get("client_id")

	limit := 20
	offset := 0
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
		limit = v
	}
	if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
		offset = v
	}

	var clientID *int64
	if clientIDStr != "" {
		if v, err := strconv.ParseInt(clientIDStr, 10, 64); err == nil {
			clientID = &v
		}
	}

	series, total, err := h.recurringService.ListSeries(r.Context(), status, clientID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list recurring bookings")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"recurring_bookings": series,
		"total":              total,
		"limit":              limit,
		"offset":             offset,
	})
}

// GetByID handles GET /api/v1/recurring-bookings/{id}
func (h *RecurringBookingHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseRecurringID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid recurring_id")
		return
	}

	series, err := h.recurringService.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "recurring booking not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(series)
}

// Update handles PATCH /api/v1/recurring-bookings/{id}
func (h *RecurringBookingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseRecurringID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid recurring_id")
		return
	}

	var req model.UpdateRecurringBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	series, err := h.recurringService.UpdateSeries(r.Context(), id, &req)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(series)
}

func parseRecurringID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}
