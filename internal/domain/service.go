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
	Order       int        `json:"order"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

// IsArchived returns true if the group is archived.
func (g *ServiceGroup) IsArchived() bool {
	return g.ArchivedAt != nil
}

// ServiceTag represents a key-value tag attached to a service.
type ServiceTag struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}
