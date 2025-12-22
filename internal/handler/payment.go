package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type PaymentHandler struct {
	paymentService *service.PaymentService
}

func NewPaymentHandler(paymentService *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService}
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req model.CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p, err := h.paymentService.Create(r.Context(), &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toPaymentResponse(p))
}

func (h *PaymentHandler) GetPaymentByBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseIDFromPath(r, "booking_id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	p, err := h.paymentService.GetByBookingID(r.Context(), bookingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "payment not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toPaymentResponse(p))
}

func (h *PaymentHandler) UpdatePaymentStatus(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseIDFromPath(r, "booking_id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req model.UpdatePaymentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p, err := h.paymentService.UpdateStatus(r.Context(), bookingID, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "payment not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toPaymentResponse(p))
}

func toPaymentResponse(p *model.Payment) model.PaymentResponse {
	return model.PaymentResponse{
		PaymentID:     p.PaymentID,
		BookingID:     p.BookingID,
		Amount:        p.Amount,
		Gateway:       p.Gateway,
		TransactionID: p.TransactionID,
		Status:        p.Status,
		WebhookID:     p.WebhookID,
		TransactionAt: p.TransactionAt,
		PaidAt:        p.PaidAt,
		RefundedAt:    p.RefundedAt,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}
