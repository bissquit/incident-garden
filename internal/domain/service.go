package domain

import "time"

type ServiceStatus string

const (
	ServiceStatusOperational   ServiceStatus = "operational"
	ServiceStatusDegraded      ServiceStatus = "degraded"
	ServiceStatusPartialOutage ServiceStatus = "partial_outage"
	ServiceStatusMajorOutage   ServiceStatus = "major_outage"
	ServiceStatusMaintenance   ServiceStatus = "maintenance"
)

type Service struct {
	ID          string
	Name        string
	Slug        string
	Description string
	Status      ServiceStatus
	GroupID     *string
	Order       int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ServiceGroup struct {
	ID        string
	Name      string
	Slug      string
	Order     int
	CreatedAt time.Time
	UpdatedAt time.Time
}
