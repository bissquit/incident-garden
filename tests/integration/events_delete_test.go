//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteEvent_ActiveEvent_Forbidden(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	// Create an active incident
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Active Event Delete Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing cannot delete active event",
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

	// Try to delete as admin - should fail with 409
	client.LoginAsAdmin(t)
	resp, err = client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode, "should not allow deleting active event")

	var errorResult struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errorResult)
	assert.Contains(t, errorResult.Error.Message, "resolve it first")

	// Cleanup: resolve the event first, then delete
	client.LoginAsOperator(t)
	resolveEvent(t, client, eventID)

	client.LoginAsAdmin(t)
	deleteEvent(t, client, eventID)
}

func TestDeleteEvent_ResolvedEvent_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	// Create incident
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Resolved Event Delete Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing delete resolved event",
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

	// Resolve the event
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Issue fixed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Delete as admin
	client.LoginAsAdmin(t)
	resp, err = client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify event is deleted
	resp, err = client.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteEvent_MaintenanceCompleted_Success(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	// Create maintenance
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":              "Maintenance Delete Test",
		"type":               "maintenance",
		"status":             "scheduled",
		"description":        "Testing delete completed maintenance",
		"scheduled_start_at": "2030-01-20T02:00:00Z",
		"scheduled_end_at":   "2030-01-20T04:00:00Z",
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

	// Move to in_progress
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "in_progress",
		"message": "Starting maintenance",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Complete the maintenance
	completeMaintenance(t, client, eventID)

	// Delete as admin
	client.LoginAsAdmin(t)
	resp, err = client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteEvent_RequiresAdmin(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	// Create and resolve incident
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Admin Role Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing admin role requirement",
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

	resolveEvent(t, client, eventID)

	// Try to delete as operator - should fail with 403
	resp, err = client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "operator should not be able to delete events")
	resp.Body.Close()

	// Delete as admin - should succeed
	client.LoginAsAdmin(t)
	resp, err = client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteEvent_NotFound(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	fakeEventID := "00000000-0000-0000-0000-000000000000"
	resp, err := client.DELETE("/api/v1/events/" + fakeEventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteEvent_CascadeDeletesEventUpdates(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	// Create incident
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Cascade Updates Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing cascade delete of updates",
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

	// Add some updates
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Found the issue",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "monitoring",
		"message": "Fix deployed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resolveEvent(t, client, eventID)

	// Verify updates exist
	resp, err = client.GET("/api/v1/events/" + eventID + "/updates")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updatesResult struct {
		Data []interface{} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &updatesResult)
	require.GreaterOrEqual(t, len(updatesResult.Data), 3, "should have at least 3 updates")

	// Delete event
	client.LoginAsAdmin(t)
	resp, err = client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify event is deleted
	resp, err = client.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteEvent_CascadeDeletesEventServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Cascade Service Test")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Cascade Services Test", []AffectedService{
		{ServiceID: serviceID, Status: "major_outage"},
	}, nil)

	// Resolve and delete
	resolveEvent(t, client, eventID)

	resp, err := client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteEvent_CascadeDeletesEventGroups(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupID, groupSlug := createTestGroup(t, client, "Cascade Group Test")
	t.Cleanup(func() { deleteGroup(t, client, groupSlug) })

	_, svcSlug := createTestService(t, client, "Cascade Group Service", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, svcSlug) })

	eventID := createTestIncident(t, client, "Cascade Groups Test", nil, []AffectedGroup{
		{GroupID: groupID, Status: "partial_outage"},
	})

	// Resolve and delete
	resolveEvent(t, client, eventID)

	resp, err := client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteEvent_CascadeDeletesServiceChanges(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	svc1ID, slug1 := createTestService(t, client, "Changes Service 1")
	t.Cleanup(func() { deleteService(t, client, slug1) })

	svc2ID, slug2 := createTestService(t, client, "Changes Service 2")
	t.Cleanup(func() { deleteService(t, client, slug2) })

	eventID := createTestIncident(t, client, "Changes Cascade Test", []AffectedService{
		{ServiceID: svc1ID, Status: "degraded"},
	}, nil)

	// Add another service (creates more change records)
	resp, err := client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Adding more services",
		"add_services": []map[string]interface{}{
			{"service_id": svc2ID, "status": "partial_outage"},
		},
	})
	require.NoError(t, err)
	resp.Body.Close()

	// Verify changes exist
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var changesResult struct {
		Data []interface{} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changesResult)
	require.GreaterOrEqual(t, len(changesResult.Data), 2, "should have at least 2 changes")

	// Resolve and delete
	resolveEvent(t, client, eventID)

	client.LoginAsAdmin(t)
	resp, err = client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteEvent_ServiceStatusUnchanged(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Status Unchanged Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Verify service starts operational
	assert.Equal(t, "operational", getServiceEffectiveStatus(t, client, slug))

	eventID := createTestIncident(t, client, "Status Unchanged Test", []AffectedService{
		{ServiceID: serviceID, Status: "major_outage"},
	}, nil)

	// Verify service is in major_outage
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, slug))

	// Resolve event (service should become operational)
	resolveEvent(t, client, eventID)

	// Verify service is operational after resolution
	assert.Equal(t, "operational", getServiceEffectiveStatus(t, client, slug))

	// Delete event
	client.LoginAsAdmin(t)
	resp, err := client.DELETE("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify service is STILL operational (deleting closed event doesn't change status)
	assert.Equal(t, "operational", getServiceEffectiveStatus(t, client, slug),
		"deleting resolved event should not change service status")
}
