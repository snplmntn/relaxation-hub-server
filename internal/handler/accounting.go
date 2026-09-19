package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// AccountingHandler exposes the daily accounting sheet line-item endpoints
// (expenses and therapist tips) that back the daily sales report.
type AccountingHandler struct {
	service *service.AccountingService
}

func NewAccountingHandler(s *service.AccountingService) *AccountingHandler {
	return &AccountingHandler{service: s}
}

// ListExpenses handles GET /api/v1/accounting/expenses?business_date=&branch_id=
func (h *AccountingHandler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	businessDate := r.URL.Query().Get("business_date")
	branchID, _ := strconv.ParseInt(r.URL.Query().Get("branch_id"), 10, 64)

	rows, total, err := h.service.ListExpenses(r.Context(), businessDate, branchID)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		slog.Error("accounting: failed to list expenses",
			"error", err, "business_date", businessDate, "branch_id", branchID)
		respondError(w, http.StatusInternalServerError, "failed to load accounting expenses")
		return
	}
	if rows == nil {
		rows = []model.AccountingExpense{}
	}
	respondJSON(w, http.StatusOK, model.AccountingExpenseListResponse{Data: rows, Total: total})
}

// CreateExpense handles POST /api/v1/accounting/expenses
func (h *AccountingHandler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.CreateAccountingExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.service.CreateExpense(r.Context(), &req, actorID)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		slog.Error("accounting: failed to create expense",
			"error", err, "business_date", req.BusinessDate, "branch_id", req.BranchID, "actor_id", actorID)
		respondError(w, http.StatusInternalServerError, "failed to record accounting expense")
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

// DeleteExpense handles DELETE /api/v1/accounting/expenses/{id}
func (h *AccountingHandler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAccountingPathID(w, r)
	if !ok {
		return
	}

	if err := h.service.DeleteExpense(r.Context(), id); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			respondError(w, http.StatusNotFound, "accounting expense not found")
			return
		}
		slog.Error("accounting: failed to delete expense", "error", err, "expense_id", id)
		respondError(w, http.StatusInternalServerError, "failed to delete accounting expense")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTips handles GET /api/v1/accounting/tips?business_date=&branch_id=
func (h *AccountingHandler) ListTips(w http.ResponseWriter, r *http.Request) {
	businessDate := r.URL.Query().Get("business_date")
	branchID, _ := strconv.ParseInt(r.URL.Query().Get("branch_id"), 10, 64)

	rows, total, err := h.service.ListTips(r.Context(), businessDate, branchID)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		slog.Error("accounting: failed to list tips",
			"error", err, "business_date", businessDate, "branch_id", branchID)
		respondError(w, http.StatusInternalServerError, "failed to load accounting tips")
		return
	}
	if rows == nil {
		rows = []model.AccountingTip{}
	}
	respondJSON(w, http.StatusOK, model.AccountingTipListResponse{Data: rows, Total: total})
}

// CreateTip handles POST /api/v1/accounting/tips
func (h *AccountingHandler) CreateTip(w http.ResponseWriter, r *http.Request) {
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.CreateAccountingTipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.service.CreateTip(r.Context(), &req, actorID)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		slog.Error("accounting: failed to create tip",
			"error", err, "business_date", req.BusinessDate, "branch_id", req.BranchID,
			"therapist_id", req.TherapistID, "actor_id", actorID)
		respondError(w, http.StatusInternalServerError, "failed to record accounting tip")
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

// DeleteTip handles DELETE /api/v1/accounting/tips/{id}
func (h *AccountingHandler) DeleteTip(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAccountingPathID(w, r)
	if !ok {
		return
	}

	if err := h.service.DeleteTip(r.Context(), id); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			respondError(w, http.StatusNotFound, "accounting tip not found")
			return
		}
		slog.Error("accounting: failed to delete tip", "error", err, "tip_id", id)
		respondError(w, http.StatusInternalServerError, "failed to delete accounting tip")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseAccountingPathID reads the {id} path parameter. A non-numeric or
// non-positive id can never identify a row, so it is reported as 404 to match
// the delete contract.
func parseAccountingPathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	if raw == "" {
		raw = r.PathValue("id")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusNotFound, "not found")
		return 0, false
	}
	return id, true
}
