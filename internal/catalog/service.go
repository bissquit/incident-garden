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
	ErrGroupNotFound   = errors.New("service group not found")
	ErrServiceNotFound = errors.New("service not found")
	ErrSlugExists      = errors.New("slug already exists")
	ErrInvalidSlug     = errors.New("invalid slug: must contain only lowercase letters, numbers, and hyphens")
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

// ListGroups returns all service groups.
func (s *Service) ListGroups(ctx context.Context) ([]domain.ServiceGroup, error) {
	return s.repo.ListGroups(ctx)
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

// DeleteGroup deletes a service group.
func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	return s.repo.DeleteGroup(ctx, id)
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

	return s.repo.CreateService(ctx, service)
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

	return s.repo.UpdateService(ctx, service)
}

// DeleteService deletes a service.
func (s *Service) DeleteService(ctx context.Context, id string) error {
	return s.repo.DeleteService(ctx, id)
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
