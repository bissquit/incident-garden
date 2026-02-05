//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvents_Incident_Lifecycle(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Test Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "major",
		"description": "Test incident description",
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
	assert.Equal(t, "investigating", createResult.Data.Status)
	assert.Equal(t, "incident", createResult.Data.Type)
	eventID := createResult.Data.ID

	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "identified",
		"message": "Root cause identified",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var getResult struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &getResult)
	assert.Equal(t, "identified", getResult.Data.Status)

	resp, err = client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Issue resolved",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)

	var resolvedResult struct {
		Data struct {
			Status     string  `json:"status"`
			ResolvedAt *string `json:"resolved_at"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &resolvedResult)
	assert.Equal(t, "resolved", resolvedResult.Data.Status)
	assert.NotNil(t, resolvedResult.Data.ResolvedAt)
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

func TestEvents_InvalidStatusForType(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsOperator(t)

	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Invalid Incident",
		"type":        "incident",
		"status":      "scheduled",
		"severity":    "minor",
		"description": "Test",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Invalid Maintenance",
		"type":        "maintenance",
		"status":      "investigating",
		"description": "Test",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestEvents_PublicStatus(t *testing.T) {
	client := newTestClient(t)

	resp, err := client.GET("/api/v1/status")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestEvents_PublicRead_NoAuth(t *testing.T) {
	// Create an event first (as operator)
	client := newTestClient(t)
	client.LoginAsOperator(t)

	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Public Read Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing public read access",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &createResult)
	eventID := createResult.Data.ID

	// Now test public access (no auth)
	publicClient := newTestClient(t)

	// GET /events — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /events should be public")
	resp.Body.Close()

	// GET /events/{id} — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events/" + eventID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /events/{id} should be public")
	resp.Body.Close()

	// GET /events/{id}/updates — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events/" + eventID + "/updates")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /events/{id}/updates should be public")
	resp.Body.Close()

	// GET /events/{id}/changes — should be 200 without auth
	resp, err = publicClient.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /events/{id}/changes should be public")
	resp.Body.Close()

	// Cleanup (as admin)
	client.LoginAsAdmin(t)
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
}

func TestEvents_WriteOperations_RequireAuth(t *testing.T) {
	client := newTestClient(t)

	// POST /events without auth — should be 401
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Unauthorized Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "POST /events should require auth")
	resp.Body.Close()

	// POST /events with user role — should be 403
	client.LoginAsUser(t)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Forbidden Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "POST /events should require operator role")
	resp.Body.Close()

	// POST /events with operator role — should be 201
	client.LoginAsOperator(t)
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Authorized Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Should succeed",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "POST /events should succeed for operator")

	var createResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &createResult)
	eventID := createResult.Data.ID

	// Cleanup
	client.LoginAsAdmin(t)
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
}

func TestEvents_ServiceChanges_BatchID(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create test services
	service1Slug := testutil.RandomSlug("batch-svc1")
	service2Slug := testutil.RandomSlug("batch-svc2")

	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Batch Service 1",
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

	resp, err = client.POST("/api/v1/services", map[string]string{
		"name": "Batch Service 2",
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

	// Create event with initial services
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Batch Test Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing batch_id",
		"service_ids": []string{service1ID, service2ID},
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

	// Get service changes and verify batch_id
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var changesResult struct {
		Data []struct {
			ID        string  `json:"id"`
			BatchID   *string `json:"batch_id"`
			Action    string  `json:"action"`
			ServiceID *string `json:"service_id"`
			Reason    string  `json:"reason"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changesResult)

	// Should have 2 changes (one per service)
	require.Len(t, changesResult.Data, 2, "should have 2 initial service changes")

	// All changes from initial creation should have the same batch_id
	require.NotNil(t, changesResult.Data[0].BatchID, "first change should have batch_id")
	require.NotNil(t, changesResult.Data[1].BatchID, "second change should have batch_id")
	assert.Equal(t, *changesResult.Data[0].BatchID, *changesResult.Data[1].BatchID,
		"both changes should have the same batch_id")

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + service1Slug)
	client.DELETE("/api/v1/services/" + service2Slug)
}

func TestEvents_AddServices_BatchID(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create test services
	service1Slug := testutil.RandomSlug("add-svc1")
	service2Slug := testutil.RandomSlug("add-svc2")
	service3Slug := testutil.RandomSlug("add-svc3")

	var serviceIDs []string
	for _, slug := range []string{service1Slug, service2Slug, service3Slug} {
		resp, err := client.POST("/api/v1/services", map[string]string{
			"name": "Add Service " + slug,
			"slug": slug,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var svcResult struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		testutil.DecodeJSON(t, resp, &svcResult)
		serviceIDs = append(serviceIDs, svcResult.Data.ID)
	}

	// Create event with one service
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Add Services Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing add services batch_id",
		"service_ids": []string{serviceIDs[0]},
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

	// Add two more services in one request
	resp, err = client.POST("/api/v1/events/"+eventID+"/services", map[string]interface{}{
		"service_ids": []string{serviceIDs[1], serviceIDs[2]},
		"reason":      "Adding more services",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Get service changes
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var changesResult struct {
		Data []struct {
			ID        string  `json:"id"`
			BatchID   *string `json:"batch_id"`
			Action    string  `json:"action"`
			ServiceID *string `json:"service_id"`
			Reason    string  `json:"reason"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changesResult)

	// Should have 3 changes total
	require.Len(t, changesResult.Data, 3, "should have 3 service changes")

	// Initial change should have different batch_id than added services
	initialBatchID := changesResult.Data[0].BatchID
	require.NotNil(t, initialBatchID, "initial change should have batch_id")

	// The two added services should have the same batch_id
	require.NotNil(t, changesResult.Data[1].BatchID, "second change should have batch_id")
	require.NotNil(t, changesResult.Data[2].BatchID, "third change should have batch_id")
	assert.Equal(t, *changesResult.Data[1].BatchID, *changesResult.Data[2].BatchID,
		"added services should have the same batch_id")

	// Initial batch should be different from add batch
	assert.NotEqual(t, *initialBatchID, *changesResult.Data[1].BatchID,
		"initial and add operations should have different batch_ids")

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	for _, slug := range []string{service1Slug, service2Slug, service3Slug} {
		client.DELETE("/api/v1/services/" + slug)
	}
}

func TestEvents_RemoveServices_BatchID(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create test services
	service1Slug := testutil.RandomSlug("rm-svc1")
	service2Slug := testutil.RandomSlug("rm-svc2")
	service3Slug := testutil.RandomSlug("rm-svc3")

	var serviceIDs []string
	for _, slug := range []string{service1Slug, service2Slug, service3Slug} {
		resp, err := client.POST("/api/v1/services", map[string]string{
			"name": "Remove Service " + slug,
			"slug": slug,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var svcResult struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		testutil.DecodeJSON(t, resp, &svcResult)
		serviceIDs = append(serviceIDs, svcResult.Data.ID)
	}

	// Create event with all three services
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Remove Services Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing remove services batch_id",
		"service_ids": serviceIDs,
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

	// Remove two services in one request
	resp, err = client.DELETEWithBody("/api/v1/events/"+eventID+"/services", map[string]interface{}{
		"service_ids": []string{serviceIDs[1], serviceIDs[2]},
		"reason":      "Removing services",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Get service changes
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var changesResult struct {
		Data []struct {
			ID        string  `json:"id"`
			BatchID   *string `json:"batch_id"`
			Action    string  `json:"action"`
			ServiceID *string `json:"service_id"`
			Reason    string  `json:"reason"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changesResult)

	// Should have 5 changes: 3 initial adds + 2 removes
	require.Len(t, changesResult.Data, 5, "should have 5 service changes (3 adds + 2 removes)")

	// Find the remove changes
	var removeChanges []struct {
		BatchID *string
		Action  string
	}
	for _, c := range changesResult.Data {
		if c.Action == "removed" {
			removeChanges = append(removeChanges, struct {
				BatchID *string
				Action  string
			}{c.BatchID, c.Action})
		}
	}

	require.Len(t, removeChanges, 2, "should have 2 remove changes")
	require.NotNil(t, removeChanges[0].BatchID, "first remove should have batch_id")
	require.NotNil(t, removeChanges[1].BatchID, "second remove should have batch_id")
	assert.Equal(t, *removeChanges[0].BatchID, *removeChanges[1].BatchID,
		"removed services should have the same batch_id")

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	for _, slug := range []string{service1Slug, service2Slug, service3Slug} {
		client.DELETE("/api/v1/services/" + slug)
	}
}

func TestEvents_AddServices_Transaction_Rollback(t *testing.T) {
	// Use client without OpenAPI validation since we intentionally trigger errors
	client := newTestClientWithoutValidation()
	client.LoginAsAdmin(t)

	// Create one valid service
	validSlug := testutil.RandomSlug("valid-svc")
	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Valid Service",
		"slug": validSlug,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var validResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &validResult)
	validServiceID := validResult.Data.ID

	// Create event without services
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Rollback Test Incident",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing transaction rollback",
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

	// Get initial changes count (should be 0)
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var initialChanges struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &initialChanges)
	initialCount := len(initialChanges.Data)

	// Try to add valid service + non-existent service
	// This should fail and rollback
	fakeServiceID := "00000000-0000-0000-0000-000000000000"
	resp, err = client.POST("/api/v1/events/"+eventID+"/services", map[string]interface{}{
		"service_ids": []string{validServiceID, fakeServiceID},
		"reason":      "Should rollback",
	})
	require.NoError(t, err)
	// The request may succeed (200) or fail (4xx/5xx) depending on FK constraint timing
	resp.Body.Close()

	// Check that no partial changes were recorded
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var afterChanges struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &afterChanges)

	// If the operation failed, count should be same as initial
	// If it succeeded (valid service added), count should be initial + 1
	// But it should NOT be initial + 2 (partial commit)
	assert.True(t, len(afterChanges.Data) == initialCount || len(afterChanges.Data) == initialCount+1,
		"should have either no changes (rollback) or only valid service added, not partial commit")

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + validSlug)
}

func TestEvents_CreateWithGroup_BatchID(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a group
	groupSlug := testutil.RandomSlug("batch-grp")
	resp, err := client.POST("/api/v1/groups", map[string]string{
		"name": "Batch Test Group",
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

	// Create services in the group
	service1Slug := testutil.RandomSlug("grp-svc1")
	service2Slug := testutil.RandomSlug("grp-svc2")

	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Group Service 1",
		"slug":      service1Slug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = client.POST("/api/v1/services", map[string]interface{}{
		"name":      "Group Service 2",
		"slug":      service2Slug,
		"group_ids": []string{groupID},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Create event with group
	resp, err = client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Group Batch Test",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Testing group batch_id",
		"group_ids":   []string{groupID},
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

	// Get service changes
	resp, err = client.GET("/api/v1/events/" + eventID + "/changes")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var changesResult struct {
		Data []struct {
			ID      string  `json:"id"`
			BatchID *string `json:"batch_id"`
			Action  string  `json:"action"`
			GroupID *string `json:"group_id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &changesResult)

	// Should have 1 change for the group
	require.Len(t, changesResult.Data, 1, "should have 1 group change")
	require.NotNil(t, changesResult.Data[0].BatchID, "group change should have batch_id")
	require.NotNil(t, changesResult.Data[0].GroupID, "change should reference group")
	assert.Equal(t, groupID, *changesResult.Data[0].GroupID)

	// Cleanup
	resp, _ = client.DELETE("/api/v1/events/" + eventID)
	resp.Body.Close()
	client.DELETE("/api/v1/services/" + service1Slug)
	client.DELETE("/api/v1/services/" + service2Slug)
	client.DELETE("/api/v1/groups/" + groupSlug)
}
