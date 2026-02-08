//go:build integration

package integration

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/require"
)

// toSlugPrefix converts a name to a valid slug prefix (lowercase, hyphens instead of spaces).
func toSlugPrefix(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	// Remove any characters that aren't alphanumeric or hyphens
	re := regexp.MustCompile(`[^a-z0-9-]`)
	s = re.ReplaceAllString(s, "")
	// Collapse multiple hyphens
	re = regexp.MustCompile(`-+`)
	s = re.ReplaceAllString(s, "-")
	// Trim leading/trailing hyphens
	s = strings.Trim(s, "-")
	if s == "" {
		s = "test"
	}
	return s
}

// createTestService creates a service and returns its ID and slug.
// Use t.Cleanup for automatic deletion.
func createTestService(t *testing.T, client *testutil.Client, name string, opts ...serviceOption) (id, slug string) {
	t.Helper()

	payload := map[string]interface{}{
		"name":   name,
		"status": "operational",
	}

	for _, opt := range opts {
		opt(payload)
	}

	// Use provided slug or generate one
	if s, ok := payload["slug"].(string); ok && s != "" {
		slug = s
	} else {
		slug = testutil.RandomSlug(toSlugPrefix(name))
		payload["slug"] = slug
	}

	resp, err := client.POST("/api/v1/services", payload)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.ID, slug
}

type serviceOption func(map[string]interface{})

func withSlug(slug string) serviceOption {
	return func(m map[string]interface{}) {
		m["slug"] = slug
	}
}

func withGroupIDs(groupIDs []string) serviceOption {
	return func(m map[string]interface{}) {
		m["group_ids"] = groupIDs
	}
}

func withStatus(status string) serviceOption {
	return func(m map[string]interface{}) {
		m["status"] = status
	}
}

func withDescription(description string) serviceOption {
	return func(m map[string]interface{}) {
		m["description"] = description
	}
}

// createTestGroup creates a group and returns its ID and slug.
func createTestGroup(t *testing.T, client *testutil.Client, name string, opts ...groupOption) (id, slug string) {
	t.Helper()

	payload := map[string]interface{}{
		"name": name,
	}

	for _, opt := range opts {
		opt(payload)
	}

	// Use provided slug or generate one
	if s, ok := payload["slug"].(string); ok && s != "" {
		slug = s
	} else {
		slug = testutil.RandomSlug(toSlugPrefix(name))
		payload["slug"] = slug
	}

	resp, err := client.POST("/api/v1/groups", payload)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.ID, slug
}

type groupOption func(map[string]interface{})

func withGroupSlug(slug string) groupOption {
	return func(m map[string]interface{}) {
		m["slug"] = slug
	}
}

func withGroupDescription(description string) groupOption {
	return func(m map[string]interface{}) {
		m["description"] = description
	}
}

// AffectedService describes a service for event creation.
type AffectedService struct {
	ServiceID string
	Status    string
}

// AffectedGroup describes a group for event creation.
type AffectedGroup struct {
	GroupID string
	Status  string
}

// createTestIncident creates an incident and returns its ID.
func createTestIncident(t *testing.T, client *testutil.Client, title string, services []AffectedService, groups []AffectedGroup) string {
	t.Helper()

	payload := map[string]interface{}{
		"title":       title,
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Test incident description",
	}

	if len(services) > 0 {
		affected := make([]map[string]string, len(services))
		for i, s := range services {
			affected[i] = map[string]string{
				"service_id": s.ServiceID,
				"status":     s.Status,
			}
		}
		payload["affected_services"] = affected
	}

	if len(groups) > 0 {
		affected := make([]map[string]string, len(groups))
		for i, g := range groups {
			affected[i] = map[string]string{
				"group_id": g.GroupID,
				"status":   g.Status,
			}
		}
		payload["affected_groups"] = affected
	}

	resp, err := client.POST("/api/v1/events", payload)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.ID
}

// resolveEvent resolves an incident (incident -> resolved).
func resolveEvent(t *testing.T, client *testutil.Client, eventID string) {
	t.Helper()
	resp, err := client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "resolved",
		"message": "Fixed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

// completeMaintenance completes a maintenance event (maintenance -> completed).
func completeMaintenance(t *testing.T, client *testutil.Client, eventID string) {
	t.Helper()
	resp, err := client.POST("/api/v1/events/"+eventID+"/updates", map[string]interface{}{
		"status":  "completed",
		"message": "Maintenance completed",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

// createTestMaintenance creates a maintenance event and returns its ID.
func createTestMaintenance(t *testing.T, client *testutil.Client, title string, services []AffectedService) string {
	t.Helper()

	payload := map[string]interface{}{
		"title":       title,
		"type":        "maintenance",
		"status":      "in_progress",
		"description": "Test maintenance description",
	}

	if len(services) > 0 {
		affected := make([]map[string]string, len(services))
		for i, s := range services {
			affected[i] = map[string]string{
				"service_id": s.ServiceID,
				"status":     s.Status,
			}
		}
		payload["affected_services"] = affected
	}

	resp, err := client.POST("/api/v1/events", payload)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.ID
}

// deleteService soft-deletes a service. Does not fail if already deleted or has active events.
func deleteService(t *testing.T, client *testutil.Client, slug string) {
	t.Helper()
	resp, err := client.DELETE("/api/v1/services/" + slug)
	if err != nil {
		t.Logf("cleanup warning (service %s): %v", slug, err)
		return
	}
	if resp.StatusCode == http.StatusConflict {
		t.Logf("cleanup warning (service %s): has active events", slug)
	}
	resp.Body.Close()
}

// deleteGroup soft-deletes a group. Does not fail if already deleted or has active services.
func deleteGroup(t *testing.T, client *testutil.Client, slug string) {
	t.Helper()
	resp, err := client.DELETE("/api/v1/groups/" + slug)
	if err != nil {
		t.Logf("cleanup warning (group %s): %v", slug, err)
		return
	}
	if resp.StatusCode == http.StatusConflict {
		t.Logf("cleanup warning (group %s): has active services", slug)
	}
	resp.Body.Close()
}

// deleteEvent deletes an event. Does not fail if already deleted.
func deleteEvent(t *testing.T, client *testutil.Client, eventID string) {
	t.Helper()
	resp, err := client.DELETE("/api/v1/events/" + eventID)
	if err != nil {
		t.Logf("cleanup warning (event %s): %v", eventID, err)
		return
	}
	resp.Body.Close()
}

// getServiceEffectiveStatus gets the effective_status of a service.
func getServiceEffectiveStatus(t *testing.T, client *testutil.Client, slug string) string {
	t.Helper()
	resp, err := client.GET("/api/v1/services/" + slug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Data struct {
			EffectiveStatus string `json:"effective_status"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.EffectiveStatus
}
