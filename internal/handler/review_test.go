package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func TestCreateReview_InvalidBody_ReturnsStructuredError(t *testing.T) {
	h := NewReviewHandler((*service.ReviewService)(nil), (*service.ClientReviewService)(nil), (repository.BookingRepository)(nil), (repository.ServiceRepository)(nil), (repository.UserRepository)(nil))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/reviews", bytes.NewBufferString("not-json"))

	h.CreateReview(rr, req)

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
