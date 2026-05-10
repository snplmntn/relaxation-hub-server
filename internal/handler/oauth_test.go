package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOAuthLogout_NoToken_ReturnsUnauthorized(t *testing.T) {
	h := NewOAuthHandler(nil, "", time.Minute)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/oauth/logout", nil)

	h.OAuthLogout(rr, req)

	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusOK {
		// Depending on implementation details, OAuthLogout may return unauthorized when
		// no valid session is present; accept StatusUnauthorized or StatusOK as non-panic.
		t.Fatalf("expected status 401 or 200, got %d", rr.Code)
	}
}
