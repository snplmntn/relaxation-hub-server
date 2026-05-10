package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	testhelpers "github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
)

func setupAdminTherapistCreationRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	userRepo := repository.NewUserRepository(d)
	authService := service.NewAuthService(userRepo, cfg)
	userService := service.NewUserService(userRepo, nil, nil)
	userHandler := handler.NewUserHandler(userService, nil, authService)

	therapistRepo := repository.NewTherapistRepository(d)
	therapistService := service.NewTherapistService(therapistRepo, userRepo)
	therapistHandler := handler.NewTherapistHandler(therapistService, nil)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Get("/therapists", therapistHandler.ListTherapists)

			r.Route("/admin", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				})
				r.Post("/users", userHandler.AdminCreateUser)
			})
		})
	})

	return r
}

func TestAdminCreateTherapist_PersistsProfileAndAppearsInUnfilteredList(t *testing.T) {
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

	router := setupAdminTherapistCreationRouter(tx, getTestConfig())
	adminToken, _, _ := createTestUser(t, tx, "admin_create_therapist_admin@test.com", "admin")
	therapistEmail := uniqueTestEmail("admin_created_therapist@test.com")

	createdUserID := postAdminCreateTherapist(t, router, adminToken, therapistEmail)

	var profileCount int
	if err := tx.QueryRow(context.Background(), `SELECT COUNT(*) FROM therapist_profiles WHERE therapist_id = $1`, createdUserID).Scan(&profileCount); err != nil {
		t.Fatalf("failed to count therapist profile: %v", err)
	}
	if profileCount != 1 {
		t.Fatalf("expected one therapist profile for admin-created therapist, got %d", profileCount)
	}

	req := httptest.NewRequest("GET", "/api/v1/therapists", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected therapist list status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var therapists []model.TherapistProfileResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &therapists); err != nil {
		t.Fatalf("failed to decode therapist list: %v", err)
	}
	for _, therapist := range therapists {
		if therapist.TherapistID == createdUserID {
			return
		}
	}

	t.Fatalf("admin-created therapist %d was not returned by unfiltered therapist list", createdUserID)
}

func TestAdminCreateTherapist_ProfileFailureDoesNotReturnSuccessOrLeaveOrphan(t *testing.T) {
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

	ctx := context.Background()
	triggerName := fmt.Sprintf("test_fail_therapist_profile_%d", time.Now().UnixNano())
	functionName := triggerName + "_fn"
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced therapist profile failure for integration test';
		END;
		$$ LANGUAGE plpgsql;

		CREATE TRIGGER %s
		BEFORE INSERT ON therapist_profiles
		FOR EACH ROW EXECUTE FUNCTION %s();
	`, functionName, triggerName, functionName))
	if err != nil {
		t.Fatalf("failed to install failing therapist profile trigger: %v", err)
	}

	router := setupAdminTherapistCreationRouter(tx, getTestConfig())
	adminToken, _, _ := createTestUser(t, tx, "admin_create_therapist_failure_admin@test.com", "admin")
	therapistEmail := uniqueTestEmail("profile_failure_therapist@test.com")

	body, _ := json.Marshal(map[string]string{
		"provider":     "email",
		"provider_key": therapistEmail,
		"password":     "TestPassword123!",
		"role":         "therapist",
		"full_name":    "Profile Failure Therapist",
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code == http.StatusCreated {
		t.Fatalf("expected profile failure to prevent success, got status %d. Body: %s", rr.Code, rr.Body.String())
	}

	var userID int64
	err = tx.QueryRow(ctx, `SELECT user_id FROM users WHERE primary_email = $1`, strings.ToLower(therapistEmail)).Scan(&userID)
	if err == nil {
		t.Fatalf("expected therapist user rollback after profile failure, found orphan user %d", userID)
	}

	var identityCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM user_auth_identities WHERE provider = 'email' AND provider_key = $1`, strings.ToLower(therapistEmail)).Scan(&identityCount); err != nil {
		t.Fatalf("failed to count auth identities after profile failure: %v", err)
	}
	if identityCount != 0 {
		t.Fatalf("expected auth identity rollback after profile failure, found %d identities", identityCount)
	}
}

func postAdminCreateTherapist(t *testing.T, router http.Handler, adminToken, therapistEmail string) int64 {
	t.Helper()

	body, _ := json.Marshal(map[string]string{
		"provider":     "email",
		"provider_key": therapistEmail,
		"password":     "TestPassword123!",
		"role":         "therapist",
		"full_name":    "Admin Created Therapist",
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode admin create response: %v", err)
	}
	if response.UserID == 0 {
		t.Fatalf("expected created user_id in response, got 0")
	}

	return response.UserID
}
