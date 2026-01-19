package domain

import "time"

// EventTemplate represents a reusable template for creating events.
type EventTemplate struct {
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Type          EventType `json:"type"`
	TitleTemplate string    `json:"title_template"`
	BodyTemplate  string    `json:"body_template"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TemplateData holds data for template rendering.
type TemplateData struct {
	ServiceName      string
	ServiceGroupName string
	StartedAt        *time.Time
	ResolvedAt       *time.Time
	ScheduledStart   *time.Time
	ScheduledEnd     *time.Time
}
