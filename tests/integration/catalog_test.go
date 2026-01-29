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
	client := testutil.NewClient(testClient.BaseURL)
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

	publicClient := testutil.NewClient(testClient.BaseURL)
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

	resp, err = client.DELETE("/api/v1/groups/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	resp, err = publicClient.GET("/api/v1/groups/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestCatalog_Service_CRUD(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
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

	publicClient := testutil.NewClient(testClient.BaseURL)
	resp, err = publicClient.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.DELETE("/api/v1/services/" + slug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestCatalog_Service_WithGroup(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
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

func TestCatalog_DuplicateSlug(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
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
	// This test verifies that list endpoints return empty arrays [] instead of null
	// when no data exists. This is important for API consistency and type safety.

	client := testutil.NewClient(testClient.BaseURL)
	client.LoginAsAdmin(t)

	// First, clean up any existing services by deleting them
	resp, err := client.GET("/api/v1/services")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listResult struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &listResult)

	// Delete all existing services
	for _, svc := range listResult.Data {
		resp, err := client.DELETE("/api/v1/services/" + svc.Slug)
		require.NoError(t, err)
		resp.Body.Close()
	}

	// Now verify that the empty list returns [] not null
	resp, err = client.GET("/api/v1/services")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse the raw JSON to verify it's an empty array, not null
	var rawResponse map[string]json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&rawResponse)
	require.NoError(t, err)
	resp.Body.Close()

	dataRaw := rawResponse["data"]
	require.NotNil(t, dataRaw, "response should have 'data' field")

	// Check that data is an empty array [], not null
	// null in JSON is represented as "null" string
	// empty array is represented as "[]"
	dataStr := string(dataRaw)
	assert.Equal(t, "[]", dataStr, "empty list should return [] not null")
}
