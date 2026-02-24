package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bissquit/incident-garden/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// Service errors.
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidResetToken  = errors.New("invalid or expired reset token")
	ErrAccountDeactivated = errors.New("account deactivated")
	ErrWrongPassword      = errors.New("wrong current password")
)

// UserCreatedHandler handles user creation events.
// Used to create default notification channels, send welcome emails, etc.
type UserCreatedHandler interface {
	OnUserCreated(ctx context.Context, user *domain.User) error
}

// Service provides identity business logic.
type Service struct {
	repo               Repository
	authenticator      Authenticator
	userCreatedHandler UserCreatedHandler
}

// NewService creates a new identity service.
// userCreatedHandler is optional and can be nil.
func NewService(repo Repository, authenticator Authenticator, userCreatedHandler UserCreatedHandler) *Service {
	return &Service{
		repo:               repo,
		authenticator:      authenticator,
		userCreatedHandler: userCreatedHandler,
	}
}

// RegisterInput contains data for user registration.
type RegisterInput struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*domain.User, error) {
	existing, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Email:              input.Email,
		PasswordHash:       string(hashedPassword),
		FirstName:          input.FirstName,
		LastName:           input.LastName,
		Role:               domain.RoleUser,
		IsActive:           true,
		MustChangePassword: false,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	// Create default notification channel
	if s.userCreatedHandler != nil {
		if err := s.userCreatedHandler.OnUserCreated(ctx, user); err != nil {
			slog.Warn("failed to create default notification channel",
				"user_id", user.ID,
				"email", user.Email,
				"error", err,
			)
			// Don't fail registration — user can create channel manually
		}
	}

	return user, nil
}

// LoginInput contains credentials for login.
type LoginInput struct {
	Email    string
	Password string
}

// Login authenticates user and returns tokens.
func (s *Service) Login(ctx context.Context, input LoginInput) (*domain.User, *TokenPair, error) {
	user, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, nil, ErrAccountDeactivated
	}

	tokens, err := s.authenticator.GenerateTokens(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// RefreshTokens generates new tokens using refresh token.
func (s *Service) RefreshTokens(ctx context.Context, refreshToken string) (*TokenPair, error) {
	storedToken, err := s.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	user, err := s.repo.GetUserByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		_ = s.repo.DeleteRefreshToken(ctx, refreshToken)
		return nil, ErrAccountDeactivated
	}

	if err := s.repo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return nil, err
	}

	return s.authenticator.GenerateTokens(ctx, user)
}

// Logout invalidates the refresh token.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.DeleteRefreshToken(ctx, refreshToken)
}

// GetUserByID returns user by ID.
func (s *Service) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetUserByID(ctx, id)
}

// ChangePasswordInput contains data for password change.
type ChangePasswordInput struct {
	UserID          string
	CurrentPassword string
	NewPassword     string
}

// ChangePassword changes the authenticated user's password.
func (s *Service) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	user, err := s.repo.GetUserByID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.CurrentPassword)); err != nil {
		return ErrWrongPassword
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.UpdateUserPassword(ctx, user.ID, string(hashedPassword)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	// Clear must_change_password flag if it was set
	if user.MustChangePassword {
		user.MustChangePassword = false
		if err := s.repo.UpdateUser(ctx, user); err != nil {
			slog.Warn("failed to clear must_change_password flag",
				"user_id", user.ID,
				"error", err,
			)
		}
	}

	// Invalidate all refresh tokens to force re-login
	if err := s.repo.DeleteUserRefreshTokens(ctx, user.ID); err != nil {
		slog.Warn("failed to invalidate refresh tokens after password change",
			"user_id", user.ID,
			"error", err,
		)
	}

	return nil
}

// UpdateProfileInput contains data for profile update.
type UpdateProfileInput struct {
	UserID    string
	FirstName *string
	LastName  *string
}

// UpdateProfile updates the authenticated user's profile.
func (s *Service) UpdateProfile(ctx context.Context, input UpdateProfileInput) (*domain.User, error) {
	user, err := s.repo.GetUserByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	if input.FirstName == nil && input.LastName == nil {
		return user, nil
	}

	if input.FirstName != nil {
		user.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		user.LastName = *input.LastName
	}

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	return user, nil
}

// ValidateToken validates access token and returns user info.
func (s *Service) ValidateToken(ctx context.Context, token string) (string, domain.Role, error) {
	return s.authenticator.ValidateAccessToken(ctx, token)
}
