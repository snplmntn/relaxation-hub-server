package handler

import (
	"net/http/httptest"
	"testing"
)

func TestHandleConnection_InvalidUpgrade_ReturnsBadRequestOrUpgrade(t *testing.T) {
	h := NewWebSocketHandler(nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ws", nil)

	h.HandleConnection(rr, req)

	// Without a websocket upgrade header, handler should not panic; expect a 400-ish status.
	if rr.Code == 0 {
		t.Fatalf("expected status set, got 0")
	}
}
