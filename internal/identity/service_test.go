package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// mockRepository implements Repository for testing.
type mockRepository struct {
	users          map[string]*domain.User
	createUserErr  error
	getUserByEmail func(email string) (*domain.User, error)

	updateUserPasswordCalled      bool
	updateUserCalled              bool
	deleteUserRefreshTokensCalled bool

	savePasswordResetTokenCalled        bool
	deleteUserPasswordResetTokensCalled bool
	latestPasswordResetToken            *domain.PasswordResetToken
	getPasswordResetTokenResult         *domain.PasswordResetToken

	listUsersCalled bool
	listUsersFilter UserFilter
	listUsersResult []*domain.User
	listUsersTotal  int
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		users: make(map[string]*domain.User),
	}
}

func (m *mockRepository) CreateUser(_ context.Context, user *domain.User) error {
	if m.createUserErr != nil {
		return m.createUserErr
	}
	user.ID = "test-user-id"
	m.users[user.Email] = user
	return nil
}

func (m *mockRepository) GetUserByID(_ context.Context, id string) (*domain.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (m *mockRepository) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	if m.getUserByEmail != nil {
		return m.getUserByEmail(email)
	}
	if u, ok := m.users[email]; ok {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (m *mockRepository) UpdateUser(_ context.Context, user *domain.User) error {
	m.updateUserCalled = true
	if existing, ok := m.users[user.Email]; ok {
		*existing = *user
	}
	return nil
}

func (m *mockRepository) SaveRefreshToken(_ context.Context, _ *domain.RefreshToken) error {
	return nil
}

func (m *mockRepository) GetRefreshToken(_ context.Context, _ string) (*domain.RefreshToken, error) {
	return nil, nil
}

func (m *mockRepository) DeleteRefreshToken(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepository) DeleteUserRefreshTokens(_ context.Context, _ string) error {
	m.deleteUserRefreshTokensCalled = true
	return nil
}

func (m *mockRepository) ListUsers(_ context.Context, filter UserFilter) ([]*domain.User, int, error) {
	m.listUsersCalled = true
	m.listUsersFilter = filter
	if m.listUsersResult != nil {
		return m.listUsersResult, m.listUsersTotal, nil
	}
	return make([]*domain.User, 0), 0, nil
}

func (m *mockRepository) UpdateUserPassword(_ context.Context, userID string, passwordHash string) error {
	m.updateUserPasswordCalled = true
	for _, u := range m.users {
		if u.ID == userID {
			u.PasswordHash = passwordHash
			return nil
		}
	}
	return ErrUserNotFound
}

func (m *mockRepository) SavePasswordResetToken(_ context.Context, _ *domain.PasswordResetToken) error {
	m.savePasswordResetTokenCalled = true
	return nil
}

func (m *mockRepository) GetPasswordResetToken(_ context.Context, _ string) (*domain.PasswordResetToken, error) {
	if m.getPasswordResetTokenResult != nil {
		return m.getPasswordResetTokenResult, nil
	}
	return nil, ErrInvalidResetToken
}

func (m *mockRepository) GetLatestPasswordResetToken(_ context.Context, _ string) (*domain.PasswordResetToken, error) {
	return m.latestPasswordResetToken, nil
}

func (m *mockRepository) DeletePasswordResetToken(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepository) DeleteUserPasswordResetTokens(_ context.Context, _ string) error {
	m.deleteUserPasswordResetTokensCalled = true
	return nil
}

// mockAuthenticator implements Authenticator for testing.
type mockAuthenticator struct{}

func (m *mockAuthenticator) GenerateTokens(_ context.Context, _ *domain.User) (*TokenPair, error) {
	return &TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil
}

func (m *mockAuthenticator) ValidateAccessToken(_ context.Context, _ string) (string, domain.Role, error) {
	return "", "", nil
}

func (m *mockAuthenticator) RefreshTokens(_ context.Context, _ string) (*TokenPair, error) {
	return &TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil
}

func (m *mockAuthenticator) RevokeRefreshToken(_ context.Context, _ string) error {
	return nil
}

func (m *mockAuthenticator) Type() string {
	return "mock"
}

// mockUserCreatedHandler implements UserCreatedHandler for testing.
type mockUserCreatedHandler struct {
	called       bool
	receivedUser *domain.User
	err          error
}

func (m *mockUserCreatedHandler) OnUserCreated(_ context.Context, user *domain.User) error {
	m.called = true
	m.receivedUser = user
	return m.err
}

// mockEmailSender implements EmailSender for testing.
type mockEmailSender struct {
	called  bool
	to      string
	subject string
	body    string
	err     error
}

func (m *mockEmailSender) SendEmail(_ context.Context, to, subject, body string) error {
	m.called = true
	m.to = to
	m.subject = subject
	m.body = body
	return m.err
}

func TestRegister_CallsUserCreatedHandler(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	auth := &mockAuthenticator{}
	handler := &mockUserCreatedHandler{}

	service := NewService(repo, auth, handler, nil, "")

	// Act
	user, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.True(t, handler.called, "handler should be called")
	assert.Equal(t, user.ID, handler.receivedUser.ID)
	assert.Equal(t, user.Email, handler.receivedUser.Email)
}

func TestRegister_ContinuesIfHandlerFails(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	auth := &mockAuthenticator{}
	handler := &mockUserCreatedHandler{err: errors.New("handler error")}

	service := NewService(repo, auth, handler, nil, "")

	// Act
	user, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	})

	// Assert — registration succeeds despite handler error
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.True(t, handler.called, "handler should still be called")
}

func TestRegister_WorksWithNilHandler(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	service := NewService(repo, auth, nil, nil, "") // nil handler

	// Act
	user, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	})

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	repo.users["existing@example.com"] = &domain.User{Email: "existing@example.com"}
	auth := &mockAuthenticator{}
	handler := &mockUserCreatedHandler{}

	service := NewService(repo, auth, handler, nil, "")

	// Act
	user, err := service.Register(context.Background(), RegisterInput{
		Email:    "existing@example.com",
		Password: "password123",
	})

	// Assert
	assert.Nil(t, user)
	assert.ErrorIs(t, err, ErrEmailExists)
	assert.False(t, handler.called, "handler should not be called for duplicate email")
}

func TestRegister_CreateUserFails(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	repo.createUserErr = errors.New("database error")
	auth := &mockAuthenticator{}
	handler := &mockUserCreatedHandler{}

	service := NewService(repo, auth, handler, nil, "")

	// Act
	user, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	})

	// Assert
	assert.Nil(t, user)
	assert.Error(t, err)
	assert.False(t, handler.called, "handler should not be called if user creation fails")
}

func TestLogin_DeactivatedUser(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	// Create user with valid password hash but inactive
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	repo.users["test@example.com"] = &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: string(hashedPassword),
		IsActive:     false,
	}

	service := NewService(repo, auth, nil, nil, "")

	_, _, err := service.Login(context.Background(), LoginInput{
		Email:    "test@example.com",
		Password: "password123",
	})

	assert.ErrorIs(t, err, ErrAccountDeactivated)
}

func TestChangePassword_Success(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("oldpassword"), bcrypt.MinCost)
	repo.users["test@example.com"] = &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: string(hashedPassword),
		IsActive:     true,
	}

	service := NewService(repo, auth, nil, nil, "")

	err := service.ChangePassword(context.Background(), ChangePasswordInput{
		UserID:          "user-1",
		CurrentPassword: "oldpassword",
		NewPassword:     "newpassword",
	})

	require.NoError(t, err)
	assert.True(t, repo.updateUserPasswordCalled)
	assert.True(t, repo.deleteUserRefreshTokensCalled)
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.MinCost)
	repo.users["test@example.com"] = &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: string(hashedPassword),
		IsActive:     true,
	}

	service := NewService(repo, auth, nil, nil, "")

	err := service.ChangePassword(context.Background(), ChangePasswordInput{
		UserID:          "user-1",
		CurrentPassword: "wrongpassword",
		NewPassword:     "newpassword",
	})

	assert.ErrorIs(t, err, ErrWrongPassword)
	assert.False(t, repo.updateUserPasswordCalled)
}

func TestChangePassword_ClearsMustChangePassword(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("temppassword"), bcrypt.MinCost)
	repo.users["test@example.com"] = &domain.User{
		ID:                 "user-1",
		Email:              "test@example.com",
		PasswordHash:       string(hashedPassword),
		IsActive:           true,
		MustChangePassword: true,
	}

	service := NewService(repo, auth, nil, nil, "")

	err := service.ChangePassword(context.Background(), ChangePasswordInput{
		UserID:          "user-1",
		CurrentPassword: "temppassword",
		NewPassword:     "newpassword",
	})

	require.NoError(t, err)
	assert.True(t, repo.updateUserCalled)
	assert.False(t, repo.users["test@example.com"].MustChangePassword)
}

func TestUpdateProfile_Success(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.users["test@example.com"] = &domain.User{
		ID:       "user-1",
		Email:    "test@example.com",
		IsActive: true,
	}

	service := NewService(repo, auth, nil, nil, "")

	firstName := "John"
	lastName := "Doe"
	user, err := service.UpdateProfile(context.Background(), UpdateProfileInput{
		UserID:    "user-1",
		FirstName: &firstName,
		LastName:  &lastName,
	})

	require.NoError(t, err)
	assert.Equal(t, "John", user.FirstName)
	assert.Equal(t, "Doe", user.LastName)
}

func TestUpdateProfile_PartialUpdate(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.users["test@example.com"] = &domain.User{
		ID:        "user-1",
		Email:     "test@example.com",
		FirstName: "Original",
		LastName:  "Name",
		IsActive:  true,
	}

	service := NewService(repo, auth, nil, nil, "")

	newFirst := "Updated"
	user, err := service.UpdateProfile(context.Background(), UpdateProfileInput{
		UserID:    "user-1",
		FirstName: &newFirst,
	})

	require.NoError(t, err)
	assert.Equal(t, "Updated", user.FirstName)
	assert.Equal(t, "Name", user.LastName) // Unchanged
}

func TestForgotPassword_NoEmailSender(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	service := NewService(repo, auth, nil, nil, "https://example.com")

	err := service.ForgotPassword(context.Background(), "test@example.com")
	assert.ErrorIs(t, err, ErrEmailNotConfigured)
}

func TestForgotPassword_UserNotFound(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}
	emailSender := &mockEmailSender{}

	service := NewService(repo, auth, nil, emailSender, "https://example.com")

	err := service.ForgotPassword(context.Background(), "nonexistent@example.com")
	require.NoError(t, err) // silent — no email enumeration
	assert.False(t, emailSender.called)
}

func TestForgotPassword_Success(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}
	emailSender := &mockEmailSender{}

	repo.users["test@example.com"] = &domain.User{
		ID:    "user-1",
		Email: "test@example.com",
	}

	service := NewService(repo, auth, nil, emailSender, "https://example.com")

	err := service.ForgotPassword(context.Background(), "test@example.com")
	require.NoError(t, err)
	assert.True(t, emailSender.called)
	assert.Equal(t, "test@example.com", emailSender.to)
	assert.True(t, repo.savePasswordResetTokenCalled)
	assert.Contains(t, emailSender.body, "https://example.com/reset-password?token=")
}

func TestForgotPassword_RateLimit(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}
	emailSender := &mockEmailSender{}

	repo.users["test@example.com"] = &domain.User{
		ID:    "user-1",
		Email: "test@example.com",
	}
	// Token created 2 minutes ago — should be rate limited
	repo.latestPasswordResetToken = &domain.PasswordResetToken{
		CreatedAt: time.Now().Add(-2 * time.Minute),
	}

	service := NewService(repo, auth, nil, emailSender, "https://example.com")

	err := service.ForgotPassword(context.Background(), "test@example.com")
	require.NoError(t, err)
	assert.False(t, emailSender.called) // rate limited, no email sent
	assert.False(t, repo.savePasswordResetTokenCalled)
}

func TestResetPassword_Success(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.users["test@example.com"] = &domain.User{
		ID:                 "user-1",
		Email:              "test@example.com",
		MustChangePassword: true,
		IsActive:           true,
	}

	repo.getPasswordResetTokenResult = &domain.PasswordResetToken{
		UserID: "user-1",
		Token:  "valid-token",
	}

	service := NewService(repo, auth, nil, nil, "")

	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Token:       "valid-token",
		NewPassword: "newpassword123",
	})

	require.NoError(t, err)
	assert.True(t, repo.updateUserPasswordCalled)
	assert.True(t, repo.deleteUserPasswordResetTokensCalled)
	assert.True(t, repo.deleteUserRefreshTokensCalled)
	assert.True(t, repo.updateUserCalled) // must_change_password cleared
	assert.False(t, repo.users["test@example.com"].MustChangePassword)
}

func TestResetPassword_InvalidToken(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	service := NewService(repo, auth, nil, nil, "")

	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Token:       "invalid-token",
		NewPassword: "newpassword123",
	})

	assert.ErrorIs(t, err, ErrInvalidResetToken)
	assert.False(t, repo.updateUserPasswordCalled)
}

func TestForgotPassword_EmailSendFails(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}
	emailSender := &mockEmailSender{err: errors.New("smtp connection refused")}

	repo.users["test@example.com"] = &domain.User{
		ID:    "user-1",
		Email: "test@example.com",
	}

	service := NewService(repo, auth, nil, emailSender, "https://example.com")

	err := service.ForgotPassword(context.Background(), "test@example.com")
	require.NoError(t, err) // still returns nil despite email failure
	assert.True(t, repo.savePasswordResetTokenCalled)
	assert.True(t, emailSender.called)
}

func TestResetPassword_MustChangePasswordAlreadyFalse(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.users["test@example.com"] = &domain.User{
		ID:                 "user-1",
		Email:              "test@example.com",
		MustChangePassword: false,
		IsActive:           true,
	}

	repo.getPasswordResetTokenResult = &domain.PasswordResetToken{
		UserID: "user-1",
		Token:  "valid-token",
	}

	service := NewService(repo, auth, nil, nil, "")

	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Token:       "valid-token",
		NewPassword: "newpassword123",
	})

	require.NoError(t, err)
	assert.True(t, repo.updateUserPasswordCalled)
	assert.True(t, repo.deleteUserPasswordResetTokensCalled)
	assert.False(t, repo.updateUserCalled) // UpdateUser NOT called — flag was already false
}

func TestAdminCreateUser_Success(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}
	handler := &mockUserCreatedHandler{}

	service := NewService(repo, auth, handler, nil, "")

	user, err := service.AdminCreateUser(context.Background(), AdminCreateUserInput{
		Email:     "newuser@example.com",
		Password:  "password123",
		FirstName: "New",
		LastName:  "User",
		Role:      domain.RoleOperator,
	})

	require.NoError(t, err)
	assert.Equal(t, "newuser@example.com", user.Email)
	assert.Equal(t, domain.RoleOperator, user.Role)
	assert.True(t, user.MustChangePassword)
	assert.True(t, user.IsActive)
	assert.True(t, handler.called)
}

func TestAdminCreateUser_InvalidRole(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	service := NewService(repo, auth, nil, nil, "")

	_, err := service.AdminCreateUser(context.Background(), AdminCreateUserInput{
		Email:    "test@example.com",
		Password: "password123",
		Role:     domain.Role("superadmin"),
	})

	assert.ErrorIs(t, err, ErrInvalidRole)
}

func TestAdminCreateUser_DuplicateEmail(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.users["existing@example.com"] = &domain.User{Email: "existing@example.com"}

	service := NewService(repo, auth, nil, nil, "")

	_, err := service.AdminCreateUser(context.Background(), AdminCreateUserInput{
		Email:    "existing@example.com",
		Password: "password123",
		Role:     domain.RoleUser,
	})

	assert.ErrorIs(t, err, ErrEmailExists)
}

func TestAdminUpdateUser_Success(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.users["target@example.com"] = &domain.User{
		ID:       "target-1",
		Email:    "target@example.com",
		Role:     domain.RoleUser,
		IsActive: true,
		LastName: "Original",
	}

	service := NewService(repo, auth, nil, nil, "")

	newRole := domain.RoleOperator
	newFirst := "Updated"
	user, err := service.AdminUpdateUser(context.Background(), "admin-1", AdminUpdateUserInput{
		UserID:    "target-1",
		Role:      &newRole,
		FirstName: &newFirst,
	})

	require.NoError(t, err)
	assert.Equal(t, domain.RoleOperator, user.Role)
	assert.Equal(t, "Updated", user.FirstName)
	assert.Equal(t, "Original", user.LastName) // unchanged
	assert.True(t, user.IsActive)              // unchanged
	assert.True(t, repo.updateUserCalled)
}

func TestAdminUpdateUser_SelfModification(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	service := NewService(repo, auth, nil, nil, "")

	newRole := domain.RoleUser
	_, err := service.AdminUpdateUser(context.Background(), "admin-1", AdminUpdateUserInput{
		UserID: "admin-1",
		Role:   &newRole,
	})

	assert.ErrorIs(t, err, ErrCannotModifySelf)
}

func TestAdminUpdateUser_Deactivation(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.users["target@example.com"] = &domain.User{
		ID:       "target-1",
		Email:    "target@example.com",
		IsActive: true,
	}

	service := NewService(repo, auth, nil, nil, "")

	isActive := false
	user, err := service.AdminUpdateUser(context.Background(), "admin-1", AdminUpdateUserInput{
		UserID:   "target-1",
		IsActive: &isActive,
	})

	require.NoError(t, err)
	assert.False(t, user.IsActive)
	assert.True(t, repo.deleteUserRefreshTokensCalled)
}

func TestAdminUpdateUser_NotFound(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	service := NewService(repo, auth, nil, nil, "")

	_, err := service.AdminUpdateUser(context.Background(), "admin-1", AdminUpdateUserInput{
		UserID: "nonexistent",
	})

	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestAdminUpdateUser_InvalidRole(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.users["target@example.com"] = &domain.User{
		ID:       "target-1",
		Email:    "target@example.com",
		IsActive: true,
	}

	service := NewService(repo, auth, nil, nil, "")

	badRole := domain.Role("superadmin")
	_, err := service.AdminUpdateUser(context.Background(), "admin-1", AdminUpdateUserInput{
		UserID: "target-1",
		Role:   &badRole,
	})

	assert.ErrorIs(t, err, ErrInvalidRole)
}

func TestAdminResetPassword_Success(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.users["target@example.com"] = &domain.User{
		ID:       "target-1",
		Email:    "target@example.com",
		IsActive: true,
	}

	service := NewService(repo, auth, nil, nil, "")

	err := service.AdminResetPassword(context.Background(), "admin-1", AdminResetPasswordInput{
		UserID:      "target-1",
		NewPassword: "newpassword123",
	})

	require.NoError(t, err)
	assert.True(t, repo.updateUserPasswordCalled)
	assert.True(t, repo.updateUserCalled)
	assert.True(t, repo.deleteUserRefreshTokensCalled)
	assert.True(t, repo.users["target@example.com"].MustChangePassword)
}

func TestAdminResetPassword_SelfReset(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	service := NewService(repo, auth, nil, nil, "")

	err := service.AdminResetPassword(context.Background(), "admin-1", AdminResetPasswordInput{
		UserID:      "admin-1",
		NewPassword: "newpassword123",
	})

	assert.ErrorIs(t, err, ErrCannotModifySelf)
}

func TestAdminResetPassword_UserNotFound(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	service := NewService(repo, auth, nil, nil, "")

	err := service.AdminResetPassword(context.Background(), "admin-1", AdminResetPasswordInput{
		UserID:      "nonexistent",
		NewPassword: "newpassword123",
	})

	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestListUsers_Passthrough(t *testing.T) {
	repo := newMockRepository()
	auth := &mockAuthenticator{}

	repo.listUsersResult = []*domain.User{
		{ID: "1", Email: "user1@example.com"},
		{ID: "2", Email: "user2@example.com"},
	}
	repo.listUsersTotal = 2

	service := NewService(repo, auth, nil, nil, "")

	role := domain.RoleUser
	users, total, err := service.ListUsers(context.Background(), UserFilter{
		Role:   &role,
		Limit:  10,
		Offset: 0,
	})

	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, 2, total)
	assert.True(t, repo.listUsersCalled)
	assert.Equal(t, &role, repo.listUsersFilter.Role)
}
