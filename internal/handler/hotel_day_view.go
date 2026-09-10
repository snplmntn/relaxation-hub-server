package handler

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"net/http"
	"strconv"
)

type HotelDayViewHandler struct {
	service  *service.HotelDayViewService
	bookings *service.HotelBookingService
}

func (h *HotelDayViewHandler) SetBookings(s *service.HotelBookingService) { h.bookings = s }
func (h *HotelDayViewHandler) ListBookingOptions(w http.ResponseWriter, r *http.Request) {
	result, err := h.bookings.ListOptions(r.Context())
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusOK, result)
}
func (h *HotelDayViewHandler) ListBookings(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, 401, "Sign in required")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	result, err := h.bookings.List(r.Context(), id, page)
	if err != nil {
		respondHotelAccessError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, 200, result)
}
func (h *HotelDayViewHandler) Analytics(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Sign in required")
		return
	}
	days := 30
	if value := r.URL.Query().Get("days"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Analytics range must be 7, 30, 90, or 365 days")
			return
		}
		days = parsed
	}
	if days != 7 && days != 30 && days != 90 && days != 365 {
		respondError(w, http.StatusBadRequest, "Analytics range must be 7, 30, 90, or 365 days")
		return
	}
	result, err := h.bookings.Analytics(r.Context(), id, days)
	if err != nil {
		if errors.Is(err, service.ErrHotelAccessDenied) {
			respondHotelAccessError(w, err)
			return
		}
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, result)
}
func (h *HotelDayViewHandler) UpdateBooking(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, 401, "Sign in required")
		return
	}
	bookingID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || bookingID <= 0 {
		respondError(w, 400, "Invalid booking ID")
		return
	}
	var req model.UpdateHotelBookingRequest
	if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		respondError(w, 400, "Invalid booking details")
		return
	}
	h.bookingResult(w, h.bookings.Update(r.Context(), id, bookingID, req))
}
func (h *HotelDayViewHandler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, 401, "Sign in required")
		return
	}
	bookingID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || bookingID <= 0 {
		respondError(w, 400, "Invalid booking ID")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		respondError(w, 400, "Invalid cancellation reason")
		return
	}
	h.bookingResult(w, h.bookings.Cancel(r.Context(), id, bookingID, req.Reason))
}
func (h *HotelDayViewHandler) bookingResult(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(w, 404, "Booking not found")
		return
	}
	if errors.Is(err, service.ErrHotelAccessDenied) {
		respondHotelAccessError(w, err)
		return
	}
	if err != nil {
		respondServiceError(w, 400, err)
		return
	}
	respondJSON(w, 200, map[string]bool{"success": true})
}

func NewHotelDayViewHandler(s *service.HotelDayViewService) *HotelDayViewHandler {
	return &HotelDayViewHandler{service: s}
}

func (h *HotelDayViewHandler) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Sign in required")
		return
	}
	view, err := h.service.Get(r.Context(), id, r.URL.Query().Get("date"))
	if errors.Is(err, service.ErrHotelDayViewDate) {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		respondHotelAccessError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, view)
}
