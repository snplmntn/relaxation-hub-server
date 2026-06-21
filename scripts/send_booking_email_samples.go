//go:build ignore

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func main() {
	to := flag.String("to", "", "recipient email address")
	flag.Parse()

	if strings.TrimSpace(*to) == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/send_booking_email_samples.go -to recipient@example.com")
		os.Exit(2)
	}

	_ = godotenv.Load()

	smtpPort := 587
	if raw := strings.TrimSpace(os.Getenv("SMTP_PORT")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &smtpPort); err != nil {
			fmt.Fprintf(os.Stderr, "invalid SMTP_PORT %q\n", raw)
			os.Exit(1)
		}
	}

	sender := service.NewSMTPEmailSender(config.SMTPConfig{
		Host:      os.Getenv("SMTP_HOST"),
		Port:      smtpPort,
		Username:  os.Getenv("SMTP_USERNAME"),
		Password:  os.Getenv("SMTP_PASSWORD"),
		FromEmail: os.Getenv("SMTP_FROM_EMAIL"),
		FromName:  os.Getenv("SMTP_FROM_NAME"),
	})
	if !sender.IsConfigured() {
		fmt.Fprintln(os.Stderr, "SMTP is not fully configured")
		os.Exit(1)
	}

	data := service.BookingEmailData{
		ClientName:    "Marc",
		ClientEmail:   strings.TrimSpace(*to),
		TherapistName: "Hiraya Therapist",
		ServiceName:   "Relaxation Massage",
		ScheduledDate: time.Now().In(time.FixedZone("Asia/Manila", 8*60*60)).Format("Monday, January 2, 2006"),
		ScheduledTime: "7:00 AM",
		Address:       "Sample address only, Manila",
		ReferenceCode: "TEST-EMAIL",
		Duration:      "1 hour",
		Total:         "PHP 0.00",
	}

	templates := []string{
		"advanced_booking_confirmed",
		"advanced_booking_d_day",
		"therapist_on_the_way",
		"booking_completed_success",
	}

	ctx := context.Background()
	for _, template := range templates {
		msg := service.RenderBookingEmail(template, data)
		if msg.Subject == "" {
			fmt.Fprintf(os.Stderr, "unknown booking email template: %s\n", template)
			os.Exit(1)
		}
		msg.To = data.ClientEmail
		msg.Subject = "[TEST] " + msg.Subject

		if err := sender.Send(ctx, msg); err != nil {
			fmt.Fprintf(os.Stderr, "send failed for %s: %v\n", template, err)
			os.Exit(1)
		}
		fmt.Printf("sent %s to %s\n", template, data.ClientEmail)
	}
}
