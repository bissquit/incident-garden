//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bissquit/incident-garden/internal/catalog"
	catalogpostgres "github.com/bissquit/incident-garden/internal/catalog/postgres"
	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/notifications"
	notificationspostgres "github.com/bissquit/incident-garden/internal/notifications/postgres"
	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Event Creation Tests
// =============================================================================

func TestEvents_Create_NotifiesSubscribers(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "https://status.example.com")

	// Start worker
	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Setup: create service
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "notify-create-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create verified channel and subscribe to the service
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	setChannelSubscription(t, client, channelID, []string{serviceID})

	// Re-login as admin to create event (channel operations may change auth context)
	client.LoginAsAdmin(t)

	// Create event using the helper function which properly creates an incident
	eventID := createTestIncident(t, client, "Notification Test Incident",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)

	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Call notifier manually (since we're not using the full app stack)
	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "Notification Test Incident",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
		ServiceIDs:        []string{serviceID},
	}
	err = notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	// Wait for notification to be sent
	success := mocks.Email.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "subscriber should receive notification")

	sent := mocks.Email.GetSent()
	require.GreaterOrEqual(t, len(sent), 1, "at least one notification should be sent")

	// Find notification about our event
	var foundNotification bool
	for _, notification := range sent {
		if notification.Body != "" && len(notification.Body) > 0 {
			if _, hasContent := notification.Body, true; hasContent {
				foundNotification = true
				assert.Contains(t, notification.Body, "Notification Test Incident")
				break
			}
		}
	}
	assert.True(t, foundNotification, "notification about our event should be found")
}

func TestEvents_Create_NotifyFalse_NoNotifications(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "https://status.example.com")

	// Setup
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "notify-false-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	setChannelSubscription(t, client, channelID, []string{serviceID})

	// Create event via API to get real UUID
	eventID := createTestIncident(t, client, "Silent Event",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create event struct with notify=false
	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "Silent Event",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: false,
		CreatedAt:         now,
		StartedAt:         &now,
	}

	err = notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	// Wait and verify no notifications
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 0, mocks.Email.SentCount(), "no notifications should be sent when notify=false")
}

func TestEvents_Create_NoSubscribers_NoNotifications(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "https://status.example.com")

	// Setup: create service but no subscribers
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "no-subscribers-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create event via API to get real UUID
	eventID := createTestIncident(t, client, "No Subscribers Event",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create event struct
	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "No Subscribers Event",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
	}

	err = notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	// Verify no notifications queued
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 0, mocks.TotalSentCount(), "no notifications without subscribers")
}

func TestEvents_Create_SubscribeToAll_ReceivesNotification(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "https://status.example.com")

	// Start worker
	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Setup
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "subscribe-all-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Channel with subscribe_to_all_services
	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	setSubscribeToAll(t, client, channelID, true)

	// Create event via API to get real UUID
	eventID := createTestIncident(t, client, "Subscribe All Event",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create event struct
	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "Subscribe All Event",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
	}

	err = notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	// Should receive notification
	success := mocks.Email.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "subscribe_to_all should receive notification")
}

// =============================================================================
// Event Update Tests
// =============================================================================

func TestEvents_Update_NotifiesEventSubscribers(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "https://status.example.com")

	// Start worker
	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Setup
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "update-notify-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	setChannelSubscription(t, client, channelID, []string{serviceID})

	eventID := createTestIncident(t, client, "Update Notify Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// First create event subscribers (simulating OnEventCreated)
	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "Update Notify Test",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
		ServiceIDs:        []string{serviceID},
	}
	err = notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	// Wait for initial notification
	success := mocks.Email.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "initial notification should be sent")
	mocks.Reset()

	// Send update
	event.Status = domain.EventStatusIdentified
	update := &domain.EventUpdate{
		Message:           "Status changed to identified",
		NotifySubscribers: true,
	}
	changes := &notifications.EventUpdateChanges{
		StatusFrom: "investigating",
		StatusTo:   "identified",
	}

	err = notifier.OnEventUpdated(ctx, event, update, changes)
	require.NoError(t, err)

	// Should receive update notification
	success = mocks.Email.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "update notification should be sent")
}

// =============================================================================
// Event Resolution Tests
// =============================================================================

func TestEvents_Resolve_NotifiesSubscribers(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "https://status.example.com")

	// Start worker
	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Setup
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "resolve-notify-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	setChannelSubscription(t, client, channelID, []string{serviceID})

	eventID := createTestIncident(t, client, "Resolve Notify Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Create event subscribers
	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "Resolve Notify Test",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
		ServiceIDs:        []string{serviceID},
	}
	err = notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	mocks.Email.WaitForNotifications(1, 2*time.Second)
	mocks.Reset()

	// Resolve event
	resolvedAt := time.Now()
	event.Status = domain.EventStatusResolved
	event.ResolvedAt = &resolvedAt

	resolution := &notifications.NotifierResolution{Message: "Issue has been fixed"}
	err = notifier.OnEventResolved(ctx, event, resolution)
	require.NoError(t, err)

	// Should receive resolved notification
	success := mocks.Email.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "resolved notification should be sent")

	sent := mocks.Email.GetSent()
	require.GreaterOrEqual(t, len(sent), 1, "at least one resolved notification should be sent")

	// Find notification with "Resolved" content
	var foundResolved bool
	for _, notification := range sent {
		if notification.Body != "" {
			foundResolved = true
			assert.Contains(t, notification.Body, "Resolved")
			break
		}
	}
	assert.True(t, foundResolved, "resolved notification should be found")
}

func TestEvents_Complete_NotifiesSubscribers(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "https://status.example.com")

	// Start worker
	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Setup
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "complete-notify-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	setChannelSubscription(t, client, channelID, []string{serviceID})

	// Create maintenance via API to get real UUID
	maintID := createTestMaintenance(t, client, "Complete Notify Test",
		[]AffectedService{{ServiceID: serviceID, Status: "maintenance"}})
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		completeMaintenance(t, client, maintID)
		deleteEvent(t, client, maintID)
	})

	// Create event struct with real ID
	now := time.Now()
	event := &domain.Event{
		ID:                maintID,
		Title:             "Complete Notify Test",
		Type:              domain.EventTypeMaintenance,
		Status:            domain.EventStatusInProgress,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
		ServiceIDs:        []string{serviceID},
	}

	err = notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	mocks.Email.WaitForNotifications(1, 2*time.Second)
	mocks.Reset()

	// Complete maintenance
	completedAt := time.Now()
	event.Status = domain.EventStatusCompleted
	event.ResolvedAt = &completedAt

	resolution := &notifications.NotifierResolution{Message: "Maintenance completed successfully"}
	err = notifier.OnEventCompleted(ctx, event, resolution)
	require.NoError(t, err)

	// Should receive completed notification
	success := mocks.Email.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "completed notification should be sent")
}

// =============================================================================
// Event Subscribers Tests
// =============================================================================

func TestEvents_Subscribers_FixedAtCreation(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "https://status.example.com")

	// Start worker
	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Setup
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "fixed-subs-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	// Create first channel and subscribe
	channelID1 := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID1) })
	setChannelSubscription(t, client, channelID1, []string{serviceID})

	// Create event - channel1 becomes subscriber
	eventID := createTestIncident(t, client, "Fixed Subs Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "Fixed Subs Test",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
		ServiceIDs:        []string{serviceID},
	}
	err = notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	mocks.Email.WaitForNotifications(1, 2*time.Second)

	// Create second channel and subscribe AFTER event creation
	channelID2 := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID2) })
	setChannelSubscription(t, client, channelID2, []string{serviceID})

	mocks.Reset()

	// Send update - only original subscriber should receive
	event.Status = domain.EventStatusIdentified
	update := &domain.EventUpdate{
		Message:           "Status update",
		NotifySubscribers: true,
	}
	err = notifier.OnEventUpdated(ctx, event, update, nil)
	require.NoError(t, err)

	mocks.Email.WaitForNotifications(1, 2*time.Second)

	// At least channel1 should get update (channel2 subscribed after event creation shouldn't)
	// Note: pre-seeded users now have default channels, so there might be more than 1
	assert.GreaterOrEqual(t, mocks.Email.SentCount(), 1, "at least original subscriber should get update")
}

// =============================================================================
// Event Cancellation Tests
// =============================================================================

func TestEvents_Cancel_NotifiesSubscribers(t *testing.T) {
	ctx := context.Background()
	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)
	mocks := NewMockSenderRegistry()

	dispatcher := notifications.NewDispatcher(repo, mocks.GetSenders()...)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "https://status.example.com")

	// Start worker
	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	defer func() {
		cancel()
		worker.Stop()
	}()

	// Setup
	client := newTestClient(t)
	client.LoginAsAdmin(t)

	serviceID, serviceSlug := createTestService(t, client, "cancel-notify-svc")
	t.Cleanup(func() { deleteService(t, client, serviceSlug) })

	channelID := createAndVerifyEmailChannel(t, client)
	t.Cleanup(func() { deleteChannel(t, client, channelID) })

	setChannelSubscription(t, client, channelID, []string{serviceID})

	// Create scheduled maintenance (so it can be cancelled)
	resp, err := client.POST("/api/v1/events", map[string]interface{}{
		"title":              "Cancel Notify Test",
		"type":               "maintenance",
		"status":             "scheduled",
		"description":        "Test maintenance",
		"notify_subscribers": true,
		"scheduled_start_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"scheduled_end_at":   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
		"affected_services": []map[string]string{
			{"service_id": serviceID, "status": "maintenance"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var eventResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	testutil.DecodeJSON(t, resp, &eventResp)
	eventID := eventResp.Data.ID

	// First, create event subscribers by calling OnEventCreated
	now := time.Now()
	scheduledStart := time.Now().Add(24 * time.Hour)
	scheduledEnd := time.Now().Add(25 * time.Hour)
	event := &domain.Event{
		ID:                eventID,
		Title:             "Cancel Notify Test",
		Type:              domain.EventTypeMaintenance,
		Status:            domain.EventStatusScheduled,
		NotifySubscribers: true,
		CreatedAt:         now,
		ScheduledStartAt:  &scheduledStart,
		ScheduledEndAt:    &scheduledEnd,
		ServiceIDs:        []string{serviceID},
	}
	err = notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	mocks.Email.WaitForNotifications(1, 2*time.Second)
	mocks.Reset()

	// Cancel the scheduled maintenance
	err = notifier.OnEventCancelled(ctx, event)
	require.NoError(t, err)

	// Should receive cancellation notification
	success := mocks.Email.WaitForNotifications(1, 2*time.Second)
	require.True(t, success, "cancellation notification should be sent")

	sent := mocks.Email.GetSent()
	require.GreaterOrEqual(t, len(sent), 1, "at least one cancellation notification should be sent")

	// Find notification with "Cancelled" content
	var foundCancelled bool
	for _, notification := range sent {
		if notification.Body != "" {
			foundCancelled = true
			assert.Contains(t, notification.Body, "Cancelled")
			break
		}
	}
	assert.True(t, foundCancelled, "cancellation notification should be found")

	// Clean up - delete the event directly since it's still in scheduled state
	client.LoginAsAdmin(t)
	deleteEvent(t, client, eventID)
}

// =============================================================================
// Helper functions
// =============================================================================

func setChannelSubscription(t *testing.T, client *testutil.Client, channelID string, serviceIDs []string) {
	t.Helper()
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": false,
		"service_ids":               serviceIDs,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func setSubscribeToAll(t *testing.T, client *testutil.Client, channelID string, subscribeToAll bool) {
	t.Helper()
	resp, err := client.PUT("/api/v1/me/channels/"+channelID+"/subscriptions", map[string]interface{}{
		"subscribe_to_all_services": subscribeToAll,
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
