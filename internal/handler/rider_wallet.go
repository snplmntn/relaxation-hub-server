package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type RiderWalletHandler struct {
	walletService *service.RiderWalletService
}

func NewRiderWalletHandler(walletService *service.RiderWalletService) *RiderWalletHandler {
	return &RiderWalletHandler{walletService: walletService}
}

// GetWallet retrieves rider's wallet balance
func (h *RiderWalletHandler) GetWallet(w http.ResponseWriter, r *http.Request) {
	// Get rider ID from auth context
	userID, ok := r.Context().Value("user_id").(int64)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	wallet, err := h.walletService.GetWallet(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := model.WalletResponse{
		Balance:             float64(wallet.BalanceCents) / 100,
		TotalEarned:         float64(wallet.TotalEarnedCents) / 100,
		TotalWithdrawn:      float64(wallet.TotalWithdrawnCents) / 100,
		BalanceCents:        wallet.BalanceCents,
		TotalEarnedCents:    wallet.TotalEarnedCents,
		TotalWithdrawnCents: wallet.TotalWithdrawnCents,
	}

	respondJSON(w, http.StatusOK, response)
}

// GetTransactions retrieves rider's transaction history
func (h *RiderWalletHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int64)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Parse pagination
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	transactions, err := h.walletService.GetTransactions(r.Context(), userID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response format
	var response []model.TransactionResponse
	for _, tx := range transactions {
		response = append(response, model.TransactionResponse{
			TransactionID: tx.TransactionID,
			Type:          tx.TransactionType,
			Amount:        float64(tx.AmountCents) / 100,
			AmountCents:   tx.AmountCents,
			RideID:        tx.RideID,
			Status:        tx.Status,
			Description:   tx.Description,
			CreatedAt:     tx.CreatedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"transactions": response,
		"limit":        limit,
		"offset":       offset,
	})
}

// RequestPayout initiates a payout request
func (h *RiderWalletHandler) RequestPayout(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int64)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.RiderPayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.walletService.RequestPayout(r.Context(), userID, req.AmountCents)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Payout request submitted for admin approval",
	})
}

// GetPerformance retrieves rider performance metrics
func (h *RiderWalletHandler) GetPerformance(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int64)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	metrics, err := h.walletService.GetPerformanceMetrics(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := model.PerformanceResponse{
		AcceptanceRate: metrics.AcceptanceRate,
		CompletionRate: metrics.CompletionRate,
		AverageRating:  metrics.AverageRating,
		TotalRides:     metrics.TotalRidesAccepted,
		CompletedRides: metrics.TotalRidesCompleted,
		TodayEarned:    float64(metrics.TodayEarnedCents) / 100,
	}

	respondJSON(w, http.StatusOK, response)
}
