package notifications

import (
	"context"

	"github.com/bissquit/incident-garden/internal/domain"
)

// Notification represents a notification to be sent.
type Notification struct {
	To      string
	Subject string
	Body    string
}

// Sender interface for different notification channels.
type Sender interface {
	Send(ctx context.Context, notification Notification) error
	Type() domain.ChannelType
}
