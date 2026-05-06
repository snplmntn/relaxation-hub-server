package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
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

func (h *PromotionHandler) AdminListPromotions(w http.ResponseWriter, r *http.Request) {
	promos, err := h.promotionService.ListAll(r.Context())
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

func (h *PromotionHandler) UpdatePromotion(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid promotion id")
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_, err = h.promotionService.Update(r.Context(), id, updates)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "promotion not found")
			return
		}
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Ideally return the updated object, but service currently returns nil/old obj
	// Let's just return 200 OK for now
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"updated"}`))
}

func (h *PromotionHandler) DeletePromotion(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid promotion id")
		return
	}

	if err := h.promotionService.Delete(r.Context(), id); err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "promotion not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"deleted"}`))
}

func toPromotionResponse(p *model.Promotion) model.PromotionResponse {
	return model.PromotionResponse{
		PromoID:        p.PromoID,
		Code:           p.Code,
		DiscountPct:    p.DiscountPct,
		DiscountAmount: p.DiscountAmount,
		AppliesTo:      p.AppliesTo,
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
