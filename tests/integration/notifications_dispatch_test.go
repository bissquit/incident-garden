//go:build integration

package integration

import (
	"context"
	"errors"
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

// =============================================================================
// Send Success Tests
// =============================================================================

func TestDispatch_Email_Success(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	// Create test data
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "dispatch-email-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Dispatch Email Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create verified email channel
	client.LoginAsUser(t)
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, channelID)
	})

	// Enqueue notification
	item := &notifications.QueueItem{
		ID:          uuid.New().String(),
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event: notifications.EventData{
				ID:    eventID,
				Title: "Dispatch Email Test",
			},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}
	require.NoError(t, repo.EnqueueNotification(ctx, item))

	// Start worker and wait for processing
	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Wait for notification to be sent
	success := mocks.Email.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "email should be sent")

	sent := mocks.Email.GetSent()
	require.Len(t, sent, 1)
	assert.Contains(t, sent[0].To, "@example.com")
	assert.NotEmpty(t, sent[0].Subject)
	assert.NotEmpty(t, sent[0].Body)
}

func TestDispatch_Telegram_Success(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	// Create test data
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "dispatch-telegram-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Dispatch Telegram Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create telegram channel and verify it
	client.LoginAsUser(t)
	channelID := createTelegramChannel(t, client, "123456789")
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, channelID)
	})
	verifyTelegramChannel(t, client, channelID)

	// Enqueue notification
	item := &notifications.QueueItem{
		ID:          uuid.New().String(),
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event: notifications.EventData{
				ID:    eventID,
				Title: "Dispatch Telegram Test",
			},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}
	require.NoError(t, repo.EnqueueNotification(ctx, item))

	// Start worker and wait for processing
	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Wait for notification to be sent
	success := mocks.Telegram.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "telegram message should be sent")

	sent := mocks.Telegram.GetSent()
	require.Len(t, sent, 1)
	assert.Equal(t, "123456789", sent[0].To)
}

func TestDispatch_Mattermost_Success(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	// Create test data
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "dispatch-mattermost-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Dispatch Mattermost Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create mattermost channel and verify it
	client.LoginAsUser(t)
	channelID := createMattermostChannel(t, client, "https://mm.example.com/hooks/test123")
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, channelID)
	})
	verifyMattermostChannel(t, client, channelID)

	// Enqueue notification
	item := &notifications.QueueItem{
		ID:          uuid.New().String(),
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event: notifications.EventData{
				ID:    eventID,
				Title: "Dispatch Mattermost Test",
			},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}
	require.NoError(t, repo.EnqueueNotification(ctx, item))

	// Start worker and wait for processing
	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Wait for notification to be sent
	success := mocks.Mattermost.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "mattermost message should be sent")

	sent := mocks.Mattermost.GetSent()
	require.Len(t, sent, 1)
	assert.Equal(t, "https://mm.example.com/hooks/test123", sent[0].To)
}

func TestDispatch_MultipleChannels_SendsToAll(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		NumWorkers:        2,
	}, repo, dispatcher, renderer)

	// Create test data
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "dispatch-multi-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Dispatch Multi Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create multiple channels
	client.LoginAsUser(t)
	emailChannelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, emailChannelID)
	})

	telegramChannelID := createTelegramChannel(t, client, "multi-tg-123")
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, telegramChannelID)
	})
	verifyTelegramChannel(t, client, telegramChannelID)

	// Enqueue notifications for both
	items := []*notifications.QueueItem{
		{
			ID:          uuid.New().String(),
			EventID:     eventID,
			ChannelID:   emailChannelID,
			MessageType: notifications.MessageTypeInitial,
			Payload: notifications.NotificationPayload{
				MessageType: notifications.MessageTypeInitial,
				Event:       notifications.EventData{ID: eventID, Title: "Multi Test"},
				GeneratedAt: time.Now(),
			},
			MaxAttempts: 3,
		},
		{
			ID:          uuid.New().String(),
			EventID:     eventID,
			ChannelID:   telegramChannelID,
			MessageType: notifications.MessageTypeInitial,
			Payload: notifications.NotificationPayload{
				MessageType: notifications.MessageTypeInitial,
				Event:       notifications.EventData{ID: eventID, Title: "Multi Test"},
				GeneratedAt: time.Now(),
			},
			MaxAttempts: 3,
		},
	}
	require.NoError(t, repo.EnqueueBatch(ctx, items))

	// Start worker and wait for processing
	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Wait for both notifications
	success := mocks.WaitForNotifications(2, 3*time.Second)
	require.True(t, success, "both notifications should be sent")

	assert.Equal(t, 1, mocks.Email.SentCount())
	assert.Equal(t, 1, mocks.Telegram.SentCount())
}

// =============================================================================
// Retry Tests
// =============================================================================

func TestDispatch_RetryScheduling_Works(t *testing.T) {
	// Test that retry scheduling correctly sets next_attempt_at
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)

	// Create test data
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "dispatch-retry-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Dispatch Retry Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	client.LoginAsUser(t)
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, channelID)
	})

	// Enqueue notification with next_attempt_at in the future
	itemID := uuid.New().String()
	item := &notifications.QueueItem{
		ID:          itemID,
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event:       notifications.EventData{ID: eventID, Title: "Retry Test"},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}
	require.NoError(t, repo.EnqueueNotification(ctx, item))

	// Fetch item (marks as processing)
	items, err := repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, itemID, items[0].ID)
	assert.Equal(t, 0, items[0].Attempts)

	// Mark for retry with future time
	nextAttempt := time.Now().Add(1 * time.Hour)
	err = repo.MarkForRetry(ctx, itemID, errors.New("temporary error"), nextAttempt)
	require.NoError(t, err)

	// Item should not be available (next_attempt_at is in future)
	items, err = repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, items, "item with future next_attempt_at should not be fetched")

	// Verify attempts was incremented
	var attempts int
	err = testDB.QueryRow(ctx, `SELECT attempts FROM notification_queue WHERE id = $1`, itemID).Scan(&attempts)
	require.NoError(t, err)
	assert.Equal(t, 1, attempts, "attempts should be incremented after MarkForRetry")

	// Mark as sent to clean up
	err = repo.MarkAsSent(ctx, itemID)
	require.NoError(t, err)
}

func TestDispatch_PermanentFailure_NoRetry(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	mocks := NewMockSenderRegistry()

	// Permanent (non-retryable) error
	mocks.Email.FailNext(notifications.NewNonRetryableError(errors.New("permanent error")))

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	// Create test data
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "dispatch-perm-fail-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Dispatch Perm Fail Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	client.LoginAsUser(t)
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, channelID)
	})

	itemID := uuid.New().String()
	item := &notifications.QueueItem{
		ID:          itemID,
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event:       notifications.EventData{ID: eventID, Title: "Perm Fail Test"},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}
	require.NoError(t, repo.EnqueueNotification(ctx, item))

	// Start worker
	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)

	// Wait for processing (enough time for item to be processed and marked failed)
	time.Sleep(1 * time.Second)
	cancel()
	worker.Stop()

	// Should have been called only once (no retry for permanent error)
	assert.Equal(t, 1, mocks.Email.CallCount())
	assert.Equal(t, 0, mocks.Email.SentCount())

	// Should be marked as failed
	stats, err := repo.GetQueueStats(ctx)
	require.NoError(t, err)
	assert.True(t, stats.Failed >= 1, "at least one item should be failed")
}

func TestDispatch_MarkAsFailed_PermanentlyFails(t *testing.T) {
	// Test that MarkAsFailed properly marks items and removes them from pending queue
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)

	// Create test data
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "dispatch-fail-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Dispatch Fail Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	client.LoginAsUser(t)
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, channelID)
	})

	itemID := uuid.New().String()
	item := &notifications.QueueItem{
		ID:          itemID,
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event:       notifications.EventData{ID: eventID, Title: "Fail Test"},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}
	require.NoError(t, repo.EnqueueNotification(ctx, item))

	// Fetch item
	items, err := repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// Mark as failed (simulating permanent failure)
	err = repo.MarkAsFailed(ctx, itemID, errors.New("permanent failure"))
	require.NoError(t, err)

	// Item should not be available anymore
	items, err = repo.FetchPendingNotifications(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, items, "failed item should not be in pending queue")

	// Stats should show failed item
	stats, err := repo.GetQueueStats(ctx)
	require.NoError(t, err)
	assert.True(t, stats.Failed >= 1, "failed count should include our item")
}

// =============================================================================
// Channel State Tests
// =============================================================================

func TestDispatch_UnverifiedChannel_NotProcessed(t *testing.T) {
	// Test that notifications to unverified channels are not sent
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	// Create test data
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "unverified-channel-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Unverified Channel Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create UNVERIFIED channel (don't verify it)
	client.LoginAsUser(t)
	channelID := createEmailChannel(t, client)
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, channelID)
	})

	// Enqueue notification
	item := &notifications.QueueItem{
		ID:          uuid.New().String(),
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event:       notifications.EventData{ID: eventID, Title: "Unverified Test"},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}
	require.NoError(t, repo.EnqueueNotification(ctx, item))

	// Start worker
	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)

	// Wait for processing attempt
	time.Sleep(500 * time.Millisecond)
	cancel()
	worker.Stop()

	// Should NOT have sent (channel is unverified - worker should skip or fail)
	// The exact behavior depends on implementation, but no successful send should occur
	assert.Equal(t, 0, mocks.Email.SentCount(), "unverified channel should not receive notifications")
}

func TestDispatch_DisabledChannel_NotProcessed(t *testing.T) {
	// Test that notifications to disabled channels are not sent
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	// Create test data
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "disabled-channel-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	eventID := createTestIncident(t, client, "Disabled Channel Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create and verify channel, then disable it
	client.LoginAsUser(t)
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() {
		client.LoginAsUser(t)
		deleteChannel(t, client, channelID)
	})

	// Disable the channel
	resp, err := client.PATCH("/api/v1/me/channels/"+channelID, map[string]interface{}{
		"is_enabled": false,
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Enqueue notification
	item := &notifications.QueueItem{
		ID:          uuid.New().String(),
		EventID:     eventID,
		ChannelID:   channelID,
		MessageType: notifications.MessageTypeInitial,
		Payload: notifications.NotificationPayload{
			MessageType: notifications.MessageTypeInitial,
			Event:       notifications.EventData{ID: eventID, Title: "Disabled Test"},
			GeneratedAt: time.Now(),
		},
		MaxAttempts: 3,
	}
	require.NoError(t, repo.EnqueueNotification(ctx, item))

	// Start worker
	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)

	// Wait for processing attempt
	time.Sleep(500 * time.Millisecond)
	cancel()
	worker.Stop()

	// Should NOT have sent (channel is disabled)
	assert.Equal(t, 0, mocks.Email.SentCount(), "disabled channel should not receive notifications")
}

// =============================================================================
// Helper functions
// =============================================================================

func createTelegramChannel(t *testing.T, client *testutil.Client, chatID string) string {
	t.Helper()
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "telegram",
		"target": chatID,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.ID
}

func createMattermostChannel(t *testing.T, client *testutil.Client, webhookURL string) string {
	t.Helper()
	resp, err := client.POST("/api/v1/me/channels", map[string]interface{}{
		"type":   "mattermost",
		"target": webhookURL,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &result)
	return result.Data.ID
}

func verifyTelegramChannel(t *testing.T, _ *testutil.Client, channelID string) {
	t.Helper()
	// Directly update DB since dispatcher is not configured in tests
	_, err := testDB.Exec(context.Background(), `
		UPDATE notification_channels SET is_verified = true WHERE id = $1
	`, channelID)
	require.NoError(t, err)
}

func verifyMattermostChannel(t *testing.T, _ *testutil.Client, channelID string) {
	t.Helper()
	// Directly update DB since dispatcher is not configured in tests
	_, err := testDB.Exec(context.Background(), `
		UPDATE notification_channels SET is_verified = true WHERE id = $1
	`, channelID)
	require.NoError(t, err)
}
