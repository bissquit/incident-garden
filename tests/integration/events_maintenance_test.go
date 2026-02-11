//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMaintenance_ScheduledDoesNotAffectStatus verifies that scheduled maintenance
// does NOT affect effective_status until it transitions to in_progress.
// This tests the full maintenance lifecycle: scheduled → in_progress → completed.
func TestMaintenance_ScheduledDoesNotAffectStatus(t *testing.T) {
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
		completeMaintenance(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Assert: effective_status = operational (scheduled does NOT affect effective_status)
	status = getServiceEffectiveStatus(t, client, serviceSlug)
	assert.Equal(t, "operational", status,
		"scheduled maintenance should NOT affect effective_status")

	// Act: Transition to in_progress
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "in_progress",
		"message": "Starting maintenance",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Assert: effective_status is now maintenance (in_progress affects status)
	status = getServiceEffectiveStatus(t, client, serviceSlug)
	assert.Equal(t, "maintenance", status,
		"in_progress maintenance should affect effective_status")

	// Act: Complete the maintenance
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "completed",
		"message": "Maintenance completed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Assert: effective_status is back to operational
	status = getServiceEffectiveStatus(t, client, serviceSlug)
	assert.Equal(t, "operational", status,
		"after maintenance completed, effective_status should be operational")
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
		completeMaintenance(t, client, eventID)
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

// TestEffectiveStatus_ScheduledMaintenanceWithActiveIncident verifies that scheduled maintenance
// does not affect effective_status even when there's an active incident.
// The effective_status should be driven only by the active incident.
func TestEffectiveStatus_ScheduledMaintenanceWithActiveIncident(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "scheduled-with-incident-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create an active incident with major_outage
	incidentResp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Active Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "critical",
		"description": "Active incident",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "major_outage"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, incidentResp.StatusCode)

	var incidentResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, incidentResp, &incidentResult)
	incidentID := incidentResult.Data.ID
	t.Cleanup(func() {
		resolveEvent(t, client, incidentID)
		deleteEvent(t, client, incidentID)
	})

	// Verify effective_status is major_outage from incident
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, serviceSlug))

	// Create scheduled maintenance for the same service
	maintResp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":              "Scheduled Maintenance During Incident",
		"type":               "maintenance",
		"status":             "scheduled",
		"description":        "Planned maintenance",
		"scheduled_start_at": "2099-01-01T00:00:00Z",
		"scheduled_end_at":   "2099-01-01T04:00:00Z",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "maintenance"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, maintResp.StatusCode)

	var maintResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, maintResp, &maintResult)
	maintID := maintResult.Data.ID
	t.Cleanup(func() {
		completeMaintenance(t, client, maintID)
		deleteEvent(t, client, maintID)
	})

	// Assert: effective_status is still major_outage (scheduled maintenance is ignored)
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, serviceSlug),
		"scheduled maintenance should be ignored, incident status should prevail")

	// Transition maintenance to in_progress
	resp, err := client.POST("/api/v1/events/"+maintID+"/updates", map[string]interface{}{
		"status":  "in_progress",
		"message": "Starting maintenance",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Assert: effective_status is still major_outage (incident > maintenance priority)
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, serviceSlug),
		"incident status (major_outage) should override maintenance (worst-case wins)")
}

// TestStoredStatus_ResetsToOperationalOnResolve verifies that when an event is resolved
// and the service has no other active events, the stored_status is reset to operational.
// This happens even if the status was manually changed during the event.
func TestStoredStatus_ResetsToOperationalOnResolve(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	_, serviceSlug := createTestService(t, client, "reset-on-resolve-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Manually set status to degraded
	resp, err := client.PATCH("/api/v1/services/"+serviceSlug, map[string]interface{}{
		"name":   "reset-on-resolve-svc",
		"slug":   serviceSlug,
		"status": "degraded",
		"reason": "Manual degradation before incident",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify stored status is degraded
	svcResp, err := client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	var svc struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, svcResp, &svc)
	assert.Equal(t, "degraded", svc.Data.Status)
	serviceID := svc.Data.ID

	// Create incident
	incidentResp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Incident To Resolve",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "critical",
		"description": "Testing reset on resolve",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "major_outage"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, incidentResp.StatusCode)

	var incidentResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, incidentResp, &incidentResult)
	incidentID := incidentResult.Data.ID
	t.Cleanup(func() { deleteEvent(t, client, incidentID) })

	// Verify effective_status is major_outage
	assert.Equal(t, "major_outage", getServiceEffectiveStatus(t, client, serviceSlug))

	// Resolve the incident
	resolveEvent(t, client, incidentID)

	// Assert: stored_status is now operational (NOT degraded)
	svcResp, err = client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	var svcAfter struct {
		Data struct {
			Status          string `json:"status"`
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, svcResp, &svcAfter)
	assert.Equal(t, "operational", svcAfter.Data.Status,
		"stored_status should be reset to operational after event resolution")
	assert.Equal(t, "operational", svcAfter.Data.EffectiveStatus,
		"effective_status should be operational after event resolution")
}

// TestStoredStatus_ManualChangeAfterResolve verifies that manual status changes
// work correctly after an event is resolved.
func TestStoredStatus_ManualChangeAfterResolve(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "manual-after-resolve-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create and resolve an incident
	incidentResp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Quick Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Quick incident for testing",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, incidentResp.StatusCode)

	var incidentResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, incidentResp, &incidentResult)
	incidentID := incidentResult.Data.ID
	t.Cleanup(func() { deleteEvent(t, client, incidentID) })

	// Resolve the incident
	resolveEvent(t, client, incidentID)

	// Verify stored_status is operational after resolve
	svcResp, err := client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	var svc struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, svcResp, &svc)
	assert.Equal(t, "operational", svc.Data.Status)

	// Manually change status to degraded
	resp, err := client.PATCH("/api/v1/services/"+serviceSlug, map[string]interface{}{
		"name":   "manual-after-resolve-svc",
		"slug":   serviceSlug,
		"status": "degraded",
		"reason": "Manual change after incident resolved",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify stored_status and effective_status are both degraded
	svcResp, err = client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	var svcAfter struct {
		Data struct {
			Status          string `json:"status"`
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, svcResp, &svcAfter)
	assert.Equal(t, "degraded", svcAfter.Data.Status,
		"stored_status should be manually changed to degraded")
	assert.Equal(t, "degraded", svcAfter.Data.EffectiveStatus,
		"effective_status should equal stored_status when no active events")
}

// TestService_ArchiveWithScheduledMaintenance verifies that a service can be archived
// when it only has scheduled maintenance (not yet in_progress).
// Scheduled maintenance is not considered "active" for archiving purposes.
func TestService_ArchiveWithScheduledMaintenance(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "archive-with-scheduled-svc")

	// Create scheduled maintenance for the service
	maintResp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":              "Scheduled Maintenance for Archive Test",
		"type":               "maintenance",
		"status":             "scheduled",
		"description":        "Planned maintenance",
		"scheduled_start_at": "2099-01-01T00:00:00Z",
		"scheduled_end_at":   "2099-01-01T04:00:00Z",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "maintenance"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, maintResp.StatusCode)

	var maintResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, maintResp, &maintResult)
	maintID := maintResult.Data.ID
	t.Cleanup(func() {
		completeMaintenance(t, client, maintID)
		deleteEvent(t, client, maintID)
	})

	// Act: Try to archive (soft delete) the service
	resp, err := client.DELETE("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)

	// Assert: Should succeed (scheduled maintenance doesn't block archiving)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode,
		"should be able to archive service with only scheduled maintenance")
	resp.Body.Close()

	// Verify service is archived (not in default list)
	listResp, err := client.GET("/api/v1/services")
	require.NoError(t, err)
	var listResult struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, listResp, &listResult)

	found := false
	for _, svc := range listResult.Data {
		if svc.Slug == serviceSlug {
			found = true
			break
		}
	}
	assert.False(t, found, "archived service should not appear in default list")

	// Verify service appears with include_archived=true
	listResp, err = client.GET("/api/v1/services?include_archived=true")
	require.NoError(t, err)
	testutil.DecodeJSON(t, listResp, &listResult)

	found = false
	for _, svc := range listResult.Data {
		if svc.Slug == serviceSlug {
			found = true
			break
		}
	}
	assert.True(t, found, "archived service should appear with include_archived=true")
}

// TestServiceEvents_ActiveFilterExcludesScheduled verifies that the status=active
// filter in /services/{slug}/events does NOT include scheduled maintenance.
func TestServiceEvents_ActiveFilterExcludesScheduled(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "active-filter-test-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create scheduled maintenance
	maintResp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":              "Scheduled Maintenance",
		"type":               "maintenance",
		"status":             "scheduled",
		"description":        "Planned maintenance",
		"scheduled_start_at": "2099-01-01T00:00:00Z",
		"scheduled_end_at":   "2099-01-01T04:00:00Z",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "maintenance"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, maintResp.StatusCode)

	var maintResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, maintResp, &maintResult)
	maintID := maintResult.Data.ID
	t.Cleanup(func() {
		completeMaintenance(t, client, maintID)
		deleteEvent(t, client, maintID)
	})

	// Create active incident
	incidentResp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Active Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Active incident",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, incidentResp.StatusCode)

	var incidentResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, incidentResp, &incidentResult)
	incidentID := incidentResult.Data.ID
	t.Cleanup(func() {
		resolveEvent(t, client, incidentID)
		deleteEvent(t, client, incidentID)
	})

	// Act: Get events with status=active filter
	resp, err := client.GET("/api/v1/services/" + serviceSlug + "/events?status=active")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var eventsResult struct {
		Data struct {
			Events []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
				Type   string `json:"type"`
			} `json:"events"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventsResult)

	// Assert: Only the incident should be returned, not the scheduled maintenance
	assert.Equal(t, 1, eventsResult.Data.Total,
		"only active events should be counted (scheduled excluded)")
	require.Len(t, eventsResult.Data.Events, 1)
	assert.Equal(t, incidentID, eventsResult.Data.Events[0].ID,
		"should return only the active incident")
	assert.Equal(t, "incident", eventsResult.Data.Events[0].Type)

	// Verify without filter returns both
	resp, err = client.GET("/api/v1/services/" + serviceSlug + "/events")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testutil.DecodeJSON(t, resp, &eventsResult)

	assert.Equal(t, 2, eventsResult.Data.Total,
		"without filter should return all events including scheduled")
}
