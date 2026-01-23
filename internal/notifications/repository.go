// Package notifications provides notification channel and subscription management.
package notifications

import (
	"context"

	"github.com/bissquit/incident-garden/internal/domain"
)

// Repository defines the interface for notifications data access.
type Repository interface {
	CreateChannel(ctx context.Context, channel *domain.NotificationChannel) error
	GetChannelByID(ctx context.Context, id string) (*domain.NotificationChannel, error)
	ListUserChannels(ctx context.Context, userID string) ([]domain.NotificationChannel, error)
	UpdateChannel(ctx context.Context, channel *domain.NotificationChannel) error
	DeleteChannel(ctx context.Context, id string) error

	CreateSubscription(ctx context.Context, subscription *domain.Subscription) error
	GetSubscriptionByID(ctx context.Context, id string) (*domain.Subscription, error)
	GetUserSubscription(ctx context.Context, userID string) (*domain.Subscription, error)
	SetSubscriptionServices(ctx context.Context, subscriptionID string, serviceIDs []string) error
	DeleteSubscription(ctx context.Context, id string) error

	GetSubscribersForServices(ctx context.Context, serviceIDs []string) ([]SubscriberInfo, error)
}

// SubscriberInfo contains user notification info for dispatcher.
type SubscriberInfo struct {
	UserID   string
	Email    string
	Channels []domain.NotificationChannel
}
