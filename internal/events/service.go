package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

	// Validate affected entities exist before starting transaction
	if err := s.validateAffectedEntities(ctx, input.AffectedServices, input.AffectedGroups); err != nil {
		return nil, err
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
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("failed to rollback transaction", "error", err)
		}
	}()

	if err := s.repo.CreateEventTx(ctx, tx, event); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}

	// Associate services with their statuses and log status changes
	serviceIDs := make([]string, 0, len(serviceStatuses))
	for serviceID, status := range serviceStatuses {
		// Get current service status for audit log
		currentStatus, err := s.catalogService.GetServiceStatus(ctx, serviceID)
		if err != nil {
			return nil, fmt.Errorf("get current status for %s: %w", serviceID, err)
		}

		if err := s.repo.AssociateServiceWithStatusTx(ctx, tx, event.ID, serviceID, status); err != nil {
			return nil, fmt.Errorf("associate service %s: %w", serviceID, err)
		}

		// Log status change
		logEntry := &domain.ServiceStatusLogEntry{
			ServiceID:  serviceID,
			OldStatus:  &currentStatus,
			NewStatus:  status,
			SourceType: domain.StatusLogSourceEvent,
			EventID:    &event.ID,
			Reason:     fmt.Sprintf("Event created: %s", input.Title),
			CreatedBy:  createdBy,
		}
		if err := s.catalogService.CreateStatusLogEntryTx(ctx, tx, logEntry); err != nil {
			return nil, fmt.Errorf("create status log: %w", err)
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

	if event.Status.IsResolved() {
		return nil, ErrEventAlreadyResolved
	}

	if err := s.validateAffectedEntities(ctx, input.AddServices, input.AddGroups); err != nil {
		return nil, err
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("failed to rollback transaction", "error", err)
		}
	}()

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

	event.Status = input.Status
	if input.Status.IsResolved() && event.ResolvedAt == nil {
		now := time.Now()
		event.ResolvedAt = &now
	}
	if err := s.repo.UpdateEventTx(ctx, tx, event); err != nil {
		return nil, fmt.Errorf("update event: %w", err)
	}

	if err := s.processServiceChanges(ctx, tx, input, createdBy); err != nil {
		return nil, err
	}

	if err := s.handleResolution(ctx, tx, input, createdBy); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return update, nil
}

// processServiceChanges handles all service modifications in a single batch.
func (s *Service) processServiceChanges(ctx context.Context, tx pgx.Tx, input CreateEventUpdateInput, createdBy string) error {
	if !s.hasServiceChanges(input) {
		return nil
	}

	batchID := uuid.New().String()
	if err := s.updateExistingServiceStatuses(ctx, tx, input.EventID, input.ServiceUpdates, input.Reason, createdBy); err != nil {
		return err
	}
	if err := s.addServicesToEvent(ctx, tx, input.EventID, batchID, input.AddServices, input.Reason, createdBy); err != nil {
		return err
	}
	if err := s.addGroupsToEvent(ctx, tx, input.EventID, batchID, input.AddGroups, input.Reason, createdBy); err != nil {
		return err
	}
	return s.removeServicesFromEvent(ctx, tx, input.EventID, batchID, input.RemoveServiceIDs, input.Reason, createdBy)
}

// handleResolution recalculates service statuses when event is resolved.
func (s *Service) handleResolution(ctx context.Context, tx pgx.Tx, input CreateEventUpdateInput, createdBy string) error {
	if !input.Status.IsResolved() {
		return nil
	}

	affectedServiceIDs, err := s.repo.GetEventServiceIDsTx(ctx, tx, input.EventID)
	if err != nil {
		return fmt.Errorf("get event services: %w", err)
	}
	return s.recalculateServicesStoredStatus(ctx, tx, affectedServiceIDs, input.EventID, createdBy)
}

// hasServiceChanges returns true if input contains any service modifications.
func (s *Service) hasServiceChanges(input CreateEventUpdateInput) bool {
	return len(input.ServiceUpdates) > 0 || len(input.AddServices) > 0 ||
		len(input.AddGroups) > 0 || len(input.RemoveServiceIDs) > 0
}

// updateExistingServiceStatuses updates statuses of services already in event.
func (s *Service) updateExistingServiceStatuses(ctx context.Context, tx pgx.Tx, eventID string, updates []domain.AffectedService, reason, createdBy string) error {
	for _, su := range updates {
		currentEventStatus, err := s.repo.GetEventServiceStatusTx(ctx, tx, eventID, su.ServiceID)
		if err != nil {
			return fmt.Errorf("get current event service status: %w", err)
		}

		if currentEventStatus == su.Status {
			continue
		}

		if err := s.repo.UpdateEventServiceStatusTx(ctx, tx, eventID, su.ServiceID, su.Status); err != nil {
			return fmt.Errorf("update service %s status: %w", su.ServiceID, err)
		}

		logEntry := &domain.ServiceStatusLogEntry{
			ServiceID:  su.ServiceID,
			OldStatus:  &currentEventStatus,
			NewStatus:  su.Status,
			SourceType: domain.StatusLogSourceEvent,
			EventID:    &eventID,
			Reason:     reason,
			CreatedBy:  createdBy,
		}
		if err := s.catalogService.CreateStatusLogEntryTx(ctx, tx, logEntry); err != nil {
			return fmt.Errorf("create status log: %w", err)
		}
	}
	return nil
}

// addServicesToEvent adds new services to event with audit trail.
func (s *Service) addServicesToEvent(ctx context.Context, tx pgx.Tx, eventID, batchID string, services []domain.AffectedService, reason, createdBy string) error {
	for _, as := range services {
		added, err := s.associateServiceIfNotExists(ctx, tx, eventID, as.ServiceID, as.Status, reason, createdBy)
		if err != nil {
			return err
		}
		if !added {
			continue
		}

		sid := as.ServiceID
		change := &domain.EventServiceChange{
			EventID:   eventID,
			BatchID:   &batchID,
			Action:    domain.ChangeActionAdded,
			ServiceID: &sid,
			Reason:    reason,
			CreatedBy: createdBy,
		}
		if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
			return fmt.Errorf("record change: %w", err)
		}
	}
	return nil
}

// addGroupsToEvent adds groups (expanding to services) with audit trail.
func (s *Service) addGroupsToEvent(ctx context.Context, tx pgx.Tx, eventID, batchID string, groups []domain.AffectedGroup, reason, createdBy string) error {
	for _, ag := range groups {
		groupServiceIDs, err := s.resolver.GetGroupServices(ctx, ag.GroupID)
		if err != nil {
			return fmt.Errorf("resolve group %s: %w", ag.GroupID, err)
		}

		for _, sid := range groupServiceIDs {
			if _, err := s.associateServiceIfNotExists(ctx, tx, eventID, sid, ag.Status, reason, createdBy); err != nil {
				return err
			}
		}

		if err := s.repo.AddGroupToEventTx(ctx, tx, eventID, ag.GroupID); err != nil {
			return fmt.Errorf("add group to event: %w", err)
		}

		gid := ag.GroupID
		change := &domain.EventServiceChange{
			EventID:   eventID,
			BatchID:   &batchID,
			Action:    domain.ChangeActionAdded,
			GroupID:   &gid,
			Reason:    reason,
			CreatedBy: createdBy,
		}
		if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
			return err
		}
	}
	return nil
}

// associateServiceIfNotExists adds service to event if not already present.
// Returns true if service was added, false if it already existed.
func (s *Service) associateServiceIfNotExists(ctx context.Context, tx pgx.Tx, eventID, serviceID string, status domain.ServiceStatus, reason, createdBy string) (bool, error) {
	exists, err := s.repo.IsServiceInEventTx(ctx, tx, eventID, serviceID)
	if err != nil {
		return false, fmt.Errorf("check service in event: %w", err)
	}
	if exists {
		return false, nil
	}

	currentStatus, _ := s.catalogService.GetServiceStatus(ctx, serviceID)

	if err := s.repo.AssociateServiceWithStatusTx(ctx, tx, eventID, serviceID, status); err != nil {
		return false, fmt.Errorf("add service: %w", err)
	}

	logEntry := &domain.ServiceStatusLogEntry{
		ServiceID:  serviceID,
		OldStatus:  &currentStatus,
		NewStatus:  status,
		SourceType: domain.StatusLogSourceEvent,
		EventID:    &eventID,
		Reason:     reason,
		CreatedBy:  createdBy,
	}
	if err := s.catalogService.CreateStatusLogEntryTx(ctx, tx, logEntry); err != nil {
		return false, fmt.Errorf("create status log: %w", err)
	}

	return true, nil
}

// removeServicesFromEvent removes services from event with audit trail.
func (s *Service) removeServicesFromEvent(ctx context.Context, tx pgx.Tx, eventID, batchID string, serviceIDs []string, reason, createdBy string) error {
	for _, sid := range serviceIDs {
		if err := s.repo.RemoveServiceFromEventTx(ctx, tx, eventID, sid); err != nil {
			if err == ErrServiceNotInEvent {
				continue
			}
			return fmt.Errorf("remove service: %w", err)
		}

		sidCopy := sid
		change := &domain.EventServiceChange{
			EventID:   eventID,
			BatchID:   &batchID,
			Action:    domain.ChangeActionRemoved,
			ServiceID: &sidCopy,
			Reason:    reason,
			CreatedBy: createdBy,
		}
		if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
			return err
		}
	}
	return nil
}

// recalculateServicesStoredStatus recalculates stored status for services after event resolution.
func (s *Service) recalculateServicesStoredStatus(ctx context.Context, tx pgx.Tx, serviceIDs []string, excludeEventID, updatedBy string) error {
	for _, serviceID := range serviceIDs {
		hasOther, err := s.repo.HasOtherActiveEventsTx(ctx, tx, serviceID, excludeEventID)
		if err != nil {
			return fmt.Errorf("check other events for %s: %w", serviceID, err)
		}

		if !hasOther {
			// Get current status for audit log
			var oldStatus *domain.ServiceStatus
			if currentStatus, err := s.catalogService.GetServiceStatus(ctx, serviceID); err != nil {
				slog.Warn("failed to get current service status for audit log",
					"service_id", serviceID,
					"error", err)
				// Continue with nil old_status
			} else {
				oldStatus = &currentStatus
			}
			newStatus := domain.ServiceStatusOperational

			// No other active events → set service to operational
			if err := s.catalogService.UpdateServiceStatusTx(ctx, tx, serviceID, newStatus); err != nil {
				return fmt.Errorf("update service %s status: %w", serviceID, err)
			}

			// Log status change
			logEntry := &domain.ServiceStatusLogEntry{
				ServiceID:  serviceID,
				OldStatus:  oldStatus,
				NewStatus:  newStatus,
				SourceType: domain.StatusLogSourceEvent,
				EventID:    &excludeEventID,
				Reason:     "Event resolved, no other active events",
				CreatedBy:  updatedBy,
			}
			if err := s.catalogService.CreateStatusLogEntryTx(ctx, tx, logEntry); err != nil {
				return fmt.Errorf("create status log: %w", err)
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

// DeleteEvent deletes a resolved event and all associated data.
//
// Deletion rules:
// 1. Only resolved/completed events can be deleted
// 2. Active events must be resolved first
//
// What gets deleted (via CASCADE and explicit deletion):
// - event_services: service associations with statuses
// - event_groups: group associations
// - event_updates: status updates
// - event_service_changes: audit trail of service additions/removals
// - service_status_log: entries referencing this event
//
// What is NOT affected:
// - services.status: stored status remains unchanged
// - effective_status: since event is already resolved, it wasn't affecting effective status
//
// This is a destructive operation that removes historical data.
func (s *Service) DeleteEvent(ctx context.Context, id string) error {
	event, err := s.repo.GetEvent(ctx, id)
	if err != nil {
		return fmt.Errorf("get event: %w", err)
	}

	if !event.Status.IsResolved() {
		return ErrEventNotResolved
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("failed to rollback transaction", "error", err)
		}
	}()

	// Delete status log entries referencing this event
	if err := s.catalogService.DeleteStatusLogByEventIDTx(ctx, tx, id); err != nil {
		return fmt.Errorf("delete status log entries: %w", err)
	}

	// Delete event (CASCADE will delete event_services, event_groups, event_updates, event_service_changes)
	if err := s.repo.DeleteEventTx(ctx, tx, id); err != nil {
		return fmt.Errorf("delete event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
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

// validateAffectedEntities checks that all referenced services and groups exist and are not archived.
func (s *Service) validateAffectedEntities(ctx context.Context, services []domain.AffectedService, groups []domain.AffectedGroup) error {
	// Collect service IDs
	serviceIDs := make([]string, 0, len(services))
	for _, as := range services {
		serviceIDs = append(serviceIDs, as.ServiceID)
	}

	// Validate services exist
	if len(serviceIDs) > 0 {
		missing, err := s.catalogService.ValidateServicesExist(ctx, serviceIDs)
		if err != nil {
			return fmt.Errorf("validate services: %w", err)
		}
		if len(missing) > 0 {
			return fmt.Errorf("%w: %s", ErrAffectedServiceNotFound, missing[0])
		}
	}

	// Collect group IDs
	groupIDs := make([]string, 0, len(groups))
	for _, ag := range groups {
		groupIDs = append(groupIDs, ag.GroupID)
	}

	// Validate groups exist
	if len(groupIDs) > 0 {
		missing, err := s.resolver.ValidateGroupsExist(ctx, groupIDs)
		if err != nil {
			return fmt.Errorf("validate groups: %w", err)
		}
		if len(missing) > 0 {
			return fmt.Errorf("%w: %s", ErrAffectedGroupNotFound, missing[0])
		}
	}

	return nil
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

// ListEventsByServiceID returns events associated with a service.
func (s *Service) ListEventsByServiceID(ctx context.Context, serviceID string, filter ServiceEventFilter) ([]*domain.Event, int, error) {
	eventsList, err := s.repo.ListEventsByServiceID(ctx, serviceID, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list events: %w", err)
	}

	total, err := s.repo.CountEventsByServiceID(ctx, serviceID, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}

	return eventsList, total, nil
}
