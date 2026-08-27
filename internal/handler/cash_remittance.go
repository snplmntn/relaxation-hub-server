package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

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
// Optional query params: date_from, date_to (RFC3339). When provided, the
// per-method breakdown columns are scoped to that range; cash-on-hand stays
// all-time so the remittance workflow is unaffected.
func (h *CashRemittanceHandler) ListCashOnHand(w http.ResponseWriter, r *http.Request) {
	dateFrom := parseOptionalTime(r.URL.Query().Get("date_from"))
	dateTo := parseOptionalTime(r.URL.Query().Get("date_to"))

	rows, err := h.service.ListCashOnHand(r.Context(), dateFrom, dateTo)
	if err != nil {
		slog.Error("cash_remittance: failed to list cash on hand", "error", err)
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
		slog.Error("cash_remittance: failed to remit cash",
			"error", err, "therapist_id", req.TherapistID, "actor_id", actorID)
		respondError(w, http.StatusInternalServerError, "failed to record remittance")
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
		slog.Error("cash_remittance: failed to list remittance history", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load remittance history")
		return
	}
	if rows == nil {
		rows = []model.CashRemittance{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": rows})
}

// ListRemittanceLog handles GET /api/v1/cash-remittances/logs
// The "vault" view: lists remittances with per-admin and grand totals.
// Super-admins see all admins (or just their own with ?mine=true); regular
// admins are always scoped to the remittances they personally recorded.
func (h *CashRemittanceHandler) ListRemittanceLog(w http.ResponseWriter, r *http.Request) {
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	// Role scoping: super-admin may view everyone; everyone else is forced to self.
	var remittedBy *int64
	if role != model.RoleSuperAdmin || r.URL.Query().Get("mine") == "true" {
		id := actorID
		remittedBy = &id
	}

	dateFrom := parseOptionalTime(r.URL.Query().Get("date_from"))
	dateTo := parseOptionalTime(r.URL.Query().Get("date_to"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	rows, total, byAdmin, err := h.service.ListRemittanceLog(r.Context(), remittedBy, dateFrom, dateTo, limit)
	if err != nil {
		slog.Error("cash_remittance: failed to list remittance log", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load remittance log")
		return
	}
	if rows == nil {
		rows = []model.CashRemittance{}
	}
	if byAdmin == nil {
		byAdmin = []model.AdminRemittanceTotal{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":         rows,
		"total_amount": total,
		"by_admin":     byAdmin,
	})
}

func parseOptionalTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	// RFC3339Nano handles both "...Z" and "...000Z" (toISOString() format).
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}
