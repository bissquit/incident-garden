// Package postgres provides PostgreSQL implementation of events repository.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/events"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// querier is an interface for database operations that both *pgxpool.Pool and pgx.Tx implement.
type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository implements events.Repository using PostgreSQL.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateEvent creates a new event in the database.
func (r *Repository) CreateEvent(ctx context.Context, event *domain.Event) error {
	query := `
		INSERT INTO events (
			title, type, status, severity, description,
			started_at, resolved_at, scheduled_start_at, scheduled_end_at,
			notify_subscribers, template_id, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		event.Title,
		event.Type,
		event.Status,
		event.Severity,
		event.Description,
		event.StartedAt,
		event.ResolvedAt,
		event.ScheduledStartAt,
		event.ScheduledEndAt,
		event.NotifySubscribers,
		event.TemplateID,
		event.CreatedBy,
	).Scan(&event.ID, &event.CreatedAt, &event.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	return nil
}

// GetEvent retrieves an event by ID.
func (r *Repository) GetEvent(ctx context.Context, id string) (*domain.Event, error) {
	query := `
		SELECT
			id, title, type, status, severity, description,
			started_at, resolved_at, scheduled_start_at, scheduled_end_at,
			notify_subscribers, template_id, created_by, created_at, updated_at
		FROM events
		WHERE id = $1
	`
	var event domain.Event
	err := r.db.QueryRow(ctx, query, id).Scan(
		&event.ID,
		&event.Title,
		&event.Type,
		&event.Status,
		&event.Severity,
		&event.Description,
		&event.StartedAt,
		&event.ResolvedAt,
		&event.ScheduledStartAt,
		&event.ScheduledEndAt,
		&event.NotifySubscribers,
		&event.TemplateID,
		&event.CreatedBy,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, events.ErrEventNotFound
		}
		return nil, fmt.Errorf("get event: %w", err)
	}

	serviceIDs, err := r.GetEventServiceIDs(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get event services: %w", err)
	}
	event.ServiceIDs = serviceIDs

	groupIDs, err := r.GetEventGroups(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get event groups: %w", err)
	}
	event.GroupIDs = groupIDs

	return &event, nil
}

// ListEvents retrieves events with optional filters.
func (r *Repository) ListEvents(ctx context.Context, filters events.EventFilters) ([]*domain.Event, error) {
	query := `
		SELECT 
			id, title, type, status, severity, description,
			started_at, resolved_at, scheduled_start_at, scheduled_end_at,
			notify_subscribers, template_id, created_by, created_at, updated_at
		FROM events
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if filters.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argNum)
		args = append(args, *filters.Type)
		argNum++
	}

	if filters.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, *filters.Status)
		argNum++
	}

	query += " ORDER BY created_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filters.Limit)
		argNum++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filters.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	eventsList := make([]*domain.Event, 0)
	for rows.Next() {
		var event domain.Event
		err := rows.Scan(
			&event.ID,
			&event.Title,
			&event.Type,
			&event.Status,
			&event.Severity,
			&event.Description,
			&event.StartedAt,
			&event.ResolvedAt,
			&event.ScheduledStartAt,
			&event.ScheduledEndAt,
			&event.NotifySubscribers,
			&event.TemplateID,
			&event.CreatedBy,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		serviceIDs, err := r.GetEventServiceIDs(ctx, event.ID)
		if err != nil {
			return nil, fmt.Errorf("get event services: %w", err)
		}
		event.ServiceIDs = serviceIDs

		groupIDs, err := r.GetEventGroups(ctx, event.ID)
		if err != nil {
			return nil, fmt.Errorf("get event groups: %w", err)
		}
		event.GroupIDs = groupIDs

		eventsList = append(eventsList, &event)
	}

	return eventsList, nil
}

// UpdateEvent updates an existing event.
func (r *Repository) UpdateEvent(ctx context.Context, event *domain.Event) error {
	query := `
		UPDATE events
		SET title = $2, status = $3, severity = $4, description = $5,
		    resolved_at = $6, scheduled_start_at = $7, scheduled_end_at = $8,
		    notify_subscribers = $9, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		event.ID,
		event.Title,
		event.Status,
		event.Severity,
		event.Description,
		event.ResolvedAt,
		event.ScheduledStartAt,
		event.ScheduledEndAt,
		event.NotifySubscribers,
	).Scan(&event.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return events.ErrEventNotFound
		}
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}

// DeleteEvent deletes an event by ID.
func (r *Repository) DeleteEvent(ctx context.Context, id string) error {
	query := `DELETE FROM events WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}

	if result.RowsAffected() == 0 {
		return events.ErrEventNotFound
	}
	return nil
}

// CreateEventUpdate creates a new event update.
func (r *Repository) CreateEventUpdate(ctx context.Context, update *domain.EventUpdate) error {
	query := `
		INSERT INTO event_updates (event_id, status, message, notify_subscribers, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		update.EventID,
		update.Status,
		update.Message,
		update.NotifySubscribers,
		update.CreatedBy,
	).Scan(&update.ID, &update.CreatedAt)

	if err != nil {
		return fmt.Errorf("create event update: %w", err)
	}
	return nil
}

// ListEventUpdates retrieves all updates for an event.
func (r *Repository) ListEventUpdates(ctx context.Context, eventID string) ([]*domain.EventUpdate, error) {
	query := `
		SELECT id, event_id, status, message, notify_subscribers, created_by, created_at
		FROM event_updates
		WHERE event_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("list event updates: %w", err)
	}
	defer rows.Close()

	updates := make([]*domain.EventUpdate, 0)
	for rows.Next() {
		var update domain.EventUpdate
		err := rows.Scan(
			&update.ID,
			&update.EventID,
			&update.Status,
			&update.Message,
			&update.NotifySubscribers,
			&update.CreatedBy,
			&update.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event update: %w", err)
		}
		updates = append(updates, &update)
	}

	return updates, nil
}

// CreateTemplate creates a new event template.
func (r *Repository) CreateTemplate(ctx context.Context, template *domain.EventTemplate) error {
	query := `
		INSERT INTO event_templates (slug, type, title_template, body_template)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		template.Slug,
		template.Type,
		template.TitleTemplate,
		template.BodyTemplate,
	).Scan(&template.ID, &template.CreatedAt, &template.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create template: %w", err)
	}
	return nil
}

// GetTemplate retrieves a template by ID.
func (r *Repository) GetTemplate(ctx context.Context, id string) (*domain.EventTemplate, error) {
	query := `
		SELECT id, slug, type, title_template, body_template, created_at, updated_at
		FROM event_templates
		WHERE id = $1
	`
	var template domain.EventTemplate
	err := r.db.QueryRow(ctx, query, id).Scan(
		&template.ID,
		&template.Slug,
		&template.Type,
		&template.TitleTemplate,
		&template.BodyTemplate,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, events.ErrTemplateNotFound
		}
		return nil, fmt.Errorf("get template: %w", err)
	}
	return &template, nil
}

// GetTemplateBySlug retrieves a template by slug.
func (r *Repository) GetTemplateBySlug(ctx context.Context, slug string) (*domain.EventTemplate, error) {
	query := `
		SELECT id, slug, type, title_template, body_template, created_at, updated_at
		FROM event_templates
		WHERE slug = $1
	`
	var template domain.EventTemplate
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&template.ID,
		&template.Slug,
		&template.Type,
		&template.TitleTemplate,
		&template.BodyTemplate,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, events.ErrTemplateNotFound
		}
		return nil, fmt.Errorf("get template by slug: %w", err)
	}
	return &template, nil
}

// ListTemplates retrieves all templates.
func (r *Repository) ListTemplates(ctx context.Context) ([]*domain.EventTemplate, error) {
	query := `
		SELECT id, slug, type, title_template, body_template, created_at, updated_at
		FROM event_templates
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	templates := make([]*domain.EventTemplate, 0)
	for rows.Next() {
		var template domain.EventTemplate
		err := rows.Scan(
			&template.ID,
			&template.Slug,
			&template.Type,
			&template.TitleTemplate,
			&template.BodyTemplate,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		templates = append(templates, &template)
	}

	return templates, nil
}

// UpdateTemplate updates an existing template.
func (r *Repository) UpdateTemplate(ctx context.Context, template *domain.EventTemplate) error {
	query := `
		UPDATE event_templates
		SET type = $2, title_template = $3, body_template = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		template.ID,
		template.Type,
		template.TitleTemplate,
		template.BodyTemplate,
	).Scan(&template.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return events.ErrTemplateNotFound
		}
		return fmt.Errorf("update template: %w", err)
	}
	return nil
}

// DeleteTemplate deletes a template by ID.
func (r *Repository) DeleteTemplate(ctx context.Context, id string) error {
	query := `DELETE FROM event_templates WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}

	if result.RowsAffected() == 0 {
		return events.ErrTemplateNotFound
	}
	return nil
}

// AssociateServices associates services with an event.
func (r *Repository) AssociateServices(ctx context.Context, eventID string, serviceIDs []string) error {
	deleteQuery := `DELETE FROM event_services WHERE event_id = $1`
	_, err := r.db.Exec(ctx, deleteQuery, eventID)
	if err != nil {
		return fmt.Errorf("delete existing event services: %w", err)
	}

	if len(serviceIDs) == 0 {
		return nil
	}

	for _, serviceID := range serviceIDs {
		if err := r.associateServiceWithStatus(ctx, r.db, eventID, serviceID, domain.ServiceStatusDegraded); err != nil {
			return err
		}
	}

	return nil
}

// GetEventServiceIDs retrieves service IDs for an event.
func (r *Repository) GetEventServiceIDs(ctx context.Context, eventID string) ([]string, error) {
	query := `SELECT service_id FROM event_services WHERE event_id = $1`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event service ids: %w", err)
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

	return serviceIDs, nil
}

// GetEventServices retrieves services with their statuses for an event.
func (r *Repository) GetEventServices(ctx context.Context, eventID string) ([]domain.EventService, error) {
	query := `
		SELECT event_id, service_id, status, updated_at
		FROM event_services
		WHERE event_id = $1
		ORDER BY service_id
	`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event services: %w", err)
	}
	defer rows.Close()

	result := make([]domain.EventService, 0)
	for rows.Next() {
		var es domain.EventService
		if err := rows.Scan(&es.EventID, &es.ServiceID, &es.Status, &es.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan event service: %w", err)
		}
		result = append(result, es)
	}

	return result, rows.Err()
}

// AssociateGroups replaces all group associations for an event.
func (r *Repository) AssociateGroups(ctx context.Context, eventID string, groupIDs []string) error {
	deleteQuery := `DELETE FROM event_groups WHERE event_id = $1`
	_, err := r.db.Exec(ctx, deleteQuery, eventID)
	if err != nil {
		return fmt.Errorf("delete existing event groups: %w", err)
	}

	if len(groupIDs) == 0 {
		return nil
	}

	insertQuery := `INSERT INTO event_groups (event_id, group_id) VALUES ($1, $2)`
	for _, groupID := range groupIDs {
		_, err := r.db.Exec(ctx, insertQuery, eventID, groupID)
		if err != nil {
			return fmt.Errorf("associate group %s: %w", groupID, err)
		}
	}

	return nil
}

// AddGroups adds groups to an event without removing existing ones.
func (r *Repository) AddGroups(ctx context.Context, eventID string, groupIDs []string) error {
	insertQuery := `INSERT INTO event_groups (event_id, group_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	for _, groupID := range groupIDs {
		_, err := r.db.Exec(ctx, insertQuery, eventID, groupID)
		if err != nil {
			return fmt.Errorf("add group %s: %w", groupID, err)
		}
	}
	return nil
}

// GetEventGroups retrieves group IDs for an event.
func (r *Repository) GetEventGroups(ctx context.Context, eventID string) ([]string, error) {
	query := `SELECT group_id FROM event_groups WHERE event_id = $1`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event groups: %w", err)
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

	return groupIDs, nil
}

// CreateServiceChange records a change to event services.
func (r *Repository) CreateServiceChange(ctx context.Context, change *domain.EventServiceChange) error {
	return r.createServiceChange(ctx, r.db, change)
}

// createServiceChange is a helper that works with both pool and transaction.
func (r *Repository) createServiceChange(ctx context.Context, q querier, change *domain.EventServiceChange) error {
	query := `
		INSERT INTO event_service_changes (event_id, batch_id, action, service_id, group_id, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	err := q.QueryRow(ctx, query,
		change.EventID,
		change.BatchID,
		change.Action,
		change.ServiceID,
		change.GroupID,
		change.Reason,
		change.CreatedBy,
	).Scan(&change.ID, &change.CreatedAt)

	if err != nil {
		return fmt.Errorf("create service change: %w", err)
	}
	return nil
}

// ListServiceChanges retrieves all service changes for an event.
func (r *Repository) ListServiceChanges(ctx context.Context, eventID string) ([]*domain.EventServiceChange, error) {
	query := `
		SELECT id, event_id, batch_id, action, service_id, group_id, reason, created_by, created_at
		FROM event_service_changes
		WHERE event_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("list service changes: %w", err)
	}
	defer rows.Close()

	changes := make([]*domain.EventServiceChange, 0)
	for rows.Next() {
		var change domain.EventServiceChange
		err := rows.Scan(
			&change.ID,
			&change.EventID,
			&change.BatchID,
			&change.Action,
			&change.ServiceID,
			&change.GroupID,
			&change.Reason,
			&change.CreatedBy,
			&change.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan service change: %w", err)
		}
		changes = append(changes, &change)
	}

	return changes, nil
}

// BeginTx starts a new database transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

// AssociateServicesTx associates services with an event within a transaction.
func (r *Repository) AssociateServicesTx(ctx context.Context, tx pgx.Tx, eventID string, serviceIDs []string) error {
	deleteQuery := `DELETE FROM event_services WHERE event_id = $1`
	_, err := tx.Exec(ctx, deleteQuery, eventID)
	if err != nil {
		return fmt.Errorf("delete existing event services: %w", err)
	}

	if len(serviceIDs) == 0 {
		return nil
	}

	for _, serviceID := range serviceIDs {
		if err := r.AssociateServiceWithStatusTx(ctx, tx, eventID, serviceID, domain.ServiceStatusDegraded); err != nil {
			return err
		}
	}

	return nil
}

// associateServiceWithStatus is a helper that works with both pool and transaction.
func (r *Repository) associateServiceWithStatus(ctx context.Context, q querier, eventID, serviceID string, status domain.ServiceStatus) error {
	query := `
		INSERT INTO event_services (event_id, service_id, status, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (event_id, service_id)
		DO UPDATE SET status = EXCLUDED.status, updated_at = NOW()
	`
	_, err := q.Exec(ctx, query, eventID, serviceID, status)
	if err != nil {
		return fmt.Errorf("associate service with status: %w", err)
	}
	return nil
}

// AssociateServiceWithStatusTx associates a service with an event and sets its status.
func (r *Repository) AssociateServiceWithStatusTx(ctx context.Context, tx pgx.Tx, eventID, serviceID string, status domain.ServiceStatus) error {
	return r.associateServiceWithStatus(ctx, tx, eventID, serviceID, status)
}

// UpdateEventServiceStatusTx updates the status of a service within an event.
func (r *Repository) UpdateEventServiceStatusTx(ctx context.Context, tx pgx.Tx, eventID, serviceID string, status domain.ServiceStatus) error {
	query := `
		UPDATE event_services
		SET status = $3, updated_at = NOW()
		WHERE event_id = $1 AND service_id = $2
	`
	result, err := tx.Exec(ctx, query, eventID, serviceID, status)
	if err != nil {
		return fmt.Errorf("update event service status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return events.ErrServiceNotInEvent
	}
	return nil
}

// AddGroupsTx adds groups to an event within a transaction.
func (r *Repository) AddGroupsTx(ctx context.Context, tx pgx.Tx, eventID string, groupIDs []string) error {
	insertQuery := `INSERT INTO event_groups (event_id, group_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	for _, groupID := range groupIDs {
		_, err := tx.Exec(ctx, insertQuery, eventID, groupID)
		if err != nil {
			return fmt.Errorf("add group %s: %w", groupID, err)
		}
	}
	return nil
}

// CreateServiceChangeTx records a change to event services within a transaction.
func (r *Repository) CreateServiceChangeTx(ctx context.Context, tx pgx.Tx, change *domain.EventServiceChange) error {
	return r.createServiceChange(ctx, tx, change)
}

// CreateEventTx creates a new event within a transaction.
func (r *Repository) CreateEventTx(ctx context.Context, tx pgx.Tx, event *domain.Event) error {
	query := `
		INSERT INTO events (
			title, type, status, severity, description,
			started_at, resolved_at, scheduled_start_at, scheduled_end_at,
			notify_subscribers, template_id, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at
	`
	err := tx.QueryRow(ctx, query,
		event.Title,
		event.Type,
		event.Status,
		event.Severity,
		event.Description,
		event.StartedAt,
		event.ResolvedAt,
		event.ScheduledStartAt,
		event.ScheduledEndAt,
		event.NotifySubscribers,
		event.TemplateID,
		event.CreatedBy,
	).Scan(&event.ID, &event.CreatedAt, &event.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	return nil
}

// AssociateGroupsTx replaces all group associations for an event within a transaction.
func (r *Repository) AssociateGroupsTx(ctx context.Context, tx pgx.Tx, eventID string, groupIDs []string) error {
	deleteQuery := `DELETE FROM event_groups WHERE event_id = $1`
	_, err := tx.Exec(ctx, deleteQuery, eventID)
	if err != nil {
		return fmt.Errorf("delete existing event groups: %w", err)
	}

	if len(groupIDs) == 0 {
		return nil
	}

	insertQuery := `INSERT INTO event_groups (event_id, group_id) VALUES ($1, $2)`
	for _, groupID := range groupIDs {
		_, err := tx.Exec(ctx, insertQuery, eventID, groupID)
		if err != nil {
			return fmt.Errorf("associate group %s: %w", groupID, err)
		}
	}

	return nil
}

// CreateEventUpdateTx creates a new event update within a transaction.
func (r *Repository) CreateEventUpdateTx(ctx context.Context, tx pgx.Tx, update *domain.EventUpdate) error {
	query := `
		INSERT INTO event_updates (event_id, status, message, notify_subscribers, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := tx.QueryRow(ctx, query,
		update.EventID,
		update.Status,
		update.Message,
		update.NotifySubscribers,
		update.CreatedBy,
	).Scan(&update.ID, &update.CreatedAt)

	if err != nil {
		return fmt.Errorf("create event update: %w", err)
	}
	return nil
}

// UpdateEventTx updates an existing event within a transaction.
func (r *Repository) UpdateEventTx(ctx context.Context, tx pgx.Tx, event *domain.Event) error {
	query := `
		UPDATE events
		SET title = $2, status = $3, severity = $4, description = $5,
		    resolved_at = $6, scheduled_start_at = $7, scheduled_end_at = $8,
		    notify_subscribers = $9, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := tx.QueryRow(ctx, query,
		event.ID,
		event.Title,
		event.Status,
		event.Severity,
		event.Description,
		event.ResolvedAt,
		event.ScheduledStartAt,
		event.ScheduledEndAt,
		event.NotifySubscribers,
	).Scan(&event.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return events.ErrEventNotFound
		}
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}

// IsServiceInEventTx checks if a service is associated with an event within a transaction.
func (r *Repository) IsServiceInEventTx(ctx context.Context, tx pgx.Tx, eventID, serviceID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM event_services WHERE event_id = $1 AND service_id = $2)`
	var exists bool
	err := tx.QueryRow(ctx, query, eventID, serviceID).Scan(&exists)
	return exists, err
}

// RemoveServiceFromEventTx removes a service from an event within a transaction.
func (r *Repository) RemoveServiceFromEventTx(ctx context.Context, tx pgx.Tx, eventID, serviceID string) error {
	query := `DELETE FROM event_services WHERE event_id = $1 AND service_id = $2`
	result, err := tx.Exec(ctx, query, eventID, serviceID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return events.ErrServiceNotInEvent
	}
	return nil
}

// AddGroupToEventTx adds a group association to an event within a transaction.
func (r *Repository) AddGroupToEventTx(ctx context.Context, tx pgx.Tx, eventID, groupID string) error {
	query := `
		INSERT INTO event_groups (event_id, group_id)
		VALUES ($1, $2)
		ON CONFLICT (event_id, group_id) DO NOTHING
	`
	_, err := tx.Exec(ctx, query, eventID, groupID)
	return err
}

// GetEventServiceIDsTx returns all service IDs associated with an event within a transaction.
func (r *Repository) GetEventServiceIDsTx(ctx context.Context, tx pgx.Tx, eventID string) ([]string, error) {
	query := `SELECT service_id FROM event_services WHERE event_id = $1`
	rows, err := tx.Query(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// HasOtherActiveEventsTx checks if a service has other active events (excluding the specified event).
func (r *Repository) HasOtherActiveEventsTx(ctx context.Context, tx pgx.Tx, serviceID, excludeEventID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM event_services es
			JOIN events e ON es.event_id = e.id
			WHERE es.service_id = $1
			  AND e.id != $2
			  AND e.status NOT IN ('resolved', 'completed')
		)
	`
	var exists bool
	err := tx.QueryRow(ctx, query, serviceID, excludeEventID).Scan(&exists)
	return exists, err
}

// GetEventServiceStatusTx returns the status of a service in an event context.
func (r *Repository) GetEventServiceStatusTx(ctx context.Context, tx pgx.Tx, eventID, serviceID string) (domain.ServiceStatus, error) {
	query := `SELECT status FROM event_services WHERE event_id = $1 AND service_id = $2`
	var status domain.ServiceStatus
	err := tx.QueryRow(ctx, query, eventID, serviceID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", events.ErrServiceNotInEvent
		}
		return "", err
	}
	return status, nil
}

// ListEventsByServiceID returns events associated with a service.
func (r *Repository) ListEventsByServiceID(ctx context.Context, serviceID string, filter events.ServiceEventFilter) ([]*domain.Event, error) {
	query := `
		SELECT
			e.id, e.title, e.type, e.status, e.severity, e.description,
			e.started_at, e.resolved_at, e.scheduled_start_at, e.scheduled_end_at,
			e.notify_subscribers, e.template_id, e.created_by,
			e.created_at, e.updated_at
		FROM events e
		WHERE EXISTS (SELECT 1 FROM event_services es WHERE es.event_id = e.id AND es.service_id = $1)
	`
	args := []interface{}{serviceID}
	argNum := 2

	switch filter.Status {
	case "active":
		query += " AND e.status NOT IN ('resolved', 'completed')"
	case "resolved":
		query += " AND e.status IN ('resolved', 'completed')"
	}

	query += `
		ORDER BY
			CASE WHEN e.status NOT IN ('resolved', 'completed') THEN 0 ELSE 1 END,
			e.created_at DESC
	`

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filter.Limit)
		argNum++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events by service: %w", err)
	}
	defer rows.Close()

	eventsList := make([]*domain.Event, 0)
	for rows.Next() {
		var event domain.Event
		if err := rows.Scan(
			&event.ID,
			&event.Title,
			&event.Type,
			&event.Status,
			&event.Severity,
			&event.Description,
			&event.StartedAt,
			&event.ResolvedAt,
			&event.ScheduledStartAt,
			&event.ScheduledEndAt,
			&event.NotifySubscribers,
			&event.TemplateID,
			&event.CreatedBy,
			&event.CreatedAt,
			&event.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		eventsList = append(eventsList, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load service_ids and group_ids for each event
	for _, event := range eventsList {
		serviceIDs, err := r.GetEventServiceIDs(ctx, event.ID)
		if err != nil {
			return nil, fmt.Errorf("get event services: %w", err)
		}
		event.ServiceIDs = serviceIDs

		groupIDs, err := r.GetEventGroups(ctx, event.ID)
		if err != nil {
			return nil, fmt.Errorf("get event groups: %w", err)
		}
		event.GroupIDs = groupIDs
	}

	return eventsList, nil
}

// CountEventsByServiceID returns the total count of events for a service.
func (r *Repository) CountEventsByServiceID(ctx context.Context, serviceID string, filter events.ServiceEventFilter) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM events e
		WHERE EXISTS (SELECT 1 FROM event_services es WHERE es.event_id = e.id AND es.service_id = $1)
	`

	switch filter.Status {
	case "active":
		query += " AND e.status NOT IN ('resolved', 'completed')"
	case "resolved":
		query += " AND e.status IN ('resolved', 'completed')"
	}

	var count int
	err := r.db.QueryRow(ctx, query, serviceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count events by service: %w", err)
	}
	return count, nil
}
