package domain

import "time"

// ServiceStatus represents the operational status of a service.
type ServiceStatus string

// Service statuses.
const (
	ServiceStatusOperational   ServiceStatus = "operational"
	ServiceStatusDegraded      ServiceStatus = "degraded"
	ServiceStatusPartialOutage ServiceStatus = "partial_outage"
	ServiceStatusMajorOutage   ServiceStatus = "major_outage"
	ServiceStatusMaintenance   ServiceStatus = "maintenance"
)

// IsValid checks if the service status is valid.
func (s ServiceStatus) IsValid() bool {
	switch s {
	case ServiceStatusOperational, ServiceStatusDegraded,
		ServiceStatusPartialOutage, ServiceStatusMajorOutage,
		ServiceStatusMaintenance:
		return true
	}
	return false
}

// Service represents a monitored service.
type Service struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Slug        string        `json:"slug"`
	Description string        `json:"description"`
	Status      ServiceStatus `json:"status"`
	GroupIDs    []string      `json:"group_ids"`
	Order       int           `json:"order"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	ArchivedAt  *time.Time    `json:"archived_at,omitempty"`
}

// IsArchived returns true if the service is archived.
func (s *Service) IsArchived() bool {
	return s.ArchivedAt != nil
}

// ServiceGroup represents a group of related services.
type ServiceGroup struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	ServiceIDs  []string   `json:"service_ids"`
	Order       int        `json:"order"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

// IsArchived returns true if the group is archived.
func (g *ServiceGroup) IsArchived() bool {
	return g.ArchivedAt != nil
}

// ServiceWithEffectiveStatus extends Service with computed effective status.
type ServiceWithEffectiveStatus struct {
	Service
	EffectiveStatus ServiceStatus `json:"effective_status"`
	HasActiveEvents bool          `json:"has_active_events"`
}

// ServiceTag represents a key-value tag attached to a service.
type ServiceTag struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

// StatusLogSourceType represents the source of a status change.
type StatusLogSourceType string

// Status log source types.
const (
	StatusLogSourceManual  StatusLogSourceType = "manual"
	StatusLogSourceEvent   StatusLogSourceType = "event"
	StatusLogSourceWebhook StatusLogSourceType = "webhook"
)

// ServiceStatusLogEntry represents a single status change in the audit log.
type ServiceStatusLogEntry struct {
	ID         string              `json:"id"`
	ServiceID  string              `json:"service_id"`
	OldStatus  *ServiceStatus      `json:"old_status,omitempty"`
	NewStatus  ServiceStatus       `json:"new_status"`
	SourceType StatusLogSourceType `json:"source_type"`
	EventID    *string             `json:"event_id,omitempty"`
	Reason     string              `json:"reason,omitempty"`
	CreatedBy  string              `json:"created_by"`
	CreatedAt  time.Time           `json:"created_at"`
}
