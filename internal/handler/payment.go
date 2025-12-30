package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type PaymentHandler struct {
	paymentService *service.PaymentService
	bookingRepo    repository.BookingRepository
	serviceRepo    repository.ServiceRepository
	addressRepo    repository.AddressRepository
}

func NewPaymentHandler(paymentService *service.PaymentService, bookingRepo repository.BookingRepository, serviceRepo repository.ServiceRepository, addressRepo repository.AddressRepository) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService, bookingRepo: bookingRepo, serviceRepo: serviceRepo, addressRepo: addressRepo}
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

	// Enrich with booking details when possible
	var bookingResp *model.BookingResponse
	if b, berr := h.bookingRepo.GetByBookingID(r.Context(), p.BookingID); berr == nil {
		var svc *model.Service
		var addr *model.Address
		if b.ServiceID != nil && h.serviceRepo != nil {
			if s, serr := h.serviceRepo.GetByID(r.Context(), *b.ServiceID); serr == nil {
				svc = s
			}
		}
		if b.AddressID != nil && h.addressRepo != nil {
			if a, aerr := h.addressRepo.GetByIDUnsafe(r.Context(), *b.AddressID); aerr == nil {
				addr = a
			}
		}
		br := toBookingResponse(b, svc, addr, "", nil, "", "", "")
		bookingResp = &br
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toPaymentResponse(p, bookingResp))
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

	var bookingResp *model.BookingResponse
	if b, berr := h.bookingRepo.GetByBookingID(r.Context(), p.BookingID); berr == nil {
		var svc *model.Service
		var addr *model.Address
		if b.ServiceID != nil && h.serviceRepo != nil {
			if s, serr := h.serviceRepo.GetByID(r.Context(), *b.ServiceID); serr == nil {
				svc = s
			}
		}
		if b.AddressID != nil && h.addressRepo != nil {
			if a, aerr := h.addressRepo.GetByIDUnsafe(r.Context(), *b.AddressID); aerr == nil {
				addr = a
			}
		}
		br := toBookingResponse(b, svc, addr, "", nil, "", "", "")
		bookingResp = &br
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toPaymentResponse(p, bookingResp))
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

	var bookingResp *model.BookingResponse
	if b, berr := h.bookingRepo.GetByBookingID(r.Context(), p.BookingID); berr == nil {
		var svc *model.Service
		var addr *model.Address
		if b.ServiceID != nil && h.serviceRepo != nil {
			if s, serr := h.serviceRepo.GetByID(r.Context(), *b.ServiceID); serr == nil {
				svc = s
			}
		}
		if b.AddressID != nil && h.addressRepo != nil {
			if a, aerr := h.addressRepo.GetByIDUnsafe(r.Context(), *b.AddressID); aerr == nil {
				addr = a
			}
		}
		br := toBookingResponse(b, svc, addr, "", nil, "", "", "")
		bookingResp = &br
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toPaymentResponse(p, bookingResp))
}
func toPaymentResponse(p *model.Payment, booking *model.BookingResponse) model.PaymentResponse {
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
		Booking:       booking,
	}
}
