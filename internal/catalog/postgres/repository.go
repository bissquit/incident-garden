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
		SELECT id, name, slug, description, "order", created_at, updated_at
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
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, catalog.ErrGroupNotFound
		}
		return nil, fmt.Errorf("get service group by slug: %w", err)
	}
	return &group, nil
}

// GetGroupByID retrieves a service group by its ID.
func (r *Repository) GetGroupByID(ctx context.Context, id string) (*domain.ServiceGroup, error) {
	query := `
		SELECT id, name, slug, description, "order", created_at, updated_at
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
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, catalog.ErrGroupNotFound
		}
		return nil, fmt.Errorf("get service group by id: %w", err)
	}
	return &group, nil
}

// ListGroups retrieves all service groups ordered by order and name.
func (r *Repository) ListGroups(ctx context.Context) ([]domain.ServiceGroup, error) {
	query := `
		SELECT id, name, slug, description, "order", created_at, updated_at
		FROM service_groups
		ORDER BY "order", name
	`
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
		)
		if err != nil {
			return nil, fmt.Errorf("scan service group: %w", err)
		}
		groups = append(groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service groups: %w", err)
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
		INSERT INTO services (name, slug, description, status, group_id, "order")
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		service.Name,
		service.Slug,
		service.Description,
		service.Status,
		service.GroupID,
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
		SELECT id, name, slug, description, status, group_id, "order", created_at, updated_at
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
		&service.GroupID,
		&service.Order,
		&service.CreatedAt,
		&service.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, catalog.ErrServiceNotFound
		}
		return nil, fmt.Errorf("get service by slug: %w", err)
	}
	return &service, nil
}

// GetServiceByID retrieves a service by its ID.
func (r *Repository) GetServiceByID(ctx context.Context, id string) (*domain.Service, error) {
	query := `
		SELECT id, name, slug, description, status, group_id, "order", created_at, updated_at
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
		&service.GroupID,
		&service.Order,
		&service.CreatedAt,
		&service.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, catalog.ErrServiceNotFound
		}
		return nil, fmt.Errorf("get service by id: %w", err)
	}
	return &service, nil
}

// ListServices retrieves all services matching the provided filter.
func (r *Repository) ListServices(ctx context.Context, filter catalog.ServiceFilter) ([]domain.Service, error) {
	query := `
		SELECT id, name, slug, description, status, group_id, "order", created_at, updated_at
		FROM services
		WHERE ($1::uuid IS NULL OR group_id = $1)
		  AND ($2::text IS NULL OR status = $2)
		ORDER BY "order", name
	`
	rows, err := r.db.Query(ctx, query, filter.GroupID, filter.Status)
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
			&service.GroupID,
			&service.Order,
			&service.CreatedAt,
			&service.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, service)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}

	return services, nil
}

// UpdateService updates an existing service.
func (r *Repository) UpdateService(ctx context.Context, service *domain.Service) error {
	query := `
		UPDATE services
		SET name = $2, slug = $3, description = $4, status = $5, group_id = $6, "order" = $7, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.QueryRow(ctx, query,
		service.ID,
		service.Name,
		service.Slug,
		service.Description,
		service.Status,
		service.GroupID,
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
