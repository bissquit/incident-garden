// Package postgres provides PostgreSQL implementation of the identity repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		INSERT INTO users (email, password_hash, first_name, last_name, role, is_active, must_change_password)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.Role,
		user.IsActive,
		user.MustChangePassword,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID.
func (r *Repository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, role, is_active, must_change_password, created_at, updated_at
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
		&user.IsActive,
		&user.MustChangePassword,
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
		SELECT id, email, password_hash, first_name, last_name, role, is_active, must_change_password, created_at, updated_at
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
		&user.IsActive,
		&user.MustChangePassword,
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
		SET email = $2, first_name = $3, last_name = $4, role = $5, is_active = $6, must_change_password = $7, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		user.ID,
		user.Email,
		user.FirstName,
		user.LastName,
		user.Role,
		user.IsActive,
		user.MustChangePassword,
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

// ListUsers returns a paginated list of users with optional role filter.
func (r *Repository) ListUsers(ctx context.Context, filter identity.UserFilter) ([]*domain.User, int, error) {
	query := `
		SELECT id, email, first_name, last_name, role, is_active, must_change_password,
		       created_at, updated_at, COUNT(*) OVER() AS total
		FROM users
	`
	args := make([]interface{}, 0)
	argIdx := 1

	var conditions []string
	if filter.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, string(*filter.Role))
		argIdx++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	var total int

	for rows.Next() {
		var user domain.User
		if err := rows.Scan(
			&user.ID, &user.Email,
			&user.FirstName, &user.LastName, &user.Role,
			&user.IsActive, &user.MustChangePassword,
			&user.CreatedAt, &user.UpdatedAt,
			&total,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}

	return users, total, nil
}

// UpdateUserPassword updates a user's password hash.
func (r *Repository) UpdateUserPassword(ctx context.Context, userID string, passwordHash string) error {
	query := `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`
	ct, err := r.db.Exec(ctx, query, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return identity.ErrUserNotFound
	}
	return nil
}

// SavePasswordResetToken saves a password reset token to the database.
func (r *Repository) SavePasswordResetToken(ctx context.Context, token *domain.PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query, token.UserID, token.Token, token.ExpiresAt).
		Scan(&token.ID, &token.CreatedAt)
	if err != nil {
		return fmt.Errorf("save password reset token: %w", err)
	}
	return nil
}

// GetPasswordResetToken retrieves a valid (non-expired) password reset token.
func (r *Repository) GetPasswordResetToken(ctx context.Context, token string) (*domain.PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM password_reset_tokens
		WHERE token = $1 AND expires_at > NOW()
	`
	var t domain.PasswordResetToken
	err := r.db.QueryRow(ctx, query, token).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, identity.ErrInvalidResetToken
		}
		return nil, fmt.Errorf("get password reset token: %w", err)
	}
	return &t, nil
}

// GetLatestPasswordResetToken retrieves the most recent password reset token for a user.
func (r *Repository) GetLatestPasswordResetToken(ctx context.Context, userID string) (*domain.PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM password_reset_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var t domain.PasswordResetToken
	err := r.db.QueryRow(ctx, query, userID).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest password reset token: %w", err)
	}
	return &t, nil
}

// DeletePasswordResetToken deletes a password reset token.
func (r *Repository) DeletePasswordResetToken(ctx context.Context, token string) error {
	query := `DELETE FROM password_reset_tokens WHERE token = $1`
	_, err := r.db.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("delete password reset token: %w", err)
	}
	return nil
}

// DeleteUserPasswordResetTokens deletes all password reset tokens for a user.
func (r *Repository) DeleteUserPasswordResetTokens(ctx context.Context, userID string) error {
	query := `DELETE FROM password_reset_tokens WHERE user_id = $1`
	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete user password reset tokens: %w", err)
	}
	return nil
}
