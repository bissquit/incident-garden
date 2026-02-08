//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvents_PublicStatus(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.GET("/api/v1/status")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestEvents_PublicRead_NoAuth(t *testing.T) {
	// Create an event first (as operator)
	client := newTestClient(t)
	client.LoginAsOperator(t)

	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Public Read Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing public read access",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &createResult)
	eventID := createResult.Data.ID

	// Now test public access (no auth)
	publicClient := newTestClient(t)

	// GET /events — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /events should be public")
	resp.Body.Close()

	// GET /events/{id} — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /events/{id} should be public")
	resp.Body.Close()

	// GET /events/{id}/updates — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events/" + eventID + "/updates")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /events/{id}/updates should be public")
	resp.Body.Close()

	// GET /events/{id}/changes — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /events/{id}/changes should be public")
	resp.Body.Close()

	// Cleanup (as admin)
	client.LoginAsAdmin(t)
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
}

func TestEvents_WriteOperations_RequireAuth(t *testing.T) {
	client := newTestClient(t)

	// POST /events without auth — should be 401
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Unauthorized Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "POST /events should require auth")
	resp.Body.Close()

	// POST /events with user role — should be 403
	client.LoginAsUser(t)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Forbidden Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "POST /events should require operator role")
	resp.Body.Close()

	// POST /events with operator role — should be 201
	client.LoginAsOperator(t)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Authorized Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Should succeed",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "POST /events should succeed for operator")

	var createResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &createResult)
	eventID := createResult.Data.ID

	// Cleanup
	client.LoginAsAdmin(t)
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
}
