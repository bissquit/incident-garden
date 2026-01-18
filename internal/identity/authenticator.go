// Package identity provides authentication and authorization functionality.
package identity

import (
	"context"

	"github.com/bissquit/incident-management/internal/domain"
)

// TokenPair contains access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// Authenticator defines the interface for authentication providers.
type Authenticator interface {
	GenerateTokens(ctx context.Context, user *domain.User) (*TokenPair, error)
	ValidateAccessToken(ctx context.Context, token string) (userID string, role domain.Role, err error)
	RefreshTokens(ctx context.Context, refreshToken string) (*TokenPair, error)
	RevokeRefreshToken(ctx context.Context, refreshToken string) error
	Type() string
}
