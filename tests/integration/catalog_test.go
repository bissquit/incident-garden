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
