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
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	promo, err := h.promotionService.Create(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toPromotionResponse(promo))
}

func (h *PromotionHandler) ListActivePromotions(w http.ResponseWriter, r *http.Request) {
	promos, err := h.promotionService.ListActive(r.Context(), time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, "promotion not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toPromotionResponse(promo))
}

func toPromotionResponse(p *model.Promotion) model.PromotionResponse {
	return model.PromotionResponse{
		PromoID:     p.PromoID,
		Code:        p.Code,
		DiscountPct: p.DiscountPct,
		ValidFrom:   p.ValidFrom,
		ValidUntil:  p.ValidUntil,
		UsageLimit:  p.UsageLimit,
		DaysOfWeek:  p.DaysOfWeek,
		StartTime:   p.StartTime,
		EndTime:     p.EndTime,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}
