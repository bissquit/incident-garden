package domain

import "time"

// ChannelType represents a notification channel type.
type ChannelType string

// Notification channel types.
const (
	ChannelTypeEmail      ChannelType = "email"
	ChannelTypeTelegram   ChannelType = "telegram"
	ChannelTypeMattermost ChannelType = "mattermost"
)

// NotificationChannel represents a user's notification channel.
type NotificationChannel struct {
	ID                     string      `json:"id"`
	UserID                 string      `json:"user_id"`
	Type                   ChannelType `json:"type"`
	Target                 string      `json:"target"`
	IsEnabled              bool        `json:"is_enabled"`
	IsVerified             bool        `json:"is_verified"`
	SubscribeToAllServices bool        `json:"subscribe_to_all_services"`
	CreatedAt              time.Time   `json:"created_at"`
	UpdatedAt              time.Time   `json:"updated_at"`
}
