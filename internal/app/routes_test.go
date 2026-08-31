package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/auth"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func testRouterForRouteGuards(t *testing.T) (http.Handler, string) {
	t.Helper()
	jwtKey := "test-secret-key-32-characters-long"
	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	registerRoutes(r, &dependencies{
		cfg:               &config.Config{JWTKey: jwtKey},
		accountingHandler: handler.NewAccountingHandler(service.NewAccountingService(nil)),
		reportHandler:     handler.NewReportHandler(nil, nil, nil, nil),
	})
	return r, jwtKey
}

func authHeader(t *testing.T, userID int, role, jwtKey string) string {
	t.Helper()
	token, err := auth.GenerateToken(userID, role, jwtKey)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return "Bearer " + token
}

func TestRegisterRoutes_RemovesLegacyAdminShimBlock(t *testing.T) {
	router, _ := testRouterForRouteGuards(t)

	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected legacy /api/v1/admin block to be disabled with 404, got %d", rr.Code)
	}
}

func TestRegisterRoutes_StaffRoutesAreSuperAdminOnly(t *testing.T) {
	router, jwtKey := testRouterForRouteGuards(t)

	req := httptest.NewRequest("POST", "/api/v1/staff", nil)
	req.Header.Set("Authorization", authHeader(t, 1, model.RoleAdmin, jwtKey))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected regular admin to be rejected from /api/v1/staff, got %d", rr.Code)
	}
}

func TestRegisterRoutes_ReportsAreSuperAdminOnly(t *testing.T) {
	router, jwtKey := testRouterForRouteGuards(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/accounting/summary", nil)
	req.Header.Set("Authorization", authHeader(t, 1, model.RoleAdmin, jwtKey))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected regular admin to be rejected from reports, got %d", rr.Code)
	}
}

func TestRegisterRoutes_AccountingSheetAllowsOperationsAdmins(t *testing.T) {
	router, jwtKey := testRouterForRouteGuards(t)

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/accounting/expenses"},
		{method: http.MethodGet, path: "/api/v1/accounting/tips"},
		{method: http.MethodGet, path: "/api/v1/reports/daily-sales"},
		{method: http.MethodPut, path: "/api/v1/reports/daily-sales/remittances"},
	} {
		req := httptest.NewRequest(test.method, test.path, nil)
		req.Header.Set("Authorization", authHeader(t, 1, model.RoleAdmin, jwtKey))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code == http.StatusForbidden {
			t.Fatalf("expected operations admin to pass the route guard for %s, got %d", test.path, rr.Code)
		}
	}
}

func TestRegisterRoutes_BookingEventsHaveCanonicalOperationalRoute(t *testing.T) {
	router, jwtKey := testRouterForRouteGuards(t)

	req := httptest.NewRequest("GET", "/api/v1/booking-events", nil)
	req.Header.Set("Authorization", authHeader(t, 1, model.RoleClient, jwtKey))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected canonical booking events route to reject non-admins with 403, got %d", rr.Code)
	}
}
