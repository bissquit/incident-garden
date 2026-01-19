package notifications

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bissquit/incident-management/internal/domain"
)

// Dispatcher sends notifications to subscribers.
type Dispatcher struct {
	repo    Repository
	senders map[domain.ChannelType]Sender
}

// NewDispatcher creates a new notification dispatcher.
func NewDispatcher(repo Repository, senders ...Sender) *Dispatcher {
	senderMap := make(map[domain.ChannelType]Sender)
	for _, s := range senders {
		senderMap[s.Type()] = s
	}
	return &Dispatcher{
		repo:    repo,
		senders: senderMap,
	}
}

// DispatchInput contains data for dispatching notifications.
type DispatchInput struct {
	ServiceIDs []string
	Subject    string
	Body       string
}

// Dispatch sends notifications to all subscribers of the given services.
func (d *Dispatcher) Dispatch(ctx context.Context, input DispatchInput) error {
	subscribers, err := d.repo.GetSubscribersForServices(ctx, input.ServiceIDs)
	if err != nil {
		return fmt.Errorf("get subscribers: %w", err)
	}

	slog.Info("dispatching notifications",
		"service_ids", input.ServiceIDs,
		"subscriber_count", len(subscribers),
	)

	for _, sub := range subscribers {
		for _, channel := range sub.Channels {
			if !channel.IsEnabled || !channel.IsVerified {
				continue
			}

			sender, ok := d.senders[channel.Type]
			if !ok {
				slog.Warn("no sender for channel type", "type", channel.Type)
				continue
			}

			notification := Notification{
				To:      channel.Target,
				Subject: input.Subject,
				Body:    input.Body,
			}

			if err := sender.Send(ctx, notification); err != nil {
				slog.Error("failed to send notification",
					"channel_type", channel.Type,
					"target", channel.Target,
					"error", err,
				)
			}
		}
	}

	return nil
}
