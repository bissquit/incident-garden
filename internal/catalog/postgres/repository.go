// Package postgres provides PostgreSQL implementation of the catalog repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bissquit/incident-garden/internal/catalog"
	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements the catalog.Repository interface using PostgreSQL.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateGroup creates a new service group in the database.
func (r *Repository) CreateGroup(ctx context.Context, group *domain.ServiceGroup) error {
	query := `
		INSERT INTO service_groups (name, slug, description, "order")
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		group.Name,
		group.Slug,
		group.Description,
		group.Order,
	).Scan(&group.ID, &group.CreatedAt, &group.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create service group: %w", err)
	}
	return nil
}

// GetGroupBySlug retrieves a service group by its slug.
func (r *Repository) GetGroupBySlug(ctx context.Context, slug string) (*domain.ServiceGroup, error) {
	query := `
		SELECT id, name, slug, description, "order", created_at, updated_at, archived_at
		FROM service_groups
		WHERE slug = $1
	`
	var group domain.ServiceGroup
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&group.ID,
		&group.Name,
		&group.Slug,
		&group.Description,
		&group.Order,
		&group.CreatedAt,
		&group.UpdatedAt,
		&group.ArchivedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, catalog.ErrGroupNotFound
		}
		return nil, fmt.Errorf("get service group by slug: %w", err)
	}

	// Load services for the group
	serviceIDs, err := r.GetGroupServices(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("get group services: %w", err)
	}
	group.ServiceIDs = serviceIDs

	return &group, nil
}

// GetGroupByID retrieves a service group by its ID.
func (r *Repository) GetGroupByID(ctx context.Context, id string) (*domain.ServiceGroup, error) {
	query := `
		SELECT id, name, slug, description, "order", created_at, updated_at, archived_at
		FROM service_groups
		WHERE id = $1
	`
	var group domain.ServiceGroup
	err := r.db.QueryRow(ctx, query, id).Scan(
		&group.ID,
		&group.Name,
		&group.Slug,
		&group.Description,
		&group.Order,
		&group.CreatedAt,
		&group.UpdatedAt,
		&group.ArchivedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, catalog.ErrGroupNotFound
		}
		return nil, fmt.Errorf("get service group by id: %w", err)
	}

	// Load services for the group
	serviceIDs, err := r.GetGroupServices(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get group services: %w", err)
	}
	group.ServiceIDs = serviceIDs

	return &group, nil
}

// ListGroups retrieves all service groups ordered by order and name.
func (r *Repository) ListGroups(ctx context.Context, filter catalog.GroupFilter) ([]domain.ServiceGroup, error) {
	query := `
		SELECT id, name, slug, description, "order", created_at, updated_at, archived_at
		FROM service_groups
	`

	if !filter.IncludeArchived {
		query += " WHERE archived_at IS NULL"
	}

	query += ` ORDER BY "order", name`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list service groups: %w", err)
	}
	defer rows.Close()

	groups := make([]domain.ServiceGroup, 0)
	for rows.Next() {
		var group domain.ServiceGroup
		err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.Slug,
			&group.Description,
			&group.Order,
			&group.CreatedAt,
			&group.UpdatedAt,
			&group.ArchivedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan service group: %w", err)
		}
		groups = append(groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service groups: %w", err)
	}

	// Load services for each group
	for i := range groups {
		serviceIDs, err := r.GetGroupServices(ctx, groups[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get group services: %w", err)
		}
		groups[i].ServiceIDs = serviceIDs
	}

	return groups, nil
}

// UpdateGroup updates an existing service group.
func (r *Repository) UpdateGroup(ctx context.Context, group *domain.ServiceGroup) error {
	query := `
		UPDATE service_groups
		SET name = $2, slug = $3, description = $4, "order" = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		group.ID,
		group.Name,
		group.Slug,
		group.Description,
		group.Order,
	).Scan(&group.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalog.ErrGroupNotFound
		}
		return fmt.Errorf("update service group: %w", err)
	}
	return nil
}

// DeleteGroup deletes a service group by its ID.
func (r *Repository) DeleteGroup(ctx context.Context, id string) error {
	query := `DELETE FROM service_groups WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete service group: %w", err)
	}

	if result.RowsAffected() == 0 {
		return catalog.ErrGroupNotFound
	}
	return nil
}

// CreateService creates a new service in the database.
func (r *Repository) CreateService(ctx context.Context, service *domain.Service) error {
	query := `
		INSERT INTO services (name, slug, description, status, "order")
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		service.Name,
		service.Slug,
		service.Description,
		service.Status,
		service.Order,
	).Scan(&service.ID, &service.CreatedAt, &service.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	return nil
}

// GetServiceBySlug retrieves a service by its slug.
func (r *Repository) GetServiceBySlug(ctx context.Context, slug string) (*domain.Service, error) {
	query := `
		SELECT id, name, slug, description, status, "order", created_at, updated_at, archived_at
		FROM services
		WHERE slug = $1
	`
	var service domain.Service
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&service.ID,
		&service.Name,
		&service.Slug,
		&service.Description,
		&service.Status,
		&service.Order,
		&service.CreatedAt,
		&service.UpdatedAt,
		&service.ArchivedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, catalog.ErrServiceNotFound
		}
		return nil, fmt.Errorf("get service by slug: %w", err)
	}

	// Load groups for the service
	groupIDs, err := r.GetServiceGroups(ctx, service.ID)
	if err != nil {
		return nil, fmt.Errorf("get service groups: %w", err)
	}
	service.GroupIDs = groupIDs

	return &service, nil
}

// GetServiceByID retrieves a service by its ID.
func (r *Repository) GetServiceByID(ctx context.Context, id string) (*domain.Service, error) {
	query := `
		SELECT id, name, slug, description, status, "order", created_at, updated_at, archived_at
		FROM services
		WHERE id = $1
	`
	var service domain.Service
	err := r.db.QueryRow(ctx, query, id).Scan(
		&service.ID,
		&service.Name,
		&service.Slug,
		&service.Description,
		&service.Status,
		&service.Order,
		&service.CreatedAt,
		&service.UpdatedAt,
		&service.ArchivedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, catalog.ErrServiceNotFound
		}
		return nil, fmt.Errorf("get service by id: %w", err)
	}

	// Load groups for the service
	groupIDs, err := r.GetServiceGroups(ctx, service.ID)
	if err != nil {
		return nil, fmt.Errorf("get service groups: %w", err)
	}
	service.GroupIDs = groupIDs

	return &service, nil
}

// ListServices retrieves all services matching the provided filter.
func (r *Repository) ListServices(ctx context.Context, filter catalog.ServiceFilter) ([]domain.Service, error) {
	var query string
	var args []interface{}
	argNum := 1

	if filter.GroupID != nil {
		// Filter by group using JOIN on service_group_members
		query = `
			SELECT DISTINCT s.id, s.name, s.slug, s.description, s.status, s."order", s.created_at, s.updated_at, s.archived_at
			FROM services s
			JOIN service_group_members sgm ON s.id = sgm.service_id
			WHERE sgm.group_id = $1
		`
		args = append(args, *filter.GroupID)
		argNum++

		if !filter.IncludeArchived {
			query += " AND s.archived_at IS NULL"
		}

		if filter.Status != nil {
			query += fmt.Sprintf(" AND s.status = $%d", argNum)
			args = append(args, *filter.Status)
		}
	} else {
		// No group filter
		query = `
			SELECT id, name, slug, description, status, "order", created_at, updated_at, archived_at
			FROM services
			WHERE 1=1
		`

		if !filter.IncludeArchived {
			query += " AND archived_at IS NULL"
		}

		if filter.Status != nil {
			query += fmt.Sprintf(" AND status = $%d", argNum)
			args = append(args, *filter.Status)
		}
	}

	query += ` ORDER BY "order", name`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	services := make([]domain.Service, 0)
	for rows.Next() {
		var service domain.Service
		err := rows.Scan(
			&service.ID,
			&service.Name,
			&service.Slug,
			&service.Description,
			&service.Status,
			&service.Order,
			&service.CreatedAt,
			&service.UpdatedAt,
			&service.ArchivedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, service)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}

	// Load groups for each service
	for i := range services {
		groupIDs, err := r.GetServiceGroups(ctx, services[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get service groups: %w", err)
		}
		services[i].GroupIDs = groupIDs
	}

	return services, nil
}

// UpdateService updates an existing service.
func (r *Repository) UpdateService(ctx context.Context, service *domain.Service) error {
	query := `
		UPDATE services
		SET name = $2, slug = $3, description = $4, status = $5, "order" = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		service.ID,
		service.Name,
		service.Slug,
		service.Description,
		service.Status,
		service.Order,
	).Scan(&service.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalog.ErrServiceNotFound
		}
		return fmt.Errorf("update service: %w", err)
	}
	return nil
}

// DeleteService deletes a service by its ID.
func (r *Repository) DeleteService(ctx context.Context, id string) error {
	query := `DELETE FROM services WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete service: %w", err)
	}

	if result.RowsAffected() == 0 {
		return catalog.ErrServiceNotFound
	}
	return nil
}

// SetServiceTags replaces all tags for a service with the provided tags.
func (r *Repository) SetServiceTags(ctx context.Context, serviceID string, tags []domain.ServiceTag) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("failed to rollback transaction", "error", err)
		}
	}()

	deleteQuery := `DELETE FROM service_tags WHERE service_id = $1`
	if _, err := tx.Exec(ctx, deleteQuery, serviceID); err != nil {
		return fmt.Errorf("delete old tags: %w", err)
	}

	if len(tags) > 0 {
		insertQuery := `
			INSERT INTO service_tags (service_id, key, value)
			VALUES ($1, $2, $3)
			RETURNING id
		`
		for i := range tags {
			err := tx.QueryRow(ctx, insertQuery, serviceID, tags[i].Key, tags[i].Value).Scan(&tags[i].ID)
			if err != nil {
				return fmt.Errorf("insert tag: %w", err)
			}
			tags[i].ServiceID = serviceID
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetServiceTags retrieves all tags for a service.
func (r *Repository) GetServiceTags(ctx context.Context, serviceID string) ([]domain.ServiceTag, error) {
	query := `
		SELECT id, service_id, key, value
		FROM service_tags
		WHERE service_id = $1
		ORDER BY key
	`
	rows, err := r.db.Query(ctx, query, serviceID)
	if err != nil {
		return nil, fmt.Errorf("get service tags: %w", err)
	}
	defer rows.Close()

	tags := make([]domain.ServiceTag, 0)
	for rows.Next() {
		var tag domain.ServiceTag
		err := rows.Scan(&tag.ID, &tag.ServiceID, &tag.Key, &tag.Value)
		if err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return tags, nil
}

// SetServiceGroups replaces all group memberships for a service.
func (r *Repository) SetServiceGroups(ctx context.Context, serviceID string, groupIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("failed to rollback transaction", "error", err)
		}
	}()

	// Delete old group memberships
	_, err = tx.Exec(ctx, `DELETE FROM service_group_members WHERE service_id = $1`, serviceID)
	if err != nil {
		return fmt.Errorf("delete old group memberships: %w", err)
	}

	// Insert new group memberships
	for _, groupID := range groupIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO service_group_members (service_id, group_id) VALUES ($1, $2)`,
			serviceID, groupID)
		if err != nil {
			return fmt.Errorf("insert group membership: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// GetServiceGroups returns all group IDs for a service.
func (r *Repository) GetServiceGroups(ctx context.Context, serviceID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT group_id FROM service_group_members WHERE service_id = $1 ORDER BY group_id`,
		serviceID)
	if err != nil {
		return nil, fmt.Errorf("get service groups: %w", err)
	}
	defer rows.Close()

	groupIDs := make([]string, 0)
	for rows.Next() {
		var groupID string
		if err := rows.Scan(&groupID); err != nil {
			return nil, fmt.Errorf("scan group id: %w", err)
		}
		groupIDs = append(groupIDs, groupID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group ids: %w", err)
	}

	return groupIDs, nil
}

// GetGroupServices returns all service IDs in a group.
func (r *Repository) GetGroupServices(ctx context.Context, groupID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT service_id FROM service_group_members WHERE group_id = $1 ORDER BY service_id`,
		groupID)
	if err != nil {
		return nil, fmt.Errorf("get group services: %w", err)
	}
	defer rows.Close()

	serviceIDs := make([]string, 0)
	for rows.Next() {
		var serviceID string
		if err := rows.Scan(&serviceID); err != nil {
			return nil, fmt.Errorf("scan service id: %w", err)
		}
		serviceIDs = append(serviceIDs, serviceID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service ids: %w", err)
	}

	return serviceIDs, nil
}

// SetGroupServices replaces all service memberships for a group.
func (r *Repository) SetGroupServices(ctx context.Context, groupID string, serviceIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("failed to rollback transaction", "error", err)
		}
	}()

	// Delete old service memberships
	_, err = tx.Exec(ctx, `DELETE FROM service_group_members WHERE group_id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("delete old service memberships: %w", err)
	}

	// Insert new service memberships
	for _, serviceID := range serviceIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO service_group_members (service_id, group_id) VALUES ($1, $2)`,
			serviceID, groupID)
		if err != nil {
			return fmt.Errorf("insert service membership: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// ArchiveService soft-deletes a service by setting archived_at.
func (r *Repository) ArchiveService(ctx context.Context, id string) error {
	query := `UPDATE services SET archived_at = NOW(), updated_at = NOW() WHERE id = $1 AND archived_at IS NULL`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("archive service: %w", err)
	}
	if result.RowsAffected() == 0 {
		// Check if not found or already archived
		var archivedAt *string
		err := r.db.QueryRow(ctx, `SELECT archived_at::text FROM services WHERE id = $1`, id).Scan(&archivedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return catalog.ErrServiceNotFound
			}
			return fmt.Errorf("check service exists: %w", err)
		}
		if archivedAt != nil {
			return catalog.ErrAlreadyArchived
		}
		return catalog.ErrServiceNotFound
	}
	return nil
}

// RestoreService restores an archived service by clearing archived_at.
func (r *Repository) RestoreService(ctx context.Context, id string) error {
	query := `UPDATE services SET archived_at = NULL, updated_at = NOW() WHERE id = $1 AND archived_at IS NOT NULL`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("restore service: %w", err)
	}
	if result.RowsAffected() == 0 {
		// Check if not found or not archived
		var archivedAt *string
		err := r.db.QueryRow(ctx, `SELECT archived_at::text FROM services WHERE id = $1`, id).Scan(&archivedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return catalog.ErrServiceNotFound
			}
			return fmt.Errorf("check service exists: %w", err)
		}
		if archivedAt == nil {
			return catalog.ErrNotArchived
		}
		return catalog.ErrServiceNotFound
	}
	return nil
}

// ArchiveGroup soft-deletes a group by setting archived_at.
func (r *Repository) ArchiveGroup(ctx context.Context, id string) error {
	query := `UPDATE service_groups SET archived_at = NOW(), updated_at = NOW() WHERE id = $1 AND archived_at IS NULL`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("archive group: %w", err)
	}
	if result.RowsAffected() == 0 {
		// Check if not found or already archived
		var archivedAt *string
		err := r.db.QueryRow(ctx, `SELECT archived_at::text FROM service_groups WHERE id = $1`, id).Scan(&archivedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return catalog.ErrGroupNotFound
			}
			return fmt.Errorf("check group exists: %w", err)
		}
		if archivedAt != nil {
			return catalog.ErrAlreadyArchived
		}
		return catalog.ErrGroupNotFound
	}
	return nil
}

// RestoreGroup restores an archived group by clearing archived_at.
func (r *Repository) RestoreGroup(ctx context.Context, id string) error {
	query := `UPDATE service_groups SET archived_at = NULL, updated_at = NOW() WHERE id = $1 AND archived_at IS NOT NULL`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("restore group: %w", err)
	}
	if result.RowsAffected() == 0 {
		// Check if not found or not archived
		var archivedAt *string
		err := r.db.QueryRow(ctx, `SELECT archived_at::text FROM service_groups WHERE id = $1`, id).Scan(&archivedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return catalog.ErrGroupNotFound
			}
			return fmt.Errorf("check group exists: %w", err)
		}
		if archivedAt == nil {
			return catalog.ErrNotArchived
		}
		return catalog.ErrGroupNotFound
	}
	return nil
}

// GetActiveEventCountForService returns count of active events for a service.
// Active events are those that affect service effective_status: NOT resolved, completed, or scheduled.
// Scheduled maintenance is not considered active until it transitions to in_progress.
func (r *Repository) GetActiveEventCountForService(ctx context.Context, serviceID string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT e.id)
		FROM events e
		JOIN event_services es ON e.id = es.event_id
		WHERE es.service_id = $1
		  AND e.status NOT IN ('resolved', 'completed', 'scheduled')
	`
	var count int
	err := r.db.QueryRow(ctx, query, serviceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get active event count for service: %w", err)
	}
	return count, nil
}

// GetActiveEventCountForGroup returns count of active events for any service in the group.
// Active events are those that affect service effective_status: NOT resolved, completed, or scheduled.
// Scheduled maintenance is not considered active until it transitions to in_progress.
func (r *Repository) GetActiveEventCountForGroup(ctx context.Context, groupID string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT e.id)
		FROM events e
		JOIN event_services es ON e.id = es.event_id
		JOIN service_group_members sgm ON es.service_id = sgm.service_id
		WHERE sgm.group_id = $1
		  AND e.status NOT IN ('resolved', 'completed', 'scheduled')
	`
	var count int
	err := r.db.QueryRow(ctx, query, groupID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get active event count for group: %w", err)
	}
	return count, nil
}

// GetNonArchivedServiceCountForGroup returns count of non-archived services in the group.
func (r *Repository) GetNonArchivedServiceCountForGroup(ctx context.Context, groupID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM service_group_members sgm
		JOIN services s ON sgm.service_id = s.id
		WHERE sgm.group_id = $1
		  AND s.archived_at IS NULL
	`
	var count int
	err := r.db.QueryRow(ctx, query, groupID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get non-archived service count for group: %w", err)
	}
	return count, nil
}

// GetEffectiveStatus returns the effective status for a service (worst-case from active events or stored status).
func (r *Repository) GetEffectiveStatus(ctx context.Context, serviceID string) (domain.ServiceStatus, bool, error) {
	query := `
		SELECT effective_status, has_active_events
		FROM v_service_effective_status
		WHERE id = $1
	`
	var effectiveStatus domain.ServiceStatus
	var hasActiveEvents bool
	err := r.db.QueryRow(ctx, query, serviceID).Scan(&effectiveStatus, &hasActiveEvents)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, catalog.ErrServiceNotFound
		}
		return "", false, fmt.Errorf("get effective status: %w", err)
	}
	return effectiveStatus, hasActiveEvents, nil
}

// GetServiceBySlugWithEffectiveStatus returns a service with its effective status.
func (r *Repository) GetServiceBySlugWithEffectiveStatus(ctx context.Context, slug string) (*domain.ServiceWithEffectiveStatus, error) {
	service, err := r.GetServiceBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	effectiveStatus, hasActiveEvents, err := r.GetEffectiveStatus(ctx, service.ID)
	if err != nil {
		return nil, err
	}

	return &domain.ServiceWithEffectiveStatus{
		Service:         *service,
		EffectiveStatus: effectiveStatus,
		HasActiveEvents: hasActiveEvents,
	}, nil
}

// GetServiceByIDWithEffectiveStatus returns a service with its effective status.
func (r *Repository) GetServiceByIDWithEffectiveStatus(ctx context.Context, id string) (*domain.ServiceWithEffectiveStatus, error) {
	service, err := r.GetServiceByID(ctx, id)
	if err != nil {
		return nil, err
	}

	effectiveStatus, hasActiveEvents, err := r.GetEffectiveStatus(ctx, id)
	if err != nil {
		return nil, err
	}

	return &domain.ServiceWithEffectiveStatus{
		Service:         *service,
		EffectiveStatus: effectiveStatus,
		HasActiveEvents: hasActiveEvents,
	}, nil
}

// ListServicesWithEffectiveStatus returns services with their effective statuses.
func (r *Repository) ListServicesWithEffectiveStatus(ctx context.Context, filter catalog.ServiceFilter) ([]domain.ServiceWithEffectiveStatus, error) {
	query := `
		SELECT
			s.id, s.name, s.slug, s.description, s.status, s."order",
			s.created_at, s.updated_at, s.archived_at,
			v.effective_status, v.has_active_events
		FROM services s
		JOIN v_service_effective_status v ON s.id = v.id
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if filter.GroupID != nil {
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM service_group_members sgm WHERE sgm.service_id = s.id AND sgm.group_id = $%d)", argNum)
		args = append(args, *filter.GroupID)
		argNum++
	}

	if filter.Status != nil {
		// Filter by effective_status, not stored status
		query += fmt.Sprintf(" AND v.effective_status = $%d", argNum)
		args = append(args, *filter.Status)
	}

	if !filter.IncludeArchived {
		query += " AND s.archived_at IS NULL"
	}

	query += ` ORDER BY s."order", s.name`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list services with effective status: %w", err)
	}
	defer rows.Close()

	result := make([]domain.ServiceWithEffectiveStatus, 0)
	for rows.Next() {
		var svc domain.ServiceWithEffectiveStatus
		err := rows.Scan(
			&svc.ID, &svc.Name, &svc.Slug, &svc.Description, &svc.Status, &svc.Order,
			&svc.CreatedAt, &svc.UpdatedAt, &svc.ArchivedAt,
			&svc.EffectiveStatus, &svc.HasActiveEvents,
		)
		if err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		result = append(result, svc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}

	// Load group_ids for each service
	for i := range result {
		groupIDs, err := r.GetServiceGroups(ctx, result[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get service groups: %w", err)
		}
		result[i].GroupIDs = groupIDs
	}

	return result, nil
}

// BeginTx starts a new transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

// UpdateServiceTx updates an existing service within a transaction.
func (r *Repository) UpdateServiceTx(ctx context.Context, tx pgx.Tx, service *domain.Service) error {
	query := `
		UPDATE services
		SET name = $2, slug = $3, description = $4, status = $5, "order" = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := tx.QueryRow(ctx, query,
		service.ID,
		service.Name,
		service.Slug,
		service.Description,
		service.Status,
		service.Order,
	).Scan(&service.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalog.ErrServiceNotFound
		}
		return fmt.Errorf("update service: %w", err)
	}
	return nil
}

// SetServiceGroupsTx replaces all group memberships for a service within a transaction.
func (r *Repository) SetServiceGroupsTx(ctx context.Context, tx pgx.Tx, serviceID string, groupIDs []string) error {
	// Delete old group memberships
	_, err := tx.Exec(ctx, `DELETE FROM service_group_members WHERE service_id = $1`, serviceID)
	if err != nil {
		return fmt.Errorf("delete old group memberships: %w", err)
	}

	// Insert new group memberships
	for _, groupID := range groupIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO service_group_members (service_id, group_id) VALUES ($1, $2)`,
			serviceID, groupID)
		if err != nil {
			return fmt.Errorf("insert group membership: %w", err)
		}
	}
	return nil
}

// UpdateServiceStatusTx updates the stored status of a service within a transaction.
func (r *Repository) UpdateServiceStatusTx(ctx context.Context, tx pgx.Tx, serviceID string, status domain.ServiceStatus) error {
	query := `UPDATE services SET status = $2, updated_at = NOW() WHERE id = $1`
	result, err := tx.Exec(ctx, query, serviceID, status)
	if err != nil {
		return fmt.Errorf("update service status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return catalog.ErrServiceNotFound
	}
	return nil
}

// CreateStatusLogEntry creates a new entry in the service status log.
func (r *Repository) CreateStatusLogEntry(ctx context.Context, entry *domain.ServiceStatusLogEntry) error {
	query := `
		INSERT INTO service_status_log (service_id, old_status, new_status, source_type, event_id, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		entry.ServiceID,
		entry.OldStatus,
		entry.NewStatus,
		entry.SourceType,
		entry.EventID,
		entry.Reason,
		entry.CreatedBy,
	).Scan(&entry.ID, &entry.CreatedAt)
}

// CreateStatusLogEntryTx creates a new entry in the service status log within a transaction.
func (r *Repository) CreateStatusLogEntryTx(ctx context.Context, tx pgx.Tx, entry *domain.ServiceStatusLogEntry) error {
	query := `
		INSERT INTO service_status_log (service_id, old_status, new_status, source_type, event_id, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	return tx.QueryRow(ctx, query,
		entry.ServiceID,
		entry.OldStatus,
		entry.NewStatus,
		entry.SourceType,
		entry.EventID,
		entry.Reason,
		entry.CreatedBy,
	).Scan(&entry.ID, &entry.CreatedAt)
}

// ListStatusLog returns the status change history for a service.
func (r *Repository) ListStatusLog(ctx context.Context, serviceID string, limit, offset int) ([]domain.ServiceStatusLogEntry, error) {
	query := `
		SELECT id, service_id, old_status, new_status, source_type, event_id, reason, created_by, created_at
		FROM service_status_log
		WHERE service_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, serviceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list status log: %w", err)
	}
	defer rows.Close()

	result := make([]domain.ServiceStatusLogEntry, 0)
	for rows.Next() {
		var entry domain.ServiceStatusLogEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.ServiceID,
			&entry.OldStatus,
			&entry.NewStatus,
			&entry.SourceType,
			&entry.EventID,
			&entry.Reason,
			&entry.CreatedBy,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan status log entry: %w", err)
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

// CountStatusLog returns the total number of log entries for a service.
func (r *Repository) CountStatusLog(ctx context.Context, serviceID string) (int, error) {
	query := `SELECT COUNT(*) FROM service_status_log WHERE service_id = $1`
	var count int
	err := r.db.QueryRow(ctx, query, serviceID).Scan(&count)
	return count, err
}

// DeleteStatusLogByEventIDTx deletes all status log entries for a given event within a transaction.
func (r *Repository) DeleteStatusLogByEventIDTx(ctx context.Context, tx pgx.Tx, eventID string) error {
	query := `DELETE FROM service_status_log WHERE event_id = $1`
	_, err := tx.Exec(ctx, query, eventID)
	return err
}

