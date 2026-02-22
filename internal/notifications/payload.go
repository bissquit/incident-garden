package notifications

import "time"

// MessageType defines the type of notification.
type MessageType string

// Message types.
const (
	MessageTypeInitial   MessageType = "initial"   // Event created
	MessageTypeUpdate    MessageType = "update"    // Event updated
	MessageTypeResolved  MessageType = "resolved"  // Incident resolved
	MessageTypeCompleted MessageType = "completed" // Maintenance completed
	MessageTypeCancelled MessageType = "cancelled" // Scheduled maintenance cancelled
)

// NotificationPayload contains data for rendering a notification.
type NotificationPayload struct {
	MessageType MessageType      `json:"message_type"`
	Event       EventData        `json:"event"`
	Changes     *EventChanges    `json:"changes,omitempty"`
	Resolution  *EventResolution `json:"resolution,omitempty"`
	EventURL    string           `json:"event_url,omitempty"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// EventData contains event information for notification.
type EventData struct {
	ID             string             `json:"id"`
	Title          string             `json:"title"`
	Type           string             `json:"type"`               // incident, maintenance
	Status         string             `json:"status"`             // investigating, identified, etc.
	Severity       string             `json:"severity,omitempty"` // minor, major, critical (empty for maintenance)
	Message        string             `json:"message"`
	Services       []ServiceInfo      `json:"services"`
	Groups         []GroupInfo        `json:"groups,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	StartedAt      *time.Time         `json:"started_at,omitempty"`
	ScheduledStart *time.Time         `json:"scheduled_start,omitempty"`
	ScheduledEnd   *time.Time         `json:"scheduled_end,omitempty"`
}

// ServiceInfo contains service data for notification context.
type ServiceInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GroupInfo contains group data for notification context.
type GroupInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// EventChanges describes what changed in an event update.
type EventChanges struct {
	StatusFrom      string                `json:"status_from,omitempty"`
	StatusTo        string                `json:"status_to,omitempty"`
	SeverityFrom    string                `json:"severity_from,omitempty"`
	SeverityTo      string                `json:"severity_to,omitempty"`
	ServicesAdded   []ServiceInfo         `json:"services_added,omitempty"`
	ServicesRemoved []ServiceInfo         `json:"services_removed,omitempty"`
	ServicesUpdated []ServiceStatusChange `json:"services_updated,omitempty"`
	Reason          string                `json:"reason,omitempty"`
}

// ServiceStatusChange describes a service status change.
type ServiceStatusChange struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	StatusFrom string `json:"status_from"`
	StatusTo   string `json:"status_to"`
}

// EventResolution contains resolution information.
type EventResolution struct {
	ResolvedAt time.Time     `json:"resolved_at"`
	Duration   time.Duration `json:"duration"`
	Message    string        `json:"message"`
}

// NewInitialPayload creates a payload for a new event notification.
func NewInitialPayload(event EventData, eventURL string) NotificationPayload {
	return NotificationPayload{
		MessageType: MessageTypeInitial,
		Event:       event,
		EventURL:    eventURL,
		GeneratedAt: time.Now(),
	}
}

// NewUpdatePayload creates a payload for an event update notification.
func NewUpdatePayload(event EventData, changes EventChanges, eventURL string) NotificationPayload {
	return NotificationPayload{
		MessageType: MessageTypeUpdate,
		Event:       event,
		Changes:     &changes,
		EventURL:    eventURL,
		GeneratedAt: time.Now(),
	}
}

// NewResolvedPayload creates a payload for an incident resolution notification.
func NewResolvedPayload(event EventData, changes EventChanges, resolution EventResolution, eventURL string) NotificationPayload {
	return NotificationPayload{
		MessageType: MessageTypeResolved,
		Event:       event,
		Changes:     &changes,
		Resolution:  &resolution,
		EventURL:    eventURL,
		GeneratedAt: time.Now(),
	}
}

// NewCompletedPayload creates a payload for a maintenance completion notification.
func NewCompletedPayload(event EventData, changes EventChanges, resolution EventResolution, eventURL string) NotificationPayload {
	return NotificationPayload{
		MessageType: MessageTypeCompleted,
		Event:       event,
		Changes:     &changes,
		Resolution:  &resolution,
		EventURL:    eventURL,
		GeneratedAt: time.Now(),
	}
}

// NewCancelledPayload creates a payload for a cancelled maintenance notification.
func NewCancelledPayload(event EventData) NotificationPayload {
	return NotificationPayload{
		MessageType: MessageTypeCancelled,
		Event:       event,
		GeneratedAt: time.Now(),
	}
}
