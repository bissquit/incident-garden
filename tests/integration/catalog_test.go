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

func TestStatusLog_ManualChange(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("status-log-manual")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Status Log Manual",
		"slug":   slug,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Change status with reason
	resp, err = client.PATCH("/api/v1/services/"+slug, map[string]interface{}{
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

	// Cleanup
	client.DELETE("/api/v1/services/" + slug)
}

func TestStatusLog_EventChange(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("status-log-event")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Status Log Event",
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

	// Create an incident
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Status Log Test Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing status log from event",
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

	// Get status log
	resp, err = client.GET("/api/v1/services/" + slug + "/status-log")
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

	// Cleanup
	client.DELETE("/api/v1/events/" + eventID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestStatusLog_EventUpdate(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("status-log-update")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Status Log Update",
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

	// Create an incident with degraded status
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Status Log Update Test Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing status log from event update",
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

	// Update the event with service_updates to change status to major_outage
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
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

	// Cleanup
	client.DELETE("/api/v1/events/" + eventID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestStatusLog_EventResolved(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("status-log-resolved")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Status Log Resolved",
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

	// Create an incident
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Resolved Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
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

	// Resolve the incident
	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Issue resolved",
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

	// Cleanup
	client.DELETE("/api/v1/events/" + eventID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestStatusLog_Pagination(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("status-log-pag")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Status Log Pagination",
		"slug":   slug,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Change status multiple times
	statuses := []string{"degraded", "partial_outage", "major_outage", "operational"}
	for _, status := range statuses {
		resp, err = client.PATCH("/api/v1/services/"+slug, map[string]interface{}{
			"name":   "Status Log Pagination",
			"slug":   slug,
			"status": status,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// Get with limit=2
	resp, err = client.GET("/api/v1/services/" + slug + "/status-log?limit=2")
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

	// Cleanup
	client.DELETE("/api/v1/services/" + slug)
}

func TestStatusLog_RequiresAuth(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("status-log-auth")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Status Log Auth",
		"slug":   slug,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Try to access without auth
	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/status-log")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// Cleanup
	client.DELETE("/api/v1/services/" + slug)
}

func TestStatusLog_RequiresOperatorRole(t *testing.T) {
	adminClient := newTestClient(t)
	adminClient.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("status-log-role")
	resp, err := adminClient.POST("/api/v1/services", map[string]interface{}{
		"name":   "Status Log Role",
		"slug":   slug,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Try as user (should fail with 403)
	userClient := newTestClient(t)
	userClient.LoginAsUser(t)
	resp, err = userClient.GET("/api/v1/services/" + slug + "/status-log")
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

	// Cleanup
	adminClient.DELETE("/api/v1/services/" + slug)
}

func TestGetServiceEvents_NoEvents(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service without any events
	slug := testutil.RandomSlug("svc-events-none")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "No Events Service",
		"slug":   slug,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// GET /services/{slug}/events without auth (public endpoint)
	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/events")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Events []interface{} `json:"events"`
			Total  int           `json:"total"`
			Limit  int           `json:"limit"`
			Offset int           `json:"offset"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.Equal(t, 0, len(result.Data.Events))
	assert.Equal(t, 0, result.Data.Total)
	assert.Equal(t, 20, result.Data.Limit)
	assert.Equal(t, 0, result.Data.Offset)

	// Cleanup
	client.DELETE("/api/v1/services/" + slug)
}

func TestGetServiceEvents_WithActiveEvent(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("svc-events-active")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Active Event Service",
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

	// Create an active incident
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Active Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing service events",
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

	// GET /services/{slug}/events
	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/events")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Events []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"events"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	assert.Equal(t, 1, result.Data.Total)
	require.Len(t, result.Data.Events, 1)
	assert.Equal(t, eventID, result.Data.Events[0].ID)
	assert.Equal(t, "Active Incident", result.Data.Events[0].Title)

	// Cleanup
	client.DELETE("/api/v1/events/" + eventID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestGetServiceEvents_FilterActive(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("svc-events-filter-active")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Filter Active Service",
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

	// Create an active incident
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Active Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Active",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var activeEvent struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &activeEvent)

	// Create and resolve another incident
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Resolved Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Will be resolved",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var resolvedEvent struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &resolvedEvent)

	// Resolve the second incident
	resp, err = client.POST("/api/v1/events/"+resolvedEvent.Data.ID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Fixed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Filter by active
	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/events?status=active")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var activeResult struct {
		Data struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &activeResult)

	assert.Equal(t, 1, activeResult.Data.Total)
	require.Len(t, activeResult.Data.Events, 1)
	assert.Equal(t, activeEvent.Data.ID, activeResult.Data.Events[0].ID)

	// Cleanup
	client.DELETE("/api/v1/events/" + activeEvent.Data.ID)
	client.DELETE("/api/v1/events/" + resolvedEvent.Data.ID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestGetServiceEvents_FilterResolved(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("svc-events-filter-resolved")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Filter Resolved Service",
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

	// Create an active incident
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Active Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Active",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var activeEvent struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &activeEvent)

	// Create and resolve another incident
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Resolved Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Will be resolved",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var resolvedEvent struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &resolvedEvent)

	// Resolve the second incident
	resp, err = client.POST("/api/v1/events/"+resolvedEvent.Data.ID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Fixed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Filter by resolved
	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/events?status=resolved")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var resolvedResult struct {
		Data struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &resolvedResult)

	assert.Equal(t, 1, resolvedResult.Data.Total)
	require.Len(t, resolvedResult.Data.Events, 1)
	assert.Equal(t, resolvedEvent.Data.ID, resolvedResult.Data.Events[0].ID)

	// Cleanup
	client.DELETE("/api/v1/events/" + activeEvent.Data.ID)
	client.DELETE("/api/v1/events/" + resolvedEvent.Data.ID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestGetServiceEvents_Pagination(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("svc-events-pagination")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Pagination Service",
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

	// Create 3 events
	eventIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		resp, err = client.POST("/api/v1/events", map[string]interface{}{
			"title":       "Incident " + string(rune('A'+i)),
			"type":        "incident",
			"status":      "investigating",
			"severity":    "minor",
			"description": "Testing pagination",
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
		eventIDs[i] = eventResult.Data.ID
	}

	publicClient := newTestClient(t)

	// Get with limit=2
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/events?limit=2")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var page1 struct {
		Data struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &page1)

	assert.Equal(t, 3, page1.Data.Total)
	assert.Equal(t, 2, len(page1.Data.Events))
	assert.Equal(t, 2, page1.Data.Limit)
	assert.Equal(t, 0, page1.Data.Offset)

	// Get with offset=2
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/events?limit=2&offset=2")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var page2 struct {
		Data struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
			Offset int `json:"offset"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &page2)

	assert.Equal(t, 1, len(page2.Data.Events))
	assert.Equal(t, 2, page2.Data.Offset)

	// Cleanup
	for _, id := range eventIDs {
		client.DELETE("/api/v1/events/" + id)
	}
	client.DELETE("/api/v1/services/" + slug)
}

func TestGetServiceEvents_SortOrder(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("svc-events-sort")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Sort Order Service",
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

	// Create active incident first (older)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Active Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Active",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var activeEvent struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &activeEvent)

	// Create and resolve another incident (newer)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Resolved Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Will be resolved",
		"affected_services": []map[string]interface{}{
			{"service_id": serviceID, "status": "degraded"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var resolvedEvent struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &resolvedEvent)

	// Resolve the second incident
	resp, err = client.POST("/api/v1/events/"+resolvedEvent.Data.ID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Fixed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Get all events - active should be first despite being older
	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/events")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			Events []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"events"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	require.Len(t, result.Data.Events, 2)
	// Active event should be first regardless of creation order
	assert.Equal(t, "investigating", result.Data.Events[0].Status, "active event should be first")
	assert.Equal(t, "resolved", result.Data.Events[1].Status, "resolved event should be second")

	// Cleanup
	client.DELETE("/api/v1/events/" + activeEvent.Data.ID)
	client.DELETE("/api/v1/events/" + resolvedEvent.Data.ID)
	client.DELETE("/api/v1/services/" + slug)
}

func TestGetServiceEvents_ServiceNotFound(t *testing.T) {
	publicClient := newTestClient(t)

	resp, err := publicClient.GET("/api/v1/services/nonexistent-service-slug/events")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestGetServiceEvents_MultipleServices(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create two services
	slug1 := testutil.RandomSlug("svc-multi-1")
	slug2 := testutil.RandomSlug("svc-multi-2")

	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Multi Service 1",
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

	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Multi Service 2",
		"slug":   slug2,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var svc2Result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &svc2Result)

	// Create an incident with both services
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Multi Service Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Affects both services",
		"affected_services": []map[string]interface{}{
			{"service_id": svc1Result.Data.ID, "status": "degraded"},
			{"service_id": svc2Result.Data.ID, "status": "degraded"},
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

	publicClient := newTestClient(t)

	// Both services should return this event
	resp, err = publicClient.GET("/api/v1/services/" + slug1 + "/events")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result1 struct {
		Data struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result1)
	assert.Equal(t, 1, result1.Data.Total)
	require.Len(t, result1.Data.Events, 1)
	assert.Equal(t, eventID, result1.Data.Events[0].ID)

	resp, err = publicClient.GET("/api/v1/services/" + slug2 + "/events")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result2 struct {
		Data struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
			Total int `json:"total"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result2)
	assert.Equal(t, 1, result2.Data.Total)
	require.Len(t, result2.Data.Events, 1)
	assert.Equal(t, eventID, result2.Data.Events[0].ID)

	// Cleanup
	client.DELETE("/api/v1/events/" + eventID)
	client.DELETE("/api/v1/services/" + slug1)
	client.DELETE("/api/v1/services/" + slug2)
}

func TestGetServiceEvents_PublicEndpoint(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("svc-events-public")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Public Endpoint Service",
		"slug":   slug,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Should work without auth
	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/events")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Cleanup
	client.DELETE("/api/v1/services/" + slug)
}

func TestGetServiceEvents_InvalidStatusFilter(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service
	slug := testutil.RandomSlug("svc-events-invalid")
	resp, err := client.POST("/api/v1/services", map[string]interface{}{
		"name":   "Invalid Filter Service",
		"slug":   slug,
		"status": "operational",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	publicClient := newTestClient(t)
	resp, err = publicClient.GET("/api/v1/services/" + slug + "/events?status=invalid")
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// Cleanup
	client.DELETE("/api/v1/services/" + slug)
}
