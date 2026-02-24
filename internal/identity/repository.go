package identity

import (
	"context"

	"github.com/bissquit/incident-garden/internal/domain"
)

// UserFilter defines filtering and pagination for user listing.
type UserFilter struct {
	Role   *domain.Role
	Limit  int
	Offset int
}

// Repository defines the interface for user data access.
type Repository interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error

	// User management
	ListUsers(ctx context.Context, filter UserFilter) ([]*domain.User, int, error)
	UpdateUserPassword(ctx context.Context, userID string, passwordHash string) error

	// Password reset tokens
	SavePasswordResetToken(ctx context.Context, token *domain.PasswordResetToken) error
	GetPasswordResetToken(ctx context.Context, token string) (*domain.PasswordResetToken, error)
	GetLatestPasswordResetToken(ctx context.Context, userID string) (*domain.PasswordResetToken, error)
	DeletePasswordResetToken(ctx context.Context, token string) error
	DeleteUserPasswordResetTokens(ctx context.Context, userID string) error

	SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error
	GetRefreshToken(ctx context.Context, token string) (*domain.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteUserRefreshTokens(ctx context.Context, userID string) error
}
