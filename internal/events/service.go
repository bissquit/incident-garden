package events

import (
	"context"
	"fmt"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Service implements event business logic.
type Service struct {
	repo           Repository
	resolver       GroupServiceResolver
	catalogService CatalogServiceUpdater
	renderer       *TemplateRenderer
}

// NewService creates a new event service.
func NewService(repo Repository, resolver GroupServiceResolver, catalogService CatalogServiceUpdater) *Service {
	return &Service{
		repo:           repo,
		resolver:       resolver,
		catalogService: catalogService,
		renderer:       NewTemplateRenderer(),
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
	ResolvedAt        *time.Time // For creating past events
	ScheduledStartAt  *time.Time
	ScheduledEndAt    *time.Time
	NotifySubscribers bool
	TemplateID        *string
	AffectedServices  []domain.AffectedService
	AffectedGroups    []domain.AffectedGroup
}

// CreateEventUpdateInput holds data for creating an event update.
type CreateEventUpdateInput struct {
	EventID           string
	Status            domain.EventStatus
	Message           string
	NotifySubscribers bool
	ServiceUpdates    []domain.AffectedService // Update statuses of existing services
	AddServices       []domain.AffectedService // Add new services
	AddGroups         []domain.AffectedGroup   // Add groups (expand to services)
	RemoveServiceIDs  []string                 // Remove services from event
	Reason            string                   // Reason for changes (audit)
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

	// Collect all services with their statuses.
	// Explicit services override group-derived statuses.
	serviceStatuses := make(map[string]domain.ServiceStatus)

	// First, add services from groups
	groupIDs := make([]string, 0, len(input.AffectedGroups))
	for _, ag := range input.AffectedGroups {
		groupIDs = append(groupIDs, ag.GroupID)
		serviceIDs, err := s.resolver.GetGroupServices(ctx, ag.GroupID)
		if err != nil {
			return nil, fmt.Errorf("resolve group %s: %w", ag.GroupID, err)
		}
		for _, sid := range serviceIDs {
			// Don't overwrite if already set (explicit service takes priority)
			if _, exists := serviceStatuses[sid]; !exists {
				serviceStatuses[sid] = ag.Status
			}
		}
	}

	// Then add explicitly specified services (they override group statuses)
	for _, as := range input.AffectedServices {
		serviceStatuses[as.ServiceID] = as.Status
	}

	event := &domain.Event{
		Title:             input.Title,
		Type:              input.Type,
		Status:            input.Status,
		Severity:          input.Severity,
		Description:       input.Description,
		StartedAt:         input.StartedAt,
		ResolvedAt:        input.ResolvedAt,
		ScheduledStartAt:  input.ScheduledStartAt,
		ScheduledEndAt:    input.ScheduledEndAt,
		NotifySubscribers: input.NotifySubscribers,
		TemplateID:        input.TemplateID,
		CreatedBy:         createdBy,
		GroupIDs:          groupIDs,
	}

	// Begin transaction for atomicity
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.CreateEventTx(ctx, tx, event); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}

	// Associate services with their statuses
	serviceIDs := make([]string, 0, len(serviceStatuses))
	for serviceID, status := range serviceStatuses {
		if err := s.repo.AssociateServiceWithStatusTx(ctx, tx, event.ID, serviceID, status); err != nil {
			return nil, fmt.Errorf("associate service %s: %w", serviceID, err)
		}
		serviceIDs = append(serviceIDs, serviceID)
	}
	event.ServiceIDs = serviceIDs

	// Save group associations
	if len(groupIDs) > 0 {
		if err := s.repo.AssociateGroupsTx(ctx, tx, event.ID, groupIDs); err != nil {
			return nil, fmt.Errorf("associate groups: %w", err)
		}
	}

	// Record initial state in change history
	if err := s.recordInitialChangesTx(ctx, tx, event.ID, input.AffectedServices, input.AffectedGroups, createdBy); err != nil {
		return nil, fmt.Errorf("record initial changes: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
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

// AddUpdate adds an update to an event and optionally modifies service associations.
func (s *Service) AddUpdate(ctx context.Context, input CreateEventUpdateInput, createdBy string) (*domain.EventUpdate, error) {
	event, err := s.repo.GetEvent(ctx, input.EventID)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}

	if !input.Status.IsValidForType(event.Type) {
		return nil, ErrInvalidStatus
	}

	// Check if event is already resolved
	if event.Status.IsResolved() {
		return nil, ErrEventAlreadyResolved
	}

	// Begin transaction
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Create EventUpdate
	update := &domain.EventUpdate{
		EventID:           input.EventID,
		Status:            input.Status,
		Message:           input.Message,
		NotifySubscribers: input.NotifySubscribers,
		CreatedBy:         createdBy,
	}
	if err := s.repo.CreateEventUpdateTx(ctx, tx, update); err != nil {
		return nil, fmt.Errorf("create update: %w", err)
	}

	// Update event status
	event.Status = input.Status
	if input.Status.IsResolved() && event.ResolvedAt == nil {
		now := time.Now()
		event.ResolvedAt = &now
	}
	if err := s.repo.UpdateEventTx(ctx, tx, event); err != nil {
		return nil, fmt.Errorf("update event: %w", err)
	}

	// Process service changes
	hasServiceChanges := len(input.ServiceUpdates) > 0 || len(input.AddServices) > 0 ||
		len(input.AddGroups) > 0 || len(input.RemoveServiceIDs) > 0

	if hasServiceChanges {
		batchID := uuid.New().String()
		reason := input.Reason
		if reason == "" {
			reason = "Event update"
		}

		// Update statuses of existing services
		for _, su := range input.ServiceUpdates {
			if err := s.repo.UpdateEventServiceStatusTx(ctx, tx, input.EventID, su.ServiceID, su.Status); err != nil {
				return nil, fmt.Errorf("update service %s status: %w", su.ServiceID, err)
			}
		}

		// Add new services
		for _, as := range input.AddServices {
			exists, err := s.repo.IsServiceInEventTx(ctx, tx, input.EventID, as.ServiceID)
			if err != nil {
				return nil, fmt.Errorf("check service in event: %w", err)
			}
			if exists {
				continue
			}

			if err := s.repo.AssociateServiceWithStatusTx(ctx, tx, input.EventID, as.ServiceID, as.Status); err != nil {
				return nil, fmt.Errorf("add service: %w", err)
			}

			sid := as.ServiceID
			change := &domain.EventServiceChange{
				EventID:   input.EventID,
				BatchID:   &batchID,
				Action:    domain.ChangeActionAdded,
				ServiceID: &sid,
				Reason:    reason,
				CreatedBy: createdBy,
			}
			if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
				return nil, fmt.Errorf("record change: %w", err)
			}
		}

		// Add groups
		for _, ag := range input.AddGroups {
			serviceIDs, err := s.resolver.GetGroupServices(ctx, ag.GroupID)
			if err != nil {
				return nil, fmt.Errorf("resolve group %s: %w", ag.GroupID, err)
			}

			for _, sid := range serviceIDs {
				exists, err := s.repo.IsServiceInEventTx(ctx, tx, input.EventID, sid)
				if err != nil {
					return nil, err
				}
				if exists {
					continue
				}

				if err := s.repo.AssociateServiceWithStatusTx(ctx, tx, input.EventID, sid, ag.Status); err != nil {
					return nil, err
				}
			}

			if err := s.repo.AddGroupToEventTx(ctx, tx, input.EventID, ag.GroupID); err != nil {
				return nil, fmt.Errorf("add group to event: %w", err)
			}

			gid := ag.GroupID
			change := &domain.EventServiceChange{
				EventID:   input.EventID,
				BatchID:   &batchID,
				Action:    domain.ChangeActionAdded,
				GroupID:   &gid,
				Reason:    reason,
				CreatedBy: createdBy,
			}
			if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
				return nil, err
			}
		}

		// Remove services
		for _, sid := range input.RemoveServiceIDs {
			if err := s.repo.RemoveServiceFromEventTx(ctx, tx, input.EventID, sid); err != nil {
				if err == ErrServiceNotInEvent {
					continue
				}
				return nil, fmt.Errorf("remove service: %w", err)
			}

			sidCopy := sid
			change := &domain.EventServiceChange{
				EventID:   input.EventID,
				BatchID:   &batchID,
				Action:    domain.ChangeActionRemoved,
				ServiceID: &sidCopy,
				Reason:    reason,
				CreatedBy: createdBy,
			}
			if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
				return nil, err
			}
		}
	}

	// If event is being resolved, recalculate stored status for affected services
	if input.Status.IsResolved() {
		affectedServiceIDs, err := s.repo.GetEventServiceIDsTx(ctx, tx, input.EventID)
		if err != nil {
			return nil, fmt.Errorf("get event services: %w", err)
		}

		if err := s.recalculateServicesStoredStatus(ctx, tx, affectedServiceIDs, input.EventID); err != nil {
			return nil, fmt.Errorf("recalculate statuses: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return update, nil
}

// recalculateServicesStoredStatus recalculates stored status for services after event resolution.
func (s *Service) recalculateServicesStoredStatus(ctx context.Context, tx pgx.Tx, serviceIDs []string, excludeEventID string) error {
	for _, serviceID := range serviceIDs {
		hasOther, err := s.repo.HasOtherActiveEventsTx(ctx, tx, serviceID, excludeEventID)
		if err != nil {
			return fmt.Errorf("check other events for %s: %w", serviceID, err)
		}

		if !hasOther {
			// No other active events → set service to operational
			if err := s.catalogService.UpdateServiceStatusTx(ctx, tx, serviceID, domain.ServiceStatusOperational); err != nil {
				return fmt.Errorf("update service %s status: %w", serviceID, err)
			}
		}
		// If there are other active events, stored status stays as-is
		// and effective_status will be computed via worst-case from remaining events
	}
	return nil
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

// recordInitialChangesTx records initial event composition in change history.
func (s *Service) recordInitialChangesTx(ctx context.Context, tx pgx.Tx, eventID string, services []domain.AffectedService, groups []domain.AffectedGroup, createdBy string) error {
	if len(services) == 0 && len(groups) == 0 {
		return nil
	}

	batchID := uuid.New().String()

	// Record individual service additions
	for _, as := range services {
		sid := as.ServiceID
		change := &domain.EventServiceChange{
			EventID:   eventID,
			BatchID:   &batchID,
			Action:    domain.ChangeActionAdded,
			ServiceID: &sid,
			Reason:    "Initial event creation",
			CreatedBy: createdBy,
		}
		if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
			return fmt.Errorf("record service change: %w", err)
		}
	}

	// Record group additions
	for _, ag := range groups {
		gid := ag.GroupID
		change := &domain.EventServiceChange{
			EventID:   eventID,
			BatchID:   &batchID,
			Action:    domain.ChangeActionAdded,
			GroupID:   &gid,
			Reason:    "Initial event creation",
			CreatedBy: createdBy,
		}
		if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
			return fmt.Errorf("record group change: %w", err)
		}
	}

	return nil
}

// GetServiceChanges returns the history of service changes for an event.
func (s *Service) GetServiceChanges(ctx context.Context, eventID string) ([]*domain.EventServiceChange, error) {
	return s.repo.ListServiceChanges(ctx, eventID)
}
