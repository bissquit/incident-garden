package events

import (
	"context"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Repository defines the interface for event storage.
type Repository interface {
	CreateEvent(ctx context.Context, event *domain.Event) error
	GetEvent(ctx context.Context, id string) (*domain.Event, error)
	ListEvents(ctx context.Context, filters EventFilters) ([]*domain.Event, error)
	UpdateEvent(ctx context.Context, event *domain.Event) error
	DeleteEvent(ctx context.Context, id string) error

	CreateEventUpdate(ctx context.Context, update *domain.EventUpdate) error
	ListEventUpdates(ctx context.Context, eventID string) ([]*domain.EventUpdate, error)

	CreateTemplate(ctx context.Context, template *domain.EventTemplate) error
	GetTemplate(ctx context.Context, id string) (*domain.EventTemplate, error)
	GetTemplateBySlug(ctx context.Context, slug string) (*domain.EventTemplate, error)
	ListTemplates(ctx context.Context) ([]*domain.EventTemplate, error)
	UpdateTemplate(ctx context.Context, template *domain.EventTemplate) error
	DeleteTemplate(ctx context.Context, id string) error

	AssociateServices(ctx context.Context, eventID string, serviceIDs []string) error
	GetEventServices(ctx context.Context, eventID string) ([]string, error)

	AssociateGroups(ctx context.Context, eventID string, groupIDs []string) error
	AddGroups(ctx context.Context, eventID string, groupIDs []string) error
	GetEventGroups(ctx context.Context, eventID string) ([]string, error)

	CreateServiceChange(ctx context.Context, change *domain.EventServiceChange) error
	ListServiceChanges(ctx context.Context, eventID string) ([]*domain.EventServiceChange, error)

	// Transaction support
	BeginTx(ctx context.Context) (pgx.Tx, error)
	CreateEventTx(ctx context.Context, tx pgx.Tx, event *domain.Event) error
	AssociateServicesTx(ctx context.Context, tx pgx.Tx, eventID string, serviceIDs []string) error
	AssociateGroupsTx(ctx context.Context, tx pgx.Tx, eventID string, groupIDs []string) error
	AddGroupsTx(ctx context.Context, tx pgx.Tx, eventID string, groupIDs []string) error
	CreateServiceChangeTx(ctx context.Context, tx pgx.Tx, change *domain.EventServiceChange) error
}

// EventFilters holds filter options for listing events.
type EventFilters struct {
	Type   *domain.EventType
	Status *domain.EventStatus
	Limit  int
	Offset int
}
