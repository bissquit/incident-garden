// Package email provides email notification sending.
package email

import (
	"context"
	"log/slog"

	"github.com/bissquit/incident-management/internal/domain"
	"github.com/bissquit/incident-management/internal/notifications"
)

// Config holds email sender configuration.
type Config struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromAddress  string
}

// Sender implements email notification sender.
type Sender struct {
	config Config
}

// NewSender creates a new email sender.
func NewSender(config Config) *Sender {
	return &Sender{config: config}
}

// Type returns the channel type.
func (s *Sender) Type() domain.ChannelType {
	return domain.ChannelTypeEmail
}

// Send sends an email notification.
func (s *Sender) Send(_ context.Context, notification notifications.Notification) error {
	slog.Info("sending email notification",
		"to", notification.To,
		"subject", notification.Subject,
	)

	return nil
}
