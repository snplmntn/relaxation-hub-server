package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func SetupBookingRouter(pool *pgxpool.Pool, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	bookingRepo := repository.NewBookingRepository(pool)
	bookingService := service.NewBookingService(bookingRepo)
	bookingHandler := handler.NewBookingHandler(bookingService)

	addressRepo := repository.NewAddressRepository(pool)
	addressService := service.NewAddressService(addressRepo, nil)
	addressHandler := handler.NewAddressHandler(addressService)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/addresses", func(r chi.Router) {
				r.Post("/", addressHandler.CreateAddress)
				r.Get("/", addressHandler.ListAddresses)
				r.Get("/{id}", addressHandler.GetAddress)
				r.Patch("/{id}", addressHandler.UpdateAddress)
				r.Delete("/{id}", addressHandler.DeleteAddress)
				r.Post("/{id}/default", addressHandler.SetDefaultAddress)
			})

			r.Route("/bookings", func(r chi.Router) {
				r.Post("/", bookingHandler.CreateBooking)
				r.Get("/", bookingHandler.ListBookings)
				r.Get("/{id}", bookingHandler.GetBooking)
				r.Patch("/{id}", bookingHandler.UpdateBooking)
				r.Post("/{id}/status", bookingHandler.UpdateBookingStatus)
			})
		})
	})

	return r
}

func TestIntegration_CreateAddress(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	router := SetupBookingRouter(pool, getTestConfig())
	token := createTestUser(t, pool, "user@test.com", "client")

	addressBody := map[string]interface{}{
		"label":          "Home",
		"street_address": "123 Test St",
		"barangay":       "Test Barangay",
		"city":           "Test City",
		"province":       "Test Province",
		"postal_code":    "1234",
		"is_default":     true,
	}

	body, _ := json.Marshal(addressBody)
	req := httptest.NewRequest("POST", "/api/v1/addresses", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Address creation successful")
}

func TestIntegration_ListAddresses(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	router := SetupBookingRouter(pool, getTestConfig())
	token := createTestUser(t, pool, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/addresses", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Address listing successful")
}

func TestIntegration_CreateBooking(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	router := SetupBookingRouter(pool, getTestConfig())
	token := createTestUser(t, pool, "user@test.com", "client")

	serviceID := createTestService(t, pool)
	addressID := createTestAddress(t, pool, token, router)

	bookingBody := map[string]interface{}{
		"service_id":       serviceID,
		"address_id":       addressID,
		"scheduled_at":     time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"special_requests": "Test request",
	}

	body, _ := json.Marshal(bookingBody)
	req := httptest.NewRequest("POST", "/api/v1/bookings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Booking creation successful")
}

func TestIntegration_ListBookings(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	router := SetupBookingRouter(pool, getTestConfig())
	token := createTestUser(t, pool, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/bookings", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Booking listing successful")
}

func createTestAddress(t *testing.T, pool *pgxpool.Pool, token string, router *chi.Mux) string {
	addressBody := map[string]interface{}{
		"label":          "Home",
		"street_address": "123 Test St",
		"barangay":       "Test Barangay",
		"city":           "Test City",
		"province":       "Test Province",
		"postal_code":    "1234",
		"is_default":     true,
	}

	body, _ := json.Marshal(addressBody)
	req := httptest.NewRequest("POST", "/api/v1/addresses", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if addressID, ok := response["address_id"].(string); ok {
		return addressID
	}

	t.Fatal("Failed to create test address")
	return ""
}

func createTestService(t *testing.T, pool *pgxpool.Pool) string {
	var serviceID string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO services (name, description, base_price, duration_minutes, category)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING service_id
	`, "Test Service", "Test Description", 1500.0, 60, "massage").Scan(&serviceID)

	if err != nil {
		t.Fatalf("Failed to create test service: %v", err)
	}

	return serviceID
}
