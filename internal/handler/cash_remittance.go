package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// CashRemittanceHandler exposes therapist cash-on-hand and remittance endpoints.
type CashRemittanceHandler struct {
	service *service.CashRemittanceService
}

func NewCashRemittanceHandler(s *service.CashRemittanceService) *CashRemittanceHandler {
	return &CashRemittanceHandler{service: s}
}

// ListCashOnHand handles GET /api/v1/cash-remittances/on-hand
func (h *CashRemittanceHandler) ListCashOnHand(w http.ResponseWriter, r *http.Request) {
	rows, err := h.service.ListCashOnHand(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load cash on hand")
		return
	}
	if rows == nil {
		rows = []model.TherapistCashOnHand{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": rows})
}

// CreateRemittance handles POST /api/v1/cash-remittances
func (h *CashRemittanceHandler) CreateRemittance(w http.ResponseWriter, r *http.Request) {
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.CreateCashRemittanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	remittance, err := h.service.RemitCash(r.Context(), &req, actorID)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(remittance)
}

// ListHistory handles GET /api/v1/cash-remittances?therapist_id=&limit=
func (h *CashRemittanceHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	therapistID, _ := strconv.ParseInt(r.URL.Query().Get("therapist_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	rows, err := h.service.ListHistory(r.Context(), therapistID, limit)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load remittance history")
		return
	}
	if rows == nil {
		rows = []model.CashRemittance{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": rows})
}
