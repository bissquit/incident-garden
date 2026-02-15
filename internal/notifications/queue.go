package notifications

import "time"

// QueueStatus represents the status of a queue item.
type QueueStatus string

// Queue statuses.
const (
	QueueStatusPending    QueueStatus = "pending"
	QueueStatusProcessing QueueStatus = "processing"
	QueueStatusSent       QueueStatus = "sent"
	QueueStatusFailed     QueueStatus = "failed"
)

// QueueItem represents a notification in the queue.
type QueueItem struct {
	ID            string
	EventID       string
	ChannelID     string
	MessageType   MessageType
	Payload       NotificationPayload
	Status        QueueStatus
	Attempts      int
	MaxAttempts   int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SentAt        *time.Time
}
