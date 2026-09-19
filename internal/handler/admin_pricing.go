package handler

import (
	"encoding/json"
	"net/http"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type AdminPricingHandler struct {
	pricingService *service.RidePricingService
}

func NewAdminPricingHandler(pricingService *service.RidePricingService) *AdminPricingHandler {
	return &AdminPricingHandler{pricingService: pricingService}
}

func (h *AdminPricingHandler) GetPricingConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.pricingService.GetConfig(r.Context())
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, config)
}

func (h *AdminPricingHandler) UpdatePricingConfig(w http.ResponseWriter, r *http.Request) {
	var req model.PricingConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.pricingService.UpdateConfig(r.Context(), &req); err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
