package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// maxWebhookBody caps what we will read and hash from an unauthenticated
// endpoint.
const maxWebhookBody = 1 << 20

type BookingCheckoutHandler struct {
	checkoutService *service.BookingCheckoutService
	paymongo        *service.PayMongoClient
}

func NewBookingCheckoutHandler(checkoutService *service.BookingCheckoutService, paymongo *service.PayMongoClient) *BookingCheckoutHandler {
	return &BookingCheckoutHandler{checkoutService: checkoutService, paymongo: paymongo}
}

// StartCheckout prices a not-yet-created booking and returns the PayMongo URL
// to send the customer to.
func (h *BookingCheckoutHandler) StartCheckout(w http.ResponseWriter, r *http.Request) {
	if h.checkoutService == nil {
		respondError(w, http.StatusServiceUnavailable, "online payment is not available")
		return
	}
	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.StartCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := h.checkoutService.Start(r.Context(), clientID, &req)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondServiceError(w, http.StatusBadGateway, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

// GetCheckoutStatus is polled by the return page until the webhook lands.
func (h *BookingCheckoutHandler) GetCheckoutStatus(w http.ResponseWriter, r *http.Request) {
	if h.checkoutService == nil {
		respondError(w, http.StatusServiceUnavailable, "online payment is not available")
		return
	}
	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	reference := chi.URLParam(r, "reference")
	if reference == "" {
		respondError(w, http.StatusBadRequest, "invalid checkout reference")
		return
	}

	res, err := h.checkoutService.Status(r.Context(), clientID, reference)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "checkout not found")
			return
		}
		if ve, ok := err.(*service.ValidationError); ok && ve.Code == "forbidden" {
			respondError(w, http.StatusForbidden, ve.Message)
			return
		}
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// HandlePayMongoWebhook is the authoritative signal that a booking was paid
// for. It is public, so the signature check is the only thing standing between
// a stranger and a free booking — it runs before the body is interpreted, and
// against the exact bytes received.
func (h *BookingCheckoutHandler) HandlePayMongoWebhook(w http.ResponseWriter, r *http.Request) {
	if h.checkoutService == nil || h.paymongo == nil {
		respondError(w, http.StatusServiceUnavailable, "online payment is not available")
		return
	}

	rawBody, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		respondError(w, http.StatusBadRequest, "could not read request body")
		return
	}

	if err := h.paymongo.VerifyWebhookSignature(r.Header.Get("Paymongo-Signature"), rawBody, time.Now()); err != nil {
		slog.Warn("[PayMongo] rejected webhook", "error", err, "remote", r.RemoteAddr)
		respondError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	var event struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Type string `json:"type"`
				Data struct {
					ID         string `json:"id"`
					Attributes struct {
						// checkout_session.payment.paid carries the session
						// directly; payment.paid nests it under the payment.
						CheckoutSessionID string `json:"checkout_session_id"`
					} `json:"attributes"`
				} `json:"data"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawBody, &event); err != nil {
		respondError(w, http.StatusBadRequest, "invalid webhook payload")
		return
	}

	eventID := event.Data.ID
	eventType := event.Data.Attributes.Type
	sessionID := event.Data.Attributes.Data.Attributes.CheckoutSessionID
	if sessionID == "" {
		// checkout_session.* events identify the session as the resource itself.
		sessionID = event.Data.Attributes.Data.ID
	}

	switch eventType {
	case "checkout_session.payment.paid", "payment.paid":
		if err := h.checkoutService.Fulfil(r.Context(), sessionID, eventID); err != nil {
			slog.Error("[PayMongo] fulfilment failed", "event_id", eventID, "session_id", sessionID, "error", err)
			// 500 asks PayMongo to retry — better than losing a paid booking.
			respondError(w, http.StatusInternalServerError, "could not fulfil checkout")
			return
		}
	case "payment.failed":
		if err := h.checkoutService.MarkFailed(r.Context(), sessionID); err != nil {
			slog.Error("[PayMongo] could not mark failed", "session_id", sessionID, "error", err)
		}
	default:
		slog.Info("[PayMongo] ignoring event", "type", eventType, "event_id", eventID)
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"received":true}`))
}
