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

type EmergencyAlertHandler struct {
	emergencyAlertService *service.EmergencyAlertService
}

func NewEmergencyAlertHandler(emergencyAlertService *service.EmergencyAlertService) *EmergencyAlertHandler {
	return &EmergencyAlertHandler{emergencyAlertService: emergencyAlertService}
}

func (h *EmergencyAlertHandler) TriggerAlert(w http.ResponseWriter, r *http.Request) {
	var req model.CreateEmergencyAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	alert, err := h.emergencyAlertService.Create(r.Context(), userID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toEmergencyAlertResponse(alert))
}

func (h *EmergencyAlertHandler) GetAlert(w http.ResponseWriter, r *http.Request) {
	alertIDStr := chi.URLParam(r, "id")
	aid, err := strconv.ParseInt(alertIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid alert id")
		return
	}

	alert, err := h.emergencyAlertService.GetByID(r.Context(), aid)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "alert not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toEmergencyAlertResponse(alert))
}

func (h *EmergencyAlertHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	alertIDStr := chi.URLParam(r, "id")
	aid, err := strconv.ParseInt(alertIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid alert id")
		return
	}

	var req model.ResolveEmergencyAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resolverID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	alert, err := h.emergencyAlertService.Resolve(r.Context(), aid, resolverID, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "alert not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toEmergencyAlertResponse(alert))
}

func toEmergencyAlertResponse(a *model.EmergencyAlert) model.EmergencyAlertResponse {
	return model.EmergencyAlertResponse{
		AlertID:        a.AlertID,
		BookingID:      a.BookingID,
		TriggeredBy:    a.TriggeredBy,
		TriggeredAt:    a.TriggeredAt,
		LocationLat:    a.LocationLat,
		LocationLng:    a.LocationLng,
		Status:         a.Status,
		Resolved:       a.Resolved,
		ResolvedAt:     a.ResolvedAt,
		ResolvedBy:     a.ResolvedBy,
		ResolutionNote: a.ResolutionNote,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
}
