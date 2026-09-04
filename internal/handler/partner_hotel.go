package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type PartnerHotelHandler struct {
	service *service.PartnerHotelService
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
