//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Create Channel Tests
// =============================================================================

func TestChannels_Create_Email_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "create-email-test@example.com",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Target     string `json:"target"`
			IsEnabled  bool   `json:"is_enabled"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.NotEmpty(t, result.Data.ID)
	assert.Equal(t, "email", result.Data.Type)
	assert.Equal(t, "create-email-test@example.com", result.Data.Target)
	assert.True(t, result.Data.IsEnabled)
	assert.False(t, result.Data.IsVerified)

	t.Cleanup(func() { deleteChannel(t, client, result.Data.ID) })
}

func TestChannels_Create_Telegram_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "telegram",
		"target": "987654321",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Target     string `json:"target"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.NotEmpty(t, result.Data.ID)
	assert.Equal(t, "telegram", result.Data.Type)
	assert.Equal(t, "987654321", result.Data.Target)
	assert.False(t, result.Data.IsVerified)

	t.Cleanup(func() { deleteChannel(t, client, result.Data.ID) })
}

func TestChannels_Create_Mattermost_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "mattermost",
		"target": "https://mattermost.example.com/hooks/abc123",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Target     string `json:"target"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.NotEmpty(t, result.Data.ID)
	assert.Equal(t, "mattermost", result.Data.Type)
	assert.Equal(t, "https://mattermost.example.com/hooks/abc123", result.Data.Target)
	assert.False(t, result.Data.IsVerified)

	t.Cleanup(func() { deleteChannel(t, client, result.Data.ID) })
}

func TestChannels_Create_InvalidType_BadRequest(t *testing.T) {
	client := newTestClientWithoutValidation()
	client.LoginAsUser(t)

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "invalid_type",
		"target": "test@example.com",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestChannels_Create_MissingTarget_BadRequest(t *testing.T) {
	client := newTestClientWithoutValidation()
	client.LoginAsUser(t)

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type": "email",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestChannels_Create_RequiresAuth(t *testing.T) {
	client := newTestClient(t)
	// No login

	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "noauth@example.com",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// =============================================================================
// List Channels Tests
// =============================================================================

func TestChannels_List_ReturnsUserChannelsOnly(t *testing.T) {
	// User 1 creates a channel
	client1 := newTestClient(t)
	client1.LoginAsUser(t)

	channelID := createEmailChannel(t, client1)
	t.Cleanup(func() {
		client1.LoginAsUser(t)
		deleteChannel(t, client1, channelID)
	})

	// User 2 should not see User 1's channel
	client2 := newTestClient(t)
	registerAndLoginUser(t, client2, "list-other-user")

	resp, err := client2.GET("/api/v1/me/channels")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	// User 2 should have no channels (or only their own)
	for _, ch := range result.Data {
		assert.NotEqual(t, channelID, ch.ID, "should not see other user's channel")
	}
}

func TestChannels_List_ReturnsArrayNotNull(t *testing.T) {
	client := newTestClient(t)
	registerAndLoginUser(t, client, "list-array")

	resp, err := client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data []interface{} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.NotNil(t, result.Data, "data should be array, not null")
	// New users now get a default email channel, so it won't be empty
	assert.Len(t, result.Data, 1, "new user should have exactly one default channel")
}

func TestChannels_List_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create two channels
	channelID1 := createEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID1) })

	resp2, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "telegram",
		"target": "123456",
	})
	require.NoError(t, err)
	var ch2 struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp2, &ch2)
	channelID2 := ch2.Data.ID
	t.Cleanup(func() { deleteChannel(t, client, channelID2) })

	// List channels
	resp, err := client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.GreaterOrEqual(t, len(result.Data), 2)

	// Verify both channels are present
	ids := make(map[string]bool)
	for _, ch := range result.Data {
		ids[ch.ID] = true
	}
	assert.True(t, ids[channelID1], "should contain email channel")
	assert.True(t, ids[channelID2], "should contain telegram channel")
}

// =============================================================================
// Update Channel Tests
// =============================================================================

func TestChannels_Update_Disable_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	channelID := createEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	// Disable channel
	resp, err := client.PATCH("/api/v1/me/channels/"+channelID, map[string]interface{}{
		"is_enabled": false,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			ID        string `json:"id"`
			IsEnabled bool   `json:"is_enabled"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, channelID, result.Data.ID)
	assert.False(t, result.Data.IsEnabled)
}

func TestChannels_Update_Enable_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	channelID := createEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	// First disable
	resp, err := client.PATCH("/api/v1/me/channels/"+channelID, map[string]interface{}{
		"is_enabled": false,
	})
	require.NoError(t, err)
	resp.Body.Close()

	// Then enable
	resp, err = client.PATCH("/api/v1/me/channels/"+channelID, map[string]interface{}{
		"is_enabled": true,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			ID        string `json:"id"`
			IsEnabled bool   `json:"is_enabled"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.True(t, result.Data.IsEnabled)
}

func TestChannels_Update_OtherUserChannel_Forbidden(t *testing.T) {
	// User 1 creates a channel
	client1 := newTestClient(t)
	client1.LoginAsUser(t)

	channelID := createEmailChannel(t, client1)
	t.Cleanup(func() {
		client1.LoginAsUser(t)
		deleteChannel(t, client1, channelID)
	})

	// User 2 tries to update
	client2 := newTestClient(t)
	registerAndLoginUser(t, client2, "update-other")

	resp, err := client2.PATCH("/api/v1/me/channels/"+channelID, map[string]interface{}{
		"is_enabled": false,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestChannels_Update_NotFound(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	resp, err := client.PATCH("/api/v1/me/channels/00000000-0000-0000-0000-000000000000", map[string]interface{}{
		"is_enabled": false,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// =============================================================================
// Delete Channel Tests
// =============================================================================

func TestChannels_Delete_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	channelID := createEmailChannel(t, client)

	resp, err := client.DELETE("/api/v1/me/channels/" + channelID)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify deleted - should not appear in list
	resp, err = client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	for _, ch := range result.Data {
		assert.NotEqual(t, channelID, ch.ID)
	}
}

func TestChannels_Delete_OtherUserChannel_Forbidden(t *testing.T) {
	// User 1 creates a channel
	client1 := newTestClient(t)
	client1.LoginAsUser(t)

	channelID := createEmailChannel(t, client1)
	t.Cleanup(func() {
		client1.LoginAsUser(t)
		deleteChannel(t, client1, channelID)
	})

	// User 2 tries to delete
	client2 := newTestClient(t)
	registerAndLoginUser(t, client2, "delete-other")

	resp, err := client2.DELETE("/api/v1/me/channels/" + channelID)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestChannels_Delete_NotFound(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	resp, err := client.DELETE("/api/v1/me/channels/00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestChannels_Delete_DefaultChannel_Conflict(t *testing.T) {
	client := newTestClient(t)

	// Register a new user (creates default email channel automatically)
	email := testutil.RandomEmail()
	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Login as new user
	client.LoginAs(t, email, "password123")

	// Get channels, find the default one
	resp, err = client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var channelsResult struct {
		Data []struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			Target    string `json:"target"`
			IsDefault bool   `json:"is_default"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &channelsResult)
	require.Len(t, channelsResult.Data, 1, "new user should have exactly one default channel")
	assert.True(t, channelsResult.Data[0].IsDefault, "channel should be marked as default")
	assert.Equal(t, "email", channelsResult.Data[0].Type)
	assert.Equal(t, email, channelsResult.Data[0].Target)

	defaultChannelID := channelsResult.Data[0].ID

	// Try to DELETE the default channel — should be rejected
	resp, err = client.DELETE("/api/v1/me/channels/" + defaultChannelID)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var errorResult struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errorResult)
	assert.Contains(t, errorResult.Error.Message, "cannot delete default channel")

	// Verify channel still exists
	resp, err = client.GET("/api/v1/me/channels")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var afterResult struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &afterResult)
	assert.Len(t, afterResult.Data, 1, "channel count should be unchanged after failed delete")
	assert.Equal(t, defaultChannelID, afterResult.Data[0].ID, "default channel should still exist")
}

func TestChannels_Delete_WithSubscriptions_CascadeDeletes(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	serviceID, serviceSlug := createTestService(t, client, "delete-cascade-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create and verify channel
	channelID := createAndVerifyEmailChannel(t, client)

	// Set subscriptions
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               []string{serviceID},
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Delete channel
	resp, err = client.DELETE("/api/v1/me/channels/" + channelID)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Channel should be gone - no error verifying cascade worked
}
