//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/pkg/httputil"
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

	// Check that cookies are set
	cookies := resp.Cookies()
	var hasAccessToken, hasRefreshToken, hasCSRFToken bool
	for _, c := range cookies {
		switch c.Name {
		case httputil.AccessTokenCookie:
			hasAccessToken = true
			assert.True(t, c.HttpOnly)
		case httputil.RefreshTokenCookie:
			hasRefreshToken = true
			assert.True(t, c.HttpOnly)
		case httputil.CSRFTokenCookie:
			hasCSRFToken = true
			assert.False(t, c.HttpOnly) // CSRF token must be readable by JS
		}
	}
	assert.True(t, hasAccessToken, "access_token cookie should be set")
	assert.True(t, hasRefreshToken, "refresh_token cookie should be set")
	assert.True(t, hasCSRFToken, "csrf_token cookie should be set")

	var loginResult struct {
		Data struct {
			User struct {
				Email string `json:"email"`
			} `json:"user"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &loginResult)
	assert.Equal(t, email, loginResult.Data.User.Email)
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

func TestAuth_CookieAuth_WorksWithCSRF(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// POST request should work with CSRF token
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":        "Test Service",
		"slug":        testutil.RandomSlug("test"),
		"description": "Test",
		"status":      "operational",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

func TestAuth_CookieAuth_FailsWithoutCSRF(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Clear CSRF token but keep cookies
	client.CSRFToken = ""

	// POST request should fail without CSRF token
	resp, err := client.WithoutValidation().POST("/api/v1/services", map[string]interface{}{
		"name":        "Test Service",
		"slug":        testutil.RandomSlug("test"),
		"description": "Test",
		"status":      "operational",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

func TestAuth_Refresh_UpdatesCookies(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Store original CSRF token
	originalCSRF := client.CSRFToken

	// Refresh tokens
	resp, err := client.POST("/api/v1/auth/refresh", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Check that new cookies are set
	var hasNewAccessToken bool
	for _, c := range resp.Cookies() {
		if c.Name == httputil.AccessTokenCookie {
			hasNewAccessToken = true
		}
		if c.Name == httputil.CSRFTokenCookie {
			client.CSRFToken = c.Value
		}
	}
	assert.True(t, hasNewAccessToken, "new access_token cookie should be set")
	assert.NotEqual(t, originalCSRF, client.CSRFToken, "CSRF token should be rotated")
	resp.Body.Close()

	// Verify auth still works with new cookies
	resp, err = client.GET("/api/v1/me")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestAuth_Logout_ClearsCookies(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Logout
	resp, err := client.POST("/api/v1/auth/logout", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Check that cookies are cleared (Max-Age < 0)
	for _, c := range resp.Cookies() {
		if c.Name == httputil.AccessTokenCookie ||
			c.Name == httputil.RefreshTokenCookie ||
			c.Name == httputil.CSRFTokenCookie {
			assert.True(t, c.MaxAge < 0, "cookie %s should be cleared", c.Name)
		}
	}
	resp.Body.Close()

	// Subsequent request should fail
	client.ClearToken() // Reset cookie jar to apply cleared cookies
	resp, err = client.WithoutValidation().GET("/api/v1/me")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestAuth_AuthorizationHeader_StillWorks(t *testing.T) {
	// Test that Authorization header still works for backward compatibility
	client := newTestClient(t)

	// Login to get token via cookie
	resp, err := client.POST("/api/v1/auth/login", map[string]string{
		"email":    "admin@example.com",
		"password": "admin123",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Extract access token from cookie
	var accessToken string
	for _, c := range resp.Cookies() {
		if c.Name == httputil.AccessTokenCookie {
			accessToken = c.Value
			break
		}
	}
	resp.Body.Close()
	require.NotEmpty(t, accessToken)

	// Create new client without cookies, use Authorization header
	apiClient := newTestClient(t)
	apiClient.Token = accessToken

	// Request should work with Authorization header (no CSRF needed)
	resp, err = apiClient.GET("/api/v1/me")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
