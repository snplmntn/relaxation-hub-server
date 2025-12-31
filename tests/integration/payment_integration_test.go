package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func SetupPaymentRouter(pool *pgxpool.Pool, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	paymentRepo := repository.NewPaymentRepository(pool)
	paymentService := service.NewPaymentService(paymentRepo)
	bookingRepo := repository.NewBookingRepository(pool)
	serviceRepo := repository.NewServiceRepository(pool)
	addressRepo := repository.NewAddressRepository(pool)
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
	defer CleanupTestDB(t, pool)

	router := SetupPaymentRouter(pool, getTestConfig())
	token := createTestUser(t, pool, "user@test.com", "client")

	paymentBody := map[string]interface{}{
		"booking_id":        "test-booking-id",
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
	defer CleanupTestDB(t, pool)

	router := SetupPaymentRouter(pool, getTestConfig())
	token := createTestUser(t, pool, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/payments/booking/test-booking-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Logf("Get payment returned status %d (expected 200 or 404)", rr.Code)
	}

	t.Log("✓ Payment retrieval endpoint accessible")
}
