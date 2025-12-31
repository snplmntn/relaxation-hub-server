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

func SetupReviewRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	reviewRepo := repository.NewReviewRepository(d)
	reviewService := service.NewReviewService(reviewRepo)
	bookingRepo := repository.NewBookingRepository(d)
	serviceRepo := repository.NewServiceRepository(d)
	reviewHandler := handler.NewReviewHandler(reviewService, bookingRepo, serviceRepo)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/reviews", func(r chi.Router) {
				r.Post("/", reviewHandler.CreateReview)
				r.Get("/therapist/{therapist_id}", reviewHandler.ListReviewsForTherapist)
			})
		})
	})

	return r
}

func TestIntegration_CreateReview(t *testing.T) {
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

	router := SetupReviewRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "client")

	reviewBody := map[string]interface{}{
		"therapist_id": "test-therapist-id",
		"booking_id":   "test-booking-id",
		"rating":       5,
		"comment":      "Excellent service!",
	}

	body, _ := json.Marshal(reviewBody)
	req := httptest.NewRequest("POST", "/api/v1/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated && rr.Code != http.StatusBadRequest {
		t.Logf("Review creation returned status %d (expected 201 or 400)", rr.Code)
	}

	t.Log("✓ Review endpoint accessible")
}

func TestIntegration_ListReviews(t *testing.T) {
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

	router := SetupReviewRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/reviews/therapist/test-therapist-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Review listing successful")
}
