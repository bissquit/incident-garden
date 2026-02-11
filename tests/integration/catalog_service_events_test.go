//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetServiceEvents_NoEvents(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "No Events Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// GET /services/{slug}/events without auth (public endpoint)
	publicClient := newTestClient(t)
	resp, err := publicClient.GET("/api/v1/services/" + slug + "/events")
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
}

func TestGetServiceEvents_WithActiveEvent(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Active Event Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	eventID := createTestIncident(t, client, "Active Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	// GET /services/{slug}/events
	publicClient := newTestClient(t)
	resp, err := publicClient.GET("/api/v1/services/" + slug + "/events")
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
}

func TestGetServiceEvents_FilterActive(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Filter Active Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	activeEventID := createTestIncident(t, client, "Active Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, activeEventID) })

	resolvedEventID := createTestIncident(t, client, "Resolved Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, resolvedEventID) })

	resolveEvent(t, client, resolvedEventID)

	// Filter by active
	publicClient := newTestClient(t)
	resp, err := publicClient.GET("/api/v1/services/" + slug + "/events?status=active")
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
	assert.Equal(t, activeEventID, activeResult.Data.Events[0].ID)
}

func TestGetServiceEvents_FilterResolved(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Filter Resolved Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	activeEventID := createTestIncident(t, client, "Active Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, activeEventID) })

	resolvedEventID := createTestIncident(t, client, "Resolved Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, resolvedEventID) })

	resolveEvent(t, client, resolvedEventID)

	// Filter by resolved
	publicClient := newTestClient(t)
	resp, err := publicClient.GET("/api/v1/services/" + slug + "/events?status=resolved")
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
	assert.Equal(t, resolvedEventID, resolvedResult.Data.Events[0].ID)
}

func TestGetServiceEvents_Pagination(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Pagination Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Create 3 events
	eventIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		eventID := createTestIncident(t, client, "Incident "+string(rune('A'+i)), []AffectedService{
			{ServiceID: serviceID, Status: "degraded"},
		}, nil)
		eventIDs[i] = eventID
	}
	t.Cleanup(func() {
		for _, id := range eventIDs {
			deleteEvent(t, client, id)
		}
	})

	publicClient := newTestClient(t)

	// Get with limit=2
	resp, err := publicClient.GET("/api/v1/services/" + slug + "/events?limit=2")
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
}

func TestGetServiceEvents_SortOrder(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, slug := createTestService(t, client, "Sort Order Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Create active incident first (older)
	activeEventID := createTestIncident(t, client, "Active Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, activeEventID) })

	// Create and resolve another incident (newer)
	resolvedEventID := createTestIncident(t, client, "Resolved Incident", []AffectedService{
		{ServiceID: serviceID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, resolvedEventID) })

	resolveEvent(t, client, resolvedEventID)

	// Get all events - active should be first despite being older
	publicClient := newTestClient(t)
	resp, err := publicClient.GET("/api/v1/services/" + slug + "/events")
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

	svc1ID, slug1 := createTestService(t, client, "Multi Service 1")
	t.Cleanup(func() { deleteService(t, client, slug1) })

	svc2ID, slug2 := createTestService(t, client, "Multi Service 2")
	t.Cleanup(func() { deleteService(t, client, slug2) })

	// Create an incident with both services
	eventID := createTestIncident(t, client, "Multi Service Incident", []AffectedService{
		{ServiceID: svc1ID, Status: "degraded"},
		{ServiceID: svc2ID, Status: "degraded"},
	}, nil)
	t.Cleanup(func() { deleteEvent(t, client, eventID) })

	publicClient := newTestClient(t)

	// Both services should return this event
	resp, err := publicClient.GET("/api/v1/services/" + slug1 + "/events")
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
}

func TestGetServiceEvents_PublicEndpoint(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "Public Endpoint Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	// Should work without auth
	publicClient := newTestClient(t)
	resp, err := publicClient.GET("/api/v1/services/" + slug + "/events")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestGetServiceEvents_InvalidStatusFilter(t *testing.T) {
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	_, slug := createTestService(t, client, "Invalid Filter Service")
	t.Cleanup(func() { deleteService(t, client, slug) })

	publicClient := newTestClient(t)
	resp, err := publicClient.GET("/api/v1/services/" + slug + "/events?status=invalid")
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}
