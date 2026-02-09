//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveStatus_NoActiveEvents(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "No Events Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Get service and verify effective_status equals stored status
	resp, err := client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Status          string `json:"status"`
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.Equal(t, "operational", result.Data.Status)
	assert.Equal(t, "operational", result.Data.EffectiveStatus)
	assert.False(t, result.Data.HasActiveEvents)
}

func TestEffectiveStatus_SingleActiveEvent(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Single Event Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Minor Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Get service and verify effective_status is degraded
	resp, err := client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Status          string `json:"status"`
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.Equal(t, "operational", result.Data.Status, "stored status unchanged")
	assert.Equal(t, "degraded", result.Data.EffectiveStatus, "effective status from event")
	assert.True(t, result.Data.HasActiveEvents)
}

func TestEffectiveStatus_MultipleActiveEvents_WorstCase(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Multi Event Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Create first incident (degraded status)
	event1ID := createTestIncident(t, client, "Minor Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, event1ID) })

	// Create second incident (major_outage status)
	event2ID := createTestIncident(t, client, "Critical Incident", []AffectedService{
		{ServiceID: serviceID, Status: "major_outage"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, event2ID) })

	// Get service and verify effective_status is major_outage (worst case)
	resp, err := client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Status          string `json:"status"`
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.Equal(t, "operational", result.Data.Status)
	assert.Equal(t, "major_outage", result.Data.EffectiveStatus, "should be worst-case status")
	assert.True(t, result.Data.HasActiveEvents)
}

func TestEffectiveStatus_ResolvedEventIgnored(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Resolved Event Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Soon Resolved Incident", []AffectedService{
		{ServiceID: serviceID, Status: "major_outage"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Verify effective_status is major_outage while active
	resp, err := client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var activeResult struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &activeResult)
	assert.Equal(t, "major_outage", activeResult.Data.EffectiveStatus)
	assert.True(t, activeResult.Data.HasActiveEvents)

	// Resolve the incident
	resolveEvent(t, client, eventID)

	// Verify effective_status is back to operational
	resp, err = client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var resolvedResult struct {
		Data struct {
			Status          string `json:"status"`
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &resolvedResult)

	assert.Equal(t, "operational", resolvedResult.Data.Status)
	assert.Equal(t, "operational", resolvedResult.Data.EffectiveStatus, "resolved event should not affect status")
	assert.False(t, resolvedResult.Data.HasActiveEvents)
}

func TestEffectiveStatus_CompletedMaintenanceIgnored(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Completed Maintenance Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestMaintenance(t, client, "Scheduled Maintenance", []AffectedService{
		{ServiceID: serviceID, Status: "maintenance"},
	})
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Verify effective_status is maintenance while active
	resp, err := client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var activeResult struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &activeResult)
	assert.Equal(t, "maintenance", activeResult.Data.EffectiveStatus)
	assert.True(t, activeResult.Data.HasActiveEvents)

	// Complete the maintenance
	completeMaintenance(t, client, eventID)

	// Verify effective_status is back to operational
	resp, err = client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var completedResult struct {
		Data struct {
			Status          string `json:"status"`
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &completedResult)

	assert.Equal(t, "operational", completedResult.Data.Status)
	assert.Equal(t, "operational", completedResult.Data.EffectiveStatus, "completed maintenance should not affect status")
	assert.False(t, completedResult.Data.HasActiveEvents)
}

func TestEffectiveStatus_MaintenanceVsIncident(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Maintenance vs Incident Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Create maintenance
	maintID := createTestMaintenance(t, client, "Scheduled Maintenance", []AffectedService{
		{ServiceID: serviceID, Status: "maintenance"},
	})
	t.Cleanup(func() { deleteEvent(t, client, maintID) })

	// Create incident (degraded status, priority 2)
	incidentID := createTestIncident(t, client, "Minor Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, incidentID) })

	// Get service and verify effective_status is degraded (priority 2 > maintenance priority 1)
	resp, err := client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.Equal(t, "degraded", result.Data.EffectiveStatus, "incident status should override maintenance")
	assert.True(t, result.Data.HasActiveEvents)
}

func TestListServices_FilterByEffectiveStatus(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	service1ID, slug1 := createTestService(t, client, "Filter Service 1")
	t.Cleanup(func() { deleteService(t, client, slug1) })

	_, slug2 := createTestService(t, client, "Filter Service 2")
	t.Cleanup(func() { deleteService(t, client, slug2) })

	// Create an incident for service 1 (degraded status)
	eventID := createTestIncident(t, client, "Filter Test Incident", []AffectedService{
		{ServiceID: service1ID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Filter by status=degraded should find service1
	resp, err := client.GET("/api/v1/services?status=degraded")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var degradedList struct {
		Data []struct {
			Slug            string `json:"slug"`
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &degradedList)

	foundService1 := false
	for _, svc := range degradedList.Data {
		assert.Equal(t, "degraded", svc.EffectiveStatus, "all services should have degraded effective status")
		if svc.Slug == slug1 {
			foundService1 = true
		}
		assert.NotEqual(t, slug2, svc.Slug, "service2 should not be in degraded list")
	}
	assert.True(t, foundService1, "service1 should be in degraded list")

	// Filter by status=operational should find service2 but not service1
	resp, err = client.GET("/api/v1/services?status=operational")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var operationalList struct {
		Data []struct {
			Slug            string `json:"slug"`
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &operationalList)

	foundService2 := false
	for _, svc := range operationalList.Data {
		assert.Equal(t, "operational", svc.EffectiveStatus, "all services should have operational effective status")
		if svc.Slug == slug2 {
			foundService2 = true
		}
		assert.NotEqual(t, slug1, svc.Slug, "service1 should not be in operational list")
	}
	assert.True(t, foundService2, "service2 should be in operational list")
}

func TestListServices_EffectiveStatusInResponse(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "List Effective Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// List services and verify effective_status and has_active_events are present
	resp, err := client.GET("/api/v1/services")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listResult struct {
		Data []struct {
			Slug            string `json:"slug"`
			Status          string `json:"status"`
			EffectiveStatus string `json:"effective_status"`
			HasActiveEvents bool   `json:"has_active_events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &listResult)

	found := false
	for _, svc := range listResult.Data {
		if svc.Slug == slug {
			found = true
			assert.Equal(t, "operational", svc.Status)
			assert.Equal(t, "operational", svc.EffectiveStatus)
			assert.False(t, svc.HasActiveEvents)
		}
	}
	assert.True(t, found, "service should be in list")
}

// TestManualStatusChange_WithActiveEvent verifies that manual status changes
// update the stored status but effective_status remains driven by active events.
// When resolved, the system resets stored status to operational (current behavior).
func TestManualStatusChange_WithActiveEvent(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "manual-with-event-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create active incident
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Active Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "critical",
		"description": "Testing manual status change during active event",
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
	// Note: cleanup will resolve the event first, so we track resolved state
	eventResolved := false
	t.Cleanup(func() {
		if !eventResolved {
			resolveEvent(t, client, eventID)
		}
		deleteEvent(t, client, eventID)
	})

	// Verify effective_status = major_outage (from event)
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, serviceSlug))

	// Act: Manual change of stored status while event is active
	resp, err = client.PATCH("/api/v1/services/"+serviceSlug, map[string]interface{}{
		"name":   "manual-with-event-svc",
		"slug":   serviceSlug,
		"status": "degraded",
		"reason": "Manual override during incident",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Assert: effective_status still major_outage (from event)
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, serviceSlug),
		"effective_status should still be from event (major_outage)")

	// Verify stored status changed to degraded
	resp, err = client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	var svc struct {
		Data struct {
			Status          string `json:"status"`
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc)
	assert.Equal(t, "degraded", svc.Data.Status,
		"stored status should be manually changed to degraded")
	assert.Equal(t, "major_outage", svc.Data.EffectiveStatus,
		"effective status should still be from event")

	// Act: Resolve incident
	resolveEvent(t, client, eventID)
	eventResolved = true

	// Assert: After resolution, system resets stored status to operational
	// (current behavior: resolve without other active events → operational)
	assert.Equal(t, "operational", getServiceEffectiveStatus(t, client, serviceSlug),
		"after event resolved, status should be reset to operational (current behavior)")
}

func TestStatusLog_ManualChange(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "Status Log Manual")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Change status with reason
	resp, err := client.PATCH("/api/v1/services/"+slug, map[string]interface{}{
		"name":   "Status Log Manual",
		"slug":   slug,
		"status": "degraded",
		"reason": "Testing status log",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Get status log
	resp, err = client.GET("/api/v1/services/" + slug + "/status-log")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var logResult struct {
		Data struct {
			Entries []struct {
				ID         string  `json:"id"`
				ServiceID  string  `json:"service_id"`
				OldStatus  *string `json:"old_status"`
				NewStatus  string  `json:"new_status"`
				SourceType string  `json:"source_type"`
				EventID    *string `json:"event_id"`
				Reason     string  `json:"reason"`
				CreatedBy  string  `json:"created_by"`
			} `json:"entries"`
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &logResult)

	require.GreaterOrEqual(t, logResult.Data.Total, 1)
	require.GreaterOrEqual(t, len(logResult.Data.Entries), 1)

	// Find the manual entry
	var foundManual bool
	for _, entry := range logResult.Data.Entries {
		if entry.SourceType == "manual" && entry.NewStatus == "degraded" {
			foundManual = true
			assert.NotNil(t, entry.OldStatus)
			assert.Equal(t, "operational", *entry.OldStatus)
			assert.Equal(t, "Testing status log", entry.Reason)
			assert.Nil(t, entry.EventID)
			break
		}
	}
	assert.True(t, foundManual, "should find manual status change entry")
}

func TestStatusLog_EventChange(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Status Log Event")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Status Log Test Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Get status log
	resp, err := client.GET("/api/v1/services/" + slug + "/status-log")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var logResult struct {
		Data struct {
			Entries []struct {
				SourceType string  `json:"source_type"`
				NewStatus  string  `json:"new_status"`
				EventID    *string `json:"event_id"`
			} `json:"entries"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &logResult)

	require.GreaterOrEqual(t, logResult.Data.Total, 1)

	// Find the event entry
	var foundEvent bool
	for _, entry := range logResult.Data.Entries {
		if entry.SourceType == "event" && entry.NewStatus == "degraded" {
			foundEvent = true
			assert.NotNil(t, entry.EventID)
			assert.Equal(t, eventID, *entry.EventID)
			break
		}
	}
	assert.True(t, foundEvent, "should find event-triggered status change entry")
}

func TestStatusLog_EventUpdate(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Status Log Update")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Status Log Update Test Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Update the event with service_updates to change status to major_outage
	resp, err := client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Issue identified, escalating severity",
		"service_updates": []map[string]interface{}{
			{"service_id": serviceID, "status": "major_outage"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Get status log
	resp, err = client.GET("/api/v1/services/" + slug + "/status-log")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var logResult struct {
		Data struct {
			Entries []struct {
				SourceType string  `json:"source_type"`
				OldStatus  *string `json:"old_status"`
				NewStatus  string  `json:"new_status"`
				EventID    *string `json:"event_id"`
			} `json:"entries"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &logResult)

	// Should have at least 2 entries: initial creation and update
	require.GreaterOrEqual(t, logResult.Data.Total, 2)

	// Find the update entry (should be first, most recent)
	var foundUpdate bool
	for _, entry := range logResult.Data.Entries {
		if entry.SourceType == "event" && entry.NewStatus == "major_outage" {
			foundUpdate = true
			assert.NotNil(t, entry.EventID)
			assert.Equal(t, eventID, *entry.EventID)
			// Old status should be degraded
			assert.NotNil(t, entry.OldStatus)
			assert.Equal(t, "degraded", *entry.OldStatus)
			break
		}
	}
	assert.True(t, foundUpdate, "should find event update status change entry")
}

func TestStatusLog_EventResolved(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Status Log Resolved")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Resolved Incident", []AffectedService{
		{ServiceID: serviceID, Status: "major_outage"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Resolve the incident
	resolveEvent(t, client, eventID)

	// Get status log
	resp, err := client.GET("/api/v1/services/" + slug + "/status-log")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var logResult struct {
		Data struct {
			Entries []struct {
				SourceType string `json:"source_type"`
				OldStatus  string `json:"old_status"`
				NewStatus  string `json:"new_status"`
				Reason     string `json:"reason"`
			} `json:"entries"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &logResult)

	// Should have at least 2 entries: creation and resolution
	require.GreaterOrEqual(t, logResult.Data.Total, 2)

	// Find the resolution entry (should be operational now)
	var foundResolution bool
	for _, entry := range logResult.Data.Entries {
		if entry.SourceType == "event" && entry.NewStatus == "operational" {
			foundResolution = true
			assert.Contains(t, entry.Reason, "resolved")
			break
		}
	}
	assert.True(t, foundResolution, "should find resolution status change entry")
}

func TestStatusLog_Pagination(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "Status Log Pagination")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Change status multiple times
	statuses := []string{"degraded", "partial_outage", "major_outage", "operational"}
	for _, status := range statuses {
		resp, err := client.PATCH("/api/v1/services/"+slug, map[string]interface{}{
			"name":   "Status Log Pagination",
			"slug":   slug,
			"status": status,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// Get with limit=2
	resp, err := client.GET("/api/v1/services/" + slug + "/status-log?limit=2")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var page1 struct {
		Data struct {
			Entries []struct {
				NewStatus string `json:"new_status"`
			} `json:"entries"`
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &page1)

	assert.Equal(t, 2, page1.Data.Limit)
	assert.Equal(t, 0, page1.Data.Offset)
	assert.Equal(t, 2, len(page1.Data.Entries))
	assert.GreaterOrEqual(t, page1.Data.Total, 4) // At least 4 changes

	// Get with offset=2
	resp, err = client.GET("/api/v1/services/" + slug + "/status-log?limit=2&offset=2")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var page2 struct {
		Data struct {
			Entries []struct {
				NewStatus string `json:"new_status"`
			} `json:"entries"`
			Offset int `json:"offset"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &page2)

	assert.Equal(t, 2, page2.Data.Offset)
	assert.GreaterOrEqual(t, len(page2.Data.Entries), 1)
}

func TestStatusLog_RequiresAuth(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "Status Log Auth")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Try to access without auth
	publicClient := newTestClient(t)
	resp, err := publicClient.GET("/api/v1/services/" + slug + "/status-log")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestStatusLog_RequiresOperatorRole(t *testing.T) {
	adminClient := newTestClient(t)
	adminClient.LoginAsAdmin(t)

	_, slug := createTestService(t, adminClient, "Status Log Role")
	t.Cleanup(func() { deleteService(t, adminClient, slug) })

	// Try as user (should fail with 403)
	userClient := newTestClient(t)
	userClient.LoginAsUser(t)
	resp, err := userClient.GET("/api/v1/services/" + slug + "/status-log")
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()

	// Try as operator (should succeed)
	operatorClient := newTestClient(t)
	operatorClient.LoginAsOperator(t)
	resp, err = operatorClient.GET("/api/v1/services/" + slug + "/status-log")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
