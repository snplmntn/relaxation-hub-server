package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// ListPayoutBalances returns unified financial status for all therapists and riders.
// GET /api/v1/reports/payouts/balances
func (h *ReportHandler) ListPayoutBalances(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationListPayoutBalances) {
		return
	}

	balances, err := h.ledgerRepo.GetPayoutBalances(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch balances: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": balances,
	})
}

type RecordSettlementRequest struct {
	UserID    int64   `json:"user_id"`
	Role      string  `json:"role"`
	Amount    float64 `json:"amount"`
	Reference string  `json:"reference"`
}

// RecordSettlement adds a settlement entry (therapist only — rider payouts go through ResolveRiderPayoutRequest).
// POST /api/v1/reports/payouts/settle
func (h *ReportHandler) RecordSettlement(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationRecordSettlement) {
		return
	}

	var req RecordSettlementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID <= 0 || req.Amount <= 0 {
		http.Error(w, "Invalid user_id or amount", http.StatusBadRequest)
		return
	}

	if req.Role == string(repository.TargetRoleRider) {
		http.Error(w, "Rider payouts must be approved via /reports/payouts/requests/{id}", http.StatusBadRequest)
		return
	}

	role := repository.TargetRoleTherapist
	if req.Role != "" {
		role = repository.TargetRole(req.Role)
	}

	actorID, _ := middleware.GetUserID(r)

	err := h.ledgerRepo.RecordSettlement(r.Context(), req.UserID, role, req.Amount, req.Reference, actorID)
	if err != nil {
		http.Error(w, "Failed to record settlement: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// ListLedgerEntries returns detailed entries for a period.
// GET /api/v1/reports/ledger/entries
func (h *ReportHandler) ListLedgerEntries(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationListLedgerEntries) {
		return
	}

	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	if startDateStr == "" || endDateStr == "" {
		http.Error(w, "start_date and end_date are required", http.StatusBadRequest)
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		http.Error(w, "Invalid start_date format (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		http.Error(w, "Invalid end_date format", http.StatusBadRequest)
		return
	}
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	entries, err := h.ledgerRepo.ListEntries(r.Context(), startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to list entries: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": entries})
}

// ListRiderPayoutRequests returns pending payout requests from riders.
// GET /api/v1/reports/payouts/requests
func (h *ReportHandler) ListRiderPayoutRequests(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationListRiderPayoutRequests) {
		return
	}

	items, err := h.riderWalletService.ListPendingRiderPayouts(r.Context())
	if err != nil {
		http.Error(w, "Failed to list rider payout requests: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": items})
}

type resolveRiderPayoutRequest struct {
	Status    string `json:"status"`    // "approved" or "rejected"
	Reference string `json:"reference"` // used for approved payouts
}

// ResolveRiderPayoutRequest approves or rejects a rider payout request.
// PATCH /api/v1/reports/payouts/requests/{id}
func (h *ReportHandler) ResolveRiderPayoutRequest(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationResolveRiderPayoutRequest) {
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "Missing transaction ID", http.StatusBadRequest)
		return
	}
	txID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	var req resolveRiderPayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	actorID, _ := middleware.GetUserID(r)

	switch req.Status {
	case "approved":
		if !h.requireSpecificReportDependencies(w, r, reportOperationResolveRiderPayoutRequest, reportDependencyLedgerRepo) {
			return
		}

		// Fetch transaction to get riderID + amount before approving
		tx, err := h.riderWalletService.GetRiderTransaction(ctx, txID)
		if err != nil {
			http.Error(w, "Transaction not found: "+err.Error(), http.StatusNotFound)
			return
		}

		if err := h.riderWalletService.ApprovePayout(ctx, txID); err != nil {
			http.Error(w, "Failed to approve payout: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Record ledger settlement for the rider
		amountPHP := float64(tx.AmountCents) / 100.0
		ref := req.Reference
		if ref == "" {
			ref = "Rider payout approved"
		}
		if err := h.ledgerRepo.RecordSettlement(ctx, tx.RiderID, repository.TargetRoleRider, amountPHP, ref, actorID); err != nil {
			// Log but don't fail — wallet already updated
			_ = err
		}

	case "rejected":
		if err := h.riderWalletService.RejectRiderPayout(ctx, txID); err != nil {
			http.Error(w, "Failed to reject payout: "+err.Error(), http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, `status must be "approved" or "rejected"`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}
