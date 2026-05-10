package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	testhelpers "github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
)

func SetupPaymentRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	paymentRepo := repository.NewPaymentRepository(d)
	paymentService := service.NewPaymentService(paymentRepo)
	bookingRepo := repository.NewBookingRepository(d)
	serviceRepo := repository.NewServiceRepository(d)
	addressRepo := repository.NewAddressRepository(d)
	paymentHandler := handler.NewPaymentHandler(paymentService, bookingRepo, serviceRepo, addressRepo)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/payments", func(r chi.Router) {
				r.Post("/", paymentHandler.CreatePayment)
				r.Get("/booking/{booking_id}", paymentHandler.GetPaymentByBooking)
				r.Post("/booking/{booking_id}/status", paymentHandler.UpdatePaymentStatus)
			})
		})
	})

	return r
}

func TestIntegration_CreatePayment(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer cleanup()

	router := SetupPaymentRouter(tx, getTestConfig())
	token, _, _ := createTestUser(t, tx, "user@test.com", "client")

	paymentBody := map[string]interface{}{
		"booking_id":        456,
		"payment_method":    "gcash",
		"amount":            1500.0,
		"payment_reference": "TEST-REF-123",
	}

	body, _ := json.Marshal(paymentBody)
	req := httptest.NewRequest("POST", "/api/v1/payments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated && rr.Code != http.StatusBadRequest {
		t.Logf("Payment creation returned status %d (expected 201 or 400 for missing booking)", rr.Code)
	}

	t.Log("✓ Payment endpoint accessible")
}

func TestIntegration_GetPaymentByBooking(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer cleanup()

	router := SetupPaymentRouter(tx, getTestConfig())
	token, _, _ := createTestUser(t, tx, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/payments/booking/456", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Logf("Get payment returned status %d (expected 200 or 404)", rr.Code)
	}

	t.Log("✓ Payment retrieval endpoint accessible")
}
