// Package postgres provides PostgreSQL database connection utilities.
package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config contains PostgreSQL connection configuration.
type Config struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnectAttempts int
}

// Connect establishes a connection pool to PostgreSQL with retry logic.
func Connect(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime

	attempts := cfg.ConnectAttempts
	if attempts <= 0 {
		attempts = 1
	}

	var pool *pgxpool.Pool
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			lastErr = err
			if attempt < attempts {
				backoff := calcBackoff(attempt)
				slog.Warn("failed to create connection pool, retrying",
					"attempt", attempt,
					"max_attempts", attempts,
					"backoff", backoff,
					"error", err,
				)
				if !sleep(ctx, backoff) {
					return nil, fmt.Errorf("connection cancelled: %w", ctx.Err())
				}
			}
			continue
		}

		if err = pool.Ping(ctx); err != nil {
			pool.Close()
			lastErr = err
			if attempt < attempts {
				backoff := calcBackoff(attempt)
				slog.Warn("failed to ping database, retrying",
					"attempt", attempt,
					"max_attempts", attempts,
					"backoff", backoff,
					"error", err,
				)
				if !sleep(ctx, backoff) {
					return nil, fmt.Errorf("connection cancelled: %w", ctx.Err())
				}
			}
			continue
		}

		slog.Info("connected to database", "attempts", attempt)
		return pool, nil
	}

	return nil, fmt.Errorf("connect to database after %d attempts: %w", attempts, lastErr)
}

// calcBackoff returns exponential backoff duration capped at 16 seconds.
func calcBackoff(attempt int) time.Duration {
	backoff := time.Duration(1<<(attempt-1)) * time.Second
	if backoff > 16*time.Second {
		backoff = 16 * time.Second
	}
	return backoff
}

// sleep waits for duration or context cancellation. Returns false if cancelled.
func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}
