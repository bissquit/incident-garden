// Package email provides email notification sending via SMTP.
package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/notifications"
)

// Config holds email sender configuration.
type Config struct {
	Enabled      bool
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromAddress  string
	BatchSize    int
}

// Sender implements email notification sender via SMTP.
type Sender struct {
	config Config
	auth   smtp.Auth
}

// NewSender creates a new email sender.
// Returns error if enabled but required config is missing.
func NewSender(config Config) (*Sender, error) {
	if config.Enabled {
		if config.SMTPHost == "" {
			return nil, errors.New("email sender: SMTP host is required when enabled")
		}
		if config.FromAddress == "" {
			return nil, errors.New("email sender: from address is required when enabled")
		}
	}

	// Set defaults
	if config.SMTPPort == 0 {
		config.SMTPPort = 587
	}
	if config.BatchSize == 0 {
		config.BatchSize = 50
	}

	var auth smtp.Auth
	if config.SMTPUser != "" && config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", config.SMTPUser, config.SMTPPassword, config.SMTPHost)
	}

	slog.Info("email sender configured",
		"enabled", config.Enabled,
		"smtp_host", config.SMTPHost,
		"smtp_port", config.SMTPPort,
		"from_address", config.FromAddress,
		"batch_size", config.BatchSize,
	)

	return &Sender{
		config: config,
		auth:   auth,
	}, nil
}

// Type returns the channel type.
func (s *Sender) Type() domain.ChannelType {
	return domain.ChannelTypeEmail
}

// Send sends an email notification to a single recipient.
func (s *Sender) Send(ctx context.Context, notification notifications.Notification) error {
	if !s.config.Enabled {
		slog.Warn("email sender disabled, skipping send",
			"recipient_count", 1,
		)
		return nil
	}

	return s.sendEmail(ctx, notification.Subject, notification.Body, []string{notification.To})
}

// SendBatch sends an email to multiple recipients using BCC.
// Recipients are split into batches to respect SMTP server limits.
func (s *Sender) SendBatch(ctx context.Context, subject, body string, recipients []string) error {
	if !s.config.Enabled {
		slog.Warn("email sender disabled, skipping send",
			"recipient_count", len(recipients),
		)
		return nil
	}

	if len(recipients) == 0 {
		return nil
	}

	var lastErr error
	for i := 0; i < len(recipients); i += s.config.BatchSize {
		end := min(i+s.config.BatchSize, len(recipients))
		batch := recipients[i:end]

		if err := s.sendEmail(ctx, subject, body, batch); err != nil {
			slog.Error("failed to send email batch",
				"batch_start", i,
				"batch_size", len(batch),
				"error", err,
			)
			lastErr = err
			continue
		}

		slog.Info("email batch sent",
			"batch_start", i,
			"batch_size", len(batch),
		)
	}

	return lastErr
}

// sendEmail sends an email to the specified recipients.
func (s *Sender) sendEmail(ctx context.Context, subject, body string, recipients []string) error {
	msg := s.buildMessage(subject, body)
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	tlsConfig := &tls.Config{
		ServerName: s.config.SMTPHost,
		MinVersion: tls.VersionTLS12,
	}

	return s.sendWithSTARTTLS(ctx, addr, tlsConfig, recipients, msg)
}

// buildMessage constructs the email message with headers.
func (s *Sender) buildMessage(subject, body string) []byte {
	var msg strings.Builder

	// Headers in deterministic order
	msg.WriteString(fmt.Sprintf("From: %s\r\n", s.config.FromAddress))
	msg.WriteString("To: undisclosed-recipients:;\r\n")
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return []byte(msg.String())
}

// sendWithSTARTTLS sends an email using STARTTLS (port 587).
func (s *Sender) sendWithSTARTTLS(ctx context.Context, addr string, tlsConfig *tls.Config, recipients []string, msg []byte) error {
	// Dial with timeout
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Create SMTP client
	client, err := smtp.NewClient(conn, s.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// STARTTLS if available
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	// Authenticate if credentials provided
	if s.auth != nil {
		if err := client.Auth(s.auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	// Set sender
	from := extractEmail(s.config.FromAddress)
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}

	// Add recipients (BCC - recipients are in envelope, not headers)
	var addedRecipients int
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			slog.Warn("failed to add recipient",
				"error", err,
			)
			continue
		}
		addedRecipients++
	}

	if addedRecipients == 0 {
		return errors.New("no valid recipients")
	}

	// Send message data
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}

	return client.Quit()
}

// extractEmail extracts the email address from formats like "Name <email@example.com>".
func extractEmail(address string) string {
	if idx := strings.Index(address, "<"); idx != -1 {
		end := strings.Index(address, ">")
		if end > idx {
			return address[idx+1 : end]
		}
	}
	return address
}

// IsRetryable determines if an error is retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Network timeout errors are retryable
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Connection refused is retryable
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	errStr := err.Error()

	// SMTP 4xx codes are temporary failures (retryable)
	if strings.Contains(errStr, "421") || // Service not available
		strings.Contains(errStr, "450") || // Mailbox unavailable
		strings.Contains(errStr, "451") || // Local error
		strings.Contains(errStr, "452") { // Insufficient storage
		return true
	}

	// 552 - Mailbox full is sometimes retryable
	if strings.Contains(errStr, "552") {
		return true
	}

	return false
}
