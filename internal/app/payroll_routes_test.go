package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestRegisterRoutes_PayrollRoutesAreSuperAdminOnly(t *testing.T) {
	router, jwtKey := testRouterForRouteGuards(t)

	req := httptest.NewRequest("GET", "/api/v1/payroll/rates", nil)
	req.Header.Set("Authorization", authHeader(t, 1, model.RoleAdmin, jwtKey))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected regular admin to be rejected from payroll routes, got %d", rr.Code)
	}
}
