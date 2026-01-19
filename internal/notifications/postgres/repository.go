// Package postgres provides PostgreSQL implementation of notifications repository.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bissquit/incident-management/internal/domain"
	"github.com/bissquit/incident-management/internal/notifications"
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
		INSERT INTO notification_channels (user_id, type, target, is_enabled, is_verified)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		channel.UserID,
		channel.Type,
		channel.Target,
		channel.IsEnabled,
		channel.IsVerified,
	).Scan(&channel.ID, &channel.CreatedAt, &channel.UpdatedAt)
}

// GetChannelByID retrieves a notification channel by ID.
func (r *Repository) GetChannelByID(ctx context.Context, id string) (*domain.NotificationChannel, error) {
	query := `
		SELECT id, user_id, type, target, is_enabled, is_verified, created_at, updated_at
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
		SELECT id, user_id, type, target, is_enabled, is_verified, created_at, updated_at
		FROM notification_channels
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list user channels: %w", err)
	}
	defer rows.Close()

	var channels []domain.NotificationChannel
	for rows.Next() {
		var channel domain.NotificationChannel
		err := rows.Scan(
			&channel.ID,
			&channel.UserID,
			&channel.Type,
			&channel.Target,
			&channel.IsEnabled,
			&channel.IsVerified,
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
		SET is_enabled = $2, is_verified = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		channel.ID,
		channel.IsEnabled,
		channel.IsVerified,
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

// CreateSubscription creates a new subscription.
func (r *Repository) CreateSubscription(ctx context.Context, subscription *domain.Subscription) error {
	query := `
		INSERT INTO subscriptions (user_id)
		VALUES ($1)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query, subscription.UserID).Scan(&subscription.ID, &subscription.CreatedAt)
}

// GetSubscriptionByID retrieves a subscription by ID.
func (r *Repository) GetSubscriptionByID(ctx context.Context, id string) (*domain.Subscription, error) {
	query := `SELECT id, user_id, created_at FROM subscriptions WHERE id = $1`
	var sub domain.Subscription
	err := r.db.QueryRow(ctx, query, id).Scan(&sub.ID, &sub.UserID, &sub.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notifications.ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("get subscription: %w", err)
	}

	serviceIDs, err := r.getSubscriptionServices(ctx, sub.ID)
	if err != nil {
		return nil, err
	}
	sub.ServiceIDs = serviceIDs

	return &sub, nil
}

// GetUserSubscription retrieves a subscription by user ID.
func (r *Repository) GetUserSubscription(ctx context.Context, userID string) (*domain.Subscription, error) {
	query := `SELECT id, user_id, created_at FROM subscriptions WHERE user_id = $1`
	var sub domain.Subscription
	err := r.db.QueryRow(ctx, query, userID).Scan(&sub.ID, &sub.UserID, &sub.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notifications.ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("get user subscription: %w", err)
	}

	serviceIDs, err := r.getSubscriptionServices(ctx, sub.ID)
	if err != nil {
		return nil, err
	}
	sub.ServiceIDs = serviceIDs

	return &sub, nil
}

// getSubscriptionServices retrieves service IDs for a subscription.
func (r *Repository) getSubscriptionServices(ctx context.Context, subscriptionID string) ([]string, error) {
	query := `SELECT service_id FROM subscription_services WHERE subscription_id = $1`
	rows, err := r.db.Query(ctx, query, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("get subscription services: %w", err)
	}
	defer rows.Close()

	var serviceIDs []string
	for rows.Next() {
		var serviceID string
		if err := rows.Scan(&serviceID); err != nil {
			return nil, fmt.Errorf("scan service id: %w", err)
		}
		serviceIDs = append(serviceIDs, serviceID)
	}

	return serviceIDs, nil
}

// SetSubscriptionServices replaces subscription services.
func (r *Repository) SetSubscriptionServices(ctx context.Context, subscriptionID string, serviceIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	deleteQuery := `DELETE FROM subscription_services WHERE subscription_id = $1`
	if _, err := tx.Exec(ctx, deleteQuery, subscriptionID); err != nil {
		return fmt.Errorf("delete old services: %w", err)
	}

	if len(serviceIDs) > 0 {
		insertQuery := `INSERT INTO subscription_services (subscription_id, service_id) VALUES ($1, $2)`
		for _, serviceID := range serviceIDs {
			if _, err := tx.Exec(ctx, insertQuery, subscriptionID, serviceID); err != nil {
				return fmt.Errorf("insert service %s: %w", serviceID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// DeleteSubscription deletes a subscription.
func (r *Repository) DeleteSubscription(ctx context.Context, id string) error {
	query := `DELETE FROM subscriptions WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}

	if result.RowsAffected() == 0 {
		return notifications.ErrSubscriptionNotFound
	}
	return nil
}

// GetSubscribersForServices retrieves subscribers for given services.
func (r *Repository) GetSubscribersForServices(ctx context.Context, serviceIDs []string) ([]notifications.SubscriberInfo, error) {
	query := `
		SELECT DISTINCT u.id, u.email
		FROM users u
		JOIN subscriptions s ON s.user_id = u.id
		LEFT JOIN subscription_services ss ON ss.subscription_id = s.id
		WHERE 
			NOT EXISTS (SELECT 1 FROM subscription_services WHERE subscription_id = s.id)
			OR ss.service_id = ANY($1::uuid[])
	`

	rows, err := r.db.Query(ctx, query, serviceIDs)
	if err != nil {
		return nil, fmt.Errorf("get subscribers: %w", err)
	}
	defer rows.Close()

	var subscribers []notifications.SubscriberInfo
	for rows.Next() {
		var sub notifications.SubscriberInfo
		if err := rows.Scan(&sub.UserID, &sub.Email); err != nil {
			return nil, fmt.Errorf("scan subscriber: %w", err)
		}

		channels, err := r.getEnabledVerifiedChannels(ctx, sub.UserID)
		if err != nil {
			return nil, fmt.Errorf("get channels for user %s: %w", sub.UserID, err)
		}

		if len(channels) > 0 {
			sub.Channels = channels
			subscribers = append(subscribers, sub)
		}
	}

	return subscribers, nil
}

// getEnabledVerifiedChannels retrieves enabled and verified channels for a user.
func (r *Repository) getEnabledVerifiedChannels(ctx context.Context, userID string) ([]domain.NotificationChannel, error) {
	query := `
		SELECT id, user_id, type, target, is_enabled, is_verified, created_at, updated_at
		FROM notification_channels
		WHERE user_id = $1 AND is_enabled = true AND is_verified = true
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query channels: %w", err)
	}
	defer rows.Close()

	var channels []domain.NotificationChannel
	for rows.Next() {
		var channel domain.NotificationChannel
		err := rows.Scan(
			&channel.ID,
			&channel.UserID,
			&channel.Type,
			&channel.Target,
			&channel.IsEnabled,
			&channel.IsVerified,
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
