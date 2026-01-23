package identity

import (
	"context"

	"github.com/bissquit/incident-garden/internal/domain"
)

// Repository defines the interface for user data access.
type Repository interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error

	SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error
	GetRefreshToken(ctx context.Context, token string) (*domain.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteUserRefreshTokens(ctx context.Context, userID string) error
}
