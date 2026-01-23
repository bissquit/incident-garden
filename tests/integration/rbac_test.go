//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBAC_UserCannotCreateService(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
	client.LoginAsUser(t)

	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Forbidden Service",
		"slug": testutil.RandomSlug("forbidden"),
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

func TestRBAC_UserCannotCreateEvent(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
	client.LoginAsUser(t)

	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Forbidden Event",
		"type":        "incident",
		"status":      "investigating",
		"description": "Test",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

func TestRBAC_UserCanAccessOwnChannels(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
	client.LoginAsUser(t)

	resp, err := client.POST("/api/v1/me/channels", map[string]string{
		"type":   "email",
		"target": testutil.RandomEmail(),
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)

	resp, err = client.DELETE("/api/v1/me/channels/" + result.Data.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestRBAC_AdminCanCreateService(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
	client.LoginAsAdmin(t)

	slug := testutil.RandomSlug("admin-service")
	resp, err := client.POST("/api/v1/services", map[string]string{
		"name": "Admin Service",
		"slug": slug,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	client.DELETE("/api/v1/services/" + slug)
}

func TestRBAC_OperatorCanCreateEvent(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)
	client.LoginAsOperator(t)

	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":       "Operator Event",
		"type":        "incident",
		"status":      "investigating",
		"severity":    "minor",
		"description": "Test",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

func TestRBAC_UnauthenticatedCanReadPublicEndpoints(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)

	tests := []struct {
		name string
		path string
	}{
		{"status", "/api/v1/status"},
		{"services", "/api/v1/services"},
		{"groups", "/api/v1/groups"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.GET(tt.path)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()
		})
	}
}

func TestRBAC_UnauthenticatedCannotAccessProtectedEndpoints(t *testing.T) {
	client := testutil.NewClient(testClient.BaseURL)

	tests := []struct {
		name string
		path string
	}{
		{"me", "/api/v1/me"},
		{"channels", "/api/v1/me/channels"},
		{"subscriptions", "/api/v1/me/subscriptions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.GET(tt.path)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			resp.Body.Close()
		})
	}
}
