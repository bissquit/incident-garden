package events

import (
	"context"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/jackc/pgx/v5"
)

// GroupServiceResolver resolves group IDs to service IDs.
type GroupServiceResolver interface {
	GetGroupServices(ctx context.Context, groupID string) ([]string, error)
}

// CatalogServiceUpdater updates service status within a transaction.
type CatalogServiceUpdater interface {
	UpdateServiceStatusTx(ctx context.Context, tx pgx.Tx, serviceID string, status domain.ServiceStatus) error
	CreateStatusLogEntryTx(ctx context.Context, tx pgx.Tx, entry *domain.ServiceStatusLogEntry) error
	DeleteStatusLogByEventIDTx(ctx context.Context, tx pgx.Tx, eventID string) error
	GetServiceStatus(ctx context.Context, serviceID string) (domain.ServiceStatus, error)
}
