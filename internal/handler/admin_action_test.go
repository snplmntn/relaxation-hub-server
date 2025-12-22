package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestLogAction_InvalidBody_ReturnsStructuredError(t *testing.T) {
	h := NewAdminActionHandler(nil)

	wrapped := middleware.AuthMiddleware(http.HandlerFunc(h.LogAction), "tests-secret")

	claims := &model.Claims{UserID: 1, Role: "admin"}
	tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("tests-secret"))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/actions", bytes.NewBufferString("not-json"))
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err != nil {
		t.Fatalf("failed to decode response: %v; body: %s", err, rr.Body.String())
	}

	if er.Error != "Bad Request" {
		t.Errorf("expected Error 'Bad Request', got %q", er.Error)
	}
	if er.Message != "invalid request body" {
		t.Errorf("expected Message 'invalid request body', got %q", er.Message)
	}
}
