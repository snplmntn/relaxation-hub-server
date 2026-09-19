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

type BookingAnnouncementHandler struct {
	service *service.BookingAnnouncementService
}

func NewBookingAnnouncementHandler(announcementService *service.BookingAnnouncementService) *BookingAnnouncementHandler {
	return &BookingAnnouncementHandler{service: announcementService}
}

func (h *BookingAnnouncementHandler) List(w http.ResponseWriter, r *http.Request) {
	announcements, err := h.service.List(r.Context())
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusOK, announcements)
}

func (h *BookingAnnouncementHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.BookingAnnouncementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	announcement, err := h.service.Create(r.Context(), req)
	if !handleBookingAnnouncementError(w, err) {
		return
	}
	respondJSON(w, http.StatusCreated, announcement)
}

func (h *BookingAnnouncementHandler) Update(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := bookingAnnouncementID(w, r, "id")
	if !ok {
		return
	}
	var req model.BookingAnnouncementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	announcement, err := h.service.Update(r.Context(), announcementID, req)
	if !handleBookingAnnouncementError(w, err) {
		return
	}
	respondJSON(w, http.StatusOK, announcement)
}

func (h *BookingAnnouncementHandler) Delete(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := bookingAnnouncementID(w, r, "id")
	if !ok {
		return
	}
	if !handleBookingAnnouncementError(w, h.service.Delete(r.Context(), announcementID)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *BookingAnnouncementHandler) ChooseWinner(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := bookingAnnouncementID(w, r, "id")
	if !ok {
		return
	}
	var req model.BookingAnnouncementWinnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	announcement, err := h.service.ChooseWinner(r.Context(), announcementID, req.VariationID)
	if !handleBookingAnnouncementError(w, err) {
		return
	}
	respondJSON(w, http.StatusOK, announcement)
}

func (h *BookingAnnouncementHandler) ResolveForBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, ok := bookingAnnouncementID(w, r, "bookingID")
	if !ok {
		return
	}
	announcements, err := h.service.ResolveForBooking(r.Context(), bookingID)
	if !handleBookingAnnouncementError(w, err) {
		return
	}
	respondJSON(w, http.StatusOK, announcements)
}

func bookingAnnouncementID(w http.ResponseWriter, r *http.Request, param string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, param), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func handleBookingAnnouncementError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	if validation, ok := err.(*service.ValidationError); ok {
		respondValidation(w, http.StatusBadRequest, validation.Code, validation.Message, validation.Details)
		return false
	}
	if err == pgx.ErrNoRows {
		respondError(w, http.StatusNotFound, "booking announcement not found")
		return false
	}
	respondServiceError(w, http.StatusInternalServerError, err)
	return false
}
