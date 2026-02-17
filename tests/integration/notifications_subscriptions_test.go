//go:build integration

package integration

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptions_GetMatrix_HasDefaultChannel(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	resp, err := client.GET("/api/v1/me/subscriptions")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Channels []interface{} `json:"channels"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	// Pre-seeded users now have default email channel from migration
	assert.NotEmpty(t, result.Data.Channels)
}

func TestSubscriptions_GetMatrix_WithChannel(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create and verify channel
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	resp, err := client.GET("/api/v1/me/subscriptions")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Channels []struct {
				Channel struct {
					ID         string `json:"id"`
					Type       string `json:"type"`
					IsVerified bool   `json:"is_verified"`
				} `json:"channel"`
				SubscribeToAllServices bool     `json:"subscribe_to_all_services"`
				SubscribedServiceIDs   []string `json:"subscribed_service_ids"`
			} `json:"channels"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	// Find our created channel among all channels
	var foundChannel bool
	for _, ch := range result.Data.Channels {
		if ch.Channel.ID == channelID {
			assert.Equal(t, "email", ch.Channel.Type)
			assert.True(t, ch.Channel.IsVerified)
			assert.False(t, ch.SubscribeToAllServices)
			assert.Empty(t, ch.SubscribedServiceIDs)
			foundChannel = true
			break
		}
	}
	assert.True(t, foundChannel, "created channel should be in matrix")
}

func TestSubscriptions_SetChannelSubscriptions_AllServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	// Subscribe to all services
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": true,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			ChannelID              string   `json:"channel_id"`
			SubscribeToAllServices bool     `json:"subscribe_to_all_services"`
			SubscribedServiceIDs   []string `json:"subscribed_service_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, channelID, result.Data.ChannelID)
	assert.True(t, result.Data.SubscribeToAllServices)
	assert.Empty(t, result.Data.SubscribedServiceIDs)
}

func TestSubscriptions_SetChannelSubscriptions_SpecificServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create services
	serviceID1, slug1 := createTestService(t, client, "sub-svc-1")
	t.Cleanup(func() { deleteService(t, client, slug1) })
	serviceID2, slug2 := createTestService(t, client, "sub-svc-2")
	t.Cleanup(func() { deleteService(t, client, slug2) })

	// Create and verify channel
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	// Subscribe to specific services
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               []string{serviceID1, serviceID2},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			ChannelID              string   `json:"channel_id"`
			SubscribeToAllServices bool     `json:"subscribe_to_all_services"`
			SubscribedServiceIDs   []string `json:"subscribed_service_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Equal(t, channelID, result.Data.ChannelID)
	assert.False(t, result.Data.SubscribeToAllServices)
	assert.Len(t, result.Data.SubscribedServiceIDs, 2)
	assert.Contains(t, result.Data.SubscribedServiceIDs, serviceID1)
	assert.Contains(t, result.Data.SubscribedServiceIDs, serviceID2)
}

func TestSubscriptions_SetChannelSubscriptions_UnverifiedChannel_Fails(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create channel (not verified)
	channelID := createEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": true,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscriptions_SetChannelSubscriptions_AllWithServiceIDs_Fails(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "sub-svc-conflict")
	t.Cleanup(func() { deleteService(t, client, slug) })

	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	// Both subscribe_to_all_services and service_ids
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": true,
		"service_ids":               []string{serviceID},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscriptions_SetChannelSubscriptions_NonexistentService_Fails(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               []string{"00000000-0000-0000-0000-000000000000"},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscriptions_SetChannelSubscriptions_ChannelNotOwned_Fails(t *testing.T) {
	client1 := newTestClient(t)
	client1.LoginAsUser(t)

	// User1 creates a channel
	channelID := createAndVerifyEmailChannel(t, client1)
	t.Cleanup(func() {
		client1.LoginAsUser(t)
		deleteChannel(t, client1, channelID)
	})

	// User2 tries to set subscriptions on User1's channel
	client2 := newTestClient(t)
	registerAndLoginUser(t, client2, "subscriptions-other")

	resp, err := client2.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": true,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSubscriptions_SetChannelSubscriptions_ChannelNotFound_Fails(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	resp, err := client.PUT("/api/v1/me/channels/00000000-0000-0000-0000-000000000000/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": true,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSubscriptions_UpdateReplacesPrevious(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create services
	serviceID1, slug1 := createTestService(t, client, "replace-svc-1")
	t.Cleanup(func() { deleteService(t, client, slug1) })
	serviceID2, slug2 := createTestService(t, client, "replace-svc-2")
	t.Cleanup(func() { deleteService(t, client, slug2) })

	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	// First: subscribe to service 1
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               []string{serviceID1},
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Second: subscribe to service 2 only (replaces service 1)
	resp, err = client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               []string{serviceID2},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			SubscribedServiceIDs []string `json:"subscribed_service_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.Len(t, result.Data.SubscribedServiceIDs, 1)
	assert.Equal(t, serviceID2, result.Data.SubscribedServiceIDs[0])
}

func TestSubscriptions_GetMatrix_IncludesBothVerifiedAndUnverified(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsUser(t)

	// Create unverified channel
	unverifiedID := createEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, unverifiedID) })

	// Create and verify another channel
	verifiedID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, verifiedID) })

	// Get matrix - should contain both created channels (plus any default channels)
	resp, err := client.GET("/api/v1/me/subscriptions")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Channels []struct {
				Channel struct {
					ID         string `json:"id"`
					IsVerified bool   `json:"is_verified"`
				} `json:"channel"`
			} `json:"channels"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	// Should have at least both channels we created
	require.GreaterOrEqual(t, len(result.Data.Channels), 2)

	// Find channels by ID
	var foundVerified, foundUnverified bool
	for _, ch := range result.Data.Channels {
		if ch.Channel.ID == verifiedID {
			assert.True(t, ch.Channel.IsVerified)
			foundVerified = true
		}
		if ch.Channel.ID == unverifiedID {
			assert.False(t, ch.Channel.IsVerified)
			foundUnverified = true
		}
	}
	assert.True(t, foundVerified, "verified channel should be in matrix")
	assert.True(t, foundUnverified, "unverified channel should also be in matrix")
}

func TestSubscriptions_SubscribeToAll_IncludesNewServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create and verify channel with subscribe_to_all
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	// Subscribe to all services
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": true,
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Create a new service AFTER subscription was set
	serviceID, serviceSlug := createTestService(t, client, "new-service-after-sub")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Get matrix and verify channel still has subscribe_to_all_services: true
	// This means it will receive notifications for the new service
	resp, err = client.GET("/api/v1/me/subscriptions")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Channels []struct {
				Channel struct {
					ID string `json:"id"`
				} `json:"channel"`
				SubscribeToAllServices bool `json:"subscribe_to_all_services"`
			} `json:"channels"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	// Find our channel and verify subscribe_to_all is still true
	var found bool
	for _, ch := range result.Data.Channels {
		if ch.Channel.ID == channelID {
			assert.True(t, ch.SubscribeToAllServices, "should still be subscribed to all")
			found = true
			break
		}
	}
	assert.True(t, found, "channel should be in matrix")
	_ = serviceID // Used to ensure service was created
}

func TestSubscriptions_SubscribeToAll_OverridesServiceList(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "override-test-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create and verify channel
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	// First subscribe to specific services
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               []string{serviceID},
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Now switch to subscribe_to_all
	resp, err = client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": true,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			SubscribeToAllServices bool     `json:"subscribe_to_all_services"`
			SubscribedServiceIDs   []string `json:"subscribed_service_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	// subscribe_to_all should override service_ids
	assert.True(t, result.Data.SubscribeToAllServices)
	assert.Empty(t, result.Data.SubscribedServiceIDs, "service_ids should be cleared when subscribing to all")
}

func TestSubscriptions_ClearSubscriptions(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "clear-sub-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create and verify channel
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	// Subscribe to specific services
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               []string{serviceID},
	})
	require.NoError(t, err)
	resp.Body.Close()

	// Clear subscriptions (empty service_ids, not subscribe to all)
	resp, err = client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               []string{},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			SubscribeToAllServices bool     `json:"subscribe_to_all_services"`
			SubscribedServiceIDs   []string `json:"subscribed_service_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.False(t, result.Data.SubscribeToAllServices)
	assert.Empty(t, result.Data.SubscribedServiceIDs)
}

// Helper functions

func randomSuffix() string {
	return fmt.Sprintf("%d", rand.Intn(100000))
}

func createEmailChannel(t *testing.T, client *testutil.Client) string {
	t.Helper()
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "email",
		"target": "test-" + randomSuffix() + "@example.com",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.ID
}

func getVerificationCode(t *testing.T, channelID string) string {
	t.Helper()
	var code string
	err := testDB.QueryRow(context.Background(), `
		SELECT code FROM channel_verification_codes WHERE channel_id = $1
	`, channelID).Scan(&code)
	require.NoError(t, err, "verification code should exist in DB")
	return code
}

func createAndVerifyEmailChannel(t *testing.T, client *testutil.Client) string {
	t.Helper()
	channelID := createEmailChannel(t, client)

	// Get verification code from DB
	code := getVerificationCode(t, channelID)

	// Verify channel
	resp, err := client.POST("/api/v1/me/channels/"+channelID+"/verify", map[string]interface{}{
		"code": code,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	return channelID
}

func registerAndLoginUser(t *testing.T, client *testutil.Client, suffix string) {
	t.Helper()
	email := fmt.Sprintf("testuser-%s-%s@example.com", suffix, randomSuffix())
	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func deleteChannel(t *testing.T, client *testutil.Client, channelID string) {
	t.Helper()
	resp, err := client.DELETE("/api/v1/me/channels/" + channelID)
	if err != nil {
		t.Logf("cleanup warning (channel %s): %v", channelID, err)
		return
	}
	resp.Body.Close()
}
