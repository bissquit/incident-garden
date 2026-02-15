// Package postgres provides PostgreSQL implementation of notifications repository.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/notifications"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements notifications.Repository using PostgreSQL.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateChannel creates a new notification channel.
func (r *Repository) CreateChannel(ctx context.Context, channel *domain.NotificationChannel) error {
	query := `
		INSERT INTO notification_channels (user_id, type, target, is_enabled, is_verified, subscribe_to_all_services)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		channel.UserID,
		channel.Type,
		channel.Target,
		channel.IsEnabled,
		channel.IsVerified,
		channel.SubscribeToAllServices,
	).Scan(&channel.ID, &channel.CreatedAt, &channel.UpdatedAt)
}

// GetChannelByID retrieves a notification channel by ID.
func (r *Repository) GetChannelByID(ctx context.Context, id string) (*domain.NotificationChannel, error) {
	query := `
		SELECT id, user_id, type, target, is_enabled, is_verified, subscribe_to_all_services, created_at, updated_at
		FROM notification_channels
		WHERE id = $1
	`
	var channel domain.NotificationChannel
	err := r.db.QueryRow(ctx, query, id).Scan(
		&channel.ID,
		&channel.UserID,
		&channel.Type,
		&channel.Target,
		&channel.IsEnabled,
		&channel.IsVerified,
		&channel.SubscribeToAllServices,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notifications.ErrChannelNotFound
		}
		return nil, fmt.Errorf("get channel: %w", err)
	}
	return &channel, nil
}

// ListUserChannels retrieves all notification channels for a user.
func (r *Repository) ListUserChannels(ctx context.Context, userID string) ([]domain.NotificationChannel, error) {
	query := `
		SELECT id, user_id, type, target, is_enabled, is_verified, subscribe_to_all_services, created_at, updated_at
		FROM notification_channels
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list user channels: %w", err)
	}
	defer rows.Close()

	channels := make([]domain.NotificationChannel, 0)
	for rows.Next() {
		var channel domain.NotificationChannel
		err := rows.Scan(
			&channel.ID,
			&channel.UserID,
			&channel.Type,
			&channel.Target,
			&channel.IsEnabled,
			&channel.IsVerified,
			&channel.SubscribeToAllServices,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, channel)
	}

	return channels, nil
}

// UpdateChannel updates an existing notification channel.
func (r *Repository) UpdateChannel(ctx context.Context, channel *domain.NotificationChannel) error {
	query := `
		UPDATE notification_channels
		SET is_enabled = $2, is_verified = $3, subscribe_to_all_services = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		channel.ID,
		channel.IsEnabled,
		channel.IsVerified,
		channel.SubscribeToAllServices,
	).Scan(&channel.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notifications.ErrChannelNotFound
		}
		return fmt.Errorf("update channel: %w", err)
	}
	return nil
}

// DeleteChannel deletes a notification channel.
func (r *Repository) DeleteChannel(ctx context.Context, id string) error {
	query := `DELETE FROM notification_channels WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}

	if result.RowsAffected() == 0 {
		return notifications.ErrChannelNotFound
	}
	return nil
}

// SetChannelSubscriptions sets subscriptions for a channel.
// If subscribeAll is true, serviceIDs are ignored and channel subscribes to all services.
// If subscribeAll is false, channel subscribes only to specified services.
func (r *Repository) SetChannelSubscriptions(ctx context.Context, channelID string, subscribeAll bool, serviceIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Update subscribe_to_all_services flag
	updateQuery := `UPDATE notification_channels SET subscribe_to_all_services = $2, updated_at = NOW() WHERE id = $1`
	result, err := tx.Exec(ctx, updateQuery, channelID, subscribeAll)
	if err != nil {
		return fmt.Errorf("update subscribe_to_all_services: %w", err)
	}
	if result.RowsAffected() == 0 {
		return notifications.ErrChannelNotFound
	}

	// Clear existing subscriptions
	deleteQuery := `DELETE FROM channel_subscriptions WHERE channel_id = $1`
	if _, err := tx.Exec(ctx, deleteQuery, channelID); err != nil {
		return fmt.Errorf("delete old subscriptions: %w", err)
	}

	// If not subscribeAll, insert specific service subscriptions
	if !subscribeAll && len(serviceIDs) > 0 {
		insertQuery := `INSERT INTO channel_subscriptions (channel_id, service_id) VALUES ($1, $2)`
		for _, serviceID := range serviceIDs {
			if _, err := tx.Exec(ctx, insertQuery, channelID, serviceID); err != nil {
				return fmt.Errorf("insert subscription for service %s: %w", serviceID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetChannelSubscriptions returns subscription settings for a channel.
func (r *Repository) GetChannelSubscriptions(ctx context.Context, channelID string) (bool, []string, error) {
	// Get subscribe_to_all_services flag
	var subscribeAll bool
	flagQuery := `SELECT subscribe_to_all_services FROM notification_channels WHERE id = $1`
	err := r.db.QueryRow(ctx, flagQuery, channelID).Scan(&subscribeAll)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, notifications.ErrChannelNotFound
		}
		return false, nil, fmt.Errorf("get subscribe_to_all_services: %w", err)
	}

	// Get specific service subscriptions
	servicesQuery := `SELECT service_id FROM channel_subscriptions WHERE channel_id = $1`
	rows, err := r.db.Query(ctx, servicesQuery, channelID)
	if err != nil {
		return false, nil, fmt.Errorf("get channel subscriptions: %w", err)
	}
	defer rows.Close()

	serviceIDs := make([]string, 0)
	for rows.Next() {
		var serviceID string
		if err := rows.Scan(&serviceID); err != nil {
			return false, nil, fmt.Errorf("scan service id: %w", err)
		}
		serviceIDs = append(serviceIDs, serviceID)
	}

	return subscribeAll, serviceIDs, nil
}

// GetUserChannelsWithSubscriptions returns all channels for a user with their subscription settings.
func (r *Repository) GetUserChannelsWithSubscriptions(ctx context.Context, userID string) ([]notifications.ChannelWithSubscriptions, error) {
	channels, err := r.ListUserChannels(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]notifications.ChannelWithSubscriptions, 0, len(channels))
	for _, ch := range channels {
		subscribeAll, serviceIDs, err := r.GetChannelSubscriptions(ctx, ch.ID)
		if err != nil {
			return nil, fmt.Errorf("get subscriptions for channel %s: %w", ch.ID, err)
		}

		result = append(result, notifications.ChannelWithSubscriptions{
			Channel:                ch,
			SubscribeToAllServices: subscribeAll,
			SubscribedServiceIDs:   serviceIDs,
		})
	}

	return result, nil
}

// CreateEventSubscribers creates event subscribers (replaces any existing).
func (r *Repository) CreateEventSubscribers(ctx context.Context, eventID string, channelIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Clear existing subscribers
	deleteQuery := `DELETE FROM event_subscribers WHERE event_id = $1`
	if _, err := tx.Exec(ctx, deleteQuery, eventID); err != nil {
		return fmt.Errorf("delete existing subscribers: %w", err)
	}

	// Insert new subscribers
	if len(channelIDs) > 0 {
		insertQuery := `INSERT INTO event_subscribers (event_id, channel_id) VALUES ($1, $2)`
		for _, channelID := range channelIDs {
			if _, err := tx.Exec(ctx, insertQuery, eventID, channelID); err != nil {
				return fmt.Errorf("insert subscriber %s: %w", channelID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetEventSubscribers returns channel IDs subscribed to an event.
func (r *Repository) GetEventSubscribers(ctx context.Context, eventID string) ([]string, error) {
	query := `SELECT channel_id FROM event_subscribers WHERE event_id = $1`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event subscribers: %w", err)
	}
	defer rows.Close()

	channelIDs := make([]string, 0)
	for rows.Next() {
		var channelID string
		if err := rows.Scan(&channelID); err != nil {
			return nil, fmt.Errorf("scan channel id: %w", err)
		}
		channelIDs = append(channelIDs, channelID)
	}

	return channelIDs, nil
}

// AddEventSubscribers adds channels to event subscribers (skips duplicates).
func (r *Repository) AddEventSubscribers(ctx context.Context, eventID string, channelIDs []string) error {
	if len(channelIDs) == 0 {
		return nil
	}

	query := `INSERT INTO event_subscribers (event_id, channel_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	for _, channelID := range channelIDs {
		if _, err := r.db.Exec(ctx, query, eventID, channelID); err != nil {
			return fmt.Errorf("add subscriber %s: %w", channelID, err)
		}
	}

	return nil
}

// FindSubscribersForServices finds all enabled and verified channels subscribed to any of the given services.
func (r *Repository) FindSubscribersForServices(ctx context.Context, serviceIDs []string) ([]notifications.ChannelInfo, error) {
	if len(serviceIDs) == 0 {
		return make([]notifications.ChannelInfo, 0), nil
	}

	query := `
		SELECT DISTINCT nc.id, nc.user_id, nc.type, nc.target, u.email
		FROM notification_channels nc
		JOIN users u ON u.id = nc.user_id
		LEFT JOIN channel_subscriptions cs ON cs.channel_id = nc.id
		WHERE nc.is_enabled = true AND nc.is_verified = true
		  AND (nc.subscribe_to_all_services = true OR cs.service_id = ANY($1::uuid[]))
	`

	rows, err := r.db.Query(ctx, query, serviceIDs)
	if err != nil {
		return nil, fmt.Errorf("find subscribers: %w", err)
	}
	defer rows.Close()

	channels := make([]notifications.ChannelInfo, 0)
	for rows.Next() {
		var info notifications.ChannelInfo
		if err := rows.Scan(&info.ID, &info.UserID, &info.Type, &info.Target, &info.Email); err != nil {
			return nil, fmt.Errorf("scan channel info: %w", err)
		}
		channels = append(channels, info)
	}

	return channels, nil
}

// GetChannelsByIDs returns channels by their IDs.
func (r *Repository) GetChannelsByIDs(ctx context.Context, ids []string) ([]notifications.ChannelInfo, error) {
	if len(ids) == 0 {
		return make([]notifications.ChannelInfo, 0), nil
	}

	query := `
		SELECT nc.id, nc.user_id, nc.type, nc.target, u.email
		FROM notification_channels nc
		JOIN users u ON u.id = nc.user_id
		WHERE nc.id = ANY($1::uuid[])
		  AND nc.is_enabled = true
		  AND nc.is_verified = true
	`

	rows, err := r.db.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("get channels by ids: %w", err)
	}
	defer rows.Close()

	channels := make([]notifications.ChannelInfo, 0, len(ids))
	for rows.Next() {
		var info notifications.ChannelInfo
		if err := rows.Scan(&info.ID, &info.UserID, &info.Type, &info.Target, &info.Email); err != nil {
			return nil, fmt.Errorf("scan channel info: %w", err)
		}
		channels = append(channels, info)
	}

	return channels, nil
}

// CreateVerificationCode creates a new verification code, replacing any existing one.
func (r *Repository) CreateVerificationCode(ctx context.Context, channelID, code string, expiresAt time.Time) error {
	// Delete any existing code first
	_, err := r.db.Exec(ctx, `DELETE FROM channel_verification_codes WHERE channel_id = $1`, channelID)
	if err != nil {
		return fmt.Errorf("delete old code: %w", err)
	}

	// Create new code
	_, err = r.db.Exec(ctx, `
		INSERT INTO channel_verification_codes (channel_id, code, expires_at)
		VALUES ($1, $2, $3)
	`, channelID, code, expiresAt)
	if err != nil {
		return fmt.Errorf("create code: %w", err)
	}

	return nil
}

// GetVerificationCode retrieves an active (non-expired) verification code for a channel.
func (r *Repository) GetVerificationCode(ctx context.Context, channelID string) (*notifications.VerificationCode, error) {
	var code notifications.VerificationCode
	err := r.db.QueryRow(ctx, `
		SELECT id, channel_id, code, expires_at, attempts, created_at
		FROM channel_verification_codes
		WHERE channel_id = $1 AND expires_at > NOW()
	`, channelID).Scan(&code.ID, &code.ChannelID, &code.Code, &code.ExpiresAt, &code.Attempts, &code.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notifications.ErrVerificationCodeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get code: %w", err)
	}
	return &code, nil
}

// IncrementCodeAttempts increments the attempt counter for a verification code.
func (r *Repository) IncrementCodeAttempts(ctx context.Context, channelID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE channel_verification_codes
		SET attempts = attempts + 1
		WHERE channel_id = $1 AND expires_at > NOW()
	`, channelID)
	if err != nil {
		return fmt.Errorf("increment attempts: %w", err)
	}
	return nil
}

// DeleteVerificationCode deletes a verification code for a channel.
func (r *Repository) DeleteVerificationCode(ctx context.Context, channelID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM channel_verification_codes WHERE channel_id = $1`, channelID)
	if err != nil {
		return fmt.Errorf("delete code: %w", err)
	}
	return nil
}

// DeleteExpiredCodes removes all expired verification codes and returns the count.
func (r *Repository) DeleteExpiredCodes(ctx context.Context) (int64, error) {
	result, err := r.db.Exec(ctx, `DELETE FROM channel_verification_codes WHERE expires_at <= NOW()`)
	if err != nil {
		return 0, fmt.Errorf("delete expired codes: %w", err)
	}
	return result.RowsAffected(), nil
}

// EnqueueNotification adds a notification to the queue.
func (r *Repository) EnqueueNotification(ctx context.Context, item *notifications.QueueItem) error {
	payloadJSON, err := json.Marshal(item.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO notification_queue
			(id, event_id, channel_id, message_type, payload, status, max_attempts, next_attempt_at)
		VALUES
			($1, $2, $3, $4, $5, 'pending', $6, NOW())
	`, item.ID, item.EventID, item.ChannelID, item.MessageType, payloadJSON, item.MaxAttempts)

	if err != nil {
		return fmt.Errorf("enqueue notification: %w", err)
	}
	return nil
}

// EnqueueBatch adds multiple notifications to the queue.
func (r *Repository) EnqueueBatch(ctx context.Context, items []*notifications.QueueItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, item := range items {
		payloadJSON, err := json.Marshal(item.Payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO notification_queue
				(id, event_id, channel_id, message_type, payload, status, max_attempts, next_attempt_at)
			VALUES
				($1, $2, $3, $4, $5, 'pending', $6, NOW())
		`, item.ID, item.EventID, item.ChannelID, item.MessageType, payloadJSON, item.MaxAttempts)

		if err != nil {
			return fmt.Errorf("enqueue notification %s: %w", item.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// FetchPendingNotifications retrieves pending notifications ready for processing.
// Uses SELECT FOR UPDATE SKIP LOCKED for concurrent processing.
// Returned items have Status set to QueueStatusProcessing.
func (r *Repository) FetchPendingNotifications(ctx context.Context, limit int) ([]*notifications.QueueItem, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	rows, err := tx.Query(ctx, `
		SELECT id, event_id, channel_id, message_type, payload,
			   status, attempts, max_attempts, next_attempt_at, last_error,
			   created_at, updated_at, sent_at
		FROM notification_queue
		WHERE status = 'pending'
		  AND next_attempt_at <= NOW()
		ORDER BY next_attempt_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch pending: %w", err)
	}
	defer rows.Close()

	items := make([]*notifications.QueueItem, 0)
	ids := make([]string, 0)

	for rows.Next() {
		item, err := scanQueueItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		ids = append(ids, item.ID)
	}

	// Mark items as processing
	if len(ids) > 0 {
		_, err = tx.Exec(ctx, `
			UPDATE notification_queue
			SET status = 'processing', updated_at = NOW()
			WHERE id = ANY($1::uuid[])
		`, ids)
		if err != nil {
			return nil, fmt.Errorf("mark as processing: %w", err)
		}

		// Update returned items to reflect actual status
		for _, item := range items {
			item.Status = notifications.QueueStatusProcessing
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return items, nil
}

// scanQueueItem scans a queue item from a row.
func scanQueueItem(rows pgx.Rows) (*notifications.QueueItem, error) {
	item := &notifications.QueueItem{}
	var payloadJSON []byte
	var lastError *string
	var sentAt *time.Time

	err := rows.Scan(
		&item.ID, &item.EventID, &item.ChannelID, &item.MessageType, &payloadJSON,
		&item.Status, &item.Attempts, &item.MaxAttempts, &item.NextAttemptAt, &lastError,
		&item.CreatedAt, &item.UpdatedAt, &sentAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	if err := json.Unmarshal(payloadJSON, &item.Payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	if lastError != nil {
		item.LastError = *lastError
	}
	item.SentAt = sentAt

	return item, nil
}

// MarkAsSent marks a notification as successfully sent.
func (r *Repository) MarkAsSent(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notification_queue
		SET status = 'sent',
			attempts = attempts + 1,
			sent_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark as sent: %w", err)
	}
	return nil
}

// MarkAsFailed marks a notification as permanently failed.
func (r *Repository) MarkAsFailed(ctx context.Context, id string, failErr error) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notification_queue
		SET status = 'failed',
			attempts = attempts + 1,
			last_error = $2,
			updated_at = NOW()
		WHERE id = $1
	`, id, failErr.Error())
	if err != nil {
		return fmt.Errorf("mark as failed: %w", err)
	}
	return nil
}

// MarkForRetry schedules a notification for retry.
func (r *Repository) MarkForRetry(ctx context.Context, id string, retryErr error, nextAttempt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notification_queue
		SET status = 'pending',
			attempts = attempts + 1,
			next_attempt_at = $2,
			last_error = $3,
			updated_at = NOW()
		WHERE id = $1
	`, id, nextAttempt, retryErr.Error())
	if err != nil {
		return fmt.Errorf("mark for retry: %w", err)
	}
	return nil
}

// MarkAsProcessing marks a notification as being processed.
func (r *Repository) MarkAsProcessing(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notification_queue
		SET status = 'processing',
			updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark as processing: %w", err)
	}
	return nil
}

// GetFailedItems returns failed notifications for potential manual retry.
func (r *Repository) GetFailedItems(ctx context.Context, limit int) ([]*notifications.QueueItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, event_id, channel_id, message_type, payload,
			   status, attempts, max_attempts, next_attempt_at,
			   last_error, created_at, updated_at, sent_at
		FROM notification_queue
		WHERE status = 'failed'
		ORDER BY updated_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("get failed items: %w", err)
	}
	defer rows.Close()

	items := make([]*notifications.QueueItem, 0)
	for rows.Next() {
		item, err := scanQueueItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// RetryFailedItem resets a failed notification back to pending for retry.
func (r *Repository) RetryFailedItem(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE notification_queue
		SET status = 'pending',
			attempts = 0,
			next_attempt_at = NOW(),
			last_error = NULL,
			updated_at = NOW()
		WHERE id = $1 AND status = 'failed'
	`, id)
	if err != nil {
		return fmt.Errorf("retry failed item: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("item not found or not in failed status")
	}
	return nil
}

// RecoverStuckProcessing resets items stuck in processing status back to pending.
// This handles cases where a worker crashed while processing.
func (r *Repository) RecoverStuckProcessing(ctx context.Context, stuckFor time.Duration) (int64, error) {
	cutoff := time.Now().Add(-stuckFor)
	result, err := r.db.Exec(ctx, `
		UPDATE notification_queue
		SET status = 'pending',
			next_attempt_at = NOW(),
			updated_at = NOW()
		WHERE status = 'processing' AND updated_at < $1
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("recover stuck processing: %w", err)
	}
	return result.RowsAffected(), nil
}

// DeleteOldSentItems removes sent notifications older than the specified duration.
func (r *Repository) DeleteOldSentItems(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.Exec(ctx, `
		DELETE FROM notification_queue
		WHERE status = 'sent' AND sent_at < $1
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old sent items: %w", err)
	}
	return result.RowsAffected(), nil
}

// GetQueueStats returns statistics about the notification queue.
func (r *Repository) GetQueueStats(ctx context.Context) (*notifications.QueueStats, error) {
	var stats notifications.QueueStats

	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'processing') as processing,
			COUNT(*) FILTER (WHERE status = 'sent') as sent,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM notification_queue
	`).Scan(&stats.Pending, &stats.Processing, &stats.Sent, &stats.Failed)

	if err != nil {
		return nil, fmt.Errorf("get queue stats: %w", err)
	}

	return &stats, nil
}
