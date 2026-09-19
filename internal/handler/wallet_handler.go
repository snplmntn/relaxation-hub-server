package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	summary, err := h.walletService.GetWalletSummary(r.Context(), userID)
	if err != nil {
		slog.Error("failed to get wallet", "therapist_id", userID, "error", err)
		respondError(w, http.StatusNotFound, "Wallet not found")
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// GetTransactions returns cursor-paginated transaction history.
// GET /api/v1/wallet/transactions?cursor_created_at=...&cursor_id=...&limit=20
func (h *WalletHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	cursor, limit, err := parseKeysetPaginationQuery(r)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	page, err := h.walletService.GetTransactionHistoryKeyset(r.Context(), userID, cursor, limit)
	if err != nil {
		slog.Error("failed to get transactions", "therapist_id", userID, "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to get transactions")
		return
	}

	respondJSON(w, http.StatusOK, page)
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
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req RequestPayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	payout, err := h.walletService.RequestPayout(r.Context(), userID, req.Amount, req.PayoutMethod, req.AccountDetails)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		slog.Error("failed to request payout", "therapist_id", userID, "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to create payout request")
		return
	}

	respondJSON(w, http.StatusCreated, payout)
}

// GetPayoutHistory returns the therapist's payout request history.
// GET /api/v1/wallet/payouts
func (h *WalletHandler) GetPayoutHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	payouts, err := h.walletService.ListPayoutRequests(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list payouts", "therapist_id", userID, "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to list payout requests")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
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
		respondError(w, http.StatusInternalServerError, "Failed to list pending payouts")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"payouts": payouts,
	})
}

// UpdatePayoutRequest is the request body for updating a payout status.
type UpdatePayoutRequest struct {
	Status               string `json:"status"`
	TransactionReference string `json:"transaction_reference"`
	Reason               string `json:"reason"`
}

// UpdatePayout updates a pending payout request (admin only, PATCH /payouts/{id}).
func (h *WalletHandler) UpdatePayout(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	requestID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request ID")
		return
	}

	var req UpdatePayoutRequest
	// Decode is optional to support shims with no body or specialized bodies
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Infers status from path if not provided in body (Shim Case)
	status := req.Status
	if status == "" {
		if strings.HasSuffix(r.URL.Path, "/approve") {
			status = "approved"
		} else if strings.HasSuffix(r.URL.Path, "/reject") {
			status = "rejected"
		}
	}

	switch status {
	case "approved":
		var txnRef *string
		if req.TransactionReference != "" {
			txnRef = &req.TransactionReference
		}
		if err := h.walletService.ApprovePayout(r.Context(), requestID, adminID, txnRef); err != nil {
			if ve, ok := err.(*service.ValidationError); ok {
				respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
				return
			}
			slog.Error("failed to approve payout", "request_id", requestID, "admin_id", adminID, "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to approve payout")
			return
		}
	case "rejected":
		// For rejection shim, we might not have a reason in body. Provide a default if so.
		reason := req.Reason
		if reason == "" && strings.HasSuffix(r.URL.Path, "/reject") {
			reason = "Rejected via legacy admin interface"
		}
		if reason == "" {
			respondError(w, http.StatusBadRequest, "Reason is required for rejection")
			return
		}
		if err := h.walletService.RejectPayout(r.Context(), requestID, adminID, reason); err != nil {
			slog.Error("failed to reject payout", "request_id", requestID, "admin_id", adminID, "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to reject payout")
			return
		}
	default:
		respondError(w, http.StatusBadRequest, "Invalid status. Must be 'approved' or 'rejected'")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": status})
}

// CreateCashAdvanceRequest is the request body for creates a cash advance.
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
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateCashAdvanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.RepaymentRate <= 0 || req.RepaymentRate > 100 {
		req.RepaymentRate = 50.0 // Default 50%
	}

	advance, err := h.walletService.CreateCashAdvance(r.Context(), req.TherapistID, req.Amount, req.RepaymentRate, userID, req.Reason)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		slog.Error("failed to create cash advance", "therapist_id", req.TherapistID, "admin_id", userID, "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to create cash advance")
		return
	}

	respondJSON(w, http.StatusCreated, advance)
}

// GetTherapistWallet gets any therapist's wallet (admin only).
// GET /api/v1/admin/wallet/{therapist_id}
func (h *WalletHandler) GetTherapistWallet(w http.ResponseWriter, r *http.Request) {
	therapistID, err := strconv.ParseInt(chi.URLParam(r, "therapist_id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid therapist ID")
		return
	}

	summary, err := h.walletService.GetWalletSummary(r.Context(), therapistID)
	if err != nil {
		slog.Error("failed to get therapist wallet", "therapist_id", therapistID, "error", err)
		respondError(w, http.StatusNotFound, "Wallet not found")
		return
	}

	respondJSON(w, http.StatusOK, summary)
}
