package catalog

import (
	"context"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Repository defines the interface for catalog data operations.
type Repository interface {
	CreateGroup(ctx context.Context, group *domain.ServiceGroup) error
	GetGroupBySlug(ctx context.Context, slug string) (*domain.ServiceGroup, error)
	GetGroupByID(ctx context.Context, id string) (*domain.ServiceGroup, error)
	ListGroups(ctx context.Context, filter GroupFilter) ([]domain.ServiceGroup, error)
	UpdateGroup(ctx context.Context, group *domain.ServiceGroup) error
	DeleteGroup(ctx context.Context, id string) error

	CreateService(ctx context.Context, service *domain.Service) error
	GetServiceBySlug(ctx context.Context, slug string) (*domain.Service, error)
	GetServiceByID(ctx context.Context, id string) (*domain.Service, error)
	ListServices(ctx context.Context, filter ServiceFilter) ([]domain.Service, error)
	UpdateService(ctx context.Context, service *domain.Service) error
	DeleteService(ctx context.Context, id string) error

	SetServiceTags(ctx context.Context, serviceID string, tags []domain.ServiceTag) error
	GetServiceTags(ctx context.Context, serviceID string) ([]domain.ServiceTag, error)

	SetServiceGroups(ctx context.Context, serviceID string, groupIDs []string) error
	GetServiceGroups(ctx context.Context, serviceID string) ([]string, error)
	GetGroupServices(ctx context.Context, groupID string) ([]string, error)
	SetGroupServices(ctx context.Context, groupID string, serviceIDs []string) error

	// Soft delete operations
	ArchiveService(ctx context.Context, id string) error
	RestoreService(ctx context.Context, id string) error
	ArchiveGroup(ctx context.Context, id string) error
	RestoreGroup(ctx context.Context, id string) error

	// Active events check
	GetActiveEventCountForService(ctx context.Context, serviceID string) (int, error)
	GetActiveEventCountForGroup(ctx context.Context, groupID string) (int, error)

	// Group membership check
	GetNonArchivedServiceCountForGroup(ctx context.Context, groupID string) (int, error)

	// Effective status methods
	GetEffectiveStatus(ctx context.Context, serviceID string) (domain.ServiceStatus, bool, error)
	GetServiceBySlugWithEffectiveStatus(ctx context.Context, slug string) (*domain.ServiceWithEffectiveStatus, error)
	GetServiceByIDWithEffectiveStatus(ctx context.Context, id string) (*domain.ServiceWithEffectiveStatus, error)
	ListServicesWithEffectiveStatus(ctx context.Context, filter ServiceFilter) ([]domain.ServiceWithEffectiveStatus, error)

	// Transaction methods
	BeginTx(ctx context.Context) (pgx.Tx, error)
	UpdateServiceTx(ctx context.Context, tx pgx.Tx, service *domain.Service) error
	SetServiceGroupsTx(ctx context.Context, tx pgx.Tx, serviceID string, groupIDs []string) error
	UpdateServiceStatusTx(ctx context.Context, tx pgx.Tx, serviceID string, status domain.ServiceStatus) error

	// Status log methods
	CreateStatusLogEntry(ctx context.Context, entry *domain.ServiceStatusLogEntry) error
	CreateStatusLogEntryTx(ctx context.Context, tx pgx.Tx, entry *domain.ServiceStatusLogEntry) error
	ListStatusLog(ctx context.Context, serviceID string, limit, offset int) ([]domain.ServiceStatusLogEntry, error)
	CountStatusLog(ctx context.Context, serviceID string) (int, error)
}

// ServiceFilter represents filter criteria for listing services.
type ServiceFilter struct {
	GroupID         *string
	Status          *domain.ServiceStatus
	IncludeArchived bool
}

// GroupFilter represents filter criteria for listing groups.
type GroupFilter struct {
	IncludeArchived bool
}
