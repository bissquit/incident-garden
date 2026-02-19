//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister_CreatesDefaultEmailChannel(t *testing.T) {
	client := newTestClient(t)

	// Register new user
	email := testutil.RandomEmail()
	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Login
	client.LoginAs(t, email, "password123")

	// Check channels
	resp, err = client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data []struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Target     string `json:"target"`
			IsVerified bool   `json:"is_verified"`
			IsEnabled  bool   `json:"is_enabled"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	// Assert: one email channel exists, verified
	require.Len(t, result.Data, 1)
	assert.Equal(t, "email", result.Data[0].Type)
	assert.Equal(t, email, result.Data[0].Target)
	assert.True(t, result.Data[0].IsVerified)
	assert.True(t, result.Data[0].IsEnabled)
}

func TestCreateChannel_DuplicateEmail_Conflict(t *testing.T) {
	client := newTestClient(t)

	// Register and login
	email := testutil.RandomEmail()
	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	client.LoginAs(t, email, "password123")

	// Try to create channel with same email
	resp, err = client.POST("/api/v1/me/channels", map[string]string{
		"type":   "email",
		"target": email,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()
}

func TestCreateChannel_DifferentEmail_RequiresVerification(t *testing.T) {
	client := newTestClient(t)

	// Register and login
	email := testutil.RandomEmail()
	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	client.LoginAs(t, email, "password123")

	// Create channel with different email
	differentEmail := testutil.RandomEmail()
	resp, err = client.POST("/api/v1/me/channels", map[string]string{
		"type":   "email",
		"target": differentEmail,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID         string `json:"id"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	// Assert: channel created but NOT verified
	assert.NotEmpty(t, result.Data.ID)
	assert.False(t, result.Data.IsVerified)
}

func TestDefaultChannel_CanSetSubscriptions(t *testing.T) {
	client := newTestClient(t)

	// Setup: create service
	client.LoginAsAdmin(t)
	serviceID, serviceSlug := createTestService(t, client, "test-service")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Register new user
	email := testutil.RandomEmail()
	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	client.LoginAs(t, email, "password123")

	// Get default channel
	resp, err = client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var channelsResult struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelsResult)
	require.Len(t, channelsResult.Data, 1)
	channelID := channelsResult.Data[0].ID

	// Set subscriptions — should work without additional verification
	resp, err = client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               []string{serviceID},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestMigration_ExistingUsersHaveDefaultChannels(t *testing.T) {
	// Test that pre-seeded users got default email channels from migration
	testCases := []struct {
		name     string
		email    string
		password string
	}{
		{"admin", "admin@example.com", "admin123"},
		{"operator", "operator@example.com", "admin123"},
		{"user", "user@example.com", "user123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t)
			client.LoginAs(t, tc.email, tc.password)

			resp, err := client.GET("/api/v1/me/channels")
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var result struct {
				Data []struct {
					Type       string `json:"type"`
					Target     string `json:"target"`
					IsVerified bool   `json:"is_verified"`
					IsEnabled  bool   `json:"is_enabled"`
				} `json:"data"`
			}
			testutil.DecodeJSON(t, resp, &result)

			// Find default email channel
			var foundDefaultChannel bool
			for _, ch := range result.Data {
				if ch.Type == "email" && ch.Target == tc.email {
					assert.True(t, ch.IsVerified, "default channel should be verified")
					assert.True(t, ch.IsEnabled, "default channel should be enabled")
					foundDefaultChannel = true
					break
				}
			}
			assert.True(t, foundDefaultChannel, "pre-seeded user should have default email channel")
		})
	}
}

func TestDefaultChannel_SubscribeToAll(t *testing.T) {
	client := newTestClient(t)

	// Register new user
	email := testutil.RandomEmail()
	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	client.LoginAs(t, email, "password123")

	// Get default channel
	resp, err = client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var channelsResult struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelsResult)
	require.Len(t, channelsResult.Data, 1)
	channelID := channelsResult.Data[0].ID

	// Cleanup: reset subscriptions to neutral state before attempting delete.
	// Default channels can't be deleted, so we must reset subscriptions
	// to prevent this channel from receiving notifications in other tests.
	t.Cleanup(func() {
		client.LoginAs(t, email, "password123")
		resetResp, resetErr := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
			"subscribe_to_all_services": false,
			"service_ids":               []string{},
		})
		if resetErr != nil {
			t.Logf("cleanup warning: failed to reset subscriptions for channel %s: %v", channelID, resetErr)
		} else {
			resetResp.Body.Close()
		}
		deleteChannel(t, client, channelID)
	})

	// Set subscribe to all
	resp, err = client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": true,
		"service_ids":               []string{},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var subsResult struct {
		Data struct {
			SubscribeToAllServices bool `json:"subscribe_to_all_services"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &subsResult)
	assert.True(t, subsResult.Data.SubscribeToAllServices)
}
