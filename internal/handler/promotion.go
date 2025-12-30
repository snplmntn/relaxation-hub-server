package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type PromotionHandler struct {
	promotionService *service.PromotionService
}

func NewPromotionHandler(promotionService *service.PromotionService) *PromotionHandler {
	return &PromotionHandler{promotionService: promotionService}
}

func (h *PromotionHandler) CreatePromotion(w http.ResponseWriter, r *http.Request) {
	var req model.CreatePromotionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondValidation(w, http.StatusBadRequest, "invalid_request_body", "invalid request body", nil)
		return
	}

	promo, err := h.promotionService.Create(r.Context(), &req)
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
	json.NewEncoder(w).Encode(toPromotionResponse(promo))
}

func (h *PromotionHandler) ListActivePromotions(w http.ResponseWriter, r *http.Request) {
	promos, err := h.promotionService.ListActive(r.Context(), time.Now())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]model.PromotionResponse, 0, len(promos))
	for i := range promos {
		out = append(out, toPromotionResponse(&promos[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *PromotionHandler) GetPromotionByCode(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	promo, err := h.promotionService.GetByCode(r.Context(), code)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "promotion not found")
			return
		}
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toPromotionResponse(promo))
}

func (h *PromotionHandler) ValidatePromotion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code   string  `json:"code"`
		Amount float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.promotionService.Validate(r.Context(), req.Code, req.Amount)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func toPromotionResponse(p *model.Promotion) model.PromotionResponse {
	return model.PromotionResponse{
		PromoID:        p.PromoID,
		Code:           p.Code,
		DiscountPct:    p.DiscountPct,
		DiscountAmount: p.DiscountAmount,
		ValidFrom:      p.ValidFrom,
		ValidUntil:     p.ValidUntil,
		UsageLimit:     p.UsageLimit,
		DaysOfWeek:     p.DaysOfWeek,
		StartTime:      p.StartTime,
		EndTime:        p.EndTime,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}
