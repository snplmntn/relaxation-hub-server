package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/config"
)

const brevoEmailEndpoint = "https://api.brevo.com/v3/smtp/email"

type EmailMessage struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

type BrevoEmailSender struct {
	apiKey    string
	fromEmail string
	fromName  string
	endpoint  string
	client    *http.Client
}

func NewBrevoEmailSender(cfg config.BrevoConfig) *BrevoEmailSender {
	return &BrevoEmailSender{
		apiKey:    strings.TrimSpace(cfg.APIKey),
		fromEmail: strings.TrimSpace(cfg.FromEmail),
		fromName:  strings.TrimSpace(cfg.FromName),
		endpoint:  brevoEmailEndpoint,
		client:    &http.Client{},
	}
}

func (s *BrevoEmailSender) IsConfigured() bool {
	return s != nil && s.apiKey != "" && s.fromEmail != ""
}

func (s *BrevoEmailSender) Send(ctx context.Context, msg EmailMessage) error {
	if !s.IsConfigured() {
		return nil
	}
	recipient, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %w", err)
	}
	sender, err := mail.ParseAddress(s.fromEmail)
	if err != nil {
		return fmt.Errorf("invalid sender email: %w", err)
	}

	body, err := json.Marshal(struct {
		Sender      map[string]string   `json:"sender"`
		To          []map[string]string `json:"to"`
		Subject     string              `json:"subject"`
		HTMLContent string              `json:"htmlContent,omitempty"`
		TextContent string              `json:"textContent,omitempty"`
	}{
		Sender:      map[string]string{"email": sender.Address, "name": s.fromName},
		To:          []map[string]string{{"email": recipient.Address, "name": recipient.Name}},
		Subject:     msg.Subject,
		HTMLContent: msg.HTMLBody,
		TextContent: msg.TextBody,
	})
	if err != nil {
		return fmt.Errorf("encode Brevo email: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create Brevo request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("api-key", s.apiKey)

		resp, err := s.client.Do(req)
		if err == nil {
			responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if readErr != nil {
				return fmt.Errorf("read Brevo response: %w", readErr)
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("Brevo email failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				return lastErr
			}
		} else {
			lastErr = fmt.Errorf("Brevo request failed: %w", err)
		}

		if attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
			}
		}
	}
	return lastErr
}
