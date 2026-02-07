//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog_Group_CRUD(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	slug := testutil.RandomSlug("test-group")

	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name":        "Test Group",
		"slug":        slug,
		"description": "Test description",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResult struct {
		Data struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
			Name string `json:"name"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &createResult)
	assert.Equal(t, slug, createResult.Data.Slug)
	assert.Equal(t, "Test Group", createResult.Data.Name)

	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/groups/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.PATCH("/api/v1/groups/"+slug, map[string]interface{}{
		"name":        "Test Group",
		"slug":        slug,
		"description": "Updated description",
		"order":       0,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updateResult struct {
		Data struct {
			Description string `json:"description"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &updateResult)
	assert.Equal(t, "Updated description", updateResult.Data.Description)

	// Delete (archive) the group
	resp, err = client.DELETE("/api/v1/groups/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Archived group is still accessible by slug (soft delete)
	resp, err = publicClient.GET("/api/v1/groups/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var archivedResult struct {
		Data struct {
			ArchivedAt *string `json:"archived_at"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &archivedResult)
	assert.NotNil(t, archivedResult.Data.ArchivedAt, "archived_at should be set after delete")

	// Archived group should NOT appear in list by default
	resp, err = publicClient.GET("/api/v1/groups")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResult struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &listResult)
	for _, g := range listResult.Data {
		assert.NotEqual(t, slug, g.Slug, "archived group should not appear in default list")
	}

	// Archived group SHOULD appear when include_archived=true
	resp, err = publicClient.GET("/api/v1/groups?include_archived=true")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listArchivedResult struct {
		Data []struct {
			Slug       string  `json:"slug"`
			ArchivedAt *string `json:"archived_at"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &listArchivedResult)
	found := false
	for _, g := range listArchivedResult.Data {
		if g.Slug == slug {
			found = true
			assert.NotNil(t, g.ArchivedAt, "archived group should have archived_at set")
		}
	}
	assert.True(t, found, "archived group should appear in list with include_archived=true")
}

func TestCatalog_Service_CRUD(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	slug := testutil.RandomSlug("test-service")

	resp, err := client.POST("/api/v1/services", map[string]string{
		"name":        "Test Service",
		"slug":        slug,
		"description": "Test service description",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResult struct {
		Data struct {
			ID     string `json:"id"`
			Slug   string `json:"slug"`
			Status string `json:"status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &createResult)
	assert.Equal(t, slug, createResult.Data.Slug)
	assert.Equal(t, "operational", createResult.Data.Status)

	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Delete (archive) the service
	resp, err = client.DELETE("/api/v1/services/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Archived service is still accessible by slug (soft delete)
	resp, err = publicClient.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var archivedResult struct {
		Data struct {
			ArchivedAt *string `json:"archived_at"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &archivedResult)
	assert.NotNil(t, archivedResult.Data.ArchivedAt, "archived_at should be set after delete")

	// Archived service should NOT appear in list by default
	resp, err = publicClient.GET("/api/v1/services")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResult struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &listResult)
	for _, s := range listResult.Data {
		assert.NotEqual(t, slug, s.Slug, "archived service should not appear in default list")
	}
}

func TestCatalog_Service_WithGroup(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupSlug := testutil.RandomSlug("group")
	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name": "Parent Group",
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

	serviceSlug := testutil.RandomSlug("service")
	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Service in Group",
		"slug":      serviceSlug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var serviceResult struct {
		Data struct {
			GroupIDs []string `json:"group_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &serviceResult)
	require.NotNil(t, serviceResult.Data.GroupIDs)
	require.Len(t, serviceResult.Data.GroupIDs, 1)
	assert.Equal(t, groupID, serviceResult.Data.GroupIDs[0])

	client.DELETE("/api/v1/services/" + serviceSlug)
	client.DELETE("/api/v1/groups/" + groupSlug)
}

func TestCatalog_Group_ArchiveWithServices_Blocked(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a group
	groupSlug := testutil.RandomSlug("group-with-svc")
	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name": "Group With Service",
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

	// Create a service in the group
	serviceSlug := testutil.RandomSlug("service-in-group")
	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Service in Group",
		"slug":      serviceSlug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Try to archive the group - should fail with 409
	resp, err = client.DELETE("/api/v1/groups/" + groupSlug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var errorResult struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errorResult)
	assert.Equal(t, "cannot archive group: has services", errorResult.Error.Message)

	// Cleanup: remove service from group, then archive both
	resp, err = client.PATCH("/api/v1/services/"+serviceSlug, map[string]interface{}{
		"name":      "Service in Group",
		"slug":      serviceSlug,
		"status":    "operational",
		"group_ids": []string{},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Now archiving group should succeed
	resp, err = client.DELETE("/api/v1/groups/" + groupSlug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Cleanup service
	client.DELETE("/api/v1/services/" + serviceSlug)
}

func TestCatalog_Group_ArchiveWithArchivedServices_Allowed(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a group
	groupSlug := testutil.RandomSlug("group-archived-svc")
	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name": "Group With Archived Service",
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

	// Create a service in the group
	serviceSlug := testutil.RandomSlug("archived-svc")
	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Soon Archived Service",
		"slug":      serviceSlug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Archive the service first
	resp, err = client.DELETE("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Now archiving the group should succeed (only archived service in it)
	resp, err = client.DELETE("/api/v1/groups/" + groupSlug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestCatalog_DuplicateSlug(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	slug := testutil.RandomSlug("duplicate")

	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "First Service",
		"slug": slug,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/services", map[string]string{
		"name": "Second Service",
		"slug": slug,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()

	client.DELETE("/api/v1/services/" + slug)
}

func TestCatalog_Group_UpdateWithServiceIDs(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a group
	groupSlug := testutil.RandomSlug("group-svc-ids")
	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name": "Group For Service IDs",
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

	// Create two services
	service1Slug := testutil.RandomSlug("svc1")
	resp, err = client.POST("/api/v1/services", map[string]string{
		"name": "Service 1",
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

	service2Slug := testutil.RandomSlug("svc2")
	resp, err = client.POST("/api/v1/services", map[string]string{
		"name": "Service 2",
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

	// Add services to group via PATCH with service_ids
	resp, err = client.PATCH("/api/v1/groups/"+groupSlug, map[string]interface{}{
		"name":        "Group For Service IDs",
		"slug":        groupSlug,
		"service_ids": []string{service1ID, service2ID},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify services are linked to group
	resp, err = client.GET("/api/v1/services/" + service1Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var svc1Updated struct {
		Data struct {
			GroupIDs []string `json:"group_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc1Updated)
	assert.Contains(t, svc1Updated.Data.GroupIDs, groupID)

	resp, err = client.GET("/api/v1/services/" + service2Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var svc2Updated struct {
		Data struct {
			GroupIDs []string `json:"group_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc2Updated)
	assert.Contains(t, svc2Updated.Data.GroupIDs, groupID)

	// Change to only service1
	resp, err = client.PATCH("/api/v1/groups/"+groupSlug, map[string]interface{}{
		"name":        "Group For Service IDs",
		"slug":        groupSlug,
		"service_ids": []string{service1ID},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify service2 is no longer linked
	resp, err = client.GET("/api/v1/services/" + service2Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	testutil.DecodeJSON(t, resp, &svc2Updated)
	assert.NotContains(t, svc2Updated.Data.GroupIDs, groupID)

	// Verify service1 is still linked
	resp, err = client.GET("/api/v1/services/" + service1Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	testutil.DecodeJSON(t, resp, &svc1Updated)
	assert.Contains(t, svc1Updated.Data.GroupIDs, groupID)

	// Test empty array - removes all services
	resp, err = client.PATCH("/api/v1/groups/"+groupSlug, map[string]interface{}{
		"name":        "Group For Service IDs",
		"slug":        groupSlug,
		"service_ids": []string{},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify service1 is no longer linked
	resp, err = client.GET("/api/v1/services/" + service1Slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	testutil.DecodeJSON(t, resp, &svc1Updated)
	assert.NotContains(t, svc1Updated.Data.GroupIDs, groupID)

	// Cleanup
	client.DELETE("/api/v1/services/" + service1Slug)
	client.DELETE("/api/v1/services/" + service2Slug)
	client.DELETE("/api/v1/groups/" + groupSlug)
}

func TestCatalog_Group_UpdateWithoutServiceIDs_NoChange(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a group
	groupSlug := testutil.RandomSlug("group-no-change")
	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name": "Group No Change",
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

	// Create a service and add it to the group
	serviceSlug := testutil.RandomSlug("svc-no-change")
	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Service No Change",
		"slug":      serviceSlug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Update group without service_ids - memberships should remain
	resp, err = client.PATCH("/api/v1/groups/"+groupSlug, map[string]interface{}{
		"name":        "Group No Change Updated",
		"slug":        groupSlug,
		"description": "Updated description",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify service is still linked
	resp, err = client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var svcResult struct {
		Data struct {
			GroupIDs []string `json:"group_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svcResult)
	assert.Contains(t, svcResult.Data.GroupIDs, groupID)

	// Cleanup
	resp, err = client.PATCH("/api/v1/services/"+serviceSlug, map[string]interface{}{
		"name":      "Service No Change",
		"slug":      serviceSlug,
		"status":    "operational",
		"group_ids": []string{},
	})
	require.NoError(t, err)
	resp.Body.Close()

	client.DELETE("/api/v1/services/" + serviceSlug)
	client.DELETE("/api/v1/groups/" + groupSlug)
}

func TestEffectiveStatus_NoActiveEvents(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service without any events
	slug := testutil.RandomSlug("eff-status-no-events")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "No Events Service",
		"slug":   slug,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Get service and verify effective_status equals stored status
	resp, err = client.GET("/api/v1/services/" + slug)
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

	// Cleanup
	client.DELETE("/api/v1/services/" + slug)
}

func TestEffectiveStatus_SingleActiveEvent(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("eff-status-single")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Single Event Service",
		"slug":   slug,
		"status": "operational",
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

	// Create an incident with this service (degraded status)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Minor Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing effective status",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
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

	// Get service and verify effective_status is degraded
	resp, err = client.GET("/api/v1/services/" + slug)
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

	// Cleanup
	client.DELETE("/api/v1/events/" + eventID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestEffectiveStatus_MultipleActiveEvents_WorstCase(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("eff-status-multi")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Multi Event Service",
		"slug":   slug,
		"status": "operational",
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

	// Create first incident (degraded status)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Minor Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Minor issue",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var event1Result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &event1Result)
	event1ID := event1Result.Data.ID

	// Create second incident (major_outage status)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Critical Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "critical",
		"description": "Critical issue",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "major_outage"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var event2Result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &event2Result)
	event2ID := event2Result.Data.ID

	// Get service and verify effective_status is major_outage (worst case)
	resp, err = client.GET("/api/v1/services/" + slug)
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

	// Cleanup
	client.DELETE("/api/v1/events/" + event1ID)
	client.DELETE("/api/v1/events/" + event2ID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestEffectiveStatus_ResolvedEventIgnored(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("eff-status-resolved")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Resolved Event Service",
		"slug":   slug,
		"status": "operational",
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

	// Create an incident (major_outage status)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Soon Resolved Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "critical",
		"description": "Will be resolved",
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

	// Verify effective_status is major_outage while active
	resp, err = client.GET("/api/v1/services/" + slug)
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
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Issue resolved",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

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

	// Cleanup
	client.DELETE("/api/v1/events/" + eventID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestEffectiveStatus_CompletedMaintenanceIgnored(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("eff-status-maint-done")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Completed Maintenance Service",
		"slug":   slug,
		"status": "operational",
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

	// Create a maintenance (maintenance status)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Scheduled Maintenance",
		"type":        "maintenance",
		"status":      "in_progress",
		"description": "Will be completed",
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

	// Verify effective_status is maintenance while active
	resp, err = client.GET("/api/v1/services/" + slug)
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
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "completed",
		"message": "Maintenance completed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

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

	// Cleanup
	client.DELETE("/api/v1/events/" + eventID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestEffectiveStatus_MaintenanceVsIncident(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("eff-status-maint")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Maintenance vs Incident Service",
		"slug":   slug,
		"status": "operational",
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

	// Create maintenance (maintenance status)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Scheduled Maintenance",
		"type":        "maintenance",
		"status":      "in_progress",
		"description": "Planned maintenance",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "maintenance"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var maint struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &maint)
	maintID := maint.Data.ID

	// Create incident (degraded status, priority 2)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Minor Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Minor issue",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var incident struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &incident)
	incidentID := incident.Data.ID

	// Get service and verify effective_status is degraded (priority 2 > maintenance priority 1)
	resp, err = client.GET("/api/v1/services/" + slug)
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

	// Cleanup
	client.DELETE("/api/v1/events/" + maintID)
	client.DELETE("/api/v1/events/" + incidentID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestListServices_FilterByEffectiveStatus(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create two services
	slug1 := testutil.RandomSlug("filter-eff1")
	slug2 := testutil.RandomSlug("filter-eff2")

	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Filter Service 1",
		"slug":   slug1,
		"status": "operational",
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
		"name":   "Filter Service 2",
		"slug":   slug2,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Create an incident for service 1 (degraded status)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Filter Test Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing filter",
		"affected_services": []map[string]interface{}{
			{"service_id": service1ID, "status": "degraded"},
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

	// Filter by status=degraded should find service1
	resp, err = client.GET("/api/v1/services?status=degraded")
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

	// Cleanup
	client.DELETE("/api/v1/events/" + eventID)
	client.DELETE("/api/v1/services/" + slug1)
	client.DELETE("/api/v1/services/" + slug2)
}

func TestListServices_EffectiveStatusInResponse(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("list-eff")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "List Effective Service",
		"slug":   slug,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// List services and verify effective_status and has_active_events are present
	resp, err = client.GET("/api/v1/services")
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

	// Cleanup
	client.DELETE("/api/v1/services/" + slug)
}

func TestCatalog_EmptyList_ReturnsEmptyArray(t *testing.T) {
	// This test verifies that list endpoints return arrays [] instead of null.
	// With soft delete, we can't guarantee an empty list since demo data may have
	// services with active events that can't be archived.
	// So we verify that the response is always an array, not null.

	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a unique test service
	slug := testutil.RandomSlug("empty-test")
	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Empty Test Service",
		"slug": slug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Delete (archive) it
	resp, err = client.DELETE("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify that the list returns an array (not null)
	resp, err = client.GET("/api/v1/services")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse the raw JSON to verify data is an array, not null
	var rawResponse map[string]json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&rawResponse)
	require.NoError(t, err)
	resp.Body.Close()

	dataRaw := rawResponse["data"]
	require.NotNil(t, dataRaw, "response should have 'data' field")

	// Verify data is an array (starts with '[') not null
	dataStr := string(dataRaw)
	assert.True(t, len(dataStr) > 0 && dataStr[0] == '[',
		"data should be an array, got: %s", dataStr)
	assert.NotEqual(t, "null", dataStr, "data should not be null")

	// Verify the archived service is not in the default list
	var listResult struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	resp, err = client.GET("/api/v1/services")
	require.NoError(t, err)
	testutil.DecodeJSON(t, resp, &listResult)

	for _, svc := range listResult.Data {
		assert.NotEqual(t, slug, svc.Slug, "archived service should not appear in default list")
	}
}
