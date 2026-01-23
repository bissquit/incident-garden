package events

import (
	"testing"

	"github.com/bissquit/incident-garden/internal/domain"
)

func TestEventStatus_IsValidForType(t *testing.T) {
	tests := []struct {
		name      string
		status    domain.EventStatus
		eventType domain.EventType
		want      bool
	}{
		{
			name:      "investigating is valid for incident",
			status:    domain.EventStatusInvestigating,
			eventType: domain.EventTypeIncident,
			want:      true,
		},
		{
			name:      "identified is valid for incident",
			status:    domain.EventStatusIdentified,
			eventType: domain.EventTypeIncident,
			want:      true,
		},
		{
			name:      "monitoring is valid for incident",
			status:    domain.EventStatusMonitoring,
			eventType: domain.EventTypeIncident,
			want:      true,
		},
		{
			name:      "resolved is valid for incident",
			status:    domain.EventStatusResolved,
			eventType: domain.EventTypeIncident,
			want:      true,
		},
		{
			name:      "scheduled is not valid for incident",
			status:    domain.EventStatusScheduled,
			eventType: domain.EventTypeIncident,
			want:      false,
		},
		{
			name:      "in_progress is not valid for incident",
			status:    domain.EventStatusInProgress,
			eventType: domain.EventTypeIncident,
			want:      false,
		},
		{
			name:      "completed is not valid for incident",
			status:    domain.EventStatusCompleted,
			eventType: domain.EventTypeIncident,
			want:      false,
		},
		{
			name:      "scheduled is valid for maintenance",
			status:    domain.EventStatusScheduled,
			eventType: domain.EventTypeMaintenance,
			want:      true,
		},
		{
			name:      "in_progress is valid for maintenance",
			status:    domain.EventStatusInProgress,
			eventType: domain.EventTypeMaintenance,
			want:      true,
		},
		{
			name:      "completed is valid for maintenance",
			status:    domain.EventStatusCompleted,
			eventType: domain.EventTypeMaintenance,
			want:      true,
		},
		{
			name:      "investigating is not valid for maintenance",
			status:    domain.EventStatusInvestigating,
			eventType: domain.EventTypeMaintenance,
			want:      false,
		},
		{
			name:      "identified is not valid for maintenance",
			status:    domain.EventStatusIdentified,
			eventType: domain.EventTypeMaintenance,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsValidForType(tt.eventType)
			if got != tt.want {
				t.Errorf("EventStatus.IsValidForType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEventStatus_IsResolved(t *testing.T) {
	tests := []struct {
		name   string
		status domain.EventStatus
		want   bool
	}{
		{
			name:   "resolved is resolved",
			status: domain.EventStatusResolved,
			want:   true,
		},
		{
			name:   "completed is resolved",
			status: domain.EventStatusCompleted,
			want:   true,
		},
		{
			name:   "investigating is not resolved",
			status: domain.EventStatusInvestigating,
			want:   false,
		},
		{
			name:   "scheduled is not resolved",
			status: domain.EventStatusScheduled,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsResolved()
			if got != tt.want {
				t.Errorf("EventStatus.IsResolved() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverity_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		severity domain.Severity
		want     bool
	}{
		{
			name:     "minor is valid",
			severity: domain.SeverityMinor,
			want:     true,
		},
		{
			name:     "major is valid",
			severity: domain.SeverityMajor,
			want:     true,
		},
		{
			name:     "critical is valid",
			severity: domain.SeverityCritical,
			want:     true,
		},
		{
			name:     "invalid severity",
			severity: domain.Severity("unknown"),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.severity.IsValid()
			if got != tt.want {
				t.Errorf("Severity.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
