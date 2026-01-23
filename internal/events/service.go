package events

import (
	"context"
	"fmt"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
)

// Service implements event business logic.
type Service struct {
	repo     Repository
	renderer *TemplateRenderer
}

// NewService creates a new event service.
func NewService(repo Repository) *Service {
	return &Service{
		repo:     repo,
		renderer: NewTemplateRenderer(),
	}
}

// CreateEventInput holds data for creating an event.
type CreateEventInput struct {
	Title             string
	Type              domain.EventType
	Status            domain.EventStatus
	Severity          *domain.Severity
	Description       string
	StartedAt         *time.Time
	ScheduledStartAt  *time.Time
	ScheduledEndAt    *time.Time
	NotifySubscribers bool
	TemplateID        *string
	ServiceIDs        []string
}

// CreateEventUpdateInput holds data for creating an event update.
type CreateEventUpdateInput struct {
	EventID           string
	Status            domain.EventStatus
	Message           string
	NotifySubscribers bool
}

// CreateTemplateInput holds data for creating a template.
type CreateTemplateInput struct {
	Slug          string
	Type          domain.EventType
	TitleTemplate string
	BodyTemplate  string
}

// CreateEvent creates a new event with validation.
func (s *Service) CreateEvent(ctx context.Context, input CreateEventInput, createdBy string) (*domain.Event, error) {
	if !input.Type.IsValid() {
		return nil, fmt.Errorf("invalid event type: %s", input.Type)
	}

	if !input.Status.IsValidForType(input.Type) {
		return nil, ErrInvalidStatus
	}

	if input.Type == domain.EventTypeIncident && input.Severity == nil {
		return nil, ErrInvalidSeverity
	}

	if input.Type == domain.EventTypeIncident && input.Severity != nil {
		if !input.Severity.IsValid() {
			return nil, fmt.Errorf("invalid severity: %s", *input.Severity)
		}
	}

	event := &domain.Event{
		Title:             input.Title,
		Type:              input.Type,
		Status:            input.Status,
		Severity:          input.Severity,
		Description:       input.Description,
		StartedAt:         input.StartedAt,
		ScheduledStartAt:  input.ScheduledStartAt,
		ScheduledEndAt:    input.ScheduledEndAt,
		NotifySubscribers: input.NotifySubscribers,
		TemplateID:        input.TemplateID,
		CreatedBy:         createdBy,
	}

	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}

	if len(input.ServiceIDs) > 0 {
		if err := s.repo.AssociateServices(ctx, event.ID, input.ServiceIDs); err != nil {
			return nil, fmt.Errorf("associate services: %w", err)
		}
		event.ServiceIDs = input.ServiceIDs
	}

	return event, nil
}

// GetEvent retrieves an event by ID.
func (s *Service) GetEvent(ctx context.Context, id string) (*domain.Event, error) {
	return s.repo.GetEvent(ctx, id)
}

// ListEvents retrieves events with optional filters.
func (s *Service) ListEvents(ctx context.Context, filters EventFilters) ([]*domain.Event, error) {
	return s.repo.ListEvents(ctx, filters)
}

// AddUpdate adds an update to an event and updates its status.
func (s *Service) AddUpdate(ctx context.Context, input CreateEventUpdateInput, createdBy string) (*domain.EventUpdate, error) {
	event, err := s.repo.GetEvent(ctx, input.EventID)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}

	if !input.Status.IsValidForType(event.Type) {
		return nil, ErrInvalidStatus
	}

	update := &domain.EventUpdate{
		EventID:           input.EventID,
		Status:            input.Status,
		Message:           input.Message,
		NotifySubscribers: input.NotifySubscribers,
		CreatedBy:         createdBy,
	}

	if err := s.repo.CreateEventUpdate(ctx, update); err != nil {
		return nil, fmt.Errorf("create event update: %w", err)
	}

	event.Status = input.Status
	if input.Status.IsResolved() && event.ResolvedAt == nil {
		now := time.Now()
		event.ResolvedAt = &now
	}

	if err := s.repo.UpdateEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("update event status: %w", err)
	}

	return update, nil
}

// GetEventUpdates retrieves all updates for an event.
func (s *Service) GetEventUpdates(ctx context.Context, eventID string) ([]*domain.EventUpdate, error) {
	return s.repo.ListEventUpdates(ctx, eventID)
}

// DeleteEvent deletes an event by ID.
func (s *Service) DeleteEvent(ctx context.Context, id string) error {
	return s.repo.DeleteEvent(ctx, id)
}

// CreateTemplate creates a new event template with validation.
func (s *Service) CreateTemplate(ctx context.Context, input CreateTemplateInput) (*domain.EventTemplate, error) {
	if !input.Type.IsValid() {
		return nil, fmt.Errorf("invalid event type: %s", input.Type)
	}

	if err := s.renderer.Validate(input.TitleTemplate); err != nil {
		return nil, fmt.Errorf("invalid title template: %w", err)
	}

	if err := s.renderer.Validate(input.BodyTemplate); err != nil {
		return nil, fmt.Errorf("invalid body template: %w", err)
	}

	template := &domain.EventTemplate{
		Slug:          input.Slug,
		Type:          input.Type,
		TitleTemplate: input.TitleTemplate,
		BodyTemplate:  input.BodyTemplate,
	}

	if err := s.repo.CreateTemplate(ctx, template); err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}

	return template, nil
}

// GetTemplate retrieves a template by ID.
func (s *Service) GetTemplate(ctx context.Context, id string) (*domain.EventTemplate, error) {
	return s.repo.GetTemplate(ctx, id)
}

// GetTemplateBySlug retrieves a template by slug.
func (s *Service) GetTemplateBySlug(ctx context.Context, slug string) (*domain.EventTemplate, error) {
	return s.repo.GetTemplateBySlug(ctx, slug)
}

// ListTemplates retrieves all templates.
func (s *Service) ListTemplates(ctx context.Context) ([]*domain.EventTemplate, error) {
	return s.repo.ListTemplates(ctx)
}

// PreviewTemplate renders a template with provided data.
func (s *Service) PreviewTemplate(ctx context.Context, templateSlug string, data domain.TemplateData) (string, string, error) {
	template, err := s.repo.GetTemplateBySlug(ctx, templateSlug)
	if err != nil {
		return "", "", fmt.Errorf("get template: %w", err)
	}

	title, err := s.renderer.Render(template.TitleTemplate, data)
	if err != nil {
		return "", "", fmt.Errorf("render title: %w", err)
	}

	body, err := s.renderer.Render(template.BodyTemplate, data)
	if err != nil {
		return "", "", fmt.Errorf("render body: %w", err)
	}

	return title, body, nil
}

// DeleteTemplate deletes a template by ID.
func (s *Service) DeleteTemplate(ctx context.Context, id string) error {
	return s.repo.DeleteTemplate(ctx, id)
}
