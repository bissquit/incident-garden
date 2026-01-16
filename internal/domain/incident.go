package domain

import "time"

type IncidentStatus string

const (
	IncidentStatusInvestigating IncidentStatus = "investigating"
	IncidentStatusIdentified    IncidentStatus = "identified"
	IncidentStatusMonitoring    IncidentStatus = "monitoring"
	IncidentStatusResolved      IncidentStatus = "resolved"
)

type IncidentSeverity string

const (
	IncidentSeverityMinor    IncidentSeverity = "minor"
	IncidentSeverityMajor    IncidentSeverity = "major"
	IncidentSeverityCritical IncidentSeverity = "critical"
)

type Incident struct {
	ID         string
	Title      string
	Status     IncidentStatus
	Severity   IncidentSeverity
	ServiceIDs []string
	CreatedBy  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ResolvedAt *time.Time
}

type IncidentUpdate struct {
	ID         string
	IncidentID string
	Status     IncidentStatus
	Message    string
	CreatedBy  string
	CreatedAt  time.Time
}
