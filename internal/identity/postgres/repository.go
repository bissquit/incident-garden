// Package postgres provides PostgreSQL implementation of the identity repository.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements identity.Repository using PostgreSQL.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateUser creates a new user.
func (r *Repository) CreateUser(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (email, password_hash, first_name, last_name, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.Role,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID.
func (r *Repository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var user domain.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, identity.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, role, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var user domain.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, identity.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &user, nil
}

// UpdateUser updates an existing user.
func (r *Repository) UpdateUser(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET email = $2, first_name = $3, last_name = $4, role = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		user.ID,
		user.Email,
		user.FirstName,
		user.LastName,
		user.Role,
	).Scan(&user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.ErrUserNotFound
		}
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// SaveRefreshToken saves a refresh token to the database.
func (r *Repository) SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query,
		token.UserID,
		token.Token,
		token.ExpiresAt,
		token.CreatedAt,
	).Scan(&token.ID)

	if err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

// GetRefreshToken retrieves a refresh token from the database.
func (r *Repository) GetRefreshToken(ctx context.Context, token string) (*domain.RefreshToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM refresh_tokens
		WHERE token = $1 AND expires_at > NOW()
	`
	var rt domain.RefreshToken
	err := r.db.QueryRow(ctx, query, token).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.Token,
		&rt.ExpiresAt,
		&rt.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, identity.ErrInvalidToken
		}
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &rt, nil
}

// DeleteRefreshToken deletes a refresh token from the database.
func (r *Repository) DeleteRefreshToken(ctx context.Context, token string) error {
	query := `DELETE FROM refresh_tokens WHERE token = $1`
	_, err := r.db.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

// DeleteUserRefreshTokens deletes all refresh tokens for a user.
func (r *Repository) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`
	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete user refresh tokens: %w", err)
	}
	return nil
}
