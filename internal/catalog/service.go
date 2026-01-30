package catalog

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/bissquit/incident-garden/internal/domain"
)

// Catalog service errors.
var (
	ErrGroupNotFound          = errors.New("service group not found")
	ErrServiceNotFound        = errors.New("service not found")
	ErrSlugExists             = errors.New("slug already exists")
	ErrInvalidSlug            = errors.New("invalid slug: must contain only lowercase letters, numbers, and hyphens")
	ErrServiceHasActiveEvents = errors.New("cannot archive service: has active events")
	ErrGroupHasActiveEvents   = errors.New("cannot archive group: has active events")
	ErrAlreadyArchived        = errors.New("already archived")
	ErrNotArchived            = errors.New("not archived")
)

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Service provides business logic for managing service groups and services.
type Service struct {
	repo Repository
}

// NewService creates a new catalog service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateGroup creates a new service group.
func (s *Service) CreateGroup(ctx context.Context, group *domain.ServiceGroup) error {
	if err := validateSlug(group.Slug); err != nil {
		return err
	}

	existing, err := s.repo.GetGroupBySlug(ctx, group.Slug)
	if err != nil && !errors.Is(err, ErrGroupNotFound) {
		return fmt.Errorf("check slug uniqueness: %w", err)
	}
	if existing != nil {
		return ErrSlugExists
	}

	return s.repo.CreateGroup(ctx, group)
}

// GetGroupBySlug returns a service group by its slug.
func (s *Service) GetGroupBySlug(ctx context.Context, slug string) (*domain.ServiceGroup, error) {
	return s.repo.GetGroupBySlug(ctx, slug)
}

// GetGroupByID returns a service group by its ID.
func (s *Service) GetGroupByID(ctx context.Context, id string) (*domain.ServiceGroup, error) {
	return s.repo.GetGroupByID(ctx, id)
}

// ListGroups returns all service groups matching the filter.
func (s *Service) ListGroups(ctx context.Context, filter GroupFilter) ([]domain.ServiceGroup, error) {
	return s.repo.ListGroups(ctx, filter)
}

// UpdateGroup updates an existing service group.
func (s *Service) UpdateGroup(ctx context.Context, group *domain.ServiceGroup) error {
	if err := validateSlug(group.Slug); err != nil {
		return err
	}

	existing, err := s.repo.GetGroupByID(ctx, group.ID)
	if err != nil {
		return err
	}

	if existing.Slug != group.Slug {
		existingBySlug, err := s.repo.GetGroupBySlug(ctx, group.Slug)
		if err != nil && !errors.Is(err, ErrGroupNotFound) {
			return fmt.Errorf("check slug uniqueness: %w", err)
		}
		if existingBySlug != nil {
			return ErrSlugExists
		}
	}

	return s.repo.UpdateGroup(ctx, group)
}

// DeleteGroup archives a service group (soft delete).
func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	// Check for active events in services belonging to this group
	activeCount, err := s.repo.GetActiveEventCountForGroup(ctx, id)
	if err != nil {
		return fmt.Errorf("check active events: %w", err)
	}
	if activeCount > 0 {
		return ErrGroupHasActiveEvents
	}

	return s.repo.ArchiveGroup(ctx, id)
}

// RestoreGroup restores an archived group.
func (s *Service) RestoreGroup(ctx context.Context, id string) error {
	return s.repo.RestoreGroup(ctx, id)
}

// CreateService creates a new service.
func (s *Service) CreateService(ctx context.Context, service *domain.Service) error {
	if err := validateSlug(service.Slug); err != nil {
		return err
	}

	existing, err := s.repo.GetServiceBySlug(ctx, service.Slug)
	if err != nil && !errors.Is(err, ErrServiceNotFound) {
		return fmt.Errorf("check slug uniqueness: %w", err)
	}
	if existing != nil {
		return ErrSlugExists
	}

	if service.Status == "" {
		service.Status = domain.ServiceStatusOperational
	}

	if err := s.repo.CreateService(ctx, service); err != nil {
		return err
	}

	// Set service groups if provided
	if len(service.GroupIDs) > 0 {
		if err := s.repo.SetServiceGroups(ctx, service.ID, service.GroupIDs); err != nil {
			return fmt.Errorf("set service groups: %w", err)
		}
	}

	return nil
}

// GetServiceBySlug returns a service by its slug.
func (s *Service) GetServiceBySlug(ctx context.Context, slug string) (*domain.Service, error) {
	return s.repo.GetServiceBySlug(ctx, slug)
}

// GetServiceByID returns a service by its ID.
func (s *Service) GetServiceByID(ctx context.Context, id string) (*domain.Service, error) {
	return s.repo.GetServiceByID(ctx, id)
}

// ListServices returns all services matching the filter.
func (s *Service) ListServices(ctx context.Context, filter ServiceFilter) ([]domain.Service, error) {
	return s.repo.ListServices(ctx, filter)
}

// UpdateService updates an existing service.
func (s *Service) UpdateService(ctx context.Context, service *domain.Service) error {
	if err := validateSlug(service.Slug); err != nil {
		return err
	}

	existing, err := s.repo.GetServiceByID(ctx, service.ID)
	if err != nil {
		return err
	}

	if existing.Slug != service.Slug {
		existingBySlug, err := s.repo.GetServiceBySlug(ctx, service.Slug)
		if err != nil && !errors.Is(err, ErrServiceNotFound) {
			return fmt.Errorf("check slug uniqueness: %w", err)
		}
		if existingBySlug != nil {
			return ErrSlugExists
		}
	}

	if err := s.repo.UpdateService(ctx, service); err != nil {
		return err
	}

	// Update service groups
	if err := s.repo.SetServiceGroups(ctx, service.ID, service.GroupIDs); err != nil {
		return fmt.Errorf("set service groups: %w", err)
	}

	return nil
}

// DeleteService archives a service (soft delete).
func (s *Service) DeleteService(ctx context.Context, id string) error {
	// Check for active events
	activeCount, err := s.repo.GetActiveEventCountForService(ctx, id)
	if err != nil {
		return fmt.Errorf("check active events: %w", err)
	}
	if activeCount > 0 {
		return ErrServiceHasActiveEvents
	}

	return s.repo.ArchiveService(ctx, id)
}

// RestoreService restores an archived service.
func (s *Service) RestoreService(ctx context.Context, id string) error {
	return s.repo.RestoreService(ctx, id)
}

// UpdateServiceTags replaces all tags for a service.
func (s *Service) UpdateServiceTags(ctx context.Context, serviceID string, tagsMap map[string]string) error {
	_, err := s.repo.GetServiceByID(ctx, serviceID)
	if err != nil {
		return err
	}

	tags := make([]domain.ServiceTag, 0, len(tagsMap))
	for key, value := range tagsMap {
		tags = append(tags, domain.ServiceTag{
			ServiceID: serviceID,
			Key:       key,
			Value:     value,
		})
	}

	return s.repo.SetServiceTags(ctx, serviceID, tags)
}

// GetServiceTags returns all tags for a service.
func (s *Service) GetServiceTags(ctx context.Context, serviceID string) ([]domain.ServiceTag, error) {
	return s.repo.GetServiceTags(ctx, serviceID)
}

// GetGroupServices returns all service IDs in a group.
func (s *Service) GetGroupServices(ctx context.Context, groupID string) ([]string, error) {
	return s.repo.GetGroupServices(ctx, groupID)
}

func validateSlug(slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ErrInvalidSlug
	}
	if !slugRegex.MatchString(slug) {
		return ErrInvalidSlug
	}
	return nil
}
