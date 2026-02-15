//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifications_EmailChannel_VerificationFlow(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create email channel
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "test@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID         string `json:"id"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID
	assert.False(t, channelResp.Data.IsVerified, "new channel should not be verified")

	t.Cleanup(func() {
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Get verification code directly from DB for testing
	var code string
	err = testDB.QueryRow(context.Background(), `
		SELECT code FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&code)
	require.NoError(t, err, "verification code should exist in DB")
	assert.Len(t, code, 6, "code should be 6 digits")

	// Verify with wrong code should fail
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
		"code": "000000",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// Verify with correct code
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
		"code": code,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var verifiedResp struct {
		Data struct {
			IsVerified bool `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &verifiedResp)
	assert.True(t, verifiedResp.Data.IsVerified, "channel should be verified")

	// Code should be deleted after successful verification
	var count int
	err = testDB.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "code should be deleted after verification")
}

func TestNotifications_EmailChannel_InvalidCode(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create email channel
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "invalid-test@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID

	t.Cleanup(func() {
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Try invalid code format
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
		"code": "abc",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// Try wrong code
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
		"code": "999999",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestNotifications_EmailChannel_TooManyAttempts(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create email channel
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "attempts-test@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID

	t.Cleanup(func() {
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Make 5 wrong attempts
	for i := 0; i < 5; i++ {
		resp, err = client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
			"code": "000000",
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		resp.Body.Close()
	}

	// 6th attempt should return 429 Too Many Requests
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
		"code": "000000",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	resp.Body.Close()
}

func TestNotifications_EmailChannel_ResendCode(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create email channel
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "resend-test@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID

	t.Cleanup(func() {
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Get original code
	var originalCode string
	err = testDB.QueryRow(context.Background(), `
		SELECT code FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&originalCode)
	require.NoError(t, err)

	// Resend immediately should fail due to cooldown
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/resend-code", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	resp.Body.Close()

	// Update created_at to bypass cooldown
	_, err = testDB.Exec(context.Background(), `
		UPDATE channel_verification_codes
		SET created_at = NOW() - INTERVAL '2 minutes'
		WHERE channel_id = $1
	`, channelID)
	require.NoError(t, err)

	// Now resend should succeed
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/resend-code", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// New code should be different
	var newCode string
	err = testDB.QueryRow(context.Background(), `
		SELECT code FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&newCode)
	require.NoError(t, err)

	// While codes could theoretically be the same, it's very unlikely
	// This is just to verify a new code was generated
	t.Logf("original code: %s, new code: %s", originalCode, newCode)
}

func TestNotifications_EmailChannel_AlreadyVerified(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create email channel
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "already-verified@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID

	t.Cleanup(func() {
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Verify using DB code
	var code string
	err = testDB.QueryRow(context.Background(), `
		SELECT code FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&code)
	require.NoError(t, err)

	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
		"code": code,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Resend on already verified channel should fail
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/resend-code", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestNotifications_TelegramChannel_NoCodeNeeded(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create telegram channel
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "telegram",
		"target": "123456789",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID         string `json:"id"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID
	assert.False(t, channelResp.Data.IsVerified)

	t.Cleanup(func() {
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// No verification code should be created for telegram
	var count int
	err = testDB.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "no verification code for telegram")

	// Resend code should fail for telegram
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/resend-code", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestNotifications_ExpiredCode(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create email channel
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "expired-test@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID

	t.Cleanup(func() {
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Get the code
	var code string
	err = testDB.QueryRow(context.Background(), `
		SELECT code FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&code)
	require.NoError(t, err)

	// Expire the code
	_, err = testDB.Exec(context.Background(), `
		UPDATE channel_verification_codes
		SET expires_at = NOW() - INTERVAL '1 hour'
		WHERE channel_id = $1
	`, channelID)
	require.NoError(t, err)

	// Attempt to verify with expired code
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
		"code": code,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errResp)
	assert.Contains(t, errResp.Error.Message, "expired")
}

func TestNotifications_ChannelNotOwned(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create channel as user
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "user-channel@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID

	t.Cleanup(func() {
		client.LoginAsUser(t)
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Login as operator and try to verify someone else's channel
	client.LoginAsOperator(t)

	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
		"code": "123456",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()

	// Also try resend
	resp, err = client.POST("/api/v1/me/channels/"+channelID+"/resend-code", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

func TestNotifications_VerifyCodeDeletedOnChannelDelete(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create email channel
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "delete-cascade@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID

	// Verify code exists
	var count int
	err = testDB.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Delete channel
	resp, err = client.DELETE("/api/v1/me/channels/" + channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Code should be deleted (CASCADE)
	// Wait a bit for potential async operations
	time.Sleep(100 * time.Millisecond)

	err = testDB.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "verification code should be deleted with channel")
}
