//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/bissquit/incident-garden/internal/pkg/httputil"
	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminCreateTestUser creates a user via the admin API and returns the user ID.
func adminCreateTestUser(t *testing.T, client *testutil.Client, email, password string, role string) string {
	t.Helper()
	resp, err := client.POST("/api/v1/users", map[string]interface{}{
		"email":    email,
		"password": password,
		"role":     role,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.ID
}

// getAdminID returns the admin user's ID via GET /me.
func getAdminID(t *testing.T, client *testutil.Client) string {
	t.Helper()
	resp, err := client.GET("/api/v1/me")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.ID
}

// insertPasswordResetToken inserts a password reset token directly into the DB.
func insertPasswordResetToken(t *testing.T, userID, token string, expiresAt time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := testDB.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
	`, userID, token, expiresAt)
	require.NoError(t, err)
}

// generateToken creates a random hex token for testing.
func generateToken(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return hex.EncodeToString(b)
}

// =============================================================================
// Group 1: Password Change (PUT /me/password)
// =============================================================================

func TestIdentity_PasswordChange_Success_And_LoginWithNewPassword(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "oldpassword123"
	newPassword := "newpassword456"
	adminCreateTestUser(t, client, email, password, "user")

	userClient := newTestClient(t)
	userClient.LoginAs(t, email, password)

	resp, err := userClient.PUT("/api/v1/me/password", map[string]interface{}{
		"current_password": password,
		"new_password":     newPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	for _, c := range resp.Cookies() {
		if c.Name == httputil.AccessTokenCookie ||
			c.Name == httputil.RefreshTokenCookie ||
			c.Name == httputil.CSRFTokenCookie {
			assert.True(t, c.MaxAge < 0, "cookie %s should be cleared", c.Name)
		}
	}
	resp.Body.Close()

	// Old password should fail
	loginClient := newTestClient(t)
	resp, err = loginClient.WithoutValidation().POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// New password should work
	resp, err = loginClient.POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": newPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_PasswordChange_WrongCurrentPassword(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "correctpassword1"
	adminCreateTestUser(t, client, email, password, "user")

	userClient := newTestClient(t)
	userClient.LoginAs(t, email, password)

	resp, err := userClient.PUT("/api/v1/me/password", map[string]interface{}{
		"current_password": "wrongpassword1",
		"new_password":     "newpassword123",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_PasswordChange_NewPasswordTooShort(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "validpassword1"
	adminCreateTestUser(t, client, email, password, "user")

	userClient := newTestClient(t)
	userClient.LoginAs(t, email, password)

	resp, err := userClient.PUT("/api/v1/me/password", map[string]interface{}{
		"current_password": password,
		"new_password":     "short",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_PasswordChange_WithoutAuth(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.WithoutValidation().PUT("/api/v1/me/password", map[string]interface{}{
		"current_password": "whatever123",
		"new_password":     "whatever456",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_PasswordChange_ClearsMustChangePassword(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	tempPassword := "temppassword1"
	adminCreateTestUser(t, client, email, tempPassword, "user")

	userClient := newTestClient(t)
	userClient.LoginAs(t, email, tempPassword)

	// Verify must_change_password is initially true
	resp, err := userClient.GET("/api/v1/me")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var me struct {
		Data struct {
			MustChangePassword bool `json:"must_change_password"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &me)
	assert.True(t, me.Data.MustChangePassword)

	newPassword := "permanentpwd1"
	resp, err = userClient.PUT("/api/v1/me/password", map[string]interface{}{
		"current_password": tempPassword,
		"new_password":     newPassword,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Re-login and check the flag
	userClient2 := newTestClient(t)
	userClient2.LoginAs(t, email, newPassword)

	resp, err = userClient2.GET("/api/v1/me")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var me2 struct {
		Data struct {
			MustChangePassword bool `json:"must_change_password"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &me2)
	assert.False(t, me2.Data.MustChangePassword)
}

func TestIdentity_PasswordChange_InvalidatesRefreshTokens(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "password12345"
	adminCreateTestUser(t, client, email, password, "user")

	userClient := newTestClient(t)
	userClient.LoginAs(t, email, password)

	// Change password
	newPassword := "newpassword789"
	resp, err := userClient.PUT("/api/v1/me/password", map[string]interface{}{
		"current_password": password,
		"new_password":     newPassword,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Cookies were cleared by the password change response (MaxAge=-1).
	// The cookie jar drops them, so the refresh request arrives without a token.
	// Server returns 400 "missing refresh token".
	resp, err = userClient.WithoutValidation().POST("/api/v1/auth/refresh", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// =============================================================================
// Group 2: Profile Update (PATCH /me)
// =============================================================================

func TestIdentity_ProfileUpdate_FirstName(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	adminCreateTestUser(t, client, email, "password1234", "user")

	userClient := newTestClient(t)
	userClient.LoginAs(t, email, "password1234")

	resp, err := userClient.PATCH("/api/v1/me", map[string]interface{}{
		"first_name": "Alice",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			FirstName string `json:"first_name"`
			Email     string `json:"email"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, "Alice", result.Data.FirstName)
	assert.Equal(t, email, result.Data.Email)
}

func TestIdentity_ProfileUpdate_BothFields(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	adminCreateTestUser(t, client, email, "password1234", "user")

	userClient := newTestClient(t)
	userClient.LoginAs(t, email, "password1234")

	resp, err := userClient.PATCH("/api/v1/me", map[string]interface{}{
		"first_name": "Bob",
		"last_name":  "Smith",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, "Bob", result.Data.FirstName)
	assert.Equal(t, "Smith", result.Data.LastName)
}

func TestIdentity_ProfileUpdate_EmptyBody(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	adminCreateTestUser(t, client, email, "password1234", "user")

	userClient := newTestClient(t)
	userClient.LoginAs(t, email, "password1234")

	// Set names first
	resp, err := userClient.PATCH("/api/v1/me", map[string]interface{}{
		"first_name": "Carol",
		"last_name":  "Jones",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Send empty body
	resp, err = userClient.PATCH("/api/v1/me", map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, "Carol", result.Data.FirstName)
	assert.Equal(t, "Jones", result.Data.LastName)
}

func TestIdentity_ProfileUpdate_WithoutAuth(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.WithoutValidation().PATCH("/api/v1/me", map[string]interface{}{
		"first_name": "NoAuth",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_ProfileUpdate_RoleAndEmailUnchanged(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	adminCreateTestUser(t, client, email, "password1234", "user")

	userClient := newTestClient(t)
	userClient.LoginAs(t, email, "password1234")

	resp, err := userClient.PATCH("/api/v1/me", map[string]interface{}{
		"first_name": "Updated",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, email, result.Data.Email)
	assert.Equal(t, "user", result.Data.Role)
}

// =============================================================================
// Group 3: Forgot Password (POST /auth/forgot-password)
// =============================================================================

func TestIdentity_ForgotPassword_EmailNotConfigured(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.POST("/api/v1/auth/forgot-password", map[string]interface{}{
		"email": "someone@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_ForgotPassword_InvalidEmail(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.WithoutValidation().POST("/api/v1/auth/forgot-password", map[string]interface{}{
		"email": "not-an-email",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// =============================================================================
// Group 4: Reset Password (POST /auth/reset-password)
// =============================================================================

func TestIdentity_ResetPassword_ValidToken(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "original1234"
	userID := adminCreateTestUser(t, client, email, password, "user")

	token := generateToken(t)
	insertPasswordResetToken(t, userID, token, time.Now().Add(1*time.Hour))

	newPassword := "resetedpwd123"
	resetClient := newTestClient(t)
	resp, err := resetClient.POST("/api/v1/auth/reset-password", map[string]interface{}{
		"token":        token,
		"new_password": newPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// New password should work
	loginClient := newTestClient(t)
	resp, err = loginClient.POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": newPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_ResetPassword_InvalidToken(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.POST("/api/v1/auth/reset-password", map[string]interface{}{
		"token":        "nonexistent-token-value-that-is-invalid",
		"new_password": "newpassword1",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_ResetPassword_ExpiredToken(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	userID := adminCreateTestUser(t, client, email, "password1234", "user")

	token := generateToken(t)
	insertPasswordResetToken(t, userID, token, time.Now().UTC().Add(-1*time.Hour))

	resetClient := newTestClient(t)
	resp, err := resetClient.POST("/api/v1/auth/reset-password", map[string]interface{}{
		"token":        token,
		"new_password": "newpassword1",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_ResetPassword_ShortNewPassword(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.WithoutValidation().POST("/api/v1/auth/reset-password", map[string]interface{}{
		"token":        "some-token",
		"new_password": "short",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_ResetPassword_ClearsMustChangePassword(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	userID := adminCreateTestUser(t, client, email, "temppass1234", "operator")

	token := generateToken(t)
	insertPasswordResetToken(t, userID, token, time.Now().Add(1*time.Hour))

	newPassword := "permanentpw1"
	resetClient := newTestClient(t)
	resp, err := resetClient.POST("/api/v1/auth/reset-password", map[string]interface{}{
		"token":        token,
		"new_password": newPassword,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	loginClient := newTestClient(t)
	loginClient.LoginAs(t, email, newPassword)

	resp, err = loginClient.GET("/api/v1/me")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var me struct {
		Data struct {
			MustChangePassword bool `json:"must_change_password"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &me)
	assert.False(t, me.Data.MustChangePassword)
}

func TestIdentity_ResetPassword_InvalidatesRefreshTokens(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "loginable123"
	userID := adminCreateTestUser(t, client, email, password, "user")

	// Login to create a refresh token
	userClient := newTestClient(t)
	userClient.LoginAs(t, email, password)

	// Reset via token
	token := generateToken(t)
	insertPasswordResetToken(t, userID, token, time.Now().Add(1*time.Hour))

	resetClient := newTestClient(t)
	resp, err := resetClient.POST("/api/v1/auth/reset-password", map[string]interface{}{
		"token":        token,
		"new_password": "afterreset12",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Old refresh token should be invalid
	resp, err = userClient.WithoutValidation().POST("/api/v1/auth/refresh", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_ResetPassword_TokenConsumed_SecondUseFails(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	userID := adminCreateTestUser(t, client, email, "password1234", "user")

	token := generateToken(t)
	insertPasswordResetToken(t, userID, token, time.Now().Add(1*time.Hour))

	resetClient := newTestClient(t)
	resp, err := resetClient.POST("/api/v1/auth/reset-password", map[string]interface{}{
		"token":        token,
		"new_password": "firstrest123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	resp, err = resetClient.POST("/api/v1/auth/reset-password", map[string]interface{}{
		"token":        token,
		"new_password": "secondrest12",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// =============================================================================
// Group 5: Login/Refresh + is_active/must_change_password
// =============================================================================

func TestIdentity_Login_DeactivatedUser(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "password1234"
	userID := adminCreateTestUser(t, client, email, password, "user")

	// Deactivate the user
	isActive := false
	resp, err := client.PATCH("/api/v1/users/"+userID, map[string]interface{}{
		"is_active": isActive,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Login should fail with 403
	loginClient := newTestClient(t)
	resp, err = loginClient.WithoutValidation().POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_Refresh_DeactivatedUser(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "password1234"
	userID := adminCreateTestUser(t, client, email, password, "user")

	// Login first to get refresh token
	userClient := newTestClient(t)
	userClient.LoginAs(t, email, password)

	// Deactivate via admin — this also invalidates all refresh tokens
	resp, err := client.PATCH("/api/v1/users/"+userID, map[string]interface{}{
		"is_active": false,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Refresh fails because tokens were deleted during deactivation (401 invalid token)
	resp, err = userClient.WithoutValidation().POST("/api/v1/auth/refresh", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_Login_ResponseContainsMustChangePassword(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "password1234"
	adminCreateTestUser(t, client, email, password, "user")

	loginClient := newTestClient(t)
	resp, err := loginClient.POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			User struct {
				MustChangePassword bool `json:"must_change_password"`
			} `json:"user"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.True(t, result.Data.User.MustChangePassword)
}

// =============================================================================
// Group 6: Admin List Users (GET /users)
// =============================================================================

func TestIdentity_AdminListUsers_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.GET("/api/v1/users")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Users  []struct{ ID string } `json:"users"`
			Total  int                   `json:"total"`
			Limit  int                   `json:"limit"`
			Offset int                   `json:"offset"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.NotEmpty(t, result.Data.Users)
	assert.Greater(t, result.Data.Total, 0)
	assert.Greater(t, result.Data.Limit, 0)
	assert.GreaterOrEqual(t, result.Data.Offset, 0)
}

func TestIdentity_AdminListUsers_FilterByRole(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.GET("/api/v1/users?role=admin")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Users []struct {
				Role string `json:"role"`
			} `json:"users"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Greater(t, result.Data.Total, 0)

	for _, u := range result.Data.Users {
		assert.Equal(t, "admin", u.Role)
	}
}

func TestIdentity_AdminListUsers_Pagination(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.GET("/api/v1/users?limit=1&offset=0")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var first struct {
		Data struct {
			Users  []struct{ ID string } `json:"users"`
			Total  int                   `json:"total"`
			Limit  int                   `json:"limit"`
			Offset int                   `json:"offset"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &first)
	assert.Len(t, first.Data.Users, 1)
	assert.Equal(t, 1, first.Data.Limit)
	assert.Equal(t, 0, first.Data.Offset)

	if first.Data.Total > 1 {
		resp, err = client.GET("/api/v1/users?limit=1&offset=1")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var second struct {
			Data struct {
				Users []struct{ ID string } `json:"users"`
			} `json:"data"`
		}
		testutil.DecodeJSON(t, resp, &second)
		assert.Len(t, second.Data.Users, 1)
		assert.NotEqual(t, first.Data.Users[0].ID, second.Data.Users[0].ID)
	}
}

func TestIdentity_AdminListUsers_InvalidRoleFilter(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.WithoutValidation().GET("/api/v1/users?role=superadmin")
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminListUsers_OperatorForbidden(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	resp, err := client.WithoutValidation().GET("/api/v1/users")
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminListUsers_NoAuth(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.WithoutValidation().GET("/api/v1/users")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

// =============================================================================
// Group 7: Admin Get User (GET /users/{id})
// =============================================================================

func TestIdentity_AdminGetUser_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	userID := adminCreateTestUser(t, client, email, "password1234", "operator")

	resp, err := client.GET("/api/v1/users/" + userID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, userID, result.Data.ID)
	assert.Equal(t, email, result.Data.Email)
	assert.Equal(t, "operator", result.Data.Role)
}

func TestIdentity_AdminGetUser_NotFound(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.GET("/api/v1/users/00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminGetUser_OperatorForbidden(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	resp, err := client.WithoutValidation().GET("/api/v1/users/00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// =============================================================================
// Group 8: Admin Create User (POST /users)
// =============================================================================

func TestIdentity_AdminCreateUser_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	resp, err := client.POST("/api/v1/users", map[string]interface{}{
		"email":    email,
		"password": "password1234",
		"role":     "operator",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID                 string `json:"id"`
			Email              string `json:"email"`
			Role               string `json:"role"`
			IsActive           bool   `json:"is_active"`
			MustChangePassword bool   `json:"must_change_password"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.NotEmpty(t, result.Data.ID)
	assert.Equal(t, email, result.Data.Email)
	assert.Equal(t, "operator", result.Data.Role)
	assert.True(t, result.Data.IsActive)
	assert.True(t, result.Data.MustChangePassword)
}

func TestIdentity_AdminCreateUser_CanLogin(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "password1234"
	adminCreateTestUser(t, client, email, password, "user")

	loginClient := newTestClient(t)
	resp, err := loginClient.POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminCreateUser_DuplicateEmail(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	adminCreateTestUser(t, client, email, "password1234", "user")

	resp, err := client.POST("/api/v1/users", map[string]interface{}{
		"email":    email,
		"password": "password5678",
		"role":     "user",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminCreateUser_InvalidRole(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.WithoutValidation().POST("/api/v1/users", map[string]interface{}{
		"email":    testutil.RandomEmail(),
		"password": "password1234",
		"role":     "superadmin",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminCreateUser_ShortPassword(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.WithoutValidation().POST("/api/v1/users", map[string]interface{}{
		"email":    testutil.RandomEmail(),
		"password": "short",
		"role":     "user",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminCreateUser_OperatorForbidden(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	resp, err := client.WithoutValidation().POST("/api/v1/users", map[string]interface{}{
		"email":    testutil.RandomEmail(),
		"password": "password1234",
		"role":     "user",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// =============================================================================
// Group 9: Admin Update User (PATCH /users/{id})
// =============================================================================

func TestIdentity_AdminUpdateUser_ChangeRole(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	userID := adminCreateTestUser(t, client, email, "password1234", "user")

	resp, err := client.PATCH("/api/v1/users/"+userID, map[string]interface{}{
		"role": "operator",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Role string `json:"role"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, "operator", result.Data.Role)
}

func TestIdentity_AdminUpdateUser_Deactivate_And_CannotLogin(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	password := "password1234"
	userID := adminCreateTestUser(t, client, email, password, "user")

	resp, err := client.PATCH("/api/v1/users/"+userID, map[string]interface{}{
		"is_active": false,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			IsActive bool `json:"is_active"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.False(t, result.Data.IsActive)

	// Verify login fails
	loginClient := newTestClient(t)
	resp, err = loginClient.WithoutValidation().POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminUpdateUser_ModifySelf(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	adminID := getAdminID(t, client)

	resp, err := client.PATCH("/api/v1/users/"+adminID, map[string]interface{}{
		"role": "user",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminUpdateUser_NotFound(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.PATCH("/api/v1/users/00000000-0000-0000-0000-000000000000", map[string]interface{}{
		"role": "operator",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminUpdateUser_InvalidRole(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	userID := adminCreateTestUser(t, client, email, "password1234", "user")

	resp, err := client.WithoutValidation().PATCH("/api/v1/users/"+userID, map[string]interface{}{
		"role": "superadmin",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminUpdateUser_OperatorForbidden(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	resp, err := client.WithoutValidation().PATCH("/api/v1/users/00000000-0000-0000-0000-000000000000", map[string]interface{}{
		"role": "user",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminUpdateUser_PartialUpdate_UnchangedFieldsPreserved(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	userID := adminCreateTestUser(t, client, email, "password1234", "user")

	// Set profile fields first
	resp, err := client.PATCH("/api/v1/users/"+userID, map[string]interface{}{
		"first_name": "Dave",
		"last_name":  "Wilson",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Update only role — name fields should be preserved
	resp, err = client.PATCH("/api/v1/users/"+userID, map[string]interface{}{
		"role": "operator",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Role      string `json:"role"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
			IsActive  bool   `json:"is_active"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, "operator", result.Data.Role)
	assert.Equal(t, "Dave", result.Data.FirstName)
	assert.Equal(t, "Wilson", result.Data.LastName)
	assert.Equal(t, email, result.Data.Email)
	assert.True(t, result.Data.IsActive)
}

// =============================================================================
// Group 10: Admin Reset Password (POST /users/{id}/reset-password)
// =============================================================================

func TestIdentity_AdminResetPassword_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	oldPassword := "password1234"
	newPassword := "adminreset12"
	userID := adminCreateTestUser(t, client, email, oldPassword, "user")

	resp, err := client.POST("/api/v1/users/"+userID+"/reset-password", map[string]interface{}{
		"new_password": newPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// New password should work
	loginClient := newTestClient(t)
	resp, err = loginClient.POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": newPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify must_change_password is set
	var loginResult struct {
		Data struct {
			User struct {
				MustChangePassword bool `json:"must_change_password"`
			} `json:"user"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &loginResult)
	assert.True(t, loginResult.Data.User.MustChangePassword)

	// Old password should not work
	oldClient := newTestClient(t)
	resp, err = oldClient.WithoutValidation().POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": oldPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminResetPassword_Self(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	adminID := getAdminID(t, client)

	resp, err := client.POST("/api/v1/users/"+adminID+"/reset-password", map[string]interface{}{
		"new_password": "newadminpwd1",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminResetPassword_NotFound(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.POST("/api/v1/users/00000000-0000-0000-0000-000000000000/reset-password", map[string]interface{}{
		"new_password": "newpassword1",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminResetPassword_ShortPassword(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	email := testutil.RandomEmail()
	userID := adminCreateTestUser(t, client, email, "password1234", "user")

	resp, err := client.WithoutValidation().POST("/api/v1/users/"+userID+"/reset-password", map[string]interface{}{
		"new_password": "short",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestIdentity_AdminResetPassword_OperatorForbidden(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	resp, err := client.WithoutValidation().POST("/api/v1/users/00000000-0000-0000-0000-000000000000/reset-password", map[string]interface{}{
		"new_password": "newpassword1",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}
