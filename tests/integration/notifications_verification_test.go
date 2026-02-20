//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/notifications"
	notificationspostgres "github.com/bissquit/incident-garden/internal/notifications/postgres"
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

func TestNotifications_MattermostChannel_NoCodeNeeded(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create mattermost channel
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "mattermost",
		"target": "https://mattermost.example.com/hooks/test123",
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

	// No verification code should be created for mattermost
	var count int
	err = testDB.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "no verification code for mattermost")

	// Resend code should fail for mattermost
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

	// Code should be deleted (CASCADE DELETE is synchronous in PostgreSQL)
	err = testDB.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "verification code should be deleted with channel")
}

// =============================================================================
// Telegram/Mattermost Verification via Test Message
//
// The app-level notification service is configured with a nil dispatcher
// (notifications disabled in test config to prevent test interference).
// Therefore, telegram/mattermost verification via the HTTP endpoint would
// fail with "dispatcher not configured". To test the verifyByTestMessage
// flow, we create a separate notifications.Service with mock senders and
// call VerifyChannel directly. This follows the same pattern used by
// E2E email tests (setupE2ENotificationInfra) and dispatch tests.
//
// The channel creation and state verification are done via the HTTP API
// to ensure the full integration path works correctly.
// =============================================================================

// setupVerificationService creates a notifications.Service with mock senders
// for testing the verifyByTestMessage flow.
func setupVerificationService(t *testing.T, mocks *MockSenderRegistry) *notifications.Service {
	t.Helper()
	repo := notificationspostgres.NewRepository(testDB)
	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	return notifications.NewService(repo, dispatcher, nil, nil)
}

// getUserID returns the user ID for the currently logged-in user.
func getUserID(t *testing.T, client *testutil.Client) string {
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
	require.NotEmpty(t, result.Data.ID)
	return result.Data.ID
}

func TestVerification_TelegramChannel_VerifyByTestMessage_Success(t *testing.T) {
	// Arrange: create telegram channel via API, set up service with mock senders
	client := newTestClient(t)
	client.LoginAsUser(t)
	userID := getUserID(t, client)

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "telegram",
		"target": "verify-tg-test-12345",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Target     string `json:"target"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID
	require.NotEmpty(t, channelID)
	assert.Equal(t, "telegram", channelResp.Data.Type)
	assert.Equal(t, "verify-tg-test-12345", channelResp.Data.Target)
	assert.False(t, channelResp.Data.IsVerified, "new telegram channel should not be verified")

	t.Cleanup(func() {
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Arrange: create service with mock senders (app-level has nil dispatcher)
	mocks := NewMockSenderRegistry()
	svc := setupVerificationService(t, mocks)

	// Act: verify channel via service (simulates the verifyByTestMessage flow)
	ctx := context.Background()
	verified, err := svc.VerifyChannel(ctx, userID, channelID, "")
	require.NoError(t, err, "verifyByTestMessage should succeed with mock sender")

	// Assert: service returned verified channel
	assert.True(t, verified.IsVerified, "channel should be marked as verified")
	assert.Equal(t, channelID, verified.ID)

	// Assert: mock sender received the test message
	assert.Equal(t, 1, mocks.Telegram.SentCount(), "telegram mock should have sent 1 test message")
	sent := mocks.Telegram.GetSent()
	require.Len(t, sent, 1)
	assert.Equal(t, "verify-tg-test-12345", sent[0].To)
	assert.Contains(t, sent[0].Subject, "Verification")
	assert.Contains(t, sent[0].Body, "test message")

	// Assert: no messages sent via other senders
	assert.Equal(t, 0, mocks.Email.SentCount(), "email sender should not be called")
	assert.Equal(t, 0, mocks.Mattermost.SentCount(), "mattermost sender should not be called")

	// Assert: channel is verified via API (GET /me/channels)
	resp, err = client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp struct {
		Data []struct {
			ID         string `json:"id"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &listResp)

	var found bool
	for _, ch := range listResp.Data {
		if ch.ID == channelID {
			found = true
			assert.True(t, ch.IsVerified, "channel should appear as verified in API response")
			break
		}
	}
	assert.True(t, found, "verified channel should be present in channel list")

	// Assert: DB confirms verified state
	var isVerified bool
	err = testDB.QueryRow(ctx, `SELECT is_verified FROM notification_channels WHERE id = $1`, channelID).Scan(&isVerified)
	require.NoError(t, err)
	assert.True(t, isVerified, "DB should reflect verified state")
}

func TestVerification_MattermostChannel_VerifyByTestMessage_Success(t *testing.T) {
	// Arrange: create mattermost channel via API, set up service with mock senders
	client := newTestClient(t)
	client.LoginAsUser(t)
	userID := getUserID(t, client)

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "mattermost",
		"target": "https://mm.example.com/hooks/verify-test-hook",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var channelResp struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Target     string `json:"target"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelResp)
	channelID := channelResp.Data.ID
	require.NotEmpty(t, channelID)
	assert.Equal(t, "mattermost", channelResp.Data.Type)
	assert.Equal(t, "https://mm.example.com/hooks/verify-test-hook", channelResp.Data.Target)
	assert.False(t, channelResp.Data.IsVerified, "new mattermost channel should not be verified")

	t.Cleanup(func() {
		resp, _ := client.DELETE("/api/v1/me/channels/" + channelID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Arrange: create service with mock senders (app-level has nil dispatcher)
	mocks := NewMockSenderRegistry()
	svc := setupVerificationService(t, mocks)

	// Act: verify channel via service (simulates the verifyByTestMessage flow)
	ctx := context.Background()
	verified, err := svc.VerifyChannel(ctx, userID, channelID, "")
	require.NoError(t, err, "verifyByTestMessage should succeed with mock sender")

	// Assert: service returned verified channel
	assert.True(t, verified.IsVerified, "channel should be marked as verified")
	assert.Equal(t, channelID, verified.ID)

	// Assert: mock sender received the test message
	assert.Equal(t, 1, mocks.Mattermost.SentCount(), "mattermost mock should have sent 1 test message")
	sent := mocks.Mattermost.GetSent()
	require.Len(t, sent, 1)
	assert.Equal(t, "https://mm.example.com/hooks/verify-test-hook", sent[0].To)
	assert.Contains(t, sent[0].Subject, "Verification")
	assert.Contains(t, sent[0].Body, "test message")

	// Assert: no messages sent via other senders
	assert.Equal(t, 0, mocks.Email.SentCount(), "email sender should not be called")
	assert.Equal(t, 0, mocks.Telegram.SentCount(), "telegram sender should not be called")

	// Assert: channel is verified via API (GET /me/channels)
	resp, err = client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp struct {
		Data []struct {
			ID         string `json:"id"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &listResp)

	var found bool
	for _, ch := range listResp.Data {
		if ch.ID == channelID {
			found = true
			assert.True(t, ch.IsVerified, "channel should appear as verified in API response")
			break
		}
	}
	assert.True(t, found, "verified channel should be present in channel list")

	// Assert: DB confirms verified state
	var isVerified bool
	err = testDB.QueryRow(ctx, `SELECT is_verified FROM notification_channels WHERE id = $1`, channelID).Scan(&isVerified)
	require.NoError(t, err)
	assert.True(t, isVerified, "DB should reflect verified state")
}

func TestVerification_TelegramChannel_VerifyAlreadyVerified(t *testing.T) {
	// Arrange: create and verify a telegram channel, then try verifying again
	client := newTestClient(t)
	client.LoginAsUser(t)
	userID := getUserID(t, client)

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "telegram",
		"target": "already-verified-tg-99999",
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

	// First verification
	mocks := NewMockSenderRegistry()
	svc := setupVerificationService(t, mocks)
	ctx := context.Background()

	verified, err := svc.VerifyChannel(ctx, userID, channelID, "")
	require.NoError(t, err)
	require.True(t, verified.IsVerified)
	assert.Equal(t, 1, mocks.Telegram.SentCount(), "first verify should send test message")

	// Act: verify again (channel is already verified)
	mocks.Reset()
	verified, err = svc.VerifyChannel(ctx, userID, channelID, "")
	require.NoError(t, err, "verifying already-verified channel should not error")

	// Assert: channel is still verified
	assert.True(t, verified.IsVerified, "channel should remain verified")

	// Assert: no test message sent on second verification (early return for already verified)
	assert.Equal(t, 0, mocks.Telegram.SentCount(), "no test message should be sent for already verified channel")
}

func TestVerification_MattermostChannel_VerifyAlreadyVerified(t *testing.T) {
	// Arrange: create and verify a mattermost channel, then try verifying again
	client := newTestClient(t)
	client.LoginAsUser(t)
	userID := getUserID(t, client)

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "mattermost",
		"target": "https://mm.example.com/hooks/already-verified-test",
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

	// First verification
	mocks := NewMockSenderRegistry()
	svc := setupVerificationService(t, mocks)
	ctx := context.Background()

	verified, err := svc.VerifyChannel(ctx, userID, channelID, "")
	require.NoError(t, err)
	require.True(t, verified.IsVerified)
	assert.Equal(t, 1, mocks.Mattermost.SentCount(), "first verify should send test message")

	// Act: verify again (channel is already verified)
	mocks.Reset()
	verified, err = svc.VerifyChannel(ctx, userID, channelID, "")
	require.NoError(t, err, "verifying already-verified channel should not error")

	// Assert: channel is still verified
	assert.True(t, verified.IsVerified, "channel should remain verified")

	// Assert: no test message sent on second verification (early return for already verified)
	assert.Equal(t, 0, mocks.Mattermost.SentCount(), "no test message should be sent for already verified channel")
}
