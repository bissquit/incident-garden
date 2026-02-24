package identity

import (
	"context"
	"errors"
	"testing"

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

func (m *mockRepository) ListUsers(_ context.Context, _ UserFilter) ([]*domain.User, int, error) {
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
	return nil
}

func (m *mockRepository) GetPasswordResetToken(_ context.Context, _ string) (*domain.PasswordResetToken, error) {
	return nil, ErrInvalidResetToken
}

func (m *mockRepository) GetLatestPasswordResetToken(_ context.Context, _ string) (*domain.PasswordResetToken, error) {
	return nil, nil
}

func (m *mockRepository) DeletePasswordResetToken(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepository) DeleteUserPasswordResetTokens(_ context.Context, _ string) error {
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

func TestRegister_CallsUserCreatedHandler(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	auth := &mockAuthenticator{}
	handler := &mockUserCreatedHandler{}

	service := NewService(repo, auth, handler)

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

	service := NewService(repo, auth, handler)

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

	service := NewService(repo, auth, nil) // nil handler

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

	service := NewService(repo, auth, handler)

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

	service := NewService(repo, auth, handler)

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

	service := NewService(repo, auth, nil)

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

	service := NewService(repo, auth, nil)

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

	service := NewService(repo, auth, nil)

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

	service := NewService(repo, auth, nil)

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

	service := NewService(repo, auth, nil)

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

	service := NewService(repo, auth, nil)

	newFirst := "Updated"
	user, err := service.UpdateProfile(context.Background(), UpdateProfileInput{
		UserID:    "user-1",
		FirstName: &newFirst,
	})

	require.NoError(t, err)
	assert.Equal(t, "Updated", user.FirstName)
	assert.Equal(t, "Name", user.LastName) // Unchanged
}
