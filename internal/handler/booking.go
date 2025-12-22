package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type BookingHandler struct {
	bookingService *service.BookingService
}

func NewBookingHandler(bookingService *service.BookingService) *BookingHandler {
	return &BookingHandler{bookingService: bookingService}
}

func (h *BookingHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var req model.CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	booking, err := h.bookingService.Create(r.Context(), clientID, &req)
	if err != nil {
		// Handle structured validation errors from service
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toBookingResponse(booking))
}

func (h *BookingHandler) ListBookings(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	bookings, err := h.bookingService.ListByClient(r.Context(), clientID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]model.BookingResponse, 0, len(bookings))
	for i := range bookings {
		out = append(out, toBookingResponse(&bookings[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *BookingHandler) GetBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	booking, err := h.bookingService.GetByID(r.Context(), bookingID, clientID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBookingResponse(booking))
}

func (h *BookingHandler) UpdateBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req model.UpdateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	booking, err := h.bookingService.Update(r.Context(), bookingID, clientID, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
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
	json.NewEncoder(w).Encode(toBookingResponse(booking))
}

func (h *BookingHandler) UpdateBookingStatus(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req model.UpdateBookingStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	booking, err := h.bookingService.UpdateStatus(r.Context(), bookingID, actorID, role, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBookingResponse(booking))
}

func parseBookingID(r *http.Request) (int64, error) {
	idParam := chi.URLParam(r, "id")
	return strconv.ParseInt(idParam, 10, 64)
}

func toBookingResponse(b *model.Booking) model.BookingResponse {
	return model.BookingResponse{
		BookingID:       b.BookingID,
		ClientID:        b.ClientID,
		TherapistID:     b.TherapistID,
		ServiceID:       b.ServiceID,
		AddressID:       b.AddressID,
		PromoID:         b.PromoID,
		GenderPref:      b.GenderPref,
		PressurePref:    b.PressurePref,
		Notes:           b.Notes,
		DurationMinutes: b.DurationMinutes,
		ScheduledStart:  b.ScheduledStart,
		ActualStart:     b.ActualStart,
		ActualEnd:       b.ActualEnd,
		RawTotal:        b.RawTotal,
		Discount:        b.Discount,
		FinalTotal:      b.FinalTotal,
		Status:          b.Status,
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
	}
}
