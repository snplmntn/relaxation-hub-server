package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type PartnerHotelHandler struct {
	service *service.PartnerHotelService
}

func (h *PartnerHotelHandler) MyAccess(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Sign in required")
		return
	}
	access, err := h.service.GetAccess(r.Context(), id)
	if err != nil {
		respondHotelAccessError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, access)
}

func (h *PartnerHotelHandler) MyStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Sign in required")
		return
	}
	staff, err := h.service.ListMyHotelStaff(r.Context(), id)
	if err != nil {
		respondHotelAccessError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, staff)
}

func respondHotelAccessError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrHotelAccessDenied) {
		respondError(w, http.StatusForbidden, err.Error())
		return
	}
	respondServiceError(w, http.StatusInternalServerError, err)
}

// Hotel accounts can only use the initial hotel workspace and read their profile.
// Resolve access on every request so revocation and role changes take effect immediately.
func (h *PartnerHotelHandler) RestrictHotelAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := middleware.GetUserRole(r)
		if !model.IsHotelRole(role) {
			next.ServeHTTP(w, r)
			return
		}
		id, _ := middleware.GetUserID(r)
		if _, err := h.service.GetAccess(r.Context(), id); err != nil {
			respondHotelAccessError(w, err)
			return
		}
		if !hotelAccountRequestAllowed(r.Method, r.URL.Path) {
			respondError(w, http.StatusForbidden, "This action is not available to hotel accounts")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hotelAccountRequestAllowed(method, path string) bool {
	path = strings.TrimSuffix(path, "/")
	if method == http.MethodGet {
		switch path {
		case "/api/v1/users/profile", "/api/v1/hotel/access", "/api/v1/hotel/staff", "/api/v1/hotel/day-view", "/api/v1/hotel/bookings", "/api/v1/hotel/analytics", "/api/v1/addresses", "/api/v1/services", "/api/v1/products":
			return true
		}
	}
	if method == http.MethodPost {
		switch path {
		case "/api/v1/bookings", "/api/v1/booking-groups", "/api/v1/addresses", "/api/v1/availability/booking":
			return true
		}
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/hotel/bookings/"), "/")
	if strings.HasPrefix(path, "/api/v1/hotel/bookings/") {
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err == nil && id > 0 {
			return method == http.MethodPatch && len(parts) == 1 || method == http.MethodPost && len(parts) == 2 && parts[1] == "cancel"
		}
	}
	return false
}

func NewPartnerHotelHandler(partnerHotelService *service.PartnerHotelService) *PartnerHotelHandler {
	return &PartnerHotelHandler{service: partnerHotelService}
}

func (h *PartnerHotelHandler) ListHotels(w http.ResponseWriter, r *http.Request) {
	hotels, err := h.service.ListHotels(r.Context())
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusOK, hotels)
}

func (h *PartnerHotelHandler) CreateHotel(w http.ResponseWriter, r *http.Request) {
	var req model.CreatePartnerHotelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	hotel, err := h.service.CreateHotel(r.Context(), &req)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusCreated, hotel)
}

func (h *PartnerHotelHandler) UpdateHotel(w http.ResponseWriter, r *http.Request) {
	hotelID, ok := parsePositivePathID(w, r, "hotelID", "hotel")
	if !ok {
		return
	}
	var req model.UpdatePartnerHotelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	hotel, err := h.service.UpdateHotel(r.Context(), hotelID, &req)
	if err != nil {
		respondPartnerHotelError(w, err, "partnered hotel")
		return
	}
	respondJSON(w, http.StatusOK, hotel)
}

func (h *PartnerHotelHandler) ListStaff(w http.ResponseWriter, r *http.Request) {
	hotelID, ok := parsePositivePathID(w, r, "hotelID", "hotel")
	if !ok {
		return
	}
	staffMembers, err := h.service.ListStaff(r.Context(), hotelID)
	if err != nil {
		respondPartnerHotelError(w, err, "partnered hotel")
		return
	}
	respondJSON(w, http.StatusOK, staffMembers)
}

func (h *PartnerHotelHandler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	hotelID, ok := parsePositivePathID(w, r, "hotelID", "hotel")
	if !ok {
		return
	}
	var req model.CreatePartnerHotelStaffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	staff, err := h.service.CreateStaff(r.Context(), hotelID, &req)
	if err != nil {
		respondPartnerHotelError(w, err, "partnered hotel")
		return
	}
	respondJSON(w, http.StatusCreated, staff)
}

func (h *PartnerHotelHandler) UpdateStaff(w http.ResponseWriter, r *http.Request) {
	hotelID, ok := parsePositivePathID(w, r, "hotelID", "hotel")
	if !ok {
		return
	}
	staffID, ok := parsePositivePathID(w, r, "staffID", "staff")
	if !ok {
		return
	}
	var req model.UpdatePartnerHotelStaffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	staff, err := h.service.UpdateStaff(r.Context(), hotelID, staffID, &req)
	if err != nil {
		respondPartnerHotelError(w, err, "hotel staff member")
		return
	}
	respondJSON(w, http.StatusOK, staff)
}

func parsePositivePathID(w http.ResponseWriter, r *http.Request, key, label string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "invalid "+label+" id")
		return 0, false
	}
	return id, true
}

func respondPartnerHotelError(w http.ResponseWriter, err error, resource string) {
	if err == pgx.ErrNoRows {
		respondError(w, http.StatusNotFound, resource+" not found")
		return
	}
	respondServiceError(w, http.StatusBadRequest, err)
}
