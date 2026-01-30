//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth_Register_Login_Flow(t *testing.T) {
	client := newTestClient(t)
	email := testutil.RandomEmail()
	password := "password123"

	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var registerResult struct {
		Data struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &registerResult)
	assert.Equal(t, email, registerResult.Data.Email)
	assert.Equal(t, "user", registerResult.Data.Role)
	assert.NotEmpty(t, registerResult.Data.ID)

	resp, err = client.POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var loginResult struct {
		Data struct {
			User struct {
				Email string `json:"email"`
			} `json:"user"`
			Tokens struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			} `json:"tokens"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &loginResult)
	assert.Equal(t, email, loginResult.Data.User.Email)
	assert.NotEmpty(t, loginResult.Data.Tokens.AccessToken)
	assert.NotEmpty(t, loginResult.Data.Tokens.RefreshToken)
}

func TestAuth_Login_InvalidCredentials(t *testing.T) {
	client := newTestClient(t)
	resp, err := client.POST("/api/v1/auth/login", map[string]string{
		"email":    "nonexistent@example.com",
		"password": "wrongpassword",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestAuth_Register_DuplicateEmail(t *testing.T) {
	client := newTestClient(t)
	email := testutil.RandomEmail()

	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "password456",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()
}

func TestAuth_Me_RequiresAuth(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.GET("/api/v1/me")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestAuth_Me_ReturnsCurrentUser(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.GET("/api/v1/me")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, "admin@example.com", result.Data.Email)
	assert.Equal(t, "admin", result.Data.Role)
}
