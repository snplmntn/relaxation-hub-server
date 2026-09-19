package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type fakeGoogleAuthService struct {
	result *service.GoogleAuthResult
	err    error
}

func (f *fakeGoogleAuthService) Authenticate(context.Context, string) (*service.GoogleAuthResult, error) {
	return f.result, f.err
}

func (f *fakeGoogleAuthService) Link(context.Context, int, string) error {
	return f.err
}

func TestGoogleAuthenticateReturnsToken(t *testing.T) {
	handler := NewGoogleAuthHandler(&fakeGoogleAuthService{result: &service.GoogleAuthResult{
		Token: "hiraya-token", UserID: 7, IsNewUser: true, NeedsProfile: true,
	}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/oauth/google/credential", bytes.NewBufferString(`{"credential":"google-token"}`))
	rr := httptest.NewRecorder()

	handler.Authenticate(rr, req)

	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"token":"hiraya-token"`) {
		t.Fatalf("unexpected response %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGoogleAuthenticateMapsAccountLinkConflict(t *testing.T) {
	handler := NewGoogleAuthHandler(&fakeGoogleAuthService{err: service.ErrGoogleAccountLinkNeeded})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/oauth/google/credential", bytes.NewBufferString(`{"credential":"google-token"}`))
	rr := httptest.NewRecorder()

	handler.Authenticate(rr, req)

	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), `"code":"account_link_required"`) {
		t.Fatalf("unexpected response %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGoogleAuthenticateRejectsMissingCredential(t *testing.T) {
	handler := NewGoogleAuthHandler(&fakeGoogleAuthService{err: errors.New("should not be called")})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/oauth/google/credential", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()

	handler.Authenticate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
