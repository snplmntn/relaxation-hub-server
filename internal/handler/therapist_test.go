package handler

import (
	"bytes"
	"encoding/json"
	"reflect"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestUpdateProfile_InvalidBody_ReturnsStructuredError(t *testing.T) {
	h := NewTherapistHandler(nil, nil)

	wrapped := middleware.AuthMiddleware(http.HandlerFunc(h.UpdateProfile), "tests-secret")

	claims := &model.Claims{UserID: 2, Role: "therapist"}
	tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("tests-secret"))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/therapists/me", bytes.NewBufferString("not-json"))
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

func TestDecodeAdminUpdateServicesRequest_ArrayPayload(t *testing.T) {
	input := `[{"service_id":1,"pressures":["soft","medium"]}]`
	got, err := decodeAdminUpdateServicesRequest(bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	want := []model.AddServiceWithPressuresRequest{
		{ServiceID: 1, Pressures: []string{"soft", "medium"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected decode result: got=%v want=%v", got, want)
	}
}

func TestDecodeAdminUpdateServicesRequest_WrappedPayload(t *testing.T) {
	input := `{"services":[{"service_id":2,"pressures":["hard"]}]}`
	got, err := decodeAdminUpdateServicesRequest(bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	want := []model.AddServiceWithPressuresRequest{
		{ServiceID: 2, Pressures: []string{"hard"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected decode result: got=%v want=%v", got, want)
	}
}

func TestDecodeAdminUpdateServicesRequest_WrappedMissingServices(t *testing.T) {
	input := `{}`
	_, err := decodeAdminUpdateServicesRequest(bytes.NewBufferString(input))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
