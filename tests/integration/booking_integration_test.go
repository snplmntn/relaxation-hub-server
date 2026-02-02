package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	testhelpers "github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
)

func SetupBookingRouter(pool *pgxpool.Pool, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	bookingRepo := repository.NewBookingRepository(pool)
	promotionRepo := repository.NewPromotionRepository(pool)
	
	// Initialize other required repositories
	queueRepo := repository.NewAssignmentQueueRepository(pool)
	therapistRepo := repository.NewTherapistRepository(pool)
	offerRepo := repository.NewBookingOfferRepository(pool)
	serviceRepo := repository.NewServiceRepository(pool)
	addressRepo := repository.NewAddressRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	
	extRepo := repository.NewExtensionRequestRepository(pool)
	bookingService := service.NewBookingService(bookingRepo, promotionRepo, pool, queueRepo, therapistRepo, offerRepo, serviceRepo, addressRepo, userRepo, nil, nil, extRepo)
	bookingHandler := handler.NewBookingHandler(bookingService, nil, serviceRepo, addressRepo, therapistRepo, nil)

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
				r.Post("/{id}/accept", bookingHandler.AcceptOffer)
				r.Post("/{id}/decline", bookingHandler.DeclineOffer)
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
	token, _, _ := createTestUser(t, pool, "user@test.com", "client")

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
	token, _, _ := createTestUser(t, pool, "user@test.com", "client")

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
	token, userID, _ := createTestUser(t, pool, "user@test.com", "client")

	serviceID := createTestService(t, pool)
	addressID := createTestAddress(t, pool, int64(userID), token, router)

	bookingBody := map[string]interface{}{
		"service_id":          serviceID,
		"address_id":          addressID,
		"scheduled_start":     time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		"notes":               "Test request",
		"payment_method":     "cash",
		"gender_preference":   "any",
		"pressure_preference": "medium",
		"duration_minutes":    60,
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

func TestIntegration_TherapistAcceptBooking(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	cfg := getTestConfig()
	router := SetupBookingRouter(pool, cfg)

	// Create test users directly in DB and generate tokens
	ctx := context.Background()
	timestamp := time.Now().UnixNano()
	clientID, err := testhelpers.CreateTestUser(ctx, pool, "Client Test", fmt.Sprintf("client_accept_%d@test.com", timestamp), "client")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	therapistID, err := testhelpers.CreateTestUser(ctx, pool, "Therapist Test", fmt.Sprintf("therapist_accept_%d@test.com", timestamp), "therapist")
	if err != nil {
		t.Fatalf("failed to create therapist: %v", err)
	}

	clientToken, err := testhelpers.GenerateTestToken(clientID, "client", cfg.JWTKey, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate client token: %v", err)
	}
	therapistToken, err := testhelpers.GenerateTestToken(therapistID, "therapist", cfg.JWTKey, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate therapist token: %v", err)
	}

	serviceID := createTestService(t, pool)
	addressID := createTestAddress(t, pool, int64(clientID), clientToken, router)

	// Client creates a booking assigned to the therapist
	bookingBody := map[string]interface{}{
		"therapist_id":        therapistID,
		"service_id":          serviceID,
		"address_id":          addressID,
		"scheduled_start":     time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		"notes":               "Please be on time",
		"payment_method":     "cash",
		"gender_preference":   "any",
		"pressure_preference": "medium",
		"duration_minutes":    60,
	}

	body, _ := json.Marshal(bookingBody)
	req := httptest.NewRequest("POST", "/api/v1/bookings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+clientToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}

	bookingData, ok := resp["booking_id"]
	if !ok {
		// Fallback: some handlers embed booking under "booking" key
		if m, ok := resp["booking"].(map[string]interface{}); ok {
			bookingData = m["booking_id"]
		}
	}
	if bookingData == nil {
		t.Fatalf("booking id not returned: %v", resp)
	}

	// booking_id may be float64 when decoded from json
	var bookingID int64
	switch v := bookingData.(type) {
	case float64:
		bookingID = int64(v)
	case string:
		// try parse
		var id64 int64
		_ = json.Unmarshal([]byte("\""+v+"\""), &id64)
		bookingID = id64
	default:
		t.Fatalf("unexpected booking id type: %T", v)
	}

	// The AssignmentWorker doesn't run during tests, so manually insert a pending offer
	_, err = pool.Exec(ctx, `
		INSERT INTO booking_offers (booking_id, therapist_id, status, expires_at)
		VALUES ($1, $2, 'pending', $3)
	`, bookingID, therapistID, time.Now().Add(10*time.Minute))
	if err != nil {
		t.Fatalf("failed to insert test offer: %v", err)
	}

	// Therapist accepts (confirms) the booking via the /accept endpoint
	req = httptest.NewRequest("POST", "/api/v1/bookings/"+fmt.Sprintf("%d", bookingID)+"/accept", nil)
	req.Header.Set("Authorization", "Bearer "+therapistToken)

	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for accept, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var statusResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("invalid status response json: %v", err)
	}
	if s, ok := statusResp["status"].(string); !ok || s != "accepted" {
		t.Fatalf("expected booking status 'accepted', got: %v", statusResp)
	}

	t.Log("✓ Therapist accept booking successful")
}

func TestIntegration_ListBookings(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	router := SetupBookingRouter(pool, getTestConfig())
	token, _, _ := createTestUser(t, pool, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/bookings", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Booking listing successful")
}

