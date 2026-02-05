package events

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/pkg/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// Handler handles HTTP requests for events and templates.
type Handler struct {
	service   *Service
	validator *validator.Validate
}

// NewHandler creates a new events handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:   service,
		validator: validator.New(),
	}
}

// RegisterPublicRoutes registers public routes (no auth required).
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/status", h.GetPublicStatus)
	r.Get("/status/history", h.GetStatusHistory)
}

// RegisterPublicEventRoutes registers public read-only event routes (no auth required).
func (h *Handler) RegisterPublicEventRoutes(r chi.Router) {
	r.Get("/events", h.ListEvents)
	r.Get("/events/{id}", h.GetEvent)
	r.Get("/events/{id}/updates", h.GetEventUpdates)
	r.Get("/events/{id}/changes", h.GetServiceChanges)
}

// RegisterOperatorRoutes registers operator-level routes (write operations only).
func (h *Handler) RegisterOperatorRoutes(r chi.Router) {
	r.Post("/events", h.CreateEvent)
	r.Post("/events/{id}/updates", h.AddUpdate)
	r.Post("/events/{id}/services", h.AddServices)
	r.Delete("/events/{id}/services", h.RemoveServices)
}

// RegisterAdminRoutes registers admin-level routes.
func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Delete("/events/{id}", h.DeleteEvent)

	r.Route("/templates", func(r chi.Router) {
		r.Post("/", h.CreateTemplate)
		r.Get("/", h.ListTemplates)
		r.Get("/{slug}", h.GetTemplate)
		r.Post("/{slug}/preview", h.PreviewTemplate)
		r.Delete("/{id}", h.DeleteTemplate)
	})
}

// CreateEventRequest represents the request body for creating an event.
type CreateEventRequest struct {
	Title             string               `json:"title" validate:"required"`
	Type              domain.EventType     `json:"type" validate:"required"`
	Status            domain.EventStatus   `json:"status" validate:"required"`
	Severity          *domain.Severity     `json:"severity"`
	Description       string               `json:"description" validate:"required"`
	StartedAt         *time.Time           `json:"started_at"`
	ScheduledStartAt  *time.Time           `json:"scheduled_start_at"`
	ScheduledEndAt    *time.Time           `json:"scheduled_end_at"`
	NotifySubscribers bool                 `json:"notify_subscribers"`
	TemplateID        *string              `json:"template_id"`
	ServiceIDs        []string             `json:"service_ids"`
	GroupIDs          []string             `json:"group_ids"`
}

// CreateEvent handles POST /events.
func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	userID := httputil.GetUserID(r.Context())
	event, err := h.service.CreateEvent(r.Context(), CreateEventInput(req), userID)

	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, event)
}

// GetEvent handles GET /events/{id}.
func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	event, err := h.service.GetEvent(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, event)
}

// ListEvents handles GET /events.
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	filters := EventFilters{}

	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		eventType := domain.EventType(typeParam)
		filters.Type = &eventType
	}

	if statusParam := r.URL.Query().Get("status"); statusParam != "" {
		status := domain.EventStatus(statusParam)
		filters.Status = &status
	}

	events, err := h.service.ListEvents(r.Context(), filters)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, events)
}

// AddUpdateRequest represents the request body for adding an event update.
type AddUpdateRequest struct {
	Status            domain.EventStatus `json:"status" validate:"required"`
	Message           string             `json:"message" validate:"required"`
	NotifySubscribers bool               `json:"notify_subscribers"`
}

// AddUpdate handles POST /events/{id}/updates.
func (h *Handler) AddUpdate(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	var req AddUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	userID := httputil.GetUserID(r.Context())
	update, err := h.service.AddUpdate(r.Context(), CreateEventUpdateInput{
		EventID:           eventID,
		Status:            req.Status,
		Message:           req.Message,
		NotifySubscribers: req.NotifySubscribers,
	}, userID)

	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, update)
}

// GetEventUpdates handles GET /events/{id}/updates.
func (h *Handler) GetEventUpdates(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	updates, err := h.service.GetEventUpdates(r.Context(), eventID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, updates)
}

// DeleteEvent handles DELETE /events/{id}.
func (h *Handler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteEvent(r.Context(), id); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateTemplateRequest represents the request body for creating a template.
type CreateTemplateRequest struct {
	Slug          string           `json:"slug" validate:"required"`
	Type          domain.EventType `json:"type" validate:"required"`
	TitleTemplate string           `json:"title_template" validate:"required"`
	BodyTemplate  string           `json:"body_template" validate:"required"`
}

// CreateTemplate handles POST /templates.
func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	template, err := h.service.CreateTemplate(r.Context(), CreateTemplateInput(req))

	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, template)
}

// GetTemplate handles GET /templates/{slug}.
func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	template, err := h.service.GetTemplateBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, template)
}

// ListTemplates handles GET /templates.
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.service.ListTemplates(r.Context())
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, templates)
}

// PreviewTemplateRequest represents the request body for previewing a template.
type PreviewTemplateRequest struct {
	ServiceName      string     `json:"service_name"`
	ServiceGroupName string     `json:"service_group_name"`
	StartedAt        *time.Time `json:"started_at"`
	ResolvedAt       *time.Time `json:"resolved_at"`
	ScheduledStart   *time.Time `json:"scheduled_start"`
	ScheduledEnd     *time.Time `json:"scheduled_end"`
}

// PreviewTemplateResponse represents the response for template preview.
type PreviewTemplateResponse struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// PreviewTemplate handles POST /templates/{slug}/preview.
func (h *Handler) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var req PreviewTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	title, body, err := h.service.PreviewTemplate(r.Context(), slug, domain.TemplateData{
		ServiceName:      req.ServiceName,
		ServiceGroupName: req.ServiceGroupName,
		StartedAt:        req.StartedAt,
		ResolvedAt:       req.ResolvedAt,
		ScheduledStart:   req.ScheduledStart,
		ScheduledEnd:     req.ScheduledEnd,
	})

	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, PreviewTemplateResponse{
		Title: title,
		Body:  body,
	})
}

// DeleteTemplate handles DELETE /templates/{id}.
func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteTemplate(r.Context(), id); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetPublicStatus handles GET /status.
func (h *Handler) GetPublicStatus(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.ListEvents(r.Context(), EventFilters{Limit: 10})
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
	})
}

// GetStatusHistory handles GET /status/history.
func (h *Handler) GetStatusHistory(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.ListEvents(r.Context(), EventFilters{Limit: 50})
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
	})
}

// AddServicesRequest represents the request body for adding services to an event.
type AddServicesRequest struct {
	ServiceIDs []string `json:"service_ids"`
	GroupIDs   []string `json:"group_ids"`
	Reason     string   `json:"reason"`
}

// AddServices handles POST /events/{id}/services.
func (h *Handler) AddServices(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	var req AddServicesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if len(req.ServiceIDs) == 0 && len(req.GroupIDs) == 0 {
		h.respondError(w, http.StatusBadRequest, "service_ids or group_ids required")
		return
	}

	userID := httputil.GetUserID(r.Context())
	err := h.service.AddServicesToEvent(r.Context(), eventID, AddServicesToEventInput(req), userID)

	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Возвращаем обновлённое событие
	event, err := h.service.GetEvent(r.Context(), eventID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, event)
}

// RemoveServicesRequest represents the request body for removing services from an event.
type RemoveServicesRequest struct {
	ServiceIDs []string `json:"service_ids" validate:"required,min=1"`
	Reason     string   `json:"reason"`
}

// RemoveServices handles DELETE /events/{id}/services.
func (h *Handler) RemoveServices(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	var req RemoveServicesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	userID := httputil.GetUserID(r.Context())
	err := h.service.RemoveServicesFromEvent(r.Context(), eventID, RemoveServicesFromEventInput(req), userID)

	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Возвращаем обновлённое событие
	event, err := h.service.GetEvent(r.Context(), eventID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, event)
}

// GetServiceChanges handles GET /events/{id}/changes.
func (h *Handler) GetServiceChanges(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	changes, err := h.service.GetServiceChanges(r.Context(), eventID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, changes)
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"data": data}); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{"message": message},
	}); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

func (h *Handler) respondValidationError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": "validation error",
			"details": err.Error(),
		},
	}); err != nil {
		slog.Error("failed to encode validation error response", "error", err)
	}
}

func (h *Handler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrEventNotFound):
		h.respondError(w, http.StatusNotFound, "event not found")
	case errors.Is(err, ErrTemplateNotFound):
		h.respondError(w, http.StatusNotFound, "template not found")
	case errors.Is(err, ErrInvalidStatus):
		h.respondError(w, http.StatusBadRequest, "invalid status for event type")
	case errors.Is(err, ErrInvalidSeverity):
		h.respondError(w, http.StatusBadRequest, "severity is required for incidents")
	default:
		slog.Error("service error", "error", err)
		h.respondError(w, http.StatusInternalServerError, "internal server error")
	}
}
