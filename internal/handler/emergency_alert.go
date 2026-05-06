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
	bookingService        *service.BookingService
}

func NewEmergencyAlertHandler(emergencyAlertService *service.EmergencyAlertService, bookingService *service.BookingService) *EmergencyAlertHandler {
	return &EmergencyAlertHandler{emergencyAlertService: emergencyAlertService, bookingService: bookingService}
}

// ListAlerts returns a list of emergency alerts for admin dashboard
func (h *EmergencyAlertHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	alerts, err := h.emergencyAlertService.List(r.Context(), status, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to responses
	responses := make([]model.EmergencyAlertResponse, 0, len(alerts))
	for _, a := range alerts {
		responses = append(responses, toEmergencyAlertResponse(a, nil))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": responses,
		"count":  len(responses),
	})
}

// CountAlerts returns count of alerts by status for admin dashboard KPI
func (h *EmergencyAlertHandler) CountAlerts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	count, err := h.emergencyAlertService.CountByStatus(r.Context(), status)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":  count,
		"status": status,
	})
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

	var bookingResp *model.BookingResponse
	if h.bookingService != nil {
		role, _ := middleware.GetUserRole(r)
		if res, err := h.bookingService.GetBookingWithTimeline(r.Context(), alert.BookingID, alert.TriggeredBy, role); err == nil && res != nil {
			br := toBookingResponse(res.Booking, res.Service, res.Address, nil, "", "", "", "", nil, res.ClientName, res.ClientPhone, res.ClientPhoto, res.ClientGender, res.PromoCode)
			bookingResp = &br
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toEmergencyAlertResponse(alert, bookingResp))
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

	var bookingResp *model.BookingResponse
	if h.bookingService != nil {
		role, _ := middleware.GetUserRole(r)
		if res, err := h.bookingService.GetBookingWithTimeline(r.Context(), alert.BookingID, alert.TriggeredBy, role); err == nil && res != nil {
			br := toBookingResponse(res.Booking, res.Service, res.Address, nil, "", "", "", "", nil, res.ClientName, res.ClientPhone, res.ClientPhoto, res.ClientGender, res.PromoCode)
			bookingResp = &br
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toEmergencyAlertResponse(alert, bookingResp))
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

	var bookingResp *model.BookingResponse
	if h.bookingService != nil {
		role, _ := middleware.GetUserRole(r)
		if res, err := h.bookingService.GetBookingWithTimeline(r.Context(), alert.BookingID, alert.TriggeredBy, role); err == nil && res != nil {
			br := toBookingResponse(res.Booking, res.Service, res.Address, nil, "", "", "", "", nil, res.ClientName, res.ClientPhone, res.ClientPhoto, res.ClientGender, res.PromoCode)
			bookingResp = &br
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toEmergencyAlertResponse(alert, bookingResp))
}
func toEmergencyAlertResponse(a *model.EmergencyAlert, booking *model.BookingResponse) model.EmergencyAlertResponse {
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
		Booking:        booking,
	}
}
