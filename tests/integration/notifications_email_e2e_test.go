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
	"github.com/bissquit/incident-garden/internal/notifications/email"
	notificationspostgres "github.com/bissquit/incident-garden/internal/notifications/postgres"
	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// E2E Email Tests with Real SMTP (Mailpit)
//
// These tests verify that emails are actually sent through SMTP and received
// by Mailpit. They complement mock-based tests by checking:
// - SMTP protocol compliance
// - Email encoding (UTF-8, MIME headers)
// - Full integration: event -> queue -> worker -> SMTP -> mailbox
// =============================================================================

// -----------------------------------------------------------------------------
// Direct Sender Tests (no database, just SMTP)
// -----------------------------------------------------------------------------

func TestEmail_E2E_BasicSend(t *testing.T) {
	// Smoke test: verify SMTP connection works
	ctx := context.Background()
	require.NoError(t, mailpitClient.DeleteAllMessages())

	sender, err := email.NewSender(email.Config{
		Enabled:     true,
		SMTPHost:    mailpitContainer.SMTPHost,
		SMTPPort:    mailpitContainer.SMTPPort,
		FromAddress: "test@example.com",
	})
	require.NoError(t, err)

	err = sender.Send(ctx, notifications.Notification{
		To:      "user@test.com",
		Subject: "Test Alert",
		Body:    "Service API is experiencing issues.",
	})
	require.NoError(t, err)

	messages, err := mailpitClient.WaitForMessages(1, 5*time.Second)
	require.NoError(t, err, "failed to receive email in mailpit")
	require.Len(t, messages, 1, "expected 1 message")

	// Get full message details (list endpoint may not have full Bcc)
	fullMsg, err := mailpitClient.GetMessageByID(messages[0].ID)
	require.NoError(t, err)

	// Our sender uses BCC for recipients (not To header)
	recipients := fullMsg.AllRecipients()
	require.NotEmpty(t, recipients, "message should have recipients")
	assert.Equal(t, "user@test.com", recipients[0].Address)
	assert.Equal(t, "Test Alert", fullMsg.Subject)
	assert.Contains(t, fullMsg.Text, "experiencing issues")
}

func TestEmail_E2E_UnicodeContent(t *testing.T) {
	// Verify UTF-8 encoding works correctly through SMTP
	ctx := context.Background()
	require.NoError(t, mailpitClient.DeleteAllMessages())

	sender, err := email.NewSender(email.Config{
		Enabled:     true,
		SMTPHost:    mailpitContainer.SMTPHost,
		SMTPPort:    mailpitContainer.SMTPPort,
		FromAddress: "status@example.com",
	})
	require.NoError(t, err)

	// Russian text + emoji in subject and body
	subject := "🚨 Инцидент: Сервис API недоступен"
	body := "Сервис «Платежи» испытывает проблемы.\n\nСтатус: деградация 🔴\nВремя: сейчас"

	err = sender.Send(ctx, notifications.Notification{
		To:      "user@example.com",
		Subject: subject,
		Body:    body,
	})
	require.NoError(t, err)

	messages, err := mailpitClient.WaitForMessages(1, 5*time.Second)
	require.NoError(t, err)
	require.Len(t, messages, 1)

	// Verify subject preserved (including emoji and Cyrillic)
	assert.Contains(t, messages[0].Subject, "🚨")
	assert.Contains(t, messages[0].Subject, "Инцидент")

	// Verify body preserved (including Cyrillic and emoji)
	fullMsg, err := mailpitClient.GetMessageByID(messages[0].ID)
	require.NoError(t, err)
	assert.Contains(t, fullMsg.Text, "Платежи")
	assert.Contains(t, fullMsg.Text, "деградация")
	assert.Contains(t, fullMsg.Text, "🔴")
}

func TestEmail_E2E_MIMEHeaders(t *testing.T) {
	// Verify email headers are correctly formatted
	ctx := context.Background()
	require.NoError(t, mailpitClient.DeleteAllMessages())

	sender, err := email.NewSender(email.Config{
		Enabled:     true,
		SMTPHost:    mailpitContainer.SMTPHost,
		SMTPPort:    mailpitContainer.SMTPPort,
		FromAddress: "StatusPage <status@example.com>", // Name + email format
	})
	require.NoError(t, err)

	err = sender.Send(ctx, notifications.Notification{
		To:      "admin@company.com",
		Subject: "Incident #123: Database connection timeout",
		Body:    "The database is experiencing connection timeouts.",
	})
	require.NoError(t, err)

	messages, err := mailpitClient.WaitForMessages(1, 5*time.Second)
	require.NoError(t, err)
	require.Len(t, messages, 1)

	// Get full message details
	fullMsg, err := mailpitClient.GetMessageByID(messages[0].ID)
	require.NoError(t, err)

	// Verify From is parsed correctly (name + email)
	assert.Equal(t, "status@example.com", fullMsg.From.Address)
	assert.Equal(t, "StatusPage", fullMsg.From.Name)

	// Our sender uses BCC for recipients (not To header)
	recipients := fullMsg.AllRecipients()
	require.NotEmpty(t, recipients, "message should have recipients")
	assert.Equal(t, "admin@company.com", recipients[0].Address)

	// Verify Subject
	assert.Equal(t, "Incident #123: Database connection timeout", fullMsg.Subject)
}

// -----------------------------------------------------------------------------
// Full Integration Tests (database + worker + SMTP)
// -----------------------------------------------------------------------------

// e2eNotificationInfra holds the components for E2E notification tests.
type e2eNotificationInfra struct {
	notifier   *notifications.Notifier
	worker     *notifications.Worker
	workerCtx  context.Context
	cancelFunc context.CancelFunc
}

// setupE2ENotificationInfra creates a notifier and worker that send to Mailpit.
func setupE2ENotificationInfra(t *testing.T) *e2eNotificationInfra {
	t.Helper()

	repo := notificationspostgres.NewRepository(testDB)
	catalogRepo := catalogpostgres.NewRepository(testDB)
	catalogService := catalog.NewService(catalogRepo)

	// Create email sender pointing to Mailpit
	emailSender, err := email.NewSender(email.Config{
		Enabled:     true,
		SMTPHost:    mailpitContainer.SMTPHost,
		SMTPPort:    mailpitContainer.SMTPPort,
		FromAddress: "StatusPage <status@example.com>",
	})
	require.NoError(t, err)

	dispatcher := notifications.NewDispatcher(repo, emailSender)
	renderer, err := notifications.NewRenderer()
	require.NoError(t, err)

	notifier := notifications.NewNotifier(repo, renderer, dispatcher, catalogService, "http://status.example.com")

	worker := notifications.NewWorker(notifications.WorkerConfig{
		BatchSize:         10,
		PollInterval:      100 * time.Millisecond,
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		NumWorkers:        1,
	}, repo, dispatcher, renderer)

	workerCtx, cancel := context.WithCancel(context.Background())
	worker.Start(workerCtx)

	return &e2eNotificationInfra{
		notifier:   notifier,
		worker:     worker,
		workerCtx:  workerCtx,
		cancelFunc: cancel,
	}
}

// stop cleans up the infrastructure.
func (infra *e2eNotificationInfra) stop() {
	infra.cancelFunc()
	infra.worker.Stop()
}

func TestEmail_E2E_WorkerIntegration(t *testing.T) {
	// Full flow: create event -> notification queued -> worker sends -> email received
	require.NoError(t, mailpitClient.DeleteAllMessages())

	// Setup notification infrastructure with real Mailpit
	infra := setupE2ENotificationInfra(t)
	t.Cleanup(infra.stop)

	client := newTestClient(t)

	// Register a fresh user to avoid interference from other tests' subscriptions
	testEmail := testutil.RandomEmail()
	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    testEmail,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	client.LoginAs(t, testEmail, "password123")
	channelID := getUserDefaultChannel(t, client)

	// Create service as admin
	client.LoginAsAdmin(t)
	serviceID, serviceSlug := createTestService(t, client, "e2e-worker-test")
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		deleteService(t, client, serviceSlug)
	})

	// Subscribe user's channel to service
	client.LoginAs(t, testEmail, "password123")
	subscribeChannelToService(t, client, channelID, serviceID)

	// Create incident via API as admin (without notification - we call notifier manually)
	client.LoginAsAdmin(t)
	eventID := createTestIncident(t, client, "E2E Worker Integration Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Manually trigger notification (since app-level notifications are disabled)
	ctx := context.Background()
	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "E2E Worker Integration Test",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
		ServiceIDs:        []string{serviceID},
	}
	err = infra.notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	// Wait for email in Mailpit
	messages, err := mailpitClient.WaitForMessages(1, 10*time.Second)
	require.NoError(t, err, "failed to receive email in mailpit")
	require.Len(t, messages, 1)

	// Get full message details
	fullMsg, err := mailpitClient.GetMessageByID(messages[0].ID)
	require.NoError(t, err)

	// Verify email content
	// Our sender uses BCC for recipients (not To header)
	recipients := fullMsg.AllRecipients()
	require.NotEmpty(t, recipients, "message should have recipients")
	assert.Equal(t, testEmail, recipients[0].Address)
	assert.Contains(t, fullMsg.Subject, "Incident")

	// Verify body contains event details
	assert.Contains(t, fullMsg.Text, "E2E Worker Integration Test")
}

func TestEmail_E2E_EventLifecycle(t *testing.T) {
	// Test full event lifecycle: create -> update -> resolve
	// Should receive 3 emails total
	require.NoError(t, mailpitClient.DeleteAllMessages())

	// Setup notification infrastructure with real Mailpit
	infra := setupE2ENotificationInfra(t)
	t.Cleanup(infra.stop)

	client := newTestClient(t)

	// Register a fresh user to avoid interference from other tests' subscriptions
	testEmail := testutil.RandomEmail()
	resp, err := client.POST("/api/v1/auth/register", map[string]string{
		"email":    testEmail,
		"password": "password123",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	client.LoginAs(t, testEmail, "password123")
	channelID := getUserDefaultChannel(t, client)

	// Create service as admin
	client.LoginAsAdmin(t)
	serviceID, serviceSlug := createTestService(t, client, "e2e-lifecycle")
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		deleteService(t, client, serviceSlug)
	})

	// Subscribe user's channel to service
	client.LoginAs(t, testEmail, "password123")
	subscribeChannelToService(t, client, channelID, serviceID)

	// 1. Create incident as admin -> email #1
	client.LoginAsAdmin(t)
	eventID := createTestIncident(t, client, "Lifecycle E2E Test",
		[]AffectedService{{ServiceID: serviceID, Status: "degraded"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		deleteEvent(t, client, eventID)
	})

	// Manually trigger notification
	ctx := context.Background()
	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "Lifecycle E2E Test",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
		ServiceIDs:        []string{serviceID},
	}
	err = infra.notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	messages, err := mailpitClient.WaitForMessages(1, 10*time.Second)
	require.NoError(t, err)
	require.Len(t, messages, 1, "should receive initial notification")
	initialSubject := messages[0].Subject

	// 2. Add update -> email #2
	addEventUpdate(t, client, eventID, "identified", "Root cause identified: database failover")
	event.Status = domain.EventStatusIdentified
	update := &domain.EventUpdate{
		ID:                "update-1",
		EventID:           eventID,
		Status:            domain.EventStatusIdentified,
		Message:           "Root cause identified: database failover",
		NotifySubscribers: true,
	}
	err = infra.notifier.OnEventUpdated(ctx, event, update, nil)
	require.NoError(t, err)

	messages, err = mailpitClient.WaitForMessages(2, 10*time.Second)
	require.NoError(t, err)
	require.Len(t, messages, 2, "should receive update notification")

	// 3. Resolve -> email #3
	resolveEvent(t, client, eventID)
	event.Status = domain.EventStatusResolved
	err = infra.notifier.OnEventResolved(ctx, event, &notifications.NotifierResolution{Message: "Fixed"})
	require.NoError(t, err)

	messages, err = mailpitClient.WaitForMessages(3, 10*time.Second)
	require.NoError(t, err)
	require.Len(t, messages, 3, "should receive resolved notification")

	// Verify email progression (Mailpit returns newest first)
	subjects := make([]string, len(messages))
	for i, m := range messages {
		subjects[i] = m.Subject
	}

	// Check that we have different types of emails
	assert.NotEqual(t, initialSubject, subjects[0], "resolved email should have different subject")

	// Verify resolved email content (subject contains "Resolved:")
	resolvedMsg, err := mailpitClient.GetMessageByID(messages[0].ID)
	require.NoError(t, err)
	assert.Contains(t, resolvedMsg.Subject, "Resolved", "resolved email subject should mention resolution")
}

func TestEmail_E2E_MultipleRecipients(t *testing.T) {
	// Test that multiple subscribers each receive their own email
	require.NoError(t, mailpitClient.DeleteAllMessages())

	// Setup notification infrastructure with real Mailpit
	infra := setupE2ENotificationInfra(t)
	t.Cleanup(infra.stop)

	client := newTestClient(t)
	client.LoginAsAdmin(t)

	// Create service
	serviceID, serviceSlug := createTestService(t, client, "e2e-multi-recipients")
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		deleteService(t, client, serviceSlug)
	})

	// Register 3 new users and subscribe each to the service
	userEmails := make([]string, 3)
	for i := 0; i < 3; i++ {
		userEmail := testutil.RandomEmail()
		userEmails[i] = userEmail

		// Register new user (creates verified email channel automatically)
		resp, err := client.POST("/api/v1/auth/register", map[string]string{
			"email":    userEmail,
			"password": "password123",
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		resp.Body.Close()

		// Login as new user
		client.LoginAs(t, userEmail, "password123")

		// Get auto-created channel and subscribe
		channelID := getUserDefaultChannel(t, client)
		subscribeChannelToService(t, client, channelID, serviceID)
	}

	// Create incident as admin (without notification - we call notifier manually)
	client.LoginAsAdmin(t)
	eventID := createTestIncident(t, client, "Multi Recipients E2E Test",
		[]AffectedService{{ServiceID: serviceID, Status: "partial_outage"}}, nil)
	t.Cleanup(func() {
		client.LoginAsAdmin(t)
		resolveEvent(t, client, eventID)
		deleteEvent(t, client, eventID)
	})

	// Manually trigger notification (since app-level notifications are disabled)
	ctx := context.Background()
	now := time.Now()
	event := &domain.Event{
		ID:                eventID,
		Title:             "Multi Recipients E2E Test",
		Type:              domain.EventTypeIncident,
		Status:            domain.EventStatusInvestigating,
		NotifySubscribers: true,
		CreatedAt:         now,
		StartedAt:         &now,
		ServiceIDs:        []string{serviceID},
	}
	err := infra.notifier.OnEventCreated(ctx, event, []string{serviceID})
	require.NoError(t, err)

	// Wait for all 3 emails
	messages, err := mailpitClient.WaitForMessages(3, 15*time.Second)
	require.NoError(t, err, "failed to receive 3 emails in mailpit")
	require.Len(t, messages, 3, "all 3 subscribers should receive email")

	// Verify each recipient received exactly one email (need to fetch full message to get recipients)
	receivedEmails := make(map[string]int)
	for _, m := range messages {
		fullMsg, err := mailpitClient.GetMessageByID(m.ID)
		if err != nil {
			t.Logf("failed to get full message %s: %v", m.ID, err)
			continue
		}
		// Our sender uses BCC for recipients (not To header)
		for _, recipient := range fullMsg.AllRecipients() {
			receivedEmails[recipient.Address]++
		}
	}

	for _, userEmail := range userEmails {
		count, exists := receivedEmails[userEmail]
		assert.True(t, exists, "user %s should receive email", userEmail)
		assert.Equal(t, 1, count, "user %s should receive exactly 1 email", userEmail)
	}

	// Verify all emails have same subject (same incident)
	firstSubject := messages[0].Subject
	for _, m := range messages[1:] {
		assert.Equal(t, firstSubject, m.Subject, "all emails should have same subject")
	}
}
