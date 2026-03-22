// Package email provides an email_send tool for the SoulGate orchestrator.
// It sends outbound email via SMTP using only Go's standard library.
//
// Configuration is read exclusively from environment variables so that
// credentials are never stored in workspace files:
//
//	SMTP_HOST  — SMTP server hostname  (default: localhost)
//	SMTP_PORT  — SMTP server port      (default: 587)
//	SMTP_USER  — SMTP username         (required unless auth not needed)
//	SMTP_PASS  — SMTP password         (required unless auth not needed)
//	SMTP_FROM  — Sender address        (defaults to SMTP_USER if empty)
package email

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

// SMTPConfig holds the SMTP connection parameters resolved from environment
// variables. All fields are read-only after construction.
type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// configFromEnv reads SMTP settings from environment variables, applying
// sensible defaults where the variable is absent.
func configFromEnv() SMTPConfig {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		host = "localhost"
	}

	port := 587
	if p := os.Getenv("SMTP_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			port = n
		}
	}

	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")

	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = user
	}

	return SMTPConfig{
		Host: host,
		Port: port,
		User: user,
		Pass: pass,
		From: from,
	}
}

// SendParams contains the parameters for a single email transmission.
type SendParams struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// validate returns an error if any required field is missing or malformed.
func (p SendParams) validate() error {
	p.To = strings.TrimSpace(p.To)
	p.Subject = strings.TrimSpace(p.Subject)

	if p.To == "" {
		return fmt.Errorf("email_send: 'to' address is required")
	}
	if !strings.Contains(p.To, "@") {
		return fmt.Errorf("email_send: 'to' does not look like an email address: %q", p.To)
	}
	if p.Subject == "" {
		return fmt.Errorf("email_send: 'subject' is required")
	}
	return nil
}

// Send transmits a single email message using the SMTP settings from env.
// The ctx deadline is honoured via a net.Dialer timeout: if the context
// is already cancelled before the dial, Send returns immediately with an error.
func Send(ctx context.Context, params SendParams) error {
	if err := params.validate(); err != nil {
		return err
	}

	cfg := configFromEnv()

	if cfg.From == "" {
		return fmt.Errorf("email_send: sender address not configured (set SMTP_FROM or SMTP_USER)")
	}

	// Compose the raw RFC-2822 message.
	msg := buildMessage(cfg.From, params.To, params.Subject, params.Body)

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	// Respect context cancellation / deadline.
	deadline, ok := ctx.Deadline()
	var dialTimeout time.Duration
	if ok {
		dialTimeout = time.Until(deadline)
		if dialTimeout <= 0 {
			return fmt.Errorf("email_send: context deadline already exceeded")
		}
	} else {
		dialTimeout = 30 * time.Second
	}

	// Use PlainAuth when credentials are present; skip auth otherwise
	// (useful for relay servers that don't require authentication).
	var auth smtp.Auth
	if cfg.User != "" && cfg.Pass != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
	}

	// net/smtp.SendMail does not accept a context directly, so we run it in a
	// goroutine and select on ctx.Done() to honour cancellation.
	type result struct {
		err error
	}

	ch := make(chan result, 1)
	go func() {
		// Attempt submission on port 587 (STARTTLS) or 25 (relay).
		// For port 465 (SMTPS/implicit TLS) a more complete implementation
		// would use tls.Dial — that path is noted here for future extension
		// but kept out of scope to stay stdlib-only.
		err := smtp.SendMail(addr, auth, cfg.From, []string{params.To}, []byte(msg))
		ch <- result{err: err}
	}()

	// Allow a local timeout even when the caller didn't set a deadline.
	timer := time.NewTimer(dialTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("email_send: %w", ctx.Err())
	case <-timer.C:
		return fmt.Errorf("email_send: timed out after %s", dialTimeout)
	case res := <-ch:
		if res.err != nil {
			return fmt.Errorf("email_send: smtp error: %w", res.err)
		}
		return nil
	}
}

// buildMessage composes a minimal RFC-2822 email message as a byte slice.
// Only plain-text bodies are supported; MIME encoding for non-ASCII content
// is intentionally out of scope for this MVP implementation.
func buildMessage(from, to, subject, body string) string {
	// Sanitise header values to prevent header injection.
	from = sanitizeHeader(from)
	to = sanitizeHeader(to)
	subject = sanitizeHeader(subject)

	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	// Body: convert bare LF to CRLF as required by RFC-2822.
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	sb.WriteString(body)
	return sb.String()
}

// sanitizeHeader strips CR and LF characters from a header value to prevent
// header injection attacks.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}
