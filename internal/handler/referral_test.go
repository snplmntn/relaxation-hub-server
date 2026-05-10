package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateReferral_InvalidBody_ReturnsStructuredError(t *testing.T) {
	h := NewReferralHandler(nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/referrals", bytes.NewBufferString("not-json"))

	h.CreateReferral(rr, req)

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
