// Package postgres provides PostgreSQL implementation of notifications repository.
package postgres

import (
	"context"
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
