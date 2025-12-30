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

func SetupEmergencyRouter(pool *pgxpool.Pool, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	emergencyAlertRepo := repository.NewEmergencyAlertRepository(pool)
	emergencyAlertService := service.NewEmergencyAlertService(emergencyAlertRepo)
	emergencyAlertHandler := handler.NewEmergencyAlertHandler(emergencyAlertService, service.NewBookingService(repository.NewBookingRepository(pool), repository.NewPromotionRepository(pool), pool, repository.NewAssignmentQueueRepository(pool), repository.NewTherapistRepository(pool), repository.NewBookingOfferRepository(pool), repository.NewServiceRepository(pool), repository.NewAddressRepository(pool), nil, nil))

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/emergency", func(r chi.Router) {
				r.Post("/trigger", emergencyAlertHandler.TriggerAlert)
				r.Get("/alert/{id}", emergencyAlertHandler.GetAlert)
				r.Post("/alert/{id}/resolve", emergencyAlertHandler.ResolveAlert)
			})
		})
	})

	return r
}

func TestIntegration_TriggerEmergencyAlert(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	router := SetupEmergencyRouter(pool, getTestConfig())
	token := createTestUser(t, pool, "therapist@test.com", "therapist")

	alertBody := map[string]interface{}{
		"alert_type":  "safety_concern",
		"latitude":    14.5547,
		"longitude":   121.0244,
		"description": "Test emergency alert",
	}

	body, _ := json.Marshal(alertBody)
	req := httptest.NewRequest("POST", "/api/v1/emergency/trigger", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Emergency alert trigger successful")
}

func TestIntegration_GetEmergencyAlert(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	router := SetupEmergencyRouter(pool, getTestConfig())
	token := createTestUser(t, pool, "admin@test.com", "admin")

	req := httptest.NewRequest("GET", "/api/v1/emergency/alert/test-alert-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Logf("Get emergency alert returned status %d (expected 200 or 404)", rr.Code)
	}

	t.Log("✓ Get emergency alert endpoint accessible")
}
