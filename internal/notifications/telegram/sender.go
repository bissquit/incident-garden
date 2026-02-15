// Package telegram provides telegram notification sending.
package telegram

import (
	"context"
	"errors"
	"log/slog"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/notifications"
)

// Config holds telegram sender configuration.
type Config struct {
	Enabled   bool
	BotToken  string
	RateLimit float64
}

// Sender implements telegram notification sender.
type Sender struct {
	config Config
}

// NewSender creates a new telegram sender.
// Returns error if enabled but required config is missing.
func NewSender(config Config) (*Sender, error) {
	if config.Enabled {
		if config.BotToken == "" {
			return nil, errors.New("telegram sender: bot token is required when enabled")
		}
	}

	slog.Info("telegram sender configured",
		"enabled", config.Enabled,
		"rate_limit", config.RateLimit,
	)

	return &Sender{config: config}, nil
}

// Type returns the channel type.
func (s *Sender) Type() domain.ChannelType {
	return domain.ChannelTypeTelegram
}

// Send sends a telegram notification.
// TODO: Implement actual Telegram Bot API sending.
func (s *Sender) Send(_ context.Context, notification notifications.Notification) error {
	if !s.config.Enabled {
		slog.Debug("telegram sender disabled, skipping",
			"to", notification.To,
		)
		return nil
	}

	slog.Info("sending telegram notification (stub)",
		"to", notification.To,
		"subject", notification.Subject,
	)

	return nil
}
