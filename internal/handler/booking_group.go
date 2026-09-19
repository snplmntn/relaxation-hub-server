package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// BookingGroupHandler handles booking group operations.
type BookingGroupHandler struct {
	groupService *service.BookingGroupService
	productRepo  repository.ProductRepository
}

// NewBookingGroupHandler creates a new BookingGroupHandler.
func NewBookingGroupHandler(groupService *service.BookingGroupService, productRepo repository.ProductRepository) *BookingGroupHandler {
	return &BookingGroupHandler{groupService: groupService, productRepo: productRepo}
}

// CreateBookingGroup handles POST /api/v1/booking-groups
func (h *BookingGroupHandler) CreateBookingGroup(w http.ResponseWriter, r *http.Request) {
	h.createBookingGroup(w, r, false)
}

func (h *BookingGroupHandler) PreviewVoucher(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.CreateBookingGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	role, _ := middleware.GetUserRole(r)
	result, err := h.groupService.PreviewVoucher(r.Context(), clientID, &req, !isAdminOperationalRole(role))
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// CreateBookingGroupAsAdmin handles POST /api/v1/admin/booking-groups
// and requires explicit client_id in the request body.
func (h *BookingGroupHandler) CreateBookingGroupAsAdmin(w http.ResponseWriter, r *http.Request) {
	h.createBookingGroup(w, r, true)
}

func (h *BookingGroupHandler) createBookingGroup(w http.ResponseWriter, r *http.Request, requireClientID bool) {
	requestingUserID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)
	isAdminActor := role == "admin" || role == "super_admin"

	var req model.CreateBookingGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if isAdminActor {
		req.BookingSource = model.BookingSourceStaffWeb
	}

	if requireClientID && req.ClientID == nil {
		respondError(w, http.StatusBadRequest, "client_id is required")
		return
	}

	effectiveClientID := requestingUserID
	if req.ClientID != nil {
		if *req.ClientID <= 0 {
			respondError(w, http.StatusBadRequest, "invalid client_id")
			return
		}

		if isAdminActor {
			effectiveClientID = *req.ClientID
		} else if *req.ClientID != requestingUserID {
			respondError(w, http.StatusForbidden, "cannot create booking group for another client")
			return
		}
	}

	var group *model.BookingGroup
	var err error
	if isAdminActor {
		group, err = h.groupService.CreateBookingGroup(r.Context(), effectiveClientID, requestingUserID, &req, false)
	} else {
		group, err = h.groupService.CreateCustomerBookingGroup(r.Context(), effectiveClientID, requestingUserID, &req)
	}
	if err != nil {
		var blockErr *service.BlockedAssignmentError
		if errors.As(err, &blockErr) {
			respondValidation(w, http.StatusConflict, "therapist_blocked", blockErr.Error(), map[string]string{"therapist_id": "blocked"})
			return
		}
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(group)
}

// GetBookingGroup handles GET /api/v1/booking-groups/{id}
func (h *BookingGroupHandler) GetBookingGroup(w http.ResponseWriter, r *http.Request) {
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	actorRole, _ := middleware.GetUserRole(r)

	groupIDStr := chi.URLParam(r, "id")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid group_id")
		return
	}

	group, err := h.groupService.GetGroupByID(r.Context(), groupID, actorID, actorRole)
	if err != nil {
		respondError(w, http.StatusNotFound, "booking group not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

// ListProducts handles GET /api/v1/products
func (h *BookingGroupHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.productRepo.ListActive(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"products": products,
	})
}

// GetProduct handles GET /api/v1/products/{id}
func (h *BookingGroupHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product_id")
		return
	}

	product, err := h.productRepo.GetByID(r.Context(), productID)
	if err != nil {
		respondError(w, http.StatusNotFound, "product not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}
