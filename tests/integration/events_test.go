//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvents_Incident_Lifecycle(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
	client.LoginAsOperator(t)

	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Test Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "major",
		"description": "Test incident description",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResult struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Type   string `json:"type"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &createResult)
	assert.Equal(t, "investigating", createResult.Data.Status)
	assert.Equal(t, "incident", createResult.Data.Type)
	eventID := createResult.Data.ID

	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Root cause identified",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var getResult struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &getResult)
	assert.Equal(t, "identified", getResult.Data.Status)

	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Issue resolved",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)

	var resolvedResult struct {
		Data struct {
			Status     string  `json:"status"`
			ResolvedAt *string `json:"resolved_at"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &resolvedResult)
	assert.Equal(t, "resolved", resolvedResult.Data.Status)
	assert.NotNil(t, resolvedResult.Data.ResolvedAt)
}

func TestEvents_Maintenance_Lifecycle(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
	client.LoginAsOperator(t)

	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":              "Scheduled Maintenance",
		"type":               "maintenance",
		"status":             "scheduled",
		"description":        "Planned database upgrade",
		"scheduled_start_at": "2030-01-20T02:00:00Z",
		"scheduled_end_at":   "2030-01-20T04:00:00Z",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResult struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Type   string `json:"type"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &createResult)
	assert.Equal(t, "scheduled", createResult.Data.Status)
	assert.Equal(t, "maintenance", createResult.Data.Type)
	eventID := createResult.Data.ID

	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "in_progress",
		"message": "Maintenance started",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "completed",
		"message": "Maintenance completed",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

func TestEvents_InvalidStatusForType(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
	client.LoginAsOperator(t)

	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Invalid Incident",
		"type":        "incident",
		"status":      "scheduled",
		"severity":    "minor",
		"description": "Test",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Invalid Maintenance",
		"type":        "maintenance",
		"status":      "investigating",
		"description": "Test",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestEvents_PublicStatus(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)

	resp, err := client.GET("/api/v1/status")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
