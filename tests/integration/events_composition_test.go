//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPartialRecovery_OneServiceOperational verifies that when one service is recovered
// (set to operational) during an active incident, it correctly shows operational effective_status
// while other services remain affected (Scenario A3).
func TestPartialRecovery_OneServiceOperational(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create 2 services
	service1ID, service1Slug := createTestService(t, client, "partial-recovery-svc1")
	service2ID, service2Slug := createTestService(t, client, "partial-recovery-svc2")
	t.Cleanup(func() {
		deleteService(t, client, service1Slug)
		deleteService(t, client, service2Slug)
	})

	// Create incident with both services
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Partial Recovery Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "major",
		"description": "Testing partial recovery scenario",
		"affected_services": []map[string]interface{}{
			{"service_id": service1ID, "status": "major_outage"},
			{"service_id": service2ID, "status": "degraded"},
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
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Verify initial statuses
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, service1Slug))
	assert.Equal(t, "degraded", getServiceEffectiveStatus(t, client, service2Slug))

	// Act: Partial recovery - service1 becomes operational
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "monitoring",
		"message": "Service 1 recovered, still working on Service 2",
		"service_updates": []map[string]interface{}{
			{"service_id": service1ID, "status": "operational"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Assert
	// Service1: operational inside event → effective = operational
	assert.Equal(t, "operational", getServiceEffectiveStatus(t, client, service1Slug),
		"recovered service should show operational")

	// Service2: still degraded
	assert.Equal(t, "degraded", getServiceEffectiveStatus(t, client, service2Slug),
		"still affected service should show degraded")

	// Incident is still active (monitoring)
	resp, err = client.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	var event struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &event)
	assert.Equal(t, "monitoring", event.Data.Status,
		"incident should still be active")
}

func TestAddUpdate_AddServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	svc1ID, slug1 := createTestService(t, client, "Add Service 1")
	t.Cleanup(func() { deleteService(t, client, slug1) })

	svc2ID, slug2 := createTestService(t, client, "Add Service 2")
	t.Cleanup(func() { deleteService(t, client, slug2) })

	svc3ID, slug3 := createTestService(t, client, "Add Service 3")
	t.Cleanup(func() { deleteService(t, client, slug3) })

	eventID := createTestIncident(t, client, "Add Services Test", []AffectedService{
		{ServiceID: svc1ID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Add two more services via update
	resp, err := client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Adding more affected services",
		"add_services": []map[string]interface{}{
			{"service_id": svc2ID, "status": "partial_outage"},
			{"service_id": svc3ID, "status": "major_outage"},
		},
		"reason": "Discovered more affected services",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Get service changes
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var changesResult struct {
		Data []struct {
			ID        string  `json:"id"`
			BatchID   *string `json:"batch_id"`
			Action    string  `json:"action"`
			ServiceID *string `json:"service_id"`
			Reason    string  `json:"reason"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changesResult)

	// Should have 3 changes total (1 initial + 2 added)
	require.Len(t, changesResult.Data, 3, "should have 3 service changes")
}

func TestAddUpdate_RemoveServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	svc1ID, slug1 := createTestService(t, client, "Remove Service 1")
	t.Cleanup(func() { deleteService(t, client, slug1) })

	svc2ID, slug2 := createTestService(t, client, "Remove Service 2")
	t.Cleanup(func() { deleteService(t, client, slug2) })

	svc3ID, slug3 := createTestService(t, client, "Remove Service 3")
	t.Cleanup(func() { deleteService(t, client, slug3) })

	eventID := createTestIncident(t, client, "Remove Services Test", []AffectedService{
		{ServiceID: svc1ID, Status: "degraded"},
		{ServiceID: svc2ID, Status: "degraded"},
		{ServiceID: svc3ID, Status: "degraded"},
	}, nil)
	serviceIDs := []string{svc1ID, svc2ID, svc3ID}

	// Remove two services via update
	resp, err := client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":             "identified",
		"message":            "Removing incorrectly added services",
		"remove_service_ids": []string{serviceIDs[1], serviceIDs[2]},
		"reason":             "Services not actually affected",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Get service changes
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var changesResult struct {
		Data []struct {
			ID        string  `json:"id"`
			BatchID   *string `json:"batch_id"`
			Action    string  `json:"action"`
			ServiceID *string `json:"service_id"`
			Reason    string  `json:"reason"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changesResult)

	// Should have 5 changes: 3 initial adds + 2 removes
	require.Len(t, changesResult.Data, 5, "should have 5 service changes (3 adds + 2 removes)")

	// Find the remove changes
	var removeChanges []struct {
		BatchID *string
		Action  string
	}
	for _, c := range changesResult.Data {
		if c.Action == "removed" {
			removeChanges = append(removeChanges, struct {
				BatchID *string
				Action  string
			}{c.BatchID, c.Action})
		}
	}

	require.Len(t, removeChanges, 2, "should have 2 remove changes")

	t.Cleanup(func() { deleteEvent(t, client, eventID) })
}

func TestAddUpdate_AddGroups(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupID, groupSlug := createTestGroup(t, client, "Add Group Test")
	t.Cleanup(func() { deleteGroup(t, client, groupSlug) })

	_, svc1Slug := createTestService(t, client, "Add Group Service 1", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, svc1Slug) })

	_, svc2Slug := createTestService(t, client, "Add Group Service 2", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, svc2Slug) })

	// Create event without services
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Add Groups Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing add_groups via update",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var eventResult struct {
		Data struct {
			ID         string   `json:"id"`
			ServiceIDs []string `json:"service_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventResult)
	eventID := eventResult.Data.ID
	assert.Empty(t, eventResult.Data.ServiceIDs, "event should start with no services")

	// Add group via update
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Adding affected group",
		"add_groups": []map[string]interface{}{
			{"group_id": groupID, "status": "partial_outage"},
		},
		"reason": "Discovered group is affected",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Get event and verify services were expanded from group
	resp, err = client.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updatedEvent struct {
		Data struct {
			ServiceIDs []string `json:"service_ids"`
			GroupIDs   []string `json:"group_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &updatedEvent)
	assert.Len(t, updatedEvent.Data.ServiceIDs, 2, "should have 2 services from group")
	assert.Contains(t, updatedEvent.Data.GroupIDs, groupID, "should have the group")

	// Verify services have correct effective status
	effectiveStatus := getServiceEffectiveStatus(t, client, svc1Slug)
	assert.Equal(t, "partial_outage", effectiveStatus)

	// Get service changes and verify group change was recorded
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var changesResult struct {
		Data []struct {
			Action  string  `json:"action"`
			GroupID *string `json:"group_id"`
			Reason  string  `json:"reason"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changesResult)

	// Find the group change
	var foundGroupChange bool
	for _, c := range changesResult.Data {
		if c.GroupID != nil && *c.GroupID == groupID {
			foundGroupChange = true
			assert.Equal(t, "added", c.Action)
			assert.Equal(t, "Discovered group is affected", c.Reason)
			break
		}
	}
	assert.True(t, foundGroupChange, "should have a change record for the added group")

	// Cleanup
	resolveEvent(t, client, eventID)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })
}

func TestAddUpdate_UpdateServiceStatuses(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Update Status Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Status Update Test", []AffectedService{
		{ServiceID: serviceID, Status: "major_outage"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Verify effective status is major_outage
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, slug))

	// Update service status to degraded via event update
	resp, err := client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "monitoring",
		"message": "Service partially recovered",
		"service_updates": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Verify effective status is now degraded
	assert.Equal(t, "degraded", getServiceEffectiveStatus(t, client, slug))
}

func TestAddUpdate_CannotUpdateResolvedEvent(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	// Create and resolve an event
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Resolved Event Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing cannot update resolved event",
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

	// Try to update resolved event - should fail with 409
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "investigating",
		"message": "Reopening...",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()

	// Cleanup
	client.LoginAsAdmin(t)
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
}

func TestAddUpdate_ResolvedRecalculatesStoredStatus(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Recalc Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Recalc Test Incident", []AffectedService{
		{ServiceID: serviceID, Status: "major_outage"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Verify service has effective_status = major_outage
	resp, err := client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var svcCheck struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svcCheck)
	assert.Equal(t, "major_outage", svcCheck.Data.EffectiveStatus)
	assert.True(t, svcCheck.Data.HasActiveEvents)

	// Resolve the event
	resolveEvent(t, client, eventID)

	// Verify service is now operational
	resp, err = client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	testutil.DecodeJSON(t, resp, &svcCheck)
	assert.Equal(t, "operational", svcCheck.Data.EffectiveStatus)
	assert.False(t, svcCheck.Data.HasActiveEvents)
}

func TestAddUpdate_ResolvedWithOtherActiveEvent(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Multi-Event Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Create first event with major_outage
	eventAID := createTestIncident(t, client, "Event A", []AffectedService{
		{ServiceID: serviceID, Status: "major_outage"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventAID) })

	// Create second event with degraded
	eventBID := createTestIncident(t, client, "Event B", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventBID) })

	// Verify service has effective_status = major_outage (worst case)
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, slug))

	// Resolve first event (major_outage)
	resolveEvent(t, client, eventAID)

	// Verify service now has effective_status = degraded (from Event B)
	assert.Equal(t, "degraded", getServiceEffectiveStatus(t, client, slug))
}
