// Package email provides email notification sending.
package email

import (
	"context"
	"errors"
	"log/slog"

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

// Sender implements email notification sender.
type Sender struct {
	config Config
}

// NewSender creates a new email sender.
// Returns error if enabled but required config is missing.
func NewSender(config Config) (*Sender, error) {
	if config.Enabled {
		if config.SMTPHost == "" {
			return nil, errors.New("email sender: SMTP host is required when enabled")
		}
		if config.SMTPPort == 0 {
			return nil, errors.New("email sender: SMTP port is required when enabled")
		}
		if config.FromAddress == "" {
			return nil, errors.New("email sender: from address is required when enabled")
		}
	}

	slog.Info("email sender configured",
		"enabled", config.Enabled,
		"smtp_host", config.SMTPHost,
		"smtp_port", config.SMTPPort,
		"from_address", config.FromAddress,
		"batch_size", config.BatchSize,
	)

	return &Sender{config: config}, nil
}

// Type returns the channel type.
func (s *Sender) Type() domain.ChannelType {
	return domain.ChannelTypeEmail
}

// Send sends an email notification.
// TODO: Implement actual SMTP sending.
func (s *Sender) Send(_ context.Context, notification notifications.Notification) error {
	if !s.config.Enabled {
		slog.Debug("email sender disabled, skipping",
			"to", notification.To,
		)
		return nil
	}

	slog.Info("sending email notification (stub)",
		"to", notification.To,
		"subject", notification.Subject,
		"smtp_host", s.config.SMTPHost,
	)

	return nil
}
