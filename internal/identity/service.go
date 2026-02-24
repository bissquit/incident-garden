package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

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
	ErrEmailNotConfigured = errors.New("email not configured")
	ErrCannotModifySelf   = errors.New("cannot modify your own account")
	ErrInvalidRole        = errors.New("invalid role")
)

// UserCreatedHandler handles user creation events.
// Used to create default notification channels, send welcome emails, etc.
type UserCreatedHandler interface {
	OnUserCreated(ctx context.Context, user *domain.User) error
}

// EmailSender sends emails directly (not through notification queue).
// Used for password reset emails that must arrive immediately.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// Service provides identity business logic.
type Service struct {
	repo               Repository
	authenticator      Authenticator
	userCreatedHandler UserCreatedHandler
	emailSender        EmailSender // optional, nil if email not configured
	frontendURL        string      // for constructing reset links
}

// NewService creates a new identity service.
// userCreatedHandler and emailSender are optional and can be nil.
func NewService(repo Repository, authenticator Authenticator, userCreatedHandler UserCreatedHandler, emailSender EmailSender, frontendURL string) *Service {
	return &Service{
		repo:               repo,
		authenticator:      authenticator,
		userCreatedHandler: userCreatedHandler,
		emailSender:        emailSender,
		frontendURL:        frontendURL,
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

// ForgotPassword initiates password reset flow.
// Always returns nil even if user not found (prevents email enumeration).
// Returns ErrEmailNotConfigured if email sender is not available.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	if s.emailSender == nil {
		return ErrEmailNotConfigured
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil // silent — don't reveal if email exists
		}
		return fmt.Errorf("get user by email: %w", err)
	}

	// Rate limit: skip if token was created less than 5 minutes ago
	latest, err := s.repo.GetLatestPasswordResetToken(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("check existing token: %w", err)
	}
	if latest != nil && time.Since(latest.CreatedAt) < 5*time.Minute {
		return nil // silently skip
	}

	// Generate token: 32 random bytes, hex-encoded = 64 chars
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	tokenStr := hex.EncodeToString(tokenBytes)

	resetToken := &domain.PasswordResetToken{
		UserID:    user.ID,
		Token:     tokenStr,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	if err := s.repo.SavePasswordResetToken(ctx, resetToken); err != nil {
		return fmt.Errorf("save reset token: %w", err)
	}

	// Build reset link and send email
	resetLink := s.frontendURL + "/reset-password?token=" + tokenStr
	subject := "Password Reset Request"
	body := fmt.Sprintf("You requested a password reset.\n\nClick the link below to reset your password:\n%s\n\nThis link expires in 1 hour.\n\nIf you did not request this, please ignore this email.", resetLink)

	if err := s.emailSender.SendEmail(ctx, user.Email, subject, body); err != nil {
		slog.Warn("failed to send password reset email",
			"user_id", user.ID,
			"error", err,
		)
		// Don't fail — token is saved, user can retry
	}

	return nil
}

// ResetPasswordInput contains data for password reset.
type ResetPasswordInput struct {
	Token       string
	NewPassword string
}

// ResetPassword resets password using a valid reset token.
func (s *Service) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	token, err := s.repo.GetPasswordResetToken(ctx, input.Token)
	if err != nil {
		return ErrInvalidResetToken
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.UpdateUserPassword(ctx, token.UserID, string(hashedPassword)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	// Clear must_change_password — user proved email ownership
	user, err := s.repo.GetUserByID(ctx, token.UserID)
	if err == nil && user.MustChangePassword {
		user.MustChangePassword = false
		if err := s.repo.UpdateUser(ctx, user); err != nil {
			slog.Warn("failed to clear must_change_password after reset",
				"user_id", token.UserID,
				"error", err,
			)
		}
	}

	// Clean up: delete used token and all user's reset tokens
	if err := s.repo.DeleteUserPasswordResetTokens(ctx, token.UserID); err != nil {
		slog.Warn("failed to delete password reset tokens",
			"user_id", token.UserID,
			"error", err,
		)
	}

	// Invalidate all refresh tokens
	if err := s.repo.DeleteUserRefreshTokens(ctx, token.UserID); err != nil {
		slog.Warn("failed to invalidate refresh tokens after password reset",
			"user_id", token.UserID,
			"error", err,
		)
	}

	return nil
}

// ListUsers returns a paginated list of users.
func (s *Service) ListUsers(ctx context.Context, filter UserFilter) ([]*domain.User, int, error) {
	return s.repo.ListUsers(ctx, filter)
}

// AdminCreateUserInput contains data for admin user creation.
type AdminCreateUserInput struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
	Role      domain.Role
}

// AdminCreateUser creates a new user with specified role (admin only).
// Always sets must_change_password=true.
func (s *Service) AdminCreateUser(ctx context.Context, input AdminCreateUserInput) (*domain.User, error) {
	if !input.Role.IsValid() {
		return nil, ErrInvalidRole
	}

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
		Role:               input.Role,
		IsActive:           true,
		MustChangePassword: true,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	// Create default notification channel
	if s.userCreatedHandler != nil {
		if err := s.userCreatedHandler.OnUserCreated(ctx, user); err != nil {
			slog.Warn("failed to create default notification channel for admin-created user",
				"user_id", user.ID,
				"email", user.Email,
				"error", err,
			)
		}
	}

	return user, nil
}

// AdminUpdateUserInput contains data for admin user update.
type AdminUpdateUserInput struct {
	UserID    string
	Role      *domain.Role
	IsActive  *bool
	FirstName *string
	LastName  *string
}

// AdminUpdateUser updates a user's role, active status, or profile (admin only).
// Cannot modify the caller's own account.
func (s *Service) AdminUpdateUser(ctx context.Context, adminUserID string, input AdminUpdateUserInput) (*domain.User, error) {
	if adminUserID == input.UserID {
		return nil, ErrCannotModifySelf
	}

	user, err := s.repo.GetUserByID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	if input.Role != nil {
		if !input.Role.IsValid() {
			return nil, ErrInvalidRole
		}
		user.Role = *input.Role
	}
	if input.FirstName != nil {
		user.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		user.LastName = *input.LastName
	}

	wasActive := user.IsActive
	if input.IsActive != nil {
		user.IsActive = *input.IsActive
	}

	if input.Role == nil && input.IsActive == nil && input.FirstName == nil && input.LastName == nil {
		return user, nil
	}

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	// On deactivation: invalidate all refresh tokens
	if wasActive && input.IsActive != nil && !*input.IsActive {
		if err := s.repo.DeleteUserRefreshTokens(ctx, user.ID); err != nil {
			slog.Warn("failed to invalidate refresh tokens after deactivation",
				"user_id", user.ID,
				"error", err,
			)
		}
	}

	return user, nil
}

// AdminResetPasswordInput contains data for admin password reset.
type AdminResetPasswordInput struct {
	UserID      string
	NewPassword string
}

// AdminResetPassword sets a new password for a user (admin only).
// Sets must_change_password=true and invalidates all refresh tokens.
func (s *Service) AdminResetPassword(ctx context.Context, adminUserID string, input AdminResetPasswordInput) error {
	if adminUserID == input.UserID {
		return ErrCannotModifySelf
	}

	user, err := s.repo.GetUserByID(ctx, input.UserID)
	if err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.UpdateUserPassword(ctx, user.ID, string(hashedPassword)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	user.MustChangePassword = true
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("set must_change_password: %w", err)
	}

	if err := s.repo.DeleteUserRefreshTokens(ctx, user.ID); err != nil {
		slog.Warn("failed to invalidate refresh tokens after admin password reset",
			"user_id", user.ID,
			"error", err,
		)
	}

	return nil
}
