package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/config"
)

type EmailMessage struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

type SMTPEmailSender struct {
	host      string
	port      int
	username  string
	password  string
	fromEmail string
	fromName  string
}

func NewSMTPEmailSender(cfg config.SMTPConfig) *SMTPEmailSender {
	return &SMTPEmailSender{
		host:      strings.TrimSpace(cfg.Host),
		port:      cfg.Port,
		username:  strings.TrimSpace(cfg.Username),
		password:  cfg.Password,
		fromEmail: strings.TrimSpace(cfg.FromEmail),
		fromName:  strings.TrimSpace(cfg.FromName),
	}
}

func (s *SMTPEmailSender) IsConfigured() bool {
	return s != nil && s.host != "" && s.port > 0 && s.fromEmail != ""
}

func (s *SMTPEmailSender) Send(ctx context.Context, msg EmailMessage) error {
	if !s.IsConfigured() {
		return nil
	}
	if _, err := mail.ParseAddress(msg.To); err != nil {
		return fmt.Errorf("invalid recipient email: %w", err)
	}
	if _, err := mail.ParseAddress(s.fromEmail); err != nil {
		return fmt.Errorf("invalid sender email: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	addr := net.JoinHostPort(s.host, fmt.Sprintf("%d", s.port))
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp client failed: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls failed: %w", err)
		}
	}

	if s.username != "" || s.password != "" {
		if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}

	if err := client.Mail(s.fromEmail); err != nil {
		return fmt.Errorf("smtp mail failed: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp recipient failed: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data failed: %w", err)
	}

	raw := renderSMTPMessage(s.fromAddress(), msg)
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp close data failed: %w", err)
	}
	return client.Quit()
}

func (s *SMTPEmailSender) fromAddress() string {
	if s.fromName == "" {
		return s.fromEmail
	}
	return (&mail.Address{Name: s.fromName, Address: s.fromEmail}).String()
}

func renderSMTPMessage(from string, msg EmailMessage) []byte {
	boundary := "relaxation-hub-booking-email"
	var buf bytes.Buffer
	writeHeader := func(key, value string) {
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(value)
		buf.WriteString("\r\n")
	}

	writeHeader("From", from)
	writeHeader("To", msg.To)
	writeHeader("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	writeHeader("MIME-Version", "1.0")
	writeHeader("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", boundary))
	buf.WriteString("\r\n")

	buf.WriteString("--" + boundary + "\r\n")
	writeHeader("Content-Type", "text/plain; charset=utf-8")
	writeHeader("Content-Transfer-Encoding", "quoted-printable")
	buf.WriteString("\r\n")
	buf.WriteString(quotePrintable(msg.TextBody))
	buf.WriteString("\r\n")

	buf.WriteString("--" + boundary + "\r\n")
	writeHeader("Content-Type", "text/html; charset=utf-8")
	writeHeader("Content-Transfer-Encoding", "quoted-printable")
	buf.WriteString("\r\n")
	buf.WriteString(quotePrintable(msg.HTMLBody))
	buf.WriteString("\r\n")
	buf.WriteString("--" + boundary + "--\r\n")

	return buf.Bytes()
}

func quotePrintable(s string) string {
	var buf bytes.Buffer
	w := quotedprintable.NewWriter(&buf)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return buf.String()
}
