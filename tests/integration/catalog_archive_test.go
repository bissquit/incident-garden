//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog_Service_ArchiveWithActiveEvent_Blocked(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "Service With Active Event")

	// Create an active incident with this service
	eventID := createTestIncident(t, client, "Active Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// Try to archive the service - should fail with 409
	resp, err := client.DELETE("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var errorResult struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.DecodeJSON(t, resp, &errorResult)
	assert.Contains(t, errorResult.Error.Message, "active events")

	// Resolve the event
	client.LoginAsOperator(t)
	resolveEvent(t, client, eventID)

	// Now archiving should succeed
	client.LoginAsAdmin(t)
	resp, err = client.DELETE("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestCatalog_Service_Restore(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "Restore Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Archive the service
	resp, err := client.DELETE("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify it's archived
	resp, err = client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var archivedResult struct {
		Data struct {
			ArchivedAt *string `json:"archived_at"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &archivedResult)
	assert.NotNil(t, archivedResult.Data.ArchivedAt, "service should be archived")

	// Restore the service
	resp, err = client.POST("/api/v1/services/"+slug+"/restore", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var restoreResult struct {
		Data struct {
			Slug       string  `json:"slug"`
			ArchivedAt *string `json:"archived_at"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &restoreResult)
	assert.Equal(t, slug, restoreResult.Data.Slug)
	assert.Nil(t, restoreResult.Data.ArchivedAt, "archived_at should be null after restore")

	// Verify it appears in default list
	resp, err = client.GET("/api/v1/services")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listResult struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &listResult)

	found := false
	for _, svc := range listResult.Data {
		if svc.Slug == slug {
			found = true
			break
		}
	}
	assert.True(t, found, "restored service should appear in default list")
}

func TestCatalog_Service_RestoreNotFound(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.POST("/api/v1/services/nonexistent-slug/restore", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestCatalog_Service_RestoreNotArchived(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "Not Archived Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Try to restore non-archived service - should fail
	resp, err := client.POST("/api/v1/services/"+slug+"/restore", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()
}

func TestCatalog_Group_Restore(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestGroup(t, client, "Restore Group")
	t.Cleanup(func() { deleteGroup(t, client, slug) })

	// Archive the group
	resp, err := client.DELETE("/api/v1/groups/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify it's archived
	resp, err = client.GET("/api/v1/groups/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var archivedResult struct {
		Data struct {
			ArchivedAt *string `json:"archived_at"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &archivedResult)
	assert.NotNil(t, archivedResult.Data.ArchivedAt, "group should be archived")

	// Restore the group
	resp, err = client.POST("/api/v1/groups/"+slug+"/restore", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var restoreResult struct {
		Data struct {
			Slug       string  `json:"slug"`
			ArchivedAt *string `json:"archived_at"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &restoreResult)
	assert.Equal(t, slug, restoreResult.Data.Slug)
	assert.Nil(t, restoreResult.Data.ArchivedAt, "archived_at should be null after restore")

	// Verify it appears in default list
	resp, err = client.GET("/api/v1/groups")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listResult struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &listResult)

	found := false
	for _, grp := range listResult.Data {
		if grp.Slug == slug {
			found = true
			break
		}
	}
	assert.True(t, found, "restored group should appear in default list")
}

func TestCatalog_Group_RestoreNotFound(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	resp, err := client.POST("/api/v1/groups/nonexistent-slug/restore", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestCatalog_Group_RestoreNotArchived(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestGroup(t, client, "Not Archived Group")
	t.Cleanup(func() { deleteGroup(t, client, slug) })

	// Try to restore non-archived group - should fail
	resp, err := client.POST("/api/v1/groups/"+slug+"/restore", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()
}

func TestCatalog_Group_ArchiveWithServices_Blocked(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupID, groupSlug := createTestGroup(t, client, "Group With Service")
	t.Cleanup(func() { deleteGroup(t, client, groupSlug) })

	_, serviceSlug := createTestService(t, client, "Service in Group", withGroupIDs([]string{groupID}))
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Try to archive the group - should fail with 409
	resp, err := client.DELETE("/api/v1/groups/" + groupSlug)
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
}

func TestCatalog_Group_ArchiveWithArchivedServices_Allowed(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	groupID, groupSlug := createTestGroup(t, client, "Group With Archived Service")

	_, serviceSlug := createTestService(t, client, "Soon Archived Service", withGroupIDs([]string{groupID}))

	// Archive the service first
	resp, err := client.DELETE("/api/v1/services/" + serviceSlug)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Now archiving the group should succeed (only archived service in it)
	resp, err = client.DELETE("/api/v1/groups/" + groupSlug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}
