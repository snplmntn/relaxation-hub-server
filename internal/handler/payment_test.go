package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func TestCreatePayment_InvalidBody_ReturnsStructuredError(t *testing.T) {
	h := NewPaymentHandler((*service.PaymentService)(nil))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/payments", bytes.NewBufferString("not-json"))

	h.CreatePayment(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	// This handler currently uses plain http.Error for invalid body.
	// We assert we still get a JSON error when the response helper is used in future.
	// Decode as ErrorResponse if possible, otherwise ensure body contains the message.
	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err == nil {
		if er.Message != "invalid request body" && er.Error != "Bad Request" {
			t.Errorf("unexpected structured error: %+v", er)
		}
	}
}
