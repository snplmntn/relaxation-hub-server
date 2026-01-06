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

func SetupServiceRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	serviceRepo := repository.NewServiceRepository(d)
	serviceCatalog := service.NewServiceCatalog(serviceRepo, nil)
	serviceHandler := handler.NewServiceHandler(serviceCatalog, nil)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/services", serviceHandler.ListServices)

		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})
			r.Use(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			})
			r.Post("/services", serviceHandler.CreateService)
		})
	})

	return r
}

func TestIntegration_ListServices(t *testing.T) {
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

	router := SetupServiceRouter(tx, getTestConfig())

	req := httptest.NewRequest("GET", "/api/v1/services", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["services"] == nil {
		t.Fatal("Expected services array in response")
	}

	t.Log("✓ Service listing successful")
}

func TestIntegration_CreateService_AdminOnly(t *testing.T) {
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

	router := SetupServiceRouter(tx, getTestConfig())

	adminToken := createTestUser(t, tx, "admin@test.com", "admin")

	serviceBody := map[string]interface{}{
		"name":             "Test Massage",
		"description":      "Test description",
		"base_price":       1500.0,
		"duration_minutes": 60,
		"category":         "massage",
	}

	body, _ := json.Marshal(serviceBody)
	req := httptest.NewRequest("POST", "/api/v1/services", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Service creation successful")
}

func TestIntegration_CreateService_ClientForbidden(t *testing.T) {
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

	router := SetupServiceRouter(tx, getTestConfig())

	clientToken := createTestUser(t, tx, "client@test.com", "client")

	serviceBody := map[string]interface{}{
		"name":             "Test Massage",
		"description":      "Test description",
		"base_price":       1500.0,
		"duration_minutes": 60,
		"category":         "massage",
	}

	body, _ := json.Marshal(serviceBody)
	req := httptest.NewRequest("POST", "/api/v1/services", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+clientToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}

	t.Log("✓ Non-admin correctly forbidden from creating services")
}
