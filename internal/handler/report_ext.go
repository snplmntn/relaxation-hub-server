package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
)

// GetTherapistBalances returns the financial status of all therapists
// GET /admin/reports/payouts/balances
func (h *ReportHandler) ListTherapistBalances(w http.ResponseWriter, r *http.Request) {
	if h.ledgerRepo == nil {
		http.Error(w, "Ledger not configured", http.StatusNotImplemented)
		return
	}

	balances, err := h.ledgerRepo.GetTherapistBalances(r.Context())
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
	TherapistID int64   `json:"therapist_id"`
	Amount      float64 `json:"amount"`
	Reference   string  `json:"reference"`
}

// RecordSettlement adds a settlement entry (payment to therapist)
// POST /admin/reports/payouts/settle
func (h *ReportHandler) RecordSettlement(w http.ResponseWriter, r *http.Request) {
	if h.ledgerRepo == nil {
		http.Error(w, "Ledger not configured", http.StatusNotImplemented)
		return
	}

	var req RecordSettlementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TherapistID <= 0 || req.Amount <= 0 {
		http.Error(w, "Invalid therapist ID or amount", http.StatusBadRequest)
		return
	}

	actorID, _ := middleware.GetUserID(r) // Admin who recorded it

	err := h.ledgerRepo.RecordSettlement(r.Context(), req.TherapistID, req.Amount, req.Reference, actorID)
	if err != nil {
		http.Error(w, "Failed to record settlement: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// ListLedgerEntries returns detailed entries for a period
// GET /admin/reports/ledger/entries
func (h *ReportHandler) ListLedgerEntries(w http.ResponseWriter, r *http.Request) {
	if h.ledgerRepo == nil {
		http.Error(w, "Ledger not configured", http.StatusNotImplemented)
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
	// Adjust end date to end of day to include all entries on that day
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	entries, err := h.ledgerRepo.ListEntries(r.Context(), startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to list entries: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": entries})
}
