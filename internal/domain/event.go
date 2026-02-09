package domain

import "time"

// EventType represents the type of an event.
type EventType string

// Event types.
const (
	EventTypeIncident    EventType = "incident"
	EventTypeMaintenance EventType = "maintenance"
)

// EventStatus represents the current status of an event.
type EventStatus string

// Event statuses.
const (
	EventStatusInvestigating EventStatus = "investigating"
	EventStatusIdentified    EventStatus = "identified"
	EventStatusMonitoring    EventStatus = "monitoring"
	EventStatusResolved      EventStatus = "resolved"
	EventStatusScheduled     EventStatus = "scheduled"
	EventStatusInProgress    EventStatus = "in_progress"
	EventStatusCompleted     EventStatus = "completed"
)

// Severity represents the severity level of an event.
type Severity string

// Severity levels.
const (
	SeverityMinor    Severity = "minor"
	SeverityMajor    Severity = "major"
	SeverityCritical Severity = "critical"
)

// Event represents an incident or maintenance event.
type Event struct {
	ID                string       `json:"id"`
	Title             string       `json:"title"`
	Type              EventType    `json:"type"`
	Status            EventStatus  `json:"status"`
	Severity          *Severity    `json:"severity"`
	Description       string       `json:"description"`
	StartedAt         *time.Time   `json:"started_at"`
	ResolvedAt        *time.Time   `json:"resolved_at"`
	ScheduledStartAt  *time.Time   `json:"scheduled_start_at"`
	ScheduledEndAt    *time.Time   `json:"scheduled_end_at"`
	NotifySubscribers bool         `json:"notify_subscribers"`
	TemplateID        *string      `json:"template_id"`
	CreatedBy         string       `json:"created_by"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	ServiceIDs        []string     `json:"service_ids"`
	GroupIDs          []string     `json:"group_ids"`
}

// EventUpdate represents a status update for an event.
type EventUpdate struct {
	ID                string      `json:"id"`
	EventID           string      `json:"event_id"`
	Status            EventStatus `json:"status"`
	Message           string      `json:"message"`
	NotifySubscribers bool        `json:"notify_subscribers"`
	CreatedBy         string      `json:"created_by"`
	CreatedAt         time.Time   `json:"created_at"`
}

// IsValidForType checks if the status is valid for the given event type.
func (s EventStatus) IsValidForType(eventType EventType) bool {
	switch eventType {
	case EventTypeIncident:
		return s == EventStatusInvestigating ||
			s == EventStatusIdentified ||
			s == EventStatusMonitoring ||
			s == EventStatusResolved
	case EventTypeMaintenance:
		return s == EventStatusScheduled ||
			s == EventStatusInProgress ||
			s == EventStatusCompleted
	}
	return false
}

// IsValid checks if the event type is valid.
func (t EventType) IsValid() bool {
	return t == EventTypeIncident || t == EventTypeMaintenance
}

// IsValid checks if the severity is valid.
func (s Severity) IsValid() bool {
	return s == SeverityMinor || s == SeverityMajor || s == SeverityCritical
}

// IsResolved checks if the status represents a resolved/completed state.
// Note: 'scheduled' is NOT considered resolved, but it's also NOT active
// for the purpose of affecting service effective_status.
func (s EventStatus) IsResolved() bool {
	return s == EventStatusResolved || s == EventStatusCompleted
}

// IsActive checks if the event status affects service effective_status.
// Scheduled maintenance is NOT active until it transitions to in_progress.
// Use this instead of !IsResolved() when determining if an event affects effective_status.
func (s EventStatus) IsActive() bool {
	return s != EventStatusResolved &&
		s != EventStatusCompleted &&
		s != EventStatusScheduled
}

// SeverityToServiceStatus converts severity to service status for incidents.
// For maintenance events, returns ServiceStatusMaintenance.
func SeverityToServiceStatus(eventType EventType, severity *Severity) ServiceStatus {
	if eventType == EventTypeMaintenance {
		return ServiceStatusMaintenance
	}

	if severity == nil {
		return ServiceStatusDegraded
	}

	switch *severity {
	case SeverityCritical:
		return ServiceStatusMajorOutage
	case SeverityMajor:
		return ServiceStatusPartialOutage
	case SeverityMinor:
		return ServiceStatusDegraded
	default:
		return ServiceStatusDegraded
	}
}

// ChangeAction represents the type of change to event services.
type ChangeAction string

// Change actions.
const (
	ChangeActionAdded   ChangeAction = "added"
	ChangeActionRemoved ChangeAction = "removed"
)

// EventServiceChange represents a change to event's affected services.
type EventServiceChange struct {
	ID        string       `json:"id"`
	EventID   string       `json:"event_id"`
	BatchID   *string      `json:"batch_id,omitempty"`
	Action    ChangeAction `json:"action"`
	ServiceID *string      `json:"service_id,omitempty"`
	GroupID   *string      `json:"group_id,omitempty"`
	Reason    string       `json:"reason,omitempty"`
	CreatedBy string       `json:"created_by"`
	CreatedAt time.Time    `json:"created_at"`
}

// EventService represents a service associated with an event and its status in that context.
type EventService struct {
	EventID   string        `json:"event_id"`
	ServiceID string        `json:"service_id"`
	Status    ServiceStatus `json:"status"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// AffectedService represents a service to be associated with an event and its status.
type AffectedService struct {
	ServiceID string        `json:"service_id" validate:"required,uuid"`
	Status    ServiceStatus `json:"status" validate:"required,oneof=operational degraded partial_outage major_outage maintenance"`
}

// AffectedGroup represents a group whose services will be associated with an event.
// All services in the group will receive the specified status.
type AffectedGroup struct {
	GroupID string        `json:"group_id" validate:"required,uuid"`
	Status  ServiceStatus `json:"status" validate:"required,oneof=operational degraded partial_outage major_outage maintenance"`
}
