package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository implements Repository for testing.
type mockRepository struct {
	users          map[string]*domain.User
	createUserErr  error
	getUserByEmail func(email string) (*domain.User, error)
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

func (m *mockRepository) UpdateUser(_ context.Context, _ *domain.User) error {
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
