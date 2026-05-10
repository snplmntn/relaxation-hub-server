package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type DayViewOrderHandler struct {
	service *service.DayViewOrderService
}

func NewDayViewOrderHandler(service *service.DayViewOrderService) *DayViewOrderHandler {
	return &DayViewOrderHandler{service: service}
}

type upsertDayViewOrderRequest struct {
	ViewKey      string  `json:"view_key"`
	TherapistIDs []int64 `json:"therapist_ids"`
}

type dayViewOrderResponse struct {
	ViewKey          string  `json:"view_key"`
	BusinessDate     string  `json:"business_date"`
	Source           string  `json:"source"`
	TherapistIDs     []int64 `json:"therapist_ids"`
	UpdatedByAdminID *int64  `json:"updated_by_admin_id,omitempty"`
}

func (h *DayViewOrderHandler) GetTherapistOrder(w http.ResponseWriter, r *http.Request) {
	viewKey := strings.TrimSpace(r.URL.Query().Get("view_key"))
	if viewKey == "" {
		respondError(w, http.StatusBadRequest, "view_key is required")
		return
	}

	order, err := h.service.GetOrGenerateOrder(r.Context(), viewKey)
	if err != nil {
		respondError(w, statusFromDayViewOrderError(err), err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dayViewOrderResponse{
		ViewKey:          order.ViewKey,
		BusinessDate:     order.BusinessDate.Format("2006-01-02"),
		Source:           order.Source,
		TherapistIDs:     order.TherapistIDs,
		UpdatedByAdminID: order.UpdatedByAdminID,
	})
}

func (h *DayViewOrderHandler) UpdateTherapistOrder(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req upsertDayViewOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.ViewKey) == "" {
		respondError(w, http.StatusBadRequest, "view_key is required")
		return
	}

	order, err := h.service.SaveManualOrder(r.Context(), req.ViewKey, req.TherapistIDs, adminID)
	if err != nil {
		respondError(w, statusFromDayViewOrderError(err), err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dayViewOrderResponse{
		ViewKey:          order.ViewKey,
		BusinessDate:     order.BusinessDate.Format("2006-01-02"),
		Source:           order.Source,
		TherapistIDs:     order.TherapistIDs,
		UpdatedByAdminID: order.UpdatedByAdminID,
	})
}

func statusFromDayViewOrderError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	msg := err.Error()
	if strings.Contains(msg, "invalid view_key") ||
		strings.Contains(msg, "duplicate therapist_id") ||
		strings.Contains(msg, "is not eligible for") {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
