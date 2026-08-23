package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestToCentavos(t *testing.T) {
	cases := []struct {
		pesos float64
		want  int64
	}{
		{2800, 280000},
		{2520, 252000},
		{1, 100},
		{0.5, 50},
		// Floating point totals must round to the nearest centavo, not truncate:
		// truncating would charge a customer less than the booking says.
		{2519.995, 252000},
		{1234.567, 123457},
	}
	for _, c := range cases {
		if got := toCentavos(c.pesos); got != c.want {
			t.Errorf("toCentavos(%v) = %d, want %d", c.pesos, got, c.want)
		}
	}
}

func signature(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookSignature(t *testing.T) {
	const secret = "whsk_test_secret"
	now := time.Unix(1_700_000_000, 0)
	ts := fmt.Sprintf("%d", now.Unix())
	body := []byte(`{"data":{"id":"evt_1","attributes":{"type":"payment.paid"}}}`)
	good := signature(secret, ts, body)

	test := NewPayMongoClient("sk_test_x", secret, false)
	live := NewPayMongoClient("sk_live_x", secret, true)

	t.Run("accepts a correct test signature", func(t *testing.T) {
		header := fmt.Sprintf("t=%s,te=%s,li=%s", ts, good, "deadbeef")
		if err := test.VerifyWebhookSignature(header, body, now); err != nil {
			t.Fatalf("expected valid signature, got %v", err)
		}
	})

	t.Run("reads the live signature in live mode", func(t *testing.T) {
		header := fmt.Sprintf("t=%s,te=%s,li=%s", ts, "deadbeef", good)
		if err := live.VerifyWebhookSignature(header, body, now); err != nil {
			t.Fatalf("expected valid signature, got %v", err)
		}
		// The same header must fail in test mode: the two signatures are not
		// interchangeable, and accepting either would let a test-mode secret
		// authorise live payments.
		if err := test.VerifyWebhookSignature(header, body, now); err == nil {
			t.Fatal("test mode accepted a live-only signature")
		}
	})

	t.Run("rejects a tampered body", func(t *testing.T) {
		header := fmt.Sprintf("t=%s,te=%s", ts, good)
		if err := test.VerifyWebhookSignature(header, []byte(`{"data":{"id":"evt_forged"}}`), now); err == nil {
			t.Fatal("accepted a body that did not match the signature")
		}
	})

	t.Run("rejects a replayed delivery", func(t *testing.T) {
		header := fmt.Sprintf("t=%s,te=%s", ts, good)
		later := now.Add(paymongoWebhookTolerance + time.Minute)
		if err := test.VerifyWebhookSignature(header, body, later); err == nil {
			t.Fatal("accepted a signature outside the replay window")
		}
	})

	t.Run("rejects a malformed or empty header", func(t *testing.T) {
		for _, header := range []string{"", "garbage", fmt.Sprintf("t=%s", ts), fmt.Sprintf("t=notatime,te=%s", good)} {
			if err := test.VerifyWebhookSignature(header, body, now); err == nil {
				t.Errorf("accepted malformed header %q", header)
			}
		}
	})

	t.Run("refuses when no secret is configured", func(t *testing.T) {
		unset := NewPayMongoClient("sk_test_x", "", false)
		header := fmt.Sprintf("t=%s,te=%s", ts, good)
		if err := unset.VerifyWebhookSignature(header, body, now); err == nil {
			t.Fatal("verified a webhook with no configured secret")
		}
	})
}

func TestIsPayMongoChannel(t *testing.T) {
	for _, c := range []string{"gcash", "paymaya", "card", "qrph"} {
		if !IsPayMongoChannel(c) {
			t.Errorf("expected %q to be a supported channel", c)
		}
	}
	// "maya" is our internal manual-transfer value; PayMongo's is "paymaya".
	// grab_pay is a real PayMongo channel but is not activated on the account,
	// so a session naming it would fail at PayMongo rather than here.
	for _, c := range []string{"", "maya", "cash", "bdo", "online", "GCash", "grab_pay"} {
		if IsPayMongoChannel(c) {
			t.Errorf("expected %q to be rejected", c)
		}
	}
}
