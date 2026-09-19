package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
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
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	wallet, err := h.walletService.GetWallet(r.Context(), userID)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
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
	userID, ok := middleware.GetUserID(r)
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
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	// Convert to response format
	var response []model.TransactionResponse
	for _, tx := range transactions {
		response = append(response, model.TransactionResponse{
			TransactionID:  tx.TransactionID,
			Type:           tx.TransactionType,
			Amount:         float64(tx.AmountCents) / 100,
			AmountCents:    tx.AmountCents,
			RideID:         tx.RideID,
			PayoutMethodID: tx.PayoutMethodID,
			Status:         tx.Status,
			Description:    tx.Description,
			CreatedAt:      tx.CreatedAt,
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
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.RiderPayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.walletService.RequestPayout(r.Context(), userID, req.AmountCents, req.PayoutMethodID)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Payout request submitted for admin approval",
	})
}

// GetPerformance retrieves rider performance metrics
func (h *RiderWalletHandler) GetPerformance(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	metrics, err := h.walletService.GetPerformanceMetrics(r.Context(), userID)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
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

// GetPayoutMethods retrieves rider's payout methods
func (h *RiderWalletHandler) GetPayoutMethods(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	methods, err := h.walletService.GetPayoutMethods(r.Context(), userID)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"payout_methods": methods,
	})
}

// AddPayoutMethod adds a new payout method
func (h *RiderWalletHandler) AddPayoutMethod(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var method model.RiderPayoutMethod
	if err := json.NewDecoder(r.Body).Decode(&method); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	method.RiderID = userID
	err := h.walletService.AddPayoutMethod(r.Context(), &method)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusCreated, method)
}

// DeletePayoutMethod removes a payout method
func (h *RiderWalletHandler) DeletePayoutMethod(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Based on plan: DELETE /rider/payout-methods/{id}
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		respondError(w, http.StatusBadRequest, "missing id parameter")
		return
	}

	methodID, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	err = h.walletService.DeletePayoutMethod(r.Context(), userID, methodID)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Payout method deleted"})
}

// UpdatePayoutMethod updates a rider payout method.
func (h *RiderWalletHandler) UpdatePayoutMethod(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		respondError(w, http.StatusBadRequest, "missing id parameter")
		return
	}

	methodID, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	var method model.RiderPayoutMethod
	if err := json.NewDecoder(r.Body).Decode(&method); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	method.ID = methodID
	method.RiderID = userID
	if err := h.walletService.UpdatePayoutMethod(r.Context(), userID, &method); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusOK, method)
}

// GetSafetyContacts lists rider safety contacts.
func (h *RiderWalletHandler) GetSafetyContacts(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	contacts, err := h.walletService.GetEmergencyContacts(r.Context(), userID)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"contacts": contacts,
	})
}

// AddSafetyContact creates a rider safety contact.
func (h *RiderWalletHandler) AddSafetyContact(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var contact model.RiderEmergencyContact
	if err := json.NewDecoder(r.Body).Decode(&contact); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	contact.RiderID = userID
	if err := h.walletService.AddEmergencyContact(r.Context(), &contact); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusCreated, contact)
}

// UpdateSafetyContact updates a rider safety contact.
func (h *RiderWalletHandler) UpdateSafetyContact(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		respondError(w, http.StatusBadRequest, "missing id parameter")
		return
	}

	contactID, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	var contact model.RiderEmergencyContact
	if err := json.NewDecoder(r.Body).Decode(&contact); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	contact.ContactID = contactID
	contact.RiderID = userID
	if err := h.walletService.UpdateEmergencyContact(r.Context(), userID, &contact); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusOK, contact)
}

// DeleteSafetyContact deletes a rider safety contact.
func (h *RiderWalletHandler) DeleteSafetyContact(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		respondError(w, http.StatusBadRequest, "missing id parameter")
		return
	}

	contactID, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	if err := h.walletService.DeleteEmergencyContact(r.Context(), userID, contactID); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Safety contact deleted"})
}
