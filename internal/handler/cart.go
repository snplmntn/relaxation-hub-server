package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// CartHandler handles shopping cart endpoints.
type CartHandler struct {
	cartRepo repository.CartRepository
}

// NewCartHandler creates a new CartHandler.
func NewCartHandler(cartRepo repository.CartRepository) *CartHandler {
	return &CartHandler{cartRepo: cartRepo}
}

// GetCart handles GET /cart - retrieves the current user's cart.
func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	cart, err := h.cartRepo.GetCartByUserID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get cart")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(cart)
}

// AddItem handles POST /cart/items - adds an item to the cart.
func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	defer r.Body.Close()

	var req model.AddToCartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.ServiceID == 0 {
		respondError(w, http.StatusBadRequest, "service_id is required")
		return
	}
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 60 // Default duration
	}
	if req.GuestName == "" {
		req.GuestName = "Self"
	}
	if req.GenderPreference == "" {
		req.GenderPreference = "any"
	}
	if req.PressurePreference == "" {
		req.PressurePreference = "medium"
	}
	if req.StartCondition == "" {
		req.StartCondition = "fixed_time"
	}

	item := &model.CartItem{
		ServiceID:          req.ServiceID,
		GuestName:          req.GuestName,
		DurationMinutes:    req.DurationMinutes,
		GenderPreference:   req.GenderPreference,
		PressurePreference: req.PressurePreference,
		Notes:              req.Notes,
		StartCondition:     req.StartCondition,
		Addons:             req.Addons,
	}

	if err := h.cartRepo.AddItem(r.Context(), userID, item); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add item to cart")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "item added to cart",
		"item":    item,
	})
}

// UpdateItem handles PUT /cart/items/{itemId} - updates a cart item.
func (h *CartHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	itemIDStr := chi.URLParam(r, "itemId")
	itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid item ID")
		return
	}

	defer r.Body.Close()

	var req model.UpdateCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.cartRepo.UpdateItem(r.Context(), itemID, &req); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update item")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "item updated"})
}

// RemoveItem handles DELETE /cart/items/{itemId} - removes an item from cart.
func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	itemIDStr := chi.URLParam(r, "itemId")
	itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid item ID")
		return
	}

	if err := h.cartRepo.RemoveItem(r.Context(), itemID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to remove item")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "item removed"})
}

// ClearCart handles DELETE /cart - clears the entire cart.
func (h *CartHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := h.cartRepo.ClearCart(r.Context(), userID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to clear cart")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "cart cleared"})
}
