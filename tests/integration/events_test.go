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
	client := newTestClient(t)
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

func TestEvents_InvalidStatusForType(t *testing.T) {
	client := newTestClient(t)
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

func TestEvents_ServiceChanges_BatchID(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	service1ID, slug1 := createTestService(t, client, "Batch Service 1")
	t.Cleanup(func() { deleteService(t, client, slug1) })

	service2ID, slug2 := createTestService(t, client, "Batch Service 2")
	t.Cleanup(func() { deleteService(t, client, slug2) })

	eventID := createTestIncident(t, client, "Batch Test Incident", []AffectedService{
		{ServiceID: service1ID, Status: "partial_outage"},
		{ServiceID: service2ID, Status: "partial_outage"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Get service changes and verify batch_id
	resp, err := client.GET("/api/v1/events/" + eventID + "/changes")
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

	// Should have 2 changes (one per service)
	require.Len(t, changesResult.Data, 2, "should have 2 initial service changes")

	// All changes from initial creation should have the same batch_id
	require.NotNil(t, changesResult.Data[0].BatchID, "first change should have batch_id")
	require.NotNil(t, changesResult.Data[1].BatchID, "second change should have batch_id")
	assert.Equal(t, *changesResult.Data[0].BatchID, *changesResult.Data[1].BatchID,
		"both changes should have the same batch_id")
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

	// Create a service
	serviceSlug := testutil.RandomSlug("recalc-svc")
	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Recalc Service",
		"slug": serviceSlug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var svcResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svcResult)
	serviceID := svcResult.Data.ID

	// Create event with the service
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Recalc Test Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "major",
		"description": "Testing stored status recalculation",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "major_outage"},
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

	// Verify service has effective_status = major_outage
	resp, err = client.GET("/api/v1/services/" + serviceSlug)
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
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Issue fixed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Verify service is now operational
	resp, err = client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	testutil.DecodeJSON(t, resp, &svcCheck)
	assert.Equal(t, "operational", svcCheck.Data.EffectiveStatus)
	assert.False(t, svcCheck.Data.HasActiveEvents)

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + serviceSlug)
}

func TestAddUpdate_ResolvedWithOtherActiveEvent(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	serviceSlug := testutil.RandomSlug("multi-event-svc")
	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Multi-Event Service",
		"slug": serviceSlug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var svcResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svcResult)
	serviceID := svcResult.Data.ID

	// Create first event with major_outage
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Event A",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "critical",
		"description": "First event",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "major_outage"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var eventAResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventAResult)
	eventAID := eventAResult.Data.ID

	// Create second event with degraded
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Event B",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Second event",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var eventBResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventBResult)
	eventBID := eventBResult.Data.ID

	// Verify service has effective_status = major_outage (worst case)
	resp, err = client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)

	var svcCheck struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svcCheck)
	assert.Equal(t, "major_outage", svcCheck.Data.EffectiveStatus)

	// Resolve first event (major_outage)
	resp, err = client.POST("/api/v1/events/"+eventAID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Event A fixed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Verify service now has effective_status = degraded (from Event B)
	resp, err = client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)

	testutil.DecodeJSON(t, resp, &svcCheck)
	assert.Equal(t, "degraded", svcCheck.Data.EffectiveStatus)

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventAID)
	resp.Body.Close()
	resp, _ = client.DELETE("/api/v1/events/" + eventBID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + serviceSlug)
}

func TestAddUpdate_UpdateServiceStatuses(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	serviceSlug := testutil.RandomSlug("update-status-svc")
	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Update Status Service",
		"slug": serviceSlug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var svcResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svcResult)
	serviceID := svcResult.Data.ID

	// Create event with service in major_outage
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Status Update Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "critical",
		"description": "Testing service status updates",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "major_outage"},
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

	// Verify effective status is major_outage
	resp, err = client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)

	var svcCheck struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svcCheck)
	assert.Equal(t, "major_outage", svcCheck.Data.EffectiveStatus)

	// Update service status to degraded via event update
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
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
	resp, err = client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)

	testutil.DecodeJSON(t, resp, &svcCheck)
	assert.Equal(t, "degraded", svcCheck.Data.EffectiveStatus)

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + serviceSlug)
}

func TestEvents_ServiceStatus_DefaultValue(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	serviceSlug := testutil.RandomSlug("status-svc")
	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Status Test Service",
		"slug": serviceSlug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var svcResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svcResult)
	serviceID := svcResult.Data.ID

	// Create an event with this service using new affected_services format
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Status Test Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing service status in event",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "partial_outage"},
		},
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

	// Verify service is associated with the event
	require.Len(t, eventResult.Data.ServiceIDs, 1)
	assert.Equal(t, serviceID, eventResult.Data.ServiceIDs[0])

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + serviceSlug)
}

func TestEvents_CreateWithGroup_BatchID(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a group
	groupSlug := testutil.RandomSlug("batch-grp")
	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name": "Batch Test Group",
		"slug": groupSlug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var groupResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &groupResult)
	groupID := groupResult.Data.ID

	// Create services in the group
	service1Slug := testutil.RandomSlug("grp-svc1")
	service2Slug := testutil.RandomSlug("grp-svc2")

	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Group Service 1",
		"slug":      service1Slug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Group Service 2",
		"slug":      service2Slug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Create event with group using new affected_groups format
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Group Batch Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing group batch_id",
		"affected_groups": []map[string]interface{}{
			{"group_id": groupID, "status": "partial_outage"},
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

	// Get service changes
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var changesResult struct {
		Data []struct {
			ID      string  `json:"id"`
			BatchID *string `json:"batch_id"`
			Action  string  `json:"action"`
			GroupID *string `json:"group_id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changesResult)

	// Should have 1 change for the group
	require.Len(t, changesResult.Data, 1, "should have 1 group change")
	require.NotNil(t, changesResult.Data[0].BatchID, "group change should have batch_id")
	require.NotNil(t, changesResult.Data[0].GroupID, "change should reference group")
	assert.Equal(t, groupID, *changesResult.Data[0].GroupID)

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + service1Slug)
	client.DELETE("/api/v1/services/" + service2Slug)
	client.DELETE("/api/v1/groups/" + groupSlug)
}

func TestEvents_CreateWithAffectedServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create test services
	service1Slug := testutil.RandomSlug("aff-svc1")
	service2Slug := testutil.RandomSlug("aff-svc2")

	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Affected Service 1",
		"slug": service1Slug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var svc1Result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc1Result)
	service1ID := svc1Result.Data.ID

	resp, err = client.POST("/api/v1/services", map[string]string{
		"name": "Affected Service 2",
		"slug": service2Slug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var svc2Result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc2Result)
	service2ID := svc2Result.Data.ID

	// Create event with different statuses for each service
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Affected Services Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "major",
		"description": "Testing affected_services",
		"affected_services": []map[string]interface{}{
			{"service_id": service1ID, "status": "degraded"},
			{"service_id": service2ID, "status": "major_outage"},
		},
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

	// Verify both services are associated
	require.Len(t, eventResult.Data.ServiceIDs, 2)

	// Check that services have correct effective_status
	resp, err = client.GET("/api/v1/services/" + service1Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var svc1StatusResult struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc1StatusResult)
	assert.Equal(t, "degraded", svc1StatusResult.Data.EffectiveStatus)

	resp, err = client.GET("/api/v1/services/" + service2Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var svc2StatusResult struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc2StatusResult)
	assert.Equal(t, "major_outage", svc2StatusResult.Data.EffectiveStatus)

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + service1Slug)
	client.DELETE("/api/v1/services/" + service2Slug)
}

func TestEvents_CreateWithAffectedGroups(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a group
	groupSlug := testutil.RandomSlug("aff-grp")
	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name": "Affected Group",
		"slug": groupSlug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var groupResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &groupResult)
	groupID := groupResult.Data.ID

	// Create services in the group
	service1Slug := testutil.RandomSlug("grp-aff-svc1")
	service2Slug := testutil.RandomSlug("grp-aff-svc2")

	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Group Affected Service 1",
		"slug":      service1Slug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Group Affected Service 2",
		"slug":      service2Slug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Create event with affected_groups - all services in group get same status
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Affected Groups Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "major",
		"description": "Testing affected_groups",
		"affected_groups": []map[string]interface{}{
			{"group_id": groupID, "status": "partial_outage"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var eventResult struct {
		Data struct {
			ID         string   `json:"id"`
			ServiceIDs []string `json:"service_ids"`
			GroupIDs   []string `json:"group_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventResult)
	eventID := eventResult.Data.ID

	// Verify both services are associated (expanded from group)
	require.Len(t, eventResult.Data.ServiceIDs, 2, "should have 2 services from group")
	require.Len(t, eventResult.Data.GroupIDs, 1, "should have 1 group")
	assert.Equal(t, groupID, eventResult.Data.GroupIDs[0])

	// Check that both services have the same effective_status from group
	resp, err = client.GET("/api/v1/services/" + service1Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var svc1StatusResult struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc1StatusResult)
	assert.Equal(t, "partial_outage", svc1StatusResult.Data.EffectiveStatus)

	resp, err = client.GET("/api/v1/services/" + service2Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var svc2StatusResult struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc2StatusResult)
	assert.Equal(t, "partial_outage", svc2StatusResult.Data.EffectiveStatus)

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + service1Slug)
	client.DELETE("/api/v1/services/" + service2Slug)
	client.DELETE("/api/v1/groups/" + groupSlug)
}

func TestEvents_ServiceOverridesGroup(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a group
	groupSlug := testutil.RandomSlug("override-grp")
	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name": "Override Group",
		"slug": groupSlug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var groupResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &groupResult)
	groupID := groupResult.Data.ID

	// Create services in the group
	service1Slug := testutil.RandomSlug("override-svc1")
	service2Slug := testutil.RandomSlug("override-svc2")

	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Override Service 1",
		"slug":      service1Slug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var svc1Result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc1Result)
	service1ID := svc1Result.Data.ID

	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Override Service 2",
		"slug":      service2Slug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Create event with group AND explicit service with different status
	// Service1 is in group but also specified explicitly - explicit should win
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Override Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "major",
		"description": "Testing service overrides group",
		"affected_groups": []map[string]interface{}{
			{"group_id": groupID, "status": "degraded"},
		},
		"affected_services": []map[string]interface{}{
			{"service_id": service1ID, "status": "major_outage"},
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

	// Service1 should have major_outage (from explicit affected_services)
	resp, err = client.GET("/api/v1/services/" + service1Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var svc1StatusResult struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc1StatusResult)
	assert.Equal(t, "major_outage", svc1StatusResult.Data.EffectiveStatus,
		"explicit service status should override group status")

	// Service2 should have degraded (from group)
	resp, err = client.GET("/api/v1/services/" + service2Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var svc2StatusResult struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc2StatusResult)
	assert.Equal(t, "degraded", svc2StatusResult.Data.EffectiveStatus,
		"service from group should have group status")

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + service1Slug)
	client.DELETE("/api/v1/services/" + service2Slug)
	client.DELETE("/api/v1/groups/" + groupSlug)
}

func TestEvents_CreateNoServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	// Create event without any services (informational event)
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Informational Event",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "General announcement",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var eventResult struct {
		Data struct {
			ID         string   `json:"id"`
			ServiceIDs []string `json:"service_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventResult)
	eventID := eventResult.Data.ID

	// ServiceIDs should be empty or nil
	assert.Empty(t, eventResult.Data.ServiceIDs, "event without services should have empty service_ids")

	// Cleanup
	client.LoginAsAdmin(t)
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
}

func TestEvents_InvalidServiceStatus(t *testing.T) {
	client := newTestClientWithoutValidation()
	client.LoginAsOperator(t)

	// Create a service first
	client.LoginAsAdmin(t)
	serviceSlug := testutil.RandomSlug("invalid-status-svc")
	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Invalid Status Service",
		"slug": serviceSlug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var svcResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svcResult)
	serviceID := svcResult.Data.ID

	client.LoginAsOperator(t)

	// Try to create event with invalid service status
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Invalid Status Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing invalid status",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "invalid_status"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "should reject invalid service status")
	resp.Body.Close()

	// Cleanup
	client.LoginAsAdmin(t)
	client.DELETE("/api/v1/services/" + serviceSlug)
}

func TestEvents_NonexistentService(t *testing.T) {
	client := newTestClientWithoutValidation()
	client.LoginAsOperator(t)

	fakeServiceID := "00000000-0000-0000-0000-000000000000"

	// Try to create event with non-existent service
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Nonexistent Service Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing nonexistent service",
		"affected_services": []map[string]interface{}{
			{"service_id": fakeServiceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	// Should fail due to FK constraint
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode, "should reject nonexistent service")
	resp.Body.Close()
}

func TestEvents_NonexistentGroup(t *testing.T) {
	client := newTestClientWithoutValidation()
	client.LoginAsOperator(t)

	fakeGroupID := "00000000-0000-0000-0000-000000000000"

	// Try to create event with non-existent group
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Nonexistent Group Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing nonexistent group",
		"affected_groups": []map[string]interface{}{
			{"group_id": fakeGroupID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	// Should fail - resolver can't find the group
	assert.NotEqual(t, http.StatusCreated, resp.StatusCode, "should reject nonexistent group")
	resp.Body.Close()
}

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
	resp, _ = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Resolved for cleanup",
	})
	resp.Body.Close()

	client.LoginAsAdmin(t)
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
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
	resp, _ = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "in_progress",
		"message": "Starting maintenance",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Complete the maintenance
	resp, _ = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "completed",
		"message": "Maintenance completed",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

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

	resp, _ = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Resolved",
	})
	resp.Body.Close()

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
	resp, _ = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Found the issue",
	})
	resp.Body.Close()

	resp, _ = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "monitoring",
		"message": "Fix deployed",
	})
	resp.Body.Close()

	resp, _ = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Confirmed fixed",
	})
	resp.Body.Close()

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
