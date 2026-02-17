// Package notifications provides notification channel and subscription management.
package notifications

import (
	"context"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
)

// Repository defines the interface for notifications data access.
type Repository interface {
	// Channel CRUD
	CreateChannel(ctx context.Context, channel *domain.NotificationChannel) error
	GetChannelByID(ctx context.Context, id string) (*domain.NotificationChannel, error)
	GetChannelByUserAndTarget(ctx context.Context, userID string, channelType domain.ChannelType, target string) (*domain.NotificationChannel, error)
	ListUserChannels(ctx context.Context, userID string) ([]domain.NotificationChannel, error)
	UpdateChannel(ctx context.Context, channel *domain.NotificationChannel) error
	DeleteChannel(ctx context.Context, id string) error

	// Channel subscriptions
	SetChannelSubscriptions(ctx context.Context, channelID string, subscribeAll bool, serviceIDs []string) error
	GetChannelSubscriptions(ctx context.Context, channelID string) (subscribeAll bool, serviceIDs []string, err error)
	GetUserChannelsWithSubscriptions(ctx context.Context, userID string) ([]ChannelWithSubscriptions, error)

	// Event subscribers
	CreateEventSubscribers(ctx context.Context, eventID string, channelIDs []string) error
	GetEventSubscribers(ctx context.Context, eventID string) ([]string, error)
	AddEventSubscribers(ctx context.Context, eventID string, channelIDs []string) error

	// Find subscribers for services (returns channels that are subscribed to any of the given services)
	FindSubscribersForServices(ctx context.Context, serviceIDs []string) ([]ChannelInfo, error)

	// GetChannelsByIDs returns channels by their IDs
	GetChannelsByIDs(ctx context.Context, ids []string) ([]ChannelInfo, error)

	// Verification codes
	CreateVerificationCode(ctx context.Context, channelID string, code string, expiresAt time.Time) error
	GetVerificationCode(ctx context.Context, channelID string) (*VerificationCode, error)
	IncrementCodeAttempts(ctx context.Context, channelID string) error
	DeleteVerificationCode(ctx context.Context, channelID string) error
	DeleteExpiredCodes(ctx context.Context) (int64, error)

	// Queue operations
	EnqueueNotification(ctx context.Context, item *QueueItem) error
	EnqueueBatch(ctx context.Context, items []*QueueItem) error
	FetchPendingNotifications(ctx context.Context, limit int) ([]*QueueItem, error)
	MarkAsProcessing(ctx context.Context, id string) error
	MarkAsSent(ctx context.Context, id string) error
	MarkAsFailed(ctx context.Context, id string, err error) error
	MarkForRetry(ctx context.Context, id string, err error, nextAttempt time.Time) error

	// Queue management and recovery
	GetFailedItems(ctx context.Context, limit int) ([]*QueueItem, error)
	RetryFailedItem(ctx context.Context, id string) error
	RecoverStuckProcessing(ctx context.Context, stuckFor time.Duration) (int64, error)
	DeleteOldSentItems(ctx context.Context, olderThan time.Duration) (int64, error)
	GetQueueStats(ctx context.Context) (*QueueStats, error)
}

// ChannelInfo contains notification channel info for dispatcher.
type ChannelInfo struct {
	ID       string
	UserID   string
	Type     domain.ChannelType
	Target   string
	Email    string // User's email (for context)
}

// ChannelWithSubscriptions contains channel with its subscription settings.
type ChannelWithSubscriptions struct {
	Channel                domain.NotificationChannel `json:"channel"`
	SubscribeToAllServices bool                       `json:"subscribe_to_all_services"`
	SubscribedServiceIDs   []string                   `json:"subscribed_service_ids"`
}

// VerificationCode represents a channel verification code.
type VerificationCode struct {
	ID        string
	ChannelID string
	Code      string
	ExpiresAt time.Time
	Attempts  int
	CreatedAt time.Time
}

// QueueStats contains queue statistics.
type QueueStats struct {
	Pending    int64
	Processing int64
	Sent       int64
	Failed     int64
}
