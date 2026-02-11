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
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var updateResult struct {
		Data struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &updateResult)
	assert.NotEmpty(t, updateResult.Data.ID)

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
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var resolveUpdateResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &resolveUpdateResult)
	assert.NotEmpty(t, resolveUpdateResult.Data.ID)

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

func TestEvents_CreateWithAffectedServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	svc1ID, slug1 := createTestService(t, client, "Affected Service 1")
	t.Cleanup(func() { deleteService(t, client, slug1) })

	svc2ID, slug2 := createTestService(t, client, "Affected Service 2")
	t.Cleanup(func() { deleteService(t, client, slug2) })

	// Create event with different statuses for each service
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Affected Services Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "major",
		"description": "Testing affected_services",
		"affected_services": []map[string]interface{}{
			{"service_id": svc1ID, "status": "degraded"},
			{"service_id": svc2ID, "status": "major_outage"},
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
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Verify both services are associated
	require.Len(t, eventResult.Data.ServiceIDs, 2)

	// Check that services have correct effective_status
	assert.Equal(t, "degraded", getServiceEffectiveStatus(t, client, slug1))
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, slug2))
}

func TestEvents_CreateWithAffectedGroups(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupID, groupSlug := createTestGroup(t, client, "Affected Group")
	t.Cleanup(func() { deleteGroup(t, client, groupSlug) })

	_, svc1Slug := createTestService(t, client, "Group Affected Service 1", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, svc1Slug) })

	_, svc2Slug := createTestService(t, client, "Group Affected Service 2", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, svc2Slug) })

	eventID := createTestIncident(t, client, "Affected Groups Test", nil, []AffectedGroup{
		{GroupID: groupID, Status: "partial_outage"},
	})
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Get event to verify services were associated
	resp, err := client.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var eventResult struct {
		Data struct {
			ServiceIDs []string `json:"service_ids"`
			GroupIDs   []string `json:"group_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventResult)

	// Verify both services are associated (expanded from group)
	require.Len(t, eventResult.Data.ServiceIDs, 2, "should have 2 services from group")
	require.Len(t, eventResult.Data.GroupIDs, 1, "should have 1 group")
	assert.Equal(t, groupID, eventResult.Data.GroupIDs[0])

	// Check that both services have the same effective_status from group
	assert.Equal(t, "partial_outage", getServiceEffectiveStatus(t, client, svc1Slug))
	assert.Equal(t, "partial_outage", getServiceEffectiveStatus(t, client, svc2Slug))
}

func TestEvents_ServiceOverridesGroup(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupID, groupSlug := createTestGroup(t, client, "Override Group")
	t.Cleanup(func() { deleteGroup(t, client, groupSlug) })

	svc1ID, svc1Slug := createTestService(t, client, "Override Service 1", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, svc1Slug) })

	_, svc2Slug := createTestService(t, client, "Override Service 2", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, svc2Slug) })

	// Create event with group AND explicit service with different status
	// Service1 is in group but also specified explicitly - explicit should win
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Override Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "major",
		"description": "Testing service overrides group",
		"affected_groups": []map[string]interface{}{
			{"group_id": groupID, "status": "degraded"},
		},
		"affected_services": []map[string]interface{}{
			{"service_id": svc1ID, "status": "major_outage"},
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
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Service1 should have major_outage (from explicit affected_services)
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, svc1Slug),
		"explicit service status should override group status")

	// Service2 should have degraded (from group)
	assert.Equal(t, "degraded", getServiceEffectiveStatus(t, client, svc2Slug),
		"service from group should have group status")
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
	deleteEvent(t, client, eventID)
}

func TestEvents_InvalidServiceStatus(t *testing.T) {
	client := newTestClientWithoutValidation()
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Invalid Status Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	client.LoginAsOperator(t)

	// Try to create event with invalid service status
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
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
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "should reject nonexistent service with 400")

	var errResult struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errResult)
	assert.Contains(t, errResult.Error.Message, "affected service not found")
	assert.Contains(t, errResult.Error.Message, fakeServiceID)
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
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "should reject nonexistent group with 400")

	var errResult struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errResult)
	assert.Contains(t, errResult.Error.Message, "affected group not found")
	assert.Contains(t, errResult.Error.Message, fakeGroupID)
}

func TestEvents_ServiceStatus_DefaultValue(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Status Test Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Create an event with this service using affected_services format
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
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
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Verify service is associated with the event
	require.Len(t, eventResult.Data.ServiceIDs, 1)
	assert.Equal(t, serviceID, eventResult.Data.ServiceIDs[0])
}

func TestEvents_CreateWithGroup_BatchID(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupID, groupSlug := createTestGroup(t, client, "Batch Test Group")
	t.Cleanup(func() { deleteGroup(t, client, groupSlug) })

	_, svc1Slug := createTestService(t, client, "Group Service 1", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, svc1Slug) })

	_, svc2Slug := createTestService(t, client, "Group Service 2", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, svc2Slug) })

	eventID := createTestIncident(t, client, "Group Batch Test", nil, []AffectedGroup{
		{GroupID: groupID, Status: "partial_outage"},
	})
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Get service changes
	resp, err := client.GET("/api/v1/events/" + eventID + "/changes")
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

func TestEvents_ArchivedService_Returns400(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create and archive a service
	serviceID, slug := createTestService(t, client, "Archived Service Test")
	resp, err := client.DELETE("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
	t.Cleanup(func() {
		client.POST("/api/v1/services/"+slug+"/restore", nil)
		deleteService(t, client, slug)
	})

	// Try to create event with archived service
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Archived Service Event",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing archived service rejection",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "should reject archived service with 400")

	var errResult struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errResult)
	assert.Contains(t, errResult.Error.Message, "affected service not found")
}

func TestEvents_AddUpdate_NonexistentService_Returns400(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a valid event first
	serviceID, slug := createTestService(t, client, "Valid Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Update Test Event", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	fakeServiceID := "00000000-0000-0000-0000-000000000000"

	// Try to add non-existent service via update
	resp, err := client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Adding fake service",
		"add_services": []map[string]interface{}{
			{"service_id": fakeServiceID, "status": "major_outage"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "should reject nonexistent service in update with 400")

	var errResult struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errResult)
	assert.Contains(t, errResult.Error.Message, "affected service not found")
	assert.Contains(t, errResult.Error.Message, fakeServiceID)
}

func TestEvents_AddUpdate_NonexistentGroup_Returns400(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a valid event first
	serviceID, slug := createTestService(t, client, "Valid Service For Group Test")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Update Group Test Event", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	fakeGroupID := "00000000-0000-0000-0000-000000000000"

	// Try to add non-existent group via update
	resp, err := client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Adding fake group",
		"add_groups": []map[string]interface{}{
			{"group_id": fakeGroupID, "status": "major_outage"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "should reject nonexistent group in update with 400")

	var errResult struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errResult)
	assert.Contains(t, errResult.Error.Message, "affected group not found")
	assert.Contains(t, errResult.Error.Message, fakeGroupID)
}
