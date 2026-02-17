package notifications

import (
	"context"
	"testing"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository implements Repository for testing.
type mockRepository struct {
	channels          []ChannelInfo
	eventSubscribers  map[string][]string
	findSubscribersErr error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		eventSubscribers: make(map[string][]string),
	}
}

func (m *mockRepository) CreateChannel(_ context.Context, _ *domain.NotificationChannel) error {
	return nil
}
func (m *mockRepository) GetChannelByID(_ context.Context, _ string) (*domain.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRepository) GetChannelByUserAndTarget(_ context.Context, _ string, _ domain.ChannelType, _ string) (*domain.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRepository) ListUserChannels(_ context.Context, _ string) ([]domain.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRepository) UpdateChannel(_ context.Context, _ *domain.NotificationChannel) error {
	return nil
}
func (m *mockRepository) DeleteChannel(_ context.Context, _ string) error {
	return nil
}
func (m *mockRepository) SetChannelSubscriptions(_ context.Context, _ string, _ bool, _ []string) error {
	return nil
}
func (m *mockRepository) GetChannelSubscriptions(_ context.Context, _ string) (bool, []string, error) {
	return false, nil, nil
}
func (m *mockRepository) GetUserChannelsWithSubscriptions(_ context.Context, _ string) ([]ChannelWithSubscriptions, error) {
	return nil, nil
}
func (m *mockRepository) CreateVerificationCode(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (m *mockRepository) GetVerificationCode(_ context.Context, _ string) (*VerificationCode, error) {
	return nil, nil
}
func (m *mockRepository) IncrementCodeAttempts(_ context.Context, _ string) error {
	return nil
}
func (m *mockRepository) DeleteVerificationCode(_ context.Context, _ string) error {
	return nil
}
func (m *mockRepository) DeleteExpiredCodes(_ context.Context) (int64, error) {
	return 0, nil
}

func (m *mockRepository) EnqueueNotification(_ context.Context, _ *QueueItem) error {
	return nil
}

func (m *mockRepository) EnqueueBatch(_ context.Context, _ []*QueueItem) error {
	return nil
}

func (m *mockRepository) FetchPendingNotifications(_ context.Context, _ int) ([]*QueueItem, error) {
	return nil, nil
}

func (m *mockRepository) MarkAsSent(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepository) MarkAsProcessing(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepository) MarkAsFailed(_ context.Context, _ string, _ error) error {
	return nil
}

func (m *mockRepository) MarkForRetry(_ context.Context, _ string, _ error, _ time.Time) error {
	return nil
}

func (m *mockRepository) GetFailedItems(_ context.Context, _ int) ([]*QueueItem, error) {
	return nil, nil
}

func (m *mockRepository) RetryFailedItem(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepository) RecoverStuckProcessing(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockRepository) DeleteOldSentItems(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockRepository) GetQueueStats(_ context.Context) (*QueueStats, error) {
	return &QueueStats{}, nil
}

func (m *mockRepository) CreateEventSubscribers(_ context.Context, eventID string, channelIDs []string) error {
	m.eventSubscribers[eventID] = channelIDs
	return nil
}

func (m *mockRepository) GetEventSubscribers(_ context.Context, eventID string) ([]string, error) {
	return m.eventSubscribers[eventID], nil
}

func (m *mockRepository) AddEventSubscribers(_ context.Context, eventID string, channelIDs []string) error {
	existing := m.eventSubscribers[eventID]
	m.eventSubscribers[eventID] = append(existing, channelIDs...)
	return nil
}

func (m *mockRepository) FindSubscribersForServices(_ context.Context, _ []string) ([]ChannelInfo, error) {
	if m.findSubscribersErr != nil {
		return nil, m.findSubscribersErr
	}
	return m.channels, nil
}

func (m *mockRepository) GetChannelsByIDs(_ context.Context, ids []string) ([]ChannelInfo, error) {
	result := make([]ChannelInfo, 0)
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	for _, ch := range m.channels {
		if idSet[ch.ID] {
			result = append(result, ch)
		}
	}
	return result, nil
}

// mockNameResolver implements ServiceNameResolver for testing.
type mockNameResolver struct {
	names map[string]string
}

func (m *mockNameResolver) GetServiceName(_ context.Context, serviceID string) (string, error) {
	if name, ok := m.names[serviceID]; ok {
		return name, nil
	}
	return serviceID, nil
}

func TestNotifier_OnEventCreated_NoSubscribers(t *testing.T) {
	repo := newMockRepository()
	notifier := NewNotifier(repo, nil, nil, nil, "https://status.example.com")

	event := &domain.Event{
		ID:                "event-1",
		Title:             "Test Incident",
		NotifySubscribers: true,
	}

	err := notifier.OnEventCreated(context.Background(), event, []string{"svc-1"})
	require.NoError(t, err)

	// No subscribers should be saved
	assert.Empty(t, repo.eventSubscribers)
}

func TestNotifier_OnEventCreated_NotifyDisabled(t *testing.T) {
	repo := newMockRepository()
	repo.channels = []ChannelInfo{{ID: "ch-1", Type: domain.ChannelTypeEmail, Target: "user@example.com"}}

	notifier := NewNotifier(repo, nil, nil, nil, "https://status.example.com")

	event := &domain.Event{
		ID:                "event-1",
		Title:             "Test Incident",
		NotifySubscribers: false, // Disabled
	}

	err := notifier.OnEventCreated(context.Background(), event, []string{"svc-1"})
	require.NoError(t, err)

	// No subscribers should be saved because notify is disabled
	assert.Empty(t, repo.eventSubscribers)
}

func TestNotifier_OnEventCreated_SavesSubscribers(t *testing.T) {
	repo := newMockRepository()
	repo.channels = []ChannelInfo{
		{ID: "ch-1", Type: domain.ChannelTypeEmail, Target: "user1@example.com"},
		{ID: "ch-2", Type: domain.ChannelTypeTelegram, Target: "123456"},
	}

	// No dispatcher - notifications won't be sent but subscribers will be saved
	notifier := NewNotifier(repo, nil, nil, nil, "https://status.example.com")

	event := &domain.Event{
		ID:                "event-1",
		Title:             "Test Incident",
		NotifySubscribers: true,
	}

	err := notifier.OnEventCreated(context.Background(), event, []string{"svc-1"})
	require.NoError(t, err)

	// Check subscribers were saved
	assert.Equal(t, []string{"ch-1", "ch-2"}, repo.eventSubscribers["event-1"])
}

func TestNotifier_OnEventCancelled_NotifyDisabled(t *testing.T) {
	repo := newMockRepository()
	notifier := NewNotifier(repo, nil, nil, nil, "https://status.example.com")

	event := &domain.Event{
		ID:                "event-1",
		Title:             "Scheduled Maintenance",
		NotifySubscribers: false,
	}

	err := notifier.OnEventCancelled(context.Background(), event)
	require.NoError(t, err)
}

func TestNotifier_BuildEventURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		eventID  string
		expected string
	}{
		{
			name:     "with base URL",
			baseURL:  "https://status.example.com",
			eventID:  "event-123",
			expected: "https://status.example.com/events/event-123",
		},
		{
			name:     "empty base URL",
			baseURL:  "",
			eventID:  "event-123",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := NewNotifier(nil, nil, nil, nil, tt.baseURL)
			result := notifier.buildEventURL(tt.eventID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNotifier_BuildEventData(t *testing.T) {
	nameResolver := &mockNameResolver{
		names: map[string]string{
			"svc-1": "API Gateway",
			"svc-2": "Database",
		},
	}

	notifier := NewNotifier(nil, nil, nil, nameResolver, "https://status.example.com")

	severity := domain.SeverityMajor
	startedAt := time.Now().Add(-1 * time.Hour)

	event := &domain.Event{
		ID:        "event-1",
		Title:     "Test Incident",
		Type:      domain.EventTypeIncident,
		Status:    domain.EventStatusInvestigating,
		Severity:  &severity,
		StartedAt: &startedAt,
		CreatedAt: time.Now(),
	}

	data := notifier.buildEventData(context.Background(), event, []string{"svc-1", "svc-2"})

	assert.Equal(t, "event-1", data.ID)
	assert.Equal(t, "Test Incident", data.Title)
	assert.Equal(t, "incident", data.Type)
	assert.Equal(t, "investigating", data.Status)
	assert.Equal(t, "major", data.Severity)
	assert.Len(t, data.Services, 2)
	assert.Equal(t, "API Gateway", data.Services[0].Name)
	assert.Equal(t, "Database", data.Services[1].Name)
}

func TestNotifier_BuildEventData_Maintenance(t *testing.T) {
	notifier := NewNotifier(nil, nil, nil, nil, "")

	scheduledStart := time.Now().Add(1 * time.Hour)
	scheduledEnd := time.Now().Add(3 * time.Hour)

	event := &domain.Event{
		ID:               "event-1",
		Title:            "Scheduled Maintenance",
		Type:             domain.EventTypeMaintenance,
		Status:           domain.EventStatusScheduled,
		ScheduledStartAt: &scheduledStart,
		ScheduledEndAt:   &scheduledEnd,
		CreatedAt:        time.Now(),
	}

	data := notifier.buildEventData(context.Background(), event, []string{})

	assert.Equal(t, "maintenance", data.Type)
	assert.Equal(t, "scheduled", data.Status)
	assert.Empty(t, data.Severity)
	assert.NotNil(t, data.ScheduledStart)
	assert.NotNil(t, data.ScheduledEnd)
}

func TestNotifier_ExtractResolution(t *testing.T) {
	notifier := NewNotifier(nil, nil, nil, nil, "")

	// Test with NotifierResolution
	res1 := &NotifierResolution{Message: "Fixed the issue"}
	extracted1 := notifier.extractResolution(res1)
	assert.NotNil(t, extracted1)
	assert.Equal(t, "Fixed the issue", extracted1.Message)

	// Test with anonymous struct with Message field
	res2 := struct{ Message string }{Message: "Another fix"}
	extracted2 := notifier.extractResolution(&res2)
	assert.NotNil(t, extracted2)
	assert.Equal(t, "Another fix", extracted2.Message)

	// Test with nil
	extracted3 := notifier.extractResolution(nil)
	assert.Nil(t, extracted3)
}

func TestNotifier_ExtractChanges(t *testing.T) {
	notifier := NewNotifier(nil, nil, nil, nil, "")

	// Test with EventUpdateChanges
	changes1 := &EventUpdateChanges{
		StatusFrom: "investigating",
		StatusTo:   "identified",
	}
	extracted1 := notifier.extractChanges(changes1)
	assert.NotNil(t, extracted1)
	assert.Equal(t, "investigating", extracted1.StatusFrom)
	assert.Equal(t, "identified", extracted1.StatusTo)

	// Test with anonymous struct
	changes2 := struct {
		StatusFrom string
		StatusTo   string
	}{
		StatusFrom: "identified",
		StatusTo:   "monitoring",
	}
	extracted2 := notifier.extractChanges(&changes2)
	assert.NotNil(t, extracted2)
	assert.Equal(t, "identified", extracted2.StatusFrom)
	assert.Equal(t, "monitoring", extracted2.StatusTo)

	// Test with nil
	extracted3 := notifier.extractChanges(nil)
	assert.Nil(t, extracted3)
}

func TestNotifier_ConvertChanges(t *testing.T) {
	nameResolver := &mockNameResolver{
		names: map[string]string{
			"svc-1": "API Gateway",
		},
	}

	notifier := NewNotifier(nil, nil, nil, nameResolver, "")

	changes := &EventUpdateChanges{
		StatusFrom: "investigating",
		StatusTo:   "identified",
		ServicesAdded: []domain.EventService{
			{ServiceID: "svc-1", Status: domain.ServiceStatusDegraded},
		},
		ServicesUpdated: []ServiceStatusUpdate{
			{ServiceID: "svc-2", ServiceName: "Database", StatusFrom: "operational", StatusTo: "degraded"},
		},
	}

	result := notifier.convertChanges(context.Background(), changes)

	assert.Equal(t, "investigating", result.StatusFrom)
	assert.Equal(t, "identified", result.StatusTo)
	assert.Len(t, result.ServicesAdded, 1)
	assert.Equal(t, "API Gateway", result.ServicesAdded[0].Name)
	assert.Len(t, result.ServicesUpdated, 1)
	assert.Equal(t, "Database", result.ServicesUpdated[0].Name)
}
