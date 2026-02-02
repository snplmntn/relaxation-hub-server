package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// WalletHandler handles wallet-related HTTP requests.
type WalletHandler struct {
	walletService *service.WalletService
}

// NewWalletHandler creates a new wallet handler.
func NewWalletHandler(ws *service.WalletService) *WalletHandler {
	return &WalletHandler{walletService: ws}
}

// GetWallet returns the therapist's wallet summary.
// GET /api/v1/wallet
func (h *WalletHandler) GetWallet(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	summary, err := h.walletService.GetWalletSummary(r.Context(), userID)
	if err != nil {
		slog.Error("failed to get wallet", "therapist_id", userID, "error", err)
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// GetTransactions returns paginated transaction history.
// GET /api/v1/wallet/transactions?page=1&limit=20
func (h *WalletHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	txns, total, err := h.walletService.GetTransactionHistory(r.Context(), userID, page, limit)
	if err != nil {
		slog.Error("failed to get transactions", "therapist_id", userID, "error", err)
		http.Error(w, "Failed to get transactions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"transactions": txns,
		"total":        total,
		"page":         page,
		"limit":        limit,
	})
}

// RequestPayoutRequest is the request body for payout requests.
type RequestPayoutRequest struct {
	Amount         float64         `json:"amount"`
	PayoutMethod   string          `json:"payout_method"`
	AccountDetails json.RawMessage `json:"account_details"`
}

// RequestPayout creates a new payout request.
// POST /api/v1/wallet/payout
func (h *WalletHandler) RequestPayout(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req RequestPayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	payout, err := h.walletService.RequestPayout(r.Context(), userID, req.Amount, req.PayoutMethod, req.AccountDetails)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ve)
			return
		}
		slog.Error("failed to request payout", "therapist_id", userID, "error", err)
		http.Error(w, "Failed to create payout request", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(payout)
}

// GetPayoutHistory returns the therapist's payout request history.
// GET /api/v1/wallet/payouts
func (h *WalletHandler) GetPayoutHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	payouts, err := h.walletService.ListPayoutRequests(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list payouts", "therapist_id", userID, "error", err)
		http.Error(w, "Failed to list payout requests", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"payouts": payouts,
	})
}

// --- Admin Endpoints ---

// ListPendingPayouts returns all pending payout requests (admin only).
// GET /api/v1/admin/wallet/payouts/pending
func (h *WalletHandler) ListPendingPayouts(w http.ResponseWriter, r *http.Request) {
	payouts, err := h.walletService.ListPendingPayouts(r.Context())
	if err != nil {
		slog.Error("failed to list pending payouts", "error", err)
		http.Error(w, "Failed to list pending payouts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"payouts": payouts,
	})
}

// ApprovePayoutRequest is the request body for approving a payout.
type ApprovePayoutRequest struct {
	TransactionReference string `json:"transaction_reference"`
}

// ApprovePayout approves a pending payout request (admin only).
// POST /api/v1/admin/wallet/payouts/{id}/approve
func (h *WalletHandler) ApprovePayout(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	requestID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var req ApprovePayoutRequest
	json.NewDecoder(r.Body).Decode(&req) // Optional body

	var txnRef *string
	if req.TransactionReference != "" {
		txnRef = &req.TransactionReference
	}

	if err := h.walletService.ApprovePayout(r.Context(), requestID, userID, txnRef); err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ve)
			return
		}
		slog.Error("failed to approve payout", "request_id", requestID, "admin_id", userID, "error", err)
		http.Error(w, "Failed to approve payout", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "approved"})
}

// RejectPayoutRequest is the request body for rejecting a payout.
type RejectPayoutRequest struct {
	Reason string `json:"reason"`
}

// RejectPayout rejects a pending payout request (admin only).
// POST /api/v1/admin/wallet/payouts/{id}/reject
func (h *WalletHandler) RejectPayout(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	requestID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var req RejectPayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Reason == "" {
		http.Error(w, "Reason is required", http.StatusBadRequest)
		return
	}

	if err := h.walletService.RejectPayout(r.Context(), requestID, userID, req.Reason); err != nil {
		slog.Error("failed to reject payout", "request_id", requestID, "admin_id", userID, "error", err)
		http.Error(w, "Failed to reject payout", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "rejected"})
}

// CreateCashAdvanceRequest is the request body for creating a cash advance.
type CreateCashAdvanceRequest struct {
	TherapistID   int64   `json:"therapist_id"`
	Amount        float64 `json:"amount"`
	RepaymentRate float64 `json:"repayment_rate"`
	Reason        string  `json:"reason"`
}

// CreateCashAdvance creates a cash advance for a therapist (admin only).
// POST /api/v1/admin/wallet/advances
func (h *WalletHandler) CreateCashAdvance(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateCashAdvanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepaymentRate <= 0 || req.RepaymentRate > 100 {
		req.RepaymentRate = 50.0 // Default 50%
	}

	advance, err := h.walletService.CreateCashAdvance(r.Context(), req.TherapistID, req.Amount, req.RepaymentRate, userID, req.Reason)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ve)
			return
		}
		slog.Error("failed to create cash advance", "therapist_id", req.TherapistID, "admin_id", userID, "error", err)
		http.Error(w, "Failed to create cash advance", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(advance)
}

// GetTherapistWallet gets any therapist's wallet (admin only).
// GET /api/v1/admin/wallet/{therapist_id}
func (h *WalletHandler) GetTherapistWallet(w http.ResponseWriter, r *http.Request) {
	therapistID, err := strconv.ParseInt(chi.URLParam(r, "therapist_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid therapist ID", http.StatusBadRequest)
		return
	}

	summary, err := h.walletService.GetWalletSummary(r.Context(), therapistID)
	if err != nil {
		slog.Error("failed to get therapist wallet", "therapist_id", therapistID, "error", err)
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}
