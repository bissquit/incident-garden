//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMaintenance_ScheduledAffectsStatus verifies that scheduled maintenance
// DOES affect effective_status (current behavior - scheduled is treated as active event).
// This tests the full maintenance lifecycle: scheduled → in_progress → completed.
func TestMaintenance_ScheduledAffectsStatus(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "scheduled-maint-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Verify service is operational
	status := getServiceEffectiveStatus(t, client, serviceSlug)
	require.Equal(t, "operational", status)

	// Act: Create maintenance with scheduled status
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":              "Scheduled Maintenance Test",
		"type":               "maintenance",
		"status":             "scheduled",
		"description":        "Planned maintenance window",
		"scheduled_start_at": "2099-01-01T00:00:00Z",
		"scheduled_end_at":   "2099-01-01T04:00:00Z",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "maintenance"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var eventResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventResult)
	eventID := eventResult.Data.ID
	t.Cleanup(func() {
		// Complete and delete
		client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
			"status": "completed", "message": "Done",
		})
		deleteEvent(t, client, eventID)
	})

	// Assert: effective_status = maintenance (scheduled is treated as active)
	// Note: Current implementation treats scheduled maintenance as active event
	status = getServiceEffectiveStatus(t, client, serviceSlug)
	assert.Equal(t, "maintenance", status,
		"scheduled maintenance affects effective_status (current behavior)")

	// Act: Transition to in_progress
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "in_progress",
		"message": "Starting maintenance",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Assert: effective_status still maintenance
	status = getServiceEffectiveStatus(t, client, serviceSlug)
	assert.Equal(t, "maintenance", status,
		"in_progress maintenance keeps effective_status as maintenance")
}

// TestMaintenance_CreateWithInProgress verifies that maintenance can be created
// directly with status=in_progress, bypassing the scheduled state (Scenario B2).
func TestMaintenance_CreateWithInProgress(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "immediate-maint-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Verify operational
	assert.Equal(t, "operational", getServiceEffectiveStatus(t, client, serviceSlug))

	// Act: Create maintenance directly in in_progress
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Immediate Maintenance",
		"type":        "maintenance",
		"status":      "in_progress",
		"description": "Emergency maintenance started immediately",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "maintenance"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode,
		"should allow creating maintenance with in_progress status")

	var eventResult struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventResult)
	eventID := eventResult.Data.ID
	t.Cleanup(func() {
		client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
			"status": "completed", "message": "Done",
		})
		deleteEvent(t, client, eventID)
	})

	// Assert: event status = in_progress
	assert.Equal(t, "in_progress", eventResult.Data.Status)

	// Assert: service effective_status = maintenance (immediately applied)
	assert.Equal(t, "maintenance", getServiceEffectiveStatus(t, client, serviceSlug),
		"in_progress maintenance should immediately affect service status")
}

func TestEvents_Maintenance_Lifecycle(t *testing.T) {
	client := newTestClient(t)
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
