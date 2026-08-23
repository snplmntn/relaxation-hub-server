package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	paymongoBaseURL        = "https://api.paymongo.com/v1"
	paymongoRequestTimeout = 15 * time.Second
	// PayMongo rejects deliveries replayed long after the fact; we do the same
	// so a captured webhook body cannot be resent later.
	paymongoWebhookTolerance = 5 * time.Minute
)

// PayMongoChannel is a payment_method_types value.
type PayMongoChannel = string

const (
	PayMongoGCash PayMongoChannel = "gcash"
	PayMongoMaya  PayMongoChannel = "paymaya"
	PayMongoCard  PayMongoChannel = "card"
	PayMongoQRPh  PayMongoChannel = "qrph"
)

// IsPayMongoChannel reports whether a channel is one we offer. The customer
// picks the channel on our page, so exactly one is sent per session and
// PayMongo's hosted page opens straight into it.
func IsPayMongoChannel(c string) bool {
	switch c {
	case PayMongoGCash, PayMongoMaya, PayMongoCard, PayMongoQRPh:
		return true
	}
	return false
}

// PayMongoClient talks to PayMongo's Checkout Sessions API.
type PayMongoClient struct {
	secretKey     string
	webhookSecret string
	liveMode      bool
	baseURL       string
	http          *http.Client
}

func NewPayMongoClient(secretKey, webhookSecret string, liveMode bool) *PayMongoClient {
	return &PayMongoClient{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		liveMode:      liveMode,
		baseURL:       paymongoBaseURL,
		http:          &http.Client{Timeout: paymongoRequestTimeout},
	}
}

// CheckoutSessionParams describes one hosted checkout.
type CheckoutSessionParams struct {
	Amount      float64 // pesos
	Description string
	LineItem    string
	Channel     PayMongoChannel
	Reference   string
	SuccessURL  string
	CancelURL   string
	Email       string
	Name        string
}

// CheckoutSession is the subset of PayMongo's response we keep.
type CheckoutSession struct {
	ID          string
	CheckoutURL string
	Raw         []byte
}

// toCentavos converts pesos to the integer minor units PayMongo expects.
// Rounding rather than truncating matters: 2519.995 must not be charged as
// ₱2,519.99 when the booking total says ₱2,520.00.
func toCentavos(pesos float64) int64 {
	return int64(math.Round(pesos * 100))
}

// CreateCheckoutSession creates a single-channel hosted checkout and returns
// the URL to send the customer to.
func (c *PayMongoClient) CreateCheckoutSession(ctx context.Context, p CheckoutSessionParams) (*CheckoutSession, error) {
	if !IsPayMongoChannel(p.Channel) {
		return nil, fmt.Errorf("unsupported payment channel %q", p.Channel)
	}
	amount := toCentavos(p.Amount)
	if amount < 100 {
		return nil, fmt.Errorf("amount must be at least PHP 1.00")
	}

	attrs := map[string]any{
		"line_items": []map[string]any{{
			"name":     p.LineItem,
			"quantity": 1,
			"amount":   amount,
			"currency": "PHP",
		}},
		"payment_method_types": []string{p.Channel},
		"description":          p.Description,
		"reference_number":     p.Reference,
		"success_url":          p.SuccessURL,
		"cancel_url":           p.CancelURL,
		"send_email_receipt":   true,
		"show_line_items":      true,
		"metadata":             map[string]string{"checkout_reference": p.Reference},
	}
	if p.Email != "" || p.Name != "" {
		billing := map[string]any{}
		if p.Email != "" {
			billing["email"] = p.Email
			attrs["customer_email"] = p.Email
		}
		if p.Name != "" {
			billing["name"] = p.Name
		}
		attrs["billing"] = billing
	}

	body, err := json.Marshal(map[string]any{"data": map[string]any{"attributes": attrs}})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/checkout_sessions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// PayMongo uses the secret key as the Basic auth username with no password.
	req.SetBasicAuth(c.secretKey, "")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paymongo request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("paymongo checkout session failed (%d): %s", resp.StatusCode, payMongoErrorDetail(raw))
	}

	var parsed struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				CheckoutURL string `json:"checkout_url"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("paymongo response was not understood: %w", err)
	}
	if parsed.Data.ID == "" || parsed.Data.Attributes.CheckoutURL == "" {
		return nil, fmt.Errorf("paymongo returned no checkout url")
	}

	return &CheckoutSession{ID: parsed.Data.ID, CheckoutURL: parsed.Data.Attributes.CheckoutURL, Raw: raw}, nil
}

// payMongoErrorDetail pulls the human-readable message out of an error body so
// a failure reads as "gcash is not enabled" rather than a bare status code.
func payMongoErrorDetail(raw []byte) string {
	var parsed struct {
		Errors []struct {
			Detail string `json:"detail"`
			Code   string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil && len(parsed.Errors) > 0 {
		parts := make([]string, 0, len(parsed.Errors))
		for _, e := range parsed.Errors {
			if e.Detail != "" {
				parts = append(parts, e.Detail)
			} else if e.Code != "" {
				parts = append(parts, e.Code)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	if len(raw) > 300 {
		raw = raw[:300]
	}
	return string(raw)
}

// VerifyWebhookSignature checks the Paymongo-Signature header against the raw
// request body.
//
// The header is `t=<unix>,te=<test sig>,li=<live sig>`, and the signature is
// HMAC-SHA256 of "<t>.<raw body>" keyed with the webhook's signing secret. The
// body must be hashed exactly as received — re-serialising the parsed JSON
// yields different bytes and every check would fail.
func (c *PayMongoClient) VerifyWebhookSignature(header string, rawBody []byte, now time.Time) error {
	if c.webhookSecret == "" {
		return fmt.Errorf("webhook secret is not configured")
	}
	var ts, testSig, liveSig string
	for _, part := range strings.Split(header, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch key {
		case "t":
			ts = value
		case "te":
			testSig = value
		case "li":
			liveSig = value
		}
	}
	if ts == "" {
		return fmt.Errorf("signature header is missing a timestamp")
	}

	seconds, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("signature timestamp is not a unix time")
	}
	if delta := now.Sub(time.Unix(seconds, 0)); delta > paymongoWebhookTolerance || delta < -paymongoWebhookTolerance {
		return fmt.Errorf("signature timestamp is outside the accepted window")
	}

	expected := liveSig
	if !c.liveMode {
		expected = testSig
	}
	if expected == "" {
		return fmt.Errorf("signature header carries no signature for this mode")
	}

	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	computed := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(computed), []byte(expected)) {
		return fmt.Errorf("signature does not match")
	}
	return nil
}
