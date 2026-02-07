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
	repo     Repository
	resolver GroupServiceResolver
	renderer *TemplateRenderer
}

// NewService creates a new event service.
func NewService(repo Repository, resolver GroupServiceResolver) *Service {
	return &Service{
		repo:     repo,
		resolver: resolver,
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
	GroupIDs          []string
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

	// Развернуть группы в сервисы
	allServiceIDs := make(map[string]bool)
	for _, sid := range input.ServiceIDs {
		allServiceIDs[sid] = true
	}

	for _, groupID := range input.GroupIDs {
		serviceIDs, err := s.resolver.GetGroupServices(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf("resolve group %s: %w", groupID, err)
		}
		for _, sid := range serviceIDs {
			allServiceIDs[sid] = true
		}
	}

	// Преобразовать map в slice
	uniqueServiceIDs := make([]string, 0, len(allServiceIDs))
	for sid := range allServiceIDs {
		uniqueServiceIDs = append(uniqueServiceIDs, sid)
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
		GroupIDs:          input.GroupIDs,
	}

	// Начинаем транзакцию для атомарности всех операций
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.CreateEventTx(ctx, tx, event); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}

	// Сохранить связи с сервисами с правильным статусом на основе severity
	if len(uniqueServiceIDs) > 0 {
		serviceStatus := domain.SeverityToServiceStatus(input.Type, input.Severity)
		for _, serviceID := range uniqueServiceIDs {
			if err := s.repo.AssociateServiceWithStatusTx(ctx, tx, event.ID, serviceID, serviceStatus); err != nil {
				return nil, fmt.Errorf("associate services: %w", err)
			}
		}
		event.ServiceIDs = uniqueServiceIDs
	}

	// Сохранить связи с группами
	if len(input.GroupIDs) > 0 {
		if err := s.repo.AssociateGroupsTx(ctx, tx, event.ID, input.GroupIDs); err != nil {
			return nil, fmt.Errorf("associate groups: %w", err)
		}
	}

	// Записать начальное состояние в историю изменений
	if err := s.recordInitialServicesTx(ctx, tx, event.ID, input.ServiceIDs, input.GroupIDs, createdBy); err != nil {
		return nil, fmt.Errorf("record initial services: %w", err)
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

// recordInitialServicesTx записывает начальный состав события в историю в рамках транзакции.
func (s *Service) recordInitialServicesTx(ctx context.Context, tx pgx.Tx, eventID string, serviceIDs, groupIDs []string, createdBy string) error {
	if len(serviceIDs) == 0 && len(groupIDs) == 0 {
		return nil
	}

	batchID := uuid.New().String()

	// Записываем добавление отдельных сервисов
	for i := range serviceIDs {
		sid := serviceIDs[i]
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

	// Записываем добавление групп
	for i := range groupIDs {
		gid := groupIDs[i]
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

// AddServicesToEventInput holds data for adding services to an event.
type AddServicesToEventInput struct {
	ServiceIDs []string
	GroupIDs   []string
	Reason     string
}

// AddServicesToEvent adds services and/or groups to an existing event.
func (s *Service) AddServicesToEvent(ctx context.Context, eventID string, input AddServicesToEventInput, userID string) error {
	event, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get event: %w", err)
	}

	// Собираем текущие сервисы
	currentServices := make(map[string]bool)
	for _, sid := range event.ServiceIDs {
		currentServices[sid] = true
	}

	// Развернуть новые группы
	newServiceIDs := make([]string, 0)
	for _, groupID := range input.GroupIDs {
		serviceIDs, err := s.resolver.GetGroupServices(ctx, groupID)
		if err != nil {
			return fmt.Errorf("resolve group %s: %w", groupID, err)
		}
		for _, sid := range serviceIDs {
			if !currentServices[sid] {
				newServiceIDs = append(newServiceIDs, sid)
				currentServices[sid] = true
			}
		}
	}

	// Добавить отдельные сервисы
	for _, sid := range input.ServiceIDs {
		if !currentServices[sid] {
			newServiceIDs = append(newServiceIDs, sid)
			currentServices[sid] = true
		}
	}

	if len(newServiceIDs) == 0 && len(input.GroupIDs) == 0 {
		return nil // Ничего не изменилось
	}

	// Генерируем batch_id для группировки изменений
	batchID := uuid.New().String()

	// Начинаем транзакцию
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Обновить связи с сервисами с правильным статусом на основе severity события
	serviceStatus := domain.SeverityToServiceStatus(event.Type, event.Severity)
	for _, sid := range newServiceIDs {
		if err := s.repo.AssociateServiceWithStatusTx(ctx, tx, eventID, sid, serviceStatus); err != nil {
			return fmt.Errorf("update services: %w", err)
		}
	}

	// Добавить группы к событию
	if len(input.GroupIDs) > 0 {
		if err := s.repo.AddGroupsTx(ctx, tx, eventID, input.GroupIDs); err != nil {
			return fmt.Errorf("add groups: %w", err)
		}
	}

	// Записать изменения в историю
	for i := range input.ServiceIDs {
		sid := input.ServiceIDs[i]
		change := &domain.EventServiceChange{
			EventID:   eventID,
			BatchID:   &batchID,
			Action:    domain.ChangeActionAdded,
			ServiceID: &sid,
			Reason:    input.Reason,
			CreatedBy: userID,
		}
		if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
			return fmt.Errorf("record service change: %w", err)
		}
	}

	for i := range input.GroupIDs {
		gid := input.GroupIDs[i]
		change := &domain.EventServiceChange{
			EventID:   eventID,
			BatchID:   &batchID,
			Action:    domain.ChangeActionAdded,
			GroupID:   &gid,
			Reason:    input.Reason,
			CreatedBy: userID,
		}
		if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
			return fmt.Errorf("record group change: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// RemoveServicesFromEventInput holds data for removing services from an event.
type RemoveServicesFromEventInput struct {
	ServiceIDs []string
	Reason     string
}

// RemoveServicesFromEvent removes services from an existing event.
func (s *Service) RemoveServicesFromEvent(ctx context.Context, eventID string, input RemoveServicesFromEventInput, userID string) error {
	event, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get event: %w", err)
	}

	// Собираем текущие сервисы и удаляем указанные
	currentServices := make(map[string]bool)
	for _, sid := range event.ServiceIDs {
		currentServices[sid] = true
	}

	removedServices := make([]string, 0)
	for _, sid := range input.ServiceIDs {
		if currentServices[sid] {
			delete(currentServices, sid)
			removedServices = append(removedServices, sid)
		}
	}

	if len(removedServices) == 0 {
		return nil // Ничего не изменилось
	}

	// Генерируем batch_id для группировки изменений
	batchID := uuid.New().String()

	// Начинаем транзакцию
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Обновить связи
	remainingServiceIDs := make([]string, 0, len(currentServices))
	for sid := range currentServices {
		remainingServiceIDs = append(remainingServiceIDs, sid)
	}
	if err := s.repo.AssociateServicesTx(ctx, tx, eventID, remainingServiceIDs); err != nil {
		return fmt.Errorf("update services: %w", err)
	}

	// Записать изменения в историю
	for i := range removedServices {
		sid := removedServices[i]
		change := &domain.EventServiceChange{
			EventID:   eventID,
			BatchID:   &batchID,
			Action:    domain.ChangeActionRemoved,
			ServiceID: &sid,
			Reason:    input.Reason,
			CreatedBy: userID,
		}
		if err := s.repo.CreateServiceChangeTx(ctx, tx, change); err != nil {
			return fmt.Errorf("record service change: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetServiceChanges returns the history of service changes for an event.
func (s *Service) GetServiceChanges(ctx context.Context, eventID string) ([]*domain.EventServiceChange, error) {
	return s.repo.ListServiceChanges(ctx, eventID)
}
