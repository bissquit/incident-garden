// Package jwt provides JWT-based authentication implementation.
package jwt

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/identity"
	"github.com/golang-jwt/jwt/v5"
)

// JWT errors.
var (
	ErrInvalidToken = errors.New("invalid token")
	ErrTokenExpired = errors.New("token expired")
)

// Config holds JWT configuration.
type Config struct {
	SecretKey            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
}

// Claims represents JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	UserID string      `json:"user_id"`
	Role   domain.Role `json:"role"`
}

// TokenStore interface for storing refresh tokens.
type TokenStore interface {
	SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error
	GetRefreshToken(ctx context.Context, token string) (*domain.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteUserRefreshTokens(ctx context.Context, userID string) error
}

// Authenticator implements JWT-based authentication.
type Authenticator struct {
	config     Config
	tokenStore TokenStore
}

// NewAuthenticator creates a new JWT authenticator.
func NewAuthenticator(config Config, tokenStore TokenStore) *Authenticator {
	if config.AccessTokenDuration == 0 {
		config.AccessTokenDuration = 15 * time.Minute
	}
	if config.RefreshTokenDuration == 0 {
		config.RefreshTokenDuration = 7 * 24 * time.Hour
	}
	return &Authenticator{
		config:     config,
		tokenStore: tokenStore,
	}
}

// Type returns the authenticator type.
func (a *Authenticator) Type() string {
	return "jwt"
}

// GenerateTokens creates a new token pair.
func (a *Authenticator) GenerateTokens(ctx context.Context, user *domain.User) (*identity.TokenPair, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.AccessTokenDuration)),
		},
		UserID: user.ID,
		Role:   user.Role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(a.config.SecretKey))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshTokenBytes := make([]byte, 32)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshToken := base64.URLEncoding.EncodeToString(refreshTokenBytes)

	rt := &domain.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: now.Add(a.config.RefreshTokenDuration),
		CreatedAt: now,
	}
	if err := a.tokenStore.SaveRefreshToken(ctx, rt); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &identity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(a.config.AccessTokenDuration.Seconds()),
	}, nil
}

// ValidateAccessToken validates the access token.
func (a *Authenticator) ValidateAccessToken(_ context.Context, tokenString string) (string, domain.Role, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.SecretKey), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", "", ErrTokenExpired
		}
		return "", "", ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", "", ErrInvalidToken
	}

	return claims.UserID, claims.Role, nil
}

// RefreshTokens generates new tokens using refresh token.
func (a *Authenticator) RefreshTokens(_ context.Context, _ string) (*identity.TokenPair, error) {
	return nil, errors.New("not implemented - use Service.RefreshTokens")
}

// RevokeRefreshToken invalidates the refresh token.
func (a *Authenticator) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	return a.tokenStore.DeleteRefreshToken(ctx, refreshToken)
}
