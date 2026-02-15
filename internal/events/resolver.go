package events

import (
	"context"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/jackc/pgx/v5"
)

// GroupServiceResolver resolves group IDs to service IDs.
type GroupServiceResolver interface {
	GetGroupServices(ctx context.Context, groupID string) ([]string, error)
	ValidateGroupsExist(ctx context.Context, ids []string) (missingIDs []string, err error)
}

// CatalogServiceUpdater updates service status within a transaction.
type CatalogServiceUpdater interface {
	UpdateServiceStatusTx(ctx context.Context, tx pgx.Tx, serviceID string, status domain.ServiceStatus) error
	CreateStatusLogEntryTx(ctx context.Context, tx pgx.Tx, entry *domain.ServiceStatusLogEntry) error
	DeleteStatusLogByEventIDTx(ctx context.Context, tx pgx.Tx, eventID string) error
	GetServiceStatus(ctx context.Context, serviceID string) (domain.ServiceStatus, error)
	ValidateServicesExist(ctx context.Context, ids []string) (missingIDs []string, err error)
	GetServiceName(ctx context.Context, serviceID string) (string, error)
}

// EventNotifier sends notifications about events.
// This interface is implemented by notifications.Notifier.
type EventNotifier interface {
	OnEventCreated(ctx context.Context, event *domain.Event, serviceIDs []string) error
	OnEventUpdated(ctx context.Context, event *domain.Event, update *domain.EventUpdate, changes interface{}) error
	OnEventResolved(ctx context.Context, event *domain.Event, resolution interface{}) error
	OnEventCompleted(ctx context.Context, event *domain.Event, resolution interface{}) error
	OnEventCancelled(ctx context.Context, event *domain.Event) error
}
