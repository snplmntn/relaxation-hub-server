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

func SetupTherapistRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	therapistRepo := repository.NewTherapistRepository(d)
	therapistService := service.NewTherapistService(therapistRepo, nil) // nil userRepo for tests
	therapistHandler := handler.NewTherapistHandler(therapistService, nil)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/therapists", func(r chi.Router) {
				r.Get("/", therapistHandler.ListTherapists)
				r.Get("/{id}", therapistHandler.GetProfile)
				r.Get("/{id}/services", therapistHandler.GetServices)
				r.Get("/{id}/documents", therapistHandler.GetDocuments)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Patch("/profile", therapistHandler.UpdateProfile)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/documents", therapistHandler.UploadDocument)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/services", therapistHandler.AddService)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Delete("/services/{service_id}", therapistHandler.RemoveService)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Post("/documents/{document_id}/verify", therapistHandler.VerifyDocument)
			})
		})
	})

	return r
}

func TestIntegration_ListTherapists(t *testing.T) {
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

	router := SetupTherapistRouter(tx, getTestConfig())
	token, _, _ := createTestUser(t, tx, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/therapists", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Therapist listing successful")
}

func TestIntegration_UpdateTherapistProfile(t *testing.T) {
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

	router := SetupTherapistRouter(tx, getTestConfig())
	therapistToken, therapistID, _ := createTestUser(t, tx, "therapist@test.com", "therapist")

	// Create profile row to satisfy FK or logic
	_, err = tx.Exec(context.Background(), "INSERT INTO therapist_profiles (therapist_id, bio, years_experience, accept_assignments) VALUES ($1, '', 0, true) ON CONFLICT DO NOTHING", therapistID)
	if err != nil {
		t.Fatalf("Failed to create initial therapist profile: %v", err)
	}

	profileBody := map[string]interface{}{
		"bio":              "Experienced therapist",
		"years_experience": 5,
		"certifications":   []string{"Cert1", "Cert2"},
	}

	body, _ := json.Marshal(profileBody)
	req := httptest.NewRequest("PATCH", "/api/v1/therapists/profile", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+therapistToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Therapist profile update successful")
}

func TestIntegration_UploadDocument_TherapistOnly(t *testing.T) {
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

	router := SetupTherapistRouter(tx, getTestConfig())
	therapistToken, therapistID, _ := createTestUser(t, tx, "therapist@test.com", "therapist")

	// Same here: need a profile row to satisfy FK or logic
	_, err = tx.Exec(context.Background(), "INSERT INTO therapist_profiles (therapist_id, bio, years_experience, accept_assignments) VALUES ($1, '', 0, true) ON CONFLICT DO NOTHING", therapistID)
	if err != nil {
		t.Fatalf("Failed to create initial therapist profile: %v", err)
	}

	documentBody := map[string]interface{}{
		"document_type": "Certification",
		"document_url":  "https://example.com/cert.pdf",
		"description":   "Test certification",
	}

	body, _ := json.Marshal(documentBody)
	req := httptest.NewRequest("POST", "/api/v1/therapists/documents", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+therapistToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Document upload successful")
}
