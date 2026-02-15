//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bissquit/incident-garden/internal/notifications"
	notificationspostgres "github.com/bissquit/incident-garden/internal/notifications/postgres"
	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationQueue_EnqueueAndFetch(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create a service and channel for the test
	serviceID, serviceSlug := createTestService(t, client, "queue-test-service")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create an event to reference
	eventID := createTestIncident(t, client, "Queue Test Incident",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Get db pool directly for repository testing
	repo := notificationspostgres.NewRepository(testDB)

	// Create a test channel first (need to create through API or directly)
	client.LoginAsUser(t)
	channelID := createTestEmailChannel(t, client, "queue-test@example.com")
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteTestChannel(t, client, channelID)
	})

	// Create a queue item
	item := &notifications.QueueItem{
		ID:          uuid.New().String(),
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event: notifications.EventData{
				ID:    eventID,
				Title: "Test Event",
			},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}

	// Enqueue
	err := repo.EnqueueNotification(ctx, item)
	require.NoError(t, err)

	// Fetch pending
	items, err := repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)

	fetchedItem := items[0]
	assert.Equal(t, item.ID, fetchedItem.ID)
	assert.Equal(t, item.EventID, fetchedItem.EventID)
	assert.Equal(t, item.ChannelID, fetchedItem.ChannelID)
	// Note: Status in fetched item reflects original status before update in transaction
	// The DB has already been updated to 'processing' but the scanned value was from before the update

	// Mark as sent
	err = repo.MarkAsSent(ctx, item.ID)
	require.NoError(t, err)

	// Should not appear in pending anymore
	items, err = repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, items)

	// Get stats
	stats, err := repo.GetQueueStats(ctx)
	require.NoError(t, err)
	assert.True(t, stats.Sent >= 1, "at least one sent item should exist")
}

func TestNotificationQueue_MarkForRetry(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create test data
	serviceID, serviceSlug := createTestService(t, client, "retry-test-service")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Retry Test Incident",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	repo := notificationspostgres.NewRepository(testDB)

	client.LoginAsUser(t)
	channelID := createTestEmailChannel(t, client, "retry-test@example.com")
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteTestChannel(t, client, channelID)
	})

	// Create and enqueue item
	item := &notifications.QueueItem{
		ID:          uuid.New().String(),
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeUpdate,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeUpdate,
			Event: notifications.EventData{
				ID:    eventID,
				Title: "Test Event",
			},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}

	err := repo.EnqueueNotification(ctx, item)
	require.NoError(t, err)

	// Fetch and process
	items, err := repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// Simulate failure and schedule retry
	nextAttempt := time.Now().Add(5 * time.Second)
	err = repo.MarkForRetry(ctx, item.ID, assert.AnError, nextAttempt)
	require.NoError(t, err)

	// Should not be available yet (next_attempt_at is in the future)
	items, err = repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestNotificationQueue_MarkAsFailed(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create test data
	serviceID, serviceSlug := createTestService(t, client, "failed-test-service")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Failed Test Incident",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	repo := notificationspostgres.NewRepository(testDB)

	client.LoginAsUser(t)
	channelID := createTestEmailChannel(t, client, "failed-test@example.com")
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteTestChannel(t, client, channelID)
	})

	// Create and enqueue item
	item := &notifications.QueueItem{
		ID:          uuid.New().String(),
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeResolved,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeResolved,
			Event: notifications.EventData{
				ID:    eventID,
				Title: "Test Event",
			},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}

	err := repo.EnqueueNotification(ctx, item)
	require.NoError(t, err)

	// Fetch and process
	items, err := repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// Mark as failed
	err = repo.MarkAsFailed(ctx, item.ID, assert.AnError)
	require.NoError(t, err)

	// Should not be available anymore
	items, err = repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, items)

	// Get stats
	stats, err := repo.GetQueueStats(ctx)
	require.NoError(t, err)
	assert.True(t, stats.Failed >= 1, "at least one failed item should exist")
}

func TestNotificationQueue_EnqueueBatch(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create test data
	serviceID, serviceSlug := createTestService(t, client, "batch-test-service")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Batch Test Incident",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	repo := notificationspostgres.NewRepository(testDB)

	client.LoginAsUser(t)
	channelID1 := createTestEmailChannel(t, client, "batch1@example.com")
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteTestChannel(t, client, channelID1)
	})

	channelID2 := createTestEmailChannel(t, client, "batch2@example.com")
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteTestChannel(t, client, channelID2)
	})

	// Create batch of items
	items := []*notifications.QueueItem{
		{
			ID:          uuid.New().String(),
			EventID:     eventID,
			ChannelID:   channelID1,
			MessageType: notifications.MessageTypeInitial,
			Payload: notifications.NotificationPayload{
				MessageType: notifications.MessageTypeInitial,
				Event:       notifications.EventData{ID: eventID, Title: "Test Event"},
				GeneratedAt: time.Now(),
			},
			MaxAttempts: 3,
		},
		{
			ID:          uuid.New().String(),
			EventID:     eventID,
			ChannelID:   channelID2,
			MessageType: notifications.MessageTypeInitial,
			Payload: notifications.NotificationPayload{
				MessageType: notifications.MessageTypeInitial,
				Event:       notifications.EventData{ID: eventID, Title: "Test Event"},
				GeneratedAt: time.Now(),
			},
			MaxAttempts: 3,
		},
	}

	// Enqueue batch
	err := repo.EnqueueBatch(ctx, items)
	require.NoError(t, err)

	// Fetch all
	fetched, err := repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, fetched, 2)
}

func TestNotificationQueue_DeleteOldSentItems(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create test data
	serviceID, serviceSlug := createTestService(t, client, "cleanup-test-service")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Cleanup Test Incident",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	repo := notificationspostgres.NewRepository(testDB)

	client.LoginAsUser(t)
	channelID := createTestEmailChannel(t, client, "cleanup@example.com")
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteTestChannel(t, client, channelID)
	})

	// Create and enqueue item
	item := &notifications.QueueItem{
		ID:          uuid.New().String(),
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event:       notifications.EventData{ID: eventID, Title: "Test Event"},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}

	err := repo.EnqueueNotification(ctx, item)
	require.NoError(t, err)

	// Fetch and mark as sent
	items, err := repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)

	err = repo.MarkAsSent(ctx, item.ID)
	require.NoError(t, err)

	// Verify item was marked as sent before cleanup
	statsBeforeDelete, err := repo.GetQueueStats(ctx)
	require.NoError(t, err)
	require.True(t, statsBeforeDelete.Sent >= 1, "item should be marked as sent")

	// Try to delete items older than 24 hours (our item is newer, so it should NOT be deleted)
	// Using 24 hours to account for any timezone differences between Go and Postgres container
	deleted, err := repo.DeleteOldSentItems(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted, "newly sent item should not be deleted")

	// Our item should still exist
	stats, err := repo.GetQueueStats(ctx)
	require.NoError(t, err)
	assert.True(t, stats.Sent >= 1, "sent item should still exist")
}

// createTestEmailChannel creates an email channel and returns its ID.
func createTestEmailChannel(t *testing.T, client *testutil.Client, target string) string {
	t.Helper()

	payload := map[string]interface{}{
		"type":   "email",
		"target": target,
	}

	resp, err := client.POST("/api/v1/me/channels", payload)
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

// deleteTestChannel deletes a test channel.
func deleteTestChannel(t *testing.T, client *testutil.Client, id string) {
	t.Helper()
	resp, err := client.DELETE("/api/v1/me/channels/" + id)
	if err != nil {
		t.Logf("cleanup warning: failed to delete channel %s: %v", id, err)
		return
	}
	resp.Body.Close()
}
