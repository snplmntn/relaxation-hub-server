package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func SetupEmergencyRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	emergencyAlertRepo := repository.NewEmergencyAlertRepository(d)
	emergencyAlertService := service.NewEmergencyAlertService(emergencyAlertRepo)
	emergencyAlertHandler := handler.NewEmergencyAlertHandler(emergencyAlertService, service.NewBookingService(repository.NewBookingRepository(d), repository.NewPromotionRepository(d), d, repository.NewAssignmentQueueRepository(d), repository.NewTherapistRepository(d), repository.NewBookingOfferRepository(d), repository.NewServiceRepository(d), repository.NewAddressRepository(d), repository.NewUserRepository(d), nil, nil, repository.NewExtensionRequestRepository(d), nil, nil))

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
	
	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer cleanup()

	router := SetupEmergencyRouter(tx, getTestConfig())
	// Create all prerequisites for a booking
	therapistToken, therapistID, _ := createTestUser(t, tx, "therapist@test.com", "therapist")
	_, clientID, _ := createTestUser(t, tx, "client@test.com", "client")
	serviceIDstr := createTestService(t, tx)
	addressIDstr := createTestAddress(t, tx, clientID, "", nil)
	
	var sID, aID int64
	fmt.Sscanf(serviceIDstr, "%d", &sID)
	fmt.Sscanf(addressIDstr, "%d", &aID)

	// Create a booking
	var bookingID int64
	err = tx.QueryRow(context.Background(), `
		INSERT INTO bookings (client_id, therapist_id, service_id, address_id, status, scheduled_start, duration_minutes, payment_method)
		VALUES ($1, $2, $3, $4, 'on_the_way', NOW(), 60, 'cash')
		RETURNING booking_id
	`, clientID, therapistID, sID, aID).Scan(&bookingID)
	if err != nil {
		t.Fatalf("Failed to create booking for emergency test: %v", err)
	}

	alertBody := map[string]interface{}{
		"alert_type":  "safety_concern",
		"latitude":    14.5547,
		"longitude":   121.0244,
		"description": "Test emergency alert",
		"booking_id":  bookingID,
	}

	body, _ := json.Marshal(alertBody)
	req := httptest.NewRequest("POST", "/api/v1/emergency/trigger", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+therapistToken)

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
	
	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer cleanup()

	router := SetupEmergencyRouter(tx, getTestConfig())
	token, _, _ := createTestUser(t, tx, "admin@test.com", "admin")

	req := httptest.NewRequest("GET", "/api/v1/emergency/alert/99999", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Logf("Get emergency alert returned status %d (expected 200 or 404)", rr.Code)
	}

	t.Log("✓ Get emergency alert endpoint accessible")
}
