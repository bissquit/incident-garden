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

func TestCatalog_Group_UpdateWithServiceIDs(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupID, groupSlug := createTestGroup(t, client, "Group For Service IDs")
	t.Cleanup(func() { deleteGroup(t, client, groupSlug) })

	service1ID, service1Slug := createTestService(t, client, "Service 1")
	t.Cleanup(func() { deleteService(t, client, service1Slug) })

	service2ID, service2Slug := createTestService(t, client, "Service 2")
	t.Cleanup(func() { deleteService(t, client, service2Slug) })

	// Add services to group via PATCH with service_ids
	resp, err := client.PATCH("/api/v1/groups/"+groupSlug, map[string]interface{}{
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

}

func TestCatalog_Group_UpdateWithoutServiceIDs_NoChange(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupID, groupSlug := createTestGroup(t, client, "Group No Change")
	t.Cleanup(func() { deleteGroup(t, client, groupSlug) })

	_, serviceSlug := createTestService(t, client, "Service No Change", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Update group without service_ids - memberships should remain
	resp, err := client.PATCH("/api/v1/groups/"+groupSlug, map[string]interface{}{
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

	// Cleanup: remove service from group before deletion
	resp, err = client.PATCH("/api/v1/services/"+serviceSlug, map[string]interface{}{
		"name":      "Service No Change",
		"slug":      serviceSlug,
		"status":    "operational",
		"group_ids": []string{},
	})
	require.NoError(t, err)
	resp.Body.Close()
}

func TestCatalog_EmptyList_ReturnsEmptyArray(t *testing.T) {
	// This test verifies that list endpoints return arrays [] instead of null.
	// With soft delete, we can't guarantee an empty list since demo data may have
	// services with active events that can't be archived.
	// So we verify that the response is always an array, not null.

	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "Empty Test Service")

	// Delete (archive) it
	deleteService(t, client, slug)

	// Verify that the list returns an array (not null)
	resp, err := client.GET("/api/v1/services")
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
