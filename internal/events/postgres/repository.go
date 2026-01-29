// Package postgres provides PostgreSQL implementation of events repository.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/events"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

	serviceIDs, err := r.GetEventServices(ctx, id)
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

		serviceIDs, err := r.GetEventServices(ctx, event.ID)
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

	insertQuery := `INSERT INTO event_services (event_id, service_id) VALUES ($1, $2)`
	for _, serviceID := range serviceIDs {
		_, err := r.db.Exec(ctx, insertQuery, eventID, serviceID)
		if err != nil {
			return fmt.Errorf("associate service %s: %w", serviceID, err)
		}
	}

	return nil
}

// GetEventServices retrieves service IDs for an event.
func (r *Repository) GetEventServices(ctx context.Context, eventID string) ([]string, error) {
	query := `SELECT service_id FROM event_services WHERE event_id = $1`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event services: %w", err)
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
	query := `
		INSERT INTO event_service_changes (event_id, action, service_id, group_id, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		change.EventID,
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
		SELECT id, event_id, action, service_id, group_id, reason, created_by, created_at
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
