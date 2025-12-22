package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func TestCreatePromotion_InvalidBody_ReturnsStructuredValidationError(t *testing.T) {
	h := NewPromotionHandler((*service.PromotionService)(nil))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/promotions", bytes.NewBufferString("not-json"))

	h.CreatePromotion(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err != nil {
		t.Fatalf("failed to decode response: %v; body: %s", err, rr.Body.String())
	}

	if er.Code != "invalid_request_body" {
		t.Errorf("expected Code 'invalid_request_body', got %q", er.Code)
	}
	if er.Message != "invalid request body" {
		t.Errorf("expected Message 'invalid request body', got %q", er.Message)
	}
}
