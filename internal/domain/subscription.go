package domain

import "time"

type SubscriptionChannel string

const (
	SubscriptionChannelEmail    SubscriptionChannel = "email"
	SubscriptionChannelTelegram SubscriptionChannel = "telegram"
)

type Subscription struct {
	ID         string
	UserID     *string
	Channel    SubscriptionChannel
	Target     string
	ServiceIDs []string
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
