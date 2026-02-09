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
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Events []interface{} `json:"events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	assert.NotNil(t, result.Data.Events, "events should be present")
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
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET /events should be public")

	var eventsList struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventsList)
	assert.NotNil(t, eventsList.Data, "data should be array")

	// GET /events/{id} — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET /events/{id} should be public")

	var eventDetail struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventDetail)
	assert.Equal(t, eventID, eventDetail.Data.ID)

	// GET /events/{id}/updates — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events/" + eventID + "/updates")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET /events/{id}/updates should be public")

	var updates struct {
		Data []interface{} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &updates)
	assert.NotNil(t, updates.Data, "updates should be array")

	// GET /events/{id}/changes — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET /events/{id}/changes should be public")

	var changes struct {
		Data []interface{} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changes)
	assert.NotNil(t, changes.Data, "changes should be array")

	// Cleanup (as admin)
	client.LoginAsAdmin(t)
	deleteEvent(t, client, eventID)
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
	deleteEvent(t, client, eventID)
}
