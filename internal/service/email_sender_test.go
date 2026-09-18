package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/config"
)

func TestBrevoEmailSenderSendsTransactionalEmail(t *testing.T) {
	var payload struct {
		Sender      map[string]string   `json:"sender"`
		To          []map[string]string `json:"to"`
		Subject     string              `json:"subject"`
		HTMLContent string              `json:"htmlContent"`
		TextContent string              `json:"textContent"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("api-key"); got != "brevo-key" {
			t.Fatalf("expected Brevo API key, got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sender := NewBrevoEmailSender(config.BrevoConfig{
		APIKey:    "brevo-key",
		FromEmail: "bookings@kalinga.example",
		FromName:  "Kalinga Spa",
	})
	sender.endpoint = server.URL

	err := sender.Send(context.Background(), EmailMessage{
		To:       "client@example.com",
		Subject:  "Booking complete",
		TextBody: "Thank you",
		HTMLBody: "<p>Thank you</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload.Sender["email"] != "bookings@kalinga.example" || payload.To[0]["email"] != "client@example.com" {
		t.Fatalf("unexpected Brevo payload: %#v", payload)
	}
	if payload.Subject != "Booking complete" || payload.TextContent != "Thank you" || payload.HTMLContent != "<p>Thank you</p>" {
		t.Fatalf("unexpected Brevo content: %#v", payload)
	}
}

func TestBrevoEmailSenderRetriesServerErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sender := NewBrevoEmailSender(config.BrevoConfig{
		APIKey:    "brevo-key",
		FromEmail: "bookings@kalinga.example",
	})
	sender.endpoint = server.URL

	if err := sender.Send(context.Background(), EmailMessage{To: "client@example.com", Subject: "Booking complete"}); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}
