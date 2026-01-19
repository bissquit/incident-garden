// Package telegram provides telegram notification sending.
package telegram

import (
	"context"
	"log/slog"

	"github.com/bissquit/incident-management/internal/domain"
	"github.com/bissquit/incident-management/internal/notifications"
)

// Config holds telegram sender configuration.
type Config struct {
	BotToken string
}

// Sender implements telegram notification sender.
type Sender struct {
	config Config
}

// NewSender creates a new telegram sender.
func NewSender(config Config) *Sender {
	return &Sender{config: config}
}

// Type returns the channel type.
func (s *Sender) Type() domain.ChannelType {
	return domain.ChannelTypeTelegram
}

// Send sends a telegram notification.
func (s *Sender) Send(_ context.Context, notification notifications.Notification) error {
	slog.Info("sending telegram notification",
		"to", notification.To,
		"subject", notification.Subject,
	)

	return nil
}
