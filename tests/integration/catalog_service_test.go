//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var getResult struct {
		Data struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &getResult)
	assert.NotEmpty(t, getResult.Data.ID)
	assert.Equal(t, slug, getResult.Data.Slug)

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

	groupID, groupSlug := createTestGroup(t, client, "Parent Group")
	t.Cleanup(func() { deleteGroup(t, client, groupSlug) })

	_, serviceSlug := createTestService(t, client, "Service in Group", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Verify group_ids in response
	resp, err := client.GET("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var serviceResult struct {
		Data struct {
			GroupIDs []string `json:"group_ids"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &serviceResult)
	require.NotNil(t, serviceResult.Data.GroupIDs)
	require.Len(t, serviceResult.Data.GroupIDs, 1)
	assert.Equal(t, groupID, serviceResult.Data.GroupIDs[0])
}

func TestCatalog_DuplicateSlug(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "First Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Try to create a second service with same slug - should fail
	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Second Service",
		"slug": slug,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()
}
