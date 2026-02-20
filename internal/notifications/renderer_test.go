package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRenderer(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)
	require.NotNil(t, r)

	// Should have all templates loaded
	expectedCount := 3 * 5 // 3 channels * 5 message types
	assert.Len(t, r.templates, expectedCount)
}

func TestRenderer_RenderInitial_Incident(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	now := time.Now()
	payload := NotificationPayload{
		MessageType: MessageTypeInitial,
		Event: EventData{
			ID:       "evt-123",
			Title:    "Database connectivity issues",
			Type:     "incident",
			Status:   "investigating",
			Severity: "major",
			Message:  "We are investigating database connectivity issues.",
			Services: []ServiceInfo{
				{ID: "svc-1", Name: "API Gateway", Status: "degraded"},
				{ID: "svc-2", Name: "User Service", Status: "partial_outage"},
			},
			CreatedAt: now,
			StartedAt: &now,
		},
		EventURL:    "https://status.example.com/events/evt-123",
		GeneratedAt: now,
	}

	subject, body, err := r.Render(domain.ChannelTypeEmail, payload)
	require.NoError(t, err)

	assert.Equal(t, "[Incident] Database connectivity issues", subject)
	assert.Contains(t, body, "Incident: Database connectivity issues")
	assert.Contains(t, body, "API Gateway")
	assert.Contains(t, body, "degraded")
	assert.Contains(t, body, "Major")
	assert.Contains(t, body, "investigating database connectivity issues")
	assert.Contains(t, body, "https://status.example.com/events/evt-123")
}

func TestRenderer_RenderInitial_Maintenance(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	now := time.Now()
	scheduledStart := now.Add(24 * time.Hour)
	scheduledEnd := now.Add(26 * time.Hour)

	payload := NotificationPayload{
		MessageType: MessageTypeInitial,
		Event: EventData{
			ID:             "evt-456",
			Title:          "Database upgrade",
			Type:           "maintenance",
			Status:         "scheduled",
			Message:        "Scheduled database upgrade.",
			Services:       []ServiceInfo{{ID: "svc-1", Name: "Database", Status: "maintenance"}},
			CreatedAt:      now,
			ScheduledStart: &scheduledStart,
			ScheduledEnd:   &scheduledEnd,
		},
		EventURL:    "https://status.example.com/events/evt-456",
		GeneratedAt: now,
	}

	subject, body, err := r.Render(domain.ChannelTypeEmail, payload)
	require.NoError(t, err)

	assert.Equal(t, "[Scheduled Maintenance] Database upgrade", subject)
	assert.Contains(t, body, "Maintenance: Database upgrade")
	assert.Contains(t, body, "Scheduled:")
	assert.NotContains(t, body, "Severity:") // maintenance has no severity
}

func TestRenderer_RenderUpdate(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	now := time.Now()
	payload := NotificationPayload{
		MessageType: MessageTypeUpdate,
		Event: EventData{
			ID:      "evt-123",
			Title:   "Database connectivity issues",
			Type:    "incident",
			Status:  "identified",
			Message: "Root cause identified.",
		},
		Changes: &EventChanges{
			StatusFrom: "investigating",
			StatusTo:   "identified",
			ServicesAdded: []ServiceInfo{
				{ID: "svc-3", Name: "Auth Service", Status: "degraded"},
			},
		},
		EventURL:    "https://status.example.com/events/evt-123",
		GeneratedAt: now,
	}

	subject, body, err := r.Render(domain.ChannelTypeEmail, payload)
	require.NoError(t, err)

	assert.Equal(t, "[Update] Database connectivity issues", subject)
	assert.Contains(t, body, "Investigating -> Identified")
	assert.Contains(t, body, "Services added:")
	assert.Contains(t, body, "Auth Service")
	assert.Contains(t, body, "Root cause identified")
}

func TestRenderer_RenderResolved(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	now := time.Now()
	payload := NotificationPayload{
		MessageType: MessageTypeResolved,
		Event: EventData{
			ID:     "evt-123",
			Title:  "Database connectivity issues",
			Type:   "incident",
			Status: "resolved",
		},
		Changes: &EventChanges{
			StatusFrom: "monitoring",
			StatusTo:   "resolved",
		},
		Resolution: &EventResolution{
			ResolvedAt: now,
			Duration:   2*time.Hour + 30*time.Minute,
			Message:    "Issue has been fully resolved.",
		},
		EventURL:    "https://status.example.com/events/evt-123",
		GeneratedAt: now,
	}

	subject, body, err := r.Render(domain.ChannelTypeEmail, payload)
	require.NoError(t, err)

	assert.Equal(t, "[Resolved] Database connectivity issues", subject)
	assert.Contains(t, body, "Resolved:")
	assert.Contains(t, body, "2h 30m")
	assert.Contains(t, body, "fully resolved")
}

func TestRenderer_RenderCompleted(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	now := time.Now()
	payload := NotificationPayload{
		MessageType: MessageTypeCompleted,
		Event: EventData{
			ID:     "evt-456",
			Title:  "Database upgrade",
			Type:   "maintenance",
			Status: "completed",
		},
		Resolution: &EventResolution{
			ResolvedAt: now,
			Duration:   45 * time.Minute,
			Message:    "Maintenance completed successfully.",
		},
		EventURL:    "https://status.example.com/events/evt-456",
		GeneratedAt: now,
	}

	subject, body, err := r.Render(domain.ChannelTypeEmail, payload)
	require.NoError(t, err)

	assert.Equal(t, "[Completed] Database upgrade", subject)
	assert.Contains(t, body, "Completed:")
	assert.Contains(t, body, "45m")
	assert.Contains(t, body, "Maintenance completed successfully")
}

func TestRenderer_RenderCancelled(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	now := time.Now()
	scheduledStart := now.Add(24 * time.Hour)
	scheduledEnd := now.Add(26 * time.Hour)

	payload := NotificationPayload{
		MessageType: MessageTypeCancelled,
		Event: EventData{
			ID:             "evt-789",
			Title:          "Planned maintenance",
			Type:           "maintenance",
			Status:         "scheduled",
			ScheduledStart: &scheduledStart,
			ScheduledEnd:   &scheduledEnd,
		},
		GeneratedAt: now,
	}

	subject, body, err := r.Render(domain.ChannelTypeEmail, payload)
	require.NoError(t, err)

	assert.Equal(t, "[Cancelled] Planned maintenance", subject)
	assert.Contains(t, body, "Cancelled:")
	assert.Contains(t, body, "Originally scheduled:")
	assert.Contains(t, body, "has been cancelled")
}

func TestRenderer_TelegramFormat(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	now := time.Now()
	payload := NotificationPayload{
		MessageType: MessageTypeInitial,
		Event: EventData{
			ID:       "evt-123",
			Title:    "API Issues",
			Type:     "incident",
			Status:   "investigating",
			Severity: "critical",
			Services: []ServiceInfo{{ID: "svc-1", Name: "API", Status: "major_outage"}},
		},
		EventURL:    "https://status.example.com/events/evt-123",
		GeneratedAt: now,
	}

	_, body, err := r.Render(domain.ChannelTypeTelegram, payload)
	require.NoError(t, err)

	// Telegram should use HTML
	assert.Contains(t, body, "<b>Incident: API Issues</b>")
	assert.Contains(t, body, "<code>API</code>")
	assert.Contains(t, body, `<a href="https://status.example.com/events/evt-123">View details</a>`)
}

func TestRenderer_MattermostFormat(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	now := time.Now()
	payload := NotificationPayload{
		MessageType: MessageTypeInitial,
		Event: EventData{
			ID:       "evt-123",
			Title:    "API Issues",
			Type:     "incident",
			Status:   "investigating",
			Severity: "critical",
			Services: []ServiceInfo{{ID: "svc-1", Name: "API", Status: "major_outage"}},
		},
		EventURL:    "https://status.example.com/events/evt-123",
		GeneratedAt: now,
	}

	_, body, err := r.Render(domain.ChannelTypeMattermost, payload)
	require.NoError(t, err)

	// Mattermost should use ** for bold
	assert.Contains(t, body, "**Incident: API Issues**")
	assert.Contains(t, body, "[View details]")
}

func TestRenderer_UnknownTemplate(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	payload := NotificationPayload{
		MessageType: "unknown",
	}

	_, _, err = r.Render(domain.ChannelTypeEmail, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template not found")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{90 * time.Minute, "1h 30m"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 15*time.Minute, "2h 15m"},
		{24 * time.Hour, "24h"},
		{25*time.Hour + 30*time.Minute, "25h 30m"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := formatDuration(tc.duration)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatTime(t *testing.T) {
	tm := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	result := formatTime(&tm)
	assert.Equal(t, "Mar 15, 2024 14:30 UTC", result)

	// nil time
	assert.Equal(t, "", formatTime(nil))
}

func TestStatusEmoji(t *testing.T) {
	assert.Equal(t, "🔍", statusEmoji("investigating"))
	assert.Equal(t, "✅", statusEmoji("resolved"))
	assert.Equal(t, "📅", statusEmoji("scheduled"))
	assert.Equal(t, "📋", statusEmoji("unknown"))
}

func TestSeverityEmoji(t *testing.T) {
	assert.Equal(t, "🟡", severityEmoji("minor"))
	assert.Equal(t, "🟠", severityEmoji("major"))
	assert.Equal(t, "🔴", severityEmoji("critical"))
	assert.Equal(t, "⚪", severityEmoji("unknown"))
}

func TestTypeEmoji(t *testing.T) {
	assert.Equal(t, "🔴", typeEmoji("incident"))
	assert.Equal(t, "🔧", typeEmoji("maintenance"))
	assert.Equal(t, "📋", typeEmoji("unknown"))
}

func TestTitleCase(t *testing.T) {
	assert.Equal(t, "Investigating", titleCase("investigating"))
	assert.Equal(t, "In Progress", titleCase("in progress"))
	assert.Equal(t, "Major", titleCase("MAJOR")) // title case transforms to "Major"
}

func TestRenderer_EmptyOptionalFields(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	payload := NotificationPayload{
		MessageType: MessageTypeInitial,
		Event: EventData{
			ID:     "evt-123",
			Title:  "Test Event",
			Type:   "incident",
			Status: "investigating",
			// No services, no severity, no message
		},
		EventURL:    "https://status.example.com",
		GeneratedAt: time.Now(),
	}

	_, body, err := r.Render(domain.ChannelTypeEmail, payload)
	require.NoError(t, err)

	// Should not contain sections for empty fields
	assert.NotContains(t, body, "Affected services:")
	assert.NotContains(t, body, "Severity:")
	assert.Contains(t, body, "Investigating")
}

func TestRenderer_WithServiceStatusChanges(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	payload := NotificationPayload{
		MessageType: MessageTypeUpdate,
		Event: EventData{
			ID:     "evt-123",
			Title:  "Test Update",
			Type:   "incident",
			Status: "identified",
		},
		Changes: &EventChanges{
			ServicesUpdated: []ServiceStatusChange{
				{ID: "svc-1", Name: "API", StatusFrom: "degraded", StatusTo: "partial_outage"},
				{ID: "svc-2", Name: "Web", StatusFrom: "operational", StatusTo: "degraded"},
			},
		},
		EventURL:    "https://status.example.com",
		GeneratedAt: time.Now(),
	}

	_, body, err := r.Render(domain.ChannelTypeEmail, payload)
	require.NoError(t, err)

	assert.Contains(t, body, "Service status changes:")
	assert.Contains(t, body, "API: degraded -> partial_outage")
	assert.Contains(t, body, "Web: operational -> degraded")
}

func TestBuilderFunctions(t *testing.T) {
	event := EventData{
		ID:    "evt-1",
		Title: "Test",
		Type:  "incident",
	}

	t.Run("NewInitialPayload", func(t *testing.T) {
		p := NewInitialPayload(event, "http://example.com")
		assert.Equal(t, MessageTypeInitial, p.MessageType)
		assert.Equal(t, event, p.Event)
		assert.Equal(t, "http://example.com", p.EventURL)
		assert.Nil(t, p.Changes)
		assert.Nil(t, p.Resolution)
		assert.False(t, p.GeneratedAt.IsZero())
	})

	t.Run("NewUpdatePayload", func(t *testing.T) {
		changes := EventChanges{StatusFrom: "a", StatusTo: "b"}
		p := NewUpdatePayload(event, changes, "http://example.com")
		assert.Equal(t, MessageTypeUpdate, p.MessageType)
		assert.NotNil(t, p.Changes)
		assert.Equal(t, "a", p.Changes.StatusFrom)
	})

	t.Run("NewResolvedPayload", func(t *testing.T) {
		changes := EventChanges{StatusTo: "resolved"}
		resolution := EventResolution{Duration: time.Hour}
		p := NewResolvedPayload(event, changes, resolution, "http://example.com")
		assert.Equal(t, MessageTypeResolved, p.MessageType)
		assert.NotNil(t, p.Resolution)
		assert.Equal(t, time.Hour, p.Resolution.Duration)
	})

	t.Run("NewCompletedPayload", func(t *testing.T) {
		changes := EventChanges{StatusTo: "completed"}
		resolution := EventResolution{Duration: 30 * time.Minute}
		p := NewCompletedPayload(event, changes, resolution, "http://example.com")
		assert.Equal(t, MessageTypeCompleted, p.MessageType)
	})

	t.Run("NewCancelledPayload", func(t *testing.T) {
		p := NewCancelledPayload(event)
		assert.Equal(t, MessageTypeCancelled, p.MessageType)
		assert.Empty(t, p.EventURL) // cancelled doesn't need URL
	})
}

func TestRenderer_AllChannelTypes(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	payload := NotificationPayload{
		MessageType: MessageTypeInitial,
		Event: EventData{
			ID:     "evt-123",
			Title:  "Test",
			Type:   "incident",
			Status: "investigating",
		},
		EventURL:    "https://example.com",
		GeneratedAt: time.Now(),
	}

	channels := []domain.ChannelType{
		domain.ChannelTypeEmail,
		domain.ChannelTypeTelegram,
		domain.ChannelTypeMattermost,
	}

	for _, ch := range channels {
		t.Run(string(ch), func(t *testing.T) {
			subject, body, err := r.Render(ch, payload)
			require.NoError(t, err)
			assert.NotEmpty(t, subject)
			assert.NotEmpty(t, body)
			assert.True(t, strings.Contains(body, "Test"))
		})
	}
}
