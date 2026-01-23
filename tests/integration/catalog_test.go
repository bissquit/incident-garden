//go:build integration

package integration

import (
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
		"name":     "Service in Group",
		"slug":     serviceSlug,
		"group_id": groupID,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var serviceResult struct {
		Data struct {
			GroupID *string `json:"group_id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &serviceResult)
	require.NotNil(t, serviceResult.Data.GroupID)
	assert.Equal(t, groupID, *serviceResult.Data.GroupID)

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
