package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/google/uuid"
)


// EventUpdateChanges describes changes in an event update.
type EventUpdateChanges struct {
	StatusFrom      string
	StatusTo        string
	SeverityFrom    string
	SeverityTo      string
	ServicesAdded   []domain.EventService
	ServicesRemoved []domain.EventService
	ServicesUpdated []ServiceStatusUpdate
	Reason          string
}

// ServiceStatusUpdate describes a service status change within an event.
type ServiceStatusUpdate struct {
	ServiceID   string
	ServiceName string
	StatusFrom  string
	StatusTo    string
}

// NotifierResolution contains resolution information for notifications.
type NotifierResolution struct {
	Message string
}

// ServiceNameResolver resolves service IDs to names.
type ServiceNameResolver interface {
	GetServiceName(ctx context.Context, serviceID string) (string, error)
}

// NotifierConfig contains notifier configuration.
type NotifierConfig struct {
	MaxAttempts int
}

// DefaultNotifierConfig returns default notifier configuration.
func DefaultNotifierConfig() NotifierConfig {
	return NotifierConfig{
		MaxAttempts: 3,
	}
}

// Notifier implements EventNotifier.
type Notifier struct {
	repo         Repository
	renderer     *Renderer
	dispatcher   *Dispatcher
	nameResolver ServiceNameResolver
	baseURL      string
	config       NotifierConfig
}

// NewNotifier creates a new Notifier.
func NewNotifier(repo Repository, renderer *Renderer, dispatcher *Dispatcher, nameResolver ServiceNameResolver, baseURL string) *Notifier {
	return &Notifier{
		repo:         repo,
		renderer:     renderer,
		dispatcher:   dispatcher,
		nameResolver: nameResolver,
		baseURL:      baseURL,
		config:       DefaultNotifierConfig(),
	}
}

// NewNotifierWithConfig creates a new Notifier with custom config.
func NewNotifierWithConfig(repo Repository, renderer *Renderer, dispatcher *Dispatcher, nameResolver ServiceNameResolver, baseURL string, config NotifierConfig) *Notifier {
	return &Notifier{
		repo:         repo,
		renderer:     renderer,
		dispatcher:   dispatcher,
		nameResolver: nameResolver,
		baseURL:      baseURL,
		config:       config,
	}
}

// OnEventCreated handles notifications for a newly created event.
func (n *Notifier) OnEventCreated(ctx context.Context, event *domain.Event, serviceIDs []string) error {
	if !event.NotifySubscribers {
		return nil
	}

	// Find subscribers for affected services
	channels, err := n.repo.FindSubscribersForServices(ctx, serviceIDs)
	if err != nil {
		return fmt.Errorf("find subscribers: %w", err)
	}

	if len(channels) == 0 {
		slog.Debug("no subscribers for event", "event_id", event.ID)
		return nil
	}

	// Extract channel IDs and save as event subscribers
	channelIDs := make([]string, len(channels))
	for i, ch := range channels {
		channelIDs[i] = ch.ID
	}

	if err := n.repo.CreateEventSubscribers(ctx, event.ID, channelIDs); err != nil {
		return fmt.Errorf("create event subscribers: %w", err)
	}

	// Build payload
	eventData := n.buildEventData(ctx, event, serviceIDs)
	payload := NewInitialPayload(eventData, n.buildEventURL(event.ID))

	// Enqueue notifications
	if err := n.enqueueForChannels(ctx, event.ID, channelIDs, payload); err != nil {
		slog.Error("failed to enqueue notifications", "event_id", event.ID, "error", err)
		// Don't return error - event is created, notifications can be retried
	}

	slog.Info("event notifications queued", "event_id", event.ID, "subscribers", len(channels))
	return nil
}

// OnEventUpdated handles notifications for an event update.
// changes should be *EventUpdateChanges or compatible struct.
func (n *Notifier) OnEventUpdated(ctx context.Context, event *domain.Event, update *domain.EventUpdate, changes interface{}) error {
	var typedChanges *EventUpdateChanges
	if changes != nil {
		// Try to extract fields from interface using reflection or type assertion
		if c, ok := changes.(*EventUpdateChanges); ok {
			typedChanges = c
		} else {
			// Try to convert from events package type via field extraction
			typedChanges = n.extractChanges(changes)
		}
	}
	return n.onEventUpdatedInternal(ctx, event, update, typedChanges)
}

// onEventUpdatedInternal is the actual implementation.
func (n *Notifier) onEventUpdatedInternal(ctx context.Context, event *domain.Event, update *domain.EventUpdate, changes *EventUpdateChanges) error {
	if update != nil && !update.NotifySubscribers {
		return nil
	}

	// If services were added, find new subscribers
	if changes != nil && len(changes.ServicesAdded) > 0 {
		addedServiceIDs := make([]string, len(changes.ServicesAdded))
		for i, s := range changes.ServicesAdded {
			addedServiceIDs[i] = s.ServiceID
		}

		newChannels, err := n.repo.FindSubscribersForServices(ctx, addedServiceIDs)
		if err != nil {
			slog.Error("failed to find new subscribers", "error", err)
		} else if len(newChannels) > 0 {
			newChannelIDs := make([]string, len(newChannels))
			for i, ch := range newChannels {
				newChannelIDs[i] = ch.ID
			}
			if err := n.repo.AddEventSubscribers(ctx, event.ID, newChannelIDs); err != nil {
				slog.Error("failed to add subscribers", "error", err)
			}
		}
	}

	// Build payload
	eventData := n.buildEventData(ctx, event, event.ServiceIDs)
	// For updates, use the update message instead of the original event description
	if update != nil && update.Message != "" {
		eventData.Message = update.Message
	}
	eventChanges := n.convertChanges(ctx, changes)
	payload := NewUpdatePayload(eventData, eventChanges, n.buildEventURL(event.ID))

	// Send to all event subscribers
	return n.sendToEventSubscribers(ctx, event.ID, payload)
}

// OnEventResolved handles notifications for an incident resolution.
// resolution should be *NotifierResolution or compatible struct with Message field.
func (n *Notifier) OnEventResolved(ctx context.Context, event *domain.Event, resolution interface{}) error {
	var res *NotifierResolution
	if resolution != nil {
		if r, ok := resolution.(*NotifierResolution); ok {
			res = r
		} else {
			res = n.extractResolution(resolution)
		}
	}
	return n.onEventResolvedInternal(ctx, event, res)
}

// onEventResolvedInternal is the actual implementation.
func (n *Notifier) onEventResolvedInternal(ctx context.Context, event *domain.Event, resolution *NotifierResolution) error {
	eventData := n.buildEventData(ctx, event, event.ServiceIDs)

	var resolvedAt time.Time
	if event.ResolvedAt != nil {
		resolvedAt = *event.ResolvedAt
	} else {
		resolvedAt = time.Now()
	}

	var duration time.Duration
	if event.StartedAt != nil {
		duration = resolvedAt.Sub(*event.StartedAt)
	}

	changes := EventChanges{
		StatusFrom: string(domain.EventStatusMonitoring), // Previous status before resolved
		StatusTo:   string(event.Status),
	}

	res := EventResolution{
		ResolvedAt: resolvedAt,
		Duration:   duration,
	}
	if resolution != nil {
		res.Message = resolution.Message
	}

	payload := NewResolvedPayload(eventData, changes, res, n.buildEventURL(event.ID))
	return n.sendToEventSubscribers(ctx, event.ID, payload)
}

// OnEventCompleted handles notifications for maintenance completion.
// resolution should be *NotifierResolution or compatible struct with Message field.
func (n *Notifier) OnEventCompleted(ctx context.Context, event *domain.Event, resolution interface{}) error {
	var res *NotifierResolution
	if resolution != nil {
		if r, ok := resolution.(*NotifierResolution); ok {
			res = r
		} else {
			res = n.extractResolution(resolution)
		}
	}
	return n.onEventCompletedInternal(ctx, event, res)
}

// onEventCompletedInternal is the actual implementation.
func (n *Notifier) onEventCompletedInternal(ctx context.Context, event *domain.Event, resolution *NotifierResolution) error {
	eventData := n.buildEventData(ctx, event, event.ServiceIDs)

	var completedAt time.Time
	if event.ResolvedAt != nil {
		completedAt = *event.ResolvedAt
	} else {
		completedAt = time.Now()
	}

	var duration time.Duration
	if event.StartedAt != nil {
		duration = completedAt.Sub(*event.StartedAt)
	}

	changes := EventChanges{
		StatusFrom: string(domain.EventStatusInProgress),
		StatusTo:   string(event.Status),
	}

	res := EventResolution{
		ResolvedAt: completedAt,
		Duration:   duration,
	}
	if resolution != nil {
		res.Message = resolution.Message
	}

	payload := NewCompletedPayload(eventData, changes, res, n.buildEventURL(event.ID))
	return n.sendToEventSubscribers(ctx, event.ID, payload)
}

// OnEventCancelled handles notifications for cancelled scheduled maintenance.
func (n *Notifier) OnEventCancelled(ctx context.Context, event *domain.Event) error {
	if !event.NotifySubscribers {
		return nil
	}

	eventData := n.buildEventData(ctx, event, event.ServiceIDs)
	payload := NewCancelledPayload(eventData)

	return n.sendToEventSubscribers(ctx, event.ID, payload)
}

// sendToEventSubscribers sends notifications to all subscribers of an event.
func (n *Notifier) sendToEventSubscribers(ctx context.Context, eventID string, payload NotificationPayload) error {
	channelIDs, err := n.repo.GetEventSubscribers(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get subscribers: %w", err)
	}

	if len(channelIDs) == 0 {
		return nil
	}

	return n.enqueueForChannels(ctx, eventID, channelIDs, payload)
}

// enqueueForChannels adds notifications to the queue for the given channels.
func (n *Notifier) enqueueForChannels(ctx context.Context, eventID string, channelIDs []string, payload NotificationPayload) error {
	if len(channelIDs) == 0 {
		return nil
	}

	items := make([]*QueueItem, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		items = append(items, &QueueItem{
			ID:          uuid.New().String(),
			EventID:     eventID,
			ChannelID:   channelID,
			MessageType: payload.MessageType,
			Payload:     payload,
			MaxAttempts: n.config.MaxAttempts,
		})
	}

	if err := n.repo.EnqueueBatch(ctx, items); err != nil {
		return fmt.Errorf("enqueue notifications: %w", err)
	}

	slog.Info("notifications queued", "event_id", eventID, "count", len(items))
	return nil
}

// buildEventData constructs EventData from domain.Event.
func (n *Notifier) buildEventData(ctx context.Context, event *domain.Event, serviceIDs []string) EventData {
	services := make([]ServiceInfo, 0, len(serviceIDs))
	for _, sid := range serviceIDs {
		name := sid // Fallback to ID if name resolution fails
		if n.nameResolver != nil {
			if resolved, err := n.nameResolver.GetServiceName(ctx, sid); err == nil {
				name = resolved
			}
		}
		services = append(services, ServiceInfo{
			ID:     sid,
			Name:   name,
			Status: "", // Status is event-specific, would need additional lookup
		})
	}

	data := EventData{
		ID:        event.ID,
		Title:     event.Title,
		Type:      string(event.Type),
		Status:    string(event.Status),
		Message:   event.Description,
		Services:  services,
		CreatedAt: event.CreatedAt,
		StartedAt: event.StartedAt,
	}

	if event.Severity != nil {
		data.Severity = string(*event.Severity)
	}

	if event.Type == domain.EventTypeMaintenance {
		data.ScheduledStart = event.ScheduledStartAt
		data.ScheduledEnd = event.ScheduledEndAt
	}

	return data
}

// convertChanges converts EventUpdateChanges to payload EventChanges.
func (n *Notifier) convertChanges(ctx context.Context, changes *EventUpdateChanges) EventChanges {
	if changes == nil {
		return EventChanges{}
	}

	result := EventChanges{
		StatusFrom:   changes.StatusFrom,
		StatusTo:     changes.StatusTo,
		SeverityFrom: changes.SeverityFrom,
		SeverityTo:   changes.SeverityTo,
		Reason:       changes.Reason,
	}

	// Convert added services
	for _, s := range changes.ServicesAdded {
		name := s.ServiceID
		if n.nameResolver != nil {
			if resolved, err := n.nameResolver.GetServiceName(ctx, s.ServiceID); err == nil {
				name = resolved
			}
		}
		result.ServicesAdded = append(result.ServicesAdded, ServiceInfo{
			ID:     s.ServiceID,
			Name:   name,
			Status: string(s.Status),
		})
	}

	// Convert removed services
	for _, s := range changes.ServicesRemoved {
		name := s.ServiceID
		if n.nameResolver != nil {
			if resolved, err := n.nameResolver.GetServiceName(ctx, s.ServiceID); err == nil {
				name = resolved
			}
		}
		result.ServicesRemoved = append(result.ServicesRemoved, ServiceInfo{
			ID:     s.ServiceID,
			Name:   name,
			Status: string(s.Status),
		})
	}

	// Convert status changes
	for _, s := range changes.ServicesUpdated {
		result.ServicesUpdated = append(result.ServicesUpdated, ServiceStatusChange{
			ID:         s.ServiceID,
			Name:       s.ServiceName,
			StatusFrom: s.StatusFrom,
			StatusTo:   s.StatusTo,
		})
	}

	return result
}

// buildEventURL constructs the URL for an event.
func (n *Notifier) buildEventURL(eventID string) string {
	if n.baseURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/events/%s", n.baseURL, eventID)
}

// extractChanges extracts EventUpdateChanges from an interface{}.
// This allows accepting structs from the events package without circular dependency.
func (n *Notifier) extractChanges(v interface{}) *EventUpdateChanges {
	if v == nil {
		return nil
	}

	// Use reflection to extract fields
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	changes := &EventUpdateChanges{}

	if f := val.FieldByName("StatusFrom"); f.IsValid() && f.Kind() == reflect.String {
		changes.StatusFrom = f.String()
	}
	if f := val.FieldByName("StatusTo"); f.IsValid() && f.Kind() == reflect.String {
		changes.StatusTo = f.String()
	}
	if f := val.FieldByName("SeverityFrom"); f.IsValid() && f.Kind() == reflect.String {
		changes.SeverityFrom = f.String()
	}
	if f := val.FieldByName("SeverityTo"); f.IsValid() && f.Kind() == reflect.String {
		changes.SeverityTo = f.String()
	}

	if f := val.FieldByName("Reason"); f.IsValid() && f.Kind() == reflect.String {
		changes.Reason = f.String()
	}

	// Extract ServicesAdded - array of structs with ServiceID and Status fields
	if f := val.FieldByName("ServicesAdded"); f.IsValid() && f.Kind() == reflect.Slice {
		for i := 0; i < f.Len(); i++ {
			elem := f.Index(i)
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct {
				svc := domain.EventService{}
				if sid := elem.FieldByName("ServiceID"); sid.IsValid() && sid.Kind() == reflect.String {
					svc.ServiceID = sid.String()
				}
				if status := elem.FieldByName("Status"); status.IsValid() {
					svc.Status = domain.ServiceStatus(status.String())
				}
				changes.ServicesAdded = append(changes.ServicesAdded, svc)
			}
		}
	}

	// Extract ServicesRemoved - array of structs with ServiceID and Status fields
	if f := val.FieldByName("ServicesRemoved"); f.IsValid() && f.Kind() == reflect.Slice {
		for i := 0; i < f.Len(); i++ {
			elem := f.Index(i)
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct {
				svc := domain.EventService{}
				if sid := elem.FieldByName("ServiceID"); sid.IsValid() && sid.Kind() == reflect.String {
					svc.ServiceID = sid.String()
				}
				if status := elem.FieldByName("Status"); status.IsValid() {
					svc.Status = domain.ServiceStatus(status.String())
				}
				changes.ServicesRemoved = append(changes.ServicesRemoved, svc)
			}
		}
	}

	// Extract ServicesUpdated - array of structs with ServiceID, ServiceName, StatusFrom, StatusTo
	if f := val.FieldByName("ServicesUpdated"); f.IsValid() && f.Kind() == reflect.Slice {
		for i := 0; i < f.Len(); i++ {
			elem := f.Index(i)
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct {
				su := ServiceStatusUpdate{}
				if sid := elem.FieldByName("ServiceID"); sid.IsValid() && sid.Kind() == reflect.String {
					su.ServiceID = sid.String()
				}
				if name := elem.FieldByName("ServiceName"); name.IsValid() && name.Kind() == reflect.String {
					su.ServiceName = name.String()
				}
				if sf := elem.FieldByName("StatusFrom"); sf.IsValid() && sf.Kind() == reflect.String {
					su.StatusFrom = sf.String()
				}
				if st := elem.FieldByName("StatusTo"); st.IsValid() && st.Kind() == reflect.String {
					su.StatusTo = st.String()
				}
				changes.ServicesUpdated = append(changes.ServicesUpdated, su)
			}
		}
	}

	return changes
}

// extractResolution extracts NotifierResolution from an interface{}.
func (n *Notifier) extractResolution(v interface{}) *NotifierResolution {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	res := &NotifierResolution{}
	if f := val.FieldByName("Message"); f.IsValid() && f.Kind() == reflect.String {
		res.Message = f.String()
	}

	return res
}
